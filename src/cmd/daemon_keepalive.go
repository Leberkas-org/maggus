package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sync/atomic"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/leberkas-org/maggus/internal/filewatcher"
	"github.com/leberkas-org/maggus/internal/gitbranch"
	"github.com/leberkas-org/maggus/internal/gitrecover"
	"github.com/leberkas-org/maggus/internal/gitutil"
	"github.com/leberkas-org/maggus/internal/globalconfig"
	"github.com/leberkas-org/maggus/internal/parser"
	"github.com/leberkas-org/maggus/internal/runlog"
	"github.com/leberkas-org/maggus/internal/stores"
	"github.com/leberkas-org/maggus/internal/usage"
)

// errStopAfterTask is returned by runOneDaemonCycle when the stop-after-task
// signal was consumed during the cycle. The daemon loop treats this as a clean
// exit request rather than an error.
var errStopAfterTask = errors.New("stop-after-task")

// runDaemonLoop runs the daemon work loop with keep-alive behaviour.
// When no work is found, it watches for feature/bug file changes and retries.
// It exits cleanly when the context is cancelled (signal received).
func runDaemonLoop(cmd printer, wc *runLoopConfig) error {
	dir := wc.dir

	if pidErr := writeDaemonPID(dir, os.Getpid()); pidErr != nil {
		cmd.Printf("Warning: could not write daemon PID: %v\n", pidErr)
	}
	defer removeDaemonPID(dir)
	defer removeDaemonStopFile(dir)

	// Signal handling — shared across all cycles.
	sigCtx, sigStop := signal.NotifyContext(context.Background(), shutdownSignals...)
	defer sigStop()

	workCtx, workCancel := context.WithCancel(context.Background())
	defer workCancel()

	go func() {
		<-sigCtx.Done()
		sigStop()
		workCancel()
	}()

	// Watch for stop signal file (used on Windows where OS signals cannot
	// reach a detached daemon process; harmless no-op on Unix).
	removeDaemonStopFile(dir)    // clean up leftover from previous run
	removeStopAfterTaskFile(dir) // clean up leftover stop-after-task signal from previous run
	go func() {
		stopFile := daemonStopFilePath(dir)
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-workCtx.Done():
				return
			case <-ticker.C:
				if _, err := os.Stat(stopFile); err == nil {
					os.Remove(stopFile)
					workCancel()
					return
				}
			}
		}
	}()

	// Open structured run log (shared across cycles).
	runLogger, logErr := runlog.Open(dir, wc.cfg.LogMaxFiles())
	if logErr != nil {
		cmd.Printf("Warning: could not open run log: %v\n", logErr)
	}
	defer func() { _ = runLogger.Close() }()
	defer runlog.RemoveSnapshot(dir)

	// Create filesystem watcher once and reuse across all wait cycles.
	fw, fwErr := filewatcher.New(dir, nil, 500*time.Millisecond)
	if fwErr != nil {
		cmd.Printf("Warning: could not create file watcher: %v\n", fwErr)
	}
	defer func() {
		if fw != nil {
			fw.Close()
		}
	}()

	for {
		// Check for signal before each cycle.
		select {
		case <-workCtx.Done():
			return nil
		default:
		}

		hadWork, err := runOneDaemonCycle(cmd, wc, dir, runLogger, workCtx)
		if errors.Is(err, errStopAfterTask) {
			return nil
		}
		if err != nil {
			runLogger.Info(fmt.Sprintf("work cycle error: %v", err))
		}

		// If work was done, immediately check for more work.
		if hadWork {
			continue
		}

		// No work found — exit immediately if stop-after-task was requested.
		if _, err := os.Stat(daemonStopAfterTaskFilePath(dir)); err == nil {
			removeStopAfterTaskFile(dir)
			return nil
		}

		// No work found — enter wait state.
		runLogger.Info("no work found, watching for changes")

		wakeReason, wakePath := waitForChanges(fw, workCtx, dir)
		switch wakeReason {
		case wakeSignal:
			return nil
		case wakeStopAfterTask:
			removeStopAfterTaskFile(dir)
			return nil
		case wakeDispatch:
			runLogger.Info("dispatch sentinel detected, restarting cycle")
		case wakeFileChange:
			runLogger.Info(fmt.Sprintf("file change detected: %s", wakePath))
		}
	}
}

