package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/leberkas-org/maggus/internal/agent"
	"github.com/leberkas-org/maggus/internal/runlog"
	"github.com/leberkas-org/maggus/internal/runner"
)

func TestNullTUIModel_TokenUsageTracking(t *testing.T) {
	var captured runner.TaskUsage
	dm := nullTUIModel{}
	dm.SetOnTaskUsage(func(tu runner.TaskUsage) {
		captured = tu
	})

	// Start an iteration.
	updated, _ := dm.Update(runner.IterationStartMsg{
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
	updated, _ = dm.Update(runner.QuitMsg{})
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
	var usages []runner.TaskUsage
	dm := nullTUIModel{}
	dm.SetOnTaskUsage(func(tu runner.TaskUsage) {
		usages = append(usages, tu)
	})

	// First task.
	updated, _ := dm.Update(runner.IterationStartMsg{
		TaskID:    "TASK-001",
		TaskTitle: "First task",
	})
	dm = updated.(nullTUIModel)

	updated, _ = dm.Update(agent.UsageMsg{InputTokens: 100, OutputTokens: 50})
	dm = updated.(nullTUIModel)

	// Second task — should flush first task's usage.
	updated, _ = dm.Update(runner.IterationStartMsg{
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
	dm.SetOnTaskUsage(func(tu runner.TaskUsage) {
		callCount++
	})

	// Start iteration with no usage data.
	updated, _ := dm.Update(runner.IterationStartMsg{TaskID: "TASK-001"})
	dm = updated.(nullTUIModel)

	// Flush via quit — no tokens accumulated, should not call callback.
	updated, _ = dm.Update(runner.QuitMsg{})
	_ = updated

	if callCount != 0 {
		t.Errorf("onTaskUsage called %d times, want 0 (no tokens)", callCount)
	}
}

func TestNullTUIModel_StartTimeSet(t *testing.T) {
	var captured runner.TaskUsage
	dm := nullTUIModel{}
	dm.SetOnTaskUsage(func(tu runner.TaskUsage) {
		captured = tu
	})

	before := time.Now()
	updated, _ := dm.Update(runner.IterationStartMsg{TaskID: "TASK-001"})
	dm = updated.(nullTUIModel)

	updated, _ = dm.Update(agent.UsageMsg{InputTokens: 10, OutputTokens: 5})
	dm = updated.(nullTUIModel)

	updated, _ = dm.Update(runner.QuitMsg{})
	_ = updated

	if captured.StartTime.Before(before) {
		t.Error("StartTime is before the iteration start")
	}
	if captured.EndTime.Before(captured.StartTime) {
		t.Error("EndTime is before StartTime")
	}
}

func TestNullTUIModel_SnapshotContainsTimestamps(t *testing.T) {
	dir := t.TempDir()
	runID := "test-snap-ts"
	runStart := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)

	dm := nullTUIModel{
		snapshotDir:   dir,
		snapshotRunID: runID,
		runStartedAt:  runStart,
	}

	// Start an iteration — sets startTime (task start).
	updated, _ := dm.Update(runner.IterationStartMsg{
		TaskID:    "TASK-006-001",
		TaskTitle: "Add timestamps",
	})
	dm = updated.(nullTUIModel)

	// Read the snapshot written by IterationStartMsg.
	target := filepath.Join(dir, ".maggus", "runs", runID, "state.json")
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
