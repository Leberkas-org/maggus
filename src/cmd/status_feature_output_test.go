package cmd

import (
	"strings"
	"testing"

	"github.com/leberkas-org/maggus/internal/parser"
	"github.com/leberkas-org/maggus/internal/runlog"
)

// TestBuildFeatureTaskHeader_ZeroTools_NoToolCount verifies that when totalTools == 0,
// the header does not contain any tool count text.
func TestBuildFeatureTaskHeader_ZeroTools_NoToolCount(t *testing.T) {
	task := parser.Task{ID: "TASK-001", Title: "Do the thing"}
	out := buildFeatureTaskHeader(task, nil, false, 80, 0)
	if strings.Contains(out, "tools") {
		t.Errorf("header with 0 tools should not show tool count; got: %q", out)
	}
	if !strings.Contains(out, "TASK-001") {
		t.Errorf("header should contain task ID; got: %q", out)
	}
}

// TestBuildFeatureTaskHeader_WithTools_ShowsToolCount verifies that when totalTools > 0,
// the header includes "[N tools]" in the dimmed meta section.
func TestBuildFeatureTaskHeader_WithTools_ShowsToolCount(t *testing.T) {
	task := parser.Task{ID: "TASK-001", Title: "Do the thing"}
	out := buildFeatureTaskHeader(task, nil, false, 80, 80)
	if !strings.Contains(out, "80 tools") {
		t.Errorf("header with 80 tools should show '80 tools'; got: %q", out)
	}
}

// TestBuildFeatureTaskHeader_SnapAndTools_AllMetaShown verifies that when a snapshot
// exists AND totalTools > 0, the tool count is shown alongside token/cost/duration data.
func TestBuildFeatureTaskHeader_SnapAndTools_AllMetaShown(t *testing.T) {
	task := parser.Task{ID: "TASK-001", Title: "Do the thing"}
	snap := &runlog.StateSnapshot{
		TokenInput:    5000,
		TokenOutput:   2000,
		TaskStartedAt: "2025-01-01T00:00:00Z",
		UpdatedAt:     "2025-01-01T00:01:00Z",
	}
	out := buildFeatureTaskHeader(task, snap, false, 120, 42)
	if !strings.Contains(out, "42 tools") {
		t.Errorf("header should show '42 tools'; got: %q", out)
	}
}

// TestBuildFeatureTaskHeader_ZeroTools_WithSnap_NoToolCount verifies that when a snapshot
// exists but totalTools == 0, no tool count is added (snap may have cost/duration though).
func TestBuildFeatureTaskHeader_ZeroTools_WithSnap_NoToolCount(t *testing.T) {
	task := parser.Task{ID: "TASK-001", Title: "Do the thing"}
	snap := &runlog.StateSnapshot{
		TokenInput:    1000,
		TokenOutput:   500,
		TaskStartedAt: "2025-01-01T00:00:00Z",
		UpdatedAt:     "2025-01-01T00:01:00Z",
	}
	out := buildFeatureTaskHeader(task, snap, false, 120, 0)
	if strings.Contains(out, "tools") {
		t.Errorf("header with 0 tools should not show tool count; got: %q", out)
	}
}

// TestRenderFeatureOutputTab_TotalToolsInHeader verifies that even when tool entries are
// capped for display, the header shows the full total count as "[N tools]".
func TestRenderFeatureOutputTab_TotalToolsInHeader(t *testing.T) {
	// Create a task with 20 tool entries.
	entries := make([]runlog.SnapshotToolEntry, 20)
	for i := range entries {
		entries[i] = runlog.SnapshotToolEntry{
			Type:        "Bash",
			Icon:        "⚡",
			Description: "go test",
			Timestamp:   "2025-01-01T00:00:10Z",
		}
	}
	snap := &runlog.StateSnapshot{
		TaskID:      "TASK-001",
		Status:      "Done",
		ToolEntries: entries,
	}
	task := parser.Task{
		ID:       "TASK-001",
		Title:    "Test task",
		Criteria: []parser.Criterion{{Checked: true, Text: "done"}},
	}
	m := statusModel{
		expandedPlans:         make(map[string]bool),
		cachedFeatureOutput:   []*runlog.StateSnapshot{snap},
		cachedFeatureOutputID: testMaggusID,
		plans: []parser.Plan{
			{ID: "f1", MaggusID: testMaggusID, Tasks: []parser.Task{task}},
		},
		logAutoScroll: true,
		width:         120,
		height:        40,
	}
	// treeCursor = 0 → feature/plan row (not expanded, so no task rows below it).
	m.treeCursor = 0

	// contentH=10, so linesPerTask = max(10/1, 7) = 10 — only 10 of 20 entries shown.
	out := m.renderFeatureOutputTab(80, 10)

	// Header should show the full total count [20 tools].
	if !strings.Contains(out, "20 tools") {
		t.Errorf("feature output header should show '20 tools' total count; got: %q", out)
	}
}

