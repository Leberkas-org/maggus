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

func TestLoadCompletedTaskOutput_ReturnsNilForMissingTask(t *testing.T) {
	dir := t.TempDir()
	runsDir := filepath.Join(dir, ".maggus", "runs")
	os.MkdirAll(runsDir, 0755)

	// Write a log file with entries for a different task.
	entries := []runlog.Entry{
		{Ts: "2025-01-01T00:00:00Z", Event: "task_start", TaskID: "TASK-OTHER", Title: "Other"},
		{Ts: "2025-01-01T00:01:00Z", Event: "task_complete", TaskID: "TASK-OTHER", Commit: "abc123"},
	}
	writeLogFile(t, runsDir, "20250101-000000.log", entries)

	snap := loadCompletedTaskOutput(dir, "TASK-001")
	if snap != nil {
		t.Errorf("expected nil for missing task, got %+v", snap)
	}
}

func TestLoadCompletedTaskOutput_LoadsToolEntries(t *testing.T) {
	dir := t.TempDir()
	runsDir := filepath.Join(dir, ".maggus", "runs")
	os.MkdirAll(runsDir, 0755)

	entries := []runlog.Entry{
		{Ts: "2025-01-01T00:00:00Z", Event: "task_start", TaskID: "TASK-001", Title: "Do the thing"},
		{Ts: "2025-01-01T00:00:10Z", Event: "tool_use", TaskID: "TASK-001", Tool: "Read", Input: map[string]string{"file_path": "/foo/bar.go"}},
		{Ts: "2025-01-01T00:00:20Z", Event: "tool_use", TaskID: "TASK-001", Tool: "Edit", Input: map[string]string{"file_path": "/foo/baz.go"}},
		{Ts: "2025-01-01T00:00:30Z", Event: "tool_use", TaskID: "TASK-001", Tool: "Bash", Input: map[string]string{"command": "go test ./..."}},
		{Ts: "2025-01-01T00:01:00Z", Event: "task_complete", TaskID: "TASK-001", Commit: "abc1234"},
		{Ts: "2025-01-01T00:01:01Z", Event: "task_usage", InputTokens: 5000, OutputTokens: 2000, CostUSD: 0.05},
	}
	writeLogFile(t, runsDir, "20250101-000000.log", entries)

	snap := loadCompletedTaskOutput(dir, "TASK-001")
	if snap == nil {
		t.Fatal("expected snapshot, got nil")
	}
	if snap.TaskID != "TASK-001" {
		t.Errorf("TaskID = %q, want TASK-001", snap.TaskID)
	}
	if snap.TaskTitle != "Do the thing" {
		t.Errorf("TaskTitle = %q, want 'Do the thing'", snap.TaskTitle)
	}
	if snap.Status != "Done" {
		t.Errorf("Status = %q, want Done", snap.Status)
	}
	if len(snap.ToolEntries) != 3 {
		t.Errorf("ToolEntries count = %d, want 3", len(snap.ToolEntries))
	}
	if snap.TokenInput != 5000 {
		t.Errorf("TokenInput = %d, want 5000", snap.TokenInput)
	}
	if snap.TokenOutput != 2000 {
		t.Errorf("TokenOutput = %d, want 2000", snap.TokenOutput)
	}
	if snap.TokenCost != 0.05 {
		t.Errorf("TokenCost = %f, want 0.05", snap.TokenCost)
	}
	if len(snap.Commits) != 1 || snap.Commits[0] != "abc1234" {
		t.Errorf("Commits = %v, want [abc1234]", snap.Commits)
	}
}

func TestLoadCompletedTaskOutput_FailedTask(t *testing.T) {
	dir := t.TempDir()
	runsDir := filepath.Join(dir, ".maggus", "runs")
	os.MkdirAll(runsDir, 0755)

	entries := []runlog.Entry{
		{Ts: "2025-01-01T00:00:00Z", Event: "task_start", TaskID: "TASK-002", Title: "Will fail"},
		{Ts: "2025-01-01T00:00:10Z", Event: "tool_use", TaskID: "TASK-002", Tool: "Bash", Input: map[string]string{"command": "make build"}},
		{Ts: "2025-01-01T00:01:00Z", Event: "task_failed", TaskID: "TASK-002", Reason: "build error"},
	}
	writeLogFile(t, runsDir, "20250101-000000.log", entries)

	snap := loadCompletedTaskOutput(dir, "TASK-002")
	if snap == nil {
		t.Fatal("expected snapshot, got nil")
	}
	if snap.Status != "Failed" {
		t.Errorf("Status = %q, want Failed", snap.Status)
	}
	if len(snap.ToolEntries) != 1 {
		t.Errorf("ToolEntries count = %d, want 1", len(snap.ToolEntries))
	}
}

