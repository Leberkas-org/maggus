package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/leberkas-org/maggus/internal/agent"
	"gopkg.in/yaml.v3"
	"github.com/leberkas-org/maggus/internal/bryan"
	"github.com/leberkas-org/maggus/internal/config"
	"github.com/leberkas-org/maggus/internal/git"
	"github.com/leberkas-org/maggus/internal/ipc"
	maotel "github.com/leberkas-org/maggus/internal/otel"
)

type MainDaemon struct {
	cfg        config.Config
	log        *slog.Logger
	queue      *TaskQueue
	state      *State
	dispatcher *Dispatcher
	pool       *WorkerPool
	gitOps     git.Operations
	agents     *agent.Registry
	bryan      bryan.Client
	importers  map[string]*Importer
	imported   map[string]bool
	watcher    *fsnotify.Watcher
	cancel     context.CancelFunc
}

func New(cfg config.Config, gitOps git.Operations, agents *agent.Registry, logger *slog.Logger) *MainDaemon {
	queue := NewTaskQueue()
	pool := NewWorkerPool(cfg.MaxWorkers)

	globalDir, _ := config.GlobalDir()
	stateWriter := ipc.NewFileStateWriter(globalDir)
	state := NewState(stateWriter, queue)

	dispatcher := NewDispatcher(queue, pool, gitOps, agents, cfg, state, logger)

	return &MainDaemon{
		cfg:        cfg,
		log:        logger,
		queue:      queue,
		state:      state,
		dispatcher: dispatcher,
		pool:       pool,
		gitOps:     gitOps,
		agents:     agents,
		importers:  make(map[string]*Importer),
		imported:   make(map[string]bool),
	}
}

func (d *MainDaemon) Run(ctx context.Context) error {
	ctx, d.cancel = context.WithCancel(ctx)

	globalDir, err := config.GlobalDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(globalDir, 0o755); err != nil {
		return err
	}

	lock, err := AcquireLock()
	if err != nil {
		return fmt.Errorf("daemon already running")
	}
	defer lock.Release()

	// Connect to Bryan if configured
	if d.cfg.Bryan != nil {
		globalDir, _ := config.GlobalDir()
		keysDir := filepath.Join(globalDir, "keys")
		client, err := bryan.NewGRPCClient(d.cfg.Bryan.Address, keysDir)
		if err != nil {
			d.log.Error("bryan client init failed", "error", err)
		} else {
			d.bryan = client
			repos := make([]string, 0, len(d.cfg.Repos))
			for _, r := range d.cfg.Repos {
				repos = append(repos, r.URL)
			}
			if err := d.bryan.Connect(ctx, d.cfg.Bryan.MachineID, repos); err != nil {
				d.log.Warn("bryan connect failed", "error", err)
			}
		}

		// Initialize OTel when Bryan is connected
		shutdown, err := maotel.InitOTel(ctx, d.cfg.Bryan.Address)
		if err != nil {
			d.log.Warn("otel init failed", "error", err)
		} else {
			defer shutdown()
			if err := maotel.InitMetrics(); err != nil {
				d.log.Warn("otel metrics init failed", "error", err)
			}
		}
	}

	// Set up fsnotify watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("create watcher: %w", err)
	}
	d.watcher = watcher
	defer watcher.Close()

	// Watch global dir for config changes and IPC commands
	if err := watcher.Add(globalDir); err != nil {
		d.log.Warn("watch global dir failed", "path", globalDir, "error", err)
	}

	// Load existing items + watch for new plans — fast, no git commands
	for _, repo := range d.cfg.Repos {
		d.registerRepo(repo)
	}

	// Process stale approve/skip commands from while daemon was down
	// but ignore stop commands — those are stale from a previous session
	d.processStaleCommands(globalDir)

	_ = d.state.ForceFlush()

	// Recover dirty state in background — slow git operations
	go func() {
		for _, repo := range d.cfg.Repos {
			if err := d.gitOps.RecoverDirtyState(repo.Path); err != nil {
				d.log.Warn("recovery failed", "repo", repo.Path, "error", err)
			}
		}
		d.log.Debug("recovery complete")
	}()

	// Dispatcher tick — still needs periodic check for ready items
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			d.pool.CancelAll()
			d.state.MarkDirty()
			_ = d.state.Flush()
			d.log.Info("daemon stopped")
			return nil
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			d.handleFSEvent(event, globalDir)
		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			d.log.Warn("watcher error", "error", err)
		case <-ticker.C:
			d.dispatcher.Tick()
			_ = d.state.Flush()
		}
	}
}

