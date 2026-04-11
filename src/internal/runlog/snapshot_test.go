package runlog

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/leberkas-org/maggus/internal/agent"
)

func TestWriteSnapshot_CreatesValidJSON(t *testing.T) {
	dir := t.TempDir()

	snap := StateSnapshot{
		RunID:     "test-run-001",
		TaskID:    "TASK-001",
		TaskTitle: "Test task",
		ItemTitle: "feature_001.md",
		Status:    "Running tool",
		ToolEntries: []SnapshotToolEntry{
			{Type: "Read", Icon: "📖", Description: "Read: foo.go", Timestamp: "2026-01-01T00:00:00Z"},
			{Type: "Bash", Icon: "⚡", Description: "go test ./...", Timestamp: "2026-01-01T00:00:01Z"},
		},
		TokenInput:  1000,
		TokenOutput: 500,
		TokenCost:   0.05,
		ModelBreakdown: map[string]agent.ModelTokens{
			"claude-opus-4-6": {InputTokens: 1000, OutputTokens: 500, CostUSD: 0.05},
		},
		Commits: []string{"feat: add snapshot"},
	}

	if err := WriteSnapshot(dir, snap); err != nil {
		t.Fatalf("WriteSnapshot failed: %v", err)
	}

	// Verify file exists.
	target := filepath.Join(dir, ".maggus", "runs", "state.json")
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("state.json not found: %v", err)
	}

	// Verify valid JSON.
	var parsed StateSnapshot
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("state.json is not valid JSON: %v", err)
	}

	// Verify fields.
	if parsed.TaskID != "TASK-001" {
		t.Errorf("TaskID = %q, want %q", parsed.TaskID, "TASK-001")
	}
	if parsed.Status != "Running tool" {
		t.Errorf("Status = %q, want %q", parsed.Status, "Running tool")
	}
	if len(parsed.ToolEntries) != 2 {
		t.Errorf("ToolEntries len = %d, want 2", len(parsed.ToolEntries))
	}
	if parsed.TokenInput != 1000 {
		t.Errorf("TokenInput = %d, want 1000", parsed.TokenInput)
	}
	if parsed.UpdatedAt == "" {
		t.Error("UpdatedAt should be set")
	}
	if len(parsed.Commits) != 1 || parsed.Commits[0] != "feat: add snapshot" {
		t.Errorf("Commits = %v, want [feat: add snapshot]", parsed.Commits)
	}
}

func TestWriteSnapshot_Atomic_OverwritesExisting(t *testing.T) {
	dir := t.TempDir()

	// Write initial snapshot.
	snap1 := StateSnapshot{TaskID: "TASK-001", Status: "Thinking..."}
	if err := WriteSnapshot(dir, snap1); err != nil {
		t.Fatalf("first WriteSnapshot failed: %v", err)
	}

	// Overwrite with updated snapshot.
	snap2 := StateSnapshot{TaskID: "TASK-001", Status: "Done"}
	if err := WriteSnapshot(dir, snap2); err != nil {
		t.Fatalf("second WriteSnapshot failed: %v", err)
	}

	// Verify the file contains the second snapshot.
	parsed, err := ReadSnapshot(dir)
	if err != nil {
		t.Fatalf("ReadSnapshot failed: %v", err)
	}
	if parsed.Status != "Done" {
		t.Errorf("Status = %q, want %q", parsed.Status, "Done")
	}
}

func TestRemoveSnapshot_CleansUp(t *testing.T) {
	dir := t.TempDir()

	snap := StateSnapshot{TaskID: "TASK-001", Status: "Running"}
	if err := WriteSnapshot(dir, snap); err != nil {
		t.Fatalf("WriteSnapshot failed: %v", err)
	}

	RemoveSnapshot(dir)

	target := filepath.Join(dir, ".maggus", "runs", "state.json")
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Error("state.json should have been removed")
	}
}

