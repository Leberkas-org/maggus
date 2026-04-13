package cmd

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/leberkas-org/maggus/internal/agent"
	"github.com/leberkas-org/maggus/internal/approval"
	"github.com/leberkas-org/maggus/internal/config"
	"github.com/leberkas-org/maggus/internal/discord"
	"github.com/leberkas-org/maggus/internal/gitutil"
	"github.com/leberkas-org/maggus/internal/globalconfig"
	"github.com/leberkas-org/maggus/internal/hooks"
	"github.com/leberkas-org/maggus/internal/notify"
	"github.com/leberkas-org/maggus/internal/parser"
	"github.com/leberkas-org/maggus/internal/runlog"
	"github.com/leberkas-org/maggus/internal/stores"
)

// OrchestratorConfig holds everything the Orchestrator needs to run the
// sequential feature work loop.
type OrchestratorConfig struct {
	// Execution context (cancellation propagated to each RunTaskWorker call).
	Ctx     context.Context
	Program *tea.Program

	// Agent and model configuration passed through to each RunTaskWorker call.
	Agent          agent.Agent
	Model          string
	SessionPersist bool
	ValidIncludes  []string

	// Plan stores for loading and re-parsing feature and bug files.
	FeatureStore stores.FeatureStore
	BugStore     stores.BugStore

	// Structured run log and sound notification.
	Logger   *runlog.Logger
	Notifier *notify.Notifier

	// Discord Rich Presence (nil when disabled).
	Presence *discord.Presence

	// Directory configuration.
	// Dir is the project root used for config/approval loading and hooks.
	// RepoDir is the main repository root used for git operations and branching.
	// In normal (non-dispatch) mode these are the same directory.
	Dir     string
	RepoDir string

	// PlanBranch is the integration branch for per-task branching. When set,
	// RunTaskWorker creates a task branch from this, merges back after commit,
	// and deletes the task branch. Empty disables per-task branching.
	PlanBranch string

	// Feature lifecycle configuration (completion actions and hook commands).
	OnComplete config.OnCompleteConfig
	Hooks      config.HooksConfig

	// FeatureGroups is the ordered list of approved plans to work (bugs first,
	// then features). Populated by buildApprovedPlans before constructing the
	// orchestrator.
	FeatureGroups []parser.Plan

	// AutoContinue mirrors config.auto_continue: when true and FeatureCount is
	// zero, the loop processes all feature groups rather than stopping after one.
	AutoContinue bool

	// FeatureCount limits the number of feature groups to process. Zero means
	// "all groups" (when AutoContinue is true) or "one group" (default).
	FeatureCount int

	// StopFlag is set by the stop-after-task sentinel watcher so the loop
	// halts between tasks rather than cancelling the active invocation.
	StopFlag *atomic.Bool

	// StopAtTaskID stores the task ID the user requested to stop at (string).
	// When set, the loop stops after completing (or skipping past) that task.
	StopAtTaskID *atomic.Value

	// Startup display info used for TUI messages and the final summary.
	StartTime     time.Time
	ModelDisplay  string
	StartHash     string
	ActiveAgentNm string
	IncludeWarns  []string
	BranchMsg     string
	SyncInfoMsg   string
}

// OrchestratorResult holds the outcome of a Run() call.
type OrchestratorResult struct {
	Completed   int
	Failed      []failedTask
	Warnings    []string
	StopReason  StopReason
	ErrorDetail string
}

// Orchestrator handles the unified feature iteration loop.
// It processes one feature group at a time, dispatching tasks via RunTaskWorker.
// Tasks classified as parallel run concurrently in isolated git worktrees;
// sequential tasks run one at a time in the main repo directory.
type Orchestrator struct {
	cfg OrchestratorConfig

	// Per-group parallel state — reset at the start of each runGroupTasks call.
	// Guarded by mu; also used as MergeMu for parallel workers.
	mu                  sync.Mutex
	completedIDs        map[string]bool // task IDs that have successfully committed
	failedIDs           map[string]bool // task IDs that failed (excluded from future batches)
	skippedOrBlockedIDs map[string]bool // task IDs that are blocked or skipped (satisfy predecessors)
	iteration           int             // monotonically increasing per-worker counter

	// Per-group worker TUI state for split pane view — reset per group.
	// Guarded by mu.
	workerOrder     []string          // ordered list of worker task IDs
	workerStatuses  map[string]string // taskID → "working"/"done"/"failed"/"blocked"
	workerTitles    map[string]string // taskID → task title
	workerStartedAt map[string]string // taskID → RFC3339 start time

	// dispatchWG tracks goroutines launched for file-based dispatch requests.
	// Run() waits on this before returning so dispatched tasks are not orphaned.
	dispatchWG sync.WaitGroup
}

