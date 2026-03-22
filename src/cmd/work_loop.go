package cmd

import (
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/leberkas-org/maggus/internal/config"
	"github.com/leberkas-org/maggus/internal/gitbranch"
	"github.com/leberkas-org/maggus/internal/globalconfig"
	"github.com/leberkas-org/maggus/internal/parser"
	"github.com/leberkas-org/maggus/internal/runner"
	"github.com/leberkas-org/maggus/internal/runtracker"
	"github.com/leberkas-org/maggus/internal/usage"
	"github.com/leberkas-org/maggus/internal/worktree"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
)

// iterationSetup holds everything needed to start the work loop TUI.
type iterationSetup struct {
	tasks   []parser.Task
	next    *parser.Task
	count   int
	run     *runtracker.Run
	workDir string
}

// initIteration parses features and bugs, merges them into a single task list
// (bugs first, then features), finds the next workable task, caps the count,
// and creates a run tracker. Returns nil setup with no error when there is
// nothing to do (e.g. all tasks complete/blocked).
func initIteration(cmd interface{ Println(...interface{}) }, dir, modelDisplay string, count int) (*iterationSetup, error) {
	// Parse bugs first — they take priority over features.
	bugTasks, bugErr := parser.ParseBugs(dir)
	if bugErr != nil {
		return nil, fmt.Errorf("parse bugs: %w", bugErr)
	}

	featureTasks, featureErr := parser.ParseFeatures(dir)
	if featureErr != nil {
		return nil, fmt.Errorf("parse features: %w", featureErr)
	}

	// Merge: bugs first, then features.
	tasks := mergeBugAndFeatureTasks(bugTasks, featureTasks)

	if len(tasks) == 0 {
		cmd.Println("No feature or bug files found.")
		return nil, nil
	}

	_, _ = parser.MarkCompletedFeatures(dir, "")
	_, _ = parser.MarkCompletedBugs(dir, "")

	next, done := findInitialTask(cmd, tasks)
	if done {
		return nil, nil
	}

	count = capCount(tasks, count)

	run, err := runtracker.New(dir, modelDisplay, count)
	if err != nil {
		return nil, fmt.Errorf("create run tracker: %w", err)
	}

	return &iterationSetup{
		tasks:   tasks,
		next:    next,
		count:   count,
		run:     run,
		workDir: dir,
	}, nil
}

// mergeBugAndFeatureTasks returns a combined task list with bugs first, then features.
func mergeBugAndFeatureTasks(bugs, features []parser.Task) []parser.Task {
	tasks := make([]parser.Task, 0, len(bugs)+len(features))
	tasks = append(tasks, bugs...)
	tasks = append(tasks, features...)
	return tasks
}

// parseAllTasks parses both bugs and features and returns a merged task list (bugs first).
func parseAllTasks(dir string) ([]parser.Task, error) {
	bugTasks, bugErr := parser.ParseBugs(dir)
	if bugErr != nil {
		return nil, fmt.Errorf("parse bugs: %w", bugErr)
	}
	featureTasks, featureErr := parser.ParseFeatures(dir)
	if featureErr != nil {
		return nil, fmt.Errorf("parse features: %w", featureErr)
	}
	return mergeBugAndFeatureTasks(bugTasks, featureTasks), nil
}

// printer is satisfied by cobra.Command.
type printer interface {
	Println(...interface{})
	Printf(string, ...interface{})
}