func TestLoadCompletedTaskOutput_ScansNewestLogFirst(t *testing.T) {
	dir := t.TempDir()
	runsDir := filepath.Join(dir, ".maggus", "runs")
	os.MkdirAll(runsDir, 0755)

	// Older log with the task (fewer tools).
	oldEntries := []runlog.Entry{
		{Ts: "2025-01-01T00:00:00Z", Event: "task_start", TaskID: "TASK-001", Title: "Old run"},
		{Ts: "2025-01-01T00:01:00Z", Event: "task_complete", TaskID: "TASK-001"},
	}
	writeLogFile(t, runsDir, "20250101-000000.log", oldEntries)

	// Newer log with the same task (more tools).
	newEntries := []runlog.Entry{
		{Ts: "2025-02-01T00:00:00Z", Event: "task_start", TaskID: "TASK-001", Title: "New run"},
		{Ts: "2025-02-01T00:00:10Z", Event: "tool_use", TaskID: "TASK-001", Tool: "Read", Input: map[string]string{"file_path": "/a.go"}},
		{Ts: "2025-02-01T00:00:20Z", Event: "tool_use", TaskID: "TASK-001", Tool: "Edit", Input: map[string]string{"file_path": "/b.go"}},
		{Ts: "2025-02-01T00:01:00Z", Event: "task_complete", TaskID: "TASK-001", Commit: "def5678"},
	}
	writeLogFile(t, runsDir, "20250201-000000.log", newEntries)

	snap := loadCompletedTaskOutput(dir, "TASK-001")
	if snap == nil {
		t.Fatal("expected snapshot, got nil")
	}
	// Should find the newer log first (sorted descending).
	if snap.TaskTitle != "New run" {
		t.Errorf("TaskTitle = %q, want 'New run' (from newer log)", snap.TaskTitle)
	}
	if len(snap.ToolEntries) != 2 {
		t.Errorf("ToolEntries count = %d, want 2 (from newer log)", len(snap.ToolEntries))
	}
}

func TestFormatToolDescription(t *testing.T) {
	tests := []struct {
		tool     string
		input    map[string]string
		wantHas  string
	}{
		{"Read", map[string]string{"file_path": "/home/user/project/main.go"}, "main.go"},
		{"Edit", map[string]string{"file_path": "/foo/bar.go"}, "bar.go"},
		{"Bash", map[string]string{"command": "go test ./..."}, "go test ./..."},
		{"Grep", map[string]string{"pattern": "TODO"}, "TODO"},
		{"Agent", map[string]string{"description": "Search codebase"}, "Search codebase"},
		{"Unknown", nil, "Unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.tool, func(t *testing.T) {
			got := formatToolDescription(tt.tool, tt.input)
			if !strings.Contains(got, tt.wantHas) {
				t.Errorf("formatToolDescription(%q, %v) = %q, want to contain %q", tt.tool, tt.input, got, tt.wantHas)
			}
		})
	}
}

func TestSnapshotForSelectedTask_Sequential(t *testing.T) {
	snap := &runlog.StateSnapshot{TaskID: "TASK-001", Status: "Running"}
	m := statusModel{
		snapshot:      snap,
		expandedPlans: make(map[string]bool),
		plans: []parser.Plan{
			{ID: "f1", Tasks: []parser.Task{{ID: "TASK-001", Title: "T1"}}},
		},
		daemon: daemonStatus{Running: true, CurrentTask: "TASK-001"},
	}
	m.expandedPlans["f1"] = true
	m.treeCursor = 1 // task row

	got := m.snapshotForSelectedTask()
	if got != snap {
		t.Errorf("expected main snapshot, got %v", got)
	}
}

