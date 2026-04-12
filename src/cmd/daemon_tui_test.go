package cmd

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/leberkas-org/maggus/internal/agent"
	"github.com/leberkas-org/maggus/internal/runlog"
)

func TestNullTUIModel_TokenUsageTracking(t *testing.T) {
	var captured TaskUsage
	dm := nullTUIModel{}
	dm.SetOnTaskUsage(func(tu TaskUsage) {
		captured = tu
	})

	// Start an iteration.
	updated, _ := dm.Update(IterationStartMsg{
		TaskID:    "TASK-001-004",
		TaskTitle: "Fix daemon-mode token usage tracking",
		ItemID:    "uuid-001",
		ItemShort: "feature_001",
		ItemTitle: "Feature 001",
	})
	dm = updated.(nullTUIModel)

	// Send usage messages.
	updated, _ = dm.Update(agent.UsageMsg{
		InputTokens:              1000,
		OutputTokens:             500,
		CacheCreationInputTokens: 2000,
		CacheReadInputTokens:     800,
		CostUSD:                  0.05,
	})
	dm = updated.(nullTUIModel)

	updated, _ = dm.Update(agent.UsageMsg{
		InputTokens:  200,
		OutputTokens: 100,
		CostUSD:      0.01,
	})
	dm = updated.(nullTUIModel)

	// Send model usage.
	updated, _ = dm.Update(agent.ModelUsageMsg{
		Models: map[string]agent.ModelTokens{
			"claude-sonnet": {InputTokens: 1200, OutputTokens: 600, CostUSD: 0.06},
		},
	})
	dm = updated.(nullTUIModel)

	// Trigger flush via QuitMsg.
	updated, _ = dm.Update(QuitMsg{})
	_ = updated

	if captured.TaskShort != "TASK-001-004" {
		t.Errorf("TaskShort = %q, want %q", captured.TaskShort, "TASK-001-004")
	}
	if captured.ItemTitle != "Feature 001" {
		t.Errorf("ItemTitle = %q, want %q", captured.ItemTitle, "Feature 001")
	}
	if captured.InputTokens != 1200 {
		t.Errorf("InputTokens = %d, want 1200", captured.InputTokens)
	}
	if captured.OutputTokens != 600 {
		t.Errorf("OutputTokens = %d, want 600", captured.OutputTokens)
	}
	if captured.CacheCreationInputTokens != 2000 {
		t.Errorf("CacheCreationInputTokens = %d, want 2000", captured.CacheCreationInputTokens)
	}
	if captured.CacheReadInputTokens != 800 {
		t.Errorf("CacheReadInputTokens = %d, want 800", captured.CacheReadInputTokens)
	}
	if captured.CostUSD < 0.059 || captured.CostUSD > 0.061 {
		t.Errorf("CostUSD = %f, want ~0.06", captured.CostUSD)
	}
	if len(captured.ModelUsage) != 1 {
		t.Errorf("ModelUsage length = %d, want 1", len(captured.ModelUsage))
	}
	if captured.ItemShort != "feature_001" {
		t.Errorf("ItemShort = %q, want %q", captured.ItemShort, "feature_001")
	}
}

func TestNullTUIModel_FlushOnIterationStart(t *testing.T) {
	var usages []TaskUsage
	dm := nullTUIModel{}
	dm.SetOnTaskUsage(func(tu TaskUsage) {
		usages = append(usages, tu)
	})

	// First task.
	updated, _ := dm.Update(IterationStartMsg{
		TaskID:    "TASK-001",
		TaskTitle: "First task",
	})
	dm = updated.(nullTUIModel)

	updated, _ = dm.Update(agent.UsageMsg{InputTokens: 100, OutputTokens: 50})
	dm = updated.(nullTUIModel)

	// Second task — should flush first task's usage.
	updated, _ = dm.Update(IterationStartMsg{
		TaskID:    "TASK-002",
		TaskTitle: "Second task",
	})
	dm = updated.(nullTUIModel)

	if len(usages) != 1 {
		t.Fatalf("expected 1 usage after second IterationStartMsg, got %d", len(usages))
	}
	if usages[0].TaskShort != "TASK-001" {
		t.Errorf("flushed TaskShort = %q, want %q", usages[0].TaskShort, "TASK-001")
	}
	if usages[0].InputTokens != 100 {
		t.Errorf("flushed InputTokens = %d, want 100", usages[0].InputTokens)
	}
}

