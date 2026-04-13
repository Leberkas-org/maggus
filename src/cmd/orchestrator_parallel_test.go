package cmd

import (
	"testing"

	"github.com/leberkas-org/maggus/internal/parser"
)

func TestOrchestratorClassifyWorkable(t *testing.T) {
	o := &Orchestrator{}
	o.completedIDs = map[string]bool{"TASK-001": true}
	o.failedIDs = map[string]bool{}

	tasks := []parser.Task{
		{ID: "TASK-001", Criteria: []parser.Criterion{{Checked: true}}},
		{ID: "TASK-002", Parallel: true, Predecessors: []string{"TASK-001"}, Criteria: []parser.Criterion{{Checked: false}}},
		{ID: "TASK-003", Parallel: false, Predecessors: []string{"TASK-001"}, Criteria: []parser.Criterion{{Checked: false}}},
		{ID: "TASK-004", Parallel: true, Predecessors: []string{"TASK-002"}, Criteria: []parser.Criterion{{Checked: false}}},
		{ID: "TASK-005", Parallel: true, Predecessors: nil, Criteria: []parser.Criterion{{Text: "BLOCKED: something", Blocked: true}}},
	}

	par, seq := o.classifyWorkable(tasks)

	if len(par) != 1 || par[0].ID != "TASK-002" {
		t.Errorf("parallel workable = %v, want [TASK-002]", taskIDs(par))
	}
	if len(seq) != 1 || seq[0].ID != "TASK-003" {
		t.Errorf("sequential workable = %v, want [TASK-003]", taskIDs(seq))
	}
}

func TestOrchestratorClassifyWorkable_NoPredecessors(t *testing.T) {
	o := &Orchestrator{}
	o.completedIDs = map[string]bool{}
	o.failedIDs = map[string]bool{}

	tasks := []parser.Task{
		{ID: "TASK-001", Parallel: true, Criteria: []parser.Criterion{{Checked: false}}},
		{ID: "TASK-002", Parallel: true, Criteria: []parser.Criterion{{Checked: false}}},
	}

	par, seq := o.classifyWorkable(tasks)
	if len(par) != 2 {
		t.Errorf("parallel workable count = %d, want 2", len(par))
	}
	if len(seq) != 0 {
		t.Errorf("sequential workable count = %d, want 0", len(seq))
	}
}

func TestOrchestratorClassifyWorkable_SkipsFailedTasks(t *testing.T) {
	o := &Orchestrator{}
	o.completedIDs = map[string]bool{}
	o.failedIDs = map[string]bool{"TASK-001": true}

	tasks := []parser.Task{
		{ID: "TASK-001", Parallel: true, Criteria: []parser.Criterion{{Checked: false}}},
		{ID: "TASK-002", Parallel: true, Criteria: []parser.Criterion{{Checked: false}}},
	}

	par, seq := o.classifyWorkable(tasks)
	if len(par) != 1 || par[0].ID != "TASK-002" {
		t.Errorf("parallel workable = %v, want [TASK-002]", taskIDs(par))
	}
	if len(seq) != 0 {
		t.Errorf("sequential workable count = %d, want 0", len(seq))
	}
}

func TestOrchestratorClassifyWorkable_MixedBatch(t *testing.T) {
	// Verify that when both parallel and sequential tasks are eligible,
	// classifyWorkable places them in the correct lists.
	o := &Orchestrator{}
	o.completedIDs = map[string]bool{}
	o.failedIDs = map[string]bool{}

	tasks := []parser.Task{
		{ID: "TASK-001", Parallel: true, Criteria: []parser.Criterion{{Checked: false}}},
		{ID: "TASK-002", Parallel: false, Criteria: []parser.Criterion{{Checked: false}}},
		{ID: "TASK-003", Parallel: true, Criteria: []parser.Criterion{{Checked: false}}},
	}

	par, seq := o.classifyWorkable(tasks)
	if len(par) != 2 {
		t.Errorf("parallel workable count = %d, want 2", len(par))
	}
	if len(seq) != 1 || seq[0].ID != "TASK-002" {
		t.Errorf("sequential workable = %v, want [TASK-002]", taskIDs(seq))
	}
}

func TestOrchestratorClassifyWorkable_BlockedPredecessorSatisfied(t *testing.T) {
	// Task 1 is blocked. Task 2 has Task 1 as predecessor.
	// A blocked predecessor must be treated as satisfied so Task 2 can run.
	o := &Orchestrator{}
	o.completedIDs = map[string]bool{}
	o.failedIDs = map[string]bool{}
	o.skippedOrBlockedIDs = map[string]bool{"TASK-001": true}

	tasks := []parser.Task{
		{ID: "TASK-001", Criteria: []parser.Criterion{{Text: "BLOCKED: something", Blocked: true}}},
		{ID: "TASK-002", Parallel: false, Predecessors: []string{"TASK-001"}, Criteria: []parser.Criterion{{Checked: false}}},
	}

	par, seq := o.classifyWorkable(tasks)
	if len(par) != 0 {
		t.Errorf("parallel workable = %v, want []", taskIDs(par))
	}
	if len(seq) != 1 || seq[0].ID != "TASK-002" {
		t.Errorf("sequential workable = %v, want [TASK-002]", taskIDs(seq))
	}
}

func TestOrchestratorClassifyWorkable_SkippedPredecessorSatisfied(t *testing.T) {
	// Task 1 is skipped. Task 2 has Task 1 as predecessor.
	// A skipped predecessor must be treated as satisfied so Task 2 can run.
	o := &Orchestrator{}
	o.completedIDs = map[string]bool{}
	o.failedIDs = map[string]bool{}
	o.skippedOrBlockedIDs = map[string]bool{"TASK-001": true}

	tasks := []parser.Task{
		{ID: "TASK-001", Criteria: []parser.Criterion{{Skipped: true}}},
		{ID: "TASK-002", Parallel: false, Predecessors: []string{"TASK-001"}, Criteria: []parser.Criterion{{Checked: false}}},
	}

	par, seq := o.classifyWorkable(tasks)
	if len(par) != 0 {
		t.Errorf("parallel workable = %v, want []", taskIDs(par))
	}
	if len(seq) != 1 || seq[0].ID != "TASK-002" {
		t.Errorf("sequential workable = %v, want [TASK-002]", taskIDs(seq))
	}
}

func TestOrchestratorClassifyWorkable_IncompletePredecessorStillHeld(t *testing.T) {
	// Task 1 is incomplete (not blocked, not skipped, not complete).
	// Task 2 must still be held back — only truly unsatisfied predecessors block.
	o := &Orchestrator{}
	o.completedIDs = map[string]bool{}
	o.failedIDs = map[string]bool{}
	o.skippedOrBlockedIDs = map[string]bool{}

	tasks := []parser.Task{
		{ID: "TASK-001", Parallel: false, Criteria: []parser.Criterion{{Checked: false}}},
		{ID: "TASK-002", Parallel: false, Predecessors: []string{"TASK-001"}, Criteria: []parser.Criterion{{Checked: false}}},
	}

	par, seq := o.classifyWorkable(tasks)
	if len(par) != 0 {
		t.Errorf("parallel workable = %v, want []", taskIDs(par))
	}
	if len(seq) != 1 || seq[0].ID != "TASK-001" {
		t.Errorf("sequential workable = %v, want [TASK-001]", taskIDs(seq))
	}
}
