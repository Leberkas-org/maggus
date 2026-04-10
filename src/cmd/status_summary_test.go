package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leberkas-org/maggus/internal/parser"
	"github.com/leberkas-org/maggus/internal/runlog"
)

// ── renderSummaryTab dispatch ──────────────────────────────────────────────────

// TestRenderSummaryTab_DispatchesToFeatureSummaryForSelFeature verifies that
// renderSummaryTab delegates to renderFeatureSummary when a feature plan is selected.
// The feature title/ID should appear in the output.
func TestRenderSummaryTab_DispatchesToFeatureSummaryForSelFeature(t *testing.T) {
	m := makeModelForCtx(selFeature)
	// makeModelForCtx sets plan ID "f1"
	out := m.renderSummaryTab(80, 20)
	if !strings.Contains(out, "f1") {
		t.Errorf("renderSummaryTab for selFeature should contain plan ID 'f1'; got: %q", out)
	}
}

// TestRenderSummaryTab_DispatchesToTaskSummaryForSelCompletedTask verifies that
// renderSummaryTab delegates to renderTaskSummary when a completed task is selected.
// The task ID should appear in the output.
func TestRenderSummaryTab_DispatchesToTaskSummaryForSelCompletedTask(t *testing.T) {
	m := makeModelForCtx(selCompletedTask)
	// makeModelForCtx creates task with ID "TASK-001"
	out := m.renderSummaryTab(80, 20)
	if !strings.Contains(out, "TASK-001") {
		t.Errorf("renderSummaryTab for selCompletedTask should contain task ID 'TASK-001'; got: %q", out)
	}
}

// TestRenderSummaryTab_ReturnsEmptyForSelNone verifies that renderSummaryTab returns
// a blank string (no meaningful content) when called defensively for selNone.
// This context does not include Summary in availableTabs(), so this is a safety check.
func TestRenderSummaryTab_ReturnsEmptyForSelNone(t *testing.T) {
	m := makeModelForCtx(selNone)
	out := m.renderSummaryTab(80, 20)
	// Should not render any feature/task-specific content
	if strings.Contains(out, "TASK-") || strings.Contains(out, "f1") {
		t.Errorf("renderSummaryTab for selNone should not contain feature/task content; got: %q", out)
	}
}

// TestRenderSummaryTab_ReturnsEmptyForSelRunningTask verifies that renderSummaryTab
// returns blank content when called defensively for selRunningTask.
func TestRenderSummaryTab_ReturnsEmptyForSelRunningTask(t *testing.T) {
	m := makeModelForCtx(selRunningTask)
	out := m.renderSummaryTab(80, 20)
	// renderSummaryTab falls through to default (empty) for selRunningTask
	if strings.Contains(out, "Status") || strings.Contains(out, "Duration") {
		t.Errorf("renderSummaryTab for selRunningTask should not contain task summary rows; got: %q", out)
	}
}

// TestAvailableTabs_SummaryNotInSelNone verifies that Summary tab is never included
// for selNone, enforcing the tab mapping constraint.
func TestAvailableTabs_SummaryNotInSelNone(t *testing.T) {
	m := makeModelForCtx(selNone)
	for _, td := range m.availableTabs() {
		if td.key == "summary" {
			t.Errorf("availableTabs for selNone should not contain summary tab")
		}
	}
}

// TestAvailableTabs_SummaryNotInSelRunningTask verifies that Summary tab is never
// included for selRunningTask, enforcing the tab mapping constraint.
func TestAvailableTabs_SummaryNotInSelRunningTask(t *testing.T) {
	m := makeModelForCtx(selRunningTask)
	for _, td := range m.availableTabs() {
		if td.key == "summary" {
			t.Errorf("availableTabs for selRunningTask should not contain summary tab")
		}
	}
}

// ── renderFeatureSummary ───────────────────────────────────────────────────────

// TestRenderFeatureSummary_ShowsTitleAndFilename verifies that the feature title
// and filename base appear in the Summary tab output for a feature selection.
func TestRenderFeatureSummary_ShowsTitleAndFilename(t *testing.T) {
	m := statusModel{
		expandedPlans: make(map[string]bool),
		plans: []parser.Plan{
			{ID: "feature_010", Title: "My Great Feature", File: "feature_010.md"},
		},
		treeCursor: 0,
		width:      120,
		height:     40,
	}
	out := m.renderFeatureSummary(80, 20)
	if !strings.Contains(out, "My Great Feature") {
		t.Errorf("renderFeatureSummary should contain the plan title; got: %q", out)
	}
	if !strings.Contains(out, "feature_010.md") {
		t.Errorf("renderFeatureSummary should contain the plan filename; got: %q", out)
	}
}

