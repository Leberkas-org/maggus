package cmd

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/leberkas-org/maggus/internal/agent"
	"github.com/leberkas-org/maggus/internal/config"
	"github.com/leberkas-org/maggus/internal/discord"
	"github.com/leberkas-org/maggus/internal/gitcommit"
	"github.com/leberkas-org/maggus/internal/gitsync"
	"github.com/leberkas-org/maggus/internal/gitutil"
	"github.com/leberkas-org/maggus/internal/globalconfig"
	"github.com/leberkas-org/maggus/internal/hooks"
	"github.com/leberkas-org/maggus/internal/notify"
	"github.com/leberkas-org/maggus/internal/parser"
	"github.com/leberkas-org/maggus/internal/prompt"
	"github.com/leberkas-org/maggus/internal/runlog"
	"github.com/leberkas-org/maggus/internal/runner"
	"github.com/leberkas-org/maggus/internal/tasklock"
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
	stopReason runner.StopReason // only set when action == taskBreak
	committed  bool              // true if a commit was made
	warning    string            // non-empty if commit succeeded but with a caveat
	failed     *failedTask       // non-nil if the task failed
	tasks      []parser.Task     // updated task list after re-parse (nil if unchanged)
	taskID     string            // ID of the task that was worked on
}

// taskContext bundles the shared state needed by runTask.
type taskContext struct {
	workCtx       context.Context
	p             *tea.Program
	activeAgent   agent.Agent
	resolvedModel string
	notifier      *notify.Notifier
	validIncludes []string
	useWorktree   bool
	repoDir       string
	workDir       string
	runID         string
	onComplete    config.OnCompleteConfig
	hooks         config.HooksConfig
	logger        *runlog.Logger // structured run log; nil-safe

	// Discord Rich Presence (nil when disabled).
	presence *discord.Presence

	// Feature-centric context (set per-group by runWorkGoroutine).
	featureSourceFile string // scope parsedTasks to this source file for progress calculation
	featureCurrent    int    // 1-based index of current feature (for TUI display)
	featureTotal      int    // total features being processed (for TUI display)
}

// runTask executes a single task iteration: finds the next task, acquires a
// lock (in worktree mode), builds the prompt, runs the agent, re-parses features,
// marks completed features, stages renames, commits, and handles the result.
//
// The caller's loop index (i) and total count are needed for progress tracking
// and prompt metadata. The tasks slice is the current parsed task list.
// maxCount is the user-requested task limit (0 = unlimited; used to cap progress total).
func runTask(tc taskContext, tasks []parser.Task, i, count, maxCount int) taskResult {
	if tc.workCtx.Err() != nil {
		return taskResult{action: taskBreak, stopReason: runner.StopReasonInterrupted}
	}

	// Find next workable task.
	next := findNextWorkableTask(tasks, tc.useWorktree, tc.repoDir)
	if next == nil {
		return taskResult{action: taskBreak}
	}

	// Acquire task lock in worktree mode.
	var lock tasklock.Lock
	if tc.useWorktree {
		var lockErr error
		lock, lockErr = tasklock.Acquire(tc.repoDir, next.ID, tc.runID)
		if lockErr != nil {
			// Another session grabbed it between check and acquire; retry.
			return taskResult{action: taskRetry}
		}
	}

	// Signal iteration start to the TUI.
	sendIterationStart(tc.p, next, tasks, i, count, tc.featureCurrent, tc.featureTotal)

	// Update Discord Rich Presence with current task info.
	if tc.presence != nil {
		tc.presence.Update(discord.PresenceState{
			TaskID:       next.ID,
			TaskTitle:    next.Title,
			FeatureTitle: parser.ParseFileTitle(next.SourceFile),
			StartTime:    time.Now(),
		})
	}

	// Build and run the prompt.
	opts := prompt.Options{
		NoBootstrap: noBootstrapFlag,
		Include:     tc.validIncludes,
		RunID:       tc.runID,
		Iteration:   i + 1,
	}

	tc.logger.TaskStart(next.ID, next.Title)
	builtPrompt := prompt.Build(next, opts)
	if err := tc.activeAgent.Run(tc.workCtx, builtPrompt, tc.resolvedModel, tc.p); err != nil {
		releaseLock(lock, tc.useWorktree)
		if tc.workCtx.Err() != nil {
			return taskResult{action: taskBreak, stopReason: runner.StopReasonInterrupted}
		}
		tc.notifier.PlayError()
		_ = globalconfig.IncrementMetrics(globalconfig.Metrics{AgentErrors: 1})
		reason := err.Error()
		tc.logger.TaskFailed(next.ID, reason)
		tc.p.Send(runner.InfoMsg{Text: fmt.Sprintf("✗ %s failed: %s — skipping to next task", next.ID, reason)})
		return taskResult{
			action: taskSkipToNext,
			taskID: next.ID,
			failed: &failedTask{ID: next.ID, Title: next.Title, Reason: reason},
		}
	}

	// Re-parse to pick up any changes the agent made (bugs + features).
	parsedTasks, parseErr := parseAllTasks(tc.workDir)
	if parseErr != nil {
		releaseLock(lock, tc.useWorktree)
		reason := fmt.Sprintf("re-parse tasks: %v", parseErr)
		return taskResult{
			action: taskSkipToNext,
			taskID: next.ID,
			failed: &failedTask{ID: next.ID, Title: next.Title, Reason: reason},
		}
	}

	// Rename or delete fully completed feature and bug files before committing.
	// When hooks are configured, snapshot file metadata before the mark operation
	// since the files may be renamed or deleted.
	featureAction := tc.onComplete.FeatureAction()
	bugAction := tc.onComplete.BugAction()
	var featureSnapshots, bugSnapshots []completionSnapshot
	if len(tc.hooks.OnFeatureComplete) > 0 {
		featureSnapshots = snapshotForHooks(tc.workDir, true)
	}
	if len(tc.hooks.OnBugComplete) > 0 {
		bugSnapshots = snapshotForHooks(tc.workDir, false)
	}

	completedFeatures, _ := parser.MarkCompletedFeatures(tc.workDir, featureAction)
	completedBugs, _ := parser.MarkCompletedBugs(tc.workDir, bugAction)
	if len(completedFeatures) > 0 || len(completedBugs) > 0 {
		_ = globalconfig.IncrementMetrics(globalconfig.Metrics{
			FeaturesCompleted: int64(len(completedFeatures)),
			BugsCompleted:     int64(len(completedBugs)),
		})
	}

	// Fire lifecycle hooks for completed features/bugs (after file action, before git add).
	fireCompletionHooks(tc, completedFeatures, featureSnapshots, featureAction, "feature_complete", tc.hooks.OnFeatureComplete)
	fireCompletionHooks(tc, completedBugs, bugSnapshots, bugAction, "bug_complete", tc.hooks.OnBugComplete)

	// Stage any feature renames so they are included in the commit.
	stageFeatures := gitutil.Command("add", "--", ".maggus/")
	stageFeatures.Dir = tc.workDir
	_, _ = stageFeatures.CombinedOutput()

	// Scope parsedTasks to the current feature source file when in feature mode.
	// This ensures progress calculation and result.tasks reflect only this feature's tasks.
	scopedTasks := parsedTasks
	if tc.featureSourceFile != "" {
		scopedTasks = filterTasksBySourceFile(parsedTasks, tc.featureSourceFile)
	}

	// Commit, release lock, update progress, and check sync.
	result := completeTask(tc, next, lock, scopedTasks, i, count, maxCount)
	result.taskID = next.ID
	return result
}