// TestRenderFeatureOutputTab_NilSnapshot_ShowsHeaderNoToolLines verifies that a task
// with no snapshot (pending/blocked) that coexists with another task that has output
// contributes only its header line — no tool lines, no placeholder lines per-task.
func TestRenderFeatureOutputTab_NilSnapshot_ShowsHeaderNoToolLines(t *testing.T) {
	entries := []runlog.SnapshotToolEntry{
		{Type: "Read", Icon: "📖", Description: "main.go", Timestamp: "2025-01-01T00:00:10Z"},
	}
	snapWithEntries := &runlog.StateSnapshot{
		TaskID:      "TASK-001",
		Status:      "Done",
		ToolEntries: entries,
	}
	task1 := parser.Task{
		ID:       "TASK-001",
		Title:    "Has output",
		Criteria: []parser.Criterion{{Checked: true, Text: "done"}},
	}
	task2 := parser.Task{
		ID:    "TASK-002",
		Title: "Pending — no output",
	}
	m := statusModel{
		expandedPlans: make(map[string]bool),
		// task1 has a snap, task2 has nil (pending).
		cachedFeatureOutput:   []*runlog.StateSnapshot{snapWithEntries, nil},
		cachedFeatureOutputID: testMaggusID,
		plans: []parser.Plan{
			{ID: "f1", MaggusID: testMaggusID, Tasks: []parser.Task{task1, task2}},
		},
		logAutoScroll: true,
		width:         120,
		height:        40,
	}
	m.treeCursor = 0 // feature row

	out := m.renderFeatureOutputTab(80, 20)

	// Both task headers should appear.
	if !strings.Contains(out, "TASK-001") {
		t.Errorf("output should contain TASK-001 header; got: %q", out)
	}
	if !strings.Contains(out, "TASK-002") {
		t.Errorf("output should contain TASK-002 header; got: %q", out)
	}
}

// TestRenderFeatureOutputTab_ZeroEntriesSnap_ShowsHeaderOnly verifies that a task
// with a snapshot but zero tool entries shows its header but no tool lines.
func TestRenderFeatureOutputTab_ZeroEntriesSnap_ShowsHeaderOnly(t *testing.T) {
	snap := &runlog.StateSnapshot{
		TaskID:      "TASK-001",
		Status:      "Done",
		ToolEntries: nil,
	}
	task := parser.Task{
		ID:       "TASK-001",
		Title:    "Test task",
		Criteria: []parser.Criterion{{Checked: true, Text: "done"}},
	}
	m := statusModel{
		expandedPlans:         make(map[string]bool),
		cachedFeatureOutput:   []*runlog.StateSnapshot{snap},
		cachedFeatureOutputID: testMaggusID,
		plans: []parser.Plan{
			{ID: "f1", MaggusID: testMaggusID, Tasks: []parser.Task{task}},
		},
		logAutoScroll: true,
		width:         120,
		height:        40,
	}
	m.treeCursor = 0

	out := m.renderFeatureOutputTab(80, 20)

	if !strings.Contains(out, "TASK-001") {
		t.Errorf("output should contain task header; got: %q", out)
	}
	// Zero entries means [0 tools] should NOT appear in the header.
	if strings.Contains(out, "tools") {
		t.Errorf("header with 0 tool entries should not show tool count; got: %q", out)
	}
}