// TestRenderFeatureSummary_FallsBackToPlanIDWhenNoTitle verifies that when a plan has
// no Title, the plan ID is used as the header instead.
func TestRenderFeatureSummary_FallsBackToPlanIDWhenNoTitle(t *testing.T) {
	m := statusModel{
		expandedPlans: make(map[string]bool),
		plans: []parser.Plan{
			{ID: "feature_020", Title: "", File: "feature_020.md"},
		},
		treeCursor: 0,
		width:      120,
		height:     40,
	}
	out := m.renderFeatureSummary(80, 20)
	if !strings.Contains(out, "feature_020") {
		t.Errorf("renderFeatureSummary should fall back to plan ID when title is empty; got: %q", out)
	}
}

// TestRenderFeatureSummary_ShowsProgressBarWithDoneTotal verifies that the progress
// bar area includes the done/total count when tasks exist.
func TestRenderFeatureSummary_ShowsProgressBarWithDoneTotal(t *testing.T) {
	m := statusModel{
		expandedPlans: make(map[string]bool),
		plans: []parser.Plan{
			{
				ID:    "feature_030",
				File:  "feature_030.md",
				Tasks: []parser.Task{
					{ID: "TASK-001", Criteria: []parser.Criterion{{Checked: true, Text: "done"}}},
					{ID: "TASK-002", Criteria: []parser.Criterion{{Checked: false, Text: "pending"}}},
					{ID: "TASK-003", Criteria: []parser.Criterion{{Checked: false, Text: "pending"}}},
				},
			},
		},
		treeCursor: 0,
		width:      120,
		height:     40,
	}
	out := m.renderFeatureSummary(80, 20)
	// progress bar shows "1/3"
	if !strings.Contains(out, "1/3") {
		t.Errorf("renderFeatureSummary should show done/total as '1/3'; got: %q", out)
	}
}

// TestRenderFeatureSummary_ShowsTaskBreakdown verifies that Done, Pending, and Blocked
// counts appear in the summary output.
func TestRenderFeatureSummary_ShowsTaskBreakdown(t *testing.T) {
	m := statusModel{
		expandedPlans: make(map[string]bool),
		plans: []parser.Plan{
			{
				ID:   "feature_040",
				File: "feature_040.md",
				Tasks: []parser.Task{
					// 1 done
					{ID: "TASK-001", Criteria: []parser.Criterion{{Checked: true, Text: "done"}}},
					// 1 pending (not done, not blocked)
					{ID: "TASK-002", Criteria: []parser.Criterion{{Checked: false, Text: "do work"}}},
					// 1 blocked (Blocked field must be set explicitly)
					{ID: "TASK-003", Criteria: []parser.Criterion{{Checked: false, Text: "BLOCKED: waiting", Blocked: true}}},
				},
			},
		},
		treeCursor: 0,
		width:      120,
		height:     40,
	}
	out := m.renderFeatureSummary(80, 20)
	if !strings.Contains(out, "Done") {
		t.Errorf("renderFeatureSummary should contain 'Done' label; got: %q", out)
	}
	if !strings.Contains(out, "Pending") {
		t.Errorf("renderFeatureSummary should contain 'Pending' label; got: %q", out)
	}
	if !strings.Contains(out, "Blocked") {
		t.Errorf("renderFeatureSummary should contain 'Blocked' label; got: %q", out)
	}
}

// TestRenderFeatureSummary_ShowsDaemonStateWhenRunningOnFeature verifies that the
// active task ID and spinner appear when the daemon is working on a task in the plan.
func TestRenderFeatureSummary_ShowsDaemonStateWhenRunningOnFeature(t *testing.T) {
	m := statusModel{
		expandedPlans: make(map[string]bool),
		plans: []parser.Plan{
			{
				ID:    "feature_050",
				File:  "feature_050.md",
				Tasks: []parser.Task{
					{ID: "TASK-001", Criteria: []parser.Criterion{{Checked: false, Text: "do work"}}},
				},
			},
		},
		treeCursor: 0,
		width:      120,
		height:     40,
		daemon:     daemonStatus{Running: true, CurrentTask: "TASK-001"},
	}
	out := m.renderFeatureSummary(80, 20)
	if !strings.Contains(out, "TASK-001") {
		t.Errorf("renderFeatureSummary should show active task ID 'TASK-001' when daemon is running; got: %q", out)
	}
}

// TestRenderFeatureSummary_NoDaemonStateWhenIdle verifies that no daemon-specific
// indicators appear when the daemon is not running.
func TestRenderFeatureSummary_NoDaemonStateWhenIdle(t *testing.T) {
	m := statusModel{
		expandedPlans: make(map[string]bool),
		plans: []parser.Plan{
			{
				ID:    "feature_051",
				File:  "feature_051.md",
				Tasks: []parser.Task{
					{ID: "TASK-001", Criteria: []parser.Criterion{{Checked: false}}},
				},
			},
		},
		treeCursor: 0,
		width:      120,
		height:     40,
		daemon:     daemonStatus{Running: false},
	}
	out := m.renderFeatureSummary(80, 20)
	// When daemon is not running, TASK-001 should not appear as a running indicator.
	// (It may appear in task breakdown section via other logic, but the spinner section is absent.)
	// We verify no "Workers" header appears (that requires running workers).
	if strings.Contains(out, "Workers") {
		t.Errorf("renderFeatureSummary should not show Workers section when daemon is idle; got: %q", out)
	}
}

