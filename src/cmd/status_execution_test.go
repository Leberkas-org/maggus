package cmd

import (
	"fmt"
	"testing"

	"github.com/leberkas-org/maggus/internal/parser"
)

// makeTask is a helper that creates a parser.Task with the given ID, parallel flag,
// and predecessors (variadic). Criteria are omitted; the task is considered pending.
func makeTask(id string, parallel bool, predecessors ...string) parser.Task {
	return parser.Task{
		ID:           id,
		Title:        id + " title",
		Parallel:     parallel,
		Predecessors: predecessors,
	}
}

// makeCompleteTask creates a task with all criteria checked (complete).
func makeCompleteTask(id string, parallel bool, predecessors ...string) parser.Task {
	t := makeTask(id, parallel, predecessors...)
	t.Criteria = []parser.Criterion{{Text: "done", Checked: true}}
	return t
}

func TestBuildExecutionPlan_NoTasks(t *testing.T) {
	steps := buildExecutionPlan(nil)
	if len(steps) != 0 {
		t.Errorf("expected 0 steps for nil input, got %d", len(steps))
	}

	steps = buildExecutionPlan([]parser.Task{})
	if len(steps) != 0 {
		t.Errorf("expected 0 steps for empty input, got %d", len(steps))
	}
}

func TestBuildExecutionPlan_SingleTask(t *testing.T) {
	tasks := []parser.Task{makeTask("T1", false)}
	steps := buildExecutionPlan(tasks)

	if len(steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(steps))
	}
	s := steps[0]
	if s.StepNumber != 1 {
		t.Errorf("expected step 1, got %d", s.StepNumber)
	}
	if len(s.TaskIDs) != 1 || s.TaskIDs[0] != "T1" {
		t.Errorf("expected TaskIDs=[T1], got %v", s.TaskIDs)
	}
	if s.Parallel {
		t.Error("expected Parallel=false for a single sequential task")
	}
	if s.Unresolved {
		t.Error("expected Unresolved=false")
	}
}

func TestBuildExecutionPlan_AllParallel(t *testing.T) {
	// All tasks parallel, no predecessors — should all land in one step.
	tasks := []parser.Task{
		makeTask("T1", true),
		makeTask("T2", true),
		makeTask("T3", true),
	}
	steps := buildExecutionPlan(tasks)

	if len(steps) != 1 {
		t.Fatalf("expected 1 step for all-parallel tasks, got %d", len(steps))
	}
	s := steps[0]
	if s.StepNumber != 1 {
		t.Errorf("expected step 1, got %d", s.StepNumber)
	}
	if !s.Parallel {
		t.Error("expected Parallel=true")
	}
	if len(s.TaskIDs) != 3 {
		t.Errorf("expected 3 task IDs in step, got %d: %v", len(s.TaskIDs), s.TaskIDs)
	}
}

func TestBuildExecutionPlan_AllSequential(t *testing.T) {
	// All tasks sequential, no predecessors — each gets its own step.
	tasks := []parser.Task{
		makeTask("T1", false),
		makeTask("T2", false),
		makeTask("T3", false),
	}
	steps := buildExecutionPlan(tasks)

	if len(steps) != 3 {
		t.Fatalf("expected 3 steps for all-sequential tasks, got %d", len(steps))
	}
	for i, s := range steps {
		if s.StepNumber != i+1 {
			t.Errorf("step[%d]: expected StepNumber=%d, got %d", i, i+1, s.StepNumber)
		}
		if s.Parallel {
			t.Errorf("step[%d]: expected Parallel=false", i)
		}
		if len(s.TaskIDs) != 1 {
			t.Errorf("step[%d]: expected 1 task ID, got %d: %v", i, len(s.TaskIDs), s.TaskIDs)
		}
	}
}

func TestBuildExecutionPlan_LinearChain(t *testing.T) {
	// T1 → T2 → T3 (all sequential)
	tasks := []parser.Task{
		makeTask("T1", false),
		makeTask("T2", false, "T1"),
		makeTask("T3", false, "T2"),
	}
	steps := buildExecutionPlan(tasks)

	if len(steps) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(steps))
	}
	wantIDs := []string{"T1", "T2", "T3"}
	for i, s := range steps {
		if s.StepNumber != i+1 {
			t.Errorf("step[%d]: expected StepNumber=%d, got %d", i, i+1, s.StepNumber)
		}
		if len(s.TaskIDs) != 1 || s.TaskIDs[0] != wantIDs[i] {
			t.Errorf("step[%d]: expected TaskIDs=[%s], got %v", i, wantIDs[i], s.TaskIDs)
		}
	}
}

func TestBuildExecutionPlan_DiamondDependency(t *testing.T) {
	// A (seq) → B (parallel), C (parallel) → D (seq)
	// Expected: Step 1 [A], Step 2 [B, C] (parallel), Step 3 [D]
	tasks := []parser.Task{
		makeTask("A", false),
		makeTask("B", true, "A"),
		makeTask("C", true, "A"),
		makeTask("D", false, "B", "C"),
	}
	steps := buildExecutionPlan(tasks)

	if len(steps) != 3 {
		t.Fatalf("expected 3 steps for diamond, got %d: %v", len(steps), stepsDebug(steps))
	}

	// Step 1: A (sequential)
	if steps[0].StepNumber != 1 || len(steps[0].TaskIDs) != 1 || steps[0].TaskIDs[0] != "A" {
		t.Errorf("step[0]: expected [A], got %v", steps[0])
	}
	if steps[0].Parallel {
		t.Error("step[0]: expected Parallel=false")
	}

	// Step 2: B and C (parallel)
	if steps[1].StepNumber != 2 || !steps[1].Parallel || len(steps[1].TaskIDs) != 2 {
		t.Errorf("step[1]: expected parallel [B,C], got %v", steps[1])
	}
	ids := map[string]bool{}
	for _, id := range steps[1].TaskIDs {
		ids[id] = true
	}
	if !ids["B"] || !ids["C"] {
		t.Errorf("step[1]: expected B and C, got %v", steps[1].TaskIDs)
	}

	// Step 3: D (sequential)
	if steps[2].StepNumber != 3 || len(steps[2].TaskIDs) != 1 || steps[2].TaskIDs[0] != "D" {
		t.Errorf("step[2]: expected [D], got %v", steps[2])
	}
}

