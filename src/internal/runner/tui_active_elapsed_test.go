package runner

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/leberkas-org/maggus/internal/agent"
)

// newTestModel creates a minimal TUIModel for testing active elapsed tracking.
func newTestModel() TUIModel {
	return TUIModel{
		status:           "Waiting...",
		stopFlag:         &atomic.Bool{},
		stopAtTaskIDFlag: &atomic.Value{},
		runStartTime:     time.Now(),
		startTime:        time.Now(),
		width:            120,
		height:           40,
		detailAutoScroll: true,
	}
}

func TestActiveElapsedIncreasesOnlyDuringTaskExecution(t *testing.T) {
	m := newTestModel()

	// Before any task starts, active elapsed should be zero
	if d := m.ActiveRunElapsed(); d != 0 {
		t.Errorf("expected 0 active elapsed before any task, got %v", d)
	}

	// Simulate starting a task
	m.handleIterationStart(IterationStartMsg{
		Current: 1, Total: 3, TaskID: "TASK-001", TaskTitle: "First",
	})

	// Give some token data so saveAndReset works on next iteration
	m.tokens.addUsage(agent.UsageMsg{InputTokens: 10, OutputTokens: 5})

	// Active elapsed should now be non-zero (task is running)
	time.Sleep(10 * time.Millisecond)
	if d := m.ActiveRunElapsed(); d < 10*time.Millisecond {
		t.Errorf("expected active elapsed >= 10ms during task, got %v", d)
	}

	// Complete the task
	m.status = "Done"
	if m.taskActive {
		m.activeRunDuration += time.Since(m.taskActiveStart)
		m.taskActive = false
	}

	saved := m.ActiveRunElapsed()

	// Simulate idle gap — active elapsed should NOT increase
	time.Sleep(20 * time.Millisecond)
	afterIdle := m.ActiveRunElapsed()
	if afterIdle != saved {
		t.Errorf("active elapsed changed during idle: was %v, now %v", saved, afterIdle)
	}
}

func TestActiveElapsedAccumulatesAcrossMultipleTasks(t *testing.T) {
	m := newTestModel()

	// Task 1
	m.handleIterationStart(IterationStartMsg{
		Current: 1, Total: 3, TaskID: "TASK-001", TaskTitle: "First",
	})
	m.tokens.addUsage(agent.UsageMsg{InputTokens: 10, OutputTokens: 5})
	// Manually set taskActiveStart to a known time for deterministic testing
	m.taskActiveStart = time.Now().Add(-100 * time.Millisecond)

	// Task 2 starts — this should accumulate task 1's time
	m.handleIterationStart(IterationStartMsg{
		Current: 2, Total: 3, TaskID: "TASK-002", TaskTitle: "Second",
	})
	m.tokens.addUsage(agent.UsageMsg{InputTokens: 10, OutputTokens: 5})

	// Set task 2's start to a known time
	m.taskActiveStart = time.Now().Add(-50 * time.Millisecond)

	// Complete task 2 via status
	updated, _ := m.Update(agent.StatusMsg{Status: "Done"})
	m = updated.(TUIModel)

	total := m.ActiveRunElapsed()
	// Should be at least 100ms (task1) + 50ms (task2) = 150ms
	if total < 140*time.Millisecond {
		t.Errorf("expected accumulated active elapsed >= 140ms, got %v", total)
	}
}

func TestActiveElapsedTaskCompletionViaStatus(t *testing.T) {
	m := newTestModel()

	m.handleIterationStart(IterationStartMsg{
		Current: 1, Total: 1, TaskID: "TASK-001", TaskTitle: "Test",
	})
	m.tokens.addUsage(agent.UsageMsg{InputTokens: 10, OutputTokens: 5})
	m.taskActiveStart = time.Now().Add(-50 * time.Millisecond)

	for _, status := range []string{"Done", "Failed", "Interrupted"} {
		m2 := m // copy
		updated, _ := m2.Update(agent.StatusMsg{Status: status})
		m3 := updated.(TUIModel)
		if m3.taskActive {
			t.Errorf("taskActive should be false after status %q", status)
		}
		if d := m3.ActiveRunElapsed(); d < 45*time.Millisecond {
			t.Errorf("expected active elapsed >= 45ms after %q, got %v", status, d)
		}
	}
}

func TestActiveElapsedNonCompletionStatusDoesNotStopTracking(t *testing.T) {
	m := newTestModel()

	m.handleIterationStart(IterationStartMsg{
		Current: 1, Total: 1, TaskID: "TASK-001", TaskTitle: "Test",
	})

	// A non-completion status should keep the task active
	updated, _ := m.Update(agent.StatusMsg{Status: "Running tool..."})
	m = updated.(TUIModel)

	if !m.taskActive {
		t.Error("taskActive should remain true for non-completion status")
	}
}

func TestActiveRunElapsedMethodIncludesInProgressTask(t *testing.T) {
	m := newTestModel()

	// Accumulate some completed time
	m.activeRunDuration = 100 * time.Millisecond

	// Start a new active task
	m.taskActive = true
	m.taskActiveStart = time.Now().Add(-50 * time.Millisecond)

	d := m.ActiveRunElapsed()
	// Should be at least 100ms + 50ms = 150ms
	if d < 140*time.Millisecond {
		t.Errorf("expected ActiveRunElapsed >= 140ms, got %v", d)
	}
}

func TestActiveRunElapsedMethodWithNoActiveTask(t *testing.T) {
	m := newTestModel()
	m.activeRunDuration = 200 * time.Millisecond
	m.taskActive = false

	d := m.ActiveRunElapsed()
	if d != 200*time.Millisecond {
		t.Errorf("expected ActiveRunElapsed = 200ms, got %v", d)
	}
}

func TestRunStartTimeAndRunElapsedRemainAvailable(t *testing.T) {
	m := newTestModel()
	before := time.Now()
	m.runStartTime = before

	time.Sleep(5 * time.Millisecond)
	wallElapsed := time.Since(m.runStartTime)

	if wallElapsed < 5*time.Millisecond {
		t.Errorf("expected wall-clock runElapsed >= 5ms, got %v", wallElapsed)
	}
}

func TestIterationStartWithNoPreviousTaskDoesNotPanic(t *testing.T) {
	m := newTestModel()

	// First iteration start with no previous task should work fine
	m.handleIterationStart(IterationStartMsg{
		Current: 1, Total: 1, TaskID: "TASK-001", TaskTitle: "First",
	})

	if !m.taskActive {
		t.Error("taskActive should be true after first iteration start")
	}
	if m.activeRunDuration != 0 {
		t.Errorf("activeRunDuration should be 0 after first start, got %v", m.activeRunDuration)
	}
}

func TestDoubleCompletionDoesNotDoubleCount(t *testing.T) {
	m := newTestModel()

	m.handleIterationStart(IterationStartMsg{
		Current: 1, Total: 1, TaskID: "TASK-001", TaskTitle: "Test",
	})
	m.taskActiveStart = time.Now().Add(-50 * time.Millisecond)

	// First completion
	updated, _ := m.Update(agent.StatusMsg{Status: "Done"})
	m = updated.(TUIModel)
	afterFirst := m.ActiveRunElapsed()

	// Second completion — should not add more time
	updated, _ = m.Update(agent.StatusMsg{Status: "Done"})
	m = updated.(TUIModel)
	afterSecond := m.ActiveRunElapsed()

	if afterSecond != afterFirst {
		t.Errorf("double completion changed elapsed: first=%v second=%v", afterFirst, afterSecond)
	}
}