func TestSnapshotForSelectedTask_Parallel(t *testing.T) {
	mainSnap := &runlog.StateSnapshot{TaskID: "TASK-001", Status: "Running"}
	workerSnap := &runlog.StateSnapshot{TaskID: "TASK-002", Status: "Running"}
	m := statusModel{
		snapshot:        mainSnap,
		expandedPlans:   make(map[string]bool),
		workerIndex:     []runlog.WorkerIndexEntry{{TaskID: "TASK-002", Status: "working"}},
		workerSnapshots: map[string]*runlog.StateSnapshot{"TASK-002": workerSnap},
		plans: []parser.Plan{
			{ID: "f1", Tasks: []parser.Task{
				{ID: "TASK-001", Title: "T1"},
				{ID: "TASK-002", Title: "T2"},
			}},
		},
		daemon: daemonStatus{Running: true},
	}
	m.expandedPlans["f1"] = true
	m.treeCursor = 2 // TASK-002 row

	got := m.snapshotForSelectedTask()
	if got != workerSnap {
		t.Errorf("expected worker snapshot for TASK-002, got TaskID=%q", got.TaskID)
	}
}

func TestEnsureCompletedTaskOutput_CachesAndInvalidates(t *testing.T) {
	dir := t.TempDir()
	runsDir := filepath.Join(dir, ".maggus", "runs")
	os.MkdirAll(runsDir, 0755)

	entries := []runlog.Entry{
		{Ts: "2025-01-01T00:00:00Z", Event: "task_start", TaskID: "TASK-001", Title: "T1"},
		{Ts: "2025-01-01T00:01:00Z", Event: "task_complete", TaskID: "TASK-001"},
	}
	writeLogFile(t, runsDir, "20250101-000000.log", entries)

	task := parser.Task{
		ID:       "TASK-001",
		Title:    "T1",
		Criteria: []parser.Criterion{{Checked: true, Text: "done"}},
	}
	m := statusModel{
		dir:           dir,
		expandedPlans: make(map[string]bool),
		plans: []parser.Plan{
			{ID: "f1", File: "feature_1.md", Tasks: []parser.Task{task}},
		},
	}
	m.expandedPlans["f1"] = true
	m.treeCursor = 1

	// First call should load from disk.
	m.ensureCompletedTaskOutput()
	if m.cachedTaskOutputID != "TASK-001" {
		t.Errorf("cachedTaskOutputID = %q, want TASK-001", m.cachedTaskOutputID)
	}
	if m.cachedTaskOutput == nil {
		t.Fatal("cachedTaskOutput should not be nil")
	}

	// Second call should use cache (no disk read).
	cached := m.cachedTaskOutput
	m.ensureCompletedTaskOutput()
	if m.cachedTaskOutput != cached {
		t.Error("expected cache to be reused on second call")
	}

	// Change selection to a different task — cache should invalidate.
	m.cachedTaskOutputID = "TASK-OTHER"
	m.ensureCompletedTaskOutput()
	if m.cachedTaskOutputID != "TASK-001" {
		t.Errorf("expected cache invalidation; cachedTaskOutputID = %q", m.cachedTaskOutputID)
	}
}

func TestRenderCompletedTaskOutput_ShowsTaskInfo(t *testing.T) {
	snap := &runlog.StateSnapshot{
		TaskID:    "TASK-001",
		TaskTitle: "Implement feature",
		Status:    "Done",
		ToolEntries: []runlog.SnapshotToolEntry{
			{Type: "Read", Icon: "📖", Description: "main.go", Timestamp: "2025-01-01T00:00:10Z"},
			{Type: "Edit", Icon: "✏️", Description: "main.go", Timestamp: "2025-01-01T00:00:20Z"},
		},
		TokenInput:   5000,
		TokenOutput:  2000,
		TokenCost:    0.05,
		TaskStartedAt: "2025-01-01T00:00:00Z",
		UpdatedAt:     "2025-01-01T00:01:00Z",
	}

	task := parser.Task{
		ID:       "TASK-001",
		Title:    "Implement feature",
		Criteria: []parser.Criterion{{Checked: true, Text: "done"}},
	}
	m := statusModel{
		expandedPlans:      make(map[string]bool),
		cachedTaskOutput:   snap,
		cachedTaskOutputID: "TASK-001",
		plans: []parser.Plan{
			{ID: "f1", File: "feature_1.md", Tasks: []parser.Task{task}},
		},
		width:  120,
		height: 40,
	}
	m.expandedPlans["f1"] = true
	m.treeCursor = 1

	out := m.renderCompletedTaskOutput(80, 25)
	if !strings.Contains(out, "TASK-001") {
		t.Errorf("output should contain task ID; got: %q", out)
	}
	if !strings.Contains(out, "Done") {
		t.Errorf("output should contain 'Done'; got: %q", out)
	}
	if !strings.Contains(out, "Tokens:") {
		t.Errorf("output should contain 'Tokens:'; got: %q", out)
	}
}