func TestBuildExecutionPlan_UnresolvablePredecessors(t *testing.T) {
	// T1 depends on "GHOST" which doesn't exist — goes to unresolved group.
	// T2 has no predecessors — goes in step 1.
	tasks := []parser.Task{
		makeTask("T2", false),
		makeTask("T1", false, "GHOST"),
	}
	steps := buildExecutionPlan(tasks)

	if len(steps) != 2 {
		t.Fatalf("expected 2 steps (1 normal + 1 unresolved), got %d: %v", len(steps), stepsDebug(steps))
	}

	// First step: T2
	if len(steps[0].TaskIDs) != 1 || steps[0].TaskIDs[0] != "T2" {
		t.Errorf("step[0]: expected [T2], got %v", steps[0].TaskIDs)
	}
	if steps[0].Unresolved {
		t.Error("step[0]: should not be marked unresolved")
	}

	// Last step: unresolved
	last := steps[len(steps)-1]
	if !last.Unresolved {
		t.Error("last step: expected Unresolved=true")
	}
	if len(last.TaskIDs) != 1 || last.TaskIDs[0] != "T1" {
		t.Errorf("last step: expected [T1] unresolved, got %v", last.TaskIDs)
	}
}

func TestBuildExecutionPlan_AllUnresolved(t *testing.T) {
	tasks := []parser.Task{
		makeTask("T1", false, "GHOST"),
		makeTask("T2", false, "GHOST2"),
	}
	steps := buildExecutionPlan(tasks)

	if len(steps) != 1 {
		t.Fatalf("expected 1 unresolved step, got %d", len(steps))
	}
	if !steps[0].Unresolved {
		t.Error("expected Unresolved=true")
	}
	if len(steps[0].TaskIDs) != 2 {
		t.Errorf("expected 2 unresolved tasks, got %d", len(steps[0].TaskIDs))
	}
}

func TestBuildExecutionPlan_CompletedTasksIncluded(t *testing.T) {
	// Completed tasks should appear in the plan just like pending tasks.
	tasks := []parser.Task{
		makeCompleteTask("T1", false),
		makeTask("T2", false, "T1"),
	}
	steps := buildExecutionPlan(tasks)

	if len(steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(steps))
	}
	if steps[0].TaskIDs[0] != "T1" {
		t.Errorf("expected T1 in step 1, got %v", steps[0].TaskIDs)
	}
	if steps[1].TaskIDs[0] != "T2" {
		t.Errorf("expected T2 in step 2, got %v", steps[1].TaskIDs)
	}
}

func TestBuildExecutionPlan_MixedParallelAndSequential(t *testing.T) {
	// P1 (parallel), P2 (parallel), S1 (sequential) — all no predecessors.
	// Expected: Step 1 [P1, P2] (parallel), Step 2 [S1] (sequential).
	tasks := []parser.Task{
		makeTask("P1", true),
		makeTask("P2", true),
		makeTask("S1", false),
	}
	steps := buildExecutionPlan(tasks)

	if len(steps) != 2 {
		t.Fatalf("expected 2 steps, got %d: %v", len(steps), stepsDebug(steps))
	}

	// Step 1: parallel group
	if !steps[0].Parallel {
		t.Error("step[0]: expected Parallel=true")
	}
	if len(steps[0].TaskIDs) != 2 {
		t.Errorf("step[0]: expected 2 tasks, got %v", steps[0].TaskIDs)
	}

	// Step 2: sequential
	if steps[1].Parallel {
		t.Error("step[1]: expected Parallel=false")
	}
	if len(steps[1].TaskIDs) != 1 || steps[1].TaskIDs[0] != "S1" {
		t.Errorf("step[1]: expected [S1], got %v", steps[1].TaskIDs)
	}
}

func TestBuildExecutionPlan_StepNumbers(t *testing.T) {
	// Verify step numbers are contiguous and start at 1.
	tasks := []parser.Task{
		makeTask("T1", false),
		makeTask("T2", true),
		makeTask("T3", true),
		makeTask("T4", false, "T1"),
	}
	steps := buildExecutionPlan(tasks)

	for i, s := range steps {
		if s.StepNumber != i+1 {
			t.Errorf("steps[%d].StepNumber = %d, want %d", i, s.StepNumber, i+1)
		}
	}
}

// stepsDebug returns a human-readable summary of steps for test failure messages.
func stepsDebug(steps []executionStep) string {
	var s string
	for _, step := range steps {
		s += fmt.Sprintf("Step %d (parallel=%v, unresolved=%v): %v\n",
			step.StepNumber, step.Parallel, step.Unresolved, step.TaskIDs)
	}
	return s
}
