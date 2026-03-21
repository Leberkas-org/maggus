package runner

import (
	"testing"
	"time"

	"github.com/leberkas-org/maggus/internal/agent"
)

func TestAddUsageAccumulatesTotals(t *testing.T) {
	ts := tokenState{}

	ts.addUsage(agent.UsageMsg{InputTokens: 100, OutputTokens: 50, CacheCreationInputTokens: 1000, CacheReadInputTokens: 500, CostUSD: 0.05})
	ts.addUsage(agent.UsageMsg{InputTokens: 200, OutputTokens: 150, CacheCreationInputTokens: 2000, CacheReadInputTokens: 1000, CostUSD: 0.10})

	if ts.iterInput != 300 {
		t.Errorf("iterInput = %d, want 300", ts.iterInput)
	}
	if ts.iterOutput != 200 {
		t.Errorf("iterOutput = %d, want 200", ts.iterOutput)
	}
	if ts.iterCacheCreation != 3000 {
		t.Errorf("iterCacheCreation = %d, want 3000", ts.iterCacheCreation)
	}
	if ts.iterCacheRead != 1500 {
		t.Errorf("iterCacheRead = %d, want 1500", ts.iterCacheRead)
	}
	if diff := ts.iterCost - 0.15; diff < -1e-9 || diff > 1e-9 {
		t.Errorf("iterCost = %f, want 0.15", ts.iterCost)
	}
	if ts.totalInput != 300 {
		t.Errorf("totalInput = %d, want 300", ts.totalInput)
	}
	if ts.totalOutput != 200 {
		t.Errorf("totalOutput = %d, want 200", ts.totalOutput)
	}
	if ts.totalCacheCreation != 3000 {
		t.Errorf("totalCacheCreation = %d, want 3000", ts.totalCacheCreation)
	}
	if ts.totalCacheRead != 1500 {
		t.Errorf("totalCacheRead = %d, want 1500", ts.totalCacheRead)
	}
	if diff := ts.totalCost - 0.15; diff < -1e-9 || diff > 1e-9 {
		t.Errorf("totalCost = %f, want 0.15", ts.totalCost)
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

func TestAddUsageCacheOnlySetsHasData(t *testing.T) {
	ts := tokenState{}
	ts.addUsage(agent.UsageMsg{CacheCreationInputTokens: 500})

	if !ts.hasData {
		t.Error("hasData should be true when cache creation tokens are non-zero")
	}

	ts2 := tokenState{}
	ts2.addUsage(agent.UsageMsg{CacheReadInputTokens: 300})

	if !ts2.hasData {
		t.Error("hasData should be true when cache read tokens are non-zero")
	}
}

func TestAddUsageCostOnlySetsHasData(t *testing.T) {
	ts := tokenState{}
	ts.addUsage(agent.UsageMsg{CostUSD: 0.01})

	if !ts.hasData {
		t.Error("hasData should be true when cost is non-zero")
	}
}

func TestSaveAndResetCreatesTaskUsageAndResetsIterCounters(t *testing.T) {
	ts := tokenState{}
	ts.addUsage(agent.UsageMsg{InputTokens: 500, OutputTokens: 250, CacheCreationInputTokens: 5000, CacheReadInputTokens: 2000, CostUSD: 0.08})
	ts.addUsage(agent.UsageMsg{InputTokens: 100, OutputTokens: 50, CacheCreationInputTokens: 1000, CacheReadInputTokens: 500, CostUSD: 0.02})

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
	if tu.CacheCreationInputTokens != 6000 {
		t.Errorf("CacheCreationInputTokens = %d, want 6000", tu.CacheCreationInputTokens)
	}
	if tu.CacheReadInputTokens != 2500 {
		t.Errorf("CacheReadInputTokens = %d, want 2500", tu.CacheReadInputTokens)
	}
	if tu.CostUSD != 0.10 {
		t.Errorf("CostUSD = %f, want 0.10", tu.CostUSD)
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
	if ts.iterCacheCreation != 0 {
		t.Errorf("iterCacheCreation = %d after reset, want 0", ts.iterCacheCreation)
	}
	if ts.iterCacheRead != 0 {
		t.Errorf("iterCacheRead = %d after reset, want 0", ts.iterCacheRead)
	}
	if ts.iterCost != 0 {
		t.Errorf("iterCost = %f after reset, want 0", ts.iterCost)
	}

	// Totals should be preserved
	if ts.totalInput != 600 {
		t.Errorf("totalInput = %d after reset, want 600", ts.totalInput)
	}
	if ts.totalOutput != 300 {
		t.Errorf("totalOutput = %d after reset, want 300", ts.totalOutput)
	}
	if ts.totalCacheCreation != 6000 {
		t.Errorf("totalCacheCreation = %d after reset, want 6000", ts.totalCacheCreation)
	}
	if ts.totalCacheRead != 2500 {
		t.Errorf("totalCacheRead = %d after reset, want 2500", ts.totalCacheRead)
	}
	if ts.totalCost != 0.10 {
		t.Errorf("totalCost = %f after reset, want 0.10", ts.totalCost)
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
		t.Error("expected no TaskUsage when all token counts are zero")
	}
}

func TestSaveAndResetWithCacheOnlyTokens(t *testing.T) {
	ts := tokenState{}
	ts.addUsage(agent.UsageMsg{CacheCreationInputTokens: 5000, CacheReadInputTokens: 3000, CostUSD: 0.05})
	ts.saveAndReset("TASK-002", "Cache Only", "plan.md", time.Now())

	if len(ts.usages) != 1 {
		t.Fatalf("expected 1 TaskUsage record, got %d", len(ts.usages))
	}

	tu := ts.usages[0]
	if tu.CacheCreationInputTokens != 5000 {
		t.Errorf("CacheCreationInputTokens = %d, want 5000", tu.CacheCreationInputTokens)
	}
	if tu.CacheReadInputTokens != 3000 {
		t.Errorf("CacheReadInputTokens = %d, want 3000", tu.CacheReadInputTokens)
	}
	if tu.CostUSD != 0.05 {
		t.Errorf("CostUSD = %f, want 0.05", tu.CostUSD)
	}
}

func TestSaveAndResetCallsOnUsageCallback(t *testing.T) {
	var called TaskUsage
	ts := tokenState{
		onUsage: func(tu TaskUsage) { called = tu },
	}
	ts.addUsage(agent.UsageMsg{InputTokens: 10, OutputTokens: 5, CacheCreationInputTokens: 100, CostUSD: 0.01})
	ts.saveAndReset("TASK-X", "CB Test", "p.md", time.Now())

	if called.TaskID != "TASK-X" {
		t.Errorf("onUsage callback not called or wrong TaskID: %q", called.TaskID)
	}
	if called.CacheCreationInputTokens != 100 {
		t.Errorf("onUsage callback CacheCreationInputTokens = %d, want 100", called.CacheCreationInputTokens)
	}
	if called.CostUSD != 0.01 {
		t.Errorf("onUsage callback CostUSD = %f, want 0.01", called.CostUSD)
	}
}

func TestAddModelUsageAccumulatesPerModel(t *testing.T) {
	ts := tokenState{}

	ts.addModelUsage(agent.ModelUsageMsg{
		Models: map[string]agent.ModelTokens{
			"claude-sonnet": {InputTokens: 100, OutputTokens: 50, CacheCreationInputTokens: 500, CacheReadInputTokens: 200, CostUSD: 0.01},
		},
	})
	ts.addModelUsage(agent.ModelUsageMsg{
		Models: map[string]agent.ModelTokens{
			"claude-sonnet": {InputTokens: 200, OutputTokens: 100, CacheCreationInputTokens: 1000, CacheReadInputTokens: 300, CostUSD: 0.02},
			"claude-haiku":  {InputTokens: 50, OutputTokens: 25, CostUSD: 0.005},
		},
	})

	// Check iter accumulation for sonnet
	sonnet := ts.iterModelUsage["claude-sonnet"]
	if sonnet.InputTokens != 300 {
		t.Errorf("iter sonnet InputTokens = %d, want 300", sonnet.InputTokens)
	}
	if sonnet.OutputTokens != 150 {
		t.Errorf("iter sonnet OutputTokens = %d, want 150", sonnet.OutputTokens)
	}
	if sonnet.CacheCreationInputTokens != 1500 {
		t.Errorf("iter sonnet CacheCreationInputTokens = %d, want 1500", sonnet.CacheCreationInputTokens)
	}
	if sonnet.CacheReadInputTokens != 500 {
		t.Errorf("iter sonnet CacheReadInputTokens = %d, want 500", sonnet.CacheReadInputTokens)
	}
	if diff := sonnet.CostUSD - 0.03; diff < -1e-9 || diff > 1e-9 {
		t.Errorf("iter sonnet CostUSD = %f, want 0.03", sonnet.CostUSD)
	}

	// Check iter accumulation for haiku
	haiku := ts.iterModelUsage["claude-haiku"]
	if haiku.InputTokens != 50 {
		t.Errorf("iter haiku InputTokens = %d, want 50", haiku.InputTokens)
	}
	if haiku.OutputTokens != 25 {
		t.Errorf("iter haiku OutputTokens = %d, want 25", haiku.OutputTokens)
	}

	// Total should match iter (no resets yet)
	totalSonnet := ts.totalModelUsage["claude-sonnet"]
	if totalSonnet.InputTokens != 300 {
		t.Errorf("total sonnet InputTokens = %d, want 300", totalSonnet.InputTokens)
	}
	totalHaiku := ts.totalModelUsage["claude-haiku"]
	if totalHaiku.InputTokens != 50 {
		t.Errorf("total haiku InputTokens = %d, want 50", totalHaiku.InputTokens)
	}
}

func TestSaveAndResetStoresModelUsageAndResetsIter(t *testing.T) {
	ts := tokenState{}

	ts.addUsage(agent.UsageMsg{InputTokens: 100, OutputTokens: 50})
	ts.addModelUsage(agent.ModelUsageMsg{
		Models: map[string]agent.ModelTokens{
			"claude-sonnet": {InputTokens: 100, OutputTokens: 50, CostUSD: 0.01},
		},
	})

	start := time.Now().Add(-time.Minute)
	ts.saveAndReset("TASK-010", "Model Test", "plan.md", start)

	if len(ts.usages) != 1 {
		t.Fatalf("expected 1 TaskUsage, got %d", len(ts.usages))
	}

	tu := ts.usages[0]
	if tu.ModelUsage == nil {
		t.Fatal("ModelUsage should not be nil")
	}
	sonnet, ok := tu.ModelUsage["claude-sonnet"]
	if !ok {
		t.Fatal("ModelUsage should contain claude-sonnet")
	}
	if sonnet.InputTokens != 100 {
		t.Errorf("ModelUsage sonnet InputTokens = %d, want 100", sonnet.InputTokens)
	}
	if sonnet.CostUSD != 0.01 {
		t.Errorf("ModelUsage sonnet CostUSD = %f, want 0.01", sonnet.CostUSD)
	}

	// iterModelUsage should be reset
	if ts.iterModelUsage != nil {
		t.Errorf("iterModelUsage should be nil after reset, got %v", ts.iterModelUsage)
	}

	// totalModelUsage should be preserved
	totalSonnet := ts.totalModelUsage["claude-sonnet"]
	if totalSonnet.InputTokens != 100 {
		t.Errorf("total sonnet InputTokens = %d after reset, want 100", totalSonnet.InputTokens)
	}
}

func TestSaveAndResetPreservesTotalModelUsageAcrossIterations(t *testing.T) {
	ts := tokenState{}

	// First iteration
	ts.addUsage(agent.UsageMsg{InputTokens: 100, OutputTokens: 50})
	ts.addModelUsage(agent.ModelUsageMsg{
		Models: map[string]agent.ModelTokens{
			"claude-sonnet": {InputTokens: 100, OutputTokens: 50},
		},
	})
	ts.saveAndReset("TASK-011", "Iter 1", "plan.md", time.Now().Add(-time.Minute))

	// Second iteration
	ts.addUsage(agent.UsageMsg{InputTokens: 200, OutputTokens: 100})
	ts.addModelUsage(agent.ModelUsageMsg{
		Models: map[string]agent.ModelTokens{
			"claude-sonnet": {InputTokens: 150, OutputTokens: 75},
			"claude-haiku":  {InputTokens: 50, OutputTokens: 25},
		},
	})
	ts.saveAndReset("TASK-012", "Iter 2", "plan.md", time.Now().Add(-30*time.Second))

	// Verify totals accumulated across both iterations
	totalSonnet := ts.totalModelUsage["claude-sonnet"]
	if totalSonnet.InputTokens != 250 {
		t.Errorf("total sonnet InputTokens = %d, want 250", totalSonnet.InputTokens)
	}
	if totalSonnet.OutputTokens != 125 {
		t.Errorf("total sonnet OutputTokens = %d, want 125", totalSonnet.OutputTokens)
	}
	totalHaiku := ts.totalModelUsage["claude-haiku"]
	if totalHaiku.InputTokens != 50 {
		t.Errorf("total haiku InputTokens = %d, want 50", totalHaiku.InputTokens)
	}

	// Second TaskUsage should only have second iteration's model usage
	tu2 := ts.usages[1]
	if tu2.ModelUsage["claude-sonnet"].InputTokens != 150 {
		t.Errorf("iter2 sonnet InputTokens = %d, want 150", tu2.ModelUsage["claude-sonnet"].InputTokens)
	}
	if _, ok := tu2.ModelUsage["claude-haiku"]; !ok {
		t.Error("iter2 should contain claude-haiku")
	}
}

// FormatTokens is already tested in runner_test.go (TestFormatTokens)
