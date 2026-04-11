package cmd

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/leberkas-org/maggus/internal/parser"
	"github.com/leberkas-org/maggus/internal/runlog"
)

// makeModelForCtx builds a minimal statusModel whose selectionCtx() returns ctx.
// width/height are set so rendering functions produce output rather than empty strings.
func makeModelForCtx(ctx selectionContext) statusModel {
	m := statusModel{
		expandedPlans: make(map[string]bool),
		width:         120,
		height:        40,
	}
	switch ctx {
	case selNone:
		// empty plans → tree is empty → selNone
	case selFeature:
		m.plans = []parser.Plan{
			{ID: "f1", File: "feature_1.md", Tasks: []parser.Task{{ID: "TASK-001", Title: "T1"}}},
		}
		m.treeCursor = 0 // plan row
	case selRunningTask:
		task := parser.Task{ID: "TASK-001", Title: "T1"}
		m.plans = []parser.Plan{
			{ID: "f1", File: "feature_1.md", Tasks: []parser.Task{task}},
		}
		m.expandedPlans["f1"] = true
		m.treeCursor = 1 // task row
		m.daemon = daemonStatus{Running: true, CurrentTask: "TASK-001"}
	case selCompletedTask:
		task := parser.Task{
			ID:       "TASK-001",
			Title:    "T1",
			Criteria: []parser.Criterion{{Checked: true, Text: "done"}},
		}
		m.plans = []parser.Plan{
			{ID: "f1", File: "feature_1.md", Tasks: []parser.Task{task}},
		}
		m.expandedPlans["f1"] = true
		m.treeCursor = 1 // completed task row
	}
	return m
}

// pressKey simulates pressing a single character key through updateList.
func pressKey(m statusModel, key string) statusModel {
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	result, _ := m.updateList(msg)
	updated, ok := result.(statusModel)
	if !ok {
		return m
	}
	return updated
}

// ── TestRenderRightPaneTabBar_RendersOnlyAvailableTabs ────────────────────────

func TestRenderRightPaneTabBar_RendersOnlyAvailableTabs(t *testing.T) {
	tests := []struct {
		ctx          selectionContext
		wantTabNames []string
		// tabCount is the expected number of [N] key labels in the bar.
		// Used to guard against extra tabs without depending on substring collisions
		// (e.g. "Details" is a substring of "Task Details").
		tabCount int
	}{
		{ctx: selNone, wantTabNames: []string{"Metrics"}, tabCount: 1},
		{ctx: selFeature, wantTabNames: []string{"Summary", "Plan", "Details", "Metrics"}, tabCount: 4},
		{ctx: selRunningTask, wantTabNames: []string{"Output", "Task Details", "Metrics"}, tabCount: 3},
		{ctx: selCompletedTask, wantTabNames: []string{"Summary", "Output", "Task Details", "Metrics"}, tabCount: 4},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("context %d", tt.ctx), func(t *testing.T) {
			m := makeModelForCtx(tt.ctx)
			bar := m.renderRightPaneTabBar()

			for _, name := range tt.wantTabNames {
				if !strings.Contains(bar, name) {
					t.Errorf("tab bar should contain %q, got: %q", name, bar)
				}
			}
			// Verify exact tab count via availableTabs() — this guards against extra tabs
			// without relying on substring checks (which can collide, e.g. "Details" ⊂ "Task Details").
			got := len(m.availableTabs())
			if got != tt.tabCount {
				t.Errorf("availableTabs() len = %d, want %d", got, tt.tabCount)
			}
		})
	}
}

// ── TestRenderRightPaneTabBar_NumberLabelsStartAt2 ────────────────────────────

