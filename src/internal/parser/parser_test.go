package parser

import (
	"os"
	"path/filepath"
	"testing"
)

const testPlan = `# Plan: Test

## Introduction

Some intro text.

## User Stories

### TASK-001: First task
**Description:** As a dev, I want to do thing one so that it works.

**Acceptance Criteria:**
- [ ] Criterion A
- [ ] Criterion B

### TASK-002: Second task
**Description:** As a dev, I want to do thing two so that it also works.

**Acceptance Criteria:**
- [x] Done criterion
- [ ] Open criterion

### TASK-003: Completed task
**Description:** As a dev, I want thing three done.

**Acceptance Criteria:**
- [x] All done A
- [x] All done B

## Non-Goals

Nothing here.
`

func writeTempPlan(t *testing.T, dir, filename, content string) {
	t.Helper()
	maggusDir := filepath.Join(dir, ".maggus")
	if err := os.MkdirAll(maggusDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(maggusDir, filename), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestParseFile(t *testing.T) {
	dir := t.TempDir()
	writeTempPlan(t, dir, "plan_1.md", testPlan)

	tasks, err := ParseFile(filepath.Join(dir, ".maggus", "plan_1.md"))
	if err != nil {
		t.Fatalf("ParseFile error: %v", err)
	}

	if len(tasks) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(tasks))
	}

	// TASK-001
	if tasks[0].ID != "TASK-001" {
		t.Errorf("task 0 ID = %q, want TASK-001", tasks[0].ID)
	}
	if tasks[0].Title != "First task" {
		t.Errorf("task 0 Title = %q, want 'First task'", tasks[0].Title)
	}
	if tasks[0].Description != "As a dev, I want to do thing one so that it works." {
		t.Errorf("task 0 Description = %q", tasks[0].Description)
	}
	if len(tasks[0].Criteria) != 2 {
		t.Fatalf("task 0 criteria count = %d, want 2", len(tasks[0].Criteria))
	}
	if tasks[0].Criteria[0].Checked || tasks[0].Criteria[1].Checked {
		t.Error("task 0 criteria should all be unchecked")
	}

	// TASK-002 — partially done
	if len(tasks[1].Criteria) != 2 {
		t.Fatalf("task 1 criteria count = %d, want 2", len(tasks[1].Criteria))
	}
	if !tasks[1].Criteria[0].Checked {
		t.Error("task 1 criterion 0 should be checked")
	}
	if tasks[1].Criteria[1].Checked {
		t.Error("task 1 criterion 1 should be unchecked")
	}

	// TASK-003 — all done
	if !tasks[2].IsComplete() {
		t.Error("task 2 should be complete")
	}
}

func TestIsComplete(t *testing.T) {
	complete := Task{Criteria: []Criterion{{Checked: true}, {Checked: true}}}
	if !complete.IsComplete() {
		t.Error("expected complete")
	}

	incomplete := Task{Criteria: []Criterion{{Checked: true}, {Checked: false}}}
	if incomplete.IsComplete() {
		t.Error("expected incomplete")
	}

	empty := Task{}
	if empty.IsComplete() {
		t.Error("task with no criteria should not be complete")
	}
}

func TestParsePlans(t *testing.T) {
	dir := t.TempDir()
	writeTempPlan(t, dir, "plan_1.md", testPlan)
	writeTempPlan(t, dir, "plan_2.md", `# Plan 2

### TASK-010: Extra task
**Description:** Another task from a second file.

**Acceptance Criteria:**
- [ ] Something
`)

	tasks, err := ParsePlans(dir)
	if err != nil {
		t.Fatalf("ParsePlans error: %v", err)
	}

	if len(tasks) != 4 {
		t.Fatalf("expected 4 tasks, got %d", len(tasks))
	}

	// Tasks from plan_1 come before plan_2
	if tasks[0].ID != "TASK-001" {
		t.Errorf("first task should be TASK-001, got %s", tasks[0].ID)
	}
	if tasks[3].ID != "TASK-010" {
		t.Errorf("last task should be TASK-010, got %s", tasks[3].ID)
	}
}

func TestFindNextIncomplete(t *testing.T) {
	tasks := []Task{
		{ID: "TASK-001", Criteria: []Criterion{{Checked: true}, {Checked: true}}},
		{ID: "TASK-002", Criteria: []Criterion{{Checked: true}, {Checked: false}}},
		{ID: "TASK-003", Criteria: []Criterion{{Checked: false}}},
	}

	next := FindNextIncomplete(tasks)
	if next == nil {
		t.Fatal("expected a task, got nil")
	}
	if next.ID != "TASK-002" {
		t.Errorf("expected TASK-002, got %s", next.ID)
	}
}