// findInitialTask finds the next task respecting --task flag. Returns (task, done).
// When done is true, the caller should return nil (informational messages already printed).
func findInitialTask(cmd interface{ Println(...interface{}) }, tasks []parser.Task) (*parser.Task, bool) {
	if taskFlag != "" {
		next := findTaskByID(tasks, taskFlag)
		if next == nil {
			cmd.Println(fmt.Sprintf("Task %s not found or already complete.", taskFlag))
			return nil, true
		}
		return next, false
	}

	next := parser.FindNextIncomplete(tasks)
	if next == nil {
		hasIgnored, hasBlocked := false, false
		for i := range tasks {
			if !tasks[i].IsComplete() {
				if tasks[i].Ignored {
					hasIgnored = true
				}
				if tasks[i].IsBlocked() {
					hasBlocked = true
				}
			}
		}
		if hasIgnored || hasBlocked {
			msg := "No workable tasks — remaining tasks are"
			switch {
			case hasBlocked && hasIgnored:
				msg += " blocked or ignored."
			case hasIgnored:
				msg += " ignored."
			default:
				msg += " blocked."
			}
			cmd.Println(msg)
		} else {
			cmd.Println("All tasks are complete! Nothing to do.")
		}
		return nil, true
	}
	return next, false
}

// capCount limits the task count to the number of workable tasks, or 1 when --task is set.
// A count of 0 means "all workable tasks".
func capCount(tasks []parser.Task, count int) int {
	if taskFlag != "" {
		return 1
	}
	workable := 0
	for i := range tasks {
		if tasks[i].IsWorkable() {
			workable++
		}
	}
	if count <= 0 || workable < count {
		return workable
	}
	return count
}