func TestNullTUIModel_NoFlushWhenNoTokens(t *testing.T) {
	callCount := 0
	dm := nullTUIModel{}
	dm.SetOnTaskUsage(func(tu TaskUsage) {
		callCount++
	})

	// Start iteration with no usage data.
	updated, _ := dm.Update(IterationStartMsg{TaskID: "TASK-001"})
	dm = updated.(nullTUIModel)

	// Flush via quit — no tokens accumulated, should not call callback.
	updated, _ = dm.Update(QuitMsg{})
	_ = updated

	if callCount != 0 {
		t.Errorf("onTaskUsage called %d times, want 0 (no tokens)", callCount)
	}
}

func TestNullTUIModel_StartTimeSet(t *testing.T) {
	var captured TaskUsage
	dm := nullTUIModel{}
	dm.SetOnTaskUsage(func(tu TaskUsage) {
		captured = tu
	})

	before := time.Now()
	updated, _ := dm.Update(IterationStartMsg{TaskID: "TASK-001"})
	dm = updated.(nullTUIModel)

	updated, _ = dm.Update(agent.UsageMsg{InputTokens: 10, OutputTokens: 5})
	dm = updated.(nullTUIModel)

	updated, _ = dm.Update(QuitMsg{})
	_ = updated

	if captured.StartTime.Before(before) {
		t.Error("StartTime is before the iteration start")
	}
	if captured.EndTime.Before(captured.StartTime) {
		t.Error("EndTime is before StartTime")
	}
}

func TestNullTUIModel_DispatchMode_WritesPerWorkerSnapshot(t *testing.T) {
	dir := t.TempDir()
	taskID := "TASK-045-001"

	dm := nullTUIModel{
		snapshotDir:     dir, // local dir (unused in dispatch mode)
		dispatchRepoDir: dir,
		dispatchTaskID:  taskID,
		runStartedAt:    time.Now(),
	}

	// Start iteration.
	updated, _ := dm.Update(IterationStartMsg{
		TaskID:    taskID,
		TaskTitle: "Dispatch test task",
	})
	dm = updated.(nullTUIModel)

	// Send a tool use event.
	updated, _ = dm.Update(agent.ToolMsg{
		Type:        "Read",
		Description: "Read: test.go",
		Timestamp:   time.Now(),
	})
	dm = updated.(nullTUIModel)

	// Verify per-worker snapshot was written (not main state.json).
	workerSnap, err := runlog.ReadWorkerSnapshot(dir, taskID)
	if err != nil {
		t.Fatalf("per-worker snapshot not found: %v", err)
	}
	if workerSnap.TaskID != taskID {
		t.Errorf("worker snapshot TaskID = %q, want %q", workerSnap.TaskID, taskID)
	}
	if len(workerSnap.ToolEntries) != 1 {
		t.Errorf("worker snapshot ToolEntries len = %d, want 1", len(workerSnap.ToolEntries))
	}

	// Verify main state.json was NOT written.
	mainSnap := filepath.Join(dir, ".maggus", "runs", "state.json")
	if _, err := os.Stat(mainSnap); err == nil {
		t.Error("main state.json should not have been written in dispatch mode")
	}
}

func TestNullTUIModel_DispatchMode_FinalizeWorkerDone(t *testing.T) {
	dir := t.TempDir()
	taskID := "TASK-045-002"

	// Write initial workers index.
	_ = runlog.WriteWorkersIndex(dir, []runlog.WorkerIndexEntry{
		{TaskID: taskID, Status: "working"},
	})

	dm := nullTUIModel{
		dispatchRepoDir: dir,
		dispatchTaskID:  taskID,
		runStartedAt:    time.Now(),
		commits:         []string{"feat: task completed"},
	}

	// Trigger QuitMsg which calls finalizeDispatchWorker.
	updated, _ := dm.Update(QuitMsg{})
	_ = updated

	// Verify worker status was updated to "done".
	workers := runlog.ReadWorkersIndex(dir)
	if len(workers) != 1 {
		t.Fatalf("expected 1 worker, got %d", len(workers))
	}
	if workers[0].Status != "done" {
		t.Errorf("worker status = %q, want %q", workers[0].Status, "done")
	}
}

