package cmd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leberkas-org/maggus/internal/parser"
)

const listTestPlan = `# Plan: Test

## User Stories

### TASK-001: First task
**Description:** Do thing one.

**Acceptance Criteria:**
- [ ] Criterion A

### TASK-002: Second task
**Description:** Do thing two.

**Acceptance Criteria:**
- [ ] Criterion B

### TASK-003: Third task
**Description:** Do thing three.

**Acceptance Criteria:**
- [ ] Criterion C

### TASK-004: Fourth task
**Description:** Do thing four.

**Acceptance Criteria:**
- [ ] Criterion D

### TASK-005: Fifth task
**Description:** Do thing five.

**Acceptance Criteria:**
- [ ] Criterion E

### TASK-006: Sixth task
**Description:** Do thing six.

**Acceptance Criteria:**
- [ ] Criterion F

### TASK-007: Completed task
**Description:** Already done.

**Acceptance Criteria:**
- [x] Done A
`

func writeListPlan(t *testing.T, dir, filename, content string) {
	t.Helper()
	maggusDir := filepath.Join(dir, ".maggus")
	if err := os.MkdirAll(maggusDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(maggusDir, filename), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func runListCmd(t *testing.T, dir string, flags ...string) string {
	t.Helper()
	var buf bytes.Buffer
	cmd := *listCmd
	cmd.ResetFlags()
	cmd.Flags().IntP("count", "c", 5, "Number of tasks to show")
	cmd.Flags().Bool("plain", false, "Plain output")
	cmd.Flags().Bool("all", false, "Show all")

	// Always add --plain for tests to avoid launching bubbletea
	allFlags := append([]string{"--plain"}, flags...)
	if err := cmd.ParseFlags(allFlags); err != nil {
		t.Fatalf("parse flags: %v", err)
	}
	cmd.SetOut(&buf)

	plain, _ := cmd.Flags().GetBool("plain")
	all, _ := cmd.Flags().GetBool("all")
	count, _ := cmd.Flags().GetInt("count")

	if err := runList(&cmd, dir, plain, all, count); err != nil {
		t.Fatalf("runList: %v", err)
	}
	return buf.String()
}

func TestListDefaultCount(t *testing.T) {
	dir := t.TempDir()
	writeListPlan(t, dir, "plan_1.md", listTestPlan)

	out := runListCmd(t, dir)

	if !strings.Contains(out, "Next 5 task(s):") {
		t.Errorf("expected 'Next 5 task(s):' in output, got:\n%s", out)
	}

	for _, id := range []string{"TASK-001", "TASK-002", "TASK-003", "TASK-004", "TASK-005"} {
		if !strings.Contains(out, id) {
			t.Errorf("expected %s in output, got:\n%s", id, out)
		}
	}

	if strings.Contains(out, "TASK-006") {
		t.Errorf("expected TASK-006 NOT in output (count=5), got:\n%s", out)
	}

	if strings.Contains(out, "TASK-007") {
		t.Errorf("expected TASK-007 NOT in output (completed), got:\n%s", out)
	}
}

func TestListNoDescriptionLine(t *testing.T) {
	dir := t.TempDir()
	writeListPlan(t, dir, "plan_1.md", listTestPlan)

	out := runListCmd(t, dir)

	if strings.Contains(out, "Do thing") {
		t.Errorf("expected no description lines in output, got:\n%s", out)
	}
}

func TestListNoBlankLinesBetweenTasks(t *testing.T) {
	dir := t.TempDir()
	writeListPlan(t, dir, "plan_1.md", listTestPlan)

	out := runListCmd(t, dir)

	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	inTasks := false
	for _, line := range lines {
		if strings.Contains(line, "TASK-001") {
			inTasks = true
		}
		if inTasks && line == "" {
			t.Errorf("found blank line between tasks:\n%s", out)
			break
		}
	}
}

func TestListAllFlag(t *testing.T) {
	dir := t.TempDir()
	writeListPlan(t, dir, "plan_1.md", listTestPlan)

	out := runListCmd(t, dir, "--all")

	if !strings.Contains(out, "All upcoming tasks:") {
		t.Errorf("expected 'All upcoming tasks:' in output, got:\n%s", out)
	}

	for _, id := range []string{"TASK-001", "TASK-002", "TASK-003", "TASK-004", "TASK-005", "TASK-006"} {
		if !strings.Contains(out, id) {
			t.Errorf("expected %s in output, got:\n%s", id, out)
		}
	}
	if strings.Contains(out, "TASK-007") {
		t.Errorf("expected completed TASK-007 NOT in output, got:\n%s", out)
	}
}

func TestListAllIgnoresCount(t *testing.T) {
	dir := t.TempDir()
	writeListPlan(t, dir, "plan_1.md", listTestPlan)

	out := runListCmd(t, dir, "--all", "--count", "2")

	for _, id := range []string{"TASK-001", "TASK-002", "TASK-003", "TASK-004", "TASK-005", "TASK-006"} {
		if !strings.Contains(out, id) {
			t.Errorf("expected %s in output with --all, got:\n%s", id, out)
		}
	}
}

func TestListCountFlag(t *testing.T) {
	dir := t.TempDir()
	writeListPlan(t, dir, "plan_1.md", listTestPlan)

	out := runListCmd(t, dir, "--count", "2")

	if !strings.Contains(out, "Next 2 task(s):") {
		t.Errorf("expected 'Next 2 task(s):' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "TASK-001") || !strings.Contains(out, "TASK-002") {
		t.Errorf("expected TASK-001 and TASK-002 in output, got:\n%s", out)
	}
	if strings.Contains(out, "TASK-003") {
		t.Errorf("expected TASK-003 NOT in output (count=2), got:\n%s", out)
	}
}

func TestListPlainFlag(t *testing.T) {
	dir := t.TempDir()
	writeListPlan(t, dir, "plan_1.md", listTestPlan)

	out := runListCmd(t, dir)

	if strings.Contains(out, "\x1b[") {
		t.Errorf("expected no ANSI codes in plain output, got:\n%s", out)
	}
	if !strings.Contains(out, "TASK-001") {
		t.Errorf("expected TASK-001 in plain output, got:\n%s", out)
	}
}

func TestListPlainAndAllCombined(t *testing.T) {
	dir := t.TempDir()
	writeListPlan(t, dir, "plan_1.md", listTestPlan)

	out := runListCmd(t, dir, "--all")

	if strings.Contains(out, "\x1b[") {
		t.Errorf("expected no ANSI codes in plain output, got:\n%s", out)
	}
	if !strings.Contains(out, "All upcoming tasks:") {
		t.Errorf("expected 'All upcoming tasks:' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "TASK-006") {
		t.Errorf("expected TASK-006 in output with --all, got:\n%s", out)
	}
}

func TestListSkipsCompletedPlanFiles(t *testing.T) {
	dir := t.TempDir()
	writeListPlan(t, dir, "plan_1_completed.md", listTestPlan)
	out := runListCmd(t, dir)
	if !strings.Contains(out, "No pending tasks found") {
		t.Errorf("expected 'No pending tasks found' when only completed plan exists, got:\n%s", out)
	}
}

func TestListNoPendingTasks(t *testing.T) {
	dir := t.TempDir()
	out := runListCmd(t, dir)
	if !strings.Contains(out, "No pending tasks found") {
		t.Errorf("expected 'No pending tasks found', got:\n%s", out)
	}
}

func TestListFirstTaskFormat(t *testing.T) {
	dir := t.TempDir()
	writeListPlan(t, dir, "plan_1.md", listTestPlan)

	out := runListCmd(t, dir)

	// First task should have arrow prefix in plain mode
	if !strings.Contains(out, "-> #1  TASK-001: First task") {
		t.Errorf("expected '-> #1  TASK-001: First task' format in output, got:\n%s", out)
	}
}

func TestListFirstTaskHighlightedInTUI(t *testing.T) {
	// Test listModel.viewList() to verify first task highlighting
	tasks := []parser.Task{
		{ID: "TASK-001", Title: "First task", SourceFile: "plan_1.md"},
		{ID: "TASK-002", Title: "Second task", SourceFile: "plan_1.md"},
	}
	m := newListModel(tasks, "claude")
	m.Width = 120
	m.Height = 40
	content := m.viewList()

	// Should contain the arrow indicator for first task
	if !strings.Contains(content, "→") {
		t.Errorf("expected arrow indicator for first task in TUI content, got:\n%s", content)
	}
	if !strings.Contains(content, "TASK-001") {
		t.Errorf("expected TASK-001 in TUI content, got:\n%s", content)
	}
	if !strings.Contains(content, "TASK-002") {
		t.Errorf("expected TASK-002 in TUI content, got:\n%s", content)
	}
}

func TestListTUIShowsBlockedTasks(t *testing.T) {
	// Blocked tasks should appear in TUI list with ⊘ icon
	tasks := []parser.Task{
		{ID: "TASK-001", Title: "Workable task", SourceFile: "plan_1.md",
			Criteria: []parser.Criterion{{Text: "Do something", Checked: false}}},
		{ID: "TASK-002", Title: "Blocked task", SourceFile: "plan_1.md",
			Criteria: []parser.Criterion{{Text: "BLOCKED: waiting on something", Checked: false, Blocked: true}}},
	}
	m := newListModel(tasks, "claude")
	m.Width = 120
	m.Height = 40
	content := m.viewList()

	if !strings.Contains(content, "TASK-001") {
		t.Errorf("expected TASK-001 in TUI content, got:\n%s", content)
	}
	if !strings.Contains(content, "TASK-002") {
		t.Errorf("expected blocked TASK-002 in TUI content, got:\n%s", content)
	}
	if !strings.Contains(content, "⊘") {
		t.Errorf("expected ⊘ icon for blocked task in TUI content, got:\n%s", content)
	}
}

func TestListTUIHeaderShowsIncompleteCount(t *testing.T) {
	tasks := []parser.Task{
		{ID: "TASK-001", Title: "First", SourceFile: "plan_1.md"},
		{ID: "TASK-002", Title: "Second", SourceFile: "plan_1.md"},
		{ID: "TASK-003", Title: "Third", SourceFile: "plan_1.md"},
	}
	m := newListModel(tasks, "claude")
	m.Width = 120
	m.Height = 40
	content := m.viewList()

	if !strings.Contains(content, "All incomplete tasks (3)") {
		t.Errorf("expected 'All incomplete tasks (3)' in header, got:\n%s", content)
	}
}

func TestListTUIScrolling(t *testing.T) {
	// Create more tasks than can fit in a small terminal
	var tasks []parser.Task
	for i := 1; i <= 50; i++ {
		tasks = append(tasks, parser.Task{
			ID:         fmt.Sprintf("TASK-%03d", i),
			Title:      fmt.Sprintf("Task number %d", i),
			SourceFile: "plan_1.md",
		})
	}
	// Small terminal: only ~5 task lines visible (height=15 minus header/footer)
	m := newListModel(tasks, "claude")
	m.Width = 120
	m.Height = 15

	// Move cursor to the bottom
	m.Cursor = 10
	m.ensureCursorVisible()
	content := m.viewList()

	// TASK-011 should be visible (cursor is there)
	if !strings.Contains(content, "TASK-011") {
		t.Errorf("expected TASK-011 visible after scrolling, got:\n%s", content)
	}
	// TASK-001 should NOT be visible (scrolled past)
	if strings.Contains(content, "TASK-001") {
		t.Errorf("expected TASK-001 NOT visible after scrolling, got:\n%s", content)
	}
}

func TestListPlainShowsIgnoredTasks(t *testing.T) {
	dir := t.TempDir()
	plan := `# Plan: Test

## User Stories

### TASK-001: Workable task
**Description:** Do thing.
**Acceptance Criteria:**
- [ ] Criterion A

### IGNORED TASK-002: Ignored task
**Description:** Skipped thing.
**Acceptance Criteria:**
- [ ] Criterion B
`
	writeListPlan(t, dir, "plan_1.md", plan)
	out := runListCmd(t, dir, "--all")

	if !strings.Contains(out, "TASK-001") {
		t.Errorf("expected TASK-001 in plain output, got:\n%s", out)
	}
	if !strings.Contains(out, "TASK-002") {
		t.Errorf("expected ignored TASK-002 in plain output, got:\n%s", out)
	}
	if !strings.Contains(out, "[~]") {
		t.Errorf("expected [~] marker for ignored task, got:\n%s", out)
	}
}

func TestListPlainShowsIgnoredPlanTasks(t *testing.T) {
	dir := t.TempDir()
	plan := `# Plan: Ignored Plan

## User Stories

### TASK-001: Task in ignored plan
**Description:** Do thing.
**Acceptance Criteria:**
- [ ] Criterion A
`
	writeListPlan(t, dir, "plan_1_ignored.md", plan)
	out := runListCmd(t, dir, "--all")

	if !strings.Contains(out, "TASK-001") {
		t.Errorf("expected TASK-001 from ignored plan in output, got:\n%s", out)
	}
	if !strings.Contains(out, "[~]") {
		t.Errorf("expected [~] marker for task from ignored plan, got:\n%s", out)
	}
}

func TestListTUIShowsIgnoredTasks(t *testing.T) {
	tasks := []parser.Task{
		{ID: "TASK-001", Title: "Workable task", SourceFile: "plan_1.md",
			Criteria: []parser.Criterion{{Text: "Do something", Checked: false}}},
		{ID: "TASK-002", Title: "Ignored task", SourceFile: "plan_1.md",
			Ignored:  true,
			Criteria: []parser.Criterion{{Text: "Do something", Checked: false}}},
	}
	m := newListModel(tasks, "claude")
	m.Width = 120
	m.Height = 40
	content := m.viewList()

	if !strings.Contains(content, "TASK-001") {
		t.Errorf("expected TASK-001 in TUI content, got:\n%s", content)
	}
	if !strings.Contains(content, "TASK-002") {
		t.Errorf("expected ignored TASK-002 in TUI content, got:\n%s", content)
	}
	if !strings.Contains(content, "~") {
		t.Errorf("expected ~ icon for ignored task in TUI content, got:\n%s", content)
	}
}

func TestListPlainStillShowsWorkableOnly(t *testing.T) {
	dir := t.TempDir()
	plan := `# Plan: Test

## User Stories

### TASK-001: Workable task
**Description:** Do thing.
**Acceptance Criteria:**
- [ ] Criterion A

### TASK-002: Blocked task
**Description:** Blocked thing.
**Acceptance Criteria:**
- [ ] BLOCKED: waiting on dep

### TASK-003: Completed task
**Description:** Done thing.
**Acceptance Criteria:**
- [x] Done
`
	writeListPlan(t, dir, "plan_1.md", plan)
	out := runListCmd(t, dir, "--all")

	if !strings.Contains(out, "TASK-001") {
		t.Errorf("expected TASK-001 in plain output, got:\n%s", out)
	}
	if strings.Contains(out, "TASK-002") {
		t.Errorf("expected blocked TASK-002 NOT in plain output, got:\n%s", out)
	}
	if strings.Contains(out, "TASK-003") {
		t.Errorf("expected completed TASK-003 NOT in plain output, got:\n%s", out)
	}
}
