package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/leberkas-org/maggus/internal/agent"
	"github.com/leberkas-org/maggus/internal/config"
	"github.com/leberkas-org/maggus/internal/gitbranch"
	"github.com/leberkas-org/maggus/internal/globalconfig"
	"github.com/leberkas-org/maggus/internal/gitworktree"
	"github.com/leberkas-org/maggus/internal/notify"
	"github.com/leberkas-org/maggus/internal/parser"
	"github.com/leberkas-org/maggus/internal/runlog"
	"github.com/leberkas-org/maggus/internal/stores"
	"golang.org/x/sync/errgroup"
)

// parallelOrchestrator manages the parallel work loop for a single plan.
type parallelOrchestrator struct {
	ctx            context.Context
	p              *tea.Program
	activeAgent    agent.Agent
	resolvedModel  string
	sessionPersist bool
	notifier       *notify.Notifier
	validIncludes  []string
	repoDir        string
	planBranch     string
	onComplete     config.OnCompleteConfig
	hooks          config.HooksConfig
	parentLogger   *runlog.Logger
	featureStore   stores.FeatureStore
	bugStore       stores.BugStore
	mu             sync.Mutex
	completedIDs   map[string]bool
	failedIDs      map[string]bool
	iteration      int
	runStartedAt   time.Time

	// Per-worker TUI state for split pane view.
	workerOrder     []string          // ordered list of worker task IDs
	workerStatuses  map[string]string // taskID → "working"/"done"/"failed"/"blocked"
	workerTitles    map[string]string // taskID → task title
	workerStartedAt map[string]string // taskID → RFC3339 start time
}

type parallelWorkResult struct {
	completed  int
	failed     []failedTask
	warnings   []string
	stopReason StopReason
}

// runParallelWorkGoroutine runs the parallel work loop in a goroutine, sending TUI events.
func runParallelWorkGoroutine(params runLoopParams, planBranch string) {
	go func() {
		defer func() { params.p.Send(QuitMsg{}) }()

		if params.activeAgentNm == "claude" {
			params.p.Send(InfoMsg{Text: "⚠ Using --dangerously-skip-permissions (Claude Code)"})
		}
		for _, w := range params.includeWarns {
			params.p.Send(InfoMsg{Text: w})
		}
		if params.branchMsg != "" {
			params.p.Send(InfoMsg{Text: params.branchMsg})
		}
		if params.syncInfoMsg != "" {
			params.p.Send(InfoMsg{Text: params.syncInfoMsg})
		}
		params.p.Send(InfoMsg{Text: "⚡ Parallel mode enabled"})

		if len(params.featureGroups) == 0 {
			params.p.Send(InfoMsg{Text: "No approved features available."})
			sp := params
			sp.count = 0
			summary := buildSummaryData(sp, 0, nil, StopReasonNoTasks, "no approved features", nil)
			params.tc.notifier.PlayRunComplete()
			params.p.Send(SummaryMsg{Data: summary})
			pushToRemote(params.p, params.tc.workDir, 0, summary.Branch)
			return
		}

		group := params.featureGroups[0]
		orch := &parallelOrchestrator{
			ctx: params.tc.workCtx, p: params.p,
			activeAgent: params.tc.activeAgent, resolvedModel: params.tc.resolvedModel,
			sessionPersist: params.tc.sessionPersistence, notifier: params.tc.notifier,
			validIncludes: params.tc.validIncludes, repoDir: params.tc.repoDir,
			planBranch: planBranch,
			onComplete: params.tc.onComplete, hooks: params.tc.hooks,
			parentLogger: params.tc.logger,
			featureStore: params.featureStore, bugStore: params.bugStore,
			completedIDs: make(map[string]bool),
			failedIDs:    make(map[string]bool),
			runStartedAt: params.startTime,
		}
		for _, t := range group.Tasks {
			if t.IsComplete() {
				orch.completedIDs[t.ID] = true
			}
		}

		result := orch.run(group)
		stopReason := result.stopReason
		if len(result.failed) > 0 && stopReason == StopReasonComplete {
			stopReason = StopReasonPartialComplete
		}
		if result.completed == 0 && stopReason == StopReasonComplete {
			if len(result.warnings) > 0 {
				stopReason = StopReasonError
			} else {
				stopReason = StopReasonNoTasks
			}
		}

		sp := params
		sp.count = result.completed + len(result.failed)
		summary := buildSummaryData(sp, result.completed, result.failed, stopReason, "", result.warnings)
		params.tc.notifier.PlayRunComplete()
		params.p.Send(SummaryMsg{Data: summary})
		pushToRemote(params.p, params.tc.workDir, result.completed, summary.Branch)
	}()
}

