package runner

import (
	"testing"
	"time"

	"github.com/leberkas-org/maggus/internal/agent"
)

func TestAddUsageAccumulatesTotals(t *testing.T) {
	ts := tokenState{}

	ts.addUsage(agent.UsageMsg{InputTokens: 100, OutputTokens: 50})
	ts.addUsage(agent.UsageMsg{InputTokens: 200, OutputTokens: 150})

	if ts.iterInput != 300 {
		t.Errorf("iterInput = %d, want 300", ts.iterInput)
	}
	if ts.iterOutput != 200 {
		t.Errorf("iterOutput = %d, want 200", ts.iterOutput)
	}
	if ts.totalInput != 300 {
		t.Errorf("totalInput = %d, want 300", ts.totalInput)
	}
	if ts.totalOutput != 200 {
		t.Errorf("totalOutput = %d, want 200", ts.totalOutput)
	}
	if !ts.hasData {
		t.Error("hasData should be true after adding usage")
	}
}

func TestAddUsageZeroTokensDoNotSetHasData(t *testing.T) {
	ts := tokenState{}
	ts.addUsage(agent.UsageMsg{InputTokens: 0, OutputTokens: 0})

	if ts.hasData {
		t.Error("hasData should be false when only zero-token messages are added")
	}
}

func TestSaveAndResetCreatesTaskUsageAndResetsIterCounters(t *testing.T) {
	ts := tokenState{}
	ts.addUsage(agent.UsageMsg{InputTokens: 500, OutputTokens: 250})
	ts.addUsage(agent.UsageMsg{InputTokens: 100, OutputTokens: 50})

	start := time.Now().Add(-time.Minute)
	ts.saveAndReset("TASK-001", "Test Task", "plan.md", start)

	if len(ts.usages) != 1 {
		t.Fatalf("expected 1 TaskUsage record, got %d", len(ts.usages))
	}

	tu := ts.usages[0]
	if tu.TaskID != "TASK-001" {
		t.Errorf("TaskID = %q, want %q", tu.TaskID, "TASK-001")
	}
	if tu.TaskTitle != "Test Task" {
		t.Errorf("TaskTitle = %q, want %q", tu.TaskTitle, "Test Task")
	}
	if tu.PlanFile != "plan.md" {
		t.Errorf("PlanFile = %q, want %q", tu.PlanFile, "plan.md")
	}
	if tu.InputTokens != 600 {
		t.Errorf("InputTokens = %d, want 600", tu.InputTokens)
	}
	if tu.OutputTokens != 300 {
		t.Errorf("OutputTokens = %d, want 300", tu.OutputTokens)
	}
	if tu.StartTime != start {
		t.Errorf("StartTime mismatch")
	}

	// Iteration counters should be reset
	if ts.iterInput != 0 {
		t.Errorf("iterInput = %d after reset, want 0", ts.iterInput)
	}
	if ts.iterOutput != 0 {
		t.Errorf("iterOutput = %d after reset, want 0", ts.iterOutput)
	}

	// Totals should be preserved
	if ts.totalInput != 600 {
		t.Errorf("totalInput = %d after reset, want 600", ts.totalInput)
	}
	if ts.totalOutput != 300 {
		t.Errorf("totalOutput = %d after reset, want 300", ts.totalOutput)
	}
}

func TestSaveAndResetSkipsEmptyTaskID(t *testing.T) {
	ts := tokenState{}
	ts.addUsage(agent.UsageMsg{InputTokens: 100, OutputTokens: 50})
	ts.saveAndReset("", "Title", "plan.md", time.Now())

	if len(ts.usages) != 0 {
		t.Error("expected no TaskUsage when taskID is empty")
	}
}

func TestSaveAndResetSkipsZeroTokens(t *testing.T) {
	ts := tokenState{}
	ts.saveAndReset("TASK-001", "Title", "plan.md", time.Now())

	if len(ts.usages) != 0 {
		t.Error("expected no TaskUsage when both token counts are zero")
	}
}

func TestSaveAndResetCallsOnUsageCallback(t *testing.T) {
	var called TaskUsage
	ts := tokenState{
		onUsage: func(tu TaskUsage) { called = tu },
	}
	ts.addUsage(agent.UsageMsg{InputTokens: 10, OutputTokens: 5})
	ts.saveAndReset("TASK-X", "CB Test", "p.md", time.Now())

	if called.TaskID != "TASK-X" {
		t.Errorf("onUsage callback not called or wrong TaskID: %q", called.TaskID)
	}
}

// FormatTokens is already tested in runner_test.go (TestFormatTokens)
