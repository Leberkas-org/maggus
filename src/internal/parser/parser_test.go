package parser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testFeature = `# Feature 001: Test

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

func writeTempFeature(t *testing.T, dir, filename, content string) {
	t.Helper()
	featuresDir := filepath.Join(dir, ".maggus", "features")
	if err := os.MkdirAll(featuresDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(featuresDir, filename), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestParseFile(t *testing.T) {
	dir := t.TempDir()
	writeTempFeature(t, dir, "feature_001.md", testFeature)

	tasks, err := ParseFile(filepath.Join(dir, ".maggus", "features", "feature_001.md"))
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

func TestParseFeatures(t *testing.T) {
	dir := t.TempDir()
	writeTempFeature(t, dir, "feature_001.md", testFeature)
	writeTempFeature(t, dir, "feature_002.md", `# Feature 002

### TASK-010: Extra task
**Description:** Another task from a second file.

**Acceptance Criteria:**
- [ ] Something
`)

	tasks, err := ParseFeatures(dir)
	if err != nil {
		t.Fatalf("ParseFeatures error: %v", err)
	}

	if len(tasks) != 4 {
		t.Fatalf("expected 4 tasks, got %d", len(tasks))
	}

	// Tasks from feature_001 come before feature_002
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

func TestParseFile_TokenEstimate(t *testing.T) {
	const content = `# Feature 001: Token Estimate Test

## User Stories

### TASK-001: First task
**Description:** A task with token estimate.

**Token Estimate:** ~35k tokens
**Predecessors:** none
**Parallel:** no

**Acceptance Criteria:**
- [ ] Criterion A

### TASK-002: Second task

**Token Estimate:** ~20k tokens

**Acceptance Criteria:**
- [ ] Criterion B
`
	dir := t.TempDir()
	writeTempFeature(t, dir, "feature_001.md", content)

	tasks, err := ParseFile(filepath.Join(dir, ".maggus", "features", "feature_001.md"))
	if err != nil {
		t.Fatalf("ParseFile error: %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(tasks))
	}
	if tasks[0].TokenEstimate != 35000 {
		t.Errorf("task 0 TokenEstimate = %d, want 35000", tasks[0].TokenEstimate)
	}
	if tasks[1].TokenEstimate != 20000 {
		t.Errorf("task 1 TokenEstimate = %d, want 20000", tasks[1].TokenEstimate)
	}
}

func TestParseTokenEstimateK(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"~35k tokens", 35000},
		{"~45k tokens", 45000},
		{"~100k", 100000},
		{"~2k tokens", 2000},
		{"35000", 35000},
		{"~0k", 0},
		{"", 0},
		{"none", 0},
		{"~10k tokens extra words", 10000},
	}
	for _, tt := range tests {
		got := parseTokenEstimateK(tt.input)
		if got != tt.want {
			t.Errorf("parseTokenEstimateK(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestParseFile_TokenEstimate_MissingField(t *testing.T) {
	const content = `# Feature 001

### TASK-001: No estimate
**Description:** Task without token estimate.

**Acceptance Criteria:**
- [ ] Criterion A
`
	dir := t.TempDir()
	writeTempFeature(t, dir, "feature_001.md", content)

	tasks, err := ParseFile(filepath.Join(dir, ".maggus", "features", "feature_001.md"))
	if err != nil {
		t.Fatalf("ParseFile error: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if tasks[0].TokenEstimate != 0 {
		t.Errorf("task without Token Estimate should have TokenEstimate=0, got %d", tasks[0].TokenEstimate)
	}
}

func TestParseFile_TokenEstimate_NoKSuffix(t *testing.T) {
	const content = `# Feature 001

### TASK-001: Plain number estimate
**Token Estimate:** 50000

**Acceptance Criteria:**
- [ ] Criterion A
`
	dir := t.TempDir()
	writeTempFeature(t, dir, "feature_001.md", content)

	tasks, err := ParseFile(filepath.Join(dir, ".maggus", "features", "feature_001.md"))
	if err != nil {
		t.Fatalf("ParseFile error: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if tasks[0].TokenEstimate != 50000 {
		t.Errorf("task TokenEstimate = %d, want 50000", tasks[0].TokenEstimate)
	}
}

func TestParseFile_TokenEstimate_ZeroK(t *testing.T) {
	const content = `# Feature 001

### TASK-001: Zero estimate
**Token Estimate:** ~0k tokens

**Acceptance Criteria:**
- [ ] Criterion A
`
	dir := t.TempDir()
	writeTempFeature(t, dir, "feature_001.md", content)

	tasks, err := ParseFile(filepath.Join(dir, ".maggus", "features", "feature_001.md"))
	if err != nil {
		t.Fatalf("ParseFile error: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if tasks[0].TokenEstimate != 0 {
		t.Errorf("expected 0, got %d", tasks[0].TokenEstimate)
	}
}

func TestParseFile_Predecessors_AndParallel(t *testing.T) {
	const content = `# Feature 001

### TASK-001: First task
**Parallel:** yes — can run alongside others

**Predecessors:** TASK-000-001, TASK-000-002

**Acceptance Criteria:**
- [ ] Criterion A

### TASK-002: Second task
**Parallel:** no

**Predecessors:** none

**Acceptance Criteria:**
- [ ] Criterion B
`
	dir := t.TempDir()
	writeTempFeature(t, dir, "feature_001.md", content)

	tasks, err := ParseFile(filepath.Join(dir, ".maggus", "features", "feature_001.md"))
	if err != nil {
		t.Fatalf("ParseFile error: %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(tasks))
	}
	if !tasks[0].Parallel {
		t.Error("task 0: expected Parallel=true")
	}
	if len(tasks[0].Predecessors) != 2 {
		t.Errorf("task 0: expected 2 predecessors, got %d: %v", len(tasks[0].Predecessors), tasks[0].Predecessors)
	}
	if tasks[1].Parallel {
		t.Error("task 1: expected Parallel=false")
	}
	if len(tasks[1].Predecessors) != 0 {
		t.Errorf("task 1: expected 0 predecessors, got %d: %v", len(tasks[1].Predecessors), tasks[1].Predecessors)
	}
}

func TestParseFile_Predecessors_NoneWithComment(t *testing.T) {
	// "none" with trailing text should be treated as no predecessors.
	const content = `# Feature 001

### TASK-001: None lowercase with parens
**Predecessors:** none (Feature 005 provides controllers, but middleware is independent)

**Acceptance Criteria:**
- [ ] Criterion A

### TASK-002: None mixed-case with em-dash
**Predecessors:** None — explanation text

**Acceptance Criteria:**
- [ ] Criterion B

### TASK-003: None plain
**Predecessors:** none

**Acceptance Criteria:**
- [ ] Criterion C
`
	dir := t.TempDir()
	writeTempFeature(t, dir, "feature_001.md", content)

	tasks, err := ParseFile(filepath.Join(dir, ".maggus", "features", "feature_001.md"))
	if err != nil {
		t.Fatalf("ParseFile error: %v", err)
	}
	if len(tasks) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(tasks))
	}
	for i, task := range tasks {
		if len(task.Predecessors) != 0 {
			t.Errorf("task %d: expected 0 predecessors, got %d: %v", i, len(task.Predecessors), task.Predecessors)
		}
	}
}

func TestParseFile_Successors_IgnoredField(t *testing.T) {
	// **Successors:** is not a task-level field stored in Task — verify it doesn't break parsing.
	const content = `# Feature 001

### TASK-001: Task with successors field
**Token Estimate:** ~10k tokens
**Successors:** TASK-002

**Acceptance Criteria:**
- [ ] Criterion A
`
	dir := t.TempDir()
	writeTempFeature(t, dir, "feature_001.md", content)

	tasks, err := ParseFile(filepath.Join(dir, ".maggus", "features", "feature_001.md"))
	if err != nil {
		t.Fatalf("ParseFile error: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if tasks[0].TokenEstimate != 10000 {
		t.Errorf("TokenEstimate = %d, want 10000", tasks[0].TokenEstimate)
	}
}

func TestParseFile_AllMetadata(t *testing.T) {
	// Verify all metadata fields are parsed correctly from a realistic task block.
	const content = `# Feature 001

