package cmd

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/leberkas-org/maggus/internal/agent"
	"github.com/leberkas-org/maggus/internal/gitcommit"
	"github.com/leberkas-org/maggus/internal/gitsync"
	"github.com/leberkas-org/maggus/internal/notify"
	"github.com/leberkas-org/maggus/internal/parser"
	"github.com/leberkas-org/maggus/internal/prompt"
	"github.com/leberkas-org/maggus/internal/runner"
	"github.com/leberkas-org/maggus/internal/runtracker"
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
	run           *runtracker.Run
	activeAgent   agent.Agent
	resolvedModel string
	notifier      *notify.Notifier
	validIncludes []string
	useWorktree   bool
	repoDir       string
	workDir       string
	runID         string
}

// runTask executes a single task iteration: finds the next task, acquires a
// lock (in worktree mode), builds the prompt, runs the agent, re-parses features,
// marks completed features, stages renames, commits, and handles the result.
//
// The caller's loop index (i) and total count are needed for progress tracking
// and prompt metadata. The tasks slice is the current parsed task list.
func runTask(tc taskContext, tasks []parser.Task, i, count int) taskResult {
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
	sendIterationStart(tc.p, next, tasks, i, count)

	// Build and run the prompt.
	opts := prompt.Options{
		NoBootstrap: noBootstrapFlag,
		Include:     tc.validIncludes,
		RunID:       tc.runID,
		RunDir:      tc.run.RelativeDir(tc.workDir),
		Iteration:   i + 1,
		IterLog:     tc.run.RelativeIterationLogPath(tc.workDir, i+1),
	}

	builtPrompt := prompt.Build(next, opts)
	if err := tc.activeAgent.Run(tc.workCtx, builtPrompt, tc.resolvedModel, tc.p); err != nil {
		releaseLock(lock, tc.useWorktree)
		if tc.workCtx.Err() != nil {
			return taskResult{action: taskBreak, stopReason: runner.StopReasonInterrupted}
		}
		tc.notifier.PlayError()
		reason := err.Error()
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

	// Rename fully completed feature and bug files before committing.
	_ = parser.MarkCompletedFeatures(tc.workDir)
	_ = parser.MarkCompletedBugs(tc.workDir)

	// Stage any feature renames so they are included in the commit.
	stageFeatures := exec.Command("git", "add", "--", ".maggus/")
	stageFeatures.Dir = tc.workDir
	_, _ = stageFeatures.CombinedOutput()

	// Commit, release lock, update progress, and check sync.
	result := completeTask(tc, next, lock, parsedTasks, i, count)
	result.taskID = next.ID
	return result
}

// completeTask encapsulates post-agent-execution logic: committing via COMMIT.md,
// releasing the task lock, sending progress updates, and running between-task sync checks.
// It returns a taskResult indicating whether the loop should continue, break, or skip.
func completeTask(tc taskContext, task *parser.Task, lock tasklock.Lock, parsedTasks []parser.Task, i, count int) taskResult {
	// Commit using COMMIT.md.
	commitResult, commitErr := gitcommit.CommitIteration(tc.workDir, task.ID+": "+task.Title)
	if commitErr != nil {
		releaseLock(lock, tc.useWorktree)
		reason := commitErr.Error()
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
		tc.p.Send(runner.CommitMsg{Message: commitResult.Message})
		tc.notifier.PlayTaskComplete()
		result.committed = true
	} else {
		msg := commitResult.Message
		if msg == "" {
			msg = "commit skipped (unknown reason)"
		}
		result.warning = fmt.Sprintf("%s: %s", task.ID, msg)
		tc.p.Send(runner.InfoMsg{Text: "⚠ " + result.warning})
	}

	// Update progress to reflect completed iteration.
	tc.p.Send(runner.ProgressMsg{Current: i + 1, Total: count})

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
func sendIterationStart(p *tea.Program, task *parser.Task, tasks []parser.Task, i, count int) {
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
