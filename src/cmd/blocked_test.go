package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
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

// mockActionPicker replaces runActionPicker for tests, returning actions from a queue.
func mockActionPicker(actions []blockedAction) func() {
	i := 0
	orig := runActionPicker
	runActionPicker = func(_ parser.Criterion) (blockedAction, error) {
		if i >= len(actions) {
			return actionSkip, nil
		}
		a := actions[i]
		i++
		return a, nil
	}
	return func() { runActionPicker = orig }
}

func runBlockedCmd(t *testing.T, dir string) string {
	t.Helper()
	restore := mockActionPicker([]blockedAction{actionSkip})
	defer restore()
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

func TestActionPickerUnblock(t *testing.T) {
	dir := t.TempDir()
	writeBlockedPlan(t, dir, "plan_1.md", `# Plan: Test

## User Stories

### TASK-001: Blocked task
**Description:** Has a blocker.

**Acceptance Criteria:**
- [ ] BLOCKED: waiting on API
`)
	restore := mockActionPicker([]blockedAction{actionUnblock})
	defer restore()
	var buf bytes.Buffer
	cmd := *blockedCmd
	cmd.SetOut(&buf)
	if err := runBlocked(&cmd, dir); err != nil {
		t.Fatalf("runBlocked: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Unblocked:") {
		t.Errorf("expected 'Unblocked:' in output, got:\n%s", out)
	}
}

func TestActionPickerResolve(t *testing.T) {
	dir := t.TempDir()
	writeBlockedPlan(t, dir, "plan_1.md", `# Plan: Test

## User Stories

### TASK-001: Blocked task
**Description:** Has a blocker.

**Acceptance Criteria:**
- [ ] BLOCKED: waiting on API
`)
	restore := mockActionPicker([]blockedAction{actionResolve})
	defer restore()
	var buf bytes.Buffer
	cmd := *blockedCmd
	cmd.SetOut(&buf)
	if err := runBlocked(&cmd, dir); err != nil {
		t.Fatalf("runBlocked: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Resolved:") {
		t.Errorf("expected 'Resolved:' in output, got:\n%s", out)
	}
}

func TestActionPickerSkip(t *testing.T) {
	dir := t.TempDir()
	writeBlockedPlan(t, dir, "plan_1.md", `# Plan: Test

## User Stories

### TASK-001: Blocked task
**Description:** Has a blocker.

**Acceptance Criteria:**
- [ ] BLOCKED: waiting on API
`)
	restore := mockActionPicker([]blockedAction{actionSkip})
	defer restore()
	var buf bytes.Buffer
	cmd := *blockedCmd
	cmd.SetOut(&buf)
	if err := runBlocked(&cmd, dir); err != nil {
		t.Fatalf("runBlocked: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Skipped:") {
		t.Errorf("expected 'Skipped:' in output, got:\n%s", out)
	}
}

func TestActionPickerAbort(t *testing.T) {
	dir := t.TempDir()
	writeBlockedPlan(t, dir, "plan_1.md", `# Plan: Test

## User Stories

### TASK-001: Blocked task
**Description:** Has two blockers.

**Acceptance Criteria:**
- [ ] BLOCKED: reason A
- [ ] BLOCKED: reason B
`)
	// Abort on first criterion — second should not be reached
	restore := mockActionPicker([]blockedAction{actionAbort})
	defer restore()
	var buf bytes.Buffer
	cmd := *blockedCmd
	cmd.SetOut(&buf)
	if err := runBlocked(&cmd, dir); err != nil {
		t.Fatalf("runBlocked: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Aborted.") {
		t.Errorf("expected 'Aborted.' in output, got:\n%s", out)
	}
	// Should not contain any unblock/resolve/skip messages
	if strings.Contains(out, "Unblocked:") || strings.Contains(out, "Resolved:") || strings.Contains(out, "Skipped:") {
		t.Errorf("abort should not process further criteria, got:\n%s", out)
	}
}

func TestActionPickerAbortPreservesEarlierActions(t *testing.T) {
	dir := t.TempDir()
	writeBlockedPlan(t, dir, "plan_1.md", `# Plan: Test

## User Stories

### TASK-001: Blocked task
**Description:** Has two blockers.

**Acceptance Criteria:**
- [ ] BLOCKED: reason A
- [ ] BLOCKED: reason B
`)
	// Unblock first, abort on second
	restore := mockActionPicker([]blockedAction{actionUnblock, actionAbort})
	defer restore()
	var buf bytes.Buffer
	cmd := *blockedCmd
	cmd.SetOut(&buf)
	if err := runBlocked(&cmd, dir); err != nil {
		t.Fatalf("runBlocked: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Unblocked:") {
		t.Errorf("expected first action 'Unblocked:' to be preserved, got:\n%s", out)
	}
	if !strings.Contains(out, "Aborted.") {
		t.Errorf("expected 'Aborted.' in output, got:\n%s", out)
	}
}

func TestActionPickerModelView(t *testing.T) {
	c := parser.Criterion{Text: "BLOCKED: waiting on API", Blocked: true}
	m := newActionPickerModel(c)
	view := m.View()
	if !strings.Contains(view, "BLOCKED: waiting on API") {
		t.Errorf("expected criterion text in view, got:\n%s", view)
	}
	if !strings.Contains(view, "Unblock") {
		t.Errorf("expected 'Unblock' in view, got:\n%s", view)
	}
	if !strings.Contains(view, "Resolve") {
		t.Errorf("expected 'Resolve' in view, got:\n%s", view)
	}
	if !strings.Contains(view, "Skip") {
		t.Errorf("expected 'Skip' in view, got:\n%s", view)
	}
	if !strings.Contains(view, "Abort") {
		t.Errorf("expected 'Abort' in view, got:\n%s", view)
	}
}

func TestActionPickerModelNavigation(t *testing.T) {
	c := parser.Criterion{Text: "BLOCKED: test", Blocked: true}
	m := newActionPickerModel(c)
	// Initial cursor should be at 0 (Unblock)
	if m.cursor != 0 {
		t.Errorf("expected initial cursor 0, got %d", m.cursor)
	}
	// Move down
	updated, _ := m.Update(tea.KeyMsg(tea.Key{Type: tea.KeyRunes, Runes: []rune("j")}))
	m = updated.(actionPickerModel)
	if m.cursor != 1 {
		t.Errorf("expected cursor 1 after down, got %d", m.cursor)
	}
	// Move up
	updated, _ = m.Update(tea.KeyMsg(tea.Key{Type: tea.KeyRunes, Runes: []rune("k")}))
	m = updated.(actionPickerModel)
	if m.cursor != 0 {
		t.Errorf("expected cursor 0 after up, got %d", m.cursor)
	}
	// Can't go above 0
	updated, _ = m.Update(tea.KeyMsg(tea.Key{Type: tea.KeyRunes, Runes: []rune("k")}))
	m = updated.(actionPickerModel)
	if m.cursor != 0 {
		t.Errorf("expected cursor to stay at 0, got %d", m.cursor)
	}
}

func TestActionPickerModelEnterSelects(t *testing.T) {
	c := parser.Criterion{Text: "BLOCKED: test", Blocked: true}
	m := newActionPickerModel(c)
	// Move to Resolve (index 1) and press enter
	updated, _ := m.Update(tea.KeyMsg(tea.Key{Type: tea.KeyRunes, Runes: []rune("j")}))
	m = updated.(actionPickerModel)
	updated, _ = m.Update(tea.KeyMsg(tea.Key{Type: tea.KeyEnter}))
	m = updated.(actionPickerModel)
	if !m.done {
		t.Error("expected done to be true after enter")
	}
	if m.chosen != actionResolve {
		t.Errorf("expected actionResolve, got %v", m.chosen)
	}
}

func TestActionPickerModelCtrlCAborts(t *testing.T) {
	c := parser.Criterion{Text: "BLOCKED: test", Blocked: true}
	m := newActionPickerModel(c)
	updated, _ := m.Update(tea.KeyMsg(tea.Key{Type: tea.KeyCtrlC}))
	m = updated.(actionPickerModel)
	if !m.done {
		t.Error("expected done to be true after ctrl+c")
	}
	if m.chosen != actionAbort {
		t.Errorf("expected actionAbort, got %v", m.chosen)
	}
}

// --- File modification tests (TASK-005) ---

func TestUnblockCriterion(t *testing.T) {
	dir := t.TempDir()
	planContent := `# Plan: Test

## User Stories

### TASK-001: Blocked task
**Description:** Has a blocker.

**Acceptance Criteria:**
- [x] Step one done
- [ ] BLOCKED: waiting on API
- [ ] Normal criterion
`
	writeBlockedPlan(t, dir, "plan_1.md", planContent)
	planPath := filepath.Join(dir, ".maggus", "plan_1.md")

	c := parser.Criterion{Text: "BLOCKED: waiting on API", Checked: false, Blocked: true}
	if err := unblockCriterion(planPath, c); err != nil {
		t.Fatalf("unblockCriterion: %v", err)
	}

	data, err := os.ReadFile(planPath)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	content := string(data)

	// Should have removed BLOCKED: prefix
	if !strings.Contains(content, "- [ ] waiting on API") {
		t.Errorf("expected unblocked criterion, got:\n%s", content)
	}
	// Should not contain the old blocked line
	if strings.Contains(content, "- [ ] BLOCKED: waiting on API") {
		t.Errorf("old blocked line should be removed, got:\n%s", content)
	}
	// Other lines should be preserved
	if !strings.Contains(content, "- [x] Step one done") {
		t.Errorf("completed criterion should be preserved, got:\n%s", content)
	}
	if !strings.Contains(content, "- [ ] Normal criterion") {
		t.Errorf("normal criterion should be preserved, got:\n%s", content)
	}
}

func TestUnblockCriterionWithEmojiPrefix(t *testing.T) {
	dir := t.TempDir()
	planContent := `# Plan: Test

## User Stories

### TASK-001: Blocked task
**Description:** Has a blocker.

**Acceptance Criteria:**
- [ ] ⚠️ BLOCKED: needs external service
`
	writeBlockedPlan(t, dir, "plan_1.md", planContent)
	planPath := filepath.Join(dir, ".maggus", "plan_1.md")

	c := parser.Criterion{Text: "⚠️ BLOCKED: needs external service", Checked: false, Blocked: true}
	if err := unblockCriterion(planPath, c); err != nil {
		t.Fatalf("unblockCriterion: %v", err)
	}

	data, _ := os.ReadFile(planPath)
	content := string(data)
	if !strings.Contains(content, "- [ ] needs external service") {
		t.Errorf("expected unblocked criterion without emoji prefix, got:\n%s", content)
	}
}

func TestResolveCriterion(t *testing.T) {
	dir := t.TempDir()
	planContent := `# Plan: Test

## User Stories

### TASK-001: Blocked task
**Description:** Has a blocker.

**Acceptance Criteria:**
- [x] Step one done
- [ ] BLOCKED: waiting on API
- [ ] Normal criterion
`
	writeBlockedPlan(t, dir, "plan_1.md", planContent)
	planPath := filepath.Join(dir, ".maggus", "plan_1.md")

	c := parser.Criterion{Text: "BLOCKED: waiting on API", Checked: false, Blocked: true}
	if err := resolveCriterion(planPath, c); err != nil {
		t.Fatalf("resolveCriterion: %v", err)
	}

	data, err := os.ReadFile(planPath)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	content := string(data)

	// The blocked line should be completely gone
	if strings.Contains(content, "BLOCKED: waiting on API") {
		t.Errorf("blocked line should be deleted, got:\n%s", content)
	}
	// Other lines should be preserved
	if !strings.Contains(content, "- [x] Step one done") {
		t.Errorf("completed criterion should be preserved, got:\n%s", content)
	}
	if !strings.Contains(content, "- [ ] Normal criterion") {
		t.Errorf("normal criterion should be preserved, got:\n%s", content)
	}
}

func TestUnblockCriterionNotFound(t *testing.T) {
	dir := t.TempDir()
	planContent := `# Plan: Test

## User Stories

### TASK-001: Task
**Acceptance Criteria:**
- [ ] Something else
`
	writeBlockedPlan(t, dir, "plan_1.md", planContent)
	planPath := filepath.Join(dir, ".maggus", "plan_1.md")

	c := parser.Criterion{Text: "BLOCKED: nonexistent criterion", Checked: false, Blocked: true}
	err := unblockCriterion(planPath, c)
	if err == nil {
		t.Fatal("expected error for missing criterion line")
	}
	if !strings.Contains(err.Error(), "criterion line not found") {
		t.Errorf("expected 'criterion line not found' error, got: %v", err)
	}
}

func TestResolveCriterionNotFound(t *testing.T) {
	dir := t.TempDir()
	planContent := `# Plan: Test

## User Stories

### TASK-001: Task
**Acceptance Criteria:**
- [ ] Something else
`
	writeBlockedPlan(t, dir, "plan_1.md", planContent)
	planPath := filepath.Join(dir, ".maggus", "plan_1.md")

	c := parser.Criterion{Text: "BLOCKED: nonexistent criterion", Checked: false, Blocked: true}
	err := resolveCriterion(planPath, c)
	if err == nil {
		t.Fatal("expected error for missing criterion line")
	}
	if !strings.Contains(err.Error(), "criterion line not found") {
		t.Errorf("expected 'criterion line not found' error, got: %v", err)
	}
}

func TestUnblockCriterionStillParseable(t *testing.T) {
	dir := t.TempDir()
	planContent := `# Plan: Test

## User Stories

### TASK-001: Blocked task
**Description:** Has a blocker.

**Acceptance Criteria:**
- [ ] BLOCKED: waiting on API
- [ ] Normal criterion
`
	writeBlockedPlan(t, dir, "plan_1.md", planContent)
	planPath := filepath.Join(dir, ".maggus", "plan_1.md")

	c := parser.Criterion{Text: "BLOCKED: waiting on API", Checked: false, Blocked: true}
	if err := unblockCriterion(planPath, c); err != nil {
		t.Fatalf("unblockCriterion: %v", err)
	}

	// The modified file should still be parseable
	tasks, err := parser.ParseFile(planPath)
	if err != nil {
		t.Fatalf("ParseFile after unblock: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	// The formerly blocked criterion should now be unblocked
	found := false
	for _, cr := range tasks[0].Criteria {
		if cr.Text == "waiting on API" {
			found = true
			if cr.Blocked {
				t.Error("criterion should no longer be blocked")
			}
			if cr.Checked {
				t.Error("criterion should still be unchecked")
			}
		}
	}
	if !found {
		t.Error("expected to find 'waiting on API' criterion after unblock")
	}
}

func TestResolveCriterionStillParseable(t *testing.T) {
	dir := t.TempDir()
	planContent := `# Plan: Test

## User Stories

### TASK-001: Blocked task
**Description:** Has a blocker.

**Acceptance Criteria:**
- [ ] BLOCKED: waiting on API
- [ ] Normal criterion
`
	writeBlockedPlan(t, dir, "plan_1.md", planContent)
	planPath := filepath.Join(dir, ".maggus", "plan_1.md")

	c := parser.Criterion{Text: "BLOCKED: waiting on API", Checked: false, Blocked: true}
	if err := resolveCriterion(planPath, c); err != nil {
		t.Fatalf("resolveCriterion: %v", err)
	}

	// The modified file should still be parseable
	tasks, err := parser.ParseFile(planPath)
	if err != nil {
		t.Fatalf("ParseFile after resolve: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	// Should only have the normal criterion left
	if len(tasks[0].Criteria) != 1 {
		t.Fatalf("expected 1 criterion after resolve, got %d", len(tasks[0].Criteria))
	}
	if tasks[0].Criteria[0].Text != "Normal criterion" {
		t.Errorf("expected 'Normal criterion', got %q", tasks[0].Criteria[0].Text)
	}
}

func TestRunBlockedUnblockModifiesFile(t *testing.T) {
	dir := t.TempDir()
	writeBlockedPlan(t, dir, "plan_1.md", `# Plan: Test

## User Stories

### TASK-001: Blocked task
**Description:** Has a blocker.

**Acceptance Criteria:**
- [ ] BLOCKED: waiting on API
`)
	restore := mockActionPicker([]blockedAction{actionUnblock})
	defer restore()
	var buf bytes.Buffer
	cmd := *blockedCmd
	cmd.SetOut(&buf)
	if err := runBlocked(&cmd, dir); err != nil {
		t.Fatalf("runBlocked: %v", err)
	}

	// Verify the file was actually modified
	data, _ := os.ReadFile(filepath.Join(dir, ".maggus", "plan_1.md"))
	content := string(data)
	if strings.Contains(content, "BLOCKED:") {
		t.Errorf("file should no longer contain BLOCKED:, got:\n%s", content)
	}
	if !strings.Contains(content, "- [ ] waiting on API") {
		t.Errorf("file should contain unblocked criterion, got:\n%s", content)
	}
}

func TestRunBlockedResolveModifiesFile(t *testing.T) {
	dir := t.TempDir()
	writeBlockedPlan(t, dir, "plan_1.md", `# Plan: Test

## User Stories

### TASK-001: Blocked task
**Description:** Has a blocker.

**Acceptance Criteria:**
- [ ] BLOCKED: waiting on API
- [ ] Normal criterion
`)
	restore := mockActionPicker([]blockedAction{actionResolve})
	defer restore()
	var buf bytes.Buffer
	cmd := *blockedCmd
	cmd.SetOut(&buf)
	if err := runBlocked(&cmd, dir); err != nil {
		t.Fatalf("runBlocked: %v", err)
	}

	// Verify the file was actually modified
	data, _ := os.ReadFile(filepath.Join(dir, ".maggus", "plan_1.md"))
	content := string(data)
	if strings.Contains(content, "BLOCKED: waiting on API") {
		t.Errorf("blocked line should be deleted, got:\n%s", content)
	}
	if !strings.Contains(content, "- [ ] Normal criterion") {
		t.Errorf("normal criterion should be preserved, got:\n%s", content)
	}
}

func TestRunBlockedErrorOnMissingLine(t *testing.T) {
	dir := t.TempDir()
	writeBlockedPlan(t, dir, "plan_1.md", `# Plan: Test

## User Stories

### TASK-001: Blocked task
**Description:** Has a blocker.

**Acceptance Criteria:**
- [ ] BLOCKED: waiting on API
`)
	planPath := filepath.Join(dir, ".maggus", "plan_1.md")

	// Simulate concurrent edit: modify file before the action runs
	// We'll manually remove the blocked line first
	data, _ := os.ReadFile(planPath)
	modified := strings.Replace(string(data), "- [ ] BLOCKED: waiting on API", "- [ ] something else", 1)
	os.WriteFile(planPath, []byte(modified), 0o644)

	// Now run with unblock — it should fail to find the line and show error
	restore := mockActionPicker([]blockedAction{actionUnblock})
	defer restore()
	var buf bytes.Buffer
	cmd := *blockedCmd
	cmd.SetOut(&buf)
	// runBlocked collects blocked tasks first, which will find the task
	// but the file has been changed, so unblockCriterion will fail.
	// We need to test the error path differently — inject a task directly.
	// Actually, since collectBlockedTasks re-parses, the task won't be blocked anymore.
	// Let's test unblockCriterion directly instead.
	c := parser.Criterion{Text: "BLOCKED: waiting on API", Checked: false, Blocked: true}
	err := unblockCriterion(planPath, c)
	if err == nil {
		t.Fatal("expected error for concurrent edit")
	}
	if !strings.Contains(err.Error(), "criterion line not found") {
		t.Errorf("expected 'criterion line not found' error, got: %v", err)
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