### TASK-001: Full metadata task
**Description:** As a user, I want everything.

**Token Estimate:** ~50k tokens
**Predecessors:** TASK-000-001
**Parallel:** yes
**Model:** opus

**Acceptance Criteria:**
- [ ] Criterion A
`
	dir := t.TempDir()
	writeTempFeature(t, dir, "feature_001.md", content)

	tasks, err := ParseFile(filepath.Join(dir, ".maggus", "features", "feature_001.md"))
	if err != nil {
		t.Fatalf("ParseFile error: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	t0 := tasks[0]
	if t0.TokenEstimate != 50000 {
		t.Errorf("TokenEstimate = %d, want 50000", t0.TokenEstimate)
	}
	if len(t0.Predecessors) != 1 || t0.Predecessors[0] != "TASK-000-001" {
		t.Errorf("Predecessors = %v, want [TASK-000-001]", t0.Predecessors)
	}
	if !t0.Parallel {
		t.Error("expected Parallel=true")
	}
	if t0.Model != "opus" {
		t.Errorf("Model = %q, want 'opus'", t0.Model)
	}
}

func TestParseFile_TokenEstimate_NilStringInput(t *testing.T) {
	// Regression: parseTokenEstimateK with completely non-numeric input returns 0.
	got := parseTokenEstimateK("not-a-number")
	if got != 0 {
		t.Errorf("parseTokenEstimateK(\"not-a-number\") = %d, want 0", got)
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
	writeTempFeature(t, dir, "feature_001.md", `# Feature 001

### TASK-001: Done task
**Description:** Already done.

**Acceptance Criteria:**
- [x] Done
`)
	writeTempFeature(t, dir, "feature_002.md", `# Feature 002

### TASK-010: Open task
**Description:** Not done yet.

