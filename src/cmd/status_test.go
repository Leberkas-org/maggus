package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/leberkas-org/maggus/internal/approval"
	"github.com/leberkas-org/maggus/internal/claude2x"
	"github.com/leberkas-org/maggus/internal/parser"
	"github.com/leberkas-org/maggus/internal/stores"
	"github.com/leberkas-org/maggus/internal/tui/styles"
)

func TestPlan_ApprovalKey(t *testing.T) {
	t.Run("uses MaggusID when set", func(t *testing.T) {
		p := &parser.Plan{ID: "feature_001", MaggusID: "abc-123"}
		if got := p.ApprovalKey(); got != "abc-123" {
			t.Errorf("ApprovalKey() = %q, want %q", got, "abc-123")
		}
	})

	t.Run("falls back to ID when MaggusID empty", func(t *testing.T) {
		p := &parser.Plan{ID: "feature_001"}
		if got := p.ApprovalKey(); got != "feature_001" {
			t.Errorf("ApprovalKey() = %q, want %q", got, "feature_001")
		}
	})
}

func TestPruneStaleApprovals(t *testing.T) {
	dir := setupApproveDir(t)

	// Write an approvals file with a stale entry
	if err := approval.Save(dir, approval.Approvals{
		"stale-uuid":  true,
		"active-uuid": true,
	}); err != nil {
		t.Fatal(err)
	}

	plans := []parser.Plan{
		{ID: "feature_001", MaggusID: "active-uuid", File: "feature_001.md"},
	}
	pruneStaleApprovals(dir, plans)

	a, err := approval.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := a["stale-uuid"]; ok {
		t.Error("stale-uuid should have been pruned")
	}
	if _, ok := a["active-uuid"]; !ok {
		t.Error("active-uuid should still be present")
	}
}

func TestPruneStaleApprovals_FallbackKey(t *testing.T) {
	dir := setupApproveDir(t)

	// Feature without MaggusID uses ID-based key
	fallbackKey := "feature_002"
	if err := approval.Save(dir, approval.Approvals{
		fallbackKey: true,
		"old-entry": false,
	}); err != nil {
		t.Fatal(err)
	}

	plans := []parser.Plan{
		{ID: "feature_002", File: "feature_002.md"}, // no MaggusID
	}
	pruneStaleApprovals(dir, plans)

	a, err := approval.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := a["old-entry"]; ok {
		t.Error("old-entry should have been pruned")
	}
	if _, ok := a[fallbackKey]; !ok {
		t.Errorf("%s should still be present", fallbackKey)
	}
}

