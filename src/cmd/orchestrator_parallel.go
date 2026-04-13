package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/leberkas-org/maggus/internal/gitbranch"
	"github.com/leberkas-org/maggus/internal/gitworktree"
	"github.com/leberkas-org/maggus/internal/globalconfig"
	"github.com/leberkas-org/maggus/internal/parser"
	"github.com/leberkas-org/maggus/internal/runlog"
	"golang.org/x/sync/errgroup"
)

// runDispatchRequests checks for pending dispatch sentinel files and runs each
// requested task in an isolated git worktree. Sentinels are atomically consumed
// (removed) before the corresponding task starts to prevent duplicate execution.
//
// Dispatched tasks run as background goroutines tracked via o.dispatchWG, so the
// normal sequential/parallel queue continues without blocking. The caller (Run)
// waits on dispatchWG before returning so dispatched tasks are never orphaned.
//
// Edge cases handled:
//   - Sentinel with unknown/malformed task ID: removed, InfoMsg sent.
//   - Task already complete or blocked: sentinel removed, InfoMsg sent.
//   - Task already registered as a running worker: sentinel removed, InfoMsg sent.
//   - Multiple sentinels for the same task: second os.Remove fails → goroutine not launched.
func (o *Orchestrator) runDispatchRequests() {
	sentinels := globDispatchSentinels(o.cfg.RepoDir)
	if len(sentinels) == 0 {
		return
	}

	// Load all plans (bugs + features) to locate each dispatched task and its group.
	allPlans, err := loadAllPlans(o.cfg.FeatureStore, o.cfg.BugStore)
	if err != nil {
		return
	}

	for _, sentinel := range sentinels {
		taskID := taskIDFromDispatchSentinel(sentinel)
		if taskID == "" {
			_ = os.Remove(sentinel)
			continue
		}

		// Atomically consume the sentinel. If two goroutines race on the same
		// file, only one Remove succeeds; the other sees an error and skips.
		if err := os.Remove(sentinel); err != nil {
			continue
		}

		// Find the task and its plan across all loaded plans.
		var foundTask *parser.Task
		var foundPlan *parser.Plan
		for pi := range allPlans {
			for ti := range allPlans[pi].Tasks {
				if allPlans[pi].Tasks[ti].ID == taskID {
					foundTask = &allPlans[pi].Tasks[ti]
					foundPlan = &allPlans[pi]
					break
				}
			}
			if foundTask != nil {
				break
			}
		}

		if foundTask == nil {
			o.cfg.Program.Send(InfoMsg{Text: fmt.Sprintf("⚠ Dispatch: task %s not found", taskID)})
			continue
		}
		if foundTask.IsComplete() {
			o.cfg.Program.Send(InfoMsg{Text: fmt.Sprintf("⊘ Dispatch: task %s already complete", taskID)})
			continue
		}
		if foundTask.IsBlocked() {
			o.cfg.Program.Send(InfoMsg{Text: fmt.Sprintf("⊘ Dispatch: task %s is blocked", taskID)})
			continue
		}

		// Skip if already registered as a running dispatch worker.
		o.mu.Lock()
		alreadyRunning := o.hasWorkerMaps() && o.workerStatuses[taskID] == "working"
		o.mu.Unlock()
		if alreadyRunning {
			o.cfg.Program.Send(InfoMsg{Text: fmt.Sprintf("⊘ Dispatch: task %s already running", taskID)})
			continue
		}

		// Register the worker so the TUI can show its status immediately.
		o.mu.Lock()
		o.ensureWorkerMaps()
		o.registerWorker(taskID, foundTask.Title)
		o.updateWorkersIndex(o.buildWorkerEntries())
		o.mu.Unlock()

		// Launch the task in an isolated worktree as a background goroutine.
		task := *foundTask
		plan := *foundPlan
		o.dispatchWG.Add(1)
		go func() {
			defer o.dispatchWG.Done()
			o.runWorktreeTask(&plan, task)
		}()
	}
}