**Acceptance Criteria:**
- [ ] Not done
`)

	tasks, err := ParseFeatures(dir)
	if err != nil {
		t.Fatalf("ParseFeatures error: %v", err)
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
	writeTempFeature(t, dir, "feature_001.md", `# Feature

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

	tasks, err := ParseFeatures(dir)
	if err != nil {
		t.Fatalf("ParseFeatures error: %v", err)
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

func TestParseFeatures_SkipsCompletedFiles(t *testing.T) {
	dir := t.TempDir()
	writeTempFeature(t, dir, "feature_001_completed.md", `# Feature 001
### TASK-001: Done task
**Acceptance Criteria:**
- [x] Done
`)
	writeTempFeature(t, dir, "feature_002.md", `# Feature 002
### TASK-010: Open task
**Acceptance Criteria:**
- [ ] Not done
`)

	tasks, err := ParseFeatures(dir)
	if err != nil {
		t.Fatalf("ParseFeatures error: %v", err)
	}

	if len(tasks) != 1 {
		t.Fatalf("expected 1 task (completed file should be skipped), got %d", len(tasks))
	}
	if tasks[0].ID != "TASK-010" {
		t.Errorf("expected TASK-010, got %s", tasks[0].ID)
	}
}

func TestMarkCompletedFeatures(t *testing.T) {
	dir := t.TempDir()

	// feature_001: all tasks complete
	writeTempFeature(t, dir, "feature_001.md", `# Feature 001
### TASK-001: Done task
**Acceptance Criteria:**
- [x] Done A
- [x] Done B
`)

	// feature_002: has incomplete task
	writeTempFeature(t, dir, "feature_002.md", `# Feature 002
### TASK-010: Open task
**Acceptance Criteria:**
- [ ] Not done
`)

	completed, err := MarkCompletedFeatures(dir, "")
	if err != nil {
		t.Fatalf("MarkCompletedFeatures error: %v", err)
	}

	// Should return exactly the completed file's original path.
	if len(completed) != 1 {
		t.Fatalf("expected 1 completed path, got %d", len(completed))
	}
	if filepath.Base(completed[0]) != "feature_001.md" {
		t.Errorf("expected feature_001.md, got %s", filepath.Base(completed[0]))
	}

	// feature_001 should have been renamed
	if _, err := os.Stat(filepath.Join(dir, ".maggus", "features", "feature_001.md")); !os.IsNotExist(err) {
		t.Error("feature_001.md should have been renamed")
	}
	if _, err := os.Stat(filepath.Join(dir, ".maggus", "features", "feature_001_completed.md")); err != nil {
		t.Error("feature_001_completed.md should exist")
	}

	// feature_002 should still be there
	if _, err := os.Stat(filepath.Join(dir, ".maggus", "features", "feature_002.md")); err != nil {
		t.Error("feature_002.md should still exist")
	}
}

func TestMarkCompletedFeatures_SkipsBlockedFeature(t *testing.T) {
	dir := t.TempDir()

	// An unchecked BLOCKED criterion means truly blocked — should NOT rename
	writeTempFeature(t, dir, "feature_001.md", `# Feature 001
### TASK-001: Blocked task
**Acceptance Criteria:**
- [x] Done
- [ ] ⚠️ BLOCKED: Needs human input
`)

	if _, err := MarkCompletedFeatures(dir, ""); err != nil {
		t.Fatalf("MarkCompletedFeatures error: %v", err)
	}

	// Should NOT be renamed because the task has an unchecked blocked criterion
	if _, err := os.Stat(filepath.Join(dir, ".maggus", "features", "feature_001.md")); err != nil {
		t.Error("feature_001.md should still exist (blocked tasks prevent completion)")
	}
}

func TestMarkCompletedFeatures_RenamesWhenBlockedCriterionResolved(t *testing.T) {
	dir := t.TempDir()

	// A checked BLOCKED criterion means the block was resolved — should rename
	writeTempFeature(t, dir, "feature_001.md", `# Feature 001
### TASK-001: Formerly blocked task
**Acceptance Criteria:**
- [x] Done
- [x] ⚠️ BLOCKED: Needs human input — resolved: not applicable for CLI tool
`)

	if _, err := MarkCompletedFeatures(dir, ""); err != nil {
		t.Fatalf("MarkCompletedFeatures error: %v", err)
	}

	// Should be renamed because all criteria are checked (block was resolved)
	if _, err := os.Stat(filepath.Join(dir, ".maggus", "features", "feature_001.md")); !os.IsNotExist(err) {
		t.Error("feature_001.md should have been renamed (resolved blocked criterion)")
	}
	if _, err := os.Stat(filepath.Join(dir, ".maggus", "features", "feature_001_completed.md")); err != nil {
		t.Error("feature_001_completed.md should exist")
	}
}

func TestBlockedOnlyMatchesPrefix(t *testing.T) {
	dir := t.TempDir()
	writeTempFeature(t, dir, "feature_001.md", `# Feature

### TASK-001: Describe blocked feature
**Description:** This task describes how blocked tasks work.

**Acceptance Criteria:**
- [ ] Blocked criteria `+"`"+`[ ] BLOCKED: ...`+"`"+` are shown in red
- [ ] Handle the BLOCKED: prefix in criterion text
- [ ] BLOCKED: This one is actually blocked
- [ ] ⚠️ BLOCKED: This one too
`)

	tasks, err := ParseFeatures(dir)
	if err != nil {
		t.Fatalf("ParseFeatures error: %v", err)
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

func TestFindNextIncomplete_AllBlocked(t *testing.T) {
	tasks := []Task{
		{
			ID: "TASK-002",
			Criteria: []Criterion{
				{Text: "BLOCKED: needs API", Checked: false, Blocked: true},
			},
		},
	}

	next := FindNextIncomplete(tasks)
	if next != nil {
		t.Errorf("expected nil, got %s", next.ID)
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

// --- Bug file helpers ---

func writeTempBug(t *testing.T, dir, filename, content string) {
	t.Helper()
	bugsDir := filepath.Join(dir, ".maggus", "bugs")
	if err := os.MkdirAll(bugsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bugsDir, filename), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestGlobBugFiles(t *testing.T) {
	dir := t.TempDir()
	writeTempBug(t, dir, "bug_001.md", "# Bug 1")
	writeTempBug(t, dir, "bug_002_completed.md", "# Bug 2")
	writeTempBug(t, dir, "bug_003.md", "# Bug 3")

	// Without completed
	files, err := GlobBugFiles(dir, false)
	if err != nil {
		t.Fatalf("GlobBugFiles error: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}

	// With completed
	files, err = GlobBugFiles(dir, true)
	if err != nil {
		t.Fatalf("GlobBugFiles error: %v", err)
	}
	if len(files) != 3 {
		t.Fatalf("expected 3 files, got %d", len(files))
	}
}

func TestGlobBugFiles_Empty(t *testing.T) {
	dir := t.TempDir()
	files, err := GlobBugFiles(dir, false)
	if err != nil {
		t.Fatalf("GlobBugFiles error: %v", err)
	}
	if len(files) != 0 {
		t.Fatalf("expected 0 files, got %d", len(files))
	}
}

func TestSortBugFiles(t *testing.T) {
	files := []string{
		"/tmp/bug_010.md",
		"/tmp/bug_002.md",
		"/tmp/bug_001.md",
		"/tmp/bug_020.md",
	}
	SortBugFiles(files)
	expected := []string{
		"/tmp/bug_001.md",
		"/tmp/bug_002.md",
		"/tmp/bug_010.md",
		"/tmp/bug_020.md",
	}
	for i, f := range files {
		if f != expected[i] {
			t.Errorf("index %d: got %s, want %s", i, f, expected[i])
		}
	}
}

func TestParseBugs(t *testing.T) {
	dir := t.TempDir()
	writeTempBug(t, dir, "bug_001.md", `# Bug 001

### BUG-001-001: Fix login crash
**Description:** Login crashes on empty password.

**Acceptance Criteria:**
- [ ] Fix the crash
- [ ] Add validation
`)
	writeTempBug(t, dir, "bug_002.md", `# Bug 002

### BUG-002-001: Fix display issue
**Description:** Display is broken.

**Acceptance Criteria:**
- [x] Fixed display
`)

	tasks, err := ParseBugs(dir)
	if err != nil {
		t.Fatalf("ParseBugs error: %v", err)
	}

	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(tasks))
	}
	if tasks[0].ID != "BUG-001-001" {
		t.Errorf("first task ID = %q, want BUG-001-001", tasks[0].ID)
	}
	if tasks[1].ID != "BUG-002-001" {
		t.Errorf("second task ID = %q, want BUG-002-001", tasks[1].ID)
	}
}

func TestParseBugsGrouped(t *testing.T) {
	dir := t.TempDir()
	writeTempBug(t, dir, "bug_001.md", `# Bug 001

### BUG-001-001: Fix crash
**Acceptance Criteria:**
- [ ] Fix it

### BUG-001-002: Add test
**Acceptance Criteria:**
- [ ] Test it
`)
	writeTempBug(t, dir, "bug_002.md", `# Bug 002

### BUG-002-001: Another fix
**Acceptance Criteria:**
- [ ] Fix another
`)

	bugs, err := ParseBugsGrouped(dir)
	if err != nil {
		t.Fatalf("ParseBugsGrouped error: %v", err)
	}

	if len(bugs) != 2 {
		t.Fatalf("expected 2 bug groups, got %d", len(bugs))
	}
	if len(bugs[0].Tasks) != 2 {
		t.Errorf("bug_001 should have 2 tasks, got %d", len(bugs[0].Tasks))
	}
	if len(bugs[1].Tasks) != 1 {
		t.Errorf("bug_002 should have 1 task, got %d", len(bugs[1].Tasks))
	}
}

func TestMarkCompletedBugs(t *testing.T) {
	dir := t.TempDir()

	// bug_001: all complete
	writeTempBug(t, dir, "bug_001.md", `# Bug 001
### BUG-001-001: Done
**Acceptance Criteria:**
- [x] Fixed
- [x] Tested
`)

	// bug_002: incomplete
	writeTempBug(t, dir, "bug_002.md", `# Bug 002
### BUG-002-001: Not done
**Acceptance Criteria:**
- [ ] Not fixed
`)

	completed, err := MarkCompletedBugs(dir, "")
	if err != nil {
		t.Fatalf("MarkCompletedBugs error: %v", err)
	}

	// Should return exactly the completed file's original path.
	if len(completed) != 1 {
		t.Fatalf("expected 1 completed path, got %d", len(completed))
	}
	if filepath.Base(completed[0]) != "bug_001.md" {
		t.Errorf("expected bug_001.md, got %s", filepath.Base(completed[0]))
	}

	// bug_001 should be renamed
	if _, err := os.Stat(filepath.Join(dir, ".maggus", "bugs", "bug_001.md")); !os.IsNotExist(err) {
		t.Error("bug_001.md should have been renamed")
	}
	if _, err := os.Stat(filepath.Join(dir, ".maggus", "bugs", "bug_001_completed.md")); err != nil {
		t.Error("bug_001_completed.md should exist")
	}

	// bug_002 should still be there
	if _, err := os.Stat(filepath.Join(dir, ".maggus", "bugs", "bug_002.md")); err != nil {
		t.Error("bug_002.md should still exist")
	}
}

func TestMarkCompletedFeatures_DeleteAction(t *testing.T) {
	dir := t.TempDir()

	writeTempFeature(t, dir, "feature_001.md", `# Feature 001
### TASK-001: Done task
**Acceptance Criteria:**
- [x] Done A
- [x] Done B
`)

	if _, err := MarkCompletedFeatures(dir, "delete"); err != nil {
		t.Fatalf("MarkCompletedFeatures error: %v", err)
	}

	// feature_001 should have been deleted (not renamed)
	if _, err := os.Stat(filepath.Join(dir, ".maggus", "features", "feature_001.md")); !os.IsNotExist(err) {
		t.Error("feature_001.md should have been deleted")
	}
	if _, err := os.Stat(filepath.Join(dir, ".maggus", "features", "feature_001_completed.md")); !os.IsNotExist(err) {
		t.Error("feature_001_completed.md should NOT exist when action is delete")
	}
}

func TestMarkCompletedFeatures_RenameAction(t *testing.T) {
	dir := t.TempDir()

	writeTempFeature(t, dir, "feature_001.md", `# Feature 001
### TASK-001: Done task
**Acceptance Criteria:**
- [x] Done A
`)

	if _, err := MarkCompletedFeatures(dir, "rename"); err != nil {
		t.Fatalf("MarkCompletedFeatures error: %v", err)
	}

	// Explicit "rename" should behave like default (empty string)
	if _, err := os.Stat(filepath.Join(dir, ".maggus", "features", "feature_001_completed.md")); err != nil {
		t.Error("feature_001_completed.md should exist when action is rename")
	}
}

func TestMarkCompletedFeatures_UnknownActionDefaultsToRename(t *testing.T) {
	dir := t.TempDir()

	writeTempFeature(t, dir, "feature_001.md", `# Feature 001
### TASK-001: Done task
**Acceptance Criteria:**
- [x] Done A
`)

	if _, err := MarkCompletedFeatures(dir, "archive"); err != nil {
		t.Fatalf("MarkCompletedFeatures error: %v", err)
	}

	// Unknown action should default to rename
	if _, err := os.Stat(filepath.Join(dir, ".maggus", "features", "feature_001_completed.md")); err != nil {
		t.Error("feature_001_completed.md should exist when action is unknown")
	}
}

func TestMarkCompletedBugs_DeleteAction(t *testing.T) {
	dir := t.TempDir()

	writeTempBug(t, dir, "bug_001.md", `# Bug 001
### BUG-001-001: Done
**Acceptance Criteria:**
- [x] Fixed
- [x] Tested
`)

	if _, err := MarkCompletedBugs(dir, "delete"); err != nil {
		t.Fatalf("MarkCompletedBugs error: %v", err)
	}

	// bug_001 should have been deleted
	if _, err := os.Stat(filepath.Join(dir, ".maggus", "bugs", "bug_001.md")); !os.IsNotExist(err) {
		t.Error("bug_001.md should have been deleted")
	}
	if _, err := os.Stat(filepath.Join(dir, ".maggus", "bugs", "bug_001_completed.md")); !os.IsNotExist(err) {
		t.Error("bug_001_completed.md should NOT exist when action is delete")
	}
}

func TestMarkCompletedBugs_UnknownActionDefaultsToRename(t *testing.T) {
	dir := t.TempDir()

	writeTempBug(t, dir, "bug_001.md", `# Bug 001
### BUG-001-001: Done
**Acceptance Criteria:**
- [x] Fixed
`)

	if _, err := MarkCompletedBugs(dir, "something"); err != nil {
		t.Fatalf("MarkCompletedBugs error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, ".maggus", "bugs", "bug_001_completed.md")); err != nil {
		t.Error("bug_001_completed.md should exist when action is unknown")
	}
}

func TestMigrateLegacyBugIDs(t *testing.T) {
	dir := t.TempDir()
	writeTempBug(t, dir, "bug_002.md", `# Bug 002

### TASK-001: Fix the crash
**Description:** Crash on login.

**Acceptance Criteria:**
- [ ] Fix it

### TASK-002: Add test
**Description:** Add a test for the fix.

**Acceptance Criteria:**
- [ ] Test it
`)

	path := filepath.Join(dir, ".maggus", "bugs", "bug_002.md")
	modified, err := MigrateLegacyBugIDs(path)
	if err != nil {
		t.Fatalf("MigrateLegacyBugIDs error: %v", err)
	}
	if !modified {
		t.Error("expected file to be modified")
	}

	// Read back and verify
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read error: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, "### BUG-002-001: Fix the crash") {
		t.Errorf("expected BUG-002-001 heading, got:\n%s", content)
	}
	if !strings.Contains(content, "### BUG-002-002: Add test") {
		t.Errorf("expected BUG-002-002 heading, got:\n%s", content)
	}
	if strings.Contains(content, "### TASK-") {
		t.Error("legacy TASK- headings should have been replaced")
	}
}

func TestMigrateLegacyBugIDs_NoLegacy(t *testing.T) {
	dir := t.TempDir()
	writeTempBug(t, dir, "bug_001.md", `# Bug 001

### BUG-001-001: Already migrated
**Acceptance Criteria:**
- [ ] Done
`)

	path := filepath.Join(dir, ".maggus", "bugs", "bug_001.md")
	modified, err := MigrateLegacyBugIDs(path)
	if err != nil {
		t.Fatalf("MigrateLegacyBugIDs error: %v", err)
	}
	if modified {
		t.Error("file should not be modified when no legacy IDs exist")
	}
}

func TestParseBugs_AutoMigration(t *testing.T) {
	dir := t.TempDir()
	writeTempBug(t, dir, "bug_001.md", `# Bug 001

### TASK-001: Legacy task
**Description:** This has a legacy ID.

**Acceptance Criteria:**
- [ ] Fix it
`)

	tasks, err := ParseBugs(dir)
	if err != nil {
		t.Fatalf("ParseBugs error: %v", err)
	}

	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if tasks[0].ID != "BUG-001-001" {
		t.Errorf("task ID = %q, want BUG-001-001 (auto-migrated)", tasks[0].ID)
	}
}

func TestParseBugs_SkipsCompletedFiles(t *testing.T) {
	dir := t.TempDir()
	writeTempBug(t, dir, "bug_001_completed.md", `# Bug 001
### BUG-001-001: Done
**Acceptance Criteria:**
- [x] Done
`)
	writeTempBug(t, dir, "bug_002.md", `# Bug 002
### BUG-002-001: Open
**Acceptance Criteria:**
- [ ] Not done
`)

	tasks, err := ParseBugs(dir)
	if err != nil {
		t.Fatalf("ParseBugs error: %v", err)
	}

	if len(tasks) != 1 {
		t.Fatalf("expected 1 task (completed file skipped), got %d", len(tasks))
	}
	if tasks[0].ID != "BUG-002-001" {
		t.Errorf("expected BUG-002-001, got %s", tasks[0].ID)
	}
}

func TestExistingFeatureParsing_NotAffected(t *testing.T) {
	dir := t.TempDir()
	writeTempFeature(t, dir, "feature_001.md", testFeature)

	tasks, err := ParseFeatures(dir)
	if err != nil {
		t.Fatalf("ParseFeatures error: %v", err)
	}

	if len(tasks) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(tasks))
	}
	if tasks[0].ID != "TASK-001" {
		t.Errorf("first task should be TASK-001, got %s", tasks[0].ID)
	}
}

func TestParseFile_BugTaskIDs(t *testing.T) {
	dir := t.TempDir()
	writeTempBug(t, dir, "bug_001.md", `# Bug 001

### BUG-001-001: First bug task
**Description:** First bug task description.

**Acceptance Criteria:**
- [ ] Fix crash
- [x] Add logging

### BUG-001-002: Second bug task
**Description:** Second bug task description.

**Acceptance Criteria:**
- [ ] Write test
`)

	tasks, err := ParseFile(filepath.Join(dir, ".maggus", "bugs", "bug_001.md"))
	if err != nil {
		t.Fatalf("ParseFile error: %v", err)
	}

	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(tasks))
	}
	if tasks[0].ID != "BUG-001-001" {
		t.Errorf("task 0 ID = %q, want BUG-001-001", tasks[0].ID)
	}
	if tasks[0].Title != "First bug task" {
		t.Errorf("task 0 Title = %q, want 'First bug task'", tasks[0].Title)
	}
	if len(tasks[0].Criteria) != 2 {
		t.Fatalf("task 0 criteria count = %d, want 2", len(tasks[0].Criteria))
	}
	if tasks[1].ID != "BUG-001-002" {
		t.Errorf("task 1 ID = %q, want BUG-001-002", tasks[1].ID)
	}
}

func TestParseMaggusID_ValidUUID(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "feature_001.md")
	os.WriteFile(path, []byte("<!-- maggus-id: d2062a56-007c-47cd-8e7b-ba3d2e361689 -->\n# Feature 001: Test\n"), 0o644)

	got := ParseMaggusID(path)
	if got != "d2062a56-007c-47cd-8e7b-ba3d2e361689" {
		t.Errorf("ParseMaggusID = %q, want d2062a56-007c-47cd-8e7b-ba3d2e361689", got)
	}
}

func TestParseMaggusID_MissingComment(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "feature_001.md")
	os.WriteFile(path, []byte("# Feature 001: No UUID\n\nSome content.\n"), 0o644)

	got := ParseMaggusID(path)
	if got != "" {
		t.Errorf("ParseMaggusID = %q, want empty string", got)
	}
}