func TestPlan_DoneCount(t *testing.T) {
	tests := []struct {
		name  string
		tasks []parser.Task
		want  int
	}{
		{
			name: "all complete",
			tasks: []parser.Task{
				{Criteria: []parser.Criterion{{Checked: true}}},
				{Criteria: []parser.Criterion{{Checked: true}}},
			},
			want: 2,
		},
		{
			name: "none complete",
			tasks: []parser.Task{
				{Criteria: []parser.Criterion{{Checked: false}}},
				{Criteria: []parser.Criterion{{Checked: false}}},
			},
			want: 0,
		},
		{
			name: "mixed",
			tasks: []parser.Task{
				{Criteria: []parser.Criterion{{Checked: true}}},
				{Criteria: []parser.Criterion{{Checked: false}}},
				{Criteria: []parser.Criterion{{Checked: true}, {Checked: true}}},
			},
			want: 2,
		},
		{
			name:  "empty tasks",
			tasks: nil,
			want:  0,
		},
		{
			name: "task with no criteria is not complete",
			tasks: []parser.Task{
				{Criteria: nil},
			},
			want: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &parser.Plan{Tasks: tt.tasks}
			got := p.DoneCount()
			if got != tt.want {
				t.Errorf("DoneCount() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestPlan_BlockedCount(t *testing.T) {
	tests := []struct {
		name  string
		tasks []parser.Task
		want  int
	}{
		{
			name: "one blocked",
			tasks: []parser.Task{
				{Criteria: []parser.Criterion{{Text: "ok"}}},
				{Criteria: []parser.Criterion{{Text: "BLOCKED: x", Blocked: true}}},
			},
			want: 1,
		},
		{
			name: "completed task with blocked criterion not counted",
			tasks: []parser.Task{
				{Criteria: []parser.Criterion{{Text: "BLOCKED: x", Blocked: true, Checked: true}}},
			},
			want: 0,
		},
		{
			name:  "empty",
			tasks: nil,
			want:  0,
		},
		{
			name: "multiple blocked",
			tasks: []parser.Task{
				{Criteria: []parser.Criterion{{Text: "BLOCKED: a", Blocked: true}}},
				{Criteria: []parser.Criterion{{Text: "BLOCKED: b", Blocked: true}}},
				{Criteria: []parser.Criterion{{Checked: true}}},
			},
			want: 2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &parser.Plan{Tasks: tt.tasks}
			got := p.BlockedCount()
			if got != tt.want {
				t.Errorf("BlockedCount() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestBuildSelectableTasksForFeature(t *testing.T) {
	plan := parser.Plan{
		Tasks: []parser.Task{
			{ID: "TASK-001", Criteria: []parser.Criterion{{Checked: true}}},
			{ID: "TASK-002", Criteria: []parser.Criterion{{Checked: false}}},
			{ID: "TASK-003", Criteria: []parser.Criterion{{Checked: true}}},
		},
	}

	t.Run("showAll false excludes complete", func(t *testing.T) {
		got := buildSelectableTasksForFeature(plan, false)
		if len(got) != 1 {
			t.Fatalf("len = %d, want 1", len(got))
		}
		if got[0].ID != "TASK-002" {
			t.Errorf("got %s, want TASK-002", got[0].ID)
		}
	})

	t.Run("showAll true includes all", func(t *testing.T) {
		got := buildSelectableTasksForFeature(plan, true)
		if len(got) != 3 {
			t.Fatalf("len = %d, want 3", len(got))
		}
	})

	t.Run("empty feature", func(t *testing.T) {
		got := buildSelectableTasksForFeature(parser.Plan{}, false)
		if len(got) != 0 {
			t.Errorf("len = %d, want 0", len(got))
		}
	})
}

func TestVisibleFeatures(t *testing.T) {
	plans := []parser.Plan{
		{ID: "plan_1", File: "plan_1.md"},
		{ID: "plan_2", File: "plan_2_completed.md", Completed: true},
		{ID: "plan_3", File: "plan_3.md"},
	}

	t.Run("showAll false hides completed", func(t *testing.T) {
		m := statusModel{plans: plans, showAll: false}
		got := m.visiblePlans()
		if len(got) != 2 {
			t.Fatalf("len = %d, want 2", len(got))
		}
		if got[0].File != "plan_1.md" || got[1].File != "plan_3.md" {
			t.Errorf("got %s, %s", got[0].File, got[1].File)
		}
	})

	t.Run("showAll true shows all", func(t *testing.T) {
		m := statusModel{plans: plans, showAll: true}
		got := m.visiblePlans()
		if len(got) != 3 {
			t.Fatalf("len = %d, want 3", len(got))
		}
	})

	t.Run("no plans", func(t *testing.T) {
		m := statusModel{plans: nil, showAll: false}
		got := m.visiblePlans()
		if len(got) != 0 {
			t.Errorf("len = %d, want 0", len(got))
		}
	})
}

func TestFindNextTask(t *testing.T) {
	t.Run("finds first incomplete task", func(t *testing.T) {
		plans := []parser.Plan{
			{
				ID:   "plan_1",
				File: "plan_1.md",
				Tasks: []parser.Task{
					{ID: "TASK-001", SourceFile: "plan_1.md", Criteria: []parser.Criterion{{Checked: true}}},
					{ID: "TASK-002", SourceFile: "plan_1.md", Criteria: []parser.Criterion{{Checked: false}}},
				},
			},
		}
		id, file := findNextTask(plans)
		if id != "TASK-002" {
			t.Errorf("id = %q, want TASK-002", id)
		}
		if file != "plan_1.md" {
			t.Errorf("file = %q, want plan_1.md", file)
		}
	})

	t.Run("skips completed plans", func(t *testing.T) {
		plans := []parser.Plan{
			{
				ID:        "plan_1",
				File:      "plan_1_completed.md",
				Completed: true,
				Tasks: []parser.Task{
					{ID: "TASK-001", SourceFile: "plan_1_completed.md", Criteria: []parser.Criterion{{Checked: false}}},
				},
			},
			{
				ID:   "plan_2",
				File: "plan_2.md",
				Tasks: []parser.Task{
					{ID: "TASK-010", SourceFile: "plan_2.md", Criteria: []parser.Criterion{{Checked: false}}},
				},
			},
		}
		id, _ := findNextTask(plans)
		if id != "TASK-010" {
			t.Errorf("id = %q, want TASK-010", id)
		}
	})

	t.Run("all complete returns empty", func(t *testing.T) {
		plans := []parser.Plan{
			{
				ID:   "plan_1",
				File: "plan_1.md",
				Tasks: []parser.Task{
					{ID: "TASK-001", Criteria: []parser.Criterion{{Checked: true}}},
				},
			},
		}
		id, file := findNextTask(plans)
		if id != "" || file != "" {
			t.Errorf("expected empty, got id=%q file=%q", id, file)
		}
	})

	t.Run("empty plans", func(t *testing.T) {
		id, file := findNextTask(nil)
		if id != "" || file != "" {
			t.Errorf("expected empty, got id=%q file=%q", id, file)
		}
	})
}

func TestRenderStatusPlain(t *testing.T) {
	t.Run("basic output", func(t *testing.T) {
		plans := []parser.Plan{
			{
				ID:   "plan_1",
				File: "plan_1.md",
				Tasks: []parser.Task{
					{ID: "TASK-001", Title: "First task", SourceFile: "plan_1.md", Criteria: []parser.Criterion{{Checked: true}}},
					{ID: "TASK-002", Title: "Second task", SourceFile: "plan_1.md", Criteria: []parser.Criterion{{Checked: false}}},
					{ID: "TASK-003", Title: "Blocked task", SourceFile: "plan_1.md", Criteria: []parser.Criterion{{Text: "BLOCKED: dep", Blocked: true}}},
				},
			},
		}
		var sb strings.Builder
		renderStatusPlain(&sb, plans, false, "TASK-002", "plan_1.md", "claude", nil, false)
		out := sb.String()

		if !strings.Contains(out, "1 features (1 active), 3 tasks total") {
			t.Error("missing header summary")
		}
		if !strings.Contains(out, "1/3 tasks complete") {
			t.Error("missing completion count")
		}
		if !strings.Contains(out, "1 pending") {
			t.Error("missing pending count")
		}
		if !strings.Contains(out, "1 blocked") {
			t.Error("missing blocked count")
		}
		if !strings.Contains(out, "Agent: claude") {
			t.Error("missing agent name")
		}
		if !strings.Contains(out, "[x]  TASK-001: First task") {
			t.Error("missing completed task")
		}
		if !strings.Contains(out, "-> o  TASK-002: Second task") {
			t.Error("missing next task indicator")
		}
		if !strings.Contains(out, "[!]  TASK-003: Blocked task") {
			t.Error("missing blocked task")
		}
		if !strings.Contains(out, "BLOCKED: dep") {
			t.Error("missing blocked reason")
		}
		if !strings.Contains(out, "Features") {
			t.Error("missing Features section")
		}
		if !strings.Contains(out, "plan_1.md") {
			t.Error("missing plan filename in table")
		}
	})

	t.Run("completed plan hidden when showAll false", func(t *testing.T) {
		plans := []parser.Plan{
			{ID: "plan_1", File: "plan_1_completed.md", Completed: true, Tasks: []parser.Task{
				{ID: "TASK-001", Criteria: []parser.Criterion{{Checked: true}}},
			}},
			{ID: "plan_2", File: "plan_2.md", Tasks: []parser.Task{
				{ID: "TASK-010", Title: "Active", SourceFile: "plan_2.md", Criteria: []parser.Criterion{{Checked: false}}},
			}},
		}
		var sb strings.Builder
		renderStatusPlain(&sb, plans, false, "TASK-010", "plan_2.md", "claude", nil, false)
		out := sb.String()

		if strings.Contains(out, "plan_1_completed.md") {
			t.Error("completed plan should be hidden when showAll is false")
		}
		if !strings.Contains(out, "plan_2.md") {
			t.Error("active plan should be visible")
		}
	})

	t.Run("completed plan shown when showAll true", func(t *testing.T) {
		plans := []parser.Plan{
			{ID: "plan_1", File: "plan_1_completed.md", Completed: true, Tasks: []parser.Task{
				{ID: "TASK-001", Criteria: []parser.Criterion{{Checked: true}}},
			}},
		}
		var sb strings.Builder
		renderStatusPlain(&sb, plans, true, "", "", "claude", nil, false)
		out := sb.String()

		if !strings.Contains(out, "plan_1_completed.md") {
			t.Error("completed plan should be visible when showAll is true")
		}
		if !strings.Contains(out, "(archived)") {
			t.Error("completed plan should show (archived)")
		}
	})

	t.Run("unapproved plan shown with marker", func(t *testing.T) {
		plans := []parser.Plan{
			{ID: "plan_1", File: "plan_1.md", Tasks: []parser.Task{
				{ID: "TASK-001", Title: "Unapproved task", SourceFile: "plan_1.md", Criteria: []parser.Criterion{{Checked: false}}},
			}},
		}
		// opt-in mode with no approvals = unapproved
		var sb strings.Builder
		renderStatusPlain(&sb, plans, false, "", "", "claude", nil, true)
		out := sb.String()

		if !strings.Contains(out, "[✗]") {
			t.Error("unapproved plan should show [✗] marker")
		}
		if !strings.Contains(out, "(unapproved)") {
			t.Error("unapproved plan should show (unapproved)")
		}
		if !strings.Contains(out, "unapproved") {
			t.Error("unapproved plan should show 'unapproved' suffix in features table")
		}
	})

	t.Run("empty plans", func(t *testing.T) {
		var sb strings.Builder
		renderStatusPlain(&sb, nil, false, "", "", "claude", nil, false)
		out := sb.String()

		if !strings.Contains(out, "0 features (0 active), 0 tasks total") {
			t.Error("empty features should show zero counts")
		}
	})

	t.Run("plans table status labels", func(t *testing.T) {
		plans := []parser.Plan{
			{ID: "plan_new", File: "plan_new.md", Tasks: []parser.Task{
				{ID: "TASK-001", Criteria: []parser.Criterion{{Checked: false}}},
			}},
			{ID: "plan_progress", File: "plan_progress.md", Tasks: []parser.Task{
				{ID: "TASK-001", Criteria: []parser.Criterion{{Checked: true}}},
				{ID: "TASK-002", Criteria: []parser.Criterion{{Checked: false}}},
			}},
			{ID: "plan_done", File: "plan_done.md", Tasks: []parser.Task{
				{ID: "TASK-001", Criteria: []parser.Criterion{{Checked: true}}},
			}},
			{ID: "plan_blocked", File: "plan_blocked.md", Tasks: []parser.Task{
				{ID: "TASK-001", Criteria: []parser.Criterion{{Text: "BLOCKED: x", Blocked: true}}},
			}},
		}
		var sb strings.Builder
		renderStatusPlain(&sb, plans, false, "TASK-001", "", "claude", nil, false)
		out := sb.String()

		if !strings.Contains(out, "new") {
			t.Error("plan with 0 done should show 'new'")
		}
		if !strings.Contains(out, "in progress") {
			t.Error("partially done plan should show 'in progress'")
		}
		if !strings.Contains(out, "blocked") {
			t.Error("plan with blocked tasks should show 'blocked'")
		}
	})
}

func TestNewStatusModel(t *testing.T) {
	plans := []parser.Plan{
		{
			ID:   "plan_1",
			File: "plan_1.md",
			Tasks: []parser.Task{
				{ID: "TASK-001", Criteria: []parser.Criterion{{Checked: true}}},
				{ID: "TASK-002", Criteria: []parser.Criterion{{Checked: false}}},
			},
		},
	}

	t.Run("initializes with correct fields", func(t *testing.T) {
		m := newStatusModel(plans, false, "TASK-002", "plan_1.md", "claude", "/tmp", false, false, nil, nil, nil)
		if m.nextTaskID != "TASK-002" {
			t.Errorf("nextTaskID = %q, want TASK-002", m.nextTaskID)
		}
		if m.agentName != "claude" {
			t.Errorf("agentName = %q, want claude", m.agentName)
		}
		if m.showAll {
			t.Error("showAll should be false")
		}
		if len(m.Tasks) != 1 {
			t.Fatalf("Tasks len = %d, want 1", len(m.Tasks))
		}
		if m.Tasks[0].ID != "TASK-002" {
			t.Errorf("Tasks[0].ID = %q, want TASK-002", m.Tasks[0].ID)
		}
	})

	t.Run("showAll includes complete tasks", func(t *testing.T) {
		m := newStatusModel(plans, true, "TASK-002", "plan_1.md", "claude", "/tmp", false, false, nil, nil, nil)
		if len(m.Tasks) != 2 {
			t.Errorf("Tasks len = %d, want 2", len(m.Tasks))
		}
	})

	t.Run("empty plans", func(t *testing.T) {
		m := newStatusModel(nil, false, "", "", "claude", "/tmp", false, false, nil, nil, nil)
		if len(m.Tasks) != 0 {
			t.Errorf("Tasks len = %d, want 0", len(m.Tasks))
		}
	})
}

func TestNewStatusModel_InitialDimensions(t *testing.T) {
	// In a non-TTY test environment xterm.GetSize returns 0,0 and newStatusModel
	// initializes to those values. The key invariant is that m.taskListComponent.Width
	// always mirrors m.width (HandleResize was called in the constructor).
	m := newStatusModel(nil, false, "", "", "claude", "/tmp", false, false, nil, nil, nil)
	if m.taskListComponent.Width != m.width {
		t.Errorf("constructor: taskListComponent.Width (%d) != m.width (%d); HandleResize not called", m.taskListComponent.Width, m.width)
	}
	if m.taskListComponent.Height != m.height {
		t.Errorf("constructor: taskListComponent.Height (%d) != m.height (%d); HandleResize not called", m.taskListComponent.Height, m.height)
	}
}

func TestStatusModel_WindowSizeMsgUpdatesDimensions(t *testing.T) {
	// WindowSizeMsg must update m.width/m.height and forward to HandleResize (regression guard).
	m := newStatusModel(nil, false, "", "", "claude", "/tmp", false, false, nil, nil, nil)
	model, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	updated := model.(statusModel)

	if updated.width != 100 {
		t.Errorf("width = %d, want 100", updated.width)
	}
	if updated.height != 30 {
		t.Errorf("height = %d, want 30", updated.height)
	}
	if updated.taskListComponent.Width != 100 {
		t.Errorf("taskListComponent.Width = %d, want 100", updated.taskListComponent.Width)
	}
	if updated.taskListComponent.Height != 30 {
		t.Errorf("taskListComponent.Height = %d, want 30", updated.taskListComponent.Height)
	}
}

func TestEnsureCursorVisible(t *testing.T) {
	m := statusModel{
		taskListComponent: taskListComponent{
			Width:       80,
			Height:      30,
			Cursor:      0,
			HeaderLines: statusHeaderLines,
		},
	}
	m.ensureCursorVisible()
	if m.ScrollOffset < 0 {
		t.Errorf("ScrollOffset = %d, should not be negative", m.ScrollOffset)
	}

	m.Cursor = 100
	m.ScrollOffset = 0
	m.ensureCursorVisible()
	if m.ScrollOffset <= 0 {
		t.Errorf("ScrollOffset should advance when cursor is beyond visible range, got %d", m.ScrollOffset)
	}

	m.ScrollOffset = 50
	m.Cursor = 10
	m.ensureCursorVisible()
	if m.ScrollOffset > m.Cursor {
		t.Errorf("ScrollOffset (%d) should not exceed Cursor (%d)", m.ScrollOffset, m.Cursor)
	}
}

func TestStatusModel_InitReturnsCmd(t *testing.T) {
	m := newStatusModel(nil, false, "", "", "claude", "/tmp", false, false, nil, nil, nil)
	cmd := m.Init()
	if cmd == nil {
		t.Error("Init() should return a non-nil Cmd for async 2x fetch")
	}
}

func TestStatusModel_UpdateClaude2xResult(t *testing.T) {
	plans := []parser.Plan{
		{ID: "plan_1", File: "plan_1.md", Tasks: []parser.Task{
			{ID: "TASK-001", Criteria: []parser.Criterion{{Checked: false}}},
		}},
	}

	t.Run("2x active sets is2x and BorderColor to Warning", func(t *testing.T) {
		m := newStatusModel(plans, false, "TASK-001", "plan_1.md", "claude", "/tmp", false, false, nil, nil, nil)
		msg := claude2xResultMsg{status: claude2x.Status{Is2x: true, TwoXWindowExpiresIn: "5h"}}
		result, _ := m.Update(msg)
		updated := result.(statusModel)
		if !updated.is2x {
			t.Error("is2x should be true")
		}
		if updated.BorderColor != styles.Warning {
			t.Errorf("BorderColor = %q, want %q (Warning/yellow)", updated.BorderColor, styles.Warning)
		}
	})

	t.Run("2x inactive keeps is2x false and BorderColor Primary", func(t *testing.T) {
		m := newStatusModel(plans, false, "TASK-001", "plan_1.md", "claude", "/tmp", false, false, nil, nil, nil)
		msg := claude2xResultMsg{status: claude2x.Status{Is2x: false}}
		result, _ := m.Update(msg)
		updated := result.(statusModel)
		if updated.is2x {
			t.Error("is2x should be false")
		}
		if updated.BorderColor != styles.Primary {
			t.Errorf("BorderColor = %q, want %q (Primary/cyan)", updated.BorderColor, styles.Primary)
		}
	})
}

func TestStatusModel_ViewBorderColor(t *testing.T) {
	plans := []parser.Plan{
		{ID: "plan_1", File: "plan_1.md", Tasks: []parser.Task{
			{ID: "TASK-001", Title: "Test", Criteria: []parser.Criterion{{Checked: false}}},
		}},
	}

	t.Run("non-2x view does not contain yellow border styling", func(t *testing.T) {
		m := newStatusModel(plans, false, "TASK-001", "plan_1.md", "claude", "/tmp", false, false, nil, nil, nil)
		m.is2x = false
		m.width = 120
		m.height = 40
		m.activeTab = 1 // Item Details tab shows task list
		m.rebuildForSelectedPlan()
		view := m.View()
		if !strings.Contains(view, "Item Details") {
			t.Error("view should contain 'Item Details' tab")
		}
		if !strings.Contains(view, "TASK-001") {
			t.Error("view should contain task ID")
		}
	})

	t.Run("2x view renders without error", func(t *testing.T) {
		m := newStatusModel(plans, false, "TASK-001", "plan_1.md", "claude", "/tmp", false, false, nil, nil, nil)
		m.is2x = true
		m.width = 120
		m.height = 40
		m.activeTab = 1 // Item Details tab shows task list
		m.rebuildForSelectedPlan()
		view := m.View()
		if !strings.Contains(view, "Item Details") {
			t.Error("view should contain 'Item Details' tab")
		}
		if !strings.Contains(view, "TASK-001") {
			t.Error("view should contain task ID")
		}
	})

	t.Run("empty features view renders with 2x", func(t *testing.T) {
		m := newStatusModel(nil, false, "", "", "claude", "/tmp", false, false, nil, nil, nil)
		m.is2x = true
		m.width = 120
		m.height = 40
		view := m.View()
		if !strings.Contains(view, "No features found") {
			t.Error("empty view should contain 'No features found'")
		}
	})
}

func TestFindNextTask_BugsPrioritized(t *testing.T) {
	t.Run("bugs before features", func(t *testing.T) {
		items := []parser.Plan{
			{
				ID:   "feature_001",
				File: "feature_001.md",
				Tasks: []parser.Task{
					{ID: "TASK-001-001", SourceFile: "feature_001.md", Criteria: []parser.Criterion{{Checked: false}}},
				},
			},
			{
				ID:    "bug_001",
				File:  "bug_001.md",
				IsBug: true,
				Tasks: []parser.Task{
					{ID: "BUG-001-001", SourceFile: "bug_001.md", Criteria: []parser.Criterion{{Checked: false}}},
				},
			},
		}
		id, file := findNextTask(items)
		if id != "BUG-001-001" {
			t.Errorf("id = %q, want BUG-001-001 (bugs should be prioritized)", id)
		}
		if file != "bug_001.md" {
			t.Errorf("file = %q, want bug_001.md", file)
		}
	})

	t.Run("falls back to features when all bugs complete", func(t *testing.T) {
		items := []parser.Plan{
			{
				ID:   "feature_001",
				File: "feature_001.md",
				Tasks: []parser.Task{
					{ID: "TASK-001-001", SourceFile: "feature_001.md", Criteria: []parser.Criterion{{Checked: false}}},
				},
			},
			{
				ID:        "bug_001",
				File:      "bug_001.md",
				IsBug:     true,
				Completed: true,
				Tasks: []parser.Task{
					{ID: "BUG-001-001", SourceFile: "bug_001.md", Criteria: []parser.Criterion{{Checked: true}}},
				},
			},
		}
		id, _ := findNextTask(items)
		if id != "TASK-001-001" {
			t.Errorf("id = %q, want TASK-001-001", id)
		}
	})

	t.Run("only bugs", func(t *testing.T) {
		items := []parser.Plan{
			{
				ID:    "bug_001",
				File:  "bug_001.md",
				IsBug: true,
				Tasks: []parser.Task{
					{ID: "BUG-001-001", SourceFile: "bug_001.md", Criteria: []parser.Criterion{{Checked: false}}},
				},
			},
		}
		id, _ := findNextTask(items)
		if id != "BUG-001-001" {
			t.Errorf("id = %q, want BUG-001-001", id)
		}
	})
}

func TestVisibleFeatures_WithBugs(t *testing.T) {
	items := []parser.Plan{
		{ID: "feature_001", File: "feature_001.md"},
		{ID: "bug_001", File: "bug_001.md", IsBug: true},
		{ID: "bug_002", File: "bug_002_completed.md", IsBug: true, Completed: true},
	}

	t.Run("showAll false hides completed bugs", func(t *testing.T) {
		m := statusModel{plans: items, showAll: false}
		got := m.visiblePlans()
		if len(got) != 2 {
			t.Fatalf("len = %d, want 2", len(got))
		}
		if got[1].File != "bug_001.md" {
			t.Errorf("got %s, want bug_001.md", got[1].File)
		}
	})

	t.Run("showAll true shows completed bugs", func(t *testing.T) {
		m := statusModel{plans: items, showAll: true}
		got := m.visiblePlans()
		if len(got) != 3 {
			t.Fatalf("len = %d, want 3", len(got))
		}
	})
}

func TestRenderStatusPlain_WithBugs(t *testing.T) {
	t.Run("mixed features and bugs", func(t *testing.T) {
		items := []parser.Plan{
			{
				ID:   "feature_001",
				File: "feature_001.md",
				Tasks: []parser.Task{
					{ID: "TASK-001-001", Title: "Feature task", SourceFile: "f", Criteria: []parser.Criterion{{Checked: false}}},
				},
			},
			{
				ID:    "bug_001",
				File:  "bug_001.md",
				IsBug: true,
				Tasks: []parser.Task{
					{ID: "BUG-001-001", Title: "Bug task", SourceFile: "b", Criteria: []parser.Criterion{{Checked: false}}},
				},
			},
		}
		var sb strings.Builder
		renderStatusPlain(&sb, items, false, "BUG-001-001", "b", "claude", nil, false)
		out := sb.String()

		if !strings.Contains(out, "1 features (1 active)") {
			t.Error("missing feature count in header")
		}
		if !strings.Contains(out, "1 bugs (1 active)") {
			t.Error("missing bug count in header")
		}
		if !strings.Contains(out, "bug_001.md") {
			t.Error("missing bug filename in tasks")
		}
		if !strings.Contains(out, "BUG-001-001") {
			t.Error("missing bug task ID")
		}
	})

	t.Run("no bugs omits bug count", func(t *testing.T) {
		items := []parser.Plan{
			{
				ID:   "feature_001",
				File: "feature_001.md",
				Tasks: []parser.Task{
					{ID: "TASK-001-001", Title: "Task", SourceFile: "f", Criteria: []parser.Criterion{{Checked: false}}},
				},
			},
		}
		var sb strings.Builder
		renderStatusPlain(&sb, items, false, "TASK-001-001", "f", "claude", nil, false)
		out := sb.String()

		if strings.Contains(out, "bugs") {
			t.Error("should not mention bugs when there are none")
		}
	})
}

func TestNewStatusModel_WithBugs(t *testing.T) {
	items := []parser.Plan{
		{
			ID:   "feature_001",
			File: "feature_001.md",
			Tasks: []parser.Task{
				{ID: "TASK-001-001", Criteria: []parser.Criterion{{Checked: false}}},
			},
		},
		{
			ID:    "bug_001",
			File:  "bug_001.md",
			IsBug: true,
			Tasks: []parser.Task{
				{ID: "BUG-001-001", Criteria: []parser.Criterion{{Checked: false}}},
			},
		},
	}

	t.Run("tabs include bugs", func(t *testing.T) {
		m := newStatusModel(items, false, "BUG-001-001", "bug_001.md", "claude", "/tmp", false, false, nil, nil, nil)
		visible := m.visiblePlans()
		if len(visible) != 2 {
			t.Fatalf("visible features = %d, want 2", len(visible))
		}
		if !visible[1].IsBug {
			t.Error("second visible item should be a bug")
		}
	})

	t.Run("navigation to bug tab", func(t *testing.T) {
		m := newStatusModel(items, false, "BUG-001-001", "bug_001.md", "claude", "/tmp", false, false, nil, nil, nil)
		m.planCursor = 1
		m.rebuildForSelectedPlan()
		if len(m.Tasks) != 1 {
			t.Fatalf("Tasks len = %d, want 1", len(m.Tasks))
		}
		if m.Tasks[0].ID != "BUG-001-001" {
			t.Errorf("Tasks[0].ID = %q, want BUG-001-001", m.Tasks[0].ID)
		}
	})
}

func TestStatusModel_ViewWithBugs(t *testing.T) {
	items := []parser.Plan{
		{
			ID:   "feature_001",
			File: "feature_001.md",
			Tasks: []parser.Task{
				{ID: "TASK-001-001", Title: "Feature task", Criteria: []parser.Criterion{{Checked: false}}},
			},
		},
		{
			ID:    "bug_001",
			File:  "bug_001.md",
			IsBug: true,
			Tasks: []parser.Task{
				{ID: "BUG-001-001", Title: "Bug task", Criteria: []parser.Criterion{{Checked: false}}},
			},
		},
	}

	t.Run("view renders bug tabs", func(t *testing.T) {
		m := newStatusModel(items, false, "BUG-001-001", "bug_001.md", "claude", "/tmp", false, false, nil, nil, nil)
		m.width = 120
		m.height = 40
		view := m.View()
		if !strings.Contains(view, "bug_001") {
			t.Error("view should contain bug plan label in left pane")
		}
		if !strings.Contains(view, "feature_001") {
			t.Error("view should contain feature plan label in left pane")
		}
	})

	t.Run("view shows bug header counts", func(t *testing.T) {
		m := newStatusModel(items, false, "BUG-001-001", "bug_001.md", "claude", "/tmp", false, false, nil, nil, nil)
		m.width = 120
		m.height = 40
		view := m.View()
		if !strings.Contains(view, "bug_001") {
			t.Error("view should show bug plan in left pane")
		}
	})

	t.Run("selected bug tab shows bug tasks", func(t *testing.T) {
		m := newStatusModel(items, false, "BUG-001-001", "bug_001.md", "claude", "/tmp", false, false, nil, nil, nil)
		m.width = 120
		m.height = 40
		m.activeTab = 1 // Feature Details tab shows task list
		m.planCursor = 1
		m.rebuildForSelectedPlan()
		view := m.View()
		if !strings.Contains(view, "BUG-001-001") {
			t.Error("view should show bug task ID when bug tab is selected")
		}
	})
}

func TestVisiblePlans(t *testing.T) {
	plans := []parser.Plan{
		{ID: "plan_1", File: "plan_1.md"},
		{ID: "plan_2", File: "plan_2_completed.md", Completed: true},
		{ID: "plan_3", File: "plan_3.md"},
	}

	t.Run("showAll false hides completed", func(t *testing.T) {
		m := statusModel{plans: plans, showAll: false}
		got := m.visiblePlans()
		if len(got) != 2 {
			t.Fatalf("len = %d, want 2", len(got))
		}
		if got[0].File != "plan_1.md" || got[1].File != "plan_3.md" {
			t.Errorf("got %s, %s", got[0].File, got[1].File)
		}
	})

	t.Run("showAll true shows all", func(t *testing.T) {
		m := statusModel{plans: plans, showAll: true}
		got := m.visiblePlans()
		if len(got) != 3 {
			t.Fatalf("len = %d, want 3", len(got))
		}
	})

	t.Run("no plans", func(t *testing.T) {
		m := statusModel{plans: nil, showAll: false}
		got := m.visiblePlans()
		if len(got) != 0 {
			t.Errorf("len = %d, want 0", len(got))
		}
	})
}

func TestSelectedPlan(t *testing.T) {
	plans := []parser.Plan{
		{ID: "plan_1", File: "plan_1.md"},
		{ID: "plan_2", File: "plan_2.md"},
		{ID: "plan_3", File: "plan_3_completed.md", Completed: true},
	}

	t.Run("returns plan at planCursor", func(t *testing.T) {
		m := statusModel{plans: plans, planCursor: 1, showAll: true}
		p := m.selectedPlan()
		if p.ID != "plan_2" {
			t.Errorf("selectedPlan() = %q, want plan_2", p.ID)
		}
	})

	t.Run("cursor out of range returns zero plan", func(t *testing.T) {
		m := statusModel{plans: plans, planCursor: 99, showAll: false}
		p := m.selectedPlan()
		if p.ID != "" {
			t.Errorf("selectedPlan() with out-of-range cursor should return zero plan, got %q", p.ID)
		}
	})

	t.Run("respects showAll filter", func(t *testing.T) {
		// showAll=false: plans has 2 visible (plan_1, plan_2)
		m := statusModel{plans: plans, planCursor: 1, showAll: false}
		p := m.selectedPlan()
		if p.ID != "plan_2" {
			t.Errorf("selectedPlan() = %q, want plan_2", p.ID)
		}
	})
}

func TestNewStatusModel_Defaults(t *testing.T) {
	plans := []parser.Plan{
		{ID: "plan_1", File: "plan_1.md", Tasks: []parser.Task{
			{ID: "TASK-001", Criteria: []parser.Criterion{{Checked: false}}},
		}},
	}

	t.Run("leftFocused is true by default", func(t *testing.T) {
		m := newStatusModel(plans, false, "TASK-001", "plan_1.md", "claude", "/tmp", false, false, nil, nil, nil)
		if !m.leftFocused {
			t.Error("leftFocused should be true after newStatusModel")
		}
	})

	t.Run("activeTab is 0 by default", func(t *testing.T) {
		m := newStatusModel(plans, false, "TASK-001", "plan_1.md", "claude", "/tmp", false, false, nil, nil, nil)
		if m.activeTab != 0 {
			t.Errorf("activeTab = %d, want 0", m.activeTab)
		}
	})

	t.Run("plans field is populated", func(t *testing.T) {
		m := newStatusModel(plans, false, "TASK-001", "plan_1.md", "claude", "/tmp", false, false, nil, nil, nil)
		if len(m.plans) != 1 {
			t.Errorf("plans len = %d, want 1", len(m.plans))
		}
	})
}

func TestRebuildForSelectedPlan(t *testing.T) {
	plans := []parser.Plan{
		{ID: "plan_1", File: "plan_1.md", Tasks: []parser.Task{
			{ID: "TASK-001", Criteria: []parser.Criterion{{Checked: false}}},
		}},
		{ID: "plan_2", File: "plan_2.md", Tasks: []parser.Task{
			{ID: "TASK-010", Criteria: []parser.Criterion{{Checked: false}}},
			{ID: "TASK-011", Criteria: []parser.Criterion{{Checked: false}}},
		}},
	}

	t.Run("selects correct plan tasks", func(t *testing.T) {
		m := statusModel{plans: plans, planCursor: 1, showAll: false}
		m.rebuildForSelectedPlan()
		if len(m.Tasks) != 2 {
			t.Fatalf("len = %d, want 2", len(m.Tasks))
		}
		if m.Tasks[0].ID != "TASK-010" {
			t.Errorf("first task = %s, want TASK-010", m.Tasks[0].ID)
		}
	})

	t.Run("resets cursor and scroll", func(t *testing.T) {
		m := statusModel{plans: plans, planCursor: 0}
		m.Cursor = 5
		m.ScrollOffset = 3
		m.rebuildForSelectedPlan()
		if m.Cursor != 0 {
			t.Errorf("Cursor = %d, want 0", m.Cursor)
		}
		if m.ScrollOffset != 0 {
			t.Errorf("ScrollOffset = %d, want 0", m.ScrollOffset)
		}
	})

	t.Run("out of bounds planCursor resets to 0", func(t *testing.T) {
		m := statusModel{plans: plans, planCursor: 99}
		m.rebuildForSelectedPlan()
		if m.planCursor != 0 {
			t.Errorf("planCursor = %d, want 0", m.planCursor)
		}
	})

	t.Run("empty plans", func(t *testing.T) {
		m := statusModel{plans: nil}
		m.rebuildForSelectedPlan()
		if m.Tasks != nil {
			t.Errorf("Tasks should be nil, got %v", m.Tasks)
		}
	})
}

func TestRenderTabBar_BugSeparator(t *testing.T) {
	items := []parser.Plan{
		{ID: "feature_001", File: "feature_001.md", Tasks: []parser.Task{{ID: "T1"}}},
		{ID: "bug_001", File: "bug_001.md", IsBug: true, Tasks: []parser.Task{{ID: "B1"}}},
	}
	m := statusModel{plans: items, showAll: false}
	m.Width = 120
	bar := m.renderTabBar()
	if !strings.Contains(bar, "┃") {
		t.Error("tab bar should contain ┃ separator between features and bugs")
	}
}

func TestRenderCurrentTaskContent(t *testing.T) {
	t.Run("empty task ID returns empty string", func(t *testing.T) {
		content := renderCurrentTaskContent("", "")
		if content != "" {
			t.Errorf("expected empty content for empty task ID, got %q", content)
		}
	})

	t.Run("missing file returns empty string", func(t *testing.T) {
		content := renderCurrentTaskContent("TASK-001-001", "/nonexistent/path/feature.md")
		if content != "" {
			t.Errorf("expected empty content for missing file, got %q", content)
		}
	})

	t.Run("valid task file returns content with task ID and title", func(t *testing.T) {
		dir := t.TempDir()
		taskFile := dir + "/feature_001.md"
		fileContent := `# Feature 001

## Tasks

### TASK-001-001: My Example Task

**Description:** Do something important

**Acceptance Criteria:**
- [ ] Some criterion
`
		if err := os.WriteFile(taskFile, []byte(fileContent), 0644); err != nil {
			t.Fatal(err)
		}

		result := renderCurrentTaskContent("TASK-001-001", taskFile)
		if !strings.Contains(result, "TASK-001-001") {
			t.Errorf("expected task ID in content, got %q", result)
		}
		if !strings.Contains(result, "My Example Task") {
			t.Errorf("expected task title in content, got %q", result)
		}
	})

	t.Run("task not found in file returns empty string", func(t *testing.T) {
		dir := t.TempDir()
		taskFile := dir + "/feature_001.md"
		fileContent := `# Feature 001

## Tasks

### TASK-001-001: Some Task

**Acceptance Criteria:**
- [ ] Criterion
`
		if err := os.WriteFile(taskFile, []byte(fileContent), 0644); err != nil {
			t.Fatal(err)
		}

		result := renderCurrentTaskContent("TASK-999-999", taskFile)
		if result != "" {
			t.Errorf("expected empty content for missing task ID, got %q", result)
		}
	})
}

func TestRenderCurrentTaskTab(t *testing.T) {
	t.Run("no pending task shows No pending tasks message", func(t *testing.T) {
		m := statusModel{nextTaskID: "", nextTaskFile: ""}
		content := m.renderCurrentTaskTab(80, 20)
		if !strings.Contains(content, "No pending tasks") {
			t.Errorf("expected 'No pending tasks', got %q", content)
		}
	})

	t.Run("with pending task renders viewport content", func(t *testing.T) {
		dir := t.TempDir()
		taskFile := dir + "/feature_001.md"
		fileContent := `# Feature 001

## Tasks

### TASK-001-001: My Example Task

**Description:** Do something

**Acceptance Criteria:**
- [ ] Some criterion
`
		if err := os.WriteFile(taskFile, []byte(fileContent), 0644); err != nil {
			t.Fatal(err)
		}

		m := statusModel{nextTaskID: "TASK-001-001", nextTaskFile: taskFile}
		m.currentTaskViewport.Width = 80
		m.currentTaskViewport.Height = 20
		m.loadCurrentTaskDetail()
		content := m.renderCurrentTaskTab(80, 20)
		if !strings.Contains(content, "TASK-001-001") {
			t.Errorf("expected task ID in Tab 3 output, got %q", content)
		}
	})
}

func TestRenderCurrentTaskTab_Tab3Active(t *testing.T) {
	t.Run("Tab 3 is wired into renderRightPane", func(t *testing.T) {
		m := statusModel{nextTaskID: "", nextTaskFile: "", activeTab: 2, width: 120, height: 30}
		// Should render "No pending tasks" not "(coming soon)"
		content := m.renderRightPane(80, 20)
		if strings.Contains(content, "coming soon") {
			t.Error("Tab 3 should not show 'coming soon' placeholder")
		}
		if !strings.Contains(content, "No pending tasks") {
			t.Errorf("Tab 3 with no task should show 'No pending tasks', got %q", content)
		}
	})
}

func TestRenderTabBar_ApprovalMark(t *testing.T) {
	t.Run("approved feature shows checkmark", func(t *testing.T) {
		items := []parser.Plan{
			{ID: "feature_001", File: "feature_001.md", Tasks: []parser.Task{{ID: "T1"}}},
		}
		// opt-out mode, no explicit unapproval → approved
		m := statusModel{plans: items, showAll: false, approvalRequired: false}
		m.Width = 120
		bar := m.renderTabBar()
		if !strings.Contains(bar, "✓") {
			t.Error("approved feature tab should show ✓ mark")
		}
		if strings.Contains(bar, "✗") {
			t.Error("approved feature tab should not show ✗ mark")
		}
	})

	t.Run("unapproved feature shows cross", func(t *testing.T) {
		items := []parser.Plan{
			{ID: "feature_001", File: "feature_001.md", Tasks: []parser.Task{{ID: "T1"}}},
		}
		// opt-in mode with no approvals → unapproved
		m := statusModel{plans: items, showAll: false, approvalRequired: true}
		m.Width = 120
		bar := m.renderTabBar()
		if !strings.Contains(bar, "✗") {
			t.Error("unapproved feature tab should show ✗ mark")
		}
		if strings.Contains(bar, "✓") {
			t.Error("unapproved feature tab should not show ✓ mark")
		}
	})

	t.Run("mixed approved and unapproved", func(t *testing.T) {
		items := []parser.Plan{
			{ID: "feature_001", File: "feature_001.md", Tasks: []parser.Task{{ID: "T1"}}},
			{ID: "feature_002", File: "feature_002.md", Tasks: []parser.Task{{ID: "T2"}}},
		}
		// opt-in mode: only feature_001 is approved
		m := statusModel{
			plans:            items,
			showAll:          false,
			approvalRequired: true,
			approvals:        approval.Approvals{"feature_001": true},
		}
		m.Width = 120
		bar := m.renderTabBar()
		if !strings.Contains(bar, "✓") {
			t.Error("tab bar should contain ✓ for approved feature")
		}
		if !strings.Contains(bar, "✗") {
			t.Error("tab bar should contain ✗ for unapproved feature")
		}
	})
}

func TestStatusModel_LeftPaneUpDownNavigation(t *testing.T) {
	plans := []parser.Plan{
		{ID: "plan_1", File: "plan_1.md", Tasks: []parser.Task{{ID: "T1"}}},
		{ID: "plan_2", File: "plan_2.md", Tasks: []parser.Task{{ID: "T2"}}},
		{ID: "plan_3", File: "plan_3.md", Tasks: []parser.Task{{ID: "T3"}}},
	}

	t.Run("down navigates to next plan", func(t *testing.T) {
		m := newStatusModel(plans, false, "", "", "claude", "/tmp", false, false, nil, nil, nil)
		m.width = 120
		m.height = 40
		m.leftFocused = true

		result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
		got := result.(statusModel)
		if got.planCursor != 1 {
			t.Errorf("planCursor = %d, want 1", got.planCursor)
		}
	})

	t.Run("up navigates to previous plan", func(t *testing.T) {
		m := newStatusModel(plans, false, "", "", "claude", "/tmp", false, false, nil, nil, nil)
		m.width = 120
		m.height = 40
		m.leftFocused = true
		m.planCursor = 2

		result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
		got := result.(statusModel)
		if got.planCursor != 1 {
			t.Errorf("planCursor = %d, want 1", got.planCursor)
		}
	})

	t.Run("down wraps at last plan", func(t *testing.T) {
		m := newStatusModel(plans, false, "", "", "claude", "/tmp", false, false, nil, nil, nil)
		m.width = 120
		m.height = 40
		m.leftFocused = true
		m.planCursor = 2

		result, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
		got := result.(statusModel)
		// CursorDown wraps: from last index stays or wraps to 0
		if got.planCursor < 0 || got.planCursor >= len(plans) {
			t.Errorf("planCursor = %d out of range [0, %d)", got.planCursor, len(plans))
		}
	})

	t.Run("up does not navigate when right pane is focused", func(t *testing.T) {
		m := newStatusModel(plans, false, "", "", "claude", "/tmp", false, false, nil, nil, nil)
		m.width = 120
		m.height = 40
		m.leftFocused = false
		m.activeTab = 0
		m.planCursor = 1

		result, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
		got := result.(statusModel)
		// Right pane focused: up scrolls log, not plan navigation
		if got.planCursor != 1 {
			t.Errorf("planCursor = %d, want 1 (should not change when right pane focused)", got.planCursor)
		}
	})
}

func TestStatusModel_LeftPaneEnterSwitchesFocus(t *testing.T) {
	plans := []parser.Plan{
		{ID: "plan_1", File: "plan_1.md", Tasks: []parser.Task{{ID: "T1"}}},
	}

	t.Run("enter switches focus to right pane on Tab 2", func(t *testing.T) {
		m := newStatusModel(plans, false, "", "", "claude", "/tmp", false, false, nil, nil, nil)
		m.width = 120
		m.height = 40
		m.leftFocused = true

		result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		got := result.(statusModel)
		if got.leftFocused {
			t.Error("leftFocused should be false after enter from left pane")
		}
		if got.activeTab != 1 {
			t.Errorf("activeTab = %d, want 1 (Feature Details)", got.activeTab)
		}
	})

	t.Run("enter in split mode sets tab 2", func(t *testing.T) {
		m := newStatusModel(plans, false, "", "", "claude", "/tmp", false, false, nil, nil, nil)
		m.width = 120
		m.height = 40
		m.leftFocused = true
		m.activeTab = 3 // currently on metrics

		result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		got := result.(statusModel)
		if got.activeTab != 1 {
			t.Errorf("activeTab = %d, want 1 after enter from left pane", got.activeTab)
		}
	})
}

func TestStatusModel_TabTogglesFocus(t *testing.T) {
	plans := []parser.Plan{
		{ID: "plan_1", File: "plan_1.md", Tasks: []parser.Task{{ID: "T1"}}},
	}

	t.Run("tab is a no-op in split mode", func(t *testing.T) {
		m := newStatusModel(plans, false, "", "", "claude", "/tmp", false, false, nil, nil, nil)
		m.width = 120
		m.height = 40
		m.leftFocused = true

		result, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
		got := result.(statusModel)
		if !got.leftFocused {
			t.Error("leftFocused should remain true after tab (tab is a no-op)")
		}
	})
}

// TestRenderLeftPane_LineCount verifies that renderLeftPane(w, h) always returns
// exactly h+1 lines: h content lines plus one bottom-separator line.
func TestRenderLeftPane_LineCount(t *testing.T) {
	m := statusModel{}
	for _, h := range []int{5, 10, 20, 34} {
		out := m.renderLeftPane(40, h)
		got := strings.Count(out, "\n") + 1
		want := h + 1
		if got != want {
			t.Errorf("renderLeftPane(40, %d): got %d lines, want %d", h, got, want)
		}
	}
}

// TestRenderRightPane_LineCount verifies that renderRightPane(w, h) returns exactly
// h+1 lines when h is large enough that tab content fits within contentH = h-2.
// The plain log tab requires contentH >= 4 (fixed overhead: blank+title+sep+no-active),
// so the minimum safe h is 6.
func TestRenderRightPane_LineCount(t *testing.T) {
	m := statusModel{width: 120, height: 40}
	for _, h := range []int{6, 10, 20, 34} {
		out := m.renderRightPane(80, h)
		got := strings.Count(out, "\n") + 1
		want := h + 1
		if got != want {
			t.Errorf("renderRightPane(80, %d): got %d lines, want %d", h, got, want)
		}
	}
}

// TestRightPaneContentHeight_EqualsInnerHMinus3 verifies that rightPaneContentHeight
// returns innerH-3, matching the contentH that renderRightPane computes when
// viewStatusSplit passes innerH-1 as the height argument (height-2 = innerH-3).
func TestRightPaneContentHeight_EqualsInnerHMinus3(t *testing.T) {
	m := statusModel{width: 120, height: 40}
	_, innerH := styles.FullScreenInnerSize(m.width, m.height)
	got := m.rightPaneContentHeight()
	want := innerH - 3
	if got != want {
		t.Errorf("rightPaneContentHeight() = %d, want innerH-3 = %d (innerH=%d, terminal 120x40)",
			got, want, innerH)
	}
}

func TestHasCompletedPlans(t *testing.T) {
	t.Run("no plans", func(t *testing.T) {
		m := statusModel{}
		if m.hasCompletedPlans() {
			t.Error("expected false for empty plans")
		}
	})

	t.Run("no completed plans", func(t *testing.T) {
		m := statusModel{plans: []parser.Plan{
			{ID: "feature_001", Completed: false},
			{ID: "feature_002", Completed: false},
		}}
		if m.hasCompletedPlans() {
			t.Error("expected false when no plans are completed")
		}
	})

	t.Run("has completed plan", func(t *testing.T) {
		m := statusModel{plans: []parser.Plan{
			{ID: "feature_001", Completed: false},
			{ID: "feature_002", Completed: true},
		}}
		if !m.hasCompletedPlans() {
			t.Error("expected true when at least one plan is completed")
		}
	})
}

func TestMigrateApprovalKeys_MigratesFilenameToUUID(t *testing.T) {
	plans := []parser.Plan{
		{ID: "feature_001", MaggusID: "uuid-abc"},
	}
	a := approval.Approvals{"feature_001": true}

	migrated := migrateApprovalKeys(plans, a)

	if !migrated {
		t.Error("expected migrated=true")
	}
	if _, ok := a["feature_001"]; ok {
		t.Error("expected filename key to be removed after migration")
	}
	if !a["uuid-abc"] {
		t.Error("expected UUID key to be approved after migration")
	}
}

func TestMigrateApprovalKeys_NoMaggusID(t *testing.T) {
	plans := []parser.Plan{
		{ID: "feature_001"}, // no MaggusID
	}
	a := approval.Approvals{"feature_001": true}

	migrated := migrateApprovalKeys(plans, a)

	if migrated {
		t.Error("expected migrated=false when plan has no MaggusID")
	}
	if !a["feature_001"] {
		t.Error("expected filename key to remain unchanged")
	}
}

func TestMigrateApprovalKeys_AlreadyUnderUUID(t *testing.T) {
	plans := []parser.Plan{
		{ID: "feature_001", MaggusID: "uuid-abc"},
	}
	a := approval.Approvals{"uuid-abc": true} // already under UUID key

	migrated := migrateApprovalKeys(plans, a)

	if migrated {
		t.Error("expected migrated=false when UUID key already exists")
	}
}

func TestMigrateApprovalKeys_BothKeysPresent_NoOverwrite(t *testing.T) {
	// When both filename and UUID keys exist, do not overwrite the UUID entry.
	plans := []parser.Plan{
		{ID: "feature_001", MaggusID: "uuid-abc"},
	}
	a := approval.Approvals{
		"feature_001": true,
		"uuid-abc":    false, // UUID key already present with different value
	}

	migrated := migrateApprovalKeys(plans, a)

	if migrated {
		t.Error("expected migrated=false when UUID key already present")
	}
	if a["uuid-abc"] {
		t.Error("expected UUID key value to remain false")
	}
}

func TestMigrateApprovalKeys_PreventsStalePrune(t *testing.T) {
	// After migration, pruneStaleApprovals should find the UUID key and NOT remove it.
	dir := setupApproveDir(t)

	const uuid = "migrate-prune-uuid"
	// Simulate: approval stored under filename key before maggus-id was added.
	if err := approval.Save(dir, approval.Approvals{"feature_001": true}); err != nil {
		t.Fatal(err)
	}

	plans := []parser.Plan{
		{ID: "feature_001", MaggusID: uuid, File: filepath.Join(dir, ".maggus", "features", "feature_001.md")},
	}

	// Migration step: move filename key to UUID key.
	a, err := approval.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if migrateApprovalKeys(plans, a) {
		if err := approval.Save(dir, a); err != nil {
			t.Fatal(err)
		}
	}

	// After migration, prune should NOT remove the UUID entry.
	pruneStaleApprovals(dir, plans)

	loaded, err := approval.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if val, ok := loaded[uuid]; !ok || !val {
		t.Errorf("expected UUID approval to survive prune after migration, got: %v", loaded)
	}
}

func TestHandleApproveToggle_NoEntry_OptOut_WritesTrue(t *testing.T) {
	// In opt-out mode, plan with no approval entry is approved by default.
	// Pressing 'a' must write explicit true, NOT false (additive-only toggle).
	dir := setupApproveDir(t)
	// UUID must use hex characters only ([0-9a-f-]) so ParseMaggusID can parse it.
	const uuid = "00000001-0000-4000-8000-000000000001"
	writeApproveFeature(t, dir, "feature_001.md", uuid)

	plan := parser.Plan{
		ID:       "feature_001",
		MaggusID: uuid,
		File:     filepath.Join(dir, ".maggus", "features", "feature_001.md"),
	}
	m := statusModel{
		dir:              dir,
		plans:            []parser.Plan{plan},
		approvals:        approval.Approvals{}, // no entry
		approvalRequired: false,                // opt-out mode
		leftFocused:      true,
		featureStore:     stores.NewFileFeatureStore(dir),
		bugStore:         stores.NewFileBugStore(dir),
	}

	result, _ := m.handleApproveToggle()
	newM := result.(statusModel)

	a, err := approval.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if val, ok := a[uuid]; !ok || !val {
		t.Errorf("expected explicit true written (not false), got approvals: %v", a)
	}
	if newM.statusNote != "feature approved" {
		t.Errorf("expected 'feature approved' note, got: %q", newM.statusNote)
	}
}

func TestHandleApproveToggle_ExplicitTrue_RemovesEntry(t *testing.T) {
	// When an explicit true entry exists, pressing 'a' removes it (back to default).
	dir := setupApproveDir(t)
	const uuid = "00000002-0000-4000-8000-000000000002"
	writeApproveFeature(t, dir, "feature_001.md", uuid)
	if err := approval.Approve(dir, uuid); err != nil {
		t.Fatal(err)
	}

	plan := parser.Plan{
		ID:       "feature_001",
		MaggusID: uuid,
		File:     filepath.Join(dir, ".maggus", "features", "feature_001.md"),
	}
	m := statusModel{
		dir:              dir,
		plans:            []parser.Plan{plan},
		approvals:        approval.Approvals{uuid: true},
		approvalRequired: false,
		leftFocused:      true,
		featureStore:     stores.NewFileFeatureStore(dir),
		bugStore:         stores.NewFileBugStore(dir),
	}

	result, _ := m.handleApproveToggle()
	newM := result.(statusModel)

	a, err := approval.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := a[uuid]; ok {
		t.Errorf("expected entry to be removed, got: %v", a)
	}
	if newM.statusNote != "feature approval removed" {
		t.Errorf("expected 'feature approval removed' note, got: %q", newM.statusNote)
	}
}

func TestHandleApproveToggle_ExplicitFalse_ReapprovesWithTrue(t *testing.T) {
	// When an explicit false entry exists, pressing 'a' writes explicit true.
	dir := setupApproveDir(t)
	const uuid = "00000003-0000-4000-8000-000000000003"
	writeApproveFeature(t, dir, "feature_001.md", uuid)
	if err := approval.Unapprove(dir, uuid); err != nil {
		t.Fatal(err)
	}

	plan := parser.Plan{
		ID:       "feature_001",
		MaggusID: uuid,
		File:     filepath.Join(dir, ".maggus", "features", "feature_001.md"),
	}
	m := statusModel{
		dir:              dir,
		plans:            []parser.Plan{plan},
		approvals:        approval.Approvals{uuid: false},
		approvalRequired: false,
		leftFocused:      true,
		featureStore:     stores.NewFileFeatureStore(dir),
		bugStore:         stores.NewFileBugStore(dir),
	}

	result, _ := m.handleApproveToggle()
	newM := result.(statusModel)

	a, err := approval.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if val, ok := a[uuid]; !ok || !val {
		t.Errorf("expected explicit true written, got: %v", a)
	}
	if newM.statusNote != "feature approved" {
		t.Errorf("expected 'feature approved' note, got: %q", newM.statusNote)
	}
}

func TestStatusSplitFooter_AltAHint(t *testing.T) {
	completedPlan := parser.Plan{ID: "feature_001", Completed: true}
	activePlan := parser.Plan{ID: "feature_002", Completed: false}

	t.Run("no completed plans — no hint", func(t *testing.T) {
		m := statusModel{
			plans:       []parser.Plan{activePlan},
			leftFocused: true,
			showAll:     false,
		}
		footer := m.statusSplitFooter()
		if strings.Contains(footer, "alt+a") {
			t.Errorf("expected no alt+a hint when no completed plans, got: %q", footer)
		}
	})

	t.Run("completed plans with showAll=false — show done hint", func(t *testing.T) {
		m := statusModel{
			plans:       []parser.Plan{activePlan, completedPlan},
			leftFocused: true,
			showAll:     false,
		}
		footer := m.statusSplitFooter()
		if !strings.Contains(footer, "alt+a: show done") {
			t.Errorf("expected 'alt+a: show done' hint, got: %q", footer)
		}
	})

	t.Run("completed plans with showAll=true — hide done hint", func(t *testing.T) {
		m := statusModel{
			plans:       []parser.Plan{activePlan, completedPlan},
			leftFocused: true,
			showAll:     true,
		}
		footer := m.statusSplitFooter()
		if !strings.Contains(footer, "alt+a: hide done") {
			t.Errorf("expected 'alt+a: hide done' hint, got: %q", footer)
		}
	})
}