func TestRemoveSnapshot_NoErrorWhenMissing(t *testing.T) {
	dir := t.TempDir()
	// Should not panic or error on missing file.
	RemoveSnapshot(dir)
}

func TestReadSnapshot_ReturnsErrorWhenMissing(t *testing.T) {
	dir := t.TempDir()
	_, err := ReadSnapshot(dir)
	if err == nil {
		t.Error("expected error when reading missing snapshot")
	}
}

func TestWriteSnapshot_TimestampsPresent(t *testing.T) {
	dir := t.TempDir()

	snap := StateSnapshot{
		TaskID:        "TASK-006",
		Status:        "Working",
		RunStartedAt:  "2026-01-01T00:00:00Z",
		TaskStartedAt: "2026-01-01T00:05:00Z",
	}

	if err := WriteSnapshot(dir, snap); err != nil {
		t.Fatalf("WriteSnapshot failed: %v", err)
	}

	target := filepath.Join(dir, ".maggus", "runs", "state.json")
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("state.json not found: %v", err)
	}

	// Verify both timestamps are present in raw JSON.
	raw := string(data)
	if !strings.Contains(raw, `"run_started_at"`) {
		t.Error("run_started_at not found in serialized JSON")
	}
	if !strings.Contains(raw, `"task_started_at"`) {
		t.Error("task_started_at not found in serialized JSON")
	}

	// Verify round-trip via struct.
	var parsed StateSnapshot
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if parsed.RunStartedAt != "2026-01-01T00:00:00Z" {
		t.Errorf("RunStartedAt = %q, want %q", parsed.RunStartedAt, "2026-01-01T00:00:00Z")
	}
	if parsed.TaskStartedAt != "2026-01-01T00:05:00Z" {
		t.Errorf("TaskStartedAt = %q, want %q", parsed.TaskStartedAt, "2026-01-01T00:05:00Z")
	}
}

func TestUpsertWorkerEntry_AddsNew(t *testing.T) {
	dir := t.TempDir()

	entry := WorkerIndexEntry{
		TaskID:    "TASK-001-001",
		TaskTitle: "First task",
		Status:    "working",
		StartedAt: "2026-01-01T00:00:00Z",
	}
	if err := UpsertWorkerEntry(dir, entry); err != nil {
		t.Fatalf("UpsertWorkerEntry failed: %v", err)
	}

	workers := ReadWorkersIndex(dir)
	if len(workers) != 1 {
		t.Fatalf("expected 1 worker, got %d", len(workers))
	}
	if workers[0].TaskID != "TASK-001-001" {
		t.Errorf("TaskID = %q, want %q", workers[0].TaskID, "TASK-001-001")
	}
	if workers[0].Status != "working" {
		t.Errorf("Status = %q, want %q", workers[0].Status, "working")
	}
}

func TestUpsertWorkerEntry_UpdatesExisting(t *testing.T) {
	dir := t.TempDir()

	// Write initial entry.
	_ = UpsertWorkerEntry(dir, WorkerIndexEntry{
		TaskID: "TASK-001-001", TaskTitle: "First", Status: "working",
	})

	// Upsert same task with new status.
	_ = UpsertWorkerEntry(dir, WorkerIndexEntry{
		TaskID: "TASK-001-001", TaskTitle: "First updated", Status: "done",
	})

	workers := ReadWorkersIndex(dir)
	if len(workers) != 1 {
		t.Fatalf("expected 1 worker (upserted, not duplicated), got %d", len(workers))
	}
	if workers[0].Status != "done" {
		t.Errorf("Status = %q, want %q", workers[0].Status, "done")
	}
	if workers[0].TaskTitle != "First updated" {
		t.Errorf("TaskTitle = %q, want %q", workers[0].TaskTitle, "First updated")
	}
}