// NewOrchestrator creates an Orchestrator with the given configuration.
func NewOrchestrator(cfg OrchestratorConfig) *Orchestrator {
	return &Orchestrator{cfg: cfg}
}

// Run executes the sequential feature loop. It is intended to be called from
// a goroutine; it sends TUI events via cfg.Program and returns when all work
// is done or a stop condition is reached.
//
// Outer loop: iterates approved feature groups respecting FeatureCount and
// AutoContinue. Between features it re-checks approval so mid-run revocations
// take effect.
//
// Inner loop: per group it finds the next workable task, calls RunTaskWorker,
// re-parses the task list, fires hooks, and performs a between-task remote
// sync check before the next iteration.
func (o *Orchestrator) Run() OrchestratorResult {
	cfg := o.cfg
	p := cfg.Program

	// Emit startup info messages.
	if cfg.ActiveAgentNm == "claude" {
		p.Send(InfoMsg{Text: "⚠ Using --dangerously-skip-permissions (Claude Code)"})
	}
	for _, w := range cfg.IncludeWarns {
		p.Send(InfoMsg{Text: w})
	}
	if cfg.BranchMsg != "" {
		p.Send(InfoMsg{Text: cfg.BranchMsg})
	}
	if cfg.SyncInfoMsg != "" {
		p.Send(InfoMsg{Text: cfg.SyncInfoMsg})
	}

	// Determine effective feature limit:
	//   FeatureCount 0 + AutoContinue false  → process 1 group
	//   FeatureCount 0 + AutoContinue true   → process all groups
	//   FeatureCount N > 0                   → process N groups
	effectiveFeatureLimit := cfg.FeatureCount
	if effectiveFeatureLimit == 0 && !cfg.AutoContinue {
		effectiveFeatureLimit = 1
	}
	featureUnlimited := effectiveFeatureLimit == 0

	groups := cfg.FeatureGroups
	featureTotal := len(groups)

	if featureTotal == 0 {
		p.Send(InfoMsg{Text: "No approved features available."})
		summary := o.buildSummary(0, nil, StopReasonNoTasks, "no approved features", nil)
		cfg.Notifier.PlayRunComplete()
		p.Send(SummaryMsg{Data: summary})
		pushToRemote(p, cfg.RepoDir, 0, summary.Branch)
		return OrchestratorResult{StopReason: StopReasonNoTasks, ErrorDetail: "no approved features"}
	}

	stopReason := StopReasonComplete
	var errorDetail string
	var warnings []string
	var failedTasks []failedTask
	totalCompleted := 0
	featuresDone := 0

	for fi, group := range groups {
		if !featureUnlimited && featuresDone >= effectiveFeatureLimit {
			break
		}
		if cfg.Ctx.Err() != nil {
			stopReason = StopReasonInterrupted
			break
		}

		// Between-feature stop flag check (only after the first feature has started).
		if featuresDone > 0 && cfg.StopFlag != nil && cfg.StopFlag.Load() {
			stopReason = StopReasonUserStop
			break
		}

		// Re-check approval before each subsequent feature so mid-run revocations
		// take effect without restarting the daemon.
		if featuresDone > 0 {
			if freshCfg, cfgErr := loadConfigFn(cfg.Dir); cfgErr == nil {
				if approvals, aErr := approval.Load(cfg.Dir); aErr == nil {
					if !isPlanApproved(group, approvals, freshCfg.IsApprovalRequired()) {
						continue
					}
				}
			}
		}

		grResult := o.runGroupTasks(group, fi, featureTotal)
		totalCompleted += grResult.completed
		failedTasks = append(failedTasks, grResult.failed...)
		warnings = append(warnings, grResult.warnings...)

		if grResult.stopped {
			stopReason = grResult.stopReason
			break
		}

		featuresDone++
	}

	// Resolve final stop reason.
	if len(failedTasks) > 0 && stopReason == StopReasonComplete {
		stopReason = StopReasonPartialComplete
	}
	if totalCompleted == 0 && stopReason == StopReasonComplete {
		if len(warnings) > 0 {
			stopReason = StopReasonError
			errorDetail = "agent ran but produced no commits"
		} else {
			stopReason = StopReasonNoTasks
		}
	}

	// Wait for any in-flight dispatch goroutines to complete before summarising.
	o.dispatchWG.Wait()

	summary := o.buildSummary(totalCompleted, failedTasks, stopReason, errorDetail, warnings)
	cfg.Notifier.PlayRunComplete()
	p.Send(SummaryMsg{Data: summary})
	pushToRemote(p, cfg.RepoDir, totalCompleted, summary.Branch)

	return OrchestratorResult{
		Completed:   totalCompleted,
		Failed:      failedTasks,
		Warnings:    warnings,
		StopReason:  stopReason,
		ErrorDetail: errorDetail,
	}
}