// completeTask encapsulates post-agent-execution logic: committing via COMMIT.md,
// releasing the task lock, sending progress updates, and running between-task sync checks.
// It returns a taskResult indicating whether the loop should continue, break, or skip.
// maxCount is the user-requested task limit; when >0 the computed progress total is capped at it.
func completeTask(tc taskContext, task *parser.Task, lock tasklock.Lock, parsedTasks []parser.Task, i, count, maxCount int) taskResult {
	// Commit using COMMIT.md.
	commitResult, commitErr := gitcommit.CommitIteration(tc.workDir, task.ID+": "+task.Title)
	if commitErr != nil {
		releaseLock(lock, tc.useWorktree)
		reason := commitErr.Error()
		tc.logger.TaskFailed(task.ID, reason)
		tc.p.Send(runner.InfoMsg{Text: fmt.Sprintf("✗ %s commit failed: %s — skipping to next task", task.ID, reason)})
		return taskResult{
			action: taskSkipToNext,
			tasks:  parsedTasks,
			failed: &failedTask{ID: task.ID, Title: task.Title, Reason: reason},
		}
	}

	releaseLock(lock, tc.useWorktree)

	result := taskResult{
		action: taskContinue,
		tasks:  parsedTasks,
	}

	if commitResult.Committed {
		_ = globalconfig.IncrementMetrics(globalconfig.Metrics{GitCommits: 1})
		commitHash := captureShortHash(tc.workDir)
		tc.logger.TaskComplete(task.ID, commitHash)
		tc.p.Send(runner.CommitMsg{Message: commitResult.Message})
		tc.notifier.PlayTaskComplete()
		result.committed = true

		// Fire task completion hooks (zero overhead when unconfigured).
		if len(tc.hooks.OnTaskComplete) > 0 {
			event := hooks.Event{
				Type:      "task_complete",
				File:      filepath.Base(task.SourceFile),
				MaggusID:  parser.ParseMaggusID(task.SourceFile),
				Title:     task.Title,
				Action:    "",
				Tasks:     []hooks.TaskInfo{{ID: task.ID, Title: task.Title}},
				Timestamp: time.Now().UTC().Format(time.RFC3339),
			}
			hooks.Run(tc.hooks.OnTaskComplete, event, tc.workDir, log.Default())
		}
	} else {
		msg := commitResult.Message
		if msg == "" {
			msg = "commit skipped (unknown reason)"
		}
		result.warning = fmt.Sprintf("%s: %s", task.ID, msg)
		tc.p.Send(runner.InfoMsg{Text: "⚠ " + result.warning})
	}

	// Update progress to reflect completed iteration.
	// Compute total from refreshed task list so the bar never shrinks when new files appear.
	progressTotal := (i + 1) + countWorkable(parsedTasks)
	if maxCount > 0 && progressTotal > maxCount {
		progressTotal = maxCount
	}
	tc.p.Send(runner.ProgressMsg{Current: i + 1, Total: progressTotal})

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

// findNextWorkableTask returns the next task to work on, respecting --task flag, worktree
// mode locking, and standard incomplete task ordering.
func findNextWorkableTask(tasks []parser.Task, useWorktree bool, repoDir string) *parser.Task {
	if taskFlag != "" {
		return findTaskByID(tasks, taskFlag)
	}
	if useWorktree {
		return findNextUnlocked(tasks, repoDir)
	}
	return parser.FindNextIncomplete(tasks)
}

// sendIterationStart sends the IterationStartMsg to the TUI with task details.
func sendIterationStart(p *tea.Program, task *parser.Task, tasks []parser.Task, i, count, featureCurrent, featureTotal int) {
	tuiCriteria := make([]runner.TaskCriterion, len(task.Criteria))
	for ci, c := range task.Criteria {
		tuiCriteria[ci] = runner.TaskCriterion{
			Text:    c.Text,
			Checked: c.Checked,
			Blocked: c.Blocked,
		}
	}

	// Build remaining tasks list (workable tasks after the current one).
	var remaining []runner.RemainingTask
	pastCurrent := false
	for ti := range tasks {
		if tasks[ti].ID == task.ID {
			pastCurrent = true
			continue
		}
		if pastCurrent && tasks[ti].IsWorkable() {
			remaining = append(remaining, runner.RemainingTask{
				ID:         tasks[ti].ID,
				Title:      tasks[ti].Title,
				SourceFile: filepath.Base(tasks[ti].SourceFile),
			})
		}
	}

	p.Send(runner.IterationStartMsg{
		Current:         i + 1,
		Total:           count,
		TaskID:          task.ID,
		TaskTitle:       task.Title,
		FeatureFile:     task.SourceFile,
		TaskDescription: task.Description,
		TaskCriteria:    tuiCriteria,
		RemainingTasks:  remaining,
		FeatureCurrent:  featureCurrent,
		FeatureTotal:    featureTotal,
	})
}

// releaseLock releases a task lock if worktree mode is active.
func releaseLock(lock tasklock.Lock, useWorktree bool) {
	if useWorktree {
		lock.Release()
	}
}

// syncBreak is returned by betweenTaskSync when the work loop should stop.
type syncBreak struct {
	stopReason runner.StopReason
}

// betweenTaskSync checks for remote changes between tasks. Returns non-nil if
// the work loop should break (user chose abort or context cancelled).
func betweenTaskSync(ctx context.Context, workDir string, p *tea.Program) *syncBreak {
	if ctx.Err() != nil {
		return &syncBreak{stopReason: runner.StopReasonInterrupted}
	}

	fetchErr := gitsync.FetchRemote(workDir)
	if fetchErr != nil {
		p.Send(runner.InfoMsg{Text: "⚠ Could not reach remote between tasks — continuing offline"})
		return nil
	}

	rs, rsErr := gitsync.RemoteStatus(workDir)
	if rsErr != nil || !rs.HasRemote || rs.Behind == 0 {
		return nil
	}

	// Remote is ahead — show sync TUI and block until resolved.
	resultCh := make(chan runner.SyncCheckResult, 1)
	p.Send(runner.SyncCheckMsg{
		Behind:       rs.Behind,
		Ahead:        rs.Ahead,
		RemoteBranch: rs.RemoteBranch,
		ResultCh:     resultCh,
	})

	select {
	case result := <-resultCh:
		if result.Action == runner.SyncAbort {
			return &syncBreak{stopReason: runner.StopReasonInterrupted}
		}
		if result.Message != "" {
			p.Send(runner.InfoMsg{Text: result.Message})
		}
		return nil
	case <-ctx.Done():
		return &syncBreak{stopReason: runner.StopReasonInterrupted}
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

// snapshotForHooks pre-reads metadata from files that are candidates for completion.
// Only called when hooks are configured, so there is zero overhead otherwise.
// isFeature selects feature vs bug file globbing.
func snapshotForHooks(workDir string, isFeature bool) []completionSnapshot {
	var files []string
	if isFeature {
		files, _ = parser.GlobFeatureFiles(workDir, false)
	} else {
		files, _ = parser.GlobBugFiles(workDir, false)
	}

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