func TestUpsertWorkerEntry_PreservesOtherWorkers(t *testing.T) {
	dir := t.TempDir()

	// Write two workers.
	_ = WriteWorkersIndex(dir, []WorkerIndexEntry{
		{TaskID: "TASK-001-001", Status: "done"},
		{TaskID: "TASK-001-002", Status: "working"},
	})

	// Upsert a third worker.
	_ = UpsertWorkerEntry(dir, WorkerIndexEntry{
		TaskID: "TASK-001-003", Status: "working",
	})

	workers := ReadWorkersIndex(dir)
	if len(workers) != 3 {
		t.Fatalf("expected 3 workers, got %d", len(workers))
	}
}

func TestUpdateWorkerStatus_UpdatesMatchingEntry(t *testing.T) {
	dir := t.TempDir()

	_ = WriteWorkersIndex(dir, []WorkerIndexEntry{
		{TaskID: "TASK-001-001", Status: "working"},
		{TaskID: "TASK-001-002", Status: "working"},
	})

	if err := UpdateWorkerStatus(dir, "TASK-001-001", "done"); err != nil {
		t.Fatalf("UpdateWorkerStatus failed: %v", err)
	}

	workers := ReadWorkersIndex(dir)
	if len(workers) != 2 {
		t.Fatalf("expected 2 workers, got %d", len(workers))
	}
	if workers[0].Status != "done" {
		t.Errorf("workers[0].Status = %q, want %q", workers[0].Status, "done")
	}
	if workers[1].Status != "working" {
		t.Errorf("workers[1].Status = %q, want %q", workers[1].Status, "working")
	}
}

func TestUpdateWorkerStatus_NoopForUnknownTaskID(t *testing.T) {
	dir := t.TempDir()

	_ = WriteWorkersIndex(dir, []WorkerIndexEntry{
		{TaskID: "TASK-001-001", Status: "working"},
	})

	if err := UpdateWorkerStatus(dir, "TASK-999-001", "done"); err != nil {
		t.Fatalf("UpdateWorkerStatus should not fail for unknown task: %v", err)
	}

	workers := ReadWorkersIndex(dir)
	if len(workers) != 1 {
		t.Fatalf("expected 1 worker, got %d", len(workers))
	}
	if workers[0].Status != "working" {
		t.Errorf("Status should be unchanged: %q", workers[0].Status)
	}
}

// BUG-039-004: tests for PruneStaleWorkerEntries.

func TestPruneStaleWorkerEntries_RemovesOldTerminalEntries(t *testing.T) {
	dir := t.TempDir()

	old := time.Now().Add(-10 * time.Minute).UTC().Format(time.RFC3339)
	_ = WriteWorkersIndex(dir, []WorkerIndexEntry{
		{TaskID: "TASK-001-001", Status: "done", StartedAt: old},
		{TaskID: "TASK-001-002", Status: "failed", StartedAt: old},
		{TaskID: "TASK-001-003", Status: "working", StartedAt: old},
	})

	if err := PruneStaleWorkerEntries(dir, 5*time.Minute); err != nil {
		t.Fatalf("PruneStaleWorkerEntries failed: %v", err)
	}

	workers := ReadWorkersIndex(dir)
	if len(workers) != 1 {
		t.Fatalf("expected 1 worker after prune (only 'working'), got %d", len(workers))
	}
	if workers[0].TaskID != "TASK-001-003" {
		t.Errorf("retained worker = %q, want %q", workers[0].TaskID, "TASK-001-003")
	}
}

func TestPruneStaleWorkerEntries_KeepsRecentTerminalEntries(t *testing.T) {
	dir := t.TempDir()

	recent := time.Now().Add(-1 * time.Minute).UTC().Format(time.RFC3339)
	_ = WriteWorkersIndex(dir, []WorkerIndexEntry{
		{TaskID: "TASK-001-001", Status: "done", StartedAt: recent},
		{TaskID: "TASK-001-002", Status: "failed", StartedAt: recent},
	})

	if err := PruneStaleWorkerEntries(dir, 5*time.Minute); err != nil {
		t.Fatalf("PruneStaleWorkerEntries failed: %v", err)
	}

	// Recent terminal entries should be kept (still within 5-minute window).
	workers := ReadWorkersIndex(dir)
	if len(workers) != 2 {
		t.Fatalf("expected 2 workers kept (recent terminal), got %d", len(workers))
	}
}