func (d *MainDaemon) registerRepo(repo config.RepoEntry) {
	tasksDir := filepath.Join(repo.Path, ".maggus", "tasks")
	if err := os.MkdirAll(tasksDir, 0o755); err != nil {
		d.log.Warn("create tasks dir", "path", tasksDir, "error", err)
		return
	}
	d.importers[repo.Path] = NewImporter(tasksDir)
	if err := d.watcher.Add(tasksDir); err != nil {
		d.log.Warn("watch tasks dir failed", "path", tasksDir, "error", err)
	}
	d.scanForPlans(repo)
}

func (d *MainDaemon) handleFSEvent(event fsnotify.Event, globalDir string) {
	name := filepath.Base(event.Name)
	dir := filepath.Dir(event.Name)

	// Skip noise from our own state file writes and log writes
	if name == "state.json" || name == "state.json.tmp" || name == "daemon.log" || name == "daemon.lock" {
		return
	}

	d.log.Debug("fs event", "name", name, "op", event.Op.String())

	// Config change → check for new repos
	if event.Name == filepath.Join(globalDir, "config.yml") {
		if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
			d.refreshRepos()
		}
		return
	}

	// IPC command files
	if dir == globalDir && strings.HasPrefix(name, "cmd.") {
		d.processCommands(globalDir)
		return
	}

	// Plan file dropped in a tasks dir
	if strings.HasSuffix(name, ".md") && (event.Has(fsnotify.Create) || event.Has(fsnotify.Write)) {
		for _, repo := range d.cfg.Repos {
			tasksDir := filepath.Join(repo.Path, ".maggus", "tasks")
			if dir == tasksDir {
				d.importSinglePlan(repo, event.Name)
				_ = d.state.Flush()
				return
			}
		}
	}
}

func (d *MainDaemon) importSinglePlan(repo config.RepoEntry, planPath string) {
	if d.imported[planPath] {
		return
	}
	imp := d.importers[repo.Path]
	if imp == nil {
		return
	}
	item, err := imp.Import(planPath, d.gitOps.RepoURL(repo.Path), d.cfg.AutoApprove)
	if err != nil {
		d.log.Error("import failed", "path", planPath, "error", err)
		d.imported[planPath] = true
		return
	}
	d.imported[planPath] = true
	item.RepoPath = repo.Path
	d.queue.Enqueue(item)
	d.state.MarkDirty()
	d.log.Info("imported plan", "title", item.Title, "tasks", len(item.Tasks))
}

func (d *MainDaemon) Stop() error {
	d.pool.CancelAll()
	return nil
}

func (d *MainDaemon) refreshRepos() {
	cfg, err := config.Load()
	if err != nil {
		return
	}
	for _, repo := range cfg.Repos {
		if _, ok := d.importers[repo.Path]; ok {
			continue
		}
		d.cfg.Repos = append(d.cfg.Repos, repo)
		d.registerRepo(repo)
		d.log.Info("registered new repo", "path", repo.Path)
	}
}