// runGroupTasks runs all workable tasks within a single plan group.
// featureIndex is the 0-based position of this group in the outer loop (used
// for IterationStartMsg feature progress display).
//
// The inner loop classifies tasks each iteration:
//   - Parallel tasks (Parallel=true with all predecessors complete) run
//     concurrently in isolated git worktrees via runParallelBatch.
//   - Sequential tasks run one at a time in the main repo directory.
func (o *Orchestrator) runGroupTasks(group parser.Plan, featureIndex, featureTotal int) groupTasksResult {
	var result groupTasksResult
	cfg := o.cfg

	if o.countRunnable(group.Tasks) == 0 {
		return result
	}

	// Initialize per-group parallel state. Pre-populate completedIDs with tasks
	// that are already done and skippedOrBlockedIDs with tasks that are blocked
	// or skipped so predecessor tracking works from the start.
	o.mu.Lock()
	o.completedIDs = make(map[string]bool)
	o.failedIDs = make(map[string]bool)
	o.skippedOrBlockedIDs = make(map[string]bool)
	o.iteration = 0
	o.workerOrder = nil
	o.workerStatuses = nil
	o.workerTitles = nil
	o.workerStartedAt = nil
	for _, t := range group.Tasks {
		if t.IsComplete() {
			o.completedIDs[t.ID] = true
		}
		if t.IsBlocked() || t.IsSkipped() {
			o.skippedOrBlockedIDs[t.ID] = true
		}
	}
	o.mu.Unlock()

	// Clean up worker snapshot files when this group finishes so stale entries
	// from a previous group never appear in the TUI for the next one.
	defer cleanupWorkerSnapshots(cfg.RepoDir)

	// Ensure a stable MaggusID exists in the plan file.
	if maggusID, err := parser.EnsureMaggusID(group.File); err == nil {
		group.MaggusID = maggusID
	}
	cfg.Logger.SetCurrentMaggusID(group.MaggusID)
	cfg.Logger.FeatureStart(group.ID)

	// Build a taskContext for buildPreCommitFn (pre-commit callback reuse).
	tc := o.buildTaskContext(&group, featureIndex+1, featureTotal)

	groupTasks := group.Tasks
	var lastCompletedTaskID string

	for batchI := 0; ; batchI++ {
		if cfg.Ctx.Err() != nil {
			result.stopped = true
			result.stopReason = StopReasonInterrupted
			return result
		}

		// Check for file-based dispatch requests at the start of each iteration.
		// Dispatched tasks run concurrently in isolated worktrees and do not block
		// the normal sequential/parallel task queue.
		o.runDispatchRequests()

		// Between-batch stop flag check (after the first batch completes).
		if batchI > 0 && cfg.StopFlag != nil && cfg.StopFlag.Load() {
			targetID := ""
			if cfg.StopAtTaskID != nil {
				if v := cfg.StopAtTaskID.Load(); v != nil {
					targetID, _ = v.(string)
				}
			}
			if targetID == "" || targetID == lastCompletedTaskID ||
				isTaskAtOrPastTarget(groupTasks, lastCompletedTaskID, targetID) {
				result.stopped = true
				result.stopReason = StopReasonUserStop
				return result
			}
		}

		// Classify workable tasks: parallel tasks run concurrently in worktrees;
		// sequential tasks run one at a time in the main repo.
		par, seq := o.classifyWorkable(groupTasks)
		if len(par) == 0 && len(seq) == 0 {
			break
		}

		if len(par) > 0 {
			// -- Parallel batch: run all eligible parallel tasks concurrently --
			batchResult := o.runParallelBatch(&group, par)
			result.completed += batchResult.completed
			result.failed = append(result.failed, batchResult.failed...)
			result.warnings = append(result.warnings, batchResult.warnings...)

			if batchResult.stopped {
				result.stopped = true
				result.stopReason = batchResult.stopReason
				return result
			}

			// Re-parse to reflect changes made by parallel workers.
			if tasks, err := parseAllTasks(cfg.FeatureStore, cfg.BugStore); err == nil {
				groupTasks = filterTasksBySourceFile(tasks, group.File)
			}
			// No between-batch sync after parallel batches; lastCompletedTaskID
			// stays unchanged (parallel batches don't update the stop-at-task cursor).
			continue
		}

		// -- Sequential task: run the first eligible task in the main repo --
		workableRemaining := o.countRunnable(groupTasks)
		displayCount := batchI + workableRemaining

		next := &seq[0]

		// Send iteration-start event to TUI.
		sendIterationStart(cfg.Program, next, groupTasks, batchI, displayCount,
			featureIndex+1, featureTotal, &group)

		// Update Discord Rich Presence with current task info.
		if cfg.Presence != nil {
			completed, total := computeTaskProgress(groupTasks, next.SourceFile)
			cfg.Presence.Update(discord.PresenceState{
				TaskID:          next.ID,
				TaskTitle:       next.Title,
				FeatureTitle:    parser.ParseFileTitle(next.SourceFile),
				StartTime:       time.Now(),
				Verb:            verbForTask(next.SourceFile),
				ProgressCurrent: completed,
				ProgressTotal:   total,
			})
		}

		// Execute the full task lifecycle via the unified worker.
		wr := RunTaskWorker(WorkerConfig{
			Ctx:            cfg.Ctx,
			Task:           *next,
			PlanFile:       group.File,
			MaggusID:       group.MaggusID,
			PlanTitle:      group.Title,
			Agent:          cfg.Agent,
			Model:          cfg.Model,
			SessionPersist: cfg.SessionPersist,
			ValidIncludes:  cfg.ValidIncludes,
			Iteration:      batchI + 1,
			RepoDir:        cfg.RepoDir,
			WorkDir:        cfg.RepoDir,
			PlanBranch:     cfg.PlanBranch,
			Logger:         cfg.Logger,
			AgentSender:    cfg.Program,
			EventSender:    cfg.Program,
			Notifier:       cfg.Notifier,
			PreCommit:      buildPreCommitFn(tc),
		})

		if wr.StopReason != 0 {
			result.stopped = true
			result.stopReason = wr.StopReason
			return result
		}

		if wr.Failed != nil {
			result.failed = append(result.failed, *wr.Failed)
			_ = globalconfig.IncrementMetrics(globalconfig.Metrics{TasksFailed: 1})
			o.mu.Lock()
			o.failedIDs[next.ID] = true
			o.mu.Unlock()
			lastCompletedTaskID = next.ID
			// Re-parse so the loop can continue with the next task.
			if tasks, err := parseAllTasks(cfg.FeatureStore, cfg.BugStore); err == nil {
				groupTasks = filterTasksBySourceFile(tasks, group.File)
			}
			continue
		}

		if wr.Blocked {
			if wr.Warning != "" {
				result.warnings = append(result.warnings, wr.Warning)
			}
			o.mu.Lock()
			o.skippedOrBlockedIDs[next.ID] = true
			o.mu.Unlock()
			if tasks, err := parseAllTasks(cfg.FeatureStore, cfg.BugStore); err == nil {
				groupTasks = filterTasksBySourceFile(tasks, group.File)
			}
			continue
		}

		// Re-parse to reflect any changes the agent made to the plan files.
		parsedTasks, parseErr := parseAllTasks(cfg.FeatureStore, cfg.BugStore)
		if parseErr != nil {
			reason := fmt.Sprintf("re-parse tasks: %v", parseErr)
			result.failed = append(result.failed, failedTask{ID: next.ID, Title: next.Title, Reason: reason})
			_ = globalconfig.IncrementMetrics(globalconfig.Metrics{TasksFailed: 1})
			continue
		}
		groupTasks = filterTasksBySourceFile(parsedTasks, group.File)
		lastCompletedTaskID = next.ID

		// Mark sequential task as completed for predecessor tracking.
		o.mu.Lock()
		o.completedIDs[next.ID] = true
		o.mu.Unlock()

		if wr.Warning != "" {
			result.warnings = append(result.warnings, wr.Warning)
		}

		if wr.Completed {
			result.completed++
			_ = globalconfig.IncrementMetrics(globalconfig.Metrics{TasksCompleted: 1})

			// Update Discord presence to reflect the completed task.
			if cfg.Presence != nil {
				completed, total := computeTaskProgress(parsedTasks, next.SourceFile)
				cfg.Presence.Update(discord.PresenceState{
					TaskID:          next.ID,
					TaskTitle:       next.Title,
					FeatureTitle:    parser.ParseFileTitle(next.SourceFile),
					StartTime:       time.Now(),
					Verb:            verbForTask(next.SourceFile),
					ProgressCurrent: completed,
					ProgressTotal:   total,
				})
			}

			// Fire task_complete hook (zero overhead when unconfigured).
			if len(cfg.Hooks.OnTaskComplete) > 0 {
				event := hooks.Event{
					Type:      "task_complete",
					File:      filepath.Base(next.SourceFile),
					MaggusID:  parser.ParseMaggusID(next.SourceFile),
					Title:     next.Title,
					Tasks:     []hooks.TaskInfo{{ID: next.ID, Title: next.Title}},
					Timestamp: time.Now().UTC().Format(time.RFC3339),
				}
				hooks.Run(cfg.Hooks.OnTaskComplete, event, cfg.Dir, log.Default())
			}
		}

		// Update progress bar using refreshed task count.
		progressTotal := (batchI + 1) + o.countRunnable(parsedTasks)
		cfg.Program.Send(ProgressMsg{Current: batchI + 1, Total: progressTotal})

		// Between-task sync check: skip when this was the last workable task.
		if workableRemaining > 1 {
			if syncResult := betweenTaskSync(cfg.Ctx, cfg.RepoDir, cfg.Program); syncResult != nil {
				result.stopped = true
				result.stopReason = syncResult.stopReason
				return result
			}
		}
	}

	cfg.Logger.FeatureComplete(group.ID)
	cfg.Logger.SetCurrentMaggusID("")
	return result
}

