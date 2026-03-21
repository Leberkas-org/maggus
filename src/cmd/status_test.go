package cmd

import (
	"strings"
	"testing"

	"github.com/leberkas-org/maggus/internal/parser"
)

func TestFeatureInfo_DoneCount(t *testing.T) {
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
			p := &featureInfo{tasks: tt.tasks}
			got := p.doneCount()
			if got != tt.want {
				t.Errorf("doneCount() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestFeatureInfo_BlockedCount(t *testing.T) {
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
			p := &featureInfo{tasks: tt.tasks}
			got := p.blockedCount()
			if got != tt.want {
				t.Errorf("blockedCount() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestBuildSelectableTasksForFeature(t *testing.T) {
	feature := featureInfo{
		tasks: []parser.Task{
			{ID: "TASK-001", Criteria: []parser.Criterion{{Checked: true}}},
			{ID: "TASK-002", Criteria: []parser.Criterion{{Checked: false}}},
			{ID: "TASK-003", Criteria: []parser.Criterion{{Checked: true}}},
		},
	}

	t.Run("showAll false excludes complete", func(t *testing.T) {
		got := buildSelectableTasksForFeature(feature, false)
		if len(got) != 1 {
			t.Fatalf("len = %d, want 1", len(got))
		}
		if got[0].ID != "TASK-002" {
			t.Errorf("got %s, want TASK-002", got[0].ID)
		}
	})

	t.Run("showAll true includes all", func(t *testing.T) {
		got := buildSelectableTasksForFeature(feature, true)
		if len(got) != 3 {
			t.Fatalf("len = %d, want 3", len(got))
		}
	})

	t.Run("empty feature", func(t *testing.T) {
		got := buildSelectableTasksForFeature(featureInfo{}, false)
		if len(got) != 0 {
			t.Errorf("len = %d, want 0", len(got))
		}
	})
}

func TestVisibleFeatures(t *testing.T) {
	plans := []featureInfo{
		{filename: "plan_1.md", completed: false},
		{filename: "plan_2_completed.md", completed: true},
		{filename: "plan_3.md", completed: false},
	}

	t.Run("showAll false hides completed", func(t *testing.T) {
		m := statusModel{features: plans, showAll: false}
		got := m.visibleFeatures()
		if len(got) != 2 {
			t.Fatalf("len = %d, want 2", len(got))
		}
		if got[0].filename != "plan_1.md" || got[1].filename != "plan_3.md" {
			t.Errorf("got %s, %s", got[0].filename, got[1].filename)
		}
	})

	t.Run("showAll true shows all", func(t *testing.T) {
		m := statusModel{features: plans, showAll: true}
		got := m.visibleFeatures()
		if len(got) != 3 {
			t.Fatalf("len = %d, want 3", len(got))
		}
	})

	t.Run("no plans", func(t *testing.T) {
		m := statusModel{features: nil, showAll: false}
		got := m.visibleFeatures()
		if len(got) != 0 {
			t.Errorf("len = %d, want 0", len(got))
		}
	})
}

func TestFindNextTask(t *testing.T) {
	t.Run("finds first incomplete task", func(t *testing.T) {
		plans := []featureInfo{
			{
				filename:  "plan_1.md",
				completed: false,
				tasks: []parser.Task{
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
		plans := []featureInfo{
			{
				filename:  "plan_1_completed.md",
				completed: true,
				tasks: []parser.Task{
					{ID: "TASK-001", SourceFile: "plan_1_completed.md", Criteria: []parser.Criterion{{Checked: false}}},
				},
			},
			{
				filename:  "plan_2.md",
				completed: false,
				tasks: []parser.Task{
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
		plans := []featureInfo{
			{
				filename:  "plan_1.md",
				completed: false,
				tasks: []parser.Task{
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
		plans := []featureInfo{
			{
				filename:  "plan_1.md",
				completed: false,
				tasks: []parser.Task{
					{ID: "TASK-001", Title: "First task", SourceFile: "plan_1.md", Criteria: []parser.Criterion{{Checked: true}}},
					{ID: "TASK-002", Title: "Second task", SourceFile: "plan_1.md", Criteria: []parser.Criterion{{Checked: false}}},
					{ID: "TASK-003", Title: "Blocked task", SourceFile: "plan_1.md", Criteria: []parser.Criterion{{Text: "BLOCKED: dep", Blocked: true}}},
				},
			},
		}
		var sb strings.Builder
		renderStatusPlain(&sb, plans, false, "TASK-002", "plan_1.md", "claude")
		out := sb.String()

		// Header
		if !strings.Contains(out, "1 features (1 active), 3 tasks total") {
			t.Error("missing header summary")
		}
		// Summary line
		if !strings.Contains(out, "1/3 tasks complete") {
			t.Error("missing completion count")
		}
		if !strings.Contains(out, "1 pending") {
			t.Error("missing pending count")
		}
		if !strings.Contains(out, "1 blocked") {
			t.Error("missing blocked count")
		}
		// Agent
		if !strings.Contains(out, "Agent: claude") {
			t.Error("missing agent name")
		}
		// Task list
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
		// Features table
		if !strings.Contains(out, "Features") {
			t.Error("missing Features section")
		}
		if !strings.Contains(out, "plan_1.md") {
			t.Error("missing plan filename in table")
		}
	})

	t.Run("completed plan hidden when showAll false", func(t *testing.T) {
		plans := []featureInfo{
			{filename: "plan_1_completed.md", completed: true, tasks: []parser.Task{
				{ID: "TASK-001", Criteria: []parser.Criterion{{Checked: true}}},
			}},
			{filename: "plan_2.md", completed: false, tasks: []parser.Task{
				{ID: "TASK-010", Title: "Active", SourceFile: "plan_2.md", Criteria: []parser.Criterion{{Checked: false}}},
			}},
		}
		var sb strings.Builder
		renderStatusPlain(&sb, plans, false, "TASK-010", "plan_2.md", "claude")
		out := sb.String()

		if strings.Contains(out, "plan_1_completed.md") {
			t.Error("completed plan should be hidden when showAll is false")
		}
		if !strings.Contains(out, "plan_2.md") {
			t.Error("active plan should be visible")
		}
	})

	t.Run("completed plan shown when showAll true", func(t *testing.T) {
		plans := []featureInfo{
			{filename: "plan_1_completed.md", completed: true, tasks: []parser.Task{
				{ID: "TASK-001", Criteria: []parser.Criterion{{Checked: true}}},
			}},
		}
		var sb strings.Builder
		renderStatusPlain(&sb, plans, true, "", "", "claude")
		out := sb.String()

		if !strings.Contains(out, "plan_1_completed.md") {
			t.Error("completed plan should be visible when showAll is true")
		}
		if !strings.Contains(out, "(archived)") {
			t.Error("completed plan should show (archived)")
		}
	})

	t.Run("ignored plan shown with marker", func(t *testing.T) {
		plans := []featureInfo{
			{filename: "plan_1_ignored.md", ignored: true, tasks: []parser.Task{
				{ID: "TASK-001", Title: "Ign task", Ignored: true, SourceFile: "plan_1_ignored.md", Criteria: []parser.Criterion{{Checked: false}}},
			}},
		}
		var sb strings.Builder
		renderStatusPlain(&sb, plans, false, "", "", "claude")
		out := sb.String()

		if !strings.Contains(out, "[~]") {
			t.Error("ignored plan/task should show [~] marker")
		}
		if !strings.Contains(out, "(ignored)") {
			t.Error("ignored plan should show (ignored)")
		}
	})

	t.Run("empty plans", func(t *testing.T) {
		var sb strings.Builder
		renderStatusPlain(&sb, nil, false, "", "", "claude")
		out := sb.String()

		if !strings.Contains(out, "0 features (0 active), 0 tasks total") {
			t.Error("empty features should show zero counts")
		}
	})

	t.Run("plans table status labels", func(t *testing.T) {
		plans := []featureInfo{
			{filename: "plan_new.md", tasks: []parser.Task{
				{ID: "TASK-001", Criteria: []parser.Criterion{{Checked: false}}},
			}},
			{filename: "plan_progress.md", tasks: []parser.Task{
				{ID: "TASK-001", Criteria: []parser.Criterion{{Checked: true}}},
				{ID: "TASK-002", Criteria: []parser.Criterion{{Checked: false}}},
			}},
			{filename: "plan_done.md", tasks: []parser.Task{
				{ID: "TASK-001", Criteria: []parser.Criterion{{Checked: true}}},
			}},
			{filename: "plan_blocked.md", tasks: []parser.Task{
				{ID: "TASK-001", Criteria: []parser.Criterion{{Text: "BLOCKED: x", Blocked: true}}},
			}},
		}
		var sb strings.Builder
		renderStatusPlain(&sb, plans, false, "TASK-001", "", "claude")
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
	plans := []featureInfo{
		{
			filename: "plan_1.md",
			tasks: []parser.Task{
				{ID: "TASK-001", Criteria: []parser.Criterion{{Checked: true}}},
				{ID: "TASK-002", Criteria: []parser.Criterion{{Checked: false}}},
			},
		},
	}

	t.Run("initializes with correct fields", func(t *testing.T) {
		m := newStatusModel(plans, false, "TASK-002", "plan_1.md", "claude", "/tmp")
		if m.nextTaskID != "TASK-002" {
			t.Errorf("nextTaskID = %q, want TASK-002", m.nextTaskID)
		}
		if m.agentName != "claude" {
			t.Errorf("agentName = %q, want claude", m.agentName)
		}
		if m.showAll {
			t.Error("showAll should be false")
		}
		// showAll=false should exclude complete tasks from selectable
		if len(m.Tasks) != 1 {
			t.Fatalf("Tasks len = %d, want 1", len(m.Tasks))
		}
		if m.Tasks[0].ID != "TASK-002" {
			t.Errorf("Tasks[0].ID = %q, want TASK-002", m.Tasks[0].ID)
		}
	})

	t.Run("showAll includes complete tasks", func(t *testing.T) {
		m := newStatusModel(plans, true, "TASK-002", "plan_1.md", "claude", "/tmp")
		if len(m.Tasks) != 2 {
			t.Errorf("Tasks len = %d, want 2", len(m.Tasks))
		}
	})

	t.Run("empty plans", func(t *testing.T) {
		m := newStatusModel(nil, false, "", "", "claude", "/tmp")
		if len(m.Tasks) != 0 {
			t.Errorf("Tasks len = %d, want 0", len(m.Tasks))
		}
	})
}

func TestRebuildForSelectedPlan(t *testing.T) {
	plans := []featureInfo{
		{filename: "plan_1.md", tasks: []parser.Task{
			{ID: "TASK-001", Criteria: []parser.Criterion{{Checked: false}}},
		}},
		{filename: "plan_2.md", tasks: []parser.Task{
			{ID: "TASK-010", Criteria: []parser.Criterion{{Checked: false}}},
			{ID: "TASK-011", Criteria: []parser.Criterion{{Checked: false}}},
		}},
	}

	t.Run("selects correct plan tasks", func(t *testing.T) {
		m := statusModel{features: plans, selectedFeature: 1, showAll: false}
		m.rebuildForSelectedFeature()
		if len(m.Tasks) != 2 {
			t.Fatalf("len = %d, want 2", len(m.Tasks))
		}
		if m.Tasks[0].ID != "TASK-010" {
			t.Errorf("first task = %s, want TASK-010", m.Tasks[0].ID)
		}
	})

	t.Run("resets cursor", func(t *testing.T) {
		m := statusModel{features: plans, selectedFeature: 0}
		m.Cursor = 5
		m.ScrollOffset = 3
		m.rebuildForSelectedFeature()
		if m.Cursor != 0 {
			t.Errorf("Cursor = %d, want 0", m.Cursor)
		}
		if m.ScrollOffset != 0 {
			t.Errorf("ScrollOffset = %d, want 0", m.ScrollOffset)
		}
	})

	t.Run("out of bounds selectedFeature resets to 0", func(t *testing.T) {
		m := statusModel{features: plans, selectedFeature: 99}
		m.rebuildForSelectedFeature()
		if m.selectedFeature != 0 {
			t.Errorf("selectedFeature = %d, want 0", m.selectedFeature)
		}
	})

	t.Run("empty plans", func(t *testing.T) {
		m := statusModel{features: nil}
		m.rebuildForSelectedFeature()
		if m.Tasks != nil {
			t.Errorf("Tasks should be nil, got %v", m.Tasks)
		}
	})
}

func TestEnsureCursorVisible(t *testing.T) {
	// Use a model with fixed dimensions so visibleTaskLines returns a known value
	m := statusModel{
		taskListComponent: taskListComponent{
			Width:       80,
			Height:      30,
			Cursor:      0,
			HeaderLines: statusHeaderLines,
		},
	}
	// Just verify it doesn't panic and ScrollOffset stays reasonable
	m.ensureCursorVisible()
	if m.ScrollOffset < 0 {
		t.Errorf("ScrollOffset = %d, should not be negative", m.ScrollOffset)
	}

	// Cursor beyond visible range
	m.Cursor = 100
	m.ScrollOffset = 0
	m.ensureCursorVisible()
	if m.ScrollOffset <= 0 {
		t.Errorf("ScrollOffset should advance when cursor is beyond visible range, got %d", m.ScrollOffset)
	}

	// Cursor before ScrollOffset
	m.ScrollOffset = 50
	m.Cursor = 10
	m.ensureCursorVisible()
	if m.ScrollOffset > m.Cursor {
		t.Errorf("ScrollOffset (%d) should not exceed Cursor (%d)", m.ScrollOffset, m.Cursor)
	}
}