// setupUsageCallback configures the TUI model to record per-task usage.
func setupUsageCallback(m *runner.TUIModel, dir string, run *runtracker.Run, modelDisplay, agentName string) {
	m.SetOnTaskUsage(func(tu runner.TaskUsage) {
		featureRel := tu.FeatureFile
		if rel, err := filepath.Rel(dir, tu.FeatureFile); err == nil {
			featureRel = rel
		}
		_ = usage.Append(dir, []usage.Record{{
			RunID:                    run.ID,
			TaskID:                   tu.TaskID,
			TaskTitle:                tu.TaskTitle,
			FeatureFile:              featureRel,
			Model:                    modelDisplay,
			Agent:                    agentName,
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
}

// workLoopParams bundles the parameters for runWorkGoroutine.
type workLoopParams struct {
	tc            taskContext
	tasks         []parser.Task
	count         int
	run           *runtracker.Run
	p             *tea.Program
	stopFlag      *atomic.Bool
	stopAtTaskID  *atomic.Value // stores string: task ID to stop after (empty = after current)
	includeWarns  []string
	branchMsg     string
	syncInfoMsg   string
	activeAgentNm string
	startHash     string
	modelDisplay  string
	dir           string
}

// runWorkGoroutine runs the work loop in a goroutine, sending TUI events.
// It reports completed count and failed tasks via the returned channels.
func runWorkGoroutine(params workLoopParams) {
	go func() {
		defer func() {
			_ = params.run.Finalize(params.tc.workDir)
			params.p.Send(runner.QuitMsg{})
		}()

		// Send startup info messages.
		if params.activeAgentNm == "claude" {
			params.p.Send(runner.InfoMsg{Text: "⚠ Using --dangerously-skip-permissions (Claude Code)"})
		}
		for _, w := range params.includeWarns {
			params.p.Send(runner.InfoMsg{Text: w})
		}
		if params.branchMsg != "" {
			params.p.Send(runner.InfoMsg{Text: params.branchMsg})
		}
		if params.syncInfoMsg != "" {
			params.p.Send(runner.InfoMsg{Text: params.syncInfoMsg})
		}

		stopReason := runner.StopReasonComplete
		var errorDetail string
		var warnings []string
		var failedTasks []failedTask
		completed := 0

		// Log ignored tasks.
		var ignoredCount int64
		for i := range params.tasks {
			if !params.tasks[i].IsComplete() && params.tasks[i].Ignored {
				params.p.Send(runner.InfoMsg{Text: fmt.Sprintf("Skipping %s: ignored", params.tasks[i].ID)})
				ignoredCount++
			}
		}
		if ignoredCount > 0 {
			_ = globalconfig.IncrementMetrics(globalconfig.Metrics{TasksSkipped: ignoredCount})
		}

		tasks := params.tasks
		var lastCompletedTaskID string
		for i := 0; i < params.count; i++ {
			if i > 0 && params.stopFlag.Load() {
				// Check if we should stop now or continue to a specific task.
				targetID := ""
				if v := params.stopAtTaskID.Load(); v != nil {
					targetID, _ = v.(string)
				}
				if targetID == "" || targetID == lastCompletedTaskID || isTaskAtOrPastTarget(tasks, lastCompletedTaskID, targetID) {
					stopReason = runner.StopReasonUserStop
					break
				}
				// Target task not yet reached — continue working.
			}

			result := runTask(params.tc, tasks, i, params.count)
			if result.taskID != "" {
				lastCompletedTaskID = result.taskID
			}
			if result.tasks != nil {
				tasks = result.tasks
			}
			if result.failed != nil {
				failedTasks = append(failedTasks, *result.failed)
				_ = globalconfig.IncrementMetrics(globalconfig.Metrics{TasksFailed: 1})
			}
			if result.warning != "" {
				warnings = append(warnings, result.warning)
			}
			if result.committed {
				completed++
				_ = globalconfig.IncrementMetrics(globalconfig.Metrics{TasksCompleted: 1})
			}

			switch result.action {
			case taskBreak:
				if result.stopReason != 0 {
					stopReason = result.stopReason
				} else if completed == 0 {
					stopReason = runner.StopReasonNoTasks
				}
			case taskRetry:
				i--
				continue
			case taskSkipToNext:
				continue
			case taskContinue:
				// proceed normally
			}

			if result.action == taskBreak {
				break
			}
		}

		// Determine final stop reason.
		if len(failedTasks) > 0 && stopReason == runner.StopReasonComplete {
			stopReason = runner.StopReasonPartialComplete
		}
		if completed == 0 && stopReason == runner.StopReasonComplete {
			if len(warnings) > 0 {
				stopReason = runner.StopReasonError
				errorDetail = "agent ran but produced no commits"
			} else {
				stopReason = runner.StopReasonNoTasks
				total, done, blocked, ignored := 0, 0, 0, 0
				for i := range tasks {
					total++
					switch {
					case tasks[i].IsComplete():
						done++
					case tasks[i].IsBlocked():
						blocked++
					case tasks[i].Ignored:
						ignored++
					}
				}
				errorDetail = fmt.Sprintf(
					"tasks: %d total, %d complete, %d blocked, %d ignored, count=%d",
					total, done, blocked, ignored, params.count)
			}
		}

		summary := buildSummaryData(params, completed, failedTasks, stopReason, errorDetail, warnings)

		params.tc.notifier.PlayRunComplete()
		params.p.Send(runner.SummaryMsg{Data: summary})

		// Push commits to remote in the background.
		pushToRemote(params.p, params.tc.workDir, completed, summary.Branch)
	}()
}

// buildSummaryData constructs the summary data for the end-of-run summary screen.
func buildSummaryData(params workLoopParams, completed int, failedTasks []failedTask, stopReason runner.StopReason, errorDetail string, warnings []string) runner.SummaryData {
	endHashCmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	endHashCmd.Dir = params.tc.workDir
	endHashBytes, _ := endHashCmd.Output()
	endHash := strings.TrimSpace(string(endHashBytes))

	branchNameCmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	branchNameCmd.Dir = params.tc.workDir
	branchNameOut, _ := branchNameCmd.Output()
	currentBranch := strings.TrimSpace(string(branchNameOut))

	var remaining []runner.RemainingTask
	latestTasks, _ := parseAllTasks(params.tc.workDir)
	for _, t := range latestTasks {
		if t.IsWorkable() {
			remaining = append(remaining, runner.RemainingTask{ID: t.ID, Title: t.Title})
		}
	}

	var runnerFailedTasks []runner.FailedTask
	for _, ft := range failedTasks {
		runnerFailedTasks = append(runnerFailedTasks, runner.FailedTask{ID: ft.ID, Title: ft.Title, Reason: ft.Reason})
	}

	return runner.SummaryData{
		RunID:          params.run.ID,
		Branch:         currentBranch,
		Model:          params.modelDisplay,
		StartTime:      params.run.StartTime,
		TasksCompleted: completed,
		TasksTotal:     params.count,
		CommitStart:    params.startHash,
		CommitEnd:      endHash,
		RemainingTasks: remaining,
		Reason:         stopReason,
		ErrorDetail:    errorDetail,
		Warnings:       warnings,
		FailedTasks:    runnerFailedTasks,
		TasksFailed:    len(failedTasks),
	}
}

// pushToRemote pushes commits to the remote in the background.
func pushToRemote(p *tea.Program, workDir string, completed int, currentBranch string) {
	if completed > 0 {
		var push *exec.Cmd
		if currentBranch != "" {
			push = exec.Command("git", "push", "--set-upstream", "origin", currentBranch)
		} else {
			push = exec.Command("git", "push")
		}
		push.Dir = workDir
		if pushOut, pushErr := push.CombinedOutput(); pushErr != nil {
			p.Send(runner.PushStatusMsg{
				Status: fmt.Sprintf("Push failed: %v", pushErr),
				Done:   true,
			})
		} else {
			msg := fmt.Sprintf("Pushed to origin/%s", currentBranch)
			if s := strings.TrimSpace(string(pushOut)); s != "" {
				_ = s
			}
			p.Send(runner.PushStatusMsg{Status: msg, Done: true})
		}
	} else {
		p.Send(runner.PushStatusMsg{Status: "Nothing to push", Done: true})
	}
}

// captureStartHash gets the current short HEAD hash.
func captureStartHash(workDir string) string {
	cmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	cmd.Dir = workDir
	out, _ := cmd.Output()
	return strings.TrimSpace(string(out))
}

// setupBranch handles worktree creation or feature branch creation.
// Returns the branch message (non-worktree mode) or empty string.
func setupBranch(useWorktree bool, repoDir string, nextTask *parser.Task, run *runtracker.Run, gitCfg config.GitConfig) (string, error) {
	if useWorktree {
		cleanStaleWorktrees(repoDir)
		branchName := gitbranch.BranchName(nextTask.ID)
		wtPath := filepath.Join(repoDir, ".maggus-work", run.ID)
		if err := worktree.Create(repoDir, wtPath, branchName); err != nil {
			return "", fmt.Errorf("create worktree: %w", err)
		}
		return "", nil
	}

	if !gitCfg.IsAutoBranchEnabled() {
		return "Auto-branch disabled, staying on current branch", nil
	}

	_, msg, err := gitbranch.EnsureFeatureBranch(repoDir, nextTask.ID, gitCfg.ProtectedBranchList())
	if err != nil {
		return "", fmt.Errorf("ensure feature branch: %w", err)
	}
	return msg, nil
}

// isTaskAtOrPastTarget returns true if lastCompletedTaskID appears at or after
// targetID in the task list ordering. This handles the case where the target
// task was skipped (blocked/already done) — we stop at the next completed task
// past the target's position in the sequence.
func isTaskAtOrPastTarget(tasks []parser.Task, lastCompletedTaskID, targetID string) bool {
	if lastCompletedTaskID == "" || targetID == "" {
		return false
	}
	targetIdx := -1
	completedIdx := -1
	for i := range tasks {
		if tasks[i].ID == targetID {
			targetIdx = i
		}
		if tasks[i].ID == lastCompletedTaskID {
			completedIdx = i
		}
	}
	if targetIdx < 0 || completedIdx < 0 {
		return false
	}
	return completedIdx >= targetIdx
}