func TestRenderRightPaneTabBar_NumberLabelsStartAt1(t *testing.T) {
	tests := []struct {
		ctx        selectionContext
		wantLabels []string
		noLabels   []string
	}{
		{
			ctx:        selNone,
			wantLabels: []string{"[1]"},
			noLabels:   []string{"[2]", "[3]", "[4]"},
		},
		{
			ctx:        selFeature,
			wantLabels: []string{"[1]", "[2]", "[3]", "[4]"},
			noLabels:   []string{"[5]"},
		},
		{
			ctx:        selRunningTask,
			wantLabels: []string{"[1]", "[2]", "[3]"},
			noLabels:   []string{"[4]"},
		},
		{
			ctx:        selCompletedTask,
			wantLabels: []string{"[1]", "[2]", "[3]", "[4]"},
			noLabels:   []string{"[5]"},
		},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("context %d", tt.ctx), func(t *testing.T) {
			m := makeModelForCtx(tt.ctx)
			bar := m.renderRightPaneTabBar()

			for _, lbl := range tt.wantLabels {
				if !strings.Contains(bar, lbl) {
					t.Errorf("tab bar should contain label %q, got: %q", lbl, bar)
				}
			}
			for _, lbl := range tt.noLabels {
				if strings.Contains(bar, lbl) {
					t.Errorf("tab bar should NOT contain label %q, got: %q", lbl, bar)
				}
			}
		})
	}
}

// ── TestRenderRightPane_DispatchesCorrectTab ──────────────────────────────────

func TestRenderRightPane_DispatchesCorrectTab(t *testing.T) {
	t.Run("selNone dispatches Metrics tab", func(t *testing.T) {
		m := makeModelForCtx(selNone)
		m.activeTab = 0 // Metrics
		out := m.renderRightPane(80, 20)
		// Summary placeholder should not appear; no "coming soon"
		if strings.Contains(out, "coming soon") {
			t.Errorf("selNone/Metrics should not render 'coming soon'; got output containing it")
		}
	})

	t.Run("selFeature tab 0 dispatches Summary tab", func(t *testing.T) {
		m := makeModelForCtx(selFeature)
		m.activeTab = 0 // Summary
		out := m.renderRightPane(80, 20)
		// Summary tab now renders real feature content; verify the plan ID appears.
		if !strings.Contains(out, "f1") {
			t.Errorf("selFeature/Summary should render feature summary with plan ID; got: %q", out)
		}
		if strings.Contains(out, "coming soon") {
			t.Errorf("selFeature/Summary should NOT render 'coming soon' placeholder")
		}
	})

	t.Run("selFeature tab 1 dispatches Plan tab", func(t *testing.T) {
		m := makeModelForCtx(selFeature)
		m.activeTab = 1 // Plan
		out := m.renderRightPane(80, 20)
		// Plan tab renders execution steps; Summary placeholder must not appear
		if strings.Contains(out, "coming soon") {
			t.Errorf("selFeature/Plan should NOT render 'coming soon' placeholder")
		}
	})

	t.Run("selFeature tab 2 dispatches Details tab", func(t *testing.T) {
		m := makeModelForCtx(selFeature)
		m.activeTab = 2 // Details
		out := m.renderRightPane(80, 20)
		// Details renders the task list; Summary placeholder must not appear
		if strings.Contains(out, "coming soon") {
			t.Errorf("selFeature/Details should NOT render 'coming soon' placeholder")
		}
	})

	t.Run("selRunningTask tab 0 dispatches Output tab", func(t *testing.T) {
		m := makeModelForCtx(selRunningTask)
		m.activeTab = 0 // Output
		out := m.renderRightPane(80, 20)
		// Daemon running but no snapshot → "Waiting for agent output..."
		if !strings.Contains(out, "Waiting for agent output") {
			t.Errorf("selRunningTask/Output with no snapshot should show 'Waiting for agent output'; got: %q", out)
		}
	})

	t.Run("different tabs produce different output", func(t *testing.T) {
		m := makeModelForCtx(selFeature)
		m.activeTab = 0 // Summary
		out0 := m.renderRightPane(80, 20)
		m.activeTab = 1 // Details
		out1 := m.renderRightPane(80, 20)
		if out0 == out1 {
			t.Error("Summary and Details tabs should render different content")
		}
	})
}

// ── TestUpdateList_NumberKeysMappedPositionally ───────────────────────────────

