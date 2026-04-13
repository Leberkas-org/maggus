package cmd

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/leberkas-org/maggus/internal/agent"
	"github.com/leberkas-org/maggus/internal/config"
	"github.com/leberkas-org/maggus/internal/discord"
	"github.com/leberkas-org/maggus/internal/gitsync"
	"github.com/leberkas-org/maggus/internal/gitutil"
	"github.com/leberkas-org/maggus/internal/globalconfig"
	"github.com/leberkas-org/maggus/internal/hooks"
	"github.com/leberkas-org/maggus/internal/notify"
	"github.com/leberkas-org/maggus/internal/parser"
	"github.com/leberkas-org/maggus/internal/runlog"
	"github.com/leberkas-org/maggus/internal/stores"
)

// taskAction indicates what the work loop should do after a task iteration.
type taskAction int

const (
	taskContinue   taskAction = iota // proceed to next iteration
	taskBreak                        // stop the work loop
	taskRetry                        // retry this iteration (decrement counter)
	taskSkipToNext                   // skip to next task (agent or commit failed)
)

// taskResult holds the outcome of a single task iteration.
type taskResult struct {
	action     taskAction
	stopReason StopReason    // only set when action == taskBreak
	committed  bool          // true if a commit was made
	warning    string        // non-empty if commit succeeded but with a caveat
	failed     *failedTask   // non-nil if the task failed
	tasks      []parser.Task // updated task list after re-parse (nil if unchanged)
	taskID     string        // ID of the task that was worked on
}

// taskContext bundles the shared state needed by runTask.
type taskContext struct {
	workCtx            context.Context
	p                  *tea.Program
	activeAgent        agent.Agent
	resolvedModel      string
	sessionPersistence bool
	notifier           *notify.Notifier
	validIncludes      []string
	repoDir            string
	workDir            string
	onComplete         config.OnCompleteConfig
	hooks              config.HooksConfig
	logger             *runlog.Logger // structured run log; nil-safe
	featureStore       stores.FeatureStore
	bugStore           stores.BugStore

	// Discord Rich Presence (nil when disabled).
	presence *discord.Presence

	// Branch context: plan branch for per-task branching (empty = no branching).
	// Set by the work goroutine from runLoopParams.planBranch. When non-empty,
	// the unified worker creates a task branch from this, merges back after
	// commit, and deletes the task branch.
	planBranch string

	// existingWorktreePath is the path to a pre-existing worktree for dispatch
	// mode. When non-empty, the worker skips branch/worktree creation and runs
	// the agent in this directory. Set in runOneDaemonCycle when dispatchRepoFlag
	// is active.
	existingWorktreePath string

	// Feature-centric context (set per-group by runWorkGoroutine).
	currentPlan       *parser.Plan // current plan being worked on (set per-group)
	featureSourceFile string       // scope parsedTasks to this source file for progress calculation
	featureCurrent    int          // 1-based index of current feature (for TUI display)
	featureTotal      int          // total features being processed (for TUI display)
}