// buildSummary constructs the SummaryData for the end-of-run screen.
func (o *Orchestrator) buildSummary(completed int, failedTasks []failedTask, stopReason StopReason, errorDetail string, warnings []string) SummaryData {
	cfg := o.cfg

	endHashCmd := gitutil.Command("rev-parse", "--short", "HEAD")
	endHashCmd.Dir = cfg.RepoDir
	endHashBytes, _ := endHashCmd.Output()
	endHash := strings.TrimSpace(string(endHashBytes))

	branchCmd := gitutil.Command("rev-parse", "--abbrev-ref", "HEAD")
	branchCmd.Dir = cfg.RepoDir
	branchOut, _ := branchCmd.Output()
	currentBranch := strings.TrimSpace(string(branchOut))

	var remaining []RemainingTask
	if latestTasks, err := parseAllTasks(cfg.FeatureStore, cfg.BugStore); err == nil {
		for _, t := range latestTasks {
			if t.IsWorkable() {
				remaining = append(remaining, RemainingTask{ID: t.ID, Title: t.Title})
			}
		}
	}

	runnerFailed := make([]FailedTask, len(failedTasks))
	for i, ft := range failedTasks {
		runnerFailed[i] = FailedTask{ID: ft.ID, Title: ft.Title, Reason: ft.Reason}
	}

	return SummaryData{
		Branch:         currentBranch,
		Model:          cfg.ModelDisplay,
		StartTime:      cfg.StartTime,
		TasksCompleted: completed,
		TasksTotal:     completed + len(failedTasks),
		CommitStart:    cfg.StartHash,
		CommitEnd:      endHash,
		RemainingTasks: remaining,
		Reason:         stopReason,
		ErrorDetail:    errorDetail,
		Warnings:       warnings,
		FailedTasks:    runnerFailed,
		TasksFailed:    len(failedTasks),
	}
}

// buildTaskContext constructs a taskContext from the orchestrator config for
// use with buildPreCommitFn and fireCompletionHooks.
func (o *Orchestrator) buildTaskContext(plan *parser.Plan, featureCurrent, featureTotal int) taskContext {
	cfg := o.cfg
	tc := taskContext{
		workCtx:            cfg.Ctx,
		p:                  cfg.Program,
		activeAgent:        cfg.Agent,
		resolvedModel:      cfg.Model,
		sessionPersistence: cfg.SessionPersist,
		notifier:           cfg.Notifier,
		validIncludes:      cfg.ValidIncludes,
		repoDir:            cfg.RepoDir,
		workDir:            cfg.Dir,
		onComplete:         cfg.OnComplete,
		hooks:              cfg.Hooks,
		logger:             cfg.Logger,
		featureStore:       cfg.FeatureStore,
		bugStore:           cfg.BugStore,
		presence:           cfg.Presence,
		planBranch:         cfg.PlanBranch,
		currentPlan:        plan,
		featureCurrent:     featureCurrent,
		featureTotal:       featureTotal,
	}
	if plan != nil {
		tc.featureSourceFile = plan.File
	}
	return tc
}