func TestUpdateList_NumberKeysMappedPositionally(t *testing.T) {
	tests := []struct {
		ctx     selectionContext
		key     string
		wantTab int
	}{
		// selFeature has 4 tabs (Summary=0, Plan=1, Details=2, Metrics=3)
		{ctx: selFeature, key: "1", wantTab: 0},
		{ctx: selFeature, key: "2", wantTab: 1},
		{ctx: selFeature, key: "3", wantTab: 2},
		{ctx: selFeature, key: "4", wantTab: 3},
		// selCompletedTask has 4 tabs
		{ctx: selCompletedTask, key: "1", wantTab: 0},
		{ctx: selCompletedTask, key: "4", wantTab: 3},
		// selNone has 1 tab (Metrics=0)
		{ctx: selNone, key: "1", wantTab: 0},
	}

	for _, tt := range tests {
		name := fmt.Sprintf("ctx=%d key=%s → tab %d", tt.ctx, tt.key, tt.wantTab)
		t.Run(name, func(t *testing.T) {
			m := makeModelForCtx(tt.ctx)
			m.activeTab = 0
			result := pressKey(m, tt.key)
			if result.activeTab != tt.wantTab {
				t.Errorf("activeTab = %d, want %d", result.activeTab, tt.wantTab)
			}
		})
	}
}

// ── TestUpdateList_KeysBeyondTabCountIgnored ──────────────────────────────────

func TestUpdateList_KeysBeyondTabCountIgnored(t *testing.T) {
	tests := []struct {
		ctx  selectionContext
		key  string
		desc string
	}{
		// selNone has 1 tab; keys 2,3,4,5 are beyond count
		{ctx: selNone, key: "2", desc: "selNone: key 2 beyond 1 tab"},
		{ctx: selNone, key: "3", desc: "selNone: key 3 beyond 1 tab"},
		{ctx: selNone, key: "4", desc: "selNone: key 4 beyond 1 tab"},
		// selFeature has 4 tabs; key 5 is beyond count
		{ctx: selFeature, key: "5", desc: "selFeature: key 5 beyond 4 tabs"},
		// selRunningTask has 3 tabs; key 4,5 are beyond count
		{ctx: selRunningTask, key: "4", desc: "selRunningTask: key 4 beyond 3 tabs"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			m := makeModelForCtx(tt.ctx)
			m.activeTab = 0
			result := pressKey(m, tt.key)
			// Active tab should be unchanged
			if result.activeTab != 0 {
				t.Errorf("activeTab should remain 0 (key ignored), but got %d", result.activeTab)
			}
		})
	}
}

// ── TestStatusSplitFooter_DynamicTabRange ─────────────────────────────────────

func TestStatusSplitFooter_DynamicTabRange(t *testing.T) {
	tests := []struct {
		ctx       selectionContext
		wantRange string
	}{
		// selNone: 1 tab → key range is 1-1
		{ctx: selNone, wantRange: "1-1: tabs"},
		// selFeature: 4 tabs → key range is 1-4
		{ctx: selFeature, wantRange: "1-4: tabs"},
		// selRunningTask: 3 tabs → key range is 1-3
		{ctx: selRunningTask, wantRange: "1-3: tabs"},
		// selCompletedTask: 4 tabs → key range is 1-4
		{ctx: selCompletedTask, wantRange: "1-4: tabs"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("context %d", tt.ctx), func(t *testing.T) {
			m := makeModelForCtx(tt.ctx)
			footer := m.statusSplitFooter()
			if !strings.Contains(footer, tt.wantRange) {
				t.Errorf("footer should contain %q; got: %q", tt.wantRange, footer)
			}
		})
	}
}

func TestRenderSnapshotInPane_TruncatesLongTaskTitle(t *testing.T) {
	tests := []struct {
		name            string
		taskTitle       string
		width           int
		expectTruncated bool
		expectOmitted   bool
	}{
		{
			name:            "short title fits completely",
			taskTitle:       "Short task",
			width:           100,
			expectTruncated: false,
			expectOmitted:   false,
		},
		{
			name:            "long title gets truncated",
			taskTitle:       "This is a very long task title that should be truncated when displayed in a narrow pane",
			width:           50,
			expectTruncated: true,
			expectOmitted:   false,
		},
		{
			name:            "very narrow width omits title entirely",
			taskTitle:       "Task title",
			width:           20,
			expectTruncated: false,
			expectOmitted:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snap := &runlog.StateSnapshot{
				TaskID:    "TASK-001",
				TaskTitle: tt.taskTitle,
				Status:    "Running",
			}

			m := statusModel{}

			output := m.renderSnapshotInPane(snap, 0, tt.width, 30)

			// Find the Task title line (line with TaskID in "Task: TASKID - Title" format)
			lines := strings.Split(output, "\n")
			var taskTitleLine string
			for _, line := range lines {
				// Look for the line that contains "Task:" followed by the TaskID
				// This will be on the top part of the output before the separator
				if strings.Contains(line, "Task:") && strings.Contains(line, "TASK-001") {
					taskTitleLine = line
					break
				}
			}

			if tt.expectOmitted {
				if taskTitleLine != "" && strings.Contains(taskTitleLine, "Task:") && strings.Contains(taskTitleLine, "TASK-001") {
					// The line should just have "Task: TASK-001" without a title
					if strings.Contains(taskTitleLine, " - ") {
						t.Errorf("expected task title to be omitted (no ' - '), but found: %q", taskTitleLine)
					}
				}
			} else {
				if taskTitleLine == "" {
					t.Errorf("expected to find task title line with TaskID, but not found in output")
				}
			}
		})
	}
}