func TestRenderOutputTab_CompletedTask_ShowsOutput(t *testing.T) {
	snap := &runlog.StateSnapshot{
		TaskID:    "TASK-001",
		TaskTitle: "Test task",
		Status:    "Done",
		ToolEntries: []runlog.SnapshotToolEntry{
			{Type: "Bash", Icon: "⚡", Description: "go test", Timestamp: "2025-01-01T00:00:10Z"},
		},
	}

	task := parser.Task{
		ID:       "TASK-001",
		Title:    "Test task",
		Criteria: []parser.Criterion{{Checked: true, Text: "done"}},
	}
	m := statusModel{
		expandedPlans:      make(map[string]bool),
		cachedTaskOutput:   snap,
		cachedTaskOutputID: "TASK-001",
		plans: []parser.Plan{
			{ID: "f1", File: "feature_1.md", Tasks: []parser.Task{task}},
		},
		width:  120,
		height: 40,
	}
	m.expandedPlans["f1"] = true
	m.treeCursor = 1

	out := m.renderOutputTab(80, 25)
	if !strings.Contains(out, "TASK-001") {
		t.Errorf("completed task Output tab should show task ID; got: %q", out)
	}
	if !strings.Contains(out, "Done") {
		t.Errorf("completed task Output tab should show 'Done'; got: %q", out)
	}
}

func TestRenderOutputTab_RunningTask_NoWorkerGrid(t *testing.T) {
	// When a running task is selected in parallel mode, the Output tab should
	// show the per-task snapshot, NOT the old worker card grid.
	workerSnap := &runlog.StateSnapshot{
		TaskID:    "TASK-001",
		TaskTitle: "Running task",
		Status:    "Running",
		ToolEntries: []runlog.SnapshotToolEntry{
			{Type: "Read", Icon: "📖", Description: "file.go", Timestamp: "2025-01-01T00:00:10Z"},
		},
	}

	task := parser.Task{ID: "TASK-001", Title: "Running task"}
	m := statusModel{
		expandedPlans:   make(map[string]bool),
		workerIndex:     []runlog.WorkerIndexEntry{{TaskID: "TASK-001", Status: "working"}},
		workerSnapshots: map[string]*runlog.StateSnapshot{"TASK-001": workerSnap},
		workerSpinners:  map[string]int{"TASK-001": 0},
		plans: []parser.Plan{
			{ID: "f1", File: "feature_1.md", Tasks: []parser.Task{task}},
		},
		daemon: daemonStatus{Running: true},
		width:  120,
		height: 40,
	}
	m.expandedPlans["f1"] = true
	m.treeCursor = 1

	out := m.renderOutputTab(80, 25)
	// Should show the per-task snapshot view, not the worker grid.
	if !strings.Contains(out, "TASK-001") {
		t.Errorf("running task Output tab should show task ID; got: %q", out)
	}
	// The snapshot view shows "Status:" which the worker grid does not.
	if !strings.Contains(out, "Status:") {
		t.Errorf("running task Output tab should show rich snapshot view; got: %q", out)
	}
}

func TestLogItemCount_CompletedTask(t *testing.T) {
	snap := &runlog.StateSnapshot{
		TaskID: "TASK-001",
		ToolEntries: []runlog.SnapshotToolEntry{
			{Type: "Read"}, {Type: "Edit"}, {Type: "Bash"},
		},
	}

	task := parser.Task{
		ID:       "TASK-001",
		Criteria: []parser.Criterion{{Checked: true, Text: "done"}},
	}
	m := statusModel{
		expandedPlans:      make(map[string]bool),
		cachedTaskOutput:   snap,
		cachedTaskOutputID: "TASK-001",
		plans: []parser.Plan{
			{ID: "f1", File: "feature_1.md", Tasks: []parser.Task{task}},
		},
		width:  120,
		height: 40,
	}
	m.expandedPlans["f1"] = true
	m.treeCursor = 1

	count := m.logItemCount()
	if count != 3 {
		t.Errorf("logItemCount() = %d, want 3", count)
	}
}

// writeLogFile writes JSONL entries to a log file in the runs directory.
func writeLogFile(t *testing.T, runsDir, name string, entries []runlog.Entry) {
	t.Helper()
	path := filepath.Join(runsDir, name)
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create log file: %v", err)
	}
	defer f.Close()
	for _, e := range entries {
		data, err := json.Marshal(e)
		if err != nil {
			t.Fatalf("marshal entry: %v", err)
		}
		f.Write(data)
		f.Write([]byte("\n"))
	}
}
