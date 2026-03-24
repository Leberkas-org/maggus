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

	if _, err := MarkCompletedFeatures(dir, ""); err != nil {
		t.Fatalf("MarkCompletedFeatures error: %v", err)
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

	if _, err := MarkCompletedBugs(dir, ""); err != nil {
		t.Fatalf("MarkCompletedBugs error: %v", err)
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