// run executes the parallel work loop for a single plan.
func (o *parallelOrchestrator) run(group parser.Plan) parallelWorkResult {
	var result parallelWorkResult
	defer cleanupWorkerSnapshots(o.repoDir)

	if maggusID, err := parser.EnsureMaggusID(group.File); err == nil {
		group.MaggusID = maggusID
	}
	o.parentLogger.SetCurrentMaggusID(group.MaggusID)
	o.parentLogger.FeatureStart(group.ID)

	for {
		if o.ctx.Err() != nil {
			result.stopReason = StopReasonInterrupted
			break
		}

		tasks, err := parser.ParseFile(group.File)
		if err != nil {
			o.p.Send(InfoMsg{Text: fmt.Sprintf("✗ re-parse error: %v", err)})
			break
		}

		par, seq := o.classifyWorkable(tasks)
		if len(par) == 0 && len(seq) == 0 {
			break
		}

		var batch parallelWorkResult
		if len(par) > 0 {
			batch = o.runParallelBatch(group, par)
		} else {
			batch = o.runSingleTask(group, seq[0], false)
		}
		result.completed += batch.completed
		result.failed = append(result.failed, batch.failed...)
		result.warnings = append(result.warnings, batch.warnings...)

		// Record failed task IDs so classifyWorkable skips them on subsequent iterations.
		if len(batch.failed) > 0 {
			o.mu.Lock()
			for _, f := range batch.failed {
				o.failedIDs[f.ID] = true
			}
			o.mu.Unlock()
		}

		if batch.stopReason == StopReasonInterrupted {
			result.stopReason = StopReasonInterrupted
			break
		}
	}

	o.parentLogger.FeatureComplete(group.ID)
	o.parentLogger.SetCurrentMaggusID("")
	return result
}