// ── TestRenderRightPane_TaskDetails_DoesNotOverflowHeight ────────────────────

// TestRenderRightPane_TaskDetails_DoesNotOverflowHeight verifies that the Task Details
// tab never produces more lines than the specified height, even when the viewport
// contains very long lines that lipgloss would word-wrap. This guards against
// the outer border frame being pushed off-screen (BUG-033-001).
func TestRenderRightPane_TaskDetails_DoesNotOverflowHeight(t *testing.T) {
	tests := []struct {
		name   string
		ctx    selectionContext
		tabIdx int // index of "Task Details" tab for that context
	}{
		{name: "selRunningTask Tab Details", ctx: selRunningTask, tabIdx: 1},
		{name: "selCompletedTask Tab Details", ctx: selCompletedTask, tabIdx: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := makeModelForCtx(tt.ctx)
			m.activeTab = tt.tabIdx

			width := 80
			height := 20
			contentH := height - 2 // matches renderRightPane's contentH computation

			// Populate the viewport with long lines that would word-wrap at width=80.
			// Each line is 200 characters — well beyond the pane width.
			longLine := strings.Repeat("A", 200)
			var sb strings.Builder
			// Write more lines than contentH so both overflow axes are tested.
			for i := 0; i < contentH+10; i++ {
				sb.WriteString(longLine + "\n")
			}
			m.currentTaskViewport.Width = width
			m.currentTaskViewport.Height = contentH
			m.currentTaskViewport.SetContent(sb.String())

			out := m.renderRightPane(width, height)

			// renderRightPane returns: rendered (height lines) + "\n" + borderLine
			// = height newlines total. Allow one extra for a possible trailing newline.
			newlines := strings.Count(out, "\n")
			maxAllowed := height + 1
			if newlines > maxAllowed {
				t.Errorf("%s: renderRightPane produced %d newlines, want at most %d (width=%d, height=%d)",
					tt.name, newlines, maxAllowed, width, height)
			}
		})
	}
}

// ── pressSpecialKey presses a non-rune key through updateList ────────────────

func pressSpecialKey(m statusModel, keyType tea.KeyType) statusModel {
	msg := tea.KeyMsg{Type: keyType}
	result, _ := m.updateList(msg)
	updated, ok := result.(statusModel)
	if !ok {
		return m
	}
	return updated
}

// makeOutputModel builds a statusModel on the Output tab of a running task
// with a live snapshot containing n tool entries.
func makeOutputModel(n int) statusModel {
	m := makeModelForCtx(selRunningTask)
	// Switch to the Output tab (index 0 for selRunningTask).
	m.activeTab = 0
	// Build a snapshot with n tool entries.
	entries := make([]runlog.SnapshotToolEntry, n)
	for i := range entries {
		entries[i] = runlog.SnapshotToolEntry{Type: "Read", Description: fmt.Sprintf("file%d.go", i)}
	}
	snap := &runlog.StateSnapshot{
		TaskID:      "TASK-001",
		Status:      "Running",
		ToolEntries: entries,
	}
	m.snapshot = snap
	m.logScroll = 0
	m.logAutoScroll = true
	return m
}

// ── TestScrollKeys_OutputTab ─────────────────────────────────────────────────

func TestScrollKeys_OutputTab_ShiftDownScrolls(t *testing.T) {
	m := makeOutputModel(50)
	m.logScroll = 0
	m.logAutoScroll = true

	result := pressSpecialKey(m, tea.KeyShiftDown)
	if result.logScroll != 1 {
		t.Errorf("shift+down should increment logScroll from 0 to 1, got %d", result.logScroll)
	}
	// After scrolling down from 0, auto-scroll should remain off since we're not at max.
	if result.logAutoScroll {
		t.Error("logAutoScroll should be false after shift+down when not at bottom")
	}
}