// TestRenderFeatureSummary_ShowsParallelWorkers verifies that the Workers section
// appears when workerIndex entries belong to the selected plan.
func TestRenderFeatureSummary_ShowsParallelWorkers(t *testing.T) {
	m := statusModel{
		expandedPlans: make(map[string]bool),
		plans: []parser.Plan{
			{
				ID:    "feature_060",
				File:  "feature_060.md",
				Tasks: []parser.Task{
					{ID: "TASK-001"},
					{ID: "TASK-002"},
				},
			},
		},
		treeCursor: 0,
		width:      120,
		height:     40,
		workerIndex: []runlog.WorkerIndexEntry{
			{TaskID: "TASK-001", Status: "working", TaskTitle: "First task"},
			{TaskID: "TASK-002", Status: "waiting", TaskTitle: "Second task"},
		},
		workerSpinners: map[string]int{"TASK-001": 0},
	}
	out := m.renderFeatureSummary(80, 20)
	if !strings.Contains(out, "Workers") {
		t.Errorf("renderFeatureSummary should show Workers section when workerIndex is populated; got: %q", out)
	}
	if !strings.Contains(out, "TASK-001") {
		t.Errorf("renderFeatureSummary should show worker task ID 'TASK-001'; got: %q", out)
	}
}

// TestRenderFeatureSummary_WorkersOnlyFromThisPlan verifies that workers from other
// plans are not shown in the feature summary.
func TestRenderFeatureSummary_WorkersOnlyFromThisPlan(t *testing.T) {
	m := statusModel{
		expandedPlans: make(map[string]bool),
		plans: []parser.Plan{
			{
				ID:    "feature_070",
				File:  "feature_070.md",
				Tasks: []parser.Task{{ID: "TASK-001"}},
			},
		},
		treeCursor: 0,
		width:      120,
		height:     40,
		workerIndex: []runlog.WorkerIndexEntry{
			{TaskID: "TASK-999", Status: "working", TaskTitle: "Other feature task"},
		},
		workerSpinners: map[string]int{},
	}
	out := m.renderFeatureSummary(80, 20)
	if strings.Contains(out, "Workers") {
		t.Errorf("renderFeatureSummary should not show Workers section when workers are from a different plan; got: %q", out)
	}
}

// TestRenderFeatureSummary_ShowsAggregateMetrics verifies that Tokens and Cost labels
// appear when cachedFeatureMetrics has non-zero data.
func TestRenderFeatureSummary_ShowsAggregateMetrics(t *testing.T) {
	m := statusModel{
		expandedPlans: make(map[string]bool),
		plans: []parser.Plan{
			{ID: "feature_080", File: "feature_080.md"},
		},
		treeCursor:           0,
		width:                120,
		height:               40,
		cachedFeatureMetrics: featureMetrics{totalTokens: 12345, totalCostUSD: 0.42},
	}
	out := m.renderFeatureSummary(80, 20)
	if !strings.Contains(out, "Tokens") {
		t.Errorf("renderFeatureSummary should show 'Tokens' label when metrics exist; got: %q", out)
	}
	if !strings.Contains(out, "Cost") {
		t.Errorf("renderFeatureSummary should show 'Cost' label when metrics exist; got: %q", out)
	}
}

// TestRenderFeatureSummary_NoMetricsSectionWhenZero verifies that the Tokens/Cost
// section is omitted when there are no cached metrics.
func TestRenderFeatureSummary_NoMetricsSectionWhenZero(t *testing.T) {
	m := statusModel{
		expandedPlans: make(map[string]bool),
		plans: []parser.Plan{
			{ID: "feature_090", File: "feature_090.md"},
		},
		treeCursor:           0,
		width:                120,
		height:               40,
		cachedFeatureMetrics: featureMetrics{}, // zero metrics
	}
	out := m.renderFeatureSummary(80, 20)
	if strings.Contains(out, "Tokens") {
		t.Errorf("renderFeatureSummary should omit 'Tokens' section when metrics are zero; got: %q", out)
	}
}

// ── renderTaskSummary ──────────────────────────────────────────────────────────