func TestFindNextIncomplete_AllDone(t *testing.T) {
	tasks := []Task{
		{ID: "TASK-001", Criteria: []Criterion{{Checked: true}}},
		{ID: "TASK-002", Criteria: []Criterion{{Checked: true}, {Checked: true}}},
	}

	next := FindNextIncomplete(tasks)
	if next != nil {
		t.Errorf("expected nil, got %s", next.ID)
	}
}

func TestFindNextIncomplete_OrderAcrossFiles(t *testing.T) {
	dir := t.TempDir()
	writeTempPlan(t, dir, "plan_1.md", `# Plan 1

### TASK-001: Done task
**Description:** Already done.

**Acceptance Criteria:**
- [x] Done
`)
	writeTempPlan(t, dir, "plan_2.md", `# Plan 2

### TASK-010: Open task
**Description:** Not done yet.

**Acceptance Criteria:**
- [ ] Not done
`)

	tasks, err := ParsePlans(dir)
	if err != nil {
		t.Fatalf("ParsePlans error: %v", err)
	}

	next := FindNextIncomplete(tasks)
	if next == nil {
		t.Fatal("expected a task, got nil")
	}
	if next.ID != "TASK-010" {
		t.Errorf("expected TASK-010, got %s", next.ID)
	}
}

func TestBlockedCriterion(t *testing.T) {
	dir := t.TempDir()
	writeTempPlan(t, dir, "plan_1.md", `# Plan

### TASK-001: Blocked task
**Description:** Has an unchecked blocked criterion.

**Acceptance Criteria:**
- [x] Done thing
- [ ] ⚠️ BLOCKED: Can't do this — needs human input

### TASK-002: Open task
**Description:** This one is workable.

**Acceptance Criteria:**
- [ ] Do the thing
`)

	tasks, err := ParsePlans(dir)
	if err != nil {
		t.Fatalf("ParsePlans error: %v", err)
	}

	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(tasks))
	}

	// TASK-001 is incomplete (unchecked blocked criterion) and blocked
	if tasks[0].IsComplete() {
		t.Error("TASK-001 should not be complete (has unchecked criterion)")
	}
	if !tasks[0].IsBlocked() {
		t.Error("TASK-001 should be blocked")
	}
	if tasks[0].IsWorkable() {
		t.Error("TASK-001 should not be workable")
	}

	// TASK-002 is workable
	if !tasks[1].IsWorkable() {
		t.Error("TASK-002 should be workable")
	}

	// FindNextIncomplete should skip blocked TASK-001 (it's complete anyway)
	// and return TASK-002
	next := FindNextIncomplete(tasks)
	if next == nil {
		t.Fatal("expected a task, got nil")
	}
	if next.ID != "TASK-002" {
		t.Errorf("expected TASK-002, got %s", next.ID)
	}
}

func TestParsePlans_SkipsCompletedFiles(t *testing.T) {
	dir := t.TempDir()
	writeTempPlan(t, dir, "plan_1_completed.md", `# Plan 1
### TASK-001: Done task
**Acceptance Criteria:**
- [x] Done
`)
	writeTempPlan(t, dir, "plan_2.md", `# Plan 2
### TASK-010: Open task
**Acceptance Criteria:**
- [ ] Not done
`)

	tasks, err := ParsePlans(dir)
	if err != nil {
		t.Fatalf("ParsePlans error: %v", err)
	}

	if len(tasks) != 1 {
		t.Fatalf("expected 1 task (completed file should be skipped), got %d", len(tasks))
	}
	if tasks[0].ID != "TASK-010" {
		t.Errorf("expected TASK-010, got %s", tasks[0].ID)
	}
}

func TestMarkCompletedPlans(t *testing.T) {
	dir := t.TempDir()

	// plan_1: all tasks complete
	writeTempPlan(t, dir, "plan_1.md", `# Plan 1
### TASK-001: Done task
**Acceptance Criteria:**
- [x] Done A
- [x] Done B
`)

	// plan_2: has incomplete task
	writeTempPlan(t, dir, "plan_2.md", `# Plan 2
### TASK-010: Open task
**Acceptance Criteria:**
- [ ] Not done
`)

	if err := MarkCompletedPlans(dir); err != nil {
		t.Fatalf("MarkCompletedPlans error: %v", err)
	}

	// plan_1 should have been renamed
	if _, err := os.Stat(filepath.Join(dir, ".maggus", "plan_1.md")); !os.IsNotExist(err) {
		t.Error("plan_1.md should have been renamed")
	}
	if _, err := os.Stat(filepath.Join(dir, ".maggus", "plan_1_completed.md")); err != nil {
		t.Error("plan_1_completed.md should exist")
	}

	// plan_2 should still be there
	if _, err := os.Stat(filepath.Join(dir, ".maggus", "plan_2.md")); err != nil {
		t.Error("plan_2.md should still exist")
	}
}

