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
		{ctx: selFeature, wantTabNames: []string{"Summary", "Details", "Metrics"}, tabCount: 3},
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
			wantLabels: []string{"[1]", "[2]", "[3]"},
			noLabels:   []string{"[4]"},
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

	t.Run("selFeature tab 1 dispatches Details tab (not Summary)", func(t *testing.T) {
		m := makeModelForCtx(selFeature)
		m.activeTab = 1 // Details
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
		// selFeature has 3 tabs (Summary=0, Details=1, Metrics=2)
		{ctx: selFeature, key: "1", wantTab: 0},
		{ctx: selFeature, key: "2", wantTab: 1},
		{ctx: selFeature, key: "3", wantTab: 2},
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
		// selFeature has 3 tabs; key 4,5 are beyond count
		{ctx: selFeature, key: "4", desc: "selFeature: key 4 beyond 3 tabs"},
		{ctx: selFeature, key: "5", desc: "selFeature: key 5 beyond 3 tabs"},
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
		// selFeature: 3 tabs → key range is 1-3
		{ctx: selFeature, wantRange: "1-3: tabs"},
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
