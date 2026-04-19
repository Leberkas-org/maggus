package daemon

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/leberkas-org/maggus/internal/agent"
	"github.com/leberkas-org/maggus/internal/config"
	"github.com/leberkas-org/maggus/internal/git"
	"github.com/leberkas-org/maggus/internal/ipc"
)

type MainDaemon struct {
	cfg        config.Config
	queue      *TaskQueue
	state      *State
	dispatcher *Dispatcher
	pool       *WorkerPool
	gitOps     git.Operations
	agents     *agent.Registry
	importers  map[string]*Importer
}

func New(cfg config.Config, gitOps git.Operations, agents *agent.Registry) *MainDaemon {
	queue := NewTaskQueue()
	pool := NewWorkerPool(cfg.MaxWorkers)

	globalDir, _ := config.GlobalDir()
	stateWriter := ipc.NewFileStateWriter(globalDir)
	state := NewState(stateWriter, queue)

	dispatcher := NewDispatcher(queue, pool, gitOps, agents, cfg, state)

	return &MainDaemon{
		cfg:        cfg,
		queue:      queue,
		state:      state,
		dispatcher: dispatcher,
		pool:       pool,
		gitOps:     gitOps,
		agents:     agents,
		importers:  make(map[string]*Importer),
	}
}

func (d *MainDaemon) Run(ctx context.Context) error {
	globalDir, err := config.GlobalDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(globalDir, 0o755); err != nil {
		return err
	}

	// Write PID file
	pidPath := filepath.Join(globalDir, "daemon.pid")
	if err := os.WriteFile(pidPath, fmt.Appendf(nil, "%d", os.Getpid()), 0o644); err != nil {
		log.Printf("write pid: %v", err)
	}
	defer os.Remove(pidPath)

	// Initial scan for plan files
	for _, repo := range d.cfg.Repos {
		tasksDir := filepath.Join(repo.Path, ".maggus", "tasks")
		d.importers[repo.Path] = NewImporter(tasksDir)
		d.scanForPlans(repo)
	}

	_ = d.state.Flush()

	// Main loop
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			d.pool.CancelAll()
			return nil
		case <-ticker.C:
			d.scanAllRepos()
			d.processCommands(globalDir)
			d.dispatcher.Tick()
			_ = d.state.Flush()
		}
	}
}

func (d *MainDaemon) Stop() error {
	d.pool.CancelAll()
	return nil
}

func (d *MainDaemon) scanAllRepos() {
	for _, repo := range d.cfg.Repos {
		d.scanForPlans(repo)
	}
}

func (d *MainDaemon) scanForPlans(repo config.RepoEntry) {
	tasksDir := filepath.Join(repo.Path, ".maggus", "tasks")
	entries, err := os.ReadDir(tasksDir)
	if err != nil {
		return
	}

	imp := d.importers[repo.Path]
	if imp == nil {
		return
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		planPath := filepath.Join(tasksDir, entry.Name())
		item, err := imp.Import(planPath, d.gitOps.RepoURL(repo.Path), d.cfg.AutoApprove)
		if err != nil {
			log.Printf("import %s: %v", planPath, err)
			continue
		}
		item.RepoPath = repo.Path
		d.queue.Enqueue(item)
		log.Printf("imported plan: %s (%d tasks)", item.Title, len(item.Tasks))
	}
}

func (d *MainDaemon) processCommands(globalDir string) {
	// Stop all
	if _, err := os.Stat(filepath.Join(globalDir, "cmd.stop")); err == nil {
		os.Remove(filepath.Join(globalDir, "cmd.stop"))
		d.pool.CancelAll()
	}

	// Process approve/skip commands
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
				log.Printf("approve %s: %v", itemID, err)
			}
		case strings.HasPrefix(name, "cmd.skip."):
			itemID := strings.TrimPrefix(name, "cmd.skip.")
			os.Remove(filepath.Join(globalDir, name))
			if err := d.queue.Skip(itemID); err != nil {
				log.Printf("skip %s: %v", itemID, err)
			}
		}
	}
}