// TestRenderTaskSummary_ShowsTaskIDAndTitle verifies that the selected task's ID
// and title appear in the Summary tab output.
func TestRenderTaskSummary_ShowsTaskIDAndTitle(t *testing.T) {
	task := parser.Task{
		ID:       "TASK-042",
		Title:    "Implement awesome feature",
		Criteria: []parser.Criterion{{Checked: true, Text: "done"}},
	}
	m := statusModel{
		expandedPlans: make(map[string]bool),
		plans: []parser.Plan{
			{ID: "f1", File: "f1.md", Tasks: []parser.Task{task}},
		},
		treeCursor: 1,
		showAll:               true,
		width:                 120,
		height:                40,
	}
	m.expandedPlans["f1"] = true
	out := m.renderTaskSummary(80, 20)
	if !strings.Contains(out, "TASK-042") {
		t.Errorf("renderTaskSummary should contain task ID 'TASK-042'; got: %q", out)
	}
	if !strings.Contains(out, "Implement awesome feature") {
		t.Errorf("renderTaskSummary should contain task title; got: %q", out)
	}
}

// TestRenderTaskSummary_ShowsCompleteOutcome verifies that a completed task shows
// the "Complete" status string.
func TestRenderTaskSummary_ShowsCompleteOutcome(t *testing.T) {
	task := parser.Task{
		ID:       "TASK-001",
		Title:    "Done",
		Criteria: []parser.Criterion{{Checked: true, Text: "criterion done"}},
	}
	m := statusModel{
		expandedPlans: make(map[string]bool),
		plans:         []parser.Plan{{ID: "f1", File: "f1.md", Tasks: []parser.Task{task}}},
		treeCursor:    1,
		showAll:       true,
		width:         120,
		height:        40,
	}
	m.expandedPlans["f1"] = true
	out := m.renderTaskSummary(80, 20)
	if !strings.Contains(out, "Complete") {
		t.Errorf("renderTaskSummary for completed task should contain 'Complete'; got: %q", out)
	}
}

// TestRenderTaskSummary_ShowsBlockedOutcome verifies that a blocked task shows
// the "Blocked" status string.
func TestRenderTaskSummary_ShowsBlockedOutcome(t *testing.T) {
	task := parser.Task{
		ID:    "TASK-001",
		Title: "Waiting",
		// Blocked field must be set explicitly; parser sets it during file parsing.
		Criteria: []parser.Criterion{{Checked: false, Text: "BLOCKED: waiting for API", Blocked: true}},
	}
	m := statusModel{
		expandedPlans: make(map[string]bool),
		plans:         []parser.Plan{{ID: "f1", File: "f1.md", Tasks: []parser.Task{task}}},
		treeCursor:    1,
		showAll:       true,
		width:         120,
		height:        40,
	}
	m.expandedPlans["f1"] = true
	out := m.renderTaskSummary(80, 20)
	if !strings.Contains(out, "Blocked") {
		t.Errorf("renderTaskSummary for blocked task should contain 'Blocked'; got: %q", out)
	}
}

// TestRenderTaskSummary_ShowsPendingOutcome verifies that a pending (unblocked,
// incomplete) task shows the "Pending" status string.
func TestRenderTaskSummary_ShowsPendingOutcome(t *testing.T) {
	task := parser.Task{
		ID:       "TASK-001",
		Title:    "Waiting",
		Criteria: []parser.Criterion{{Checked: false, Text: "do the work"}},
	}
	m := statusModel{
		expandedPlans: make(map[string]bool),
		plans:         []parser.Plan{{ID: "f1", File: "f1.md", Tasks: []parser.Task{task}}},
		treeCursor:    1,
		showAll:       true,
		width:         120,
		height:        40,
	}
	m.expandedPlans["f1"] = true
	out := m.renderTaskSummary(80, 20)
	if !strings.Contains(out, "Pending") {
		t.Errorf("renderTaskSummary for pending task should contain 'Pending'; got: %q", out)
	}
}

// TestRenderTaskSummary_ShowsTokensAndCostWhenMetricsExist verifies that the task
// summary shows token and cost data from cachedTaskMetrics when available.
func TestRenderTaskSummary_ShowsTokensAndCostWhenMetricsExist(t *testing.T) {
	task := parser.Task{
		ID:       "TASK-001",
		Title:    "Task with metrics",
		Criteria: []parser.Criterion{{Checked: true, Text: "done"}},
	}
	m := statusModel{
		expandedPlans: make(map[string]bool),
		plans:         []parser.Plan{{ID: "f1", File: "f1.md", Tasks: []parser.Task{task}}},
		treeCursor:    1,
		showAll:       true,
		width:         120,
		height:        40,
		cachedTaskMetrics: taskMetrics{
			taskID:       "TASK-001",
			inputTokens:  5000,
			outputTokens: 1000,
			costUSD:      0.25,
			durationSecs: 120,
		},
	}
	m.expandedPlans["f1"] = true
	out := m.renderTaskSummary(80, 20)
	if !strings.Contains(out, "Tokens") {
		t.Errorf("renderTaskSummary should contain 'Tokens' when metrics exist; got: %q", out)
	}
	if !strings.Contains(out, "Cost") {
		t.Errorf("renderTaskSummary should contain 'Cost' when metrics exist; got: %q", out)
	}
	if !strings.Contains(out, "Duration") {
		t.Errorf("renderTaskSummary should contain 'Duration' when metrics exist; got: %q", out)
	}
}