func TestParseMaggusID_WrongFormat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "feature_001.md")
	os.WriteFile(path, []byte("<!-- wrong-key: d2062a56-007c-47cd-8e7b-ba3d2e361689 -->\n"), 0o644)

	got := ParseMaggusID(path)
	if got != "" {
		t.Errorf("ParseMaggusID = %q, want empty string for wrong format", got)
	}
}

func TestParseMaggusID_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.md")
	os.WriteFile(path, []byte(""), 0o644)

	got := ParseMaggusID(path)
	if got != "" {
		t.Errorf("ParseMaggusID = %q, want empty string for empty file", got)
	}
}

func TestParseMaggusID_FileNotFound(t *testing.T) {
	got := ParseMaggusID("/nonexistent/path/feature_001.md")
	if got != "" {
		t.Errorf("ParseMaggusID = %q, want empty string for missing file", got)
	}
}

func TestParseFile_ModelField(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		wantModel string
	}{
		{
			name: "model present",
			content: `# Feature 001: Test

### TASK-001: Task with model
**Description:** A task.
**Model:** opus

**Acceptance Criteria:**
- [ ] Something
`,
			wantModel: "opus",
		},
		{
			name: "model absent",
			content: `# Feature 001: Test

### TASK-001: Task without model
**Description:** A task.

**Acceptance Criteria:**
- [ ] Something
`,
			wantModel: "",
		},
		{
			name: "model with whitespace",
			content: `# Feature 001: Test

### TASK-001: Task with whitespace model
**Description:** A task.
**Model:**    sonnet

**Acceptance Criteria:**
- [ ] Something
`,
			wantModel: "sonnet",
		},
		{
			name: "model with full ID",
			content: `# Feature 001: Test

### TASK-001: Task with full model ID
**Description:** A task.
**Model:** claude-opus-4-6

**Acceptance Criteria:**
- [ ] Something
`,
			wantModel: "claude-opus-4-6",
		},
		{
			name: "model with dash comment",
			content: `# Feature 001: Test

### TASK-001: Task with comment
**Description:** A task.
**Model:** haiku — fast enough for this

**Acceptance Criteria:**
- [ ] Something
`,
			wantModel: "haiku",
		},
		{
			name: "model with space comment",
			content: `# Feature 001: Test

### TASK-001: Task with space comment
**Description:** A task.
**Model:** opus straightforward

**Acceptance Criteria:**
- [ ] Something
`,
			wantModel: "opus",
		},
		{
			name: "model full ID with dash comment",
			content: `# Feature 001: Test

### TASK-001: Task with full ID and comment
**Description:** A task.
**Model:** claude-sonnet-4-6 — fast

**Acceptance Criteria:**
- [ ] Something
`,
			wantModel: "claude-sonnet-4-6",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			writeTempFeature(t, dir, "feature_001.md", tt.content)

			tasks, err := ParseFile(filepath.Join(dir, ".maggus", "features", "feature_001.md"))
			if err != nil {
				t.Fatalf("ParseFile error: %v", err)
			}
			if len(tasks) != 1 {
				t.Fatalf("expected 1 task, got %d", len(tasks))
			}
			if tasks[0].Model != tt.wantModel {
				t.Errorf("Model = %q, want %q", tasks[0].Model, tt.wantModel)
			}
		})
	}
}