func TestNullTUIModel_DispatchMode_FinalizeWorkerFailed(t *testing.T) {
	dir := t.TempDir()
	taskID := "TASK-045-003"

	// Write initial workers index.
	_ = runlog.WriteWorkersIndex(dir, []runlog.WorkerIndexEntry{
		{TaskID: taskID, Status: "working"},
	})

	dm := nullTUIModel{
		dispatchRepoDir: dir,
		dispatchTaskID:  taskID,
		runStartedAt:    time.Now(),
		// No commits — worker should be marked as failed.
	}

	updated, _ := dm.Update(QuitMsg{})
	_ = updated

	workers := runlog.ReadWorkersIndex(dir)
	if len(workers) != 1 {
		t.Fatalf("expected 1 worker, got %d", len(workers))
	}
	if workers[0].Status != "failed" {
		t.Errorf("worker status = %q, want %q (no commits = failed)", workers[0].Status, "failed")
	}
}

func TestNullTUIModel_DispatchMode_MergeBackSuccess(t *testing.T) {
	dir := t.TempDir()
	taskID := "TASK-045-010"

	_ = runlog.WriteWorkersIndex(dir, []runlog.WorkerIndexEntry{
		{TaskID: taskID, Status: "working"},
	})

	// Stub merge-back to succeed.
	origFn := mergeDispatchedTaskBackFn
	defer func() { mergeDispatchedTaskBackFn = origFn }()
	var mergeCalled bool
	var mergeRepoDir, mergeBaseBranch, mergeTaskID string
	mergeDispatchedTaskBackFn = func(repoDir, baseBranch, tID string) error {
		mergeCalled = true
		mergeRepoDir = repoDir
		mergeBaseBranch = baseBranch
		mergeTaskID = tID
		return nil
	}

	dm := nullTUIModel{
		dispatchRepoDir:    dir,
		dispatchTaskID:     taskID,
		dispatchBaseBranch: "feature/maggus-045",
		runStartedAt:       time.Now(),
		commits:            []string{"feat: done"},
	}

	updated, _ := dm.Update(QuitMsg{})
	_ = updated

	if !mergeCalled {
		t.Fatal("merge-back function was not called")
	}
	if mergeRepoDir != dir {
		t.Errorf("merge repoDir = %q, want %q", mergeRepoDir, dir)
	}
	if mergeBaseBranch != "feature/maggus-045" {
		t.Errorf("merge baseBranch = %q, want %q", mergeBaseBranch, "feature/maggus-045")
	}
	if mergeTaskID != taskID {
		t.Errorf("merge taskID = %q, want %q", mergeTaskID, taskID)
	}

	workers := runlog.ReadWorkersIndex(dir)
	if len(workers) != 1 {
		t.Fatalf("expected 1 worker, got %d", len(workers))
	}
	if workers[0].Status != "done" {
		t.Errorf("worker status = %q, want %q", workers[0].Status, "done")
	}
}

func TestNullTUIModel_DispatchMode_MergeBackConflict(t *testing.T) {
	dir := t.TempDir()
	taskID := "TASK-045-011"

	_ = runlog.WriteWorkersIndex(dir, []runlog.WorkerIndexEntry{
		{TaskID: taskID, Status: "working"},
	})

	// Stub merge-back to return a conflict error.
	origFn := mergeDispatchedTaskBackFn
	defer func() { mergeDispatchedTaskBackFn = origFn }()
	mergeDispatchedTaskBackFn = func(repoDir, baseBranch, tID string) error {
		return errors.New("merge conflict")
	}

	dm := nullTUIModel{
		dispatchRepoDir:    dir,
		dispatchTaskID:     taskID,
		dispatchBaseBranch: "feature/maggus-045",
		runStartedAt:       time.Now(),
		commits:            []string{"feat: done"},
	}

	updated, _ := dm.Update(QuitMsg{})
	_ = updated

	// Merge failed — worker should be marked as failed.
	workers := runlog.ReadWorkersIndex(dir)
	if len(workers) != 1 {
		t.Fatalf("expected 1 worker, got %d", len(workers))
	}
	if workers[0].Status != "failed" {
		t.Errorf("worker status = %q, want %q (merge conflict = failed)", workers[0].Status, "failed")
	}

	// Verify the snapshot status indicates manual intervention needed.
	snap, err := runlog.ReadWorkerSnapshot(dir, taskID)
	if err != nil {
		t.Fatalf("per-worker snapshot not found: %v", err)
	}
	if !strings.Contains(snap.Status, "manual resolution") {
		t.Errorf("snapshot status should mention manual resolution, got %q", snap.Status)
	}
}