// wakeReason describes why the daemon woke from the wait state.
type wakeReason int

const (
	wakeSignal        wakeReason = iota // shutdown signal received
	wakeFileChange                      // file change detected
	wakeStopAfterTask                   // stop-after-task sentinel file detected
	wakeDispatch                        // dispatch sentinel file detected
)

// daemonIdlePollInterval is the maximum time the daemon will wait idle before
// re-checking for work, providing a fallback for missed fsnotify events.
const daemonIdlePollInterval = 30 * time.Second

// waitForChanges blocks until a file change, context cancellation, or
// stop-after-task sentinel file detection.
// It uses the provided filewatcher (which may be nil if creation failed).
// Returns the reason for waking and the path of the changed file (if applicable).
func waitForChanges(fw *filewatcher.Watcher, ctx context.Context, dir string) (wakeReason, string) {
	stopAfterTaskTicker := time.NewTicker(500 * time.Millisecond)
	defer stopAfterTaskTicker.Stop()

	if fw == nil {
		// No watcher available — block on context, stop-after-task, or dispatch only.
		for {
			select {
			case <-ctx.Done():
				return wakeSignal, ""
			case <-stopAfterTaskTicker.C:
				if _, err := os.Stat(daemonStopAfterTaskFilePath(dir)); err == nil {
					return wakeStopAfterTask, ""
				}
				if sentinels := globDispatchSentinels(dir); len(sentinels) > 0 {
					return wakeDispatch, ""
				}
			}
		}
	}

	type fileEvent struct {
		path string
	}

	wakeCh := make(chan fileEvent, 1)
	fw.SetSend(func(msg any) {
		if m, ok := msg.(filewatcher.UpdateMsg); ok {
			path := m.Path
			if path == "" {
				path = filepath.Join(".maggus", "features")
			}
			select {
			case wakeCh <- fileEvent{path: path}:
			default:
			}
		}
	})
	defer fw.SetSend(nil)

	for {
		select {
		case <-ctx.Done():
			return wakeSignal, ""
		case evt := <-wakeCh:
			return wakeFileChange, evt.path
		case <-stopAfterTaskTicker.C:
			if _, err := os.Stat(daemonStopAfterTaskFilePath(dir)); err == nil {
				return wakeStopAfterTask, ""
			}
			if sentinels := globDispatchSentinels(dir); len(sentinels) > 0 {
				return wakeDispatch, ""
			}
		case <-time.After(daemonIdlePollInterval):
			return wakeFileChange, ""
		}
	}
}