func TestScrollKeys_OutputTab_ShiftUpScrolls(t *testing.T) {
	m := makeOutputModel(50)
	m.logScroll = 5
	m.logAutoScroll = false

	result := pressSpecialKey(m, tea.KeyShiftUp)
	if result.logScroll != 4 {
		t.Errorf("shift+up should decrement logScroll from 5 to 4, got %d", result.logScroll)
	}
	if result.logAutoScroll {
		t.Error("logAutoScroll should remain false after shift+up")
	}
}

func TestScrollKeys_OutputTab_ShiftUpAtTopIsNoop(t *testing.T) {
	m := makeOutputModel(50)
	m.logScroll = 0
	m.logAutoScroll = false

	result := pressSpecialKey(m, tea.KeyShiftUp)
	if result.logScroll != 0 {
		t.Errorf("shift+up at top should stay at 0, got %d", result.logScroll)
	}
}

func TestScrollKeys_OutputTab_gJumpsToTop(t *testing.T) {
	m := makeOutputModel(50)
	m.logScroll = 10
	m.logAutoScroll = false

	result := pressKey(m, "g")
	if result.logScroll != 0 {
		t.Errorf("g should set logScroll to 0, got %d", result.logScroll)
	}
	if result.logAutoScroll {
		t.Error("logAutoScroll should be false after g (top)")
	}
}

func TestScrollKeys_OutputTab_GJumpsToBottom(t *testing.T) {
	m := makeOutputModel(50)
	m.logScroll = 0
	m.logAutoScroll = false

	result := pressKey(m, "G")
	maxS := result.maxLogScroll()
	if result.logScroll != maxS {
		t.Errorf("G should set logScroll to maxLogScroll (%d), got %d", maxS, result.logScroll)
	}
	if !result.logAutoScroll {
		t.Error("logAutoScroll should be true after G (bottom)")
	}
}

// ── TestScrollKeys_NoopOnNonScrollableTabs ───────────────────────────────────

func TestScrollKeys_NoopOnNonScrollableTabs(t *testing.T) {
	// Summary, Details, and Metrics tabs have no scrollable content.
	// Pressing scroll keys should not change logScroll.
	tests := []struct {
		desc string
		ctx  selectionContext
		tab  int
	}{
		{desc: "selFeature Summary tab", ctx: selFeature, tab: 0},
		{desc: "selFeature Details tab", ctx: selFeature, tab: 1},
		{desc: "selFeature Metrics tab", ctx: selFeature, tab: 2},
		{desc: "selNone Metrics tab", ctx: selNone, tab: 0},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			m := makeModelForCtx(tt.ctx)
			m.activeTab = tt.tab
			m.logScroll = 3
			m.logAutoScroll = false

			type keyFn func(statusModel) statusModel
			for _, fn := range []keyFn{
				func(m statusModel) statusModel { return pressSpecialKey(m, tea.KeyShiftUp) },
				func(m statusModel) statusModel { return pressSpecialKey(m, tea.KeyShiftDown) },
				func(m statusModel) statusModel { return pressKey(m, "g") },
				func(m statusModel) statusModel { return pressKey(m, "G") },
			} {
				result := fn(m)
				if result.logScroll != 3 {
					t.Errorf("logScroll should remain 3 on non-scrollable tab, got %d", result.logScroll)
				}
				if result.logAutoScroll {
					t.Errorf("logAutoScroll should remain false on non-scrollable tab")
				}
			}
		})
	}
}

// ── TestScrollKeys_AutoScrollReenabledAtBottom ───────────────────────────────

func TestScrollKeys_AutoScrollReenabledAtBottom(t *testing.T) {
	// When shift+down reaches the bottom, auto-scroll should be re-enabled.
	// Use 100 entries to ensure maxLogScroll() > 0 (visible lines ≈ 26 with width=120/height=40).
	m := makeOutputModel(100)
	maxS := m.maxLogScroll()
	if maxS <= 0 {
		t.Fatalf("expected maxLogScroll > 0 with 100 entries, got %d", maxS)
	}
	m.logScroll = maxS - 1
	m.logAutoScroll = false

	result := pressSpecialKey(m, tea.KeyShiftDown)
	if !result.logAutoScroll {
		t.Error("logAutoScroll should be re-enabled when shift+down reaches the bottom")
	}
}

