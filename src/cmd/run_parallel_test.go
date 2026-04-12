package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leberkas-org/maggus/internal/parser"
)

func TestClassifyWorkable(t *testing.T) {
	orch := &parallelOrchestrator{
		completedIDs: map[string]bool{
			"TASK-001": true,
		},
	}

	tasks := []parser.Task{
		{ID: "TASK-001", Criteria: []parser.Criterion{{Checked: true}}},
		{ID: "TASK-002", Parallel: true, Predecessors: []string{"TASK-001"}, Criteria: []parser.Criterion{{Checked: false}}},
		{ID: "TASK-003", Parallel: false, Predecessors: []string{"TASK-001"}, Criteria: []parser.Criterion{{Checked: false}}},
		{ID: "TASK-004", Parallel: true, Predecessors: []string{"TASK-002"}, Criteria: []parser.Criterion{{Checked: false}}},
		{ID: "TASK-005", Parallel: true, Predecessors: nil, Criteria: []parser.Criterion{{Text: "BLOCKED: something", Blocked: true}}},
	}

	par, seq := orch.classifyWorkable(tasks)

	if len(par) != 1 || par[0].ID != "TASK-002" {
		t.Errorf("parallel workable = %v, want [TASK-002]", taskIDs(par))
	}
	if len(seq) != 1 || seq[0].ID != "TASK-003" {
		t.Errorf("sequential workable = %v, want [TASK-003]", taskIDs(seq))
	}
}

func TestClassifyWorkable_NoPredecessors(t *testing.T) {
	orch := &parallelOrchestrator{
		completedIDs: map[string]bool{},
	}

	tasks := []parser.Task{
		{ID: "TASK-001", Parallel: true, Criteria: []parser.Criterion{{Checked: false}}},
		{ID: "TASK-002", Parallel: true, Criteria: []parser.Criterion{{Checked: false}}},
	}

	par, seq := orch.classifyWorkable(tasks)
	if len(par) != 2 {
		t.Errorf("parallel workable count = %d, want 2", len(par))
	}
	if len(seq) != 0 {
		t.Errorf("sequential workable count = %d, want 0", len(seq))
	}
}

func TestClassifyWorkable_SkipsFailedTasks(t *testing.T) {
	orch := &parallelOrchestrator{
		completedIDs: map[string]bool{},
		failedIDs:    map[string]bool{"TASK-001": true},
	}

	tasks := []parser.Task{
		{ID: "TASK-001", Parallel: true, Criteria: []parser.Criterion{{Checked: false}}},
		{ID: "TASK-002", Parallel: true, Criteria: []parser.Criterion{{Checked: false}}},
	}

	par, seq := orch.classifyWorkable(tasks)
	if len(par) != 1 || par[0].ID != "TASK-002" {
		t.Errorf("parallel workable = %v, want [TASK-002]", taskIDs(par))
	}
	if len(seq) != 0 {
		t.Errorf("sequential workable count = %d, want 0", len(seq))
	}
}

func TestCheckCriterionInFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "feature.md")
	content := "### TASK-001: Test\n- [ ] First criterion\n- [ ] Second criterion\n- [x] Third done\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := checkCriterionInFile(path, "First criterion"); err != nil {
		t.Fatalf("checkCriterionInFile: %v", err)
	}

	data, _ := os.ReadFile(path)
	got := string(data)
	if !strings.Contains(got, "- [x] First criterion") {
		t.Error("expected criterion to be checked")
	}
	if !strings.Contains(got, "- [ ] Second criterion") {
		t.Error("expected second criterion to remain unchecked")
	}
}


func taskIDs(tasks []parser.Task) []string {
	var result []string
	for _, t := range tasks {
		result = append(result, t.ID)
	}
	return result
}