func TestNullTUIModel_DispatchMode_NoBaseBranch_SkipsMerge(t *testing.T) {
	dir := t.TempDir()
	taskID := "TASK-045-012"

	_ = runlog.WriteWorkersIndex(dir, []runlog.WorkerIndexEntry{
		{TaskID: taskID, Status: "working"},
	})

	// Stub merge-back to track if it's called.
	origFn := mergeDispatchedTaskBackFn
	defer func() { mergeDispatchedTaskBackFn = origFn }()
	var mergeCalled bool
	mergeDispatchedTaskBackFn = func(repoDir, baseBranch, tID string) error {
		mergeCalled = true
		// mergeDispatchedTaskBackImpl returns nil for empty baseBranch.
		return mergeDispatchedTaskBackImpl(repoDir, baseBranch, tID)
	}

	dm := nullTUIModel{
		dispatchRepoDir: dir,
		dispatchTaskID:  taskID,
		// No dispatchBaseBranch — merge should be skipped gracefully.
		runStartedAt: time.Now(),
		commits:      []string{"feat: done"},
	}

	updated, _ := dm.Update(QuitMsg{})
	_ = updated

	if !mergeCalled {
		t.Fatal("merge-back function should have been called")
	}

	// Should still be marked as done (merge returned nil).
	workers := runlog.ReadWorkersIndex(dir)
	if len(workers) != 1 {
		t.Fatalf("expected 1 worker, got %d", len(workers))
	}
	if workers[0].Status != "done" {
		t.Errorf("worker status = %q, want %q (no base branch = graceful skip)", workers[0].Status, "done")
	}
}

func TestNullTUIModel_DispatchMode_NoCommits_SkipsMerge(t *testing.T) {
	dir := t.TempDir()
	taskID := "TASK-045-013"

	_ = runlog.WriteWorkersIndex(dir, []runlog.WorkerIndexEntry{
		{TaskID: taskID, Status: "working"},
	})

	// Stub merge-back — should NOT be called when there are no commits.
	origFn := mergeDispatchedTaskBackFn
	defer func() { mergeDispatchedTaskBackFn = origFn }()
	var mergeCalled bool
	mergeDispatchedTaskBackFn = func(repoDir, baseBranch, tID string) error {
		mergeCalled = true
		return nil
	}

	dm := nullTUIModel{
		dispatchRepoDir:    dir,
		dispatchTaskID:     taskID,
		dispatchBaseBranch: "feature/maggus-045",
		runStartedAt:       time.Now(),
		// No commits — task failed before producing output.
	}

	updated, _ := dm.Update(QuitMsg{})
	_ = updated

	if mergeCalled {
		t.Error("merge-back should NOT be called when there are no commits")
	}

	workers := runlog.ReadWorkersIndex(dir)
	if len(workers) != 1 {
		t.Fatalf("expected 1 worker, got %d", len(workers))
	}
	if workers[0].Status != "failed" {
		t.Errorf("worker status = %q, want %q (no commits = failed)", workers[0].Status, "failed")
	}
}

// BUG-039-003: tests for task_id snapshot correctness.