func TestParseFile_ParallelAndPredecessors(t *testing.T) {
	content := `# Feature 038: Parallel Test

### TASK-038-001: First task
**Description:** First task
**Predecessors:** none
**Parallel:** yes — can run alongside TASK-038-002

**Acceptance Criteria:**
- [ ] Something

### TASK-038-002: Second task
**Description:** Second task
**Predecessors:** TASK-038-001, TASK-038-003
**Parallel:** no

**Acceptance Criteria:**
- [ ] Something else

### TASK-038-003: Third task
**Description:** Third task

**Acceptance Criteria:**
- [ ] Another thing
`

	dir := t.TempDir()
	writeTempFeature(t, dir, "feature_038.md", content)
	tasks, err := ParseFile(filepath.Join(dir, ".maggus", "features", "feature_038.md"))
	if err != nil {
		t.Fatalf("ParseFile error: %v", err)
	}
	if len(tasks) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(tasks))
	}

	// Task 1: Parallel=yes, no predecessors
	if !tasks[0].Parallel {
		t.Error("TASK-038-001 should have Parallel=true")
	}
	if len(tasks[0].Predecessors) != 0 {
		t.Errorf("TASK-038-001 predecessors = %v, want empty", tasks[0].Predecessors)
	}

	// Task 2: Parallel=no, two predecessors
	if tasks[1].Parallel {
		t.Error("TASK-038-002 should have Parallel=false")
	}
	if len(tasks[1].Predecessors) != 2 {
		t.Fatalf("TASK-038-002 predecessors count = %d, want 2", len(tasks[1].Predecessors))
	}
	if tasks[1].Predecessors[0] != "TASK-038-001" || tasks[1].Predecessors[1] != "TASK-038-003" {
		t.Errorf("TASK-038-002 predecessors = %v", tasks[1].Predecessors)
	}

	// Task 3: defaults (no Parallel or Predecessors lines)
	if tasks[2].Parallel {
		t.Error("TASK-038-003 should have Parallel=false (default)")
	}
	if len(tasks[2].Predecessors) != 0 {
		t.Errorf("TASK-038-003 predecessors = %v, want empty", tasks[2].Predecessors)
	}
}

// --- SKIPPED criterion tests ---

func TestParseFile_SkippedCriteria(t *testing.T) {
	dir := t.TempDir()
	writeTempFeature(t, dir, "feature_001.md", `# Feature 001: Skipped Test

### TASK-001: Task with skipped criteria
**Description:** Has both skip markers.

**Acceptance Criteria:**
- [ ] Normal criterion
- [ ] SKIPPED: Skip via unchecked
- [>] SKIPPED: Skip via [>] marker
- [x] Done criterion
`)

	tasks, err := ParseFile(filepath.Join(dir, ".maggus", "features", "feature_001.md"))
	if err != nil {
		t.Fatalf("ParseFile error: %v", err)
	}

	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	task := tasks[0]
	if len(task.Criteria) != 4 {
		t.Fatalf("expected 4 criteria, got %d", len(task.Criteria))
	}

	// Criterion 0: normal
	if task.Criteria[0].Skipped {
		t.Error("criterion 0 should not be skipped")
	}

	// Criterion 1: - [ ] SKIPPED:
	if !task.Criteria[1].Skipped {
		t.Errorf("criterion 1 should be skipped: %q", task.Criteria[1].Text)
	}
	if task.Criteria[1].Checked {
		t.Error("criterion 1 should not be checked")
	}
	if task.Criteria[1].Blocked {
		t.Error("criterion 1 should not be blocked")
	}

	// Criterion 2: - [>] SKIPPED:
	if !task.Criteria[2].Skipped {
		t.Errorf("criterion 2 should be skipped: %q", task.Criteria[2].Text)
	}
	if task.Criteria[2].Checked {
		t.Error("criterion 2 should not be checked")
	}
	if task.Criteria[2].Blocked {
		t.Error("criterion 2 should not be blocked")
	}

	// Criterion 3: - [x] checked
	if task.Criteria[3].Skipped {
		t.Error("criterion 3 should not be skipped")
	}
	if !task.Criteria[3].Checked {
		t.Error("criterion 3 should be checked")
	}
}