func TestMarkCompletedPlans_SkipsBlockedPlan(t *testing.T) {
	dir := t.TempDir()

	// An unchecked BLOCKED criterion means truly blocked — should NOT rename
	writeTempPlan(t, dir, "plan_1.md", `# Plan 1
### TASK-001: Blocked task
**Acceptance Criteria:**
- [x] Done
- [ ] ⚠️ BLOCKED: Needs human input
`)

	if err := MarkCompletedPlans(dir); err != nil {
		t.Fatalf("MarkCompletedPlans error: %v", err)
	}

	// Should NOT be renamed because the task has an unchecked blocked criterion
	if _, err := os.Stat(filepath.Join(dir, ".maggus", "plan_1.md")); err != nil {
		t.Error("plan_1.md should still exist (blocked tasks prevent completion)")
	}
}

func TestMarkCompletedPlans_RenamesWhenBlockedCriterionResolved(t *testing.T) {
	dir := t.TempDir()

	// A checked BLOCKED criterion means the block was resolved — should rename
	writeTempPlan(t, dir, "plan_1.md", `# Plan 1
### TASK-001: Formerly blocked task
**Acceptance Criteria:**
- [x] Done
- [x] ⚠️ BLOCKED: Needs human input — resolved: not applicable for CLI tool
`)

	if err := MarkCompletedPlans(dir); err != nil {
		t.Fatalf("MarkCompletedPlans error: %v", err)
	}

	// Should be renamed because all criteria are checked (block was resolved)
	if _, err := os.Stat(filepath.Join(dir, ".maggus", "plan_1.md")); !os.IsNotExist(err) {
		t.Error("plan_1.md should have been renamed (resolved blocked criterion)")
	}
	if _, err := os.Stat(filepath.Join(dir, ".maggus", "plan_1_completed.md")); err != nil {
		t.Error("plan_1_completed.md should exist")
	}
}

func TestBlockedOnlyMatchesPrefix(t *testing.T) {
	dir := t.TempDir()
	writeTempPlan(t, dir, "plan_1.md", `# Plan

### TASK-001: Describe blocked feature
**Description:** This task describes how blocked tasks work.

**Acceptance Criteria:**
- [ ] Blocked criteria `+"`"+`[ ] BLOCKED: ...`+"`"+` are shown in red
- [ ] Handle the BLOCKED: prefix in criterion text
- [ ] BLOCKED: This one is actually blocked
- [ ] ⚠️ BLOCKED: This one too
`)

	tasks, err := ParsePlans(dir)
	if err != nil {
		t.Fatalf("ParsePlans error: %v", err)
	}

	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}

	task := tasks[0]
	if len(task.Criteria) != 4 {
		t.Fatalf("expected 4 criteria, got %d", len(task.Criteria))
	}

	// Criteria that mention BLOCKED mid-text should NOT be blocked
	if task.Criteria[0].Blocked {
		t.Errorf("criterion 0 should not be blocked (BLOCKED: appears mid-text): %q", task.Criteria[0].Text)
	}
	if task.Criteria[1].Blocked {
		t.Errorf("criterion 1 should not be blocked (BLOCKED: appears mid-text): %q", task.Criteria[1].Text)
	}

	// Criteria that START with BLOCKED: should be blocked
	if !task.Criteria[2].Blocked {
		t.Errorf("criterion 2 should be blocked (starts with BLOCKED:): %q", task.Criteria[2].Text)
	}
	if !task.Criteria[3].Blocked {
		t.Errorf("criterion 3 should be blocked (starts with ⚠️ BLOCKED:): %q", task.Criteria[3].Text)
	}

	// Task should be blocked overall
	if !task.IsBlocked() {
		t.Error("task should be blocked")
	}
}

func TestBlockedIncompleteTask_Skipped(t *testing.T) {
	// Task has unchecked criteria AND a blocked criterion — should be skipped
	tasks := []Task{
		{
			ID: "TASK-001",
			Criteria: []Criterion{
				{Text: "Done", Checked: true},
				{Text: "⚠️ BLOCKED: Needs API key", Checked: false, Blocked: true},
				{Text: "Not done yet", Checked: false},
			},
		},
		{
			ID:       "TASK-002",
			Criteria: []Criterion{{Text: "Do it", Checked: false}},
		},
	}

	// TASK-001 is incomplete (has unchecked) AND blocked
	if tasks[0].IsComplete() {
		t.Error("TASK-001 should not be complete")
	}
	if !tasks[0].IsBlocked() {
		t.Error("TASK-001 should be blocked")
	}
	if tasks[0].IsWorkable() {
		t.Error("TASK-001 should not be workable")
	}

	next := FindNextIncomplete(tasks)
	if next == nil {
		t.Fatal("expected a task, got nil")
	}
	if next.ID != "TASK-002" {
		t.Errorf("expected TASK-002, got %s", next.ID)
	}
}