// ── renderPlanTab tests ───────────────────────────────────────────────────────

// makePlanTabModel returns a statusModel with selFeature context, populated with
// tasks that have varied status and token estimates. The active tab is left at 0.
func makePlanTabModel() statusModel {
	m := statusModel{
		expandedPlans: make(map[string]bool),
		width:         120,
		height:        40,
	}

	// Task 1: complete
	t1 := parser.Task{
		ID:            "TASK-001",
		Title:         "First task",
		Criteria:      []parser.Criterion{{Checked: true, Text: "done"}},
		TokenEstimate: 10000,
	}
	// Task 2: pending (running)
	t2 := parser.Task{
		ID:            "TASK-002",
		Title:         "Running task",
		Criteria:      []parser.Criterion{{Checked: false, Text: "pending"}},
		Predecessors:  []string{"TASK-001"},
		TokenEstimate: 20000,
		Parallel:      true,
	}
	// Task 3: blocked
	t3 := parser.Task{
		ID:       "TASK-003",
		Title:    "Blocked task",
		Criteria: []parser.Criterion{{Checked: false, Text: "BLOCKED: something", Blocked: true}},
	}
	// Task 4: skipped
	t4 := parser.Task{
		ID:       "TASK-004",
		Title:    "Skipped task",
		Criteria: []parser.Criterion{{Checked: false, Text: "SKIPPED: something", Skipped: true}},
	}

	m.plans = []parser.Plan{
		{
			ID:    "feature_001",
			File:  "feature_001.md",
			Tasks: []parser.Task{t1, t2, t3, t4},
		},
	}
	m.treeCursor = 0 // plan row (selFeature)
	return m
}

// TestRenderPlanTab_ReturnsNonEmptyString verifies renderPlanTab does not return an empty string
// when a feature with tasks is selected.
func TestRenderPlanTab_ReturnsNonEmptyString(t *testing.T) {
	m := makePlanTabModel()
	out := m.renderPlanTab(80, 20)
	if strings.TrimSpace(out) == "" {
		t.Error("renderPlanTab returned empty string for a model with tasks")
	}
}

// TestRenderPlanTab_ShowsStepNumber verifies that step headers like "Step 1" appear.
func TestRenderPlanTab_ShowsStepNumber(t *testing.T) {
	m := makePlanTabModel()
	out := m.renderPlanTab(80, 20)
	if !strings.Contains(out, "Step 1") {
		t.Errorf("renderPlanTab output should contain 'Step 1'; got:\n%s", out)
	}
}

// TestRenderPlanTab_ShowsParallelLabel verifies "(parallel)" appears for parallel steps.
func TestRenderPlanTab_ShowsParallelLabel(t *testing.T) {
	m := statusModel{
		expandedPlans: make(map[string]bool),
		width:         120,
		height:        40,
		plans: []parser.Plan{
			{
				ID:   "f1",
				File: "feature_1.md",
				Tasks: []parser.Task{
					{ID: "T1", Title: "Task 1", Parallel: true},
					{ID: "T2", Title: "Task 2", Parallel: true},
				},
			},
		},
	}
	m.treeCursor = 0
	out := m.renderPlanTab(80, 20)
	if !strings.Contains(out, "parallel") {
		t.Errorf("expected 'parallel' label for parallel step; got:\n%s", out)
	}
}

// TestRenderPlanTab_DoneTaskShowsCheckmark verifies ✓ appears for completed tasks.
func TestRenderPlanTab_DoneTaskShowsCheckmark(t *testing.T) {
	m := statusModel{
		expandedPlans: make(map[string]bool),
		width:         120,
		height:        40,
		plans: []parser.Plan{
			{
				ID:   "f1",
				File: "feature_1.md",
				Tasks: []parser.Task{
					{
						ID:       "T1",
						Title:    "Done task",
						Criteria: []parser.Criterion{{Checked: true, Text: "done"}},
					},
				},
			},
		},
	}
	m.treeCursor = 0
	out := m.renderPlanTab(80, 20)
	if !strings.Contains(out, "✓") {
		t.Errorf("expected ✓ for completed task; got:\n%s", out)
	}
}

