package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dirnei/maggus/internal/parser"
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

func TestRenderBlockedTaskDetail_ShowsPlanAndTitle(t *testing.T) {
	task := parser.Task{
		ID:          "TASK-042",
		Title:       "Fix the widget",
		Description: "We need to fix the widget because it is broken.",
		SourceFile:  "/some/path/.maggus/plan_6.md",
		Criteria: []parser.Criterion{
			{Text: "Widget is fixed", Checked: true},
			{Text: "BLOCKED: Needs API key from vendor", Checked: false, Blocked: true},
			{Text: "Tests pass", Checked: false},
		},
	}

	var buf bytes.Buffer
	renderBlockedTaskDetail(&buf, task, 80)
	out := buf.String()

	if !strings.Contains(out, "Plan: plan_6.md") {
		t.Errorf("expected plan filename, got:\n%s", out)
	}
	if !strings.Contains(out, "TASK-042") {
		t.Errorf("expected task ID, got:\n%s", out)
	}
	if !strings.Contains(out, "Fix the widget") {
		t.Errorf("expected task title, got:\n%s", out)
	}
	if !strings.Contains(out, "fix the widget because it is broken") {
		t.Errorf("expected description, got:\n%s", out)
	}
}

func TestRenderBlockedTaskDetail_CriteriaFormatting(t *testing.T) {
	task := parser.Task{
		ID:         "TASK-001",
		Title:      "Test task",
		SourceFile: "/path/.maggus/plan_1.md",
		Criteria: []parser.Criterion{
			{Text: "Done thing", Checked: true},
			{Text: "BLOCKED: waiting on API", Checked: false, Blocked: true},
			{Text: "Normal unchecked", Checked: false},
		},
	}

	var buf bytes.Buffer
	renderBlockedTaskDetail(&buf, task, 80)
	out := buf.String()

	// Completed criterion: green checkmark
	if !strings.Contains(out, colorGreen+"✓ Done thing"+colorReset) {
		t.Errorf("expected green checkmark for completed criterion, got:\n%s", out)
	}
	// Blocked criterion: red with >>> marker
	if !strings.Contains(out, colorRed+">>> ⚠ BLOCKED: waiting on API"+colorReset) {
		t.Errorf("expected red blocked criterion with markers, got:\n%s", out)
	}
	// Normal unchecked: circle marker, no color codes
	if !strings.Contains(out, "○ Normal unchecked") {
		t.Errorf("expected normal unchecked criterion with circle, got:\n%s", out)
	}
}

func TestRenderBlockedTaskDetail_NoDescription(t *testing.T) {
	task := parser.Task{
		ID:         "TASK-001",
		Title:      "No desc task",
		SourceFile: "/path/.maggus/plan_1.md",
		Criteria: []parser.Criterion{
			{Text: "BLOCKED: something", Checked: false, Blocked: true},
		},
	}

	var buf bytes.Buffer
	renderBlockedTaskDetail(&buf, task, 80)
	out := buf.String()

	if !strings.Contains(out, "TASK-001") {
		t.Errorf("expected task ID, got:\n%s", out)
	}
	if !strings.Contains(out, "Acceptance Criteria:") {
		t.Errorf("expected acceptance criteria header, got:\n%s", out)
	}
}

func TestRenderBlockedTaskDetail_DisplaysInRunBlocked(t *testing.T) {
	dir := t.TempDir()
	writeBlockedPlan(t, dir, "plan_5.md", `# Plan: Five

## User Stories

### TASK-050: Blocked feature
**Description:** This feature is blocked.

**Acceptance Criteria:**
- [x] Step one done
- [ ] BLOCKED: Needs external service
- [ ] Step three pending
`)
	out := runBlockedCmd(t, dir)

	if !strings.Contains(out, "Found 1 blocked task(s).") {
		t.Errorf("expected found message, got:\n%s", out)
	}
	if !strings.Contains(out, "Plan: plan_5.md") {
		t.Errorf("expected plan filename in detail view, got:\n%s", out)
	}
	if !strings.Contains(out, "TASK-050") {
		t.Errorf("expected task ID in detail view, got:\n%s", out)
	}
	if !strings.Contains(out, "Blocked feature") {
		t.Errorf("expected task title in detail view, got:\n%s", out)
	}
	if !strings.Contains(out, "Acceptance Criteria:") {
		t.Errorf("expected criteria section, got:\n%s", out)
	}
}

func TestWrapText(t *testing.T) {
	// Short text should not wrap
	result := wrapText("hello", 80, "  ")
	if result != "  hello" {
		t.Errorf("expected '  hello', got %q", result)
	}

	// Long text should wrap
	long := "this is a long line that should be wrapped at word boundaries when it exceeds the maximum width"
	result = wrapText(long, 40, "  ")
	lines := strings.Split(result, "\n")
	if len(lines) < 2 {
		t.Errorf("expected multiple lines, got %d: %q", len(lines), result)
	}
	for _, line := range lines {
		if len(line) > 40 {
			t.Errorf("line exceeds max width 40: %q (len=%d)", line, len(line))
		}
	}
}