// classifyWorkable splits the given tasks into parallel-workable and
// sequential-workable lists using the same rules as the parallel orchestrator:
//   - Complete, blocked, and previously-failed tasks are skipped.
//   - Tasks whose predecessors have not yet completed are skipped.
//   - Tasks with Parallel=true go into the parallel list.
//   - Tasks with Parallel=false go into the sequential list.
func (o *Orchestrator) classifyWorkable(tasks []parser.Task) (parallel, sequential []parser.Task) {
	o.mu.Lock()
	defer o.mu.Unlock()
	for _, t := range tasks {
		if t.IsComplete() || t.IsBlocked() {
			continue
		}
		if o.failedIDs[t.ID] {
			continue
		}
		if !o.predecessorsComplete(t) {
			continue
		}
		if t.Parallel {
			parallel = append(parallel, t)
		} else {
			sequential = append(sequential, t)
		}
	}
	return
}

// predecessorsComplete reports whether all of t's predecessor task IDs are in
// o.completedIDs. Caller must hold o.mu.
func (o *Orchestrator) predecessorsComplete(t parser.Task) bool {
	for _, predID := range t.Predecessors {
		if !o.completedIDs[predID] {
			return false
		}
	}
	return true
}

// runParallelBatch launches all tasks concurrently, each in its own isolated
// git worktree. It blocks until all goroutines finish and returns an aggregate
// result. Workers update o.completedIDs and o.failedIDs as they finish so
// that subsequent calls to classifyWorkable reflect updated predecessor state.
func (o *Orchestrator) runParallelBatch(group *parser.Plan, tasks []parser.Task) groupTasksResult {
	var result groupTasksResult
	var resultMu sync.Mutex

	cfg := o.cfg

	// Register all workers and write the initial workers index so the TUI
	// can render the split pane before any worker completes.
	o.mu.Lock()
	o.ensureWorkerMaps()
	for _, t := range tasks {
		o.registerWorker(t.ID, t.Title)
	}
	o.updateWorkersIndex(o.buildWorkerEntries())
	o.mu.Unlock()

	cfg.Program.Send(InfoMsg{Text: fmt.Sprintf("⚡ Launching %d parallel tasks", len(tasks))})

	g, _ := errgroup.WithContext(cfg.Ctx)
	for _, task := range tasks {
		g.Go(func() error {
			wr := o.runWorktreeTask(group, task)
			resultMu.Lock()
			result.completed += wr.completed
			result.failed = append(result.failed, wr.failed...)
			result.warnings = append(result.warnings, wr.warnings...)
			if wr.stopped {
				result.stopped = true
				result.stopReason = wr.stopReason
			}
			resultMu.Unlock()
			return nil
		})
	}
	_ = g.Wait()

	if cfg.Ctx.Err() != nil {
		result.stopped = true
		result.stopReason = StopReasonInterrupted
	}
	return result
}

