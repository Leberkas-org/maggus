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
	"github.com/leberkas-org/maggus/internal/gitcommit"
	"github.com/leberkas-org/maggus/internal/gitmerge"
	"github.com/leberkas-org/maggus/internal/gitworktree"
	"github.com/leberkas-org/maggus/internal/globalconfig"
	"github.com/leberkas-org/maggus/internal/notify"
	"github.com/leberkas-org/maggus/internal/parser"
	"github.com/leberkas-org/maggus/internal/prompt"
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
	runID          string
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
			planBranch: planBranch, runID: params.runID,
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

// runSingleTask runs one task. When useWorktree is true, the task runs in its own
// worktree on its own branch (parallel mode). When false, it runs in the main worktree.
func (o *parallelOrchestrator) runSingleTask(group parser.Plan, task parser.Task, useWorktree bool) parallelWorkResult {
	var result parallelWorkResult
	taskBranch := gitbranch.BranchName(task.ID)

	o.mu.Lock()
	o.iteration++
	iter := o.iteration
	o.mu.Unlock()

	workerLogger, logErr := runlog.OpenWorker(iter, task.ID, o.repoDir)
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
		wsw = newWorkerSnapshotWriter(o.repoDir, o.runID, task.ID, task.Title, group.Title, o.runStartedAt)
	}

	workDir := o.repoDir
	if useWorktree {
		worktreePath := filepath.Join(o.repoDir, ".maggus", "worktrees", task.ID)
		o.p.Send(InfoMsg{Text: fmt.Sprintf("▶ %s: Starting in worktree", task.ID)})

		if err := gitbranch.CreateBranchFrom(o.repoDir, taskBranch, o.planBranch); err != nil {
			o.markWorkerFailed(task.ID, wsw)
			return o.failTask(&result, workerLogger, task, fmt.Sprintf("create branch: %v", err))
		}
		if err := gitworktree.CreateWorktree(o.repoDir, worktreePath, taskBranch); err != nil {
			o.markWorkerFailed(task.ID, wsw)
			return o.failTask(&result, workerLogger, task, fmt.Sprintf("create worktree: %v", err))
		}
		workDir = worktreePath
	} else {
		o.p.Send(InfoMsg{Text: fmt.Sprintf("▶ %s: Starting sequentially (Parallel: no)", task.ID)})
		if _, _, err := gitbranch.EnsureTaskBranchFromBase(o.repoDir, task.ID, o.planBranch); err != nil {
			return o.failTask(&result, workerLogger, task, fmt.Sprintf("create task branch: %v", err))
		}
	}

	workerLogger.TaskStart(task.ID, task.Title)

	// Run agent — parallel workers use per-worker snapshot writer, sequential use main program.
	opts := prompt.Options{Include: o.validIncludes, RunID: o.runID, Iteration: iter}
	builtPrompt := prompt.Build(&task, opts)
	model := resolveTaskModel(task.Model, o.resolvedModel)

	var agentSender agent.MessageSender = o.p
	if wsw != nil {
		agentSender = wsw
	}
	if err := o.activeAgent.Run(o.ctx, builtPrompt, model, o.sessionPersist, agentSender); err != nil {
		if o.ctx.Err() != nil {
			result.stopReason = StopReasonInterrupted
			return result
		}
		o.notifier.PlayError()
		_ = globalconfig.IncrementMetrics(globalconfig.Metrics{AgentErrors: 1})
		reason := err.Error()
		workerLogger.TaskFailed(task.ID, reason)
		o.p.Send(InfoMsg{Text: fmt.Sprintf("✗ %s failed: %s", task.ID, reason)})
		result.failed = append(result.failed, failedTask{ID: task.ID, Title: task.Title, Reason: reason})
		o.markWorkerFailed(task.ID, wsw)
		if useWorktree {
			_ = gitworktree.RemoveWorktree(o.repoDir, filepath.Join(o.repoDir, ".maggus", "worktrees", task.ID))
		}
		return result
	}

	// Commit.
	commitResult, commitErr := gitcommit.CommitIteration(workDir, task.ID+": "+task.Title)
	if commitErr != nil {
		reason := commitErr.Error()
		workerLogger.TaskFailed(task.ID, reason)
		o.p.Send(InfoMsg{Text: fmt.Sprintf("✗ %s commit failed: %s", task.ID, reason)})
		result.failed = append(result.failed, failedTask{ID: task.ID, Title: task.Title, Reason: reason})
		o.markWorkerFailed(task.ID, wsw)
		if useWorktree {
			_ = gitworktree.RemoveWorktree(o.repoDir, filepath.Join(o.repoDir, ".maggus", "worktrees", task.ID))
		}
		return result
	}

	if !commitResult.Committed {
		msg := commitResult.Message
		if msg == "" {
			msg = "commit skipped (unknown reason)"
		}
		result.warnings = append(result.warnings, fmt.Sprintf("%s: %s", task.ID, msg))
		o.p.Send(InfoMsg{Text: fmt.Sprintf("⚠ %s: %s", task.ID, msg)})
	} else {
		workerLogger.TaskComplete(task.ID, captureShortHash(workDir))
		o.p.Send(CommitMsg{Message: commitResult.Message})
		o.notifier.PlayTaskComplete()
		_ = globalconfig.IncrementMetrics(globalconfig.Metrics{GitCommits: 1})
	}

	// Merge task branch into plan branch (serialized for worktree tasks).
	if useWorktree {
		o.mu.Lock()
		mergeErr := gitmerge.MergeTaskBranch(o.repoDir, o.planBranch, taskBranch)
		o.mu.Unlock()
		if mergeErr != nil {
			o.markWorkerBlocked(task.ID, wsw, mergeErr)
			return o.handleMergeErr(&result, workerLogger, task, mergeErr)
		}
	} else {
		if mergeErr := gitmerge.MergeTaskBranch(o.repoDir, o.planBranch, taskBranch); mergeErr != nil {
			return o.handleMergeErr(&result, workerLogger, task, mergeErr)
		}
	}

	// Mark task complete in the plan file (serialized).
	o.mu.Lock()
	o.markTaskCriteriaComplete(group.File, task.ID)
	o.completedIDs[task.ID] = true
	o.mu.Unlock()

	// Update worker snapshot and index with done status.
	o.markWorkerDone(task.ID, wsw)

	o.p.Send(InfoMsg{Text: fmt.Sprintf("✓ %s: Completed and merged into %s", task.ID, o.planBranch)})
	_ = globalconfig.IncrementMetrics(globalconfig.Metrics{TasksCompleted: 1})

	if commitResult.Committed {
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

// failTask records a failed task and returns the result.
func (o *parallelOrchestrator) failTask(result *parallelWorkResult, logger *runlog.Logger, task parser.Task, reason string) parallelWorkResult {
	logger.TaskFailed(task.ID, reason)
	o.p.Send(InfoMsg{Text: fmt.Sprintf("✗ %s: %s", task.ID, reason)})
	result.failed = append(result.failed, failedTask{ID: task.ID, Title: task.Title, Reason: reason})
	return *result
}

// handleMergeErr handles merge errors (conflicts vs other failures).
func (o *parallelOrchestrator) handleMergeErr(result *parallelWorkResult, logger *runlog.Logger, task parser.Task, err error) parallelWorkResult {
	if _, ok := err.(*gitmerge.MergeConflictError); ok {
		o.p.Send(InfoMsg{Text: fmt.Sprintf("⚠ %s: merge conflict — task blocked, worktree preserved", task.ID)})
		result.warnings = append(result.warnings, fmt.Sprintf("%s: merge conflict", task.ID))
	} else {
		reason := fmt.Sprintf("merge: %v", err)
		logger.TaskFailed(task.ID, reason)
		o.p.Send(InfoMsg{Text: fmt.Sprintf("✗ %s: %s", task.ID, reason)})
		result.failed = append(result.failed, failedTask{ID: task.ID, Title: task.Title, Reason: reason})
	}
	return *result
}

// markTaskCriteriaComplete checks off all unchecked, non-blocked acceptance criteria.
// Must be called with o.mu held.
func (o *parallelOrchestrator) markTaskCriteriaComplete(planFile, taskID string) {
	tasks, err := parser.ParseFile(planFile)
	if err != nil {
		return
	}
	for _, t := range tasks {
		if t.ID != taskID {
			continue
		}
		for _, c := range t.Criteria {
			if !c.Checked && !c.Blocked {
				_ = checkCriterionInFile(planFile, c.Text)
			}
		}
		return
	}
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