func TestParseFile_SkippedCriteria_NonSkipBracket(t *testing.T) {
	// [>] without SKIPPED: prefix — not considered skipped
	dir := t.TempDir()
	writeTempFeature(t, dir, "feature_001.md", `# Feature 001

### TASK-001: Edge case
**Acceptance Criteria:**
- [>] Not a skipped criterion (no SKIPPED: prefix)
`)

	tasks, err := ParseFile(filepath.Join(dir, ".maggus", "features", "feature_001.md"))
	if err != nil {
		t.Fatalf("ParseFile error: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if tasks[0].Criteria[0].Skipped {
		t.Error("[>] without SKIPPED: prefix should not be skipped")
	}
}

func TestIsSkipped(t *testing.T) {
	notSkipped := Task{Criteria: []Criterion{
		{Text: "Normal", Checked: false, Skipped: false},
		{Text: "Done", Checked: true, Skipped: false},
	}}
	if notSkipped.IsSkipped() {
		t.Error("expected not skipped")
	}

	skipped := Task{Criteria: []Criterion{
		{Text: "Normal", Checked: false, Skipped: false},
		{Text: "SKIPPED: something", Checked: false, Skipped: true},
	}}
	if !skipped.IsSkipped() {
		t.Error("expected skipped")
	}

	empty := Task{}
	if empty.IsSkipped() {
		t.Error("task with no criteria should not be skipped")
	}
}

func TestIsWorkable_SkippedNotWorkable(t *testing.T) {
	task := Task{
		ID: "TASK-001",
		Criteria: []Criterion{
			{Text: "Normal", Checked: false, Skipped: false},
			{Text: "SKIPPED: skip this", Checked: false, Skipped: true},
		},
	}

	if task.IsWorkable() {
		t.Error("skipped task should not be workable")
	}
	if task.IsComplete() {
		t.Error("skipped task should not be complete")
	}
	if task.IsBlocked() {
		t.Error("skipped task should not be blocked")
	}
	if !task.IsSkipped() {
		t.Error("expected IsSkipped() true")
	}
}

func TestFindNextIncomplete_SkipsSkippedTasks(t *testing.T) {
	tasks := []Task{
		{
			ID: "TASK-001",
			Criteria: []Criterion{
				{Text: "SKIPPED: user skipped this", Checked: false, Skipped: true},
			},
		},
		{
			ID:       "TASK-002",
			Criteria: []Criterion{{Text: "Do the thing", Checked: false}},
		},
	}

	next := FindNextIncomplete(tasks)
	if next == nil {
		t.Fatal("expected a task, got nil")
	}
	if next.ID != "TASK-002" {
		t.Errorf("expected TASK-002, got %s", next.ID)
	}
}

func TestFindNextIncomplete_AllSkipped(t *testing.T) {
	tasks := []Task{
		{
			ID: "TASK-001",
			Criteria: []Criterion{
				{Text: "SKIPPED: done later", Checked: false, Skipped: true},
			},
		},
	}

	next := FindNextIncomplete(tasks)
	if next != nil {
		t.Errorf("expected nil when all tasks are skipped, got %s", next.ID)
	}
}

func TestSkipAndUnskipCriterion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "feature_001.md")
	content := "# Feature 001\n\n### TASK-001: Skippable task\n**Acceptance Criteria:**\n- [ ] Do something important\n- [ ] Another criterion\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	c := Criterion{Text: "Do something important", Checked: false, Skipped: false}

	// Skip it
	if err := SkipCriterion(path, c); err != nil {
		t.Fatalf("SkipCriterion error: %v", err)
	}

	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "- [>] SKIPPED: Do something important") {
		t.Errorf("after SkipCriterion, expected [>] SKIPPED: marker, got:\n%s", string(data))
	}
	if strings.Contains(string(data), "- [ ] Do something important") {
		t.Error("original unchecked line should be gone after skip")
	}

	// Parse back and verify
	tasks, err := ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile after skip: %v", err)
	}
	if len(tasks) != 1 || len(tasks[0].Criteria) != 2 {
		t.Fatalf("expected 1 task with 2 criteria, got %d tasks", len(tasks))
	}
	if !tasks[0].Criteria[0].Skipped {
		t.Error("criterion 0 should be skipped after SkipCriterion")
	}
	if !tasks[0].IsSkipped() {
		t.Error("task should be IsSkipped() after SkipCriterion")
	}

	// Unskip it
	skippedC := tasks[0].Criteria[0]
	if err := UnskipCriterion(path, skippedC); err != nil {
		t.Fatalf("UnskipCriterion error: %v", err)
	}

	data, _ = os.ReadFile(path)
	if !strings.Contains(string(data), "- [ ] Do something important") {
		t.Errorf("after UnskipCriterion, expected original [ ] line, got:\n%s", string(data))
	}
	if strings.Contains(string(data), "[>]") {
		t.Error("after UnskipCriterion, [>] marker should be gone")
	}
}

func TestSkipCriterion_CheckedCriterion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "feature_001.md")
	content := "# Feature 001\n\n### TASK-001: Task\n**Acceptance Criteria:**\n- [x] Already done\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	c := Criterion{Text: "Already done", Checked: true}
	if err := SkipCriterion(path, c); err != nil {
		t.Fatalf("SkipCriterion error: %v", err)
	}

	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "- [>] SKIPPED: Already done") {
		t.Errorf("expected [>] SKIPPED: marker after skipping checked criterion, got:\n%s", string(data))
	}
}

func TestSkipCriterion_NotFound(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "feature_001.md")
	content := "# Feature 001\n### TASK-001: Task\n**Acceptance Criteria:**\n- [ ] Existing criterion\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	c := Criterion{Text: "Nonexistent criterion", Checked: false}
	err := SkipCriterion(path, c)
	if err == nil {
		t.Error("expected error when criterion not found")
	}
}

func TestUnskipCriterion_NotFound(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "feature_001.md")
	content := "# Feature 001\n### TASK-001: Task\n**Acceptance Criteria:**\n- [ ] Normal criterion\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	c := Criterion{Text: "SKIPPED: Nonexistent", Checked: false, Skipped: true}
	err := UnskipCriterion(path, c)
	if err == nil {
		t.Error("expected error when skipped criterion not found")
	}
}

func TestIsSkipped_And_IsBlocked_Interaction(t *testing.T) {
	// A task can be both blocked and skipped
	task := Task{
		ID: "TASK-001",
		Criteria: []Criterion{
			{Text: "BLOCKED: needs API", Checked: false, Blocked: true},
			{Text: "SKIPPED: defer this", Checked: false, Skipped: true},
			{Text: "Normal", Checked: false},
		},
	}

	if !task.IsBlocked() {
		t.Error("task should be blocked")
	}
	if !task.IsSkipped() {
		t.Error("task should be skipped")
	}
	if task.IsWorkable() {
		t.Error("task should not be workable when both blocked and skipped")
	}
	if task.IsComplete() {
		t.Error("task should not be complete")
	}
}

// ─── PredecessorsSatisfied and IsRunnable ─────────────────────────────────────

func TestPredecessorsSatisfied_NoPredecessors(t *testing.T) {
	task := Task{ID: "T1"}
	// A task with no predecessors is always satisfied, even with nil maps.
	if !task.PredecessorsSatisfied(nil, nil) {
		t.Error("expected true for task with no predecessors")
	}
	if !task.PredecessorsSatisfied(map[string]bool{}, map[string]bool{}) {
		t.Error("expected true for task with no predecessors and empty maps")
	}
}

func TestPredecessorsSatisfied_PredInCompleted(t *testing.T) {
	task := Task{ID: "T2", Predecessors: []string{"T1"}}
	if !task.PredecessorsSatisfied(map[string]bool{"T1": true}, nil) {
		t.Error("expected true when predecessor is in completedIDs")
	}
}

func TestPredecessorsSatisfied_PredInSkippedOrBlocked(t *testing.T) {
	task := Task{ID: "T2", Predecessors: []string{"T1"}}
	if !task.PredecessorsSatisfied(nil, map[string]bool{"T1": true}) {
		t.Error("expected true when predecessor is in skippedOrBlockedIDs")
	}
}

func TestPredecessorsSatisfied_PredUnsatisfied(t *testing.T) {
	task := Task{ID: "T2", Predecessors: []string{"T1"}}
	// Non-nil maps that don't contain the predecessor → unsatisfied.
	if task.PredecessorsSatisfied(map[string]bool{"T9": true}, map[string]bool{}) {
		t.Error("expected false when predecessor is not the one in the map")
	}
	if task.PredecessorsSatisfied(map[string]bool{}, map[string]bool{}) {
		t.Error("expected false when predecessor is in neither non-nil map")
	}
}

func TestPredecessorsSatisfied_NilMapsDegrade(t *testing.T) {
	// When both maps are nil, no predecessor context is available.
	// PredecessorsSatisfied must return true regardless of Predecessors length,
	// degrading gracefully to IsWorkable() behaviour.
	task := Task{ID: "T2", Predecessors: []string{"T1"}}
	if !task.PredecessorsSatisfied(nil, nil) {
		t.Error("expected true when both maps are nil (no predecessor context)")
	}
}

func TestPredecessorsSatisfied_MultiplePreds(t *testing.T) {
	task := Task{ID: "T3", Predecessors: []string{"T1", "T2"}}
	// Both satisfied via different maps.
	if !task.PredecessorsSatisfied(map[string]bool{"T1": true}, map[string]bool{"T2": true}) {
		t.Error("expected true when all predecessors are satisfied")
	}
	// Only one satisfied — must return false.
	if task.PredecessorsSatisfied(map[string]bool{"T1": true}, nil) {
		t.Error("expected false when only one of two predecessors is satisfied")
	}
}