// runWorktreeTask runs a single parallel task in an isolated git worktree.
// It creates the task branch and worktree before invoking the worker, then
// removes them after the worker returns (the worker merges the task branch
// back into PlanBranch before returning). On completion or failure,
// o.completedIDs or o.failedIDs is updated for predecessor tracking.
func (o *Orchestrator) runWorktreeTask(group *parser.Plan, task parser.Task) groupTasksResult {
	var result groupTasksResult
	cfg := o.cfg

	// Allocate a unique iteration number for this worker's prompt.
	o.mu.Lock()
	o.iteration++
	iter := o.iteration
	o.mu.Unlock()

	// Open a per-task logger so parallel workers don't share a log handle.
	workerLogger, logErr := runlog.Open(cfg.RepoDir, 0)
	if logErr != nil {
		workerLogger = cfg.Logger
	}
	defer func() {
		if logErr == nil {
			_ = workerLogger.Close()
		}
	}()
	workerLogger.SetCurrentMaggusID(group.MaggusID)

	cfg.Program.Send(InfoMsg{Text: fmt.Sprintf("▶ %s: Starting in worktree", task.ID)})

	// Create per-worker snapshot writer for TUI split view.
	// The writer receives agent events and writes to .maggus/runs/state-{taskID}.json.
	wsw := o.newWorkerSnapshotWriterForTask(task, group)

	// Create the task branch and an isolated worktree for this worker.
	taskBranch := gitbranch.BranchName(task.ID)
	worktreePath := filepath.Join(cfg.RepoDir, ".maggus", "worktrees", task.ID)

	if err := gitbranch.CreateBranchFrom(cfg.RepoDir, taskBranch, cfg.PlanBranch); err != nil {
		o.mu.Lock()
		o.failedIDs[task.ID] = true
		o.mu.Unlock()
		o.markWorkerFailed(task.ID, wsw)
		result.failed = append(result.failed, failedTask{
			ID:     task.ID,
			Title:  task.Title,
			Reason: fmt.Sprintf("create branch: %v", err),
		})
		return result
	}

	if err := gitworktree.CreateWorktree(cfg.RepoDir, worktreePath, taskBranch); err != nil {
		_ = gitbranch.DeleteBranch(cfg.RepoDir, taskBranch)
		o.mu.Lock()
		o.failedIDs[task.ID] = true
		o.mu.Unlock()
		o.markWorkerFailed(task.ID, wsw)
		result.failed = append(result.failed, failedTask{
			ID:     task.ID,
			Title:  task.Title,
			Reason: fmt.Sprintf("create worktree: %v", err),
		})
		return result
	}

	wr := RunTaskWorker(WorkerConfig{
		Ctx:            cfg.Ctx,
		Task:           task,
		PlanFile:       group.File,
		MaggusID:       group.MaggusID,
		PlanTitle:      group.Title,
		Agent:          cfg.Agent,
		Model:          cfg.Model,
		SessionPersist: cfg.SessionPersist,
		ValidIncludes:  cfg.ValidIncludes,
		Iteration:      iter,
		RepoDir:        cfg.RepoDir,
		WorkDir:        worktreePath,
		PlanBranch:     cfg.PlanBranch,
		MergeMu:        &o.mu,
		Logger:         workerLogger,
		AgentSender:    wsw,
		EventSender:    cfg.Program,
		Notifier:       cfg.Notifier,
	})

	// Remove the worktree after the worker returns. On the success path the worker
	// already merged and deleted the task branch. On failure paths below we delete
	// the branch explicitly to avoid orphaned refs from the previous failed cycle.
	if err := gitworktree.RemoveWorktree(cfg.RepoDir, worktreePath); err != nil {
		cfg.Program.Send(InfoMsg{Text: fmt.Sprintf("⚠ %s: worktree cleanup failed: %v", task.ID, err)})
	}

	if wr.StopReason == StopReasonInterrupted {
		_ = gitbranch.DeleteBranch(cfg.RepoDir, taskBranch)
		o.mu.Lock()
		o.failedIDs[task.ID] = true
		o.mu.Unlock()
		o.markWorkerFailed(task.ID, wsw)
		result.stopped = true
		result.stopReason = StopReasonInterrupted
		return result
	}

	if wr.Failed != nil {
		_ = gitbranch.DeleteBranch(cfg.RepoDir, taskBranch)
		o.mu.Lock()
		o.failedIDs[task.ID] = true
		o.mu.Unlock()
		o.markWorkerFailed(task.ID, wsw)
		result.failed = append(result.failed, *wr.Failed)
		_ = globalconfig.IncrementMetrics(globalconfig.Metrics{TasksFailed: 1})
		return result
	}

	if wr.Blocked {
		if wr.Warning != "" {
			result.warnings = append(result.warnings, wr.Warning)
		}
		o.markWorkerBlocked(task.ID, wsw)
		return result
	}

	if wr.Warning != "" {
		result.warnings = append(result.warnings, wr.Warning)
	}

	o.mu.Lock()
	o.completedIDs[task.ID] = true
	o.mu.Unlock()

	o.markWorkerDone(task.ID, wsw)
	cfg.Program.Send(InfoMsg{Text: fmt.Sprintf("✓ %s: Completed and merged into %s", task.ID, cfg.PlanBranch)})
	_ = globalconfig.IncrementMetrics(globalconfig.Metrics{TasksCompleted: 1})

	if wr.Completed {
		result.completed = 1
	}
	return result
}