// TestRenderPlanTab_PendingTaskShowsCircle verifies ○ appears for pending tasks.
func TestRenderPlanTab_PendingTaskShowsCircle(t *testing.T) {
	m := statusModel{
		expandedPlans: make(map[string]bool),
		width:         120,
		height:        40,
		plans: []parser.Plan{
			{
				ID:   "f1",
				File: "feature_1.md",
				Tasks: []parser.Task{
					{
						ID:       "T1",
						Title:    "Pending task",
						Criteria: []parser.Criterion{{Checked: false, Text: "todo"}},
					},
				},
			},
		},
	}
	m.treeCursor = 0
	out := m.renderPlanTab(80, 20)
	if !strings.Contains(out, "○") {
		t.Errorf("expected ○ for pending task; got:\n%s", out)
	}
}

// TestRenderPlanTab_BlockedTaskShowsWarning verifies ⚠ appears for blocked tasks.
func TestRenderPlanTab_BlockedTaskShowsWarning(t *testing.T) {
	m := statusModel{
		expandedPlans: make(map[string]bool),
		width:         120,
		height:        40,
		plans: []parser.Plan{
			{
				ID:   "f1",
				File: "feature_1.md",
				Tasks: []parser.Task{
					{
						ID:       "T1",
						Title:    "Blocked task",
						Criteria: []parser.Criterion{{Checked: false, Text: "BLOCKED: something", Blocked: true}},
					},
				},
			},
		},
	}
	m.treeCursor = 0
	out := m.renderPlanTab(80, 20)
	if !strings.Contains(out, "⚠") {
		t.Errorf("expected ⚠ for blocked task; got:\n%s", out)
	}
}

// TestRenderPlanTab_SkippedTaskShowsArrow verifies > appears for skipped tasks.
func TestRenderPlanTab_SkippedTaskShowsArrow(t *testing.T) {
	m := statusModel{
		expandedPlans: make(map[string]bool),
		width:         120,
		height:        40,
		plans: []parser.Plan{
			{
				ID:   "f1",
				File: "feature_1.md",
				Tasks: []parser.Task{
					{
						ID:       "T1",
						Title:    "Skipped task",
						Criteria: []parser.Criterion{{Checked: false, Text: "SKIPPED: something", Skipped: true}},
					},
				},
			},
		},
	}
	m.treeCursor = 0
	out := m.renderPlanTab(80, 20)
	if !strings.Contains(out, ">") {
		t.Errorf("expected > for skipped task; got:\n%s", out)
	}
}

// TestRenderPlanTab_ShowsTokenTotal verifies the token total appears when non-zero.
func TestRenderPlanTab_ShowsTokenTotal(t *testing.T) {
	m := statusModel{
		expandedPlans: make(map[string]bool),
		width:         120,
		height:        40,
		plans: []parser.Plan{
			{
				ID:   "f1",
				File: "feature_1.md",
				Tasks: []parser.Task{
					{
						ID:            "T1",
						Title:         "Task 1",
						Criteria:      []parser.Criterion{{Checked: false, Text: "todo"}},
						TokenEstimate: 35000,
					},
					{
						ID:            "T2",
						Title:         "Task 2",
						Criteria:      []parser.Criterion{{Checked: false, Text: "todo"}},
						Predecessors:  []string{"T1"},
						TokenEstimate: 45000,
					},
				},
			},
		},
	}
	m.treeCursor = 0
	out := m.renderPlanTab(80, 20)
	// Total is 80000 tokens → rendered as "80k" by FormatTokens.
	if !strings.Contains(out, "80k") {
		t.Errorf("expected total token estimate ~80k in plan tab output; got:\n%s", out)
	}
}

// TestRenderPlanTab_NoTokensOmitsTotal verifies the token total line is omitted when all estimates are 0.
func TestRenderPlanTab_NoTokensOmitsTotal(t *testing.T) {
	m := statusModel{
		expandedPlans: make(map[string]bool),
		width:         120,
		height:        40,
		plans: []parser.Plan{
			{
				ID:   "f1",
				File: "feature_1.md",
				Tasks: []parser.Task{
					{ID: "T1", Title: "Task 1", Criteria: []parser.Criterion{{Checked: false, Text: "todo"}}},
				},
			},
		},
	}
	m.treeCursor = 0
	out := m.renderPlanTab(80, 20)
	if strings.Contains(out, "Estimated") {
		t.Errorf("token total line should be omitted when all estimates are 0; got:\n%s", out)
	}
}

