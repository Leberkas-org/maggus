package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
	if err := cmd.ParseFlags(flags); err != nil {
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

	// Header should say "Next 5 task(s):"
	if !strings.Contains(out, "Next 5 task(s):") {
		t.Errorf("expected 'Next 5 task(s):' in output, got:\n%s", out)
	}

	// Should contain tasks 1-5
	for _, id := range []string{"TASK-001", "TASK-002", "TASK-003", "TASK-004", "TASK-005"} {
		if !strings.Contains(out, id) {
			t.Errorf("expected %s in output, got:\n%s", id, out)
		}
	}

	// Should NOT contain task 6 (beyond count=5)
	if strings.Contains(out, "TASK-006") {
		t.Errorf("expected TASK-006 NOT in output (count=5), got:\n%s", out)
	}

	// Should NOT contain completed task
	if strings.Contains(out, "TASK-007") {
		t.Errorf("expected TASK-007 NOT in output (completed), got:\n%s", out)
	}
}

func TestListNoDescriptionLine(t *testing.T) {
	dir := t.TempDir()
	writeListPlan(t, dir, "plan_1.md", listTestPlan)

	out := runListCmd(t, dir)

	// Should not contain description text
	if strings.Contains(out, "Do thing") {
		t.Errorf("expected no description lines in output, got:\n%s", out)
	}
}

func TestListNoBlankLinesBetweenTasks(t *testing.T) {
	dir := t.TempDir()
	writeListPlan(t, dir, "plan_1.md", listTestPlan)

	out := runListCmd(t, dir)

	// After the blank line following header, consecutive task lines should not be separated by blank lines
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

	// Header should say "All upcoming tasks:"
	if !strings.Contains(out, "All upcoming tasks:") {
		t.Errorf("expected 'All upcoming tasks:' in output, got:\n%s", out)
	}

	// Should contain all workable tasks (001-006), not completed (007)
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

	// --all with --count 2 should still show all tasks
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

	out := runListCmd(t, dir, "--plain")

	// Should not contain ANSI escape codes
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

	out := runListCmd(t, dir, "--plain", "--all")

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
	// Only a completed plan file — no workable tasks expected
	out := runListCmd(t, dir)
	if !strings.Contains(out, "No pending tasks found") {
		t.Errorf("expected 'No pending tasks found' when only completed plan exists, got:\n%s", out)
	}
}

func TestListNoPendingTasks(t *testing.T) {
	dir := t.TempDir()
	// No .maggus dir
	out := runListCmd(t, dir)
	if !strings.Contains(out, "No pending tasks found") {
		t.Errorf("expected 'No pending tasks found', got:\n%s", out)
	}
}

func TestListFirstTaskFormat(t *testing.T) {
	dir := t.TempDir()
	writeListPlan(t, dir, "plan_1.md", listTestPlan)

	out := runListCmd(t, dir)

	// Check task line format: " #1  TASK-001: First task"
	if !strings.Contains(out, "#1  TASK-001: First task") {
		t.Errorf("expected '#1  TASK-001: First task' format in output, got:\n%s", out)
	}
}
