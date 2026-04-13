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
	"github.com/leberkas-org/maggus/internal/runlog"
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

func TestStatusModel_UpdateClaude2xResultMsg(t *testing.T) {
	plans := []parser.Plan{
		{ID: "plan_1", File: "plan_1.md", Tasks: []parser.Task{
			{ID: "TASK-001", Criteria: []parser.Criterion{{Checked: false}}},
		}},
	}

	t.Run("nerfed sets isNerfed and BorderColor to Error", func(t *testing.T) {
		m := newStatusModel(plans, false, "TASK-001", "plan_1.md", "claude", "/tmp", false, false, nil, nil, nil)
		msg := claude2xResultMsg{status: claude2x.Status{IsNerfed: true, TwoXWindowExpiresIn: "5h"}}
		result, _ := m.Update(msg)
		updated := result.(statusModel)
		if !updated.isNerfed {
			t.Error("isNerfed should be true")
		}
		if updated.BorderColor != styles.Error {
			t.Errorf("BorderColor = %q, want %q (Error/red)", updated.BorderColor, styles.Error)
		}
	})

	t.Run("normal keeps isNerfed false and BorderColor Primary", func(t *testing.T) {
		m := newStatusModel(plans, false, "TASK-001", "plan_1.md", "claude", "/tmp", false, false, nil, nil, nil)
		msg := claude2xResultMsg{status: claude2x.Status{IsNerfed: false}}
		result, _ := m.Update(msg)
		updated := result.(statusModel)
		if updated.isNerfed {
			t.Error("isNerfed should be false")
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

	t.Run("normal view does not contain red border styling", func(t *testing.T) {
		m := newStatusModel(plans, false, "TASK-001", "plan_1.md", "claude", "/tmp", false, false, nil, nil, nil)
		m.isNerfed = false
		m.width = 120
		m.height = 40
		m.activeTabFeature = 2 // Details tab shows task list (selFeature context: Summary, Plan, Details, Metrics)
		m.rebuildForSelectedPlan()
		view := m.View()
		if !strings.Contains(view, "Details") {
			t.Error("view should contain 'Details' tab")
		}
		if !strings.Contains(view, "TASK-001") {
			t.Error("view should contain task ID")
		}
	})

	t.Run("nerfed view renders without error", func(t *testing.T) {
		m := newStatusModel(plans, false, "TASK-001", "plan_1.md", "claude", "/tmp", false, false, nil, nil, nil)
		m.isNerfed = true
		m.width = 120
		m.height = 40
		m.activeTabFeature = 2 // Details tab shows task list (selFeature context: Summary, Plan, Details, Metrics)
		m.rebuildForSelectedPlan()
		view := m.View()
		if !strings.Contains(view, "Details") {
			t.Error("view should contain 'Details' tab")
		}
		if !strings.Contains(view, "TASK-001") {
			t.Error("view should contain task ID")
		}
	})

	t.Run("empty features view renders with nerfed", func(t *testing.T) {
		m := newStatusModel(nil, false, "", "", "claude", "/tmp", false, false, nil, nil, nil)
		m.isNerfed = true
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
		m.activeTabFeature = 1 // Feature Details tab shows task list (Plan tab in selFeature context)
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

	t.Run("returns plan at treeCursor", func(t *testing.T) {
		m := statusModel{plans: plans, treeCursor: 1, showAll: true}
		p := m.selectedPlan()
		if p.ID != "plan_2" {
			t.Errorf("selectedPlan() = %q, want plan_2", p.ID)
		}
	})

	t.Run("cursor out of range returns zero plan", func(t *testing.T) {
		m := statusModel{plans: plans, treeCursor: 99, showAll: false}
		p := m.selectedPlan()
		if p.ID != "" {
			t.Errorf("selectedPlan() with out-of-range cursor should return zero plan, got %q", p.ID)
		}
	})

	t.Run("respects showAll filter", func(t *testing.T) {
		// showAll=false: plans has 2 visible (plan_1, plan_2); treeCursor=1 selects plan_2
		m := statusModel{plans: plans, treeCursor: 1, showAll: false}
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

	t.Run("activeTabFeature and activeTabTask are 0 by default", func(t *testing.T) {
		m := newStatusModel(plans, false, "TASK-001", "plan_1.md", "claude", "/tmp", false, false, nil, nil, nil)
		if m.activeTabFeature != 0 {
			t.Errorf("activeTabFeature = %d, want 0", m.activeTabFeature)
		}
		if m.activeTabTask != 0 {
			t.Errorf("activeTabTask = %d, want 0", m.activeTabTask)
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
	t.Run("no pending task shows No task selected message", func(t *testing.T) {
		m := statusModel{nextTaskID: "", nextTaskFile: ""}
		content := m.renderCurrentTaskTab(80, 20)
		if !strings.Contains(content, "No task selected") {
			t.Errorf("expected 'No task selected', got %q", content)
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
	t.Run("Details tab is wired into renderRightPane", func(t *testing.T) {
		// Set up a selFeature context so availableTabs = [Summary, Plan, Details, Metrics].
		// Index 3 = Metrics (no "taskdetails" in this context).
		// To test taskdetails, use a selRunningTask context: [Output, Details, Metrics].
		// But we just want to verify taskdetails renders — use selCompletedTask:
		// [Summary, Output, Details, Metrics] — index 2 = taskdetails.
		completedTask := parser.Task{
			ID:       "TASK-001",
			Criteria: []parser.Criterion{{Checked: true}},
		}
		m := statusModel{
			plans: []parser.Plan{{
				ID:   "plan_1",
				File: "plan_1.md",
				Tasks: []parser.Task{completedTask},
			}},
			expandedPlans: map[string]bool{"plan_1": true},
			treeCursor:    1, // task row (plan at 0, task at 1)
			showAll:       true,
			activeTabTask: 2, // taskdetails in selCompletedTask context (index 2)
			nextTaskID:    "",
			nextTaskFile:  "",
			width:         120,
			height:        30,
		}
		content := m.renderRightPane(80, 20)
		if strings.Contains(content, "coming soon") {
			t.Error("Details tab should not show 'coming soon' placeholder")
		}
		// A task is selected (treeCursor=1 → completed task), so "No task selected" should NOT appear.
		if strings.Contains(content, "No task selected") {
			t.Errorf("Details tab should not show 'No task selected' when a task is selected, got %q", content)
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
		m.planCursor = 2
		m.treeCursor = 2 // all plans collapsed, so treeCursor == planCursor

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
		m.planCursor = 2
		m.treeCursor = 2 // all plans collapsed, so treeCursor == planCursor

		result, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
		got := result.(statusModel)
		// CursorDown wraps: from last index wraps to 0
		if got.planCursor < 0 || got.planCursor >= len(plans) {
			t.Errorf("planCursor = %d out of range [0, %d)", got.planCursor, len(plans))
		}
	})

	t.Run("up always navigates tree regardless of activeTabFeature", func(t *testing.T) {
		m := newStatusModel(plans, false, "", "", "claude", "/tmp", false, false, nil, nil, nil)
		m.width = 120
		m.height = 40
		m.activeTabFeature = 0
		m.planCursor = 1
		m.treeCursor = 1

		result, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
		got := result.(statusModel)
		// Up always navigates the tree — no focus state
		if got.planCursor != 0 {
			t.Errorf("planCursor = %d, want 0 (up always navigates tree)", got.planCursor)
		}
	})
}

func TestStatusModel_TreeExpandCollapse(t *testing.T) {
	plans := []parser.Plan{
		{ID: "plan_1", File: "plan_1.md", Tasks: []parser.Task{
			{ID: "TASK-001", Title: "First"},
			{ID: "TASK-002", Title: "Second"},
		}},
		{ID: "plan_2", File: "plan_2.md", Tasks: []parser.Task{
			{ID: "TASK-010", Title: "Ten"},
		}},
	}

	t.Run("right on plan row expands it", func(t *testing.T) {
		m := newStatusModel(plans, true, "", "", "claude", "/tmp", false, false, nil, nil, nil)
		m.width = 120
		m.height = 40
		m.treeCursor = 0

		result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
		got := result.(statusModel)
		if !got.expandedPlans["plan_1"] {
			t.Error("plan_1 should be expanded after right arrow")
		}
	})

	t.Run("l on plan row expands it", func(t *testing.T) {
		m := newStatusModel(plans, true, "", "", "claude", "/tmp", false, false, nil, nil, nil)
		m.width = 120
		m.height = 40
		m.treeCursor = 0

		result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
		got := result.(statusModel)
		if !got.expandedPlans["plan_1"] {
			t.Error("plan_1 should be expanded after l")
		}
	})

	t.Run("right on already expanded plan does nothing", func(t *testing.T) {
		m := newStatusModel(plans, true, "", "", "claude", "/tmp", false, false, nil, nil, nil)
		m.width = 120
		m.height = 40
		m.expandedPlans["plan_1"] = true
		m.treeCursor = 0

		result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
		got := result.(statusModel)
		if !got.expandedPlans["plan_1"] {
			t.Error("plan_1 should still be expanded")
		}
	})

	t.Run("left on plan row collapses it", func(t *testing.T) {
		m := newStatusModel(plans, true, "", "", "claude", "/tmp", false, false, nil, nil, nil)
		m.width = 120
		m.height = 40
		m.expandedPlans["plan_1"] = true
		m.treeCursor = 0

		result, _ := m.Update(tea.KeyMsg{Type: tea.KeyLeft})
		got := result.(statusModel)
		if got.expandedPlans["plan_1"] {
			t.Error("plan_1 should be collapsed after left arrow")
		}
	})

	t.Run("h on plan row collapses it", func(t *testing.T) {
		m := newStatusModel(plans, true, "", "", "claude", "/tmp", false, false, nil, nil, nil)
		m.width = 120
		m.height = 40
		m.expandedPlans["plan_1"] = true
		m.treeCursor = 0

		result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")})
		got := result.(statusModel)
		if got.expandedPlans["plan_1"] {
			t.Error("plan_1 should be collapsed after h")
		}
	})

	t.Run("left on task row collapses parent and moves to plan row", func(t *testing.T) {
		m := newStatusModel(plans, true, "", "", "claude", "/tmp", false, false, nil, nil, nil)
		m.width = 120
		m.height = 40
		m.expandedPlans["plan_1"] = true
		// Tree is: [0]=plan_1, [1]=TASK-001, [2]=TASK-002, [3]=plan_2
		m.treeCursor = 1 // on TASK-001

		result, _ := m.Update(tea.KeyMsg{Type: tea.KeyLeft})
		got := result.(statusModel)
		if got.expandedPlans["plan_1"] {
			t.Error("plan_1 should be collapsed")
		}
		if got.treeCursor != 0 {
			t.Errorf("treeCursor = %d, want 0 (plan_1 row)", got.treeCursor)
		}
		if got.planCursor != 0 {
			t.Errorf("planCursor = %d, want 0", got.planCursor)
		}
	})

	t.Run("down through expanded tree visits task rows", func(t *testing.T) {
		m := newStatusModel(plans, true, "", "", "claude", "/tmp", false, false, nil, nil, nil)
		m.width = 120
		m.height = 40
		m.expandedPlans["plan_1"] = true
		m.treeCursor = 0 // plan_1

		// Press down once: should move to TASK-001 (treeCursor=1)
		result, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
		got := result.(statusModel)
		if got.treeCursor != 1 {
			t.Errorf("treeCursor = %d, want 1 (TASK-001)", got.treeCursor)
		}
		// planCursor stays 0 (still plan_1's task)
		if got.planCursor != 0 {
			t.Errorf("planCursor = %d, want 0 (still plan_1)", got.planCursor)
		}
	})

	t.Run("right always expands regardless of activeTabFeature", func(t *testing.T) {
		m := newStatusModel(plans, true, "", "", "claude", "/tmp", false, false, nil, nil, nil)
		m.width = 120
		m.height = 40
		m.activeTabFeature = 0
		m.treeCursor = 0

		result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
		got := result.(statusModel)
		if !got.expandedPlans["plan_1"] {
			t.Error("right arrow should always expand (no focus state)")
		}
	})
}

func TestStatusModel_EnterOnPlanSwitchesToDetailsTab(t *testing.T) {
	plans := []parser.Plan{
		{ID: "plan_1", File: "plan_1.md", Tasks: []parser.Task{{ID: "T1"}}},
	}

	t.Run("enter on plan row switches to Details tab", func(t *testing.T) {
		m := newStatusModel(plans, false, "", "", "claude", "/tmp", false, false, nil, nil, nil)
		m.width = 120
		m.height = 40

		result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		got := result.(statusModel)
		if got.activeTabFeature != 2 {
			t.Errorf("activeTabFeature = %d, want 2 (Details)", got.activeTabFeature)
		}
	})

	t.Run("enter on plan row sets Details tab from any starting tab", func(t *testing.T) {
		m := newStatusModel(plans, false, "", "", "claude", "/tmp", false, false, nil, nil, nil)
		m.width = 120
		m.height = 40
		m.activeTabFeature = 3 // currently on metrics

		result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		got := result.(statusModel)
		if got.activeTabFeature != 2 {
			t.Errorf("activeTabFeature = %d, want 2 after enter on plan row", got.activeTabFeature)
		}
	})
}

func TestStatusModel_TabKeyIsNoOp(t *testing.T) {
	plans := []parser.Plan{
		{ID: "plan_1", File: "plan_1.md", Tasks: []parser.Task{{ID: "T1"}}},
	}

	t.Run("tab key does not change activeTabFeature", func(t *testing.T) {
		m := newStatusModel(plans, false, "", "", "claude", "/tmp", false, false, nil, nil, nil)
		m.width = 120
		m.height = 40

		result, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
		got := result.(statusModel)
		if got.activeTabFeature != m.activeTabFeature {
			t.Errorf("activeTabFeature changed from %d to %d after tab key", m.activeTabFeature, got.activeTabFeature)
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
// Uses a selFeature context (activeTabFeature=0 → Summary tab). Heights must be large enough
// that contentH (h-2) accommodates the ~10-line feature summary content.
func TestRenderRightPane_LineCount(t *testing.T) {
	m := statusModel{
		plans:            []parser.Plan{{ID: "plan_1", File: "plan_1.md"}},
		width:            120,
		height:           40,
		activeTabFeature: 0, // Summary tab
	}
	for _, h := range []int{20, 34} {
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

func TestHandleApproveToggle_OptIn_NoEntry_WritesTrue(t *testing.T) {
	// In opt-in mode, plan with no approval entry is unapproved by default.
	// Pressing 'a' must write explicit true (additive-only toggle).
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
		approvalRequired: true,                 // opt-in mode
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

func TestHandleApproveToggle_OptOutMode_IsNoop(t *testing.T) {
	// In opt-out mode, pressing 'a' should be a no-op: sets statusNote and does not write.
	dir := setupApproveDir(t)
	const uuid = "00000099-0000-4000-8000-000000000099"
	writeApproveFeature(t, dir, "feature_001.md", uuid)

	plan := parser.Plan{
		ID:       "feature_001",
		MaggusID: uuid,
		File:     filepath.Join(dir, ".maggus", "features", "feature_001.md"),
	}
	m := statusModel{
		dir:              dir,
		plans:            []parser.Plan{plan},
		approvals:        approval.Approvals{},
		approvalRequired: false, // opt-out mode
		featureStore:     stores.NewFileFeatureStore(dir),
		bugStore:         stores.NewFileBugStore(dir),
	}

	result, _ := m.handleApproveToggle()
	newM := result.(statusModel)

	// No write should have happened.
	a, err := approval.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(a) != 0 {
		t.Errorf("expected no writes in opt-out mode, got approvals: %v", a)
	}
	if newM.statusNote != "approval not required (opt-out mode)" {
		t.Errorf("expected opt-out note, got: %q", newM.statusNote)
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
		approvalRequired: true, // opt-in mode
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
		approvalRequired: true, // opt-in mode
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
			plans:   []parser.Plan{activePlan},
			showAll: false,
		}
		footer := m.statusSplitFooter()
		if strings.Contains(footer, "alt+a") {
			t.Errorf("expected no alt+a hint when no completed plans, got: %q", footer)
		}
	})

	t.Run("completed plans with showAll=false — show done hint", func(t *testing.T) {
		m := statusModel{
			plans:   []parser.Plan{activePlan, completedPlan},
			showAll: false,
		}
		footer := m.statusSplitFooter()
		if !strings.Contains(footer, "alt+a: show done") {
			t.Errorf("expected 'alt+a: show done' hint, got: %q", footer)
		}
	})

	t.Run("completed plans with showAll=true — hide done hint", func(t *testing.T) {
		m := statusModel{
			plans:   []parser.Plan{activePlan, completedPlan},
			showAll: true,
		}
		footer := m.statusSplitFooter()
		if !strings.Contains(footer, "alt+a: hide done") {
			t.Errorf("expected 'alt+a: hide done' hint, got: %q", footer)
		}
	})
}

func TestShouldPromptOnExit(t *testing.T) {
	t.Run("returns true when daemon is running", func(t *testing.T) {
		m := statusModel{daemon: daemonStatus{Running: true}}
		if !m.shouldPromptOnExit() {
			t.Error("shouldPromptOnExit() = false, want true when daemon is running")
		}
	})

	t.Run("returns false when daemon is not running", func(t *testing.T) {
		m := statusModel{daemon: daemonStatus{Running: false}}
		if m.shouldPromptOnExit() {
			t.Error("shouldPromptOnExit() = true, want false when daemon is not running")
		}
	})

	t.Run("returns true when daemon is running with active task", func(t *testing.T) {
		m := statusModel{daemon: daemonStatus{Running: true, CurrentTask: "TASK-001"}}
		if !m.shouldPromptOnExit() {
			t.Error("shouldPromptOnExit() = false, want true when daemon is running with active task")
		}
	})
}

// --- Selection context and dynamic tab mapping tests ---

func TestSelectionCtx(t *testing.T) {
	t.Run("empty tree returns selNone", func(t *testing.T) {
		m := statusModel{plans: nil, treeCursor: 0}
		if got := m.selectionCtx(); got != selNone {
			t.Errorf("selectionCtx() = %d, want selNone (%d)", got, selNone)
		}
	})

	t.Run("cursor on plan row returns selFeature", func(t *testing.T) {
		m := statusModel{
			plans:      []parser.Plan{{ID: "plan_1", File: "plan_1.md", Tasks: []parser.Task{{ID: "T1"}}}},
			treeCursor: 0,
		}
		if got := m.selectionCtx(); got != selFeature {
			t.Errorf("selectionCtx() = %d, want selFeature (%d)", got, selFeature)
		}
	})

	t.Run("cursor on completed task returns selCompletedTask", func(t *testing.T) {
		completedTask := parser.Task{
			ID:       "TASK-001",
			Criteria: []parser.Criterion{{Checked: true}},
		}
		m := statusModel{
			plans: []parser.Plan{{
				ID:    "plan_1",
				File:  "plan_1.md",
				Tasks: []parser.Task{completedTask},
			}},
			expandedPlans: map[string]bool{"plan_1": true},
			treeCursor:    1, // task row
			showAll:       true,
		}
		if got := m.selectionCtx(); got != selCompletedTask {
			t.Errorf("selectionCtx() = %d, want selCompletedTask (%d)", got, selCompletedTask)
		}
	})

	t.Run("cursor on running task returns selRunningTask", func(t *testing.T) {
		pendingTask := parser.Task{
			ID:       "TASK-001",
			Criteria: []parser.Criterion{{Checked: false}},
		}
		m := statusModel{
			plans: []parser.Plan{{
				ID:    "plan_1",
				File:  "plan_1.md",
				Tasks: []parser.Task{pendingTask},
			}},
			expandedPlans: map[string]bool{"plan_1": true},
			treeCursor:    1,
			daemon:        daemonStatus{Running: true, CurrentTask: "TASK-001"},
		}
		if got := m.selectionCtx(); got != selRunningTask {
			t.Errorf("selectionCtx() = %d, want selRunningTask (%d)", got, selRunningTask)
		}
	})

	t.Run("cursor on pending non-running task returns selCompletedTask", func(t *testing.T) {
		pendingTask := parser.Task{
			ID:       "TASK-001",
			Criteria: []parser.Criterion{{Checked: false}},
		}
		m := statusModel{
			plans: []parser.Plan{{
				ID:    "plan_1",
				File:  "plan_1.md",
				Tasks: []parser.Task{pendingTask},
			}},
			expandedPlans: map[string]bool{"plan_1": true},
			treeCursor:    1,
			daemon:        daemonStatus{Running: false},
		}
		if got := m.selectionCtx(); got != selCompletedTask {
			t.Errorf("selectionCtx() = %d, want selCompletedTask (%d)", got, selCompletedTask)
		}
	})

	t.Run("cursor out of range returns selNone", func(t *testing.T) {
		m := statusModel{
			plans:      []parser.Plan{{ID: "plan_1", File: "plan_1.md"}},
			treeCursor: 99,
		}
		if got := m.selectionCtx(); got != selNone {
			t.Errorf("selectionCtx() = %d, want selNone (%d)", got, selNone)
		}
	})
}

func TestIsTaskRunning(t *testing.T) {
	t.Run("daemon not running returns false", func(t *testing.T) {
		m := statusModel{daemon: daemonStatus{Running: false}}
		if m.isTaskRunning("TASK-001") {
			t.Error("isTaskRunning should be false when daemon is not running")
		}
	})

	t.Run("sequential mode matches CurrentTask", func(t *testing.T) {
		m := statusModel{daemon: daemonStatus{Running: true, CurrentTask: "TASK-001"}}
		if !m.isTaskRunning("TASK-001") {
			t.Error("isTaskRunning should be true when CurrentTask matches")
		}
		if m.isTaskRunning("TASK-002") {
			t.Error("isTaskRunning should be false for non-matching task")
		}
	})

	t.Run("parallel mode matches working worker", func(t *testing.T) {
		m := statusModel{
			daemon: daemonStatus{Running: true},
			workerIndex: []runlog.WorkerIndexEntry{
				{TaskID: "TASK-001", Status: "working"},
				{TaskID: "TASK-002", Status: "done"},
			},
		}
		if !m.isTaskRunning("TASK-001") {
			t.Error("isTaskRunning should be true for working worker")
		}
		if m.isTaskRunning("TASK-002") {
			t.Error("isTaskRunning should be false for done worker")
		}
	})

	t.Run("dispatched worker detected when daemon not running", func(t *testing.T) {
		m := statusModel{
			daemon: daemonStatus{Running: false},
			workerIndex: []runlog.WorkerIndexEntry{
				{TaskID: "TASK-001", Status: "working"},
				{TaskID: "TASK-002", Status: "done"},
			},
		}
		if !m.isTaskRunning("TASK-001") {
			t.Error("isTaskRunning should be true for dispatched working worker even when daemon is not running")
		}
		if m.isTaskRunning("TASK-002") {
			t.Error("isTaskRunning should be false for completed dispatched worker")
		}
	})

	t.Run("snapshot with matching task and non-terminal status returns true", func(t *testing.T) {
		m := statusModel{
			daemon: daemonStatus{Running: true, CurrentTask: "TASK-999"}, // JSONL says different task
			snapshot: &runlog.StateSnapshot{
				TaskID: "TASK-001",
				Status: "Running",
			},
		}
		if !m.isTaskRunning("TASK-001") {
			t.Error("isTaskRunning should be true when snapshot task matches and status is non-terminal")
		}
		// JSONL says TASK-999 is running, but snapshot overrides
		if m.isTaskRunning("TASK-999") {
			t.Error("isTaskRunning should be false for JSONL task when snapshot points elsewhere")
		}
	})

	t.Run("snapshot with terminal status returns false", func(t *testing.T) {
		for _, status := range []string{"Done", "Failed", "Interrupted"} {
			m := statusModel{
				daemon: daemonStatus{Running: true, CurrentTask: "TASK-001"},
				snapshot: &runlog.StateSnapshot{
					TaskID: "TASK-001",
					Status: status,
				},
			}
			if m.isTaskRunning("TASK-001") {
				t.Errorf("isTaskRunning should be false when snapshot status is %q (terminal)", status)
			}
		}
	})

	t.Run("snapshot with empty task_id returns false (no false positives)", func(t *testing.T) {
		m := statusModel{
			daemon: daemonStatus{Running: true, CurrentTask: "TASK-001"},
			snapshot: &runlog.StateSnapshot{
				TaskID: "",
				Status: "Running",
			},
		}
		if m.isTaskRunning("TASK-001") {
			t.Error("isTaskRunning should be false when snapshot has empty task_id")
		}
	})

	t.Run("no snapshot falls back to JSONL daemon state", func(t *testing.T) {
		m := statusModel{
			daemon:   daemonStatus{Running: true, CurrentTask: "TASK-001"},
			snapshot: nil,
		}
		if !m.isTaskRunning("TASK-001") {
			t.Error("isTaskRunning should fall back to daemon.CurrentTask when no snapshot")
		}
		if m.isTaskRunning("TASK-002") {
			t.Error("isTaskRunning should be false for non-matching JSONL task when no snapshot")
		}
	})

	t.Run("worker index takes precedence over snapshot for parallel mode", func(t *testing.T) {
		// When a workerIndex entry says "working", it should return true even if
		// the main snapshot points to a different task (parallel mode).
		m := statusModel{
			daemon: daemonStatus{Running: true},
			snapshot: &runlog.StateSnapshot{
				TaskID: "TASK-002",
				Status: "Running",
			},
			workerIndex: []runlog.WorkerIndexEntry{
				{TaskID: "TASK-001", Status: "working"},
			},
		}
		if !m.isTaskRunning("TASK-001") {
			t.Error("isTaskRunning should be true for working worker even when snapshot points elsewhere")
		}
	})
}

func TestAvailableTabs(t *testing.T) {
	t.Run("selNone returns Metrics only", func(t *testing.T) {
		m := statusModel{plans: nil, treeCursor: 0}
		tabs := m.availableTabs()
		if len(tabs) != 1 {
			t.Fatalf("len(availableTabs) = %d, want 1", len(tabs))
		}
		if tabs[0].key != "metrics" {
			t.Errorf("tabs[0].key = %q, want metrics", tabs[0].key)
		}
		if tabs[0].name != "Metrics" {
			t.Errorf("tabs[0].name = %q, want Metrics", tabs[0].name)
		}
	})

	t.Run("selFeature returns Summary, Plan, Details, Metrics", func(t *testing.T) {
		m := statusModel{
			plans:      []parser.Plan{{ID: "plan_1", File: "plan_1.md"}},
			treeCursor: 0,
		}
		tabs := m.availableTabs()
		if len(tabs) != 4 {
			t.Fatalf("len(availableTabs) = %d, want 4", len(tabs))
		}
		expected := []struct{ key, name string }{
			{"summary", "Summary"},
			{"plan", "Plan"},
			{"details", "Details"},
			{"metrics", "Metrics"},
		}
		for i, e := range expected {
			if tabs[i].key != e.key {
				t.Errorf("tabs[%d].key = %q, want %q", i, tabs[i].key, e.key)
			}
			if tabs[i].name != e.name {
				t.Errorf("tabs[%d].name = %q, want %q", i, tabs[i].name, e.name)
			}
		}
	})

	t.Run("selRunningTask returns Output, Details, Metrics", func(t *testing.T) {
		pendingTask := parser.Task{
			ID:       "TASK-001",
			Criteria: []parser.Criterion{{Checked: false}},
		}
		m := statusModel{
			plans: []parser.Plan{{
				ID:    "plan_1",
				File:  "plan_1.md",
				Tasks: []parser.Task{pendingTask},
			}},
			expandedPlans: map[string]bool{"plan_1": true},
			treeCursor:    1,
			daemon:        daemonStatus{Running: true, CurrentTask: "TASK-001"},
		}
		tabs := m.availableTabs()
		if len(tabs) != 3 {
			t.Fatalf("len(availableTabs) = %d, want 3", len(tabs))
		}
		expected := []struct{ key, name string }{
			{"output", "Output"},
			{"taskdetails", "Details"},
			{"metrics", "Metrics"},
		}
		for i, e := range expected {
			if tabs[i].key != e.key {
				t.Errorf("tabs[%d].key = %q, want %q", i, tabs[i].key, e.key)
			}
			if tabs[i].name != e.name {
				t.Errorf("tabs[%d].name = %q, want %q", i, tabs[i].name, e.name)
			}
		}
	})

	t.Run("selCompletedTask returns Summary, Output, Details, Metrics", func(t *testing.T) {
		completedTask := parser.Task{
			ID:       "TASK-001",
			Criteria: []parser.Criterion{{Checked: true}},
		}
		m := statusModel{
			plans: []parser.Plan{{
				ID:    "plan_1",
				File:  "plan_1.md",
				Tasks: []parser.Task{completedTask},
			}},
			expandedPlans: map[string]bool{"plan_1": true},
			treeCursor:    1,
			showAll:       true,
		}
		tabs := m.availableTabs()
		if len(tabs) != 4 {
			t.Fatalf("len(availableTabs) = %d, want 4", len(tabs))
		}
		expected := []struct{ key, name string }{
			{"summary", "Summary"},
			{"output", "Output"},
			{"taskdetails", "Details"},
			{"metrics", "Metrics"},
		}
		for i, e := range expected {
			if tabs[i].key != e.key {
				t.Errorf("tabs[%d].key = %q, want %q", i, tabs[i].key, e.key)
			}
			if tabs[i].name != e.name {
				t.Errorf("tabs[%d].name = %q, want %q", i, tabs[i].name, e.name)
			}
		}
	})
}

func TestClampActiveTab(t *testing.T) {
	t.Run("clamps above max for selFeature", func(t *testing.T) {
		m := statusModel{
			plans:            []parser.Plan{{ID: "plan_1", File: "plan_1.md"}},
			treeCursor:       0,
			activeTabFeature: 10,
		}
		m.clampActiveTab()
		tabs := m.availableTabs() // selFeature: 4 tabs
		if m.activeTabFeature != len(tabs)-1 {
			t.Errorf("activeTabFeature = %d, want %d", m.activeTabFeature, len(tabs)-1)
		}
	})

	t.Run("keeps valid index for selFeature", func(t *testing.T) {
		m := statusModel{
			plans:            []parser.Plan{{ID: "plan_1", File: "plan_1.md"}},
			treeCursor:       0,
			activeTabFeature: 1,
		}
		m.clampActiveTab()
		if m.activeTabFeature != 1 {
			t.Errorf("activeTabFeature = %d, want 1", m.activeTabFeature)
		}
	})

	t.Run("clamps negative to 0 for selFeature", func(t *testing.T) {
		m := statusModel{
			plans:            []parser.Plan{{ID: "plan_1", File: "plan_1.md"}},
			treeCursor:       0,
			activeTabFeature: -1,
		}
		m.clampActiveTab()
		if m.activeTabFeature != 0 {
			t.Errorf("activeTabFeature = %d, want 0", m.activeTabFeature)
		}
	})
}

func TestUpdateTabsForSelectionChange(t *testing.T) {
	t.Run("does not reset task tracker when transitioning feature→task", func(t *testing.T) {
		completedTask := parser.Task{
			ID:       "TASK-001",
			Criteria: []parser.Criterion{{Checked: true}},
		}
		m := statusModel{
			plans: []parser.Plan{{
				ID:    "plan_1",
				File:  "plan_1.md",
				Tasks: []parser.Task{completedTask},
			}},
			expandedPlans: map[string]bool{"plan_1": true},
			treeCursor:    1, // task row → selCompletedTask
			showAll:       true,
			activeTabTask: 2, // task tracker at Details (index 2; valid in 4-tab selCompletedTask set)
		}
		// Simulate moving from selFeature (prevCtx) to selCompletedTask (current)
		m.updateTabsForSelectionChange(selFeature)
		// Task tracker should be preserved (clamped only if out of bounds, which 2 is not in 4-tab set)
		if m.activeTabTask != 2 {
			t.Errorf("activeTabTask = %d, want 2 (independent trackers — task tab preserved)", m.activeTabTask)
		}
	})

	t.Run("preserves activeTabFeature when context stays selFeature", func(t *testing.T) {
		m := statusModel{
			plans:            []parser.Plan{{ID: "plan_1", File: "plan_1.md"}, {ID: "plan_2", File: "plan_2.md"}},
			treeCursor:       0, // plan row → selFeature
			activeTabFeature: 2,
		}
		// Same context (selFeature → selFeature)
		m.updateTabsForSelectionChange(selFeature)
		if m.activeTabFeature != 2 {
			t.Errorf("activeTabFeature = %d, want 2 (should preserve when context unchanged)", m.activeTabFeature)
		}
	})

	t.Run("clamps when same context but out of range", func(t *testing.T) {
		// selNone has 1 tab; any tracker value > 0 should clamp to 0.
		// Since selNone returns 0 from activeTabIndex() (no-op on set), use selFeature with out-of-range value.
		m := statusModel{
			plans:            []parser.Plan{{ID: "plan_1", File: "plan_1.md"}},
			treeCursor:       0, // selFeature: 4 tabs
			activeTabFeature: 10,
		}
		m.updateTabsForSelectionChange(selFeature)
		tabs := m.availableTabs() // 4 tabs for selFeature
		if m.activeTabFeature != len(tabs)-1 {
			t.Errorf("activeTabFeature = %d, want %d (should clamp to max index)", m.activeTabFeature, len(tabs)-1)
		}
	})
}

// --- Skip toggle tests ---

func TestHandleSkipToggle_SkipsFirstUncheckedCriterion(t *testing.T) {
	dir := setupApproveDir(t)

	plan := parser.Plan{
		ID:   "feature_001",
		File: filepath.Join(dir, ".maggus", "features", "feature_001.md"),
		Tasks: []parser.Task{
			{
				ID:         "TASK-001-001",
				Title:      "First task",
				SourceFile: filepath.Join(dir, ".maggus", "features", "feature_001.md"),
				Criteria: []parser.Criterion{
					{Text: "Do the thing", Checked: false, Blocked: false, Skipped: false},
				},
			},
		},
	}

	fs := stores.NewMemFeatureStore([]parser.Plan{plan})
	bs := stores.NewMemBugStore(nil)

	m := statusModel{
		taskListComponent: taskListComponent{
			featureStore: fs,
			bugStore:     bs,
		},
		dir:           dir,
		plans:         []parser.Plan{plan},
		expandedPlans: map[string]bool{"feature_001": true},
		treeCursor:    1, // task row (plan row = 0, first task row = 1)
		featureStore:  fs,
		bugStore:      bs,
	}

	result, _ := m.handleSkipToggle()
	newM := result.(statusModel)

	if newM.statusNote != "task skipped" {
		t.Errorf("expected 'task skipped' note, got: %q", newM.statusNote)
	}
}

func TestHandleSkipToggle_UnskipsSkippedCriterion(t *testing.T) {
	dir := setupApproveDir(t)

	plan := parser.Plan{
		ID:   "feature_001",
		File: filepath.Join(dir, ".maggus", "features", "feature_001.md"),
		Tasks: []parser.Task{
			{
				ID:         "TASK-001-001",
				Title:      "First task",
				SourceFile: filepath.Join(dir, ".maggus", "features", "feature_001.md"),
				Criteria: []parser.Criterion{
					{Text: "SKIPPED: Do the thing", Checked: false, Blocked: false, Skipped: true},
				},
			},
		},
	}

	fs := stores.NewMemFeatureStore([]parser.Plan{plan})
	bs := stores.NewMemBugStore(nil)

	m := statusModel{
		taskListComponent: taskListComponent{
			featureStore: fs,
			bugStore:     bs,
		},
		dir:           dir,
		plans:         []parser.Plan{plan},
		expandedPlans: map[string]bool{"feature_001": true},
		treeCursor:    1, // task row
		featureStore:  fs,
		bugStore:      bs,
	}

	result, _ := m.handleSkipToggle()
	newM := result.(statusModel)

	if newM.statusNote != "task unskipped" {
		t.Errorf("expected 'task unskipped' note, got: %q", newM.statusNote)
	}
}

func TestHandleSkipToggle_PlanRowIsNoop(t *testing.T) {
	plan := parser.Plan{
		ID:   "feature_001",
		File: "feature_001.md",
		Tasks: []parser.Task{
			{
				ID:       "TASK-001-001",
				Criteria: []parser.Criterion{{Text: "Do the thing", Checked: false}},
			},
		},
	}

	m := statusModel{
		plans:         []parser.Plan{plan},
		expandedPlans: map[string]bool{}, // not expanded; cursor 0 = plan row
		treeCursor:    0,
	}

	result, _ := m.handleSkipToggle()
	newM := result.(statusModel)

	if newM.statusNote != "" {
		t.Errorf("expected empty note for plan row, got: %q", newM.statusNote)
	}
}

func TestHandleSkipToggle_CompleteTaskIsNoop(t *testing.T) {
	plan := parser.Plan{
		ID:   "feature_001",
		File: "feature_001.md",
		Tasks: []parser.Task{
			{
				ID:         "TASK-001-001",
				SourceFile: "feature_001.md",
				Criteria:   []parser.Criterion{{Text: "Done", Checked: true}},
			},
		},
	}

	m := statusModel{
		plans:         []parser.Plan{plan},
		expandedPlans: map[string]bool{"feature_001": true},
		treeCursor:    1, // task row
	}

	result, _ := m.handleSkipToggle()
	newM := result.(statusModel)

	if newM.statusNote != "" {
		t.Errorf("expected empty note for complete task, got: %q", newM.statusNote)
	}
}

func TestUpdateList_XKey_SkipsTask(t *testing.T) {
	dir := setupApproveDir(t)

	plan := parser.Plan{
		ID:   "feature_001",
		File: filepath.Join(dir, ".maggus", "features", "feature_001.md"),
		Tasks: []parser.Task{
			{
				ID:         "TASK-001-001",
				Title:      "First task",
				SourceFile: filepath.Join(dir, ".maggus", "features", "feature_001.md"),
				Criteria: []parser.Criterion{
					{Text: "Do the thing", Checked: false, Blocked: false, Skipped: false},
				},
			},
		},
	}

	fs := stores.NewMemFeatureStore([]parser.Plan{plan})
	bs := stores.NewMemBugStore(nil)

	m := statusModel{
		taskListComponent: taskListComponent{
			featureStore: fs,
			bugStore:     bs,
		},
		dir:           dir,
		plans:         []parser.Plan{plan},
		expandedPlans: map[string]bool{"feature_001": true},
		treeCursor:    1, // task row
		featureStore:  fs,
		bugStore:      bs,
	}

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	newM := result.(statusModel)

	if newM.statusNote != "task skipped" {
		t.Errorf("x key: expected 'task skipped' note, got: %q", newM.statusNote)
	}
}

func TestStatusSplitFooter_ShowsStatusNote(t *testing.T) {
	m := statusModel{
		plans:      []parser.Plan{{ID: "feature_001", File: "feature_001.md"}},
		statusNote: "task skipped",
	}
	footer := m.statusSplitFooter()
	if !strings.Contains(footer, "task skipped") {
		t.Errorf("expected footer to contain 'task skipped', got: %q", footer)
	}
}

func TestStatusSplitFooter_ContainsSkipHint(t *testing.T) {
	// Skip hint should appear when a task row is selected.
	task := parser.Task{ID: "TASK-001-001", Title: "Test task",
		Criteria: []parser.Criterion{{Text: "do something", Checked: false}}}
	plan := parser.Plan{ID: "feature_001", File: "feature_001.md", Tasks: []parser.Task{task}}
	m := statusModel{
		plans:         []parser.Plan{plan},
		expandedPlans: map[string]bool{"feature_001": true},
		treeCursor:    1, // task row (0 = plan row, 1 = first task)
	}
	footer := m.statusSplitFooter()
	if !strings.Contains(footer, "x: skip") {
		t.Errorf("expected footer to contain 'x: skip/unskip' hint when task selected, got: %q", footer)
	}

	// Skip hint should NOT appear when a plan row is selected.
	mPlan := statusModel{
		plans:      []parser.Plan{plan},
		treeCursor: 0, // plan row
	}
	footerPlan := mPlan.statusSplitFooter()
	if strings.Contains(footerPlan, "x: skip") {
		t.Errorf("expected footer to NOT contain 'x: skip/unskip' hint when plan row selected, got: %q", footerPlan)
	}
}

// ── handleAltRunDispatch tests ────────────────────────────────────────────────

func TestHandleAltRunDispatch_NilTaskIsNoop(t *testing.T) {
	m := statusModel{}
	result, cmd := m.handleAltRunDispatch(nil)
	if result.statusNote != "" {
		t.Errorf("expected empty status note, got: %q", result.statusNote)
	}
	if cmd != nil {
		t.Error("expected nil cmd for nil task")
	}
}

func TestHandleAltRunDispatch_CompleteTaskIsNoop(t *testing.T) {
	task := &parser.Task{
		ID:       "TASK-001",
		Criteria: []parser.Criterion{{Text: "done", Checked: true}},
	}
	m := statusModel{}
	result, cmd := m.handleAltRunDispatch(task)
	if result.statusNote != "" {
		t.Errorf("expected empty status note for complete task, got: %q", result.statusNote)
	}
	if cmd != nil {
		t.Error("expected nil cmd for complete task")
	}
}

func TestHandleAltRunDispatch_BlockedTaskIsNoop(t *testing.T) {
	task := &parser.Task{
		ID:       "TASK-001",
		Criteria: []parser.Criterion{{Text: "BLOCKED: waiting", Checked: false, Blocked: true}},
	}
	m := statusModel{}
	result, cmd := m.handleAltRunDispatch(task)
	if result.statusNote != "" {
		t.Errorf("expected empty status note for blocked task, got: %q", result.statusNote)
	}
	if cmd != nil {
		t.Error("expected nil cmd for blocked task")
	}
}

func TestHandleAltRunDispatch_AlreadyRunningShowsNote(t *testing.T) {
	task := &parser.Task{
		ID:       "TASK-001",
		Criteria: []parser.Criterion{{Text: "do something", Checked: false}},
	}
	m := statusModel{
		daemon: daemonStatus{Running: true, CurrentTask: "TASK-001"},
	}
	result, cmd := m.handleAltRunDispatch(task)
	if result.statusNote != "Task already running" {
		t.Errorf("expected 'Task already running', got: %q", result.statusNote)
	}
	if cmd != nil {
		t.Error("expected nil cmd when task already running")
	}
}

func TestHandleAltRunDispatch_DaemonNotRunning_ReturnsForegroundCmd(t *testing.T) {
	task := &parser.Task{
		ID:       "TASK-001",
		Criteria: []parser.Criterion{{Text: "do something", Checked: false}},
	}
	m := statusModel{
		daemon: daemonStatus{Running: false},
	}
	result, cmd := m.handleAltRunDispatch(task)
	if result.taskListComponent.RunTaskID != "TASK-001" {
		t.Errorf("expected RunTaskID=TASK-001, got: %q", result.taskListComponent.RunTaskID)
	}
	if cmd == nil {
		t.Error("expected non-nil cmd for foreground run when daemon not running")
	}
}

// ── Footer alt+r hint tests ───────────────────────────────────────────────────

func TestStatusSplitFooter_AltRHint_DaemonRunning(t *testing.T) {
	task := parser.Task{ID: "TASK-001", Title: "Test",
		Criteria: []parser.Criterion{{Text: "do something", Checked: false}}}
	plan := parser.Plan{ID: "feature_001", File: "feature_001.md", Tasks: []parser.Task{task}}
	m := statusModel{
		plans:         []parser.Plan{plan},
		expandedPlans: map[string]bool{"feature_001": true},
		treeCursor:    1, // task row
		daemon:        daemonStatus{Running: true},
	}
	footer := m.statusSplitFooter()
	if !strings.Contains(footer, "alt+r: run") {
		t.Errorf("expected 'alt+r: run' when daemon running, got: %q", footer)
	}
}

func TestStatusSplitFooter_AltRHint_DaemonNotRunning(t *testing.T) {
	task := parser.Task{ID: "TASK-001", Title: "Test",
		Criteria: []parser.Criterion{{Text: "do something", Checked: false}}}
	plan := parser.Plan{ID: "feature_001", File: "feature_001.md", Tasks: []parser.Task{task}}
	m := statusModel{
		plans:         []parser.Plan{plan},
		expandedPlans: map[string]bool{"feature_001": true},
		treeCursor:    1, // task row
		daemon:        daemonStatus{Running: false},
	}
	footer := m.statusSplitFooter()
	if !strings.Contains(footer, "alt+r: run") {
		t.Errorf("expected 'alt+r: run' when daemon not running, got: %q", footer)
	}
}

func TestStatusSplitFooter_AltRHint_NoPlanRow(t *testing.T) {
	// No alt+r hint when a plan row (not task) is selected.
	plan := parser.Plan{ID: "feature_001", File: "feature_001.md",
		Tasks: []parser.Task{{ID: "TASK-001", Criteria: []parser.Criterion{{Text: "do", Checked: false}}}}}
	m := statusModel{
		plans:      []parser.Plan{plan},
		treeCursor: 0, // plan row
	}
	footer := m.statusSplitFooter()
	if strings.Contains(footer, "alt+r") {
		t.Errorf("expected no alt+r hint when plan row selected, got: %q", footer)
	}
}

func TestStatusSplitFooter_DetailView_AltRHint_DaemonRunning(t *testing.T) {
	task := parser.Task{ID: "TASK-001", Title: "Test",
		Criteria: []parser.Criterion{{Text: "do something", Checked: false}}}
	plan := parser.Plan{ID: "feature_001", File: "feature_001.md", Tasks: []parser.Task{task}}
	m := statusModel{
		plans:         []parser.Plan{plan},
		expandedPlans: map[string]bool{"feature_001": true},
		treeCursor:    1,
		daemon:        daemonStatus{Running: true},
	}
	m.taskListComponent.Tasks = []parser.Task{task}
	m.taskListComponent.ShowDetail = true
	footer := m.statusSplitFooter()
	if !strings.Contains(footer, "alt+r: run") {
		t.Errorf("expected 'alt+r: run' in detail footer when daemon running, got: %q", footer)
	}
}

func TestStatusSplitFooter_DetailView_AltRHint_DaemonNotRunning(t *testing.T) {
	task := parser.Task{ID: "TASK-001", Title: "Test",
		Criteria: []parser.Criterion{{Text: "do something", Checked: false}}}
	plan := parser.Plan{ID: "feature_001", File: "feature_001.md", Tasks: []parser.Task{task}}
	m := statusModel{
		plans:         []parser.Plan{plan},
		expandedPlans: map[string]bool{"feature_001": true},
		treeCursor:    1,
		daemon:        daemonStatus{Running: false},
	}
	m.taskListComponent.Tasks = []parser.Task{task}
	m.taskListComponent.ShowDetail = true
	footer := m.statusSplitFooter()
	if !strings.Contains(footer, "alt+r: run") {
		t.Errorf("expected 'alt+r: run' in detail footer when daemon not running, got: %q", footer)
	}
}

func TestStatusSplitFooter_HidesApproveHintWhenOptOut(t *testing.T) {
	// In opt-out mode, "a: approve" hint must not appear in the footer.
	plan := parser.Plan{ID: "feature_001", Completed: false}
	m := statusModel{
		plans:            []parser.Plan{plan},
		approvalRequired: false, // opt-out mode
	}
	footer := m.statusSplitFooter()
	if strings.Contains(footer, "a: approve") {
		t.Errorf("expected 'a: approve' hint to be hidden in opt-out mode, got: %q", footer)
	}
}

func TestStatusSplitFooter_ShowsApproveHintWhenOptIn(t *testing.T) {
	// In opt-in mode, "a: approve" hint must appear in the footer.
	plan := parser.Plan{ID: "feature_001", Completed: false}
	m := statusModel{
		plans:            []parser.Plan{plan},
		approvalRequired: true, // opt-in mode
	}
	footer := m.statusSplitFooter()
	if !strings.Contains(footer, "a: approve") {
		t.Errorf("expected 'a: approve' hint in opt-in mode, got: %q", footer)
	}
}

func TestRenderLeftPane_BadgeOptOut_NeverShowsUnapprovedBadge(t *testing.T) {
	// In opt-out mode, even an explicitly-unapproved plan shows ✓ (not ○) for its approval badge.
	// The plan row must be visible — use enough height to show header + daemon + separator + plan rows.
	plan := parser.Plan{
		ID:    "feature_001",
		File:  "feature_001.md",
		Tasks: []parser.Task{{ID: "T1"}},
	}
	m := statusModel{
		plans:            []parser.Plan{plan},
		approvals:        approval.Approvals{"feature_001": false}, // explicitly unapproved
		approvalRequired: false,                                    // opt-out mode
	}
	out := m.renderLeftPane(40, 10)
	lines := strings.Split(out, "\n")

	// Find the line that renders the feature plan (contains "feature_001").
	var planLine string
	for _, l := range lines {
		if strings.Contains(l, "feature_001") {
			planLine = l
			break
		}
	}
	if planLine == "" {
		t.Fatalf("plan line for feature_001 not found in left pane output:\n%q", out)
	}
	if !strings.Contains(planLine, "✓") {
		t.Errorf("expected ✓ badge on plan line in opt-out mode, got: %q", planLine)
	}
	if strings.Contains(planLine, "○") {
		t.Errorf("expected no ○ badge on plan line in opt-out mode, got: %q", planLine)
	}
}

// BUG-039-004: refreshWorkerSnapshots must skip index entries without snapshot files.

// TestRefreshWorkerSnapshots_SkipsEntriesWithMissingSnapshot verifies that index
// entries whose per-worker snapshot file does not exist are not surfaced in the TUI.
func TestRefreshWorkerSnapshots_SkipsEntriesWithMissingSnapshot(t *testing.T) {
	dir := t.TempDir()

	// Write an index with two workers — only one will have a snapshot file.
	_ = runlog.WriteWorkersIndex(dir, []runlog.WorkerIndexEntry{
		{TaskID: "BUG-039-004-001", Status: "working"},
		{TaskID: "BUG-039-004-002", Status: "done"}, // no snapshot file
	})

	// Write a snapshot only for the first worker.
	_ = runlog.WriteWorkerSnapshot(dir, "BUG-039-004-001", runlog.StateSnapshot{
		TaskID: "BUG-039-004-001",
		Status: "Working",
	})

	m := statusModel{dir: dir}
	m.refreshWorkerSnapshots()

	// Only the worker with a snapshot should appear.
	if len(m.workerIndex) != 1 {
		t.Fatalf("workerIndex len = %d, want 1 (ghost entry should be skipped)", len(m.workerIndex))
	}
	if m.workerIndex[0].TaskID != "BUG-039-004-001" {
		t.Errorf("workerIndex[0].TaskID = %q, want %q", m.workerIndex[0].TaskID, "BUG-039-004-001")
	}
	// The missing-snapshot entry must not appear in the snapshot map.
	if _, ok := m.workerSnapshots["BUG-039-004-002"]; ok {
		t.Error("ghost worker snapshot must not be in workerSnapshots map")
	}
}

// TestRefreshWorkerSnapshots_NilIndexWhenAllMissingSnapshots verifies that when
// every index entry lacks a snapshot file, workerIndex is set to nil (not parallel mode).
func TestRefreshWorkerSnapshots_NilIndexWhenAllMissingSnapshots(t *testing.T) {
	dir := t.TempDir()

	// Write an index with two workers — neither has a snapshot file.
	_ = runlog.WriteWorkersIndex(dir, []runlog.WorkerIndexEntry{
		{TaskID: "BUG-039-004-003", Status: "done"},
		{TaskID: "BUG-039-004-004", Status: "failed"},
	})

	m := statusModel{dir: dir}
	m.refreshWorkerSnapshots()

	if m.workerIndex != nil {
		t.Errorf("workerIndex should be nil when no snapshot files exist, got len=%d", len(m.workerIndex))
	}
}

// TestRefreshWorkerSnapshots_NilWhenEmptyIndex verifies the empty-index path.
func TestRefreshWorkerSnapshots_NilWhenEmptyIndex(t *testing.T) {
	dir := t.TempDir()
	m := statusModel{dir: dir}
	m.refreshWorkerSnapshots()

	if m.workerIndex != nil {
		t.Error("workerIndex should be nil when no index file exists")
	}
}

// ── handleAltRunDispatch file-based dispatch tests ────────────────────────────

func TestHandleAltRunDispatch_DaemonRunning_WritesDispatchSentinel(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".maggus"), 0o755); err != nil {
		t.Fatal(err)
	}
	task := &parser.Task{
		ID:       "TASK-007",
		Criteria: []parser.Criterion{{Text: "do something", Checked: false}},
	}
	m := statusModel{
		dir:    dir,
		daemon: daemonStatus{Running: true},
	}
	result, cmd := m.handleAltRunDispatch(task)

	if result.statusNote != "Dispatched TASK-007" {
		t.Errorf("expected statusNote 'Dispatched TASK-007', got: %q", result.statusNote)
	}
	if cmd == nil {
		t.Error("expected non-nil cmd for daemon dispatch")
	}
	// Execute the cmd to trigger the sentinel write.
	msg := cmd()
	if msg != nil {
		t.Errorf("expected nil msg from dispatch cmd, got: %v", msg)
	}

	// Verify the sentinel file was written.
	sentinelPath := dispatchSentinelPath(dir, "TASK-007")
	if _, err := os.Stat(sentinelPath); err != nil {
		t.Errorf("expected dispatch sentinel file to exist: %v", err)
	}
}

func TestHandleAltRunDispatch_DaemonRunning_AlreadyRunning_NoSentinel(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".maggus"), 0o755); err != nil {
		t.Fatal(err)
	}
	task := &parser.Task{
		ID:       "TASK-007",
		Criteria: []parser.Criterion{{Text: "do something", Checked: false}},
	}
	m := statusModel{
		dir:    dir,
		daemon: daemonStatus{Running: true, CurrentTask: "TASK-007"},
	}
	result, cmd := m.handleAltRunDispatch(task)

	if result.statusNote != "Task already running" {
		t.Errorf("expected 'Task already running', got: %q", result.statusNote)
	}
	if cmd != nil {
		t.Error("expected nil cmd when task already running")
	}
	// No sentinel should be written.
	sentinelPath := dispatchSentinelPath(dir, "TASK-007")
	if _, err := os.Stat(sentinelPath); !os.IsNotExist(err) {
		t.Error("expected no sentinel file when task already running")
	}
}

// ── TestActiveTabIndex_UsesContextAppropriateTracker ─────────────────────────

func TestActiveTabIndex_UsesContextAppropriateTracker(t *testing.T) {
	t.Run("selFeature reads activeTabFeature", func(t *testing.T) {
		m := makeModelForCtx(selFeature)
		m.activeTabFeature = 2
		m.activeTabTask = 1
		if got := m.activeTabIndex(); got != 2 {
			t.Errorf("activeTabIndex() = %d, want 2 (activeTabFeature)", got)
		}
	})

	t.Run("selRunningTask reads activeTabTask", func(t *testing.T) {
		m := makeModelForCtx(selRunningTask)
		m.activeTabFeature = 2
		m.activeTabTask = 1
		if got := m.activeTabIndex(); got != 1 {
			t.Errorf("activeTabIndex() = %d, want 1 (activeTabTask)", got)
		}
	})

	t.Run("selCompletedTask reads activeTabTask", func(t *testing.T) {
		m := makeModelForCtx(selCompletedTask)
		m.activeTabFeature = 2
		m.activeTabTask = 1
		if got := m.activeTabIndex(); got != 1 {
			t.Errorf("activeTabIndex() = %d, want 1 (activeTabTask)", got)
		}
	})

	t.Run("selNone returns 0 regardless of trackers", func(t *testing.T) {
		m := makeModelForCtx(selNone)
		m.activeTabFeature = 2
		m.activeTabTask = 1
		if got := m.activeTabIndex(); got != 0 {
			t.Errorf("activeTabIndex() = %d, want 0 for selNone", got)
		}
	})
}

// ── TestSetActiveTabIndex_UpdatesContextAppropriateTracker ───────────────────

func TestSetActiveTabIndex_UpdatesContextAppropriateTracker(t *testing.T) {
	t.Run("selFeature sets activeTabFeature", func(t *testing.T) {
		m := makeModelForCtx(selFeature)
		m.setActiveTabIndex(3)
		if m.activeTabFeature != 3 {
			t.Errorf("activeTabFeature = %d, want 3", m.activeTabFeature)
		}
		if m.activeTabTask != 0 {
			t.Errorf("activeTabTask = %d, want 0 (should not change)", m.activeTabTask)
		}
	})

	t.Run("selRunningTask sets activeTabTask", func(t *testing.T) {
		m := makeModelForCtx(selRunningTask)
		m.setActiveTabIndex(2)
		if m.activeTabTask != 2 {
			t.Errorf("activeTabTask = %d, want 2", m.activeTabTask)
		}
		if m.activeTabFeature != 0 {
			t.Errorf("activeTabFeature = %d, want 0 (should not change)", m.activeTabFeature)
		}
	})

	t.Run("selCompletedTask sets activeTabTask", func(t *testing.T) {
		m := makeModelForCtx(selCompletedTask)
		m.setActiveTabIndex(1)
		if m.activeTabTask != 1 {
			t.Errorf("activeTabTask = %d, want 1", m.activeTabTask)
		}
		if m.activeTabFeature != 0 {
			t.Errorf("activeTabFeature = %d, want 0 (should not change)", m.activeTabFeature)
		}
	})

	t.Run("selNone is a no-op", func(t *testing.T) {
		m := makeModelForCtx(selNone)
		m.activeTabFeature = 1
		m.activeTabTask = 2
		m.setActiveTabIndex(99)
		if m.activeTabFeature != 1 || m.activeTabTask != 2 {
			t.Errorf("selNone setActiveTabIndex should be no-op: feature=%d task=%d", m.activeTabFeature, m.activeTabTask)
		}
	})
}

// ── TestNavigationPreservesTabTrackers ───────────────────────────────────────

func TestNavigationPreservesTabTrackers(t *testing.T) {
	// Two-feature setup for feature↔feature navigation.
	twoFeaturePlans := []parser.Plan{
		{ID: "f1", File: "feature_1.md", Tasks: []parser.Task{{ID: "TASK-001", Title: "T1"}}},
		{ID: "f2", File: "feature_2.md", Tasks: []parser.Task{{ID: "TASK-002", Title: "T2"}}},
	}

	t.Run("navigating between feature rows preserves activeTabFeature", func(t *testing.T) {
		m := statusModel{
			plans:         twoFeaturePlans,
			expandedPlans: make(map[string]bool),
			treeCursor:    0, // on f1 → selFeature
			width:         120,
			height:        40,
		}
		m.activeTabFeature = 2 // set feature tab tracker to Details (index 2)

		// Navigate down to f2 (still selFeature)
		result := pressKey(m, "j")
		if result.activeTabFeature != 2 {
			t.Errorf("activeTabFeature = %d, want 2 after navigating between features", result.activeTabFeature)
		}
	})

	// Two-task setup: expand plan so both task rows are visible.
	twoTaskPlan := []parser.Plan{
		{ID: "f1", File: "feature_1.md", Tasks: []parser.Task{
			{ID: "TASK-001", Title: "T1", Criteria: []parser.Criterion{{Checked: true, Text: "done"}}},
			{ID: "TASK-002", Title: "T2", Criteria: []parser.Criterion{{Checked: true, Text: "done"}}},
		}},
	}

	t.Run("navigating between task rows preserves activeTabTask", func(t *testing.T) {
		m := statusModel{
			plans:         twoTaskPlan,
			expandedPlans: map[string]bool{"f1": true},
			treeCursor:    1, // task row TASK-001 → selCompletedTask
			showAll:       true,
			width:         120,
			height:        40,
		}
		m.activeTabTask = 2 // task tab tracker at index 2 (Details)

		// Navigate down to TASK-002 (still selCompletedTask)
		result := pressKey(m, "j")
		if result.activeTabTask != 2 {
			t.Errorf("activeTabTask = %d, want 2 after navigating between tasks", result.activeTabTask)
		}
	})

	t.Run("navigating from feature to task does not reset activeTabFeature", func(t *testing.T) {
		m := statusModel{
			plans: []parser.Plan{
				{ID: "f1", File: "feature_1.md", Tasks: []parser.Task{
					{ID: "TASK-001", Title: "T1", Criteria: []parser.Criterion{{Checked: true, Text: "done"}}},
				}},
			},
			expandedPlans: map[string]bool{"f1": true},
			treeCursor:    0, // plan row → selFeature
			showAll:       true,
			width:         120,
			height:        40,
		}
		m.activeTabFeature = 2 // feature tracker at Details

		// Navigate down to the task row → selCompletedTask
		result := pressKey(m, "j")
		// Feature tracker must be undisturbed
		if result.activeTabFeature != 2 {
			t.Errorf("activeTabFeature = %d, want 2 (should not be reset when entering task context)", result.activeTabFeature)
		}
	})

	t.Run("navigating from task to feature does not reset activeTabTask", func(t *testing.T) {
		m := statusModel{
			plans: []parser.Plan{
				{ID: "f1", File: "feature_1.md", Tasks: []parser.Task{
					{ID: "TASK-001", Title: "T1", Criteria: []parser.Criterion{{Checked: true, Text: "done"}}},
				}},
			},
			expandedPlans: map[string]bool{"f1": true},
			treeCursor:    1, // task row → selCompletedTask
			showAll:       true,
			width:         120,
			height:        40,
		}
		m.activeTabTask = 1 // task tracker at Output

		// Navigate up to the plan row → selFeature
		result := pressKey(m, "k")
		// Task tracker must be undisturbed
		if result.activeTabTask != 1 {
			t.Errorf("activeTabTask = %d, want 1 (should not be reset when entering feature context)", result.activeTabTask)
		}
	})
}