// runOneDaemonCycle runs a single iteration of the daemon work loop.
// Returns true if work was found and executed, false if no work was available.
func runOneDaemonCycle(cmd printer, wc *runLoopConfig, dir string, runLogger *runlog.Logger, workCtx context.Context) (bool, error) {
	// Prune stale worker entries (done/failed/blocked older than 5 min).
	_ = runlog.PruneStaleWorkerEntries(dir, 5*time.Minute)

	featureStore := stores.NewFileFeatureStore(dir)
	bugStore := stores.NewFileBugStore(dir)

	// Recovery: detect and fix any dirty state left by a previous interrupted run.
	// Errors are warnings only — the normal work cycle is attempted regardless.
	recoveryLogs, recoveryErr := gitrecover.RecoverDirtyState(dir, wc.cfg, featureStore, bugStore)
	for _, msg := range recoveryLogs {
		cmd.Println(msg)
	}
	if recoveryErr != nil {
		cmd.Printf("Warning: recovery error: %v\n", recoveryErr)
	}

	// Parse tasks and check for work.
	setup, err := initIteration(cmd, dir, wc.modelDisplay, 0, featureStore, bugStore)
	if err != nil {
		return false, err
	}
	if setup == nil {
		return false, nil
	}

	// Build approved plans with approval filtering.
	featureGroups, fgErr := buildApprovedPlans(dir, wc.cfg, featureStore, bugStore)
	if fgErr != nil {
		return false, fmt.Errorf("build approved plans: %w", fgErr)
	}

	// Remove plans with no workable tasks.
	var workableGroups []parser.Plan
	for _, g := range featureGroups {
		if countWorkable(g.Tasks) > 0 {
			workableGroups = append(workableGroups, g)
		}
	}
	featureGroups = workableGroups

	if len(featureGroups) == 0 {
		return false, nil
	}

	// Work is available — run it.
	branchTask := firstWorkableTask(featureGroups)
	if branchTask == nil {
		branchTask = setup.next
	}

	// Set up the plan-level integration branch. The orchestrator handles
	// task-level branching and worktree creation for parallel tasks internally.
	repoDir := dir
	planBranch := ""
	branchMsg := ""
	if wc.cfg.Git.IsAutoBranchEnabled() {
		var pbErr error
		planBranch, branchMsg, pbErr = gitbranch.EnsurePlanBranch(repoDir, branchTask.ID, wc.cfg.Git.ProtectedBranchList())
		if pbErr != nil {
			return false, fmt.Errorf("setup plan branch: %w", pbErr)
		}
	} else {
		branchMsg = "Auto-branch disabled, staying on current branch"
	}

	// Write an early "Preparing" snapshot so the status TUI shows which
	// feature/task the daemon is picking up immediately, before the work
	// goroutine starts and sends the full IterationStartMsg.
	if len(featureGroups) > 0 {
		if ft := firstWorkableTask(featureGroups); ft != nil {
			fg := featureGroups[0]
			earlySnap := runlog.StateSnapshot{
				MaggusID:      fg.MaggusID,
				TaskID:        ft.ID,
				TaskTitle:     ft.Title,
				ItemTitle:     parser.ParseFileTitle(fg.File),
				Status:        "Preparing",
				RunStartedAt:  setup.startTime.UTC().Format(time.RFC3339),
				TaskStartedAt: setup.startTime.UTC().Format(time.RFC3339),
			}
			_ = runlog.WriteSnapshot(dir, earlySnap)
		}
	}

	// Create tea.Program with nullTUIModel for this cycle.
	dm := nullTUIModel{
		snapshotDir:  dir,
		runStartedAt: setup.startTime,
	}
	dm.SetOnToolUse(func(taskID, toolType string, params map[string]string) {
		runLogger.ToolUse(taskID, toolType, params)
	})
	dm.SetOnOutput(func(taskID, text string) {
		runLogger.Output(taskID, text)
	})
	repoURL := gitutil.RepoURL(dir)
	dm.SetOnTaskUsage(func(tu TaskUsage) {
		_ = usage.Append([]usage.Record{{
			RunID:                    "",
			Repository:               repoURL,
			ItemID:                   tu.ItemID,
			ItemShort:                tu.ItemShort,
			ItemTitle:                tu.ItemTitle,
			TaskShort:                tu.TaskShort,
			Model:                    wc.modelDisplay,
			Agent:                    wc.activeAgent.Name(),
			InputTokens:              tu.InputTokens,
			OutputTokens:             tu.OutputTokens,
			CacheCreationInputTokens: tu.CacheCreationInputTokens,
			CacheReadInputTokens:     tu.CacheReadInputTokens,
			CostUSD:                  tu.CostUSD,
			ModelUsage:               tu.ModelUsage,
			StartTime:                tu.StartTime,
			EndTime:                  tu.EndTime,
		}})
		totalTokens := int64(tu.InputTokens + tu.OutputTokens + tu.CacheCreationInputTokens + tu.CacheReadInputTokens)
		if totalTokens > 0 {
			_ = globalconfig.IncrementMetrics(globalconfig.Metrics{TokensUsed: totalTokens})
		}
		modelUsage := make(map[string]runlog.ModelTokensEntry, len(tu.ModelUsage))
		for name, mt := range tu.ModelUsage {
			modelUsage[name] = runlog.ModelTokensEntry{
				InputTokens:              mt.InputTokens,
				OutputTokens:             mt.OutputTokens,
				CacheCreationInputTokens: mt.CacheCreationInputTokens,
				CacheReadInputTokens:     mt.CacheReadInputTokens,
				CostUSD:                  mt.CostUSD,
			}
		}
		runLogger.TaskUsage(runlog.TaskUsageData{
			TaskID:                   tu.TaskShort,
			InputTokens:              tu.InputTokens,
			OutputTokens:             tu.OutputTokens,
			CacheCreationInputTokens: tu.CacheCreationInputTokens,
			CacheReadInputTokens:     tu.CacheReadInputTokens,
			CostUSD:                  tu.CostUSD,
			ModelUsage:               modelUsage,
		})
	})

	var p *tea.Program
	pipeR, pipeW, pipeErr := os.Pipe()
	if pipeErr == nil {
		defer pipeW.Close()
		defer pipeR.Close()
		p = tea.NewProgram(dm, tea.WithoutRenderer(), tea.WithInput(pipeR))
	} else {
		p = tea.NewProgram(dm, tea.WithoutRenderer())
	}

	stopFlagAtomic := &atomic.Bool{}
	stopAtTaskIDAtomic := &atomic.Value{}

	// Poll for the stop-after-task sentinel file. When found, set stopFlagAtomic so
	// the work loop stops between tasks rather than cancelling the active invocation.
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		stopAfterTaskFile := daemonStopAfterTaskFilePath(dir)
		for {
			select {
			case <-workCtx.Done():
				return
			case <-ticker.C:
				if _, err := os.Stat(stopAfterTaskFile); err == nil {
					removeStopAfterTaskFile(dir)
					stopFlagAtomic.Store(true)
					return
				}
			}
		}
	}()

	orchCfg := OrchestratorConfig{
		Ctx:            workCtx,
		Program:        p,
		Agent:          wc.activeAgent,
		Model:          wc.resolvedModel,
		SessionPersist: wc.sessionPersistence,
		ValidIncludes:  wc.validIncludes,
		FeatureStore:   featureStore,
		BugStore:       bugStore,
		Logger:         runLogger,
		Notifier:       wc.notifier,
		Dir:            dir,
		RepoDir:        repoDir,
		PlanBranch:     planBranch,
		OnComplete:     wc.cfg.OnComplete,
		Hooks:          wc.cfg.Hooks,
		FeatureGroups:  featureGroups,
		AutoContinue:   wc.cfg.IsAutoContinueEnabled(),
		StopFlag:       stopFlagAtomic,
		StopAtTaskID:   stopAtTaskIDAtomic,
		StartTime:      setup.startTime,
		ModelDisplay:   wc.modelDisplay,
		StartHash:      captureStartHash(dir),
		ActiveAgentNm:  wc.activeAgent.Name(),
		IncludeWarns:   wc.includeWarnings,
		BranchMsg:      branchMsg,
	}

	orch := NewOrchestrator(orchCfg)
	go func() {
		orch.Run()
		p.Send(QuitMsg{})
	}()

	_, tuiErr := p.Run()
	if tuiErr != nil {
		return true, fmt.Errorf("TUI error: %w", tuiErr)
	}

	if stopFlagAtomic.Load() {
		return true, errStopAfterTask
	}

	return true, nil
}