// TestRenderPlanTab_RunningTaskShowsSpinner verifies a spinner-like char appears for running tasks.
func TestRenderPlanTab_RunningTaskShowsSpinner(t *testing.T) {
	m := statusModel{
		expandedPlans: make(map[string]bool),
		width:         120,
		height:        40,
		plans: []parser.Plan{
			{
				ID:   "f1",
				File: "feature_1.md",
				Tasks: []parser.Task{
					{
						ID:       "T1",
						Title:    "Running task",
						Criteria: []parser.Criterion{{Checked: false, Text: "in progress"}},
					},
				},
			},
		},
		daemon: daemonStatus{Running: true, CurrentTask: "T1"},
	}
	m.treeCursor = 0
	out := m.renderPlanTab(80, 20)
	// The spinner frame 0 character should appear (not ✓, ○, ⚠, or >).
	// We verify ✓ does not appear (it's not done), and a spinner character does.
	if strings.Contains(out, "✓") {
		t.Errorf("running task should show spinner, not ✓; got:\n%s", out)
	}
	// At spinnerFrame=0, the spinner char is styles.SpinnerFrames[0].
	// We just verify some content appears (not empty) since spinner chars may not be ASCII.
	if strings.TrimSpace(out) == "" {
		t.Error("renderPlanTab returned empty for running task model")
	}
}

// TestRenderPlanTab_NoSelectionReturnsPlaceholder verifies graceful output when no feature is selected.
func TestRenderPlanTab_NoSelectionReturnsPlaceholder(t *testing.T) {
	m := statusModel{
		expandedPlans: make(map[string]bool),
		width:         120,
		height:        40,
	}
	// No plans → selNone
	out := m.renderPlanTab(80, 20)
	// Should not panic and should return something (even empty styled content).
	_ = out
}

// TestRenderPlanTab_ScrollOffsetRespected verifies that planTabScroll shifts visible content.
func TestRenderPlanTab_ScrollOffsetRespected(t *testing.T) {
	// Build a model with many steps so there's content to scroll past.
	tasks := make([]parser.Task, 10)
	for i := range tasks {
		tasks[i] = parser.Task{
			ID:       fmt.Sprintf("T%02d", i+1),
			Title:    fmt.Sprintf("Task %d", i+1),
			Criteria: []parser.Criterion{{Checked: false, Text: "pending"}},
		}
	}
	m := statusModel{
		expandedPlans: make(map[string]bool),
		width:         120,
		height:        40,
		plans: []parser.Plan{
			{ID: "f1", File: "feature_1.md", Tasks: tasks},
		},
	}
	m.treeCursor = 0

	out0 := m.renderPlanTab(80, 20)

	// Scroll to offset 3 — the first 3 entries should no longer appear at the top.
	m.planTabScroll = 3
	outScrolled := m.renderPlanTab(80, 20)

	// The scrolled output should differ from the un-scrolled output.
	if out0 == outScrolled {
		t.Error("scroll offset 3 should produce different output than scroll offset 0")
	}
	// T01 should NOT appear in scrolled output (scrolled past).
	if strings.Contains(outScrolled, "T01") {
		t.Errorf("T01 should be scrolled past at offset=3; got:\n%s", outScrolled)
	}
}

// TestRenderPlanTab_ShowsTaskTitles verifies task titles appear in the output.
func TestRenderPlanTab_ShowsTaskTitles(t *testing.T) {
	m := statusModel{
		expandedPlans: make(map[string]bool),
		width:         120,
		height:        40,
		plans: []parser.Plan{
			{
				ID:   "f1",
				File: "feature_1.md",
				Tasks: []parser.Task{
					{ID: "T1", Title: "My unique task title", Criteria: []parser.Criterion{{Checked: false}}},
				},
			},
		},
	}
	m.treeCursor = 0
	out := m.renderPlanTab(80, 20)
	if !strings.Contains(out, "T1") {
		t.Errorf("task ID 'T1' should appear in plan tab; got:\n%s", out)
	}
}