func (d *MainDaemon) scanForPlans(repo config.RepoEntry) {
	tasksDir := filepath.Join(repo.Path, ".maggus", "tasks")
	entries, err := os.ReadDir(tasksDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() {
			// Check for existing imported items
			d.loadExistingItem(repo, filepath.Join(tasksDir, entry.Name()))
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		planPath := filepath.Join(tasksDir, entry.Name())
		d.importSinglePlan(repo, planPath)
	}
}

func (d *MainDaemon) loadExistingItem(repo config.RepoEntry, itemDir string) {
	yamlPath := filepath.Join(itemDir, "item.yaml")
	data, err := os.ReadFile(yamlPath)
	if err != nil {
		return
	}

	var meta ItemMeta
	if err := yaml.Unmarshal(data, &meta); err != nil {
		d.log.Warn("parse item.yaml", "path", yamlPath, "error", err)
		return
	}

	// Skip if already in queue
	if d.queue.Get(meta.ItemID) != nil {
		return
	}

	// Load context
	contextData, _ := os.ReadFile(filepath.Join(itemDir, "context.md"))

	var tasks []TaskSpec
	for _, t := range meta.Tasks {
		taskContent, _ := os.ReadFile(filepath.Join(itemDir, t.File))
		tasks = append(tasks, TaskSpec{
			ID:      t.ID,
			Title:   t.Title,
			Content: string(taskContent),
			Status:  ItemStatus(t.Status),
		})
	}

	item := &WorkItem{
		ID:          meta.ItemID,
		PlanFile:    filepath.Join(itemDir, "source.md"),
		RepoURL:     meta.RepoURL,
		RepoPath:    repo.Path,
		Title:       meta.Title,
		Description: string(contextData),
		Tasks:       tasks,
		Status:      ItemStatus(meta.Status),
		Priority:    meta.Priority,
	}

	d.queue.Enqueue(item)
	d.state.MarkDirty()
	d.log.Info("loaded existing item", "title", meta.Title, "status", meta.Status, "tasks", len(tasks))
}

func (d *MainDaemon) processStaleCommands(globalDir string) {
	// Remove stale stop commands
	os.Remove(filepath.Join(globalDir, "cmd.stop"))

	entries, err := os.ReadDir(globalDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		name := entry.Name()
		switch {
		case strings.HasPrefix(name, "cmd.approve."):
			itemID := strings.TrimPrefix(name, "cmd.approve.")
			os.Remove(filepath.Join(globalDir, name))
			if err := d.queue.Approve(itemID); err != nil {
				d.log.Warn("stale approve failed", "item_id", itemID, "error", err)
			} else {
				d.log.Info("processed stale approve", "item_id", itemID)
				d.state.MarkDirty()
			}
		case strings.HasPrefix(name, "cmd.skip."):
			itemID := strings.TrimPrefix(name, "cmd.skip.")
			os.Remove(filepath.Join(globalDir, name))
			if err := d.queue.Skip(itemID); err != nil {
				d.log.Warn("stale skip failed", "item_id", itemID, "error", err)
			} else {
				d.state.MarkDirty()
			}
		case strings.HasPrefix(name, "cmd.stop."):
			// Remove stale repo stop commands
			os.Remove(filepath.Join(globalDir, name))
		case name == "cmd.reorder":
			os.Remove(filepath.Join(globalDir, name))
		}
	}
}

func (d *MainDaemon) processCommands(globalDir string) {
	// Stop all — cancel workers and shut down the daemon
	stopPath := filepath.Join(globalDir, "cmd.stop")
	if _, err := os.Stat(stopPath); err == nil {
		d.log.Info("received stop command")
		os.Remove(stopPath)
		d.pool.CancelAll()
		d.cancel()
		return
	}

	// Process approve/skip commands
	entries, err := os.ReadDir(globalDir)
	if err != nil {
		d.log.Warn("read global dir failed", "error", err)
		return
	}
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, "cmd.") {
			d.log.Debug("processing command file", "name", name)
		}
		switch {
		case strings.HasPrefix(name, "cmd.approve."):
			itemID := strings.TrimPrefix(name, "cmd.approve.")
			os.Remove(filepath.Join(globalDir, name))
			if err := d.queue.Approve(itemID); err != nil {
				d.log.Warn("approve failed", "item_id", itemID, "error", err)
			} else {
				d.log.Info("item approved", "item_id", itemID)
				d.state.MarkDirty()
			}
		case strings.HasPrefix(name, "cmd.skip."):
			itemID := strings.TrimPrefix(name, "cmd.skip.")
			os.Remove(filepath.Join(globalDir, name))
			if err := d.queue.Skip(itemID); err != nil {
				d.log.Warn("skip failed", "item_id", itemID, "error", err)
			}
		case strings.HasPrefix(name, "cmd.stop."):
			repoKey := strings.TrimPrefix(name, "cmd.stop.")
			os.Remove(filepath.Join(globalDir, name))
			d.cancelRepoTasks(repoKey)
		case name == "cmd.reorder":
			path := filepath.Join(globalDir, name)
			data, err := os.ReadFile(path)
			os.Remove(path)
			if err != nil {
				break
			}
			var priorities map[string]int
			if err := json.Unmarshal(data, &priorities); err != nil {
				d.log.Error("parse reorder", "error", err)
				break
			}
			for id, prio := range priorities {
				if err := d.queue.Reorder(id, prio); err != nil {
					d.log.Warn("reorder failed", "item_id", id, "error", err)
				}
			}
		}
	}
}

func (d *MainDaemon) cancelRepoTasks(repoKey string) {
	for _, item := range d.queue.All() {
		if strings.Contains(item.RepoURL, repoKey) || strings.Contains(item.RepoPath, repoKey) {
			for _, t := range item.Tasks {
				_ = d.pool.Cancel(t.ID)
			}
		}
	}
}