// TestNullTUIModel_IterationStartMsg_SetsTaskIDBeforeSnapshot verifies that
// the snapshot written by IterationStartMsg always carries the new task ID
// (criteria 1 & 2: snap.TaskID is set from m.taskID, and m.taskID is set
// before the first writeSnapshot call in the IterationStartMsg handler).
func TestNullTUIModel_IterationStartMsg_SetsTaskIDBeforeSnapshot(t *testing.T) {
	dir := t.TempDir()
	dm := nullTUIModel{
		snapshotDir:   dir,
		runStartedAt:  time.Now(),
	}

	updated, _ := dm.Update(IterationStartMsg{
		TaskID:    "TASK-039-003",
		TaskTitle: "Ensure taskID in snapshot",
	})
	dm = updated.(nullTUIModel)
	_ = dm

	snap, err := runlog.ReadSnapshot(dir)
	if err != nil {
		t.Fatalf("snapshot not found after IterationStartMsg: %v", err)
	}
	if snap.TaskID == "" {
		t.Error("snapshot TaskID is empty after IterationStartMsg — m.taskID must be set before writeSnapshot()")
	}
	if snap.TaskID != "TASK-039-003" {
		t.Errorf("snapshot TaskID = %q, want %q", snap.TaskID, "TASK-039-003")
	}
	if snap.Status == "Idle" {
		t.Error("snapshot Status must not be Idle after IterationStartMsg sets a valid taskID")
	}
}

// TestNullTUIModel_WriteSnapshot_EmptyTaskID_StatusIsIdle verifies criterion 3:
// if writeSnapshot() is called before any IterationStartMsg has set m.taskID,
// the snapshot must carry Status="Idle" rather than silently writing task_id:"".
func TestNullTUIModel_WriteSnapshot_EmptyTaskID_StatusIsIdle(t *testing.T) {
	dir := t.TempDir()
	dm := nullTUIModel{
		snapshotDir:   dir,
		runStartedAt:  time.Now(),
		status:        "Starting", // status set but no taskID
		// taskID intentionally left empty
	}

	dm.writeSnapshot()

	snap, err := runlog.ReadSnapshot(dir)
	if err != nil {
		t.Fatalf("snapshot not found: %v", err)
	}
	if snap.TaskID != "" {
		t.Errorf("snapshot TaskID = %q, want empty string", snap.TaskID)
	}
	if snap.Status != "Idle" {
		t.Errorf("snapshot Status = %q, want %q — empty taskID must produce Idle status, not silently empty", snap.Status, "Idle")
	}
}

func TestMergeDispatchedTaskBackImpl_EmptyBaseBranch(t *testing.T) {
	// Empty base branch should be a no-op (returns nil).
	if err := mergeDispatchedTaskBackImpl("/tmp/test", "", "TASK-045-001"); err != nil {
		t.Errorf("expected nil for empty baseBranch, got: %v", err)
	}
}

func TestMergeDispatchedTaskBackImpl_EmptyRepoDir(t *testing.T) {
	// Empty repoDir should be a no-op (returns nil).
	if err := mergeDispatchedTaskBackImpl("", "feature/maggus-045", "TASK-045-001"); err != nil {
		t.Errorf("expected nil for empty repoDir, got: %v", err)
	}
}

func TestNullTUIModel_SnapshotContainsTimestamps(t *testing.T) {
	dir := t.TempDir()
	runStart := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)

	dm := nullTUIModel{
		snapshotDir:  dir,
		runStartedAt: runStart,
	}

	// Start an iteration — sets startTime (task start).
	updated, _ := dm.Update(IterationStartMsg{
		TaskID:    "TASK-006-001",
		TaskTitle: "Add timestamps",
	})
	dm = updated.(nullTUIModel)

	// Read the snapshot written by IterationStartMsg.
	target := filepath.Join(dir, ".maggus", "runs", "state.json")
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("state.json not found: %v", err)
	}

	var snap runlog.StateSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if snap.RunStartedAt != "2026-01-15T10:00:00Z" {
		t.Errorf("RunStartedAt = %q, want %q", snap.RunStartedAt, "2026-01-15T10:00:00Z")
	}
	if snap.TaskStartedAt == "" {
		t.Error("TaskStartedAt should not be empty")
	}

	// TaskStartedAt should be parseable as RFC3339.
	taskTime, err := time.Parse(time.RFC3339, snap.TaskStartedAt)
	if err != nil {
		t.Fatalf("TaskStartedAt is not valid RFC3339: %v", err)
	}
	// It should be recent (set by IterationStartMsg handler).
	if time.Since(taskTime) > 5*time.Second {
		t.Error("TaskStartedAt seems too old")
	}
}