func TestIsRunnable_WorkableNoPredecessors(t *testing.T) {
	task := Task{
		ID:       "T1",
		Criteria: []Criterion{{Text: "Do it", Checked: false}},
	}
	// No predecessors, workable — should be runnable with nil maps.
	if !task.IsRunnable(nil, nil) {
		t.Error("expected true for workable task with no predecessors")
	}
}

func TestIsRunnable_NotWorkable(t *testing.T) {
	// Complete task — not runnable regardless of predecessor maps.
	task := Task{
		ID:       "T1",
		Criteria: []Criterion{{Text: "Done", Checked: true}},
	}
	if task.IsRunnable(nil, nil) {
		t.Error("expected false for complete task")
	}
	if task.IsRunnable(map[string]bool{"T0": true}, map[string]bool{}) {
		t.Error("expected false for complete task even with satisfied predecessors")
	}
}

// ─── Cross-feature predecessor parsing ────────────────────────────────────────

func TestParseCrossFeatureToken_SingleFeature(t *testing.T) {
	ref, ok := parseCrossFeatureToken("Feature 5")
	if !ok {
		t.Fatal("expected match for 'Feature 5'")
	}
	if len(ref.FeatureNums) != 1 || ref.FeatureNums[0] != 5 {
		t.Errorf("FeatureNums = %v, want [5]", ref.FeatureNums)
	}
	if ref.Label != "" {
		t.Errorf("Label = %q, want empty", ref.Label)
	}
}

func TestParseCrossFeatureToken_SingleFeatureWithLabel(t *testing.T) {
	ref, ok := parseCrossFeatureToken("Feature 42 (controllers)")
	if !ok {
		t.Fatal("expected match for 'Feature 42 (controllers)'")
	}
	if len(ref.FeatureNums) != 1 || ref.FeatureNums[0] != 42 {
		t.Errorf("FeatureNums = %v, want [42]", ref.FeatureNums)
	}
	if ref.Label != "controllers" {
		t.Errorf("Label = %q, want 'controllers'", ref.Label)
	}
}

func TestParseCrossFeatureToken_SingleFeatureCaseInsensitive(t *testing.T) {
	for _, token := range []string{"FEATURE 7", "feature 7", "fEaTuRe 7"} {
		ref, ok := parseCrossFeatureToken(token)
		if !ok {
			t.Fatalf("expected match for %q", token)
		}
		if len(ref.FeatureNums) != 1 || ref.FeatureNums[0] != 7 {
			t.Errorf("%q: FeatureNums = %v, want [7]", token, ref.FeatureNums)
		}
	}
}

func TestParseCrossFeatureToken_RangeNoLabel(t *testing.T) {
	ref, ok := parseCrossFeatureToken("Features 3-5")
	if !ok {
		t.Fatal("expected match for 'Features 3-5'")
	}
	if len(ref.FeatureNums) != 3 || ref.FeatureNums[0] != 3 || ref.FeatureNums[1] != 4 || ref.FeatureNums[2] != 5 {
		t.Errorf("FeatureNums = %v, want [3 4 5]", ref.FeatureNums)
	}
	if ref.Label != "" {
		t.Errorf("Label = %q, want empty", ref.Label)
	}
}

func TestParseCrossFeatureToken_RangeWithLabel(t *testing.T) {
	ref, ok := parseCrossFeatureToken("Features 10-12 (UI layer)")
	if !ok {
		t.Fatal("expected match for 'Features 10-12 (UI layer)'")
	}
	if len(ref.FeatureNums) != 3 || ref.FeatureNums[0] != 10 || ref.FeatureNums[2] != 12 {
		t.Errorf("FeatureNums = %v, want [10 11 12]", ref.FeatureNums)
	}
	if ref.Label != "UI layer" {
		t.Errorf("Label = %q, want 'UI layer'", ref.Label)
	}
}

func TestParseCrossFeatureToken_SingleFeatureSameAsRange(t *testing.T) {
	// "Features 5-5" is a single-element range — allowed by the range regex.
	ref, ok := parseCrossFeatureToken("Features 5-5")
	if !ok {
		t.Fatal("expected match for 'Features 5-5'")
	}
	if len(ref.FeatureNums) != 1 || ref.FeatureNums[0] != 5 {
		t.Errorf("FeatureNums = %v, want [5]", ref.FeatureNums)
	}
}

func TestParseCrossFeatureToken_NotACrossFeatureRef(t *testing.T) {
	for _, token := range []string{"TASK-038-001", "BUG-002-001", "something-else", "", "TASK-001"} {
		_, ok := parseCrossFeatureToken(token)
		if ok {
			t.Errorf("expected no match for %q", token)
		}
	}
}

func TestParseFile_CrossFeaturePredecessors_SingleRef(t *testing.T) {
	const content = `# Feature 055: Test

### TASK-055-001: A task
**Predecessors:** Feature 5 (controllers)

**Acceptance Criteria:**
- [ ] Do it
`
	dir := t.TempDir()
	writeTempFeature(t, dir, "feature_055.md", content)
	tasks, err := ParseFile(filepath.Join(dir, ".maggus", "features", "feature_055.md"))
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	task := tasks[0]
	if len(task.Predecessors) != 0 {
		t.Errorf("Predecessors = %v, want empty (cross-feature ref must not appear here)", task.Predecessors)
	}
	if len(task.CrossFeaturePredecessors) != 1 {
		t.Fatalf("CrossFeaturePredecessors len = %d, want 1", len(task.CrossFeaturePredecessors))
	}
	ref := task.CrossFeaturePredecessors[0]
	if len(ref.FeatureNums) != 1 || ref.FeatureNums[0] != 5 {
		t.Errorf("FeatureNums = %v, want [5]", ref.FeatureNums)
	}
	if ref.Label != "controllers" {
		t.Errorf("Label = %q, want 'controllers'", ref.Label)
	}
}

func TestParseFile_CrossFeaturePredecessors_RangeRef(t *testing.T) {
	const content = `# Feature 055: Test

### TASK-055-001: A task
**Predecessors:** Features 3-5 (UI layer)

**Acceptance Criteria:**
- [ ] Do it
`
	dir := t.TempDir()
	writeTempFeature(t, dir, "feature_055.md", content)
	tasks, err := ParseFile(filepath.Join(dir, ".maggus", "features", "feature_055.md"))
	if err != nil {
		t.Fatal(err)
	}
	task := tasks[0]
	if len(task.Predecessors) != 0 {
		t.Errorf("Predecessors = %v, want empty", task.Predecessors)
	}
	if len(task.CrossFeaturePredecessors) != 1 {
		t.Fatalf("CrossFeaturePredecessors len = %d, want 1", len(task.CrossFeaturePredecessors))
	}
	ref := task.CrossFeaturePredecessors[0]
	if len(ref.FeatureNums) != 3 || ref.FeatureNums[0] != 3 || ref.FeatureNums[2] != 5 {
		t.Errorf("FeatureNums = %v, want [3 4 5]", ref.FeatureNums)
	}
	if ref.Label != "UI layer" {
		t.Errorf("Label = %q, want 'UI layer'", ref.Label)
	}
}

func TestParseFile_CrossFeaturePredecessors_MixedWithTaskID(t *testing.T) {
	const content = `# Feature 055: Test

### TASK-055-001: A task
**Predecessors:** TASK-054-001, Feature 42 (auth), TASK-054-003

**Acceptance Criteria:**
- [ ] Do it
`
	dir := t.TempDir()
	writeTempFeature(t, dir, "feature_055.md", content)
	tasks, err := ParseFile(filepath.Join(dir, ".maggus", "features", "feature_055.md"))
	if err != nil {
		t.Fatal(err)
	}
	task := tasks[0]
	if len(task.Predecessors) != 2 {
		t.Fatalf("Predecessors = %v, want [TASK-054-001, TASK-054-003]", task.Predecessors)
	}
	if task.Predecessors[0] != "TASK-054-001" || task.Predecessors[1] != "TASK-054-003" {
		t.Errorf("Predecessors = %v", task.Predecessors)
	}
	if len(task.CrossFeaturePredecessors) != 1 {
		t.Fatalf("CrossFeaturePredecessors len = %d, want 1", len(task.CrossFeaturePredecessors))
	}
	ref := task.CrossFeaturePredecessors[0]
	if len(ref.FeatureNums) != 1 || ref.FeatureNums[0] != 42 {
		t.Errorf("FeatureNums = %v, want [42]", ref.FeatureNums)
	}
	if ref.Label != "auth" {
		t.Errorf("Label = %q, want 'auth'", ref.Label)
	}
}

