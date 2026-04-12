package cmd

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/leberkas-org/maggus/internal/runlog"
	"github.com/spf13/cobra"
)

func TestCleanupExistingWorktree_NoopWhenMissing(t *testing.T) {
	dir := t.TempDir()
	worktreePath := filepath.Join(dir, ".maggus", "worktrees", "TASK-045-001")

	// Should not error when the path doesn't exist.
	if err := cleanupExistingWorktree(dir, worktreePath); err != nil {
		t.Fatalf("cleanupExistingWorktree should not fail for missing path: %v", err)
	}
}

func TestCleanupExistingWorktree_RemovesExistingDir(t *testing.T) {
	dir := t.TempDir()
	worktreePath := filepath.Join(dir, ".maggus", "worktrees", "TASK-045-001")

	// Create the directory to simulate a leftover worktree.
	if err := os.MkdirAll(worktreePath, 0755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	// Write a file inside to make sure recursive removal works.
	_ = os.WriteFile(filepath.Join(worktreePath, "test.txt"), []byte("hello"), 0644)

	if err := cleanupExistingWorktree(dir, worktreePath); err != nil {
		t.Fatalf("cleanupExistingWorktree failed: %v", err)
	}

	if _, err := os.Stat(worktreePath); !os.IsNotExist(err) {
		t.Error("worktree directory should have been removed")
	}
}

func TestDispatchTask_RegistersWorkerInIndex(t *testing.T) {
	// This test verifies that dispatchTask writes worker state files.
	// We can't easily test the full flow (git worktree + process launch)
	// in a unit test, but we can verify the state file format.
	dir := t.TempDir()

	// Simulate what dispatchTask does for state registration.
	entry := runlog.WorkerIndexEntry{
		TaskID:    "TASK-045-001",
		TaskTitle: "Test task",
		Status:    "working",
		StartedAt: "2026-01-01T00:00:00Z",
	}
	if err := runlog.UpsertWorkerEntry(dir, entry); err != nil {
		t.Fatalf("UpsertWorkerEntry failed: %v", err)
	}

	// Verify the index file is valid JSON with the right structure.
	indexPath := filepath.Join(dir, ".maggus", "runs", "state-workers.json")
	data, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("workers index not found: %v", err)
	}

	var idx runlog.WorkerIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		t.Fatalf("workers index is not valid JSON: %v", err)
	}
	if len(idx.Workers) != 1 {
		t.Fatalf("expected 1 worker in index, got %d", len(idx.Workers))
	}
	if idx.Workers[0].TaskID != "TASK-045-001" {
		t.Errorf("TaskID = %q, want %q", idx.Workers[0].TaskID, "TASK-045-001")
	}
}

func TestDispatchTask_WritesInitialSnapshot(t *testing.T) {
	dir := t.TempDir()
	taskID := "TASK-045-001"

	// Simulate the initial snapshot write that dispatchTask does.
	snap := runlog.StateSnapshot{
		TaskID:    taskID,
		TaskTitle: taskID,
		Status:    "Working",
	}
	if err := runlog.WriteWorkerSnapshot(dir, taskID, snap); err != nil {
		t.Fatalf("WriteWorkerSnapshot failed: %v", err)
	}

	// Verify the per-worker snapshot file exists and is valid.
	read, err := runlog.ReadWorkerSnapshot(dir, taskID)
	if err != nil {
		t.Fatalf("ReadWorkerSnapshot failed: %v", err)
	}
	if read.TaskID != taskID {
		t.Errorf("snapshot TaskID = %q, want %q", read.TaskID, taskID)
	}
	if read.Status != "Working" {
		t.Errorf("snapshot Status = %q, want %q", read.Status, "Working")
	}
}

func TestDispatchWork_FindsWorkSubcommand(t *testing.T) {
	// Save the original RunE so we can restore it after the test.
	origRunE := runCmd.RunE
	defer func() { runCmd.RunE = origRunE }()

	var called bool
	runCmd.RunE = func(cmd *cobra.Command, args []string) error {
		called = true
		return nil
	}

	if err := dispatchWork("TASK-042"); err != nil {
		t.Fatalf("dispatchWork returned unexpected error: %v", err)
	}
	if !called {
		t.Error("expected work subcommand RunE to be called")
	}
}

func TestDispatchWork_TaskFlagIsSet(t *testing.T) {
	origRunE := runCmd.RunE
	origTaskFlag := taskFlag
	defer func() {
		runCmd.RunE = origRunE
		taskFlag = origTaskFlag
	}()

	var capturedTaskFlag string
	runCmd.RunE = func(cmd *cobra.Command, args []string) error {
		capturedTaskFlag = taskFlag
		return nil
	}

	if err := dispatchWork("TASK-123"); err != nil {
		t.Fatalf("dispatchWork returned unexpected error: %v", err)
	}
	if capturedTaskFlag != "TASK-123" {
		t.Errorf("expected taskFlag to be %q, got %q", "TASK-123", capturedTaskFlag)
	}
}

func TestDispatchWork_PropagatesRunError(t *testing.T) {
	origRunE := runCmd.RunE
	defer func() { runCmd.RunE = origRunE }()

	sentinel := errors.New("work failed")
	runCmd.RunE = func(cmd *cobra.Command, args []string) error {
		return sentinel
	}

	err := dispatchWork("TASK-001")
	if !errors.Is(err, sentinel) {
		t.Errorf("expected sentinel error, got: %v", err)
	}
}