// classifyWorkable splits tasks into parallel-workable and sequential-workable lists.
func (o *parallelOrchestrator) classifyWorkable(tasks []parser.Task) (parallel, sequential []parser.Task) {
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

func (o *parallelOrchestrator) predecessorsComplete(t parser.Task) bool {
	for _, predID := range t.Predecessors {
		if !o.completedIDs[predID] {
			return false
		}
	}
	return true
}

// runParallelBatch launches all parallel-workable tasks concurrently using errgroup.
func (o *parallelOrchestrator) runParallelBatch(group parser.Plan, tasks []parser.Task) parallelWorkResult {
	var result parallelWorkResult
	var resultMu sync.Mutex

	// Register all workers and write initial index.
	o.mu.Lock()
	o.ensureWorkerMaps()
	for _, t := range tasks {
		o.registerWorker(t.ID, t.Title)
	}
	o.updateWorkersIndex(o.buildWorkerEntries())
	o.mu.Unlock()

	o.p.Send(InfoMsg{Text: fmt.Sprintf("⚡ Launching %d parallel tasks", len(tasks))})
	g, _ := errgroup.WithContext(o.ctx)

	for _, task := range tasks {
		g.Go(func() error {
			wr := o.runSingleTask(group, task, true)
			resultMu.Lock()
			result.completed += wr.completed
			result.failed = append(result.failed, wr.failed...)
			result.warnings = append(result.warnings, wr.warnings...)
			resultMu.Unlock()
			return nil
		})
	}
	_ = g.Wait()

	if o.ctx.Err() != nil {
		result.stopReason = StopReasonInterrupted
	}
	return result
}

// runSingleTask runs one task via the unified worker. When useWorktree is true,
// the task runs in its own worktree on its own branch (parallel mode). When false,
// it runs in the main repo directory sequentially.
func (o *parallelOrchestrator) runSingleTask(group parser.Plan, task parser.Task, useWorktree bool) parallelWorkResult {
	var result parallelWorkResult

	o.mu.Lock()
	o.iteration++
	iter := o.iteration
	o.mu.Unlock()

	workerLogger, logErr := runlog.Open(o.repoDir, 0)
	if logErr != nil {
		workerLogger = o.parentLogger
	}
	defer func() {
		if logErr == nil {
			_ = workerLogger.Close()
		}
	}()
	workerLogger.SetCurrentMaggusID(group.MaggusID)

	// Create per-worker snapshot writer for TUI split view (parallel tasks only).
	var wsw *workerSnapshotWriter
	if useWorktree {
		wsw = newWorkerSnapshotWriter(o.repoDir, group.MaggusID, task.ID, task.Title, group.Title, o.runStartedAt)
		o.p.Send(InfoMsg{Text: fmt.Sprintf("▶ %s: Starting in worktree", task.ID)})
	} else {
		o.p.Send(InfoMsg{Text: fmt.Sprintf("▶ %s: Starting sequentially (Parallel: no)", task.ID)})
		// Notify the main snapshot (state.json) of the active task so the TUI
		// can show the spinner on the task row for sequential tasks in parallel mode.
		o.p.Send(IterationStartMsg{
			TaskID:    task.ID,
			TaskTitle: task.Title,
			ItemID:    group.MaggusID,
			ItemShort: group.ID,
			ItemTitle: group.Title,
		})
	}

	// Parallel workers write to their own snapshot; sequential tasks use the main program.
	var agentSender agent.MessageSender = o.p
	if wsw != nil {
		agentSender = wsw
	}

	// Serialize merges for parallel (worktree) tasks only.
	var mergeMu *sync.Mutex
	if useWorktree {
		mergeMu = &o.mu
	}

	// Determine the working directory. For parallel tasks, create a dedicated
	// branch and worktree; clean up the worktree after the worker returns.
	workDir := o.repoDir
	var worktreePath string
	if useWorktree {
		taskBranch := gitbranch.BranchName(task.ID)
		worktreePath = filepath.Join(o.repoDir, ".maggus", "worktrees", task.ID)
		if err := gitbranch.CreateBranchFrom(o.repoDir, taskBranch, o.planBranch); err != nil {
			o.markWorkerFailed(task.ID, wsw)
			result.failed = append(result.failed, failedTask{ID: task.ID, Title: task.Title, Reason: fmt.Sprintf("create branch: %v", err)})
			return result
		}
		if err := gitworktree.CreateWorktree(o.repoDir, worktreePath, taskBranch); err != nil {
			_ = gitbranch.DeleteBranch(o.repoDir, taskBranch)
			o.markWorkerFailed(task.ID, wsw)
			result.failed = append(result.failed, failedTask{ID: task.ID, Title: task.Title, Reason: fmt.Sprintf("create worktree: %v", err)})
			return result
		}
		workDir = worktreePath
	}

	cfg := WorkerConfig{
		Ctx:            o.ctx,
		Task:           task,
		PlanFile:       group.File,
		MaggusID:       group.MaggusID,
		PlanTitle:      group.Title,
		Agent:          o.activeAgent,
		Model:          o.resolvedModel,
		SessionPersist: o.sessionPersist,
		ValidIncludes:  o.validIncludes,
		Iteration:      iter,
		RepoDir:        o.repoDir,
		WorkDir:        workDir,
		PlanBranch:     o.planBranch,
		MergeMu:        mergeMu,
		Logger:         workerLogger,
		AgentSender:    agentSender,
		EventSender:    o.p,
		Notifier:       o.notifier,
	}

	wr := RunTaskWorker(cfg)

	// Best-effort cleanup of the worktree (must happen after worker returns,
	// which has already merged and deleted the task branch).
	if worktreePath != "" {
		if err := gitworktree.RemoveWorktree(o.repoDir, worktreePath); err != nil {
			o.p.Send(InfoMsg{Text: fmt.Sprintf("⚠ %s: worktree cleanup failed: %v", task.ID, err)})
		}
	}

	// Handle interruption.
	if wr.StopReason == StopReasonInterrupted {
		o.markWorkerFailed(task.ID, wsw)
		result.stopReason = StopReasonInterrupted
		return result
	}

	// Handle task failure (agent error, commit error, branch error).
	if wr.Failed != nil {
		result.failed = append(result.failed, *wr.Failed)
		o.markWorkerFailed(task.ID, wsw)
		return result
	}

	// Handle merge conflict (task blocked).
	if wr.Blocked {
		result.warnings = append(result.warnings, wr.Warning)
		o.markWorkerBlocked(task.ID, wsw, nil)
		return result
	}

	// Propagate any commit warning (e.g. commit skipped).
	if wr.Warning != "" {
		result.warnings = append(result.warnings, wr.Warning)
	}

	// Mark task as completed in orchestrator state.
	o.mu.Lock()
	o.completedIDs[task.ID] = true
	o.mu.Unlock()

	o.markWorkerDone(task.ID, wsw)
	o.p.Send(InfoMsg{Text: fmt.Sprintf("✓ %s: Completed and merged into %s", task.ID, o.planBranch)})
	_ = globalconfig.IncrementMetrics(globalconfig.Metrics{TasksCompleted: 1})

	if wr.Completed {
		result.completed = 1
	}
	return result
}

// markWorkerDone updates a worker's status to "done" in the index and snapshot.
func (o *parallelOrchestrator) markWorkerDone(taskID string, wsw *workerSnapshotWriter) {
	if wsw != nil {
		wsw.SetStatus("Done")
	}
	o.mu.Lock()
	if o.hasWorkerMaps() {
		o.setWorkerStatus(taskID, "done")
	}
	o.mu.Unlock()
}

// markWorkerFailed updates a worker's status to "failed" in the index and snapshot.
func (o *parallelOrchestrator) markWorkerFailed(taskID string, wsw *workerSnapshotWriter) {
	if wsw != nil {
		wsw.SetStatus("Failed")
	}
	o.mu.Lock()
	if o.hasWorkerMaps() {
		o.setWorkerStatus(taskID, "failed")
	}
	o.mu.Unlock()
}

// markWorkerBlocked updates a worker's status to "blocked" in the index and snapshot.
func (o *parallelOrchestrator) markWorkerBlocked(taskID string, wsw *workerSnapshotWriter, _ error) {
	if wsw != nil {
		wsw.SetStatus("Blocked")
	}
	o.mu.Lock()
	if o.hasWorkerMaps() {
		o.setWorkerStatus(taskID, "blocked")
	}
	o.mu.Unlock()
}


func checkCriterionInFile(filePath, criterionText string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	oldLine := "- [ ] " + criterionText
	newLine := "- [x] " + criterionText
	content := string(data)
	if !strings.Contains(content, oldLine) {
		return nil
	}
	content = strings.Replace(content, oldLine, newLine, 1)
	return os.WriteFile(filePath, []byte(content), 0o644)
}