// TestRenderTaskSummary_ShowsModelFromMetrics verifies that the model name appears
// in the summary when cachedTaskMetrics has a model field.
func TestRenderTaskSummary_ShowsModelFromMetrics(t *testing.T) {
	task := parser.Task{
		ID:       "TASK-001",
		Title:    "Task with model",
		Criteria: []parser.Criterion{{Checked: true, Text: "done"}},
	}
	m := statusModel{
		expandedPlans: make(map[string]bool),
		plans:         []parser.Plan{{ID: "f1", File: "f1.md", Tasks: []parser.Task{task}}},
		treeCursor:    1,
		showAll:       true,
		width:         120,
		height:        40,
		cachedTaskMetrics: taskMetrics{
			taskID:       "TASK-001",
			inputTokens:  1000,
			outputTokens: 200,
			costUSD:      0.05,
			model:        "anthropic/claude-sonnet-4-6",
		},
	}
	m.expandedPlans["f1"] = true
	out := m.renderTaskSummary(80, 20)
	// Model is shortened to the part after the last "/"
	if !strings.Contains(out, "claude-sonnet-4-6") {
		t.Errorf("renderTaskSummary should show short model name 'claude-sonnet-4-6'; got: %q", out)
	}
}

// TestRenderTaskSummary_ShowsDashDurationWhenNoMetrics verifies that Duration shows
// "—" when no task metrics are available.
func TestRenderTaskSummary_ShowsDashDurationWhenNoMetrics(t *testing.T) {
	task := parser.Task{
		ID:       "TASK-001",
		Title:    "Task without metrics",
		Criteria: []parser.Criterion{{Checked: false, Text: "do work"}},
	}
	m := statusModel{
		expandedPlans:     make(map[string]bool),
		plans:             []parser.Plan{{ID: "f1", File: "f1.md", Tasks: []parser.Task{task}}},
		treeCursor:        1,
		showAll:           true,
		width:             120,
		height:            40,
		cachedTaskMetrics: taskMetrics{}, // zero metrics
	}
	m.expandedPlans["f1"] = true
	out := m.renderTaskSummary(80, 20)
	if !strings.Contains(out, "Duration") {
		t.Errorf("renderTaskSummary should always show 'Duration' label; got: %q", out)
	}
	if !strings.Contains(out, "—") {
		t.Errorf("renderTaskSummary should show '—' for duration when no metrics; got: %q", out)
	}
}

// TestRenderTaskSummary_NilTaskReturnsEmpty verifies that renderTaskSummary returns
// empty content gracefully when no task is found at the cursor position.
func TestRenderTaskSummary_NilTaskReturnsEmpty(t *testing.T) {
	// Empty plans → selectedTask() returns nil
	m := statusModel{
		expandedPlans: make(map[string]bool),
		plans:         nil,
		treeCursor:    0,
		width:         120,
		height:        40,
	}
	// renderTaskSummary directly; should not panic and should return something
	out := m.renderTaskSummary(80, 20)
	if strings.Contains(out, "TASK-") {
		t.Errorf("renderTaskSummary with nil task should not contain task content; got: %q", out)
	}
}

// ── taskStatusDisplay ──────────────────────────────────────────────────────────

// TestTaskStatusDisplay_CompleteTask verifies the display string for a completed task.
func TestTaskStatusDisplay_CompleteTask(t *testing.T) {
	task := &parser.Task{
		ID:       "TASK-001",
		Criteria: []parser.Criterion{{Checked: true, Text: "done"}},
	}
	str, _ := taskStatusDisplay(task)
	if !strings.Contains(str, "Complete") {
		t.Errorf("taskStatusDisplay for complete task should contain 'Complete'; got: %q", str)
	}
}

// TestTaskStatusDisplay_BlockedTask verifies the display string for a blocked task.
func TestTaskStatusDisplay_BlockedTask(t *testing.T) {
	task := &parser.Task{
		ID: "TASK-001",
		// Blocked must be set explicitly (the parser sets it; tests must mirror that).
		Criteria: []parser.Criterion{{Checked: false, Text: "BLOCKED: waiting", Blocked: true}},
	}
	str, _ := taskStatusDisplay(task)
	if !strings.Contains(str, "Blocked") {
		t.Errorf("taskStatusDisplay for blocked task should contain 'Blocked'; got: %q", str)
	}
}

// TestTaskStatusDisplay_PendingTask verifies the display string for a pending task.
func TestTaskStatusDisplay_PendingTask(t *testing.T) {
	task := &parser.Task{
		ID:       "TASK-001",
		Criteria: []parser.Criterion{{Checked: false, Text: "do the work"}},
	}
	str, _ := taskStatusDisplay(task)
	if !strings.Contains(str, "Pending") {
		t.Errorf("taskStatusDisplay for pending task should contain 'Pending'; got: %q", str)
	}
}

