package cmd

import (
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"os/exec"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/leberkas-org/maggus/internal/approval"
	"github.com/leberkas-org/maggus/internal/config"
	"github.com/leberkas-org/maggus/internal/gitutil"
	"github.com/leberkas-org/maggus/internal/globalconfig"
	"github.com/leberkas-org/maggus/internal/parser"
	"github.com/leberkas-org/maggus/internal/runner"
	"github.com/leberkas-org/maggus/internal/usage"
)

// iterationSetup holds everything needed to start the work loop TUI.
type iterationSetup struct {
	tasks     []parser.Task
	next      *parser.Task
	count     int
	runID     string
	startTime time.Time
	workDir   string
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

	now := time.Now()
	runID := now.Format("20060102-150405")

	return &iterationSetup{
		tasks:     tasks,
		next:      next,
		count:     count,
		runID:     runID,
		startTime: now,
		workDir:   dir,
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
		hasBlocked := false
		for i := range tasks {
			if !tasks[i].IsComplete() && tasks[i].IsBlocked() {
				hasBlocked = true
			}
		}
		if hasBlocked {
			cmd.Println("No workable tasks — remaining tasks are blocked.")
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
	workable := countWorkable(tasks)
	if count <= 0 || workable < count {
		return workable
	}
	return count
}

// countWorkable returns the number of workable (incomplete + not blocked) tasks.
func countWorkable(tasks []parser.Task) int {
	n := 0
	for i := range tasks {
		if tasks[i].IsWorkable() {
			n++
		}
	}
	return n
}

// buildApprovedPlans loads all plans (bugs first, then features), filters by
// approval state, prunes stale approvals, and returns the ordered list.
func buildApprovedPlans(dir string, cfg config.Config) ([]parser.Plan, error) {
	plans, err := parser.LoadPlans(dir, false)
	if err != nil {
		return nil, err
	}

	approvals, err := approval.Load(dir)
	if err != nil {
		return nil, fmt.Errorf("load approvals: %w", err)
	}
	approvalRequired := cfg.IsApprovalRequired()

	// Collect all known IDs for pruning, then filter by approval.
	var knownIDs []string
	var approved []parser.Plan
	for _, p := range plans {
		knownIDs = append(knownIDs, p.ApprovalKey())
		if isPlanApproved(p, approvals, approvalRequired) {
			approved = append(approved, p)
		}
	}

	if err := approval.Prune(dir, knownIDs); err != nil {
		return nil, fmt.Errorf("prune approvals: %w", err)
	}

	return approved, nil
}

// filterTasksBySourceFile returns only tasks whose SourceFile matches file.
func filterTasksBySourceFile(tasks []parser.Task, file string) []parser.Task {
	var result []parser.Task
	for _, t := range tasks {
		if t.SourceFile == file {
			result = append(result, t)
		}
	}
	return result
}

// findGroupForTask finds the first plan containing an incomplete task with the given ID.
func findGroupForTask(plans []parser.Plan, taskID string) *parser.Plan {
	for i := range plans {
		for _, t := range plans[i].Tasks {
			if t.ID == taskID && !t.IsComplete() {
				return &plans[i]
			}
		}
	}
	return nil
}

// firstWorkableTask returns the first workable task from the first plan that has one.
func firstWorkableTask(plans []parser.Plan) *parser.Task {
	for _, p := range plans {
		for i := range p.Tasks {
			if p.Tasks[i].IsWorkable() {
				return &p.Tasks[i]
			}
		}
	}
	return nil
}

// setupUsageCallback configures the TUI model to record per-task usage.
func setupUsageCallback(m *runner.TUIModel, runID string, dir, modelDisplay, agentName string) {
	repoURL := gitutil.RepoURL(dir)
	m.SetOnTaskUsage(func(tu runner.TaskUsage) {
		_ = usage.Append([]usage.Record{{
			RunID:                    runID,
			Repository:               repoURL,
			Kind:                     tu.Kind,
			ItemID:                   tu.ItemID,
			ItemShort:                tu.ItemShort,
			ItemTitle:                tu.ItemTitle,
			TaskShort:                tu.TaskShort,
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
	tasks         []parser.Task // flat merged list — used for summary remaining tasks
	featureGroups []parser.Plan // ordered approved plans to work (bugs first, then features)
	count         int           // number of features to work (0 = determined by autoContinue)
	autoContinue  bool          // from config: continue to next feature after completing one
	runID         string
	startTime     time.Time
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

// runWorkGoroutine runs the feature-centric work loop in a goroutine, sending TUI events.
// It processes one feature group at a time: all tasks in a feature are completed before
// moving to the next. Feature progression is controlled by --count (feature limit) and
// the auto_continue config option.
func runWorkGoroutine(params workLoopParams) {
	go func() {
		defer func() {
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

		// Determine effective feature limit.
		// --count 0 (default) + auto_continue:false (default) → 1 feature
		// --count 0 + auto_continue:true → all features
		// --count N > 0 → N features (explicit, overrides auto_continue)
		effectiveFeatureLimit := params.count
		if effectiveFeatureLimit == 0 && !params.autoContinue {
			effectiveFeatureLimit = 1
		}
		featureUnlimited := effectiveFeatureLimit == 0

		groups := params.featureGroups
		featureTotal := len(groups)

		if featureTotal == 0 {
			params.p.Send(runner.InfoMsg{Text: "No approved features available."})
			summaryParams := params
			summaryParams.count = 0
			summary := buildSummaryData(summaryParams, 0, nil, runner.StopReasonNoTasks, "no approved features", nil)
			params.tc.notifier.PlayRunComplete()
			params.p.Send(runner.SummaryMsg{Data: summary})
			pushToRemote(params.p, params.tc.workDir, 0, summary.Branch)
			return
		}

		stopReason := runner.StopReasonComplete
		var errorDetail string
		var warnings []string
		var failedTasks []failedTask
		totalCompleted := 0
		featuresDone := 0

		for fi, group := range groups {
			if !featureUnlimited && featuresDone >= effectiveFeatureLimit {
				break
			}
			if params.tc.workCtx.Err() != nil {
				stopReason = runner.StopReasonInterrupted
				break
			}

			// Between-feature stop flag check (only after the first feature starts).
			if featuresDone > 0 && params.stopFlag.Load() {
				stopReason = runner.StopReasonUserStop
				break
			}

			// Set feature context in task context for TUI progress display.
			tc := params.tc
			tc.featureSourceFile = group.File
			tc.featureCurrent = fi + 1
			tc.featureTotal = featureTotal

			// Run all tasks in this feature group.
			grResult := runGroupTasks(tc, params, group)
			totalCompleted += grResult.completed
			failedTasks = append(failedTasks, grResult.failed...)
			warnings = append(warnings, grResult.warnings...)

			if grResult.stopped {
				stopReason = grResult.stopReason
				break
			}

			featuresDone++
		}

		// Determine final stop reason.
		if len(failedTasks) > 0 && stopReason == runner.StopReasonComplete {
			stopReason = runner.StopReasonPartialComplete
		}
		if totalCompleted == 0 && stopReason == runner.StopReasonComplete {
			if len(warnings) > 0 {
				stopReason = runner.StopReasonError
				errorDetail = "agent ran but produced no commits"
			} else {
				stopReason = runner.StopReasonNoTasks
			}
		}

		// Use actual task count (completed + failed) as the effective total for the summary.
		summaryParams := params
		summaryParams.count = totalCompleted + len(failedTasks)
		summary := buildSummaryData(summaryParams, totalCompleted, failedTasks, stopReason, errorDetail, warnings)

		params.tc.notifier.PlayRunComplete()
		params.p.Send(runner.SummaryMsg{Data: summary})

		// Push commits to remote in the background.
		pushToRemote(params.p, params.tc.workDir, totalCompleted, summary.Branch)
	}()
}

// groupTasksResult holds the outcome of running all tasks in a feature group.
type groupTasksResult struct {
	completed  int
	failed     []failedTask
	warnings   []string
	stopped    bool
	stopReason runner.StopReason
}

// runGroupTasks runs all workable tasks within a single plan.
// The inner loop mirrors the old task loop but is scoped to one source file.
func runGroupTasks(tc taskContext, params workLoopParams, group parser.Plan) groupTasksResult {
	var result groupTasksResult

	groupTasks := group.Tasks

	if countWorkable(groupTasks) == 0 {
		return result
	}

	// Ensure the plan file has a MaggusID and update the group with it.
	if maggusID, err := parser.EnsureMaggusID(group.File); err == nil {
		group.MaggusID = maggusID
	}
	tc.currentPlan = &group

	tc.logger.FeatureStart(group.ID)

	var lastCompletedTaskID string

	for innerI := 0; ; innerI++ {
		if tc.workCtx.Err() != nil {
			result.stopped = true
			result.stopReason = runner.StopReasonInterrupted
			return result
		}

		// Between-task stop flag check (after first task).
		if innerI > 0 && params.stopFlag.Load() {
			targetID := ""
			if v := params.stopAtTaskID.Load(); v != nil {
				targetID, _ = v.(string)
			}
			if targetID == "" || targetID == lastCompletedTaskID || isTaskAtOrPastTarget(groupTasks, lastCompletedTaskID, targetID) {
				result.stopped = true
				result.stopReason = runner.StopReasonUserStop
				return result
			}
		}

		// Compute display count from remaining workable tasks in this group.
		workableRemaining := countWorkable(groupTasks)
		if workableRemaining == 0 {
			break
		}
		displayCount := innerI + workableRemaining

		taskResult := runTask(tc, groupTasks, innerI, displayCount, 0)
		if taskResult.taskID != "" {
			lastCompletedTaskID = taskResult.taskID
		}

		// Update group task list from re-parsed result, scoped to this source file.
		if taskResult.tasks != nil {
			groupTasks = filterTasksBySourceFile(taskResult.tasks, group.File)
		}

		if taskResult.failed != nil {
			result.failed = append(result.failed, *taskResult.failed)
			_ = globalconfig.IncrementMetrics(globalconfig.Metrics{TasksFailed: 1})
		}
		if taskResult.warning != "" {
			result.warnings = append(result.warnings, taskResult.warning)
		}
		if taskResult.committed {
			result.completed++
			_ = globalconfig.IncrementMetrics(globalconfig.Metrics{TasksCompleted: 1})
		}

		switch taskResult.action {
		case taskBreak:
			if taskResult.stopReason != 0 {
				result.stopped = true
				result.stopReason = taskResult.stopReason
			}
			return result
		case taskRetry:
			innerI--
			continue
		case taskSkipToNext:
			continue
		case taskContinue:
			// proceed normally
		}
	}

	tc.logger.FeatureComplete(group.ID)
	return result
}

// buildSummaryData constructs the summary data for the end-of-run summary screen.
func buildSummaryData(params workLoopParams, completed int, failedTasks []failedTask, stopReason runner.StopReason, errorDetail string, warnings []string) runner.SummaryData {
	endHashCmd := gitutil.Command("rev-parse", "--short", "HEAD")
	endHashCmd.Dir = params.tc.workDir
	endHashBytes, _ := endHashCmd.Output()
	endHash := strings.TrimSpace(string(endHashBytes))

	branchNameCmd := gitutil.Command("rev-parse", "--abbrev-ref", "HEAD")
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
		RunID:          params.runID,
		Branch:         currentBranch,
		Model:          params.modelDisplay,
		StartTime:      params.startTime,
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
			push = gitutil.Command("push", "--set-upstream", "origin", currentBranch)
		} else {
			push = gitutil.Command("push")
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
	return captureShortHash(workDir)
}

// captureShortHash returns the current short HEAD git hash, or empty string on error.
func captureShortHash(workDir string) string {
	cmd := gitutil.Command("rev-parse", "--short", "HEAD")
	cmd.Dir = workDir
	out, _ := cmd.Output()
	return strings.TrimSpace(string(out))
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
