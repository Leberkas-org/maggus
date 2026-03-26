package runlog

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leberkas-org/maggus/internal/agent"
)

func TestWriteSnapshot_CreatesValidJSON(t *testing.T) {
	dir := t.TempDir()
	runID := "test-run-001"

	snap := StateSnapshot{
		TaskID:      "TASK-001",
		TaskTitle:   "Test task",
		ItemTitle: "feature_001.md",
		Status:      "Running tool",
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

	if err := WriteSnapshot(dir, runID, snap); err != nil {
		t.Fatalf("WriteSnapshot failed: %v", err)
	}

	// Verify file exists.
	target := filepath.Join(dir, ".maggus", "runs", runID, "state.json")
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
	runID := "test-run-002"

	// Write initial snapshot.
	snap1 := StateSnapshot{TaskID: "TASK-001", Status: "Thinking..."}
	if err := WriteSnapshot(dir, runID, snap1); err != nil {
		t.Fatalf("first WriteSnapshot failed: %v", err)
	}

	// Overwrite with updated snapshot.
	snap2 := StateSnapshot{TaskID: "TASK-001", Status: "Done"}
	if err := WriteSnapshot(dir, runID, snap2); err != nil {
		t.Fatalf("second WriteSnapshot failed: %v", err)
	}

	// Verify the file contains the second snapshot.
	parsed, err := ReadSnapshot(dir, runID)
	if err != nil {
		t.Fatalf("ReadSnapshot failed: %v", err)
	}
	if parsed.Status != "Done" {
		t.Errorf("Status = %q, want %q", parsed.Status, "Done")
	}
}

func TestRemoveSnapshot_CleansUp(t *testing.T) {
	dir := t.TempDir()
	runID := "test-run-003"

	snap := StateSnapshot{TaskID: "TASK-001", Status: "Running"}
	if err := WriteSnapshot(dir, runID, snap); err != nil {
		t.Fatalf("WriteSnapshot failed: %v", err)
	}

	RemoveSnapshot(dir, runID)

	target := filepath.Join(dir, ".maggus", "runs", runID, "state.json")
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Error("state.json should have been removed")
	}
}

func TestRemoveSnapshot_NoErrorWhenMissing(t *testing.T) {
	dir := t.TempDir()
	// Should not panic or error on missing file.
	RemoveSnapshot(dir, "nonexistent-run")
}

func TestReadSnapshot_ReturnsErrorWhenMissing(t *testing.T) {
	dir := t.TempDir()
	_, err := ReadSnapshot(dir, "nonexistent-run")
	if err == nil {
		t.Error("expected error when reading missing snapshot")
	}
}

func TestWriteSnapshot_TimestampsPresent(t *testing.T) {
	dir := t.TempDir()
	runID := "test-run-timestamps"

	snap := StateSnapshot{
		TaskID:        "TASK-006",
		Status:        "Working",
		RunStartedAt:  "2026-01-01T00:00:00Z",
		TaskStartedAt: "2026-01-01T00:05:00Z",
	}

	if err := WriteSnapshot(dir, runID, snap); err != nil {
		t.Fatalf("WriteSnapshot failed: %v", err)
	}

	target := filepath.Join(dir, ".maggus", "runs", runID, "state.json")
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

func TestWriteSnapshot_NoTempFileLeftBehind(t *testing.T) {
	dir := t.TempDir()
	runID := "test-run-004"

	snap := StateSnapshot{TaskID: "TASK-001"}
	if err := WriteSnapshot(dir, runID, snap); err != nil {
		t.Fatalf("WriteSnapshot failed: %v", err)
	}

	runDir := filepath.Join(dir, ".maggus", "runs", runID)
	entries, err := os.ReadDir(runDir)
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}

	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".tmp" {
			t.Errorf("temp file left behind: %s", e.Name())
		}
	}
}
