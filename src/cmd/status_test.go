package cmd

import (
	"os"
	"strings"
	"testing"

	"github.com/leberkas-org/maggus/internal/approval"
	"github.com/leberkas-org/maggus/internal/claude2x"
	"github.com/leberkas-org/maggus/internal/parser"
	"github.com/leberkas-org/maggus/internal/tui/styles"
)

func TestFeatureInfo_ApprovalKey(t *testing.T) {
	t.Run("uses maggusID when set", func(t *testing.T) {
		f := &featureInfo{filename: "feature_001.md", maggusID: "abc-123"}
		if got := f.approvalKey(); got != "abc-123" {
			t.Errorf("approvalKey() = %q, want %q", got, "abc-123")
		}
	})

	t.Run("falls back to featureIDFromPath when maggusID empty", func(t *testing.T) {
		f := &featureInfo{filename: "feature_001.md"}
		want := featureIDFromPath("feature_001.md")
		if got := f.approvalKey(); got != want {
			t.Errorf("approvalKey() = %q, want %q", got, want)
		}
	})
}

func TestPruneStaleApprovals(t *testing.T) {
	dir := t.TempDir()
	maggusDir := dir + "/.maggus"
	if err := os.MkdirAll(maggusDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Write an approvals file with a stale entry
	data := "stale-uuid: true\nactive-uuid: true\n"
	if err := os.WriteFile(maggusDir+"/feature_approvals.yml", []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}

	features := []featureInfo{
		{filename: "feature_001.md", maggusID: "active-uuid"},
	}
	pruneStaleApprovals(dir, features)

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
	dir := t.TempDir()
	maggusDir := dir + "/.maggus"
	if err := os.MkdirAll(maggusDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Feature without maggusID uses filename-based key
	fallbackKey := featureIDFromPath("feature_002.md")
	data := fallbackKey + ": true\nold-entry: false\n"
	if err := os.WriteFile(maggusDir+"/feature_approvals.yml", []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}

	features := []featureInfo{
		{filename: "feature_002.md"}, // no maggusID
	}
	pruneStaleApprovals(dir, features)

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

	t.Run("unapproved plan shown with marker", func(t *testing.T) {
		plans := []featureInfo{
			{filename: "plan_1.md", approved: false, tasks: []parser.Task{
				{ID: "TASK-001", Title: "Unapproved task", SourceFile: "plan_1.md", Criteria: []parser.Criterion{{Checked: false}}},
			}},
		}
		var sb strings.Builder
		renderStatusPlain(&sb, plans, false, "", "", "claude")
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
		renderStatusPlain(&sb, nil, false, "", "", "claude")
		out := sb.String()

		if !strings.Contains(out, "0 features (0 active), 0 tasks total") {
			t.Error("empty features should show zero counts")
		}
	})

	t.Run("plans table status labels", func(t *testing.T) {
		plans := []featureInfo{
			{filename: "plan_new.md", approved: true, tasks: []parser.Task{
				{ID: "TASK-001", Criteria: []parser.Criterion{{Checked: false}}},
			}},
			{filename: "plan_progress.md", approved: true, tasks: []parser.Task{
				{ID: "TASK-001", Criteria: []parser.Criterion{{Checked: true}}},
				{ID: "TASK-002", Criteria: []parser.Criterion{{Checked: false}}},
			}},
			{filename: "plan_done.md", approved: true, tasks: []parser.Task{
				{ID: "TASK-001", Criteria: []parser.Criterion{{Checked: true}}},
			}},
			{filename: "plan_blocked.md", approved: true, tasks: []parser.Task{
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
		m := newStatusModel(plans, false, "TASK-002", "plan_1.md", "claude", "/tmp", false, false)
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
		m := newStatusModel(plans, true, "TASK-002", "plan_1.md", "claude", "/tmp", false, false)
		if len(m.Tasks) != 2 {
			t.Errorf("Tasks len = %d, want 2", len(m.Tasks))
		}
	})

	t.Run("empty plans", func(t *testing.T) {
		m := newStatusModel(nil, false, "", "", "claude", "/tmp", false, false)
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

func TestStatusModel_InitReturnsCmd(t *testing.T) {
	m := newStatusModel(nil, false, "", "", "claude", "/tmp", false, false)
	cmd := m.Init()
	if cmd == nil {
		t.Error("Init() should return a non-nil Cmd for async 2x fetch")
	}
}

func TestStatusModel_UpdateClaude2xResult(t *testing.T) {
	plans := []featureInfo{
		{filename: "plan_1.md", tasks: []parser.Task{
			{ID: "TASK-001", Criteria: []parser.Criterion{{Checked: false}}},
		}},
	}

	t.Run("2x active sets is2x and BorderColor to Warning", func(t *testing.T) {
		m := newStatusModel(plans, false, "TASK-001", "plan_1.md", "claude", "/tmp", false, false)
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
		m := newStatusModel(plans, false, "TASK-001", "plan_1.md", "claude", "/tmp", false, false)
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
	plans := []featureInfo{
		{filename: "plan_1.md", tasks: []parser.Task{
			{ID: "TASK-001", Title: "Test", Criteria: []parser.Criterion{{Checked: false}}},
		}},
	}

	t.Run("non-2x view does not contain yellow border styling", func(t *testing.T) {
		m := newStatusModel(plans, false, "TASK-001", "plan_1.md", "claude", "/tmp", false, false)
		m.is2x = false
		view := m.View()
		// Verify the view renders without error and contains expected content
		if !strings.Contains(view, "Status") {
			t.Error("view should contain 'Status' header")
		}
		if !strings.Contains(view, "TASK-001") {
			t.Error("view should contain task ID")
		}
	})

	t.Run("2x view renders without error", func(t *testing.T) {
		m := newStatusModel(plans, false, "TASK-001", "plan_1.md", "claude", "/tmp", false, false)
		m.is2x = true
		view := m.View()
		if !strings.Contains(view, "Status") {
			t.Error("view should contain 'Status' header")
		}
		if !strings.Contains(view, "TASK-001") {
			t.Error("view should contain task ID")
		}
	})

	t.Run("empty features view renders with 2x", func(t *testing.T) {
		m := newStatusModel(nil, false, "", "", "claude", "/tmp", false, false)
		m.is2x = true
		view := m.View()
		if !strings.Contains(view, "No features found") {
			t.Error("empty view should contain 'No features found'")
		}
	})
}

func TestFindNextTask_BugsPrioritized(t *testing.T) {
	t.Run("bugs before features", func(t *testing.T) {
		items := []featureInfo{
			{
				filename: "feature_001.md",
				tasks: []parser.Task{
					{ID: "TASK-001-001", SourceFile: "feature_001.md", Criteria: []parser.Criterion{{Checked: false}}},
				},
			},
			{
				filename: "bug_001.md",
				isBug:    true,
				tasks: []parser.Task{
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
		items := []featureInfo{
			{
				filename: "feature_001.md",
				tasks: []parser.Task{
					{ID: "TASK-001-001", SourceFile: "feature_001.md", Criteria: []parser.Criterion{{Checked: false}}},
				},
			},
			{
				filename:  "bug_001.md",
				isBug:     true,
				completed: true,
				tasks: []parser.Task{
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
		items := []featureInfo{
			{
				filename: "bug_001.md",
				isBug:    true,
				tasks: []parser.Task{
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
	items := []featureInfo{
		{filename: "feature_001.md"},
		{filename: "bug_001.md", isBug: true},
		{filename: "bug_002_completed.md", isBug: true, completed: true},
	}

	t.Run("showAll false hides completed bugs", func(t *testing.T) {
		m := statusModel{features: items, showAll: false}
		got := m.visibleFeatures()
		if len(got) != 2 {
			t.Fatalf("len = %d, want 2", len(got))
		}
		if got[1].filename != "bug_001.md" {
			t.Errorf("got %s, want bug_001.md", got[1].filename)
		}
	})

	t.Run("showAll true shows completed bugs", func(t *testing.T) {
		m := statusModel{features: items, showAll: true}
		got := m.visibleFeatures()
		if len(got) != 3 {
			t.Fatalf("len = %d, want 3", len(got))
		}
	})
}

func TestRenderStatusPlain_WithBugs(t *testing.T) {
	t.Run("mixed features and bugs", func(t *testing.T) {
		items := []featureInfo{
			{
				filename: "feature_001.md",
				tasks: []parser.Task{
					{ID: "TASK-001-001", Title: "Feature task", SourceFile: "f", Criteria: []parser.Criterion{{Checked: false}}},
				},
			},
			{
				filename: "bug_001.md",
				isBug:    true,
				tasks: []parser.Task{
					{ID: "BUG-001-001", Title: "Bug task", SourceFile: "b", Criteria: []parser.Criterion{{Checked: false}}},
				},
			},
		}
		var sb strings.Builder
		renderStatusPlain(&sb, items, false, "BUG-001-001", "b", "claude")
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
		items := []featureInfo{
			{
				filename: "feature_001.md",
				tasks: []parser.Task{
					{ID: "TASK-001-001", Title: "Task", SourceFile: "f", Criteria: []parser.Criterion{{Checked: false}}},
				},
			},
		}
		var sb strings.Builder
		renderStatusPlain(&sb, items, false, "TASK-001-001", "f", "claude")
		out := sb.String()

		if strings.Contains(out, "bugs") {
			t.Error("should not mention bugs when there are none")
		}
	})
}

func TestNewStatusModel_WithBugs(t *testing.T) {
	items := []featureInfo{
		{
			filename: "feature_001.md",
			tasks: []parser.Task{
				{ID: "TASK-001-001", Criteria: []parser.Criterion{{Checked: false}}},
			},
		},
		{
			filename: "bug_001.md",
			isBug:    true,
			tasks: []parser.Task{
				{ID: "BUG-001-001", Criteria: []parser.Criterion{{Checked: false}}},
			},
		},
	}

	t.Run("tabs include bugs", func(t *testing.T) {
		m := newStatusModel(items, false, "BUG-001-001", "bug_001.md", "claude", "/tmp", false, false)
		visible := m.visibleFeatures()
		if len(visible) != 2 {
			t.Fatalf("visible features = %d, want 2", len(visible))
		}
		if !visible[1].isBug {
			t.Error("second visible item should be a bug")
		}
	})

	t.Run("navigation to bug tab", func(t *testing.T) {
		m := newStatusModel(items, false, "BUG-001-001", "bug_001.md", "claude", "/tmp", false, false)
		m.selectedFeature = 1
		m.rebuildForSelectedFeature()
		if len(m.Tasks) != 1 {
			t.Fatalf("Tasks len = %d, want 1", len(m.Tasks))
		}
		if m.Tasks[0].ID != "BUG-001-001" {
			t.Errorf("Tasks[0].ID = %q, want BUG-001-001", m.Tasks[0].ID)
		}
	})
}

func TestStatusModel_ViewWithBugs(t *testing.T) {
	items := []featureInfo{
		{
			filename: "feature_001.md",
			tasks: []parser.Task{
				{ID: "TASK-001-001", Title: "Feature task", Criteria: []parser.Criterion{{Checked: false}}},
			},
		},
		{
			filename: "bug_001.md",
			isBug:    true,
			tasks: []parser.Task{
				{ID: "BUG-001-001", Title: "Bug task", Criteria: []parser.Criterion{{Checked: false}}},
			},
		},
	}

	t.Run("view renders bug tabs", func(t *testing.T) {
		m := newStatusModel(items, false, "BUG-001-001", "bug_001.md", "claude", "/tmp", false, false)
		view := m.View()
		if !strings.Contains(view, "bug_001") {
			t.Error("view should contain bug tab label")
		}
		if !strings.Contains(view, "feature_001") {
			t.Error("view should contain feature tab label")
		}
	})

	t.Run("view shows bug header counts", func(t *testing.T) {
		m := newStatusModel(items, false, "BUG-001-001", "bug_001.md", "claude", "/tmp", false, false)
		view := m.View()
		if !strings.Contains(view, "1 bugs") {
			t.Error("view header should show bug count")
		}
	})

	t.Run("selected bug tab shows bug tasks", func(t *testing.T) {
		m := newStatusModel(items, false, "BUG-001-001", "bug_001.md", "claude", "/tmp", false, false)
		m.selectedFeature = 1
		m.rebuildForSelectedFeature()
		view := m.View()
		if !strings.Contains(view, "BUG-001-001") {
			t.Error("view should show bug task ID when bug tab is selected")
		}
	})
}

func TestRenderTabBar_BugSeparator(t *testing.T) {
	items := []featureInfo{
		{filename: "feature_001.md", approved: true, tasks: []parser.Task{{ID: "T1"}}},
		{filename: "bug_001.md", isBug: true, approved: true, tasks: []parser.Task{{ID: "B1"}}},
	}
	m := statusModel{features: items, showAll: false}
	m.Width = 120
	bar := m.renderTabBar()
	// The separator ┃ should appear between features and bugs
	if !strings.Contains(bar, "┃") {
		t.Error("tab bar should contain ┃ separator between features and bugs")
	}
}

func TestRenderTabBar_ApprovalMark(t *testing.T) {
	t.Run("approved feature shows checkmark", func(t *testing.T) {
		items := []featureInfo{
			{filename: "feature_001.md", approved: true, tasks: []parser.Task{{ID: "T1"}}},
		}
		m := statusModel{features: items, showAll: false}
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
		items := []featureInfo{
			{filename: "feature_001.md", approved: false, tasks: []parser.Task{{ID: "T1"}}},
		}
		m := statusModel{features: items, showAll: false}
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
		items := []featureInfo{
			{filename: "feature_001.md", approved: true, tasks: []parser.Task{{ID: "T1"}}},
			{filename: "feature_002.md", approved: false, tasks: []parser.Task{{ID: "T2"}}},
		}
		m := statusModel{features: items, showAll: false}
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