// ── summaryRow helper ──────────────────────────────────────────────────────────

// TestSummaryRow_ContainsLabelAndValue verifies the basic row rendering.
func TestSummaryRow_ContainsLabelAndValue(t *testing.T) {
	labelStyle := statusDimStyle
	valueStyle := statusBoldStyle
	row := summaryRow(labelStyle, valueStyle, "  Duration", "42s")
	if !strings.Contains(row, "Duration") {
		t.Errorf("summaryRow should contain label 'Duration'; got: %q", row)
	}
	if !strings.Contains(row, "42s") {
		t.Errorf("summaryRow should contain value '42s'; got: %q", row)
	}
	if !strings.HasSuffix(row, "\n") {
		t.Errorf("summaryRow should end with newline; got: %q", row)
	}
}

// TestSummaryRow_ShortLabelIsPadded verifies that labels shorter than labelWidth are
// padded with spaces so the value column aligns consistently.
func TestSummaryRow_ShortLabelIsPadded(t *testing.T) {
	labelStyle := statusDimStyle
	valueStyle := statusBoldStyle
	short := summaryRow(labelStyle, valueStyle, "  A", "v1")
	long := summaryRow(labelStyle, valueStyle, "  LongerLabel", "v1")
	// Both values should appear at the same visual column (12 chars of label space)
	// We verify the row is wider than just "A" + " " + "v1"
	if len(short) <= len("  A v1\n") {
		t.Errorf("summaryRow should pad short labels; got row of len %d: %q", len(short), short)
	}
	_ = long
}

// ── featureDaemonState ──────────────────────────────────────────────────────────

// TestFeatureDaemonState_ReturnsEmptyWhenDaemonNotRunning verifies that all return
// values are empty strings when the daemon is idle.
func TestFeatureDaemonState_ReturnsEmptyWhenDaemonNotRunning(t *testing.T) {
	m := statusModel{
		daemon: daemonStatus{Running: false},
	}
	plan := parser.Plan{
		ID:    "f1",
		Tasks: []parser.Task{{ID: "TASK-001"}},
	}
	taskID, elapsed, spinner := m.featureDaemonState(plan)
	if taskID != "" {
		t.Errorf("featureDaemonState: taskID = %q, want empty", taskID)
	}
	if elapsed != "" {
		t.Errorf("featureDaemonState: elapsed = %q, want empty", elapsed)
	}
	if spinner != "" {
		t.Errorf("featureDaemonState: spinner = %q, want empty", spinner)
	}
}

// TestFeatureDaemonState_ReturnsTaskIDWhenDaemonRunningOnTask verifies that the
// task ID and non-empty spinner are returned when the daemon works on a plan task.
func TestFeatureDaemonState_ReturnsTaskIDWhenDaemonRunningOnTask(t *testing.T) {
	m := statusModel{
		daemon: daemonStatus{Running: true, CurrentTask: "TASK-001"},
	}
	plan := parser.Plan{
		ID:    "f1",
		Tasks: []parser.Task{{ID: "TASK-001"}},
	}
	taskID, _, spinner := m.featureDaemonState(plan)
	if taskID != "TASK-001" {
		t.Errorf("featureDaemonState: taskID = %q, want TASK-001", taskID)
	}
	if spinner == "" {
		t.Error("featureDaemonState: spinner should be non-empty when daemon is running")
	}
}

// TestFeatureDaemonState_ReturnsEmptyWhenDaemonRunningOnDifferentFeature verifies that
// no task ID is returned when the daemon works on a task not in the given plan.
func TestFeatureDaemonState_ReturnsEmptyWhenDaemonRunningOnDifferentFeature(t *testing.T) {
	m := statusModel{
		daemon: daemonStatus{Running: true, CurrentTask: "TASK-999"},
	}
	plan := parser.Plan{
		ID:    "f1",
		Tasks: []parser.Task{{ID: "TASK-001"}},
	}
	taskID, _, _ := m.featureDaemonState(plan)
	if taskID != "" {
		t.Errorf("featureDaemonState: taskID = %q, want empty (different feature)", taskID)
	}
}

// ── featureWorkers ──────────────────────────────────────────────────────────────

// TestFeatureWorkers_ReturnsNilWhenWorkerIndexEmpty verifies that nil is returned
// when the global worker index is empty.
func TestFeatureWorkers_ReturnsNilWhenWorkerIndexEmpty(t *testing.T) {
	m := statusModel{workerIndex: nil}
	plan := parser.Plan{Tasks: []parser.Task{{ID: "TASK-001"}}}
	result := m.featureWorkers(plan)
	if result != nil {
		t.Errorf("featureWorkers should return nil when workerIndex is empty; got %v", result)
	}
}