func TestPruneStaleWorkerEntries_AlwaysPrunesEmptyStartedAt(t *testing.T) {
	dir := t.TempDir()

	_ = WriteWorkersIndex(dir, []WorkerIndexEntry{
		{TaskID: "TASK-001-001", Status: "done", StartedAt: ""},   // no timestamp
		{TaskID: "TASK-001-002", Status: "failed", StartedAt: ""}, // no timestamp
		{TaskID: "TASK-001-003", Status: "working", StartedAt: ""},
	})

	if err := PruneStaleWorkerEntries(dir, 5*time.Minute); err != nil {
		t.Fatalf("PruneStaleWorkerEntries failed: %v", err)
	}

	workers := ReadWorkersIndex(dir)
	if len(workers) != 1 {
		t.Fatalf("expected 1 worker after prune (empty StartedAt terminal entries removed), got %d", len(workers))
	}
	if workers[0].TaskID != "TASK-001-003" {
		t.Errorf("retained worker = %q, want %q", workers[0].TaskID, "TASK-001-003")
	}
}

func TestPruneStaleWorkerEntries_PrunesBlockedEntries(t *testing.T) {
	dir := t.TempDir()

	old := time.Now().Add(-10 * time.Minute).UTC().Format(time.RFC3339)
	_ = WriteWorkersIndex(dir, []WorkerIndexEntry{
		{TaskID: "TASK-001-001", Status: "blocked", StartedAt: old},
		{TaskID: "TASK-001-002", Status: "working", StartedAt: old},
	})

	if err := PruneStaleWorkerEntries(dir, 5*time.Minute); err != nil {
		t.Fatalf("PruneStaleWorkerEntries failed: %v", err)
	}

	workers := ReadWorkersIndex(dir)
	if len(workers) != 1 {
		t.Fatalf("expected 1 worker after prune (blocked removed), got %d", len(workers))
	}
	if workers[0].Status != "working" {
		t.Errorf("retained worker status = %q, want %q", workers[0].Status, "working")
	}
}

func TestPruneStaleWorkerEntries_NoopWhenEmpty(t *testing.T) {
	dir := t.TempDir()
	// No index file — should not error.
	if err := PruneStaleWorkerEntries(dir, 5*time.Minute); err != nil {
		t.Fatalf("PruneStaleWorkerEntries should not fail with no index: %v", err)
	}
}

func TestPruneStaleWorkerEntries_NoopWhenNothingStale(t *testing.T) {
	dir := t.TempDir()

	recent := time.Now().Add(-1 * time.Minute).UTC().Format(time.RFC3339)
	_ = WriteWorkersIndex(dir, []WorkerIndexEntry{
		{TaskID: "TASK-001-001", Status: "working", StartedAt: recent},
	})

	if err := PruneStaleWorkerEntries(dir, 5*time.Minute); err != nil {
		t.Fatalf("PruneStaleWorkerEntries failed: %v", err)
	}

	workers := ReadWorkersIndex(dir)
	if len(workers) != 1 {
		t.Fatalf("expected 1 worker unchanged, got %d", len(workers))
	}
}

func TestWriteSnapshot_NoTempFileLeftBehind(t *testing.T) {
	dir := t.TempDir()

	snap := StateSnapshot{TaskID: "TASK-001"}
	if err := WriteSnapshot(dir, snap); err != nil {
		t.Fatalf("WriteSnapshot failed: %v", err)
	}

	runsDir := filepath.Join(dir, ".maggus", "runs")
	entries, err := os.ReadDir(runsDir)
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}

	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".tmp" {
			t.Errorf("temp file left behind: %s", e.Name())
		}
	}
}