func TestParseFile_CrossFeaturePredecessors_AtStartMiddleEnd(t *testing.T) {
	const content = `# Feature 055: Test

### TASK-055-001: Cross-feature at start
**Predecessors:** Feature 1, TASK-054-002, TASK-054-003

**Acceptance Criteria:**
- [ ] Do it

### TASK-055-002: Cross-feature in middle
**Predecessors:** TASK-054-001, Feature 2, TASK-054-003

**Acceptance Criteria:**
- [ ] Do it

### TASK-055-003: Cross-feature at end
**Predecessors:** TASK-054-001, TASK-054-002, Feature 3

**Acceptance Criteria:**
- [ ] Do it
`
	dir := t.TempDir()
	writeTempFeature(t, dir, "feature_055.md", content)
	tasks, err := ParseFile(filepath.Join(dir, ".maggus", "features", "feature_055.md"))
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(tasks))
	}

	// Task 0: Feature at start
	t0 := tasks[0]
	if len(t0.CrossFeaturePredecessors) != 1 || t0.CrossFeaturePredecessors[0].FeatureNums[0] != 1 {
		t.Errorf("task 0: CrossFeaturePredecessors = %v", t0.CrossFeaturePredecessors)
	}
	if len(t0.Predecessors) != 2 || t0.Predecessors[0] != "TASK-054-002" || t0.Predecessors[1] != "TASK-054-003" {
		t.Errorf("task 0: Predecessors = %v", t0.Predecessors)
	}

	// Task 1: Feature in middle
	t1 := tasks[1]
	if len(t1.CrossFeaturePredecessors) != 1 || t1.CrossFeaturePredecessors[0].FeatureNums[0] != 2 {
		t.Errorf("task 1: CrossFeaturePredecessors = %v", t1.CrossFeaturePredecessors)
	}
	if len(t1.Predecessors) != 2 || t1.Predecessors[0] != "TASK-054-001" || t1.Predecessors[1] != "TASK-054-003" {
		t.Errorf("task 1: Predecessors = %v", t1.Predecessors)
	}

	// Task 2: Feature at end
	t2 := tasks[2]
	if len(t2.CrossFeaturePredecessors) != 1 || t2.CrossFeaturePredecessors[0].FeatureNums[0] != 3 {
		t.Errorf("task 2: CrossFeaturePredecessors = %v", t2.CrossFeaturePredecessors)
	}
	if len(t2.Predecessors) != 2 || t2.Predecessors[0] != "TASK-054-001" || t2.Predecessors[1] != "TASK-054-002" {
		t.Errorf("task 2: Predecessors = %v", t2.Predecessors)
	}
}

func TestParseFile_CrossFeaturePredecessors_NoneUnaffected(t *testing.T) {
	const content = `# Feature 055: Test

### TASK-055-001: None prefix
**Predecessors:** none (Feature 005 provides this, but it's independent)

**Acceptance Criteria:**
- [ ] Do it

### TASK-055-002: None plain
**Predecessors:** none

**Acceptance Criteria:**
- [ ] Do it
`
	dir := t.TempDir()
	writeTempFeature(t, dir, "feature_055.md", content)
	tasks, err := ParseFile(filepath.Join(dir, ".maggus", "features", "feature_055.md"))
	if err != nil {
		t.Fatal(err)
	}
	for i, task := range tasks {
		if len(task.Predecessors) != 0 {
			t.Errorf("task %d: Predecessors = %v, want empty", i, task.Predecessors)
		}
		if len(task.CrossFeaturePredecessors) != 0 {
			t.Errorf("task %d: CrossFeaturePredecessors = %v, want empty", i, task.CrossFeaturePredecessors)
		}
	}
}

func TestParseFile_CrossFeaturePredecessors_NormalTaskIDsUnaffected(t *testing.T) {
	const content = `# Feature 055: Test

### TASK-055-001: Normal task IDs
**Predecessors:** TASK-038-001, BUG-002-001, TASK-038-004

**Acceptance Criteria:**
- [ ] Do it
`
	dir := t.TempDir()
	writeTempFeature(t, dir, "feature_055.md", content)
	tasks, err := ParseFile(filepath.Join(dir, ".maggus", "features", "feature_055.md"))
	if err != nil {
		t.Fatal(err)
	}
	task := tasks[0]
	if len(task.CrossFeaturePredecessors) != 0 {
		t.Errorf("CrossFeaturePredecessors = %v, want empty", task.CrossFeaturePredecessors)
	}
	if len(task.Predecessors) != 3 {
		t.Fatalf("Predecessors = %v, want 3 entries", task.Predecessors)
	}
	if task.Predecessors[0] != "TASK-038-001" || task.Predecessors[1] != "BUG-002-001" || task.Predecessors[2] != "TASK-038-004" {
		t.Errorf("Predecessors = %v", task.Predecessors)
	}
}

func TestIsRunnable_WorkableWithSatisfiedPredecessor(t *testing.T) {
	task := Task{
		ID:           "T2",
		Predecessors: []string{"T1"},
		Criteria:     []Criterion{{Text: "Do it", Checked: false}},
	}
	if !task.IsRunnable(map[string]bool{"T1": true}, nil) {
		t.Error("expected true when workable and predecessor satisfied")
	}
}

func TestIsRunnable_WorkableWithUnsatisfiedPredecessor(t *testing.T) {
	task := Task{
		ID:           "T2",
		Predecessors: []string{"T1"},
		Criteria:     []Criterion{{Text: "Do it", Checked: false}},
	}
	// T1 is not in either non-nil map → unsatisfied.
	if task.IsRunnable(map[string]bool{}, map[string]bool{}) {
		t.Error("expected false when predecessor not in non-nil maps")
	}
	if task.IsRunnable(map[string]bool{"T9": true}, nil) {
		t.Error("expected false when predecessor not the one in completedIDs")
	}
}

func TestIsRunnable_DegradesToIsWorkableWhenNilMaps(t *testing.T) {
	// IsRunnable(nil, nil) must behave identically to IsWorkable() for all task
	// states, including tasks that have predecessors — this is the "graceful
	// degradation" guarantee used by the daemon pre-check.
	tasks := []Task{
		{ID: "A", Criteria: []Criterion{{Checked: false}}},                                                          // workable, no predecessors
		{ID: "B", Criteria: []Criterion{{Checked: true}}},                                                           // complete, no predecessors
		{ID: "C", Criteria: []Criterion{{Blocked: true, Checked: false}}},                                           // blocked, no predecessors
		{ID: "D", Criteria: []Criterion{{Skipped: true, Checked: false}}},                                           // skipped, no predecessors
		{ID: "E", Predecessors: []string{"A"}, Criteria: []Criterion{{Checked: false}}},                             // workable, with predecessor
		{ID: "F", Predecessors: []string{"A"}, Criteria: []Criterion{{Checked: true}}},                              // complete, with predecessor
		{ID: "G", Predecessors: []string{"A"}, Criteria: []Criterion{{Blocked: true, Checked: false}}},              // blocked, with predecessor
		{ID: "H", Predecessors: []string{"A", "B"}, Criteria: []Criterion{{Skipped: true, Checked: false}}},         // skipped, with multiple predecessors
	}
	for _, tc := range tasks {
		want := tc.IsWorkable()
		got := tc.IsRunnable(nil, nil)
		if got != want {
			t.Errorf("task %s: IsRunnable(nil,nil)=%v but IsWorkable()=%v", tc.ID, got, want)
		}
	}
}