// TestFeatureWorkers_FiltersToOnlyPlanTasks verifies that only workers whose task IDs
// belong to the plan's tasks are returned.
func TestFeatureWorkers_FiltersToOnlyPlanTasks(t *testing.T) {
	m := statusModel{
		workerIndex: []runlog.WorkerIndexEntry{
			{TaskID: "TASK-001", Status: "working"},
			{TaskID: "TASK-002", Status: "waiting"},
			{TaskID: "TASK-999", Status: "working"}, // not in plan
		},
	}
	plan := parser.Plan{
		Tasks: []parser.Task{{ID: "TASK-001"}, {ID: "TASK-002"}},
	}
	result := m.featureWorkers(plan)
	if len(result) != 2 {
		t.Fatalf("featureWorkers: got %d entries, want 2", len(result))
	}
	for _, w := range result {
		if w.TaskID == "TASK-999" {
			t.Error("featureWorkers: TASK-999 should not appear (not in plan)")
		}
	}
}

// TestFeatureWorkers_ReturnsNilWhenNoWorkersInPlan verifies nil is returned when
// none of the worker index entries match plan tasks.
func TestFeatureWorkers_ReturnsNilWhenNoWorkersInPlan(t *testing.T) {
	m := statusModel{
		workerIndex: []runlog.WorkerIndexEntry{
			{TaskID: "TASK-999", Status: "working"},
		},
	}
	plan := parser.Plan{
		Tasks: []parser.Task{{ID: "TASK-001"}},
	}
	result := m.featureWorkers(plan)
	if len(result) != 0 {
		t.Errorf("featureWorkers: expected 0 results, got %d", len(result))
	}
}

// ── loadTaskCommit / scanLogFileForTaskCommit ────────────────────────────────────

// TestLoadTaskCommit_ReturnsEmptyForMissingDir verifies that loadTaskCommit returns
// an empty string without panicking when the runs directory does not exist.
func TestLoadTaskCommit_ReturnsEmptyForMissingDir(t *testing.T) {
	dir := t.TempDir()
	result := loadTaskCommit(dir, "TASK-001")
	if result != "" {
		t.Errorf("loadTaskCommit: expected empty string for missing runs dir; got %q", result)
	}
}

// TestScanLogFileForTaskCommit_FindsCommitHashForMatchingTask verifies that the
// commit hash is extracted from a JSONL log file for the given task ID.
func TestScanLogFileForTaskCommit_FindsCommitHashForMatchingTask(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "run.log")

	entry := runlog.Entry{
		Event:  "task_complete",
		TaskID: "TASK-001",
		Commit: "abc1234567890",
	}
	data, _ := json.Marshal(entry)
	if err := os.WriteFile(logPath, append(data, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}

	commit := scanLogFileForTaskCommit(logPath, "TASK-001")
	if commit != "abc1234567890" {
		t.Errorf("scanLogFileForTaskCommit: expected 'abc1234567890'; got %q", commit)
	}
}

// TestScanLogFileForTaskCommit_ReturnsEmptyForNonMatchingTask verifies that an empty
// string is returned when the log file has no entry matching the task ID.
func TestScanLogFileForTaskCommit_ReturnsEmptyForNonMatchingTask(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "run.log")

	entry := runlog.Entry{
		Event:  "task_complete",
		TaskID: "TASK-002",
		Commit: "abc1234567890",
	}
	data, _ := json.Marshal(entry)
	if err := os.WriteFile(logPath, append(data, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}

	commit := scanLogFileForTaskCommit(logPath, "TASK-001")
	if commit != "" {
		t.Errorf("scanLogFileForTaskCommit: expected empty for non-matching task; got %q", commit)
	}
}

// TestScanLogFileForTaskCommit_ReturnsLastMatchForMultipleEntries verifies that
// when multiple task_complete entries match, the last one wins.
func TestScanLogFileForTaskCommit_ReturnsLastMatchForMultipleEntries(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "run.log")

	entry1 := runlog.Entry{Event: "task_complete", TaskID: "TASK-001", Commit: "first111"}
	entry2 := runlog.Entry{Event: "task_complete", TaskID: "TASK-001", Commit: "last9999"}

	data1, _ := json.Marshal(entry1)
	data2, _ := json.Marshal(entry2)
	content := append(data1, '\n')
	content = append(content, data2...)
	content = append(content, '\n')
	if err := os.WriteFile(logPath, content, 0o600); err != nil {
		t.Fatal(err)
	}

	commit := scanLogFileForTaskCommit(logPath, "TASK-001")
	if commit != "last9999" {
		t.Errorf("scanLogFileForTaskCommit: expected 'last9999' (last match); got %q", commit)
	}
}