// runTask executes a single task iteration via the unified RunTaskWorker:
// finds the next workable task, signals the TUI, invokes the worker (which
// handles branch creation → agent → pre-commit ops → commit → merge-back →
// branch cleanup), re-parses the task list, fires post-commit hooks, updates
// progress, and performs between-task sync checks.
//
// The caller's loop index (i) and total count are needed for progress tracking.
// The tasks slice is the current parsed task list.
// maxCount is the user-requested task limit (0 = unlimited; used to cap progress total).
func runTask(tc taskContext, tasks []parser.Task, i, count, maxCount int) taskResult {
	if tc.workCtx.Err() != nil {
		return taskResult{action: taskBreak, stopReason: StopReasonInterrupted}
	}

	// Find next workable task.
	next := findNextWorkableTask(tasks)
	if next == nil {
		return taskResult{action: taskBreak}
	}

	// Signal iteration start to the TUI.
	sendIterationStart(tc.p, next, tasks, i, count, tc.featureCurrent, tc.featureTotal, tc.currentPlan)

	// Update Discord Rich Presence with current task info and progress.
	if tc.presence != nil {
		completed, total := computeTaskProgress(tasks, next.SourceFile)
		tc.presence.Update(discord.PresenceState{
			TaskID:          next.ID,
			TaskTitle:       next.Title,
			FeatureTitle:    parser.ParseFileTitle(next.SourceFile),
			StartTime:       time.Now(),
			Verb:            verbForTask(next.SourceFile),
			ProgressCurrent: completed,
			ProgressTotal:   total,
		})
	}

	// Resolve plan metadata from the current group context.
	var planFile, maggusID, planTitle string
	if tc.currentPlan != nil {
		planFile = tc.currentPlan.File
		maggusID = tc.currentPlan.MaggusID
		planTitle = tc.currentPlan.Title
	}

	// Determine the effective work directory: use an existing worktree path when
	// set (dispatch mode), otherwise use the main repo directory.
	workerWorkDir := tc.repoDir
	if tc.existingWorktreePath != "" {
		workerWorkDir = tc.existingWorktreePath
	}

	// Run the unified task worker. The worker owns the complete lifecycle:
	// branch creation → agent → pre-commit callback → commit → merge-back → cleanup.
	wr := RunTaskWorker(WorkerConfig{
		Ctx:            tc.workCtx,
		Task:           *next,
		PlanFile:       planFile,
		MaggusID:       maggusID,
		PlanTitle:      planTitle,
		Agent:          tc.activeAgent,
		Model:          tc.resolvedModel,
		SessionPersist: tc.sessionPersistence,
		ValidIncludes:  tc.validIncludes,
		Iteration:      i + 1,
		RepoDir:        tc.repoDir,
		WorkDir:        workerWorkDir,
		PlanBranch:     tc.planBranch,
		Logger:         tc.logger,
		AgentSender:    tc.p,
		EventSender:    tc.p,
		Notifier:       tc.notifier,
		PreCommit:      buildPreCommitFn(tc),
	})

	// Handle interruption.
	if wr.StopReason != 0 {
		return taskResult{action: taskBreak, stopReason: wr.StopReason}
	}
	// Handle agent / commit / branch failures.
	if wr.Failed != nil {
		return taskResult{
			action: taskSkipToNext,
			taskID: next.ID,
			failed: wr.Failed,
		}
	}
	// Handle merge conflict (task marked blocked, loop skips to next).
	if wr.Blocked {
		return taskResult{
			action:  taskSkipToNext,
			taskID:  next.ID,
			warning: wr.Warning,
		}
	}

	// Re-parse to pick up any changes the agent made (bugs + features).
	parsedTasks, parseErr := parseAllTasks(tc.featureStore, tc.bugStore)
	if parseErr != nil {
		reason := fmt.Sprintf("re-parse tasks: %v", parseErr)
		return taskResult{
			action: taskSkipToNext,
			taskID: next.ID,
			failed: &failedTask{ID: next.ID, Title: next.Title, Reason: reason},
		}
	}

	// Scope parsed tasks to the current feature source file.
	scopedTasks := parsedTasks
	if tc.featureSourceFile != "" {
		scopedTasks = filterTasksBySourceFile(parsedTasks, tc.featureSourceFile)
	}

	result := taskResult{
		action:    taskContinue,
		tasks:     scopedTasks,
		taskID:    next.ID,
		committed: wr.Completed,
		warning:   wr.Warning,
	}

	if wr.Completed {
		// Update Discord presence with incremented progress (task now counts as done).
		if tc.presence != nil {
			completed, total := computeTaskProgress(parsedTasks, next.SourceFile)
			tc.presence.Update(discord.PresenceState{
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
		if len(tc.hooks.OnTaskComplete) > 0 {
			event := hooks.Event{
				Type:      "task_complete",
				File:      filepath.Base(next.SourceFile),
				MaggusID:  parser.ParseMaggusID(next.SourceFile),
				Title:     next.Title,
				Action:    "",
				Tasks:     []hooks.TaskInfo{{ID: next.ID, Title: next.Title}},
				Timestamp: time.Now().UTC().Format(time.RFC3339),
			}
			hooks.Run(tc.hooks.OnTaskComplete, event, tc.workDir, log.Default())
		}
	}

	// Update progress to reflect completed iteration.
	// Compute total from refreshed task list so the bar never shrinks when new files appear.
	progressTotal := (i + 1) + countWorkable(parsedTasks)
	if maxCount > 0 && progressTotal > maxCount {
		progressTotal = maxCount
	}
	tc.p.Send(ProgressMsg{Current: i + 1, Total: progressTotal})

	// Between-task sync check: detect if remote changed while working.
	// Skip on the final iteration (push happens next anyway).
	if i < count-1 {
		if syncResult := betweenTaskSync(tc.workCtx, tc.workDir, tc.p); syncResult != nil {
			result.action = taskBreak
			result.stopReason = syncResult.stopReason
			return result
		}
	}

	return result
}

// buildPreCommitFn creates the pre-commit callback injected into RunTaskWorker
// for sequential task execution. It marks completed feature/bug files (renaming
// or deleting them per config), fires feature/bug lifecycle hooks, and stages
// the .maggus/ directory so file renames are included in the commit.
func buildPreCommitFn(tc taskContext) func(workDir string) {
	return func(workDir string) {
		featureAction := tc.onComplete.FeatureAction()
		bugAction := tc.onComplete.BugAction()

		// Snapshot metadata before MarkCompleted renames/deletes files so hook
		// payloads can be built after the file action.
		var featureSnapshots, bugSnapshots []completionSnapshot
		if len(tc.hooks.OnFeatureComplete) > 0 {
			featureSnapshots = snapshotForHooks(tc.featureStore)
		}
		if len(tc.hooks.OnBugComplete) > 0 {
			bugSnapshots = snapshotForHooks(tc.bugStore)
		}

		completedFeatures, _ := tc.featureStore.MarkCompleted(featureAction)
		completedBugs, _ := tc.bugStore.MarkCompleted(bugAction)
		if len(completedFeatures) > 0 || len(completedBugs) > 0 {
			_ = globalconfig.IncrementMetrics(globalconfig.Metrics{
				FeaturesCompleted: int64(len(completedFeatures)),
				BugsCompleted:     int64(len(completedBugs)),
			})
		}

		// Fire lifecycle hooks for completed features/bugs (after file action, before commit).
		fireCompletionHooks(tc, completedFeatures, featureSnapshots, featureAction, "feature_complete", tc.hooks.OnFeatureComplete)
		fireCompletionHooks(tc, completedBugs, bugSnapshots, bugAction, "bug_complete", tc.hooks.OnBugComplete)

		// Stage .maggus/ so file renames/deletions are included in the commit.
		stageFeatures := gitutil.Command("add", "--", ".maggus/")
		stageFeatures.Dir = workDir
		_, _ = stageFeatures.CombinedOutput()
	}
}

// findNextWorkableTask returns the next task to work on, respecting --task flag, worktree
func findNextWorkableTask(tasks []parser.Task) *parser.Task {
	if taskFlag != "" {
		return findTaskByID(tasks, taskFlag)
	}
	return parser.FindNextIncomplete(tasks)
}

// sendIterationStart sends the IterationStartMsg to the TUI with task details.
// When plan is non-nil, its MaggusID and ID are used to populate item-level fields.
func sendIterationStart(p *tea.Program, task *parser.Task, tasks []parser.Task, i, count, featureCurrent, featureTotal int, plan *parser.Plan) {
	tuiCriteria := make([]TaskCriterion, len(task.Criteria))
	for ci, c := range task.Criteria {
		tuiCriteria[ci] = TaskCriterion{
			Text:    c.Text,
			Checked: c.Checked,
			Blocked: c.Blocked,
		}
	}

	// Build remaining tasks list (workable tasks after the current one).
	var remaining []RemainingTask
	pastCurrent := false
	for ti := range tasks {
		if tasks[ti].ID == task.ID {
			pastCurrent = true
			continue
		}
		if pastCurrent && tasks[ti].IsWorkable() {
			remaining = append(remaining, RemainingTask{
				ID:         tasks[ti].ID,
				Title:      tasks[ti].Title,
				SourceFile: filepath.Base(tasks[ti].SourceFile),
			})
		}
	}

	var itemID, itemShort, itemTitle, kind string
	if plan != nil {
		itemID = plan.MaggusID
		itemShort = plan.ID
		itemTitle = parser.ParseFileTitle(plan.File)
		if plan.IsBug {
			kind = "bug"
		} else {
			kind = "feature"
		}
	}

	p.Send(IterationStartMsg{
		Current:         i + 1,
		Total:           count,
		TaskID:          task.ID,
		TaskTitle:       task.Title,
		ItemID:          itemID,
		ItemShort:       itemShort,
		ItemTitle:       itemTitle,
		Kind:            kind,
		TaskDescription: task.Description,
		TaskCriteria:    tuiCriteria,
		RemainingTasks:  remaining,
		FeatureCurrent:  featureCurrent,
		FeatureTotal:    featureTotal,
		TaskModel:       task.Model,
	})
}

// syncBreak is returned by betweenTaskSync when the work loop should stop.
type syncBreak struct {
	stopReason StopReason
}

// betweenTaskSync checks for remote changes between tasks. Returns non-nil if
// the work loop should break (user chose abort or context cancelled).
func betweenTaskSync(ctx context.Context, workDir string, p *tea.Program) *syncBreak {
	if ctx.Err() != nil {
		return &syncBreak{stopReason: StopReasonInterrupted}
	}

	fetchErr := gitsync.FetchRemote(workDir)
	if fetchErr != nil {
		p.Send(InfoMsg{Text: "⚠ Could not reach remote between tasks — continuing offline"})
		return nil
	}

	rs, rsErr := gitsync.RemoteStatus(workDir)
	if rsErr != nil || !rs.HasRemote || rs.Behind == 0 {
		return nil
	}

	// Remote is ahead — show sync TUI and block until resolved.
	resultCh := make(chan SyncCheckResult, 1)
	p.Send(SyncCheckMsg{
		Behind:       rs.Behind,
		Ahead:        rs.Ahead,
		RemoteBranch: rs.RemoteBranch,
		ResultCh:     resultCh,
	})

	select {
	case result := <-resultCh:
		if result.Action == SyncAbort {
			return &syncBreak{stopReason: StopReasonInterrupted}
		}
		if result.Message != "" {
			p.Send(InfoMsg{Text: result.Message})
		}
		return nil
	case <-ctx.Done():
		return &syncBreak{stopReason: StopReasonInterrupted}
	}
}

// completionSnapshot holds pre-mark metadata for a file that may be completed.
// Captured before MarkCompleted renames/deletes the file so hook payloads can
// be built after the file action.
type completionSnapshot struct {
	path     string
	basename string
	maggusID string
	title    string
	tasks    []hooks.TaskInfo
}

// fileGlober is satisfied by FeatureStore and BugStore for snapshotForHooks.
type fileGlober interface {
	GlobFiles(includeCompleted bool) ([]string, error)
}

// snapshotForHooks pre-reads metadata from files that are candidates for completion.
// Only called when hooks are configured, so there is zero overhead otherwise.
func snapshotForHooks(store fileGlober) []completionSnapshot {
	files, _ := store.GlobFiles(false)

	snapshots := make([]completionSnapshot, 0, len(files))
	for _, f := range files {
		tasks, err := parser.ParseFile(f)
		if err != nil || len(tasks) == 0 {
			continue
		}
		allComplete := true
		for _, t := range tasks {
			if !t.IsComplete() || t.IsBlocked() {
				allComplete = false
				break
			}
		}
		if !allComplete {
			continue
		}
		taskInfos := make([]hooks.TaskInfo, len(tasks))
		for i, t := range tasks {
			taskInfos[i] = hooks.TaskInfo{ID: t.ID, Title: t.Title}
		}
		snapshots = append(snapshots, completionSnapshot{
			path:     f,
			basename: filepath.Base(f),
			maggusID: parser.ParseMaggusID(f),
			title:    parser.ParseFileTitle(f),
			tasks:    taskInfos,
		})
	}
	return snapshots
}

// fireCompletionHooks fires hooks for each completed file, using pre-captured snapshots.
// completedPaths are the original paths returned by MarkCompleted*; snapshots hold the
// metadata captured before the file was renamed/deleted.
func fireCompletionHooks(tc taskContext, completedPaths []string, snapshots []completionSnapshot, action, eventType string, commands []config.HookEntry) {
	if len(commands) == 0 || len(completedPaths) == 0 {
		return
	}

	// Index snapshots by original path for O(1) lookup.
	byPath := make(map[string]*completionSnapshot, len(snapshots))
	for i := range snapshots {
		byPath[snapshots[i].path] = &snapshots[i]
	}

	for _, p := range completedPaths {
		snap, ok := byPath[p]
		if !ok {
			continue
		}
		event := hooks.Event{
			Type:      eventType,
			File:      snap.basename,
			MaggusID:  snap.maggusID,
			Title:     snap.title,
			Action:    action,
			Tasks:     snap.tasks,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		}
		hooks.Run(commands, event, tc.workDir, log.Default())
	}
}

// computeTaskProgress counts completed and total tasks scoped to the given source file.
// When taskFlag is set (single-task mode), it returns progress scoped to that one task only.
func computeTaskProgress(tasks []parser.Task, sourceFile string) (completed, total int) {
	if taskFlag != "" {
		// Single-task mode: progress is 0/1 or 1/1.
		for i := range tasks {
			if tasks[i].ID == taskFlag {
				if tasks[i].IsComplete() {
					return 1, 1
				}
				return 0, 1
			}
		}
		return 0, 1
	}

	for i := range tasks {
		if tasks[i].SourceFile != sourceFile {
			continue
		}
		total++
		if tasks[i].IsComplete() {
			completed++
		}
	}
	return completed, total
}

// resolveTaskModel returns the model to use for a task. If the task specifies a
// per-task model override, it is resolved through config.ResolveModel (supporting
// aliases like "opus" → "claude-opus-4-6"). Otherwise the default model is returned.
func resolveTaskModel(taskModel, defaultModel string) string {
	if taskModel != "" {
		return config.ResolveModel(taskModel)
	}
	return defaultModel
}

// verbForTask returns the Discord presence verb based on the task's source file path.
// Bug files (containing /bugs/ or \bugs\) get "Fixing"; everything else gets "Working".
func verbForTask(sourceFile string) string {
	if strings.Contains(sourceFile, "/bugs/") || strings.Contains(sourceFile, `\bugs\`) {
		return "Fixing"
	}
	return "Working"
}
