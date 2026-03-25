package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sync/atomic"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/leberkas-org/maggus/internal/filewatcher"
	"github.com/leberkas-org/maggus/internal/gitsync"
	"github.com/leberkas-org/maggus/internal/globalconfig"
	"github.com/leberkas-org/maggus/internal/runlog"
	"github.com/leberkas-org/maggus/internal/runner"
	"github.com/leberkas-org/maggus/internal/usage"
)

// runDaemonLoop runs the daemon work loop with keep-alive behaviour.
// When no work is found, it watches for feature/bug file changes and retries.
// It exits cleanly when the context is cancelled (signal received).
func runDaemonLoop(cmd printer, wc *workConfig) error {
	dir := wc.dir

	// Write daemon PID so 'maggus stop' can find this process.
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
	removeDaemonStopFile(dir) // clean up leftover from previous run
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

	// Initialise sync functions (once for the whole daemon lifetime).
	runner.InitSyncFuncs(gitsync.Pull, gitsync.PullRebase, gitsync.ForcePull)

	runID := daemonRunIDFlag

	// Open structured run log (shared across cycles).
	runLogger, logErr := runlog.Open(runID, dir)
	if logErr != nil {
		cmd.Printf("Warning: could not open run log: %v\n", logErr)
	}
	defer func() { _ = runLogger.Close() }()
	defer runlog.RemoveSnapshot(dir, runID)

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

		hadWork, err := runOneDaemonCycle(cmd, wc, dir, runID, runLogger, workCtx)
		if err != nil {
			runLogger.Info(fmt.Sprintf("work cycle error: %v", err))
		}

		// If work was done, immediately check for more work.
		if hadWork {
			continue
		}

		// No work found — enter wait state.
		runLogger.Info("no work found, watching for changes")

		wakeReason, wakePath := waitForChanges(fw, workCtx)
		switch wakeReason {
		case wakeSignal:
			return nil
		case wakeFileChange:
			runLogger.Info(fmt.Sprintf("file change detected: %s", wakePath))
		}
	}
}

// wakeReason describes why the daemon woke from the wait state.
type wakeReason int

const (
	wakeSignal     wakeReason = iota // shutdown signal received
	wakeFileChange                   // file change detected
)

// waitForChanges blocks until a file change or context cancellation.
// It uses the provided filewatcher (which may be nil if creation failed).
// Returns the reason for waking and the path of the changed file (if applicable).
func waitForChanges(fw *filewatcher.Watcher, ctx context.Context) (wakeReason, string) {
	if fw == nil {
		// No watcher available — block on context only.
		<-ctx.Done()
		return wakeSignal, ""
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

	select {
	case <-ctx.Done():
		return wakeSignal, ""
	case evt := <-wakeCh:
		return wakeFileChange, evt.path
	}
}

// runOneDaemonCycle runs a single iteration of the daemon work loop.
// Returns true if work was found and executed, false if no work was available.
func runOneDaemonCycle(cmd printer, wc *workConfig, dir, runID string, runLogger *runlog.Logger, workCtx context.Context) (bool, error) {
	// Parse tasks and check for work.
	setup, err := initIteration(cmd, dir, wc.modelDisplay, 0)
	if err != nil {
		return false, err
	}
	if setup == nil {
		return false, nil
	}

	// Build feature groups with approval filtering.
	featureGroups, fgErr := buildApprovedFeatureGroups(dir, wc.cfg)
	if fgErr != nil {
		return false, fmt.Errorf("build feature groups: %w", fgErr)
	}

	// Remove groups with no workable tasks.
	var workableGroups []featureGroup
	for _, g := range featureGroups {
		if countWorkable(g.tasks) > 0 {
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

	repoDir := dir
	workDir := dir

	branchMsg, brErr := setupBranch(wc.useWorktree, repoDir, branchTask, runID, wc.cfg.Git)
	if brErr != nil {
		return false, fmt.Errorf("setup branch: %w", brErr)
	}

	// Create tea.Program with nullTUIModel for this cycle.
	dm := nullTUIModel{
		snapshotDir:   dir,
		snapshotRunID: runID,
		runStartedAt:  setup.startTime,
	}
	dm.SetOnToolUse(func(taskID, toolType, description string) {
		runLogger.ToolUse(taskID, toolType, description)
	})
	dm.SetOnOutput(func(taskID, text string) {
		runLogger.Output(taskID, text)
	})
	dm.SetOnTaskUsage(func(tu runner.TaskUsage) {
		featureRel := tu.FeatureFile
		if rel, err := filepath.Rel(dir, tu.FeatureFile); err == nil {
			featureRel = rel
		}
		_ = usage.Append(dir, []usage.Record{{
			RunID:                    runID,
			TaskID:                   tu.TaskID,
			TaskTitle:                tu.TaskTitle,
			FeatureFile:              featureRel,
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

	tc := taskContext{
		workCtx:       workCtx,
		p:             p,
		activeAgent:   wc.activeAgent,
		resolvedModel: wc.resolvedModel,
		notifier:      wc.notifier,
		validIncludes: wc.validIncludes,
		useWorktree:   wc.useWorktree,
		repoDir:       repoDir,
		workDir:       workDir,
		runID:         runID,
		onComplete:    wc.cfg.OnComplete,
		hooks:         wc.cfg.Hooks,
		logger:        runLogger,
	}

	runWorkGoroutine(workLoopParams{
		tc:            tc,
		tasks:         setup.tasks,
		featureGroups: featureGroups,
		count:         0,
		autoContinue:  wc.cfg.IsAutoContinueEnabled(),
		runID:         runID,
		startTime:     setup.startTime,
		p:             p,
		stopFlag:      stopFlagAtomic,
		stopAtTaskID:  stopAtTaskIDAtomic,
		branchMsg:     branchMsg,
		activeAgentNm: wc.activeAgent.Name(),
		startHash:     captureStartHash(workDir),
		modelDisplay:  wc.modelDisplay,
		dir:           dir,
	})

	_, tuiErr := p.Run()
	if tuiErr != nil {
		return true, fmt.Errorf("TUI error: %w", tuiErr)
	}

	return true, nil
}
