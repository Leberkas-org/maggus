package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeBlockedPlan(t *testing.T, dir, filename, content string) {
	t.Helper()
	maggusDir := filepath.Join(dir, ".maggus")
	if err := os.MkdirAll(maggusDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(maggusDir, filename), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func runBlockedCmd(t *testing.T, dir string) string {
	t.Helper()
	var buf bytes.Buffer
	cmd := *blockedCmd
	cmd.SetOut(&buf)
	if err := runBlocked(&cmd, dir); err != nil {
		t.Fatalf("runBlocked: %v", err)
	}
	return buf.String()
}

func TestBlockedNoMaggusDir(t *testing.T) {
	dir := t.TempDir()
	out := runBlockedCmd(t, dir)
	if !strings.Contains(out, "No blocked tasks found.") {
		t.Errorf("expected 'No blocked tasks found.' got:\n%s", out)
	}
}

func TestBlockedNoBlockedTasks(t *testing.T) {
	dir := t.TempDir()
	writeBlockedPlan(t, dir, "plan_1.md", `# Plan: Test

## User Stories

### TASK-001: Normal task
**Description:** Nothing blocked here.

**Acceptance Criteria:**
- [ ] Criterion A
- [x] Criterion B
`)
	out := runBlockedCmd(t, dir)
	if !strings.Contains(out, "No blocked tasks found.") {
		t.Errorf("expected 'No blocked tasks found.' got:\n%s", out)
	}
}

func TestBlockedFindsBlockedTasks(t *testing.T) {
	dir := t.TempDir()
	writeBlockedPlan(t, dir, "plan_1.md", `# Plan: Test

## User Stories

### TASK-001: Blocked task
**Description:** Has a blocker.

**Acceptance Criteria:**
- [ ] BLOCKED: waiting on API
- [ ] Normal criterion
`)
	out := runBlockedCmd(t, dir)
	if !strings.Contains(out, "Found 1 blocked task(s).") {
		t.Errorf("expected 'Found 1 blocked task(s).' got:\n%s", out)
	}
}

func TestBlockedMultipleTasks(t *testing.T) {
	dir := t.TempDir()
	writeBlockedPlan(t, dir, "plan_1.md", `# Plan: Test

## User Stories

### TASK-001: First blocked
**Description:** Blocked one.

**Acceptance Criteria:**
- [ ] BLOCKED: reason A

### TASK-002: Second blocked
**Description:** Blocked two.

**Acceptance Criteria:**
- [ ] BLOCKED: reason B

### TASK-003: Not blocked
**Description:** Fine.

**Acceptance Criteria:**
- [ ] Criterion C
`)
	out := runBlockedCmd(t, dir)
	if !strings.Contains(out, "Found 2 blocked task(s).") {
		t.Errorf("expected 'Found 2 blocked task(s).' got:\n%s", out)
	}
}

func TestBlockedSkipsCompletedPlans(t *testing.T) {
	dir := t.TempDir()
	writeBlockedPlan(t, dir, "plan_1_completed.md", `# Plan: Done

## User Stories

### TASK-001: Blocked in completed plan
**Description:** Should be skipped.

**Acceptance Criteria:**
- [ ] BLOCKED: old blocker
`)
	out := runBlockedCmd(t, dir)
	if !strings.Contains(out, "No blocked tasks found.") {
		t.Errorf("expected 'No blocked tasks found.' for completed plan, got:\n%s", out)
	}
}

func TestBlockedCheckedBlockedNotCounted(t *testing.T) {
	dir := t.TempDir()
	// A checked BLOCKED criterion means it was resolved — task should not be blocked
	writeBlockedPlan(t, dir, "plan_1.md", `# Plan: Test

## User Stories

### TASK-001: Resolved blocked
**Description:** Was blocked but resolved.

**Acceptance Criteria:**
- [x] BLOCKED: was waiting on API
- [x] Other done
`)
	out := runBlockedCmd(t, dir)
	if !strings.Contains(out, "No blocked tasks found.") {
		t.Errorf("expected 'No blocked tasks found.' for resolved blocked criterion, got:\n%s", out)
	}
}

func TestBlockedHelpDescription(t *testing.T) {
	if blockedCmd.Short == "" {
		t.Error("blockedCmd.Short should not be empty")
	}
	if blockedCmd.Long == "" {
		t.Error("blockedCmd.Long should not be empty")
	}
}

func TestCollectBlockedTasksRetainsSourceFile(t *testing.T) {
	dir := t.TempDir()
	writeBlockedPlan(t, dir, "plan_2.md", `# Plan: Two

## User Stories

### TASK-010: Blocked in plan two
**Description:** Has a blocker.

**Acceptance Criteria:**
- [ ] BLOCKED: needs dependency
`)
	blocked, err := collectBlockedTasks(dir)
	if err != nil {
		t.Fatalf("collectBlockedTasks: %v", err)
	}
	if len(blocked) != 1 {
		t.Fatalf("expected 1 blocked task, got %d", len(blocked))
	}
	expectedFile := filepath.Join(dir, ".maggus", "plan_2.md")
	if blocked[0].SourceFile != expectedFile {
		t.Errorf("expected SourceFile %q, got %q", expectedFile, blocked[0].SourceFile)
	}
}

func TestCollectBlockedTasksOrderedByPlanThenDocument(t *testing.T) {
	dir := t.TempDir()
	writeBlockedPlan(t, dir, "plan_1.md", `# Plan: One

## User Stories

### TASK-002: Second in plan one
**Description:** Blocked.

**Acceptance Criteria:**
- [ ] BLOCKED: reason B

### TASK-001: First in plan one
**Description:** Blocked.

**Acceptance Criteria:**
- [ ] BLOCKED: reason A
`)
	writeBlockedPlan(t, dir, "plan_2.md", `# Plan: Two

## User Stories

### TASK-003: In plan two
**Description:** Blocked.

**Acceptance Criteria:**
- [ ] BLOCKED: reason C
`)
	blocked, err := collectBlockedTasks(dir)
	if err != nil {
		t.Fatalf("collectBlockedTasks: %v", err)
	}
	if len(blocked) != 3 {
		t.Fatalf("expected 3 blocked tasks, got %d", len(blocked))
	}
	// plan_1.md tasks come first in document order, then plan_2.md
	if blocked[0].ID != "TASK-002" {
		t.Errorf("expected first blocked task TASK-002, got %s", blocked[0].ID)
	}
	if blocked[1].ID != "TASK-001" {
		t.Errorf("expected second blocked task TASK-001, got %s", blocked[1].ID)
	}
	if blocked[2].ID != "TASK-003" {
		t.Errorf("expected third blocked task TASK-003, got %s", blocked[2].ID)
	}
}

func TestCollectBlockedTasksMultiplePlansSourceFiles(t *testing.T) {
	dir := t.TempDir()
	writeBlockedPlan(t, dir, "plan_1.md", `# Plan: One

## User Stories

### TASK-001: Blocked in one
**Description:** Blocked.

**Acceptance Criteria:**
- [ ] BLOCKED: reason
`)
	writeBlockedPlan(t, dir, "plan_3.md", `# Plan: Three

## User Stories

### TASK-005: Blocked in three
**Description:** Blocked.

**Acceptance Criteria:**
- [ ] BLOCKED: reason
`)
	blocked, err := collectBlockedTasks(dir)
	if err != nil {
		t.Fatalf("collectBlockedTasks: %v", err)
	}
	if len(blocked) != 2 {
		t.Fatalf("expected 2 blocked tasks, got %d", len(blocked))
	}
	plan1 := filepath.Join(dir, ".maggus", "plan_1.md")
	plan3 := filepath.Join(dir, ".maggus", "plan_3.md")
	if blocked[0].SourceFile != plan1 {
		t.Errorf("first task SourceFile: expected %q, got %q", plan1, blocked[0].SourceFile)
	}
	if blocked[1].SourceFile != plan3 {
		t.Errorf("second task SourceFile: expected %q, got %q", plan3, blocked[1].SourceFile)
	}
}

func TestCollectBlockedTasksEmpty(t *testing.T) {
	dir := t.TempDir()
	// No .maggus dir at all
	blocked, err := collectBlockedTasks(dir)
	if err != nil {
		t.Fatalf("collectBlockedTasks: %v", err)
	}
	if len(blocked) != 0 {
		t.Errorf("expected 0 blocked tasks, got %d", len(blocked))
	}
}