// TestLoadTaskCommit_FindsCommitFromRunsDirectory verifies the full path: writing
// a log file to .maggus/runs/ and retrieving the commit via loadTaskCommit.
func TestLoadTaskCommit_FindsCommitFromRunsDirectory(t *testing.T) {
	dir := t.TempDir()
	runsDir := filepath.Join(dir, ".maggus", "runs")
	if err := os.MkdirAll(runsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Use a timestamp-prefixed filename so sort order is deterministic
	logPath := filepath.Join(runsDir, "20260101-120000-run.log")
	entry := runlog.Entry{
		Event:  "task_complete",
		TaskID: "TASK-042",
		Commit: "deadbeef",
	}
	data, _ := json.Marshal(entry)
	if err := os.WriteFile(logPath, append(data, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}

	commit := loadTaskCommit(dir, "TASK-042")
	if commit != "deadbeef" {
		t.Errorf("loadTaskCommit: expected 'deadbeef'; got %q", commit)
	}
}

// TestRenderTaskSummary_ShowsCommitHashForCompletedTask verifies that when a commit
// is available for a completed task, its shortened hash appears in the summary.
func TestRenderTaskSummary_ShowsCommitHashForCompletedTask(t *testing.T) {
	// Set up a real runs directory with a task_complete entry so loadTaskCommit works.
	dir := t.TempDir()
	runsDir := filepath.Join(dir, ".maggus", "runs")
	if err := os.MkdirAll(runsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(runsDir, "20260101-120000-run.log")
	entry := runlog.Entry{Event: "task_complete", TaskID: "TASK-001", Commit: "cafebabe12345"}
	data, _ := json.Marshal(entry)
	if err := os.WriteFile(logPath, append(data, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}

	task := parser.Task{
		ID:       "TASK-001",
		Title:    "Completed with commit",
		Criteria: []parser.Criterion{{Checked: true, Text: "done"}},
	}
	m := statusModel{
		expandedPlans: make(map[string]bool),
		plans:         []parser.Plan{{ID: "f1", File: "f1.md", Tasks: []parser.Task{task}}},
		treeCursor:    1,
		showAll:       true,
		width:         120,
		height:        40,
		dir:           dir,
	}
	m.expandedPlans["f1"] = true
	out := m.renderTaskSummary(80, 20)
	// commit is shortened to first 7 chars: "cafebab"
	if !strings.Contains(out, "cafebab") {
		t.Errorf("renderTaskSummary should show shortened commit 'cafebab'; got: %q", out)
	}
	if !strings.Contains(out, "Commit") {
		t.Errorf("renderTaskSummary should show 'Commit' label; got: %q", out)
	}
}

// TestRenderTaskSummary_NoCommitForPendingTask verifies that the Commit row is absent
// for a pending (incomplete) task since commits are only shown for completed tasks.
func TestRenderTaskSummary_NoCommitForPendingTask(t *testing.T) {
	task := parser.Task{
		ID:       "TASK-001",
		Title:    "Pending task",
		Criteria: []parser.Criterion{{Checked: false, Text: "do work"}},
	}
	m := statusModel{
		expandedPlans: make(map[string]bool),
		plans:         []parser.Plan{{ID: "f1", File: "f1.md", Tasks: []parser.Task{task}}},
		treeCursor:    1,
		showAll:       true,
		width:         120,
		height:        40,
	}
	m.expandedPlans["f1"] = true
	out := m.renderTaskSummary(80, 20)
	if strings.Contains(out, "Commit") {
		t.Errorf("renderTaskSummary should not show 'Commit' row for pending task; got: %q", out)
	}
}

// ── Integration: selectionCtx + renderSummaryTab ──────────────────────────────

// TestRenderSummaryTab_ProduceDifferentOutputForFeatureAndTask verifies that the
// feature summary and task summary produce different output, confirming correct dispatch.
func TestRenderSummaryTab_ProduceDifferentOutputForFeatureAndTask(t *testing.T) {
	featureModel := makeModelForCtx(selFeature)
	completedModel := makeModelForCtx(selCompletedTask)

	featureOut := featureModel.renderSummaryTab(80, 20)
	taskOut := completedModel.renderSummaryTab(80, 20)

	if featureOut == taskOut {
		t.Error("renderSummaryTab: feature and completed-task views should produce different output")
	}
}

// ── Format helpers used by renderFeatureSummary ────────────────────────────────

// TestFormatTokens_FormatsLargeNumbers verifies FormatTokens produces human-readable output.
// FormatTokens always uses a "k" suffix (thousands); there is no "M" suffix.
func TestFormatTokens_FormatsLargeNumbers(t *testing.T) {
	tests := []struct {
		input int
		want  string
	}{
		{999, "999"},      // small values: plain number, no suffix
		{1500, "1.5k"},    // 1.5 thousands
		{12345, "12.3k"},  // 12.3 thousands
		{1_000_000, "1000k"}, // large values still use k suffix
	}
	for _, tt := range tests {
		got := FormatTokens(tt.input)
		if got != tt.want {
			t.Errorf("FormatTokens(%d) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
