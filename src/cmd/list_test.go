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

const listTestBug = `# Bug: Test Bug

## Tasks

### BUG-001-001: Fix crash on startup
**Description:** App crashes.

**Acceptance Criteria:**
- [ ] No more crash

### BUG-001-002: Fix login issue
**Description:** Login fails.

**Acceptance Criteria:**
- [ ] Login works
`

func writeListPlan(t *testing.T, dir, filename, content string) {
	t.Helper()
	featuresDir := filepath.Join(dir, ".maggus", "features")
	if err := os.MkdirAll(featuresDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(featuresDir, filename), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeListBug(t *testing.T, dir, filename, content string) {
	t.Helper()
	bugsDir := filepath.Join(dir, ".maggus", "bugs")
	if err := os.MkdirAll(bugsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bugsDir, filename), []byte(content), 0o644); err != nil {
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
	writeListPlan(t, dir, "feature_001.md", listTestPlan)

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

func TestListAllFlag(t *testing.T) {
	dir := t.TempDir()
	writeListPlan(t, dir, "feature_001.md", listTestPlan)

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

func TestListCountFlag(t *testing.T) {
	dir := t.TempDir()
	writeListPlan(t, dir, "feature_001.md", listTestPlan)

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

func TestListNoPendingTasks(t *testing.T) {
	dir := t.TempDir()
	out := runListCmd(t, dir)
	if !strings.Contains(out, "No pending tasks found") {
		t.Errorf("expected 'No pending tasks found', got:\n%s", out)
	}
}

func TestListSkipsCompletedFeatureFiles(t *testing.T) {
	dir := t.TempDir()
	writeListPlan(t, dir, "feature_001_completed.md", listTestPlan)
	out := runListCmd(t, dir)
	if !strings.Contains(out, "No pending tasks found") {
		t.Errorf("expected 'No pending tasks found' when only completed feature file exists, got:\n%s", out)
	}
}

func TestListFirstTaskFormat(t *testing.T) {
	dir := t.TempDir()
	writeListPlan(t, dir, "feature_001.md", listTestPlan)

	out := runListCmd(t, dir)

	if !strings.Contains(out, "-> #1  TASK-001: First task") {
		t.Errorf("expected '-> #1  TASK-001: First task' format in output, got:\n%s", out)
	}
}

// --- Bug task tests ---

func TestListBugTasksAppearBeforeFeatureTasks(t *testing.T) {
	dir := t.TempDir()
	writeListPlan(t, dir, "feature_001.md", listTestPlan)
	writeListBug(t, dir, "bug_001.md", listTestBug)

	out := runListCmd(t, dir, "--all")

	bugPos := strings.Index(out, "BUG-001-001")
	featurePos := strings.Index(out, "TASK-001")
	if bugPos < 0 {
		t.Fatalf("expected BUG-001-001 in output, got:\n%s", out)
	}
	if featurePos < 0 {
		t.Fatalf("expected TASK-001 in output, got:\n%s", out)
	}
	if bugPos > featurePos {
		t.Errorf("expected bug tasks before feature tasks, but BUG-001-001 at %d, TASK-001 at %d:\n%s", bugPos, featurePos, out)
	}
}

func TestListBugTasksDisplayBugIDs(t *testing.T) {
	dir := t.TempDir()
	writeListBug(t, dir, "bug_001.md", listTestBug)

	out := runListCmd(t, dir, "--all")

	if !strings.Contains(out, "BUG-001-001") {
		t.Errorf("expected BUG-001-001 in output, got:\n%s", out)
	}
	if !strings.Contains(out, "BUG-001-002") {
		t.Errorf("expected BUG-001-002 in output, got:\n%s", out)
	}
}

func TestListBugSourceFileShown(t *testing.T) {
	dir := t.TempDir()
	writeListBug(t, dir, "bug_001.md", listTestBug)

	out := runListCmd(t, dir, "--all")

	if !strings.Contains(out, "bug_001.md") {
		t.Errorf("expected bug_001.md as source file in output, got:\n%s", out)
	}
}

func TestListMixedBugAndFeatureCount(t *testing.T) {
	dir := t.TempDir()
	writeListPlan(t, dir, "feature_001.md", listTestPlan)
	writeListBug(t, dir, "bug_001.md", listTestBug)

	// With count=3, should get 2 bug tasks + 1 feature task
	out := runListCmd(t, dir, "--count", "3")

	if !strings.Contains(out, "Next 3 task(s):") {
		t.Errorf("expected 'Next 3 task(s):' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "BUG-001-001") {
		t.Errorf("expected BUG-001-001 in output, got:\n%s", out)
	}
	if !strings.Contains(out, "BUG-001-002") {
		t.Errorf("expected BUG-001-002 in output, got:\n%s", out)
	}
	if !strings.Contains(out, "TASK-001") {
		t.Errorf("expected TASK-001 in output, got:\n%s", out)
	}
	// Should not include more than 3
	if strings.Contains(out, "TASK-002") {
		t.Errorf("expected TASK-002 NOT in output (count=3, 2 bugs + 1 feature), got:\n%s", out)
	}
}

func TestListBugOnlyNoFeatures(t *testing.T) {
	dir := t.TempDir()
	writeListBug(t, dir, "bug_001.md", listTestBug)

	out := runListCmd(t, dir, "--all")

	if !strings.Contains(out, "BUG-001-001") {
		t.Errorf("expected BUG-001-001 in output, got:\n%s", out)
	}
	if !strings.Contains(out, "BUG-001-002") {
		t.Errorf("expected BUG-001-002 in output, got:\n%s", out)
	}
}

func TestListTUIBugTasksBeforeFeatures(t *testing.T) {
	bugTasks := []parser.Task{
		{ID: "BUG-001-001", Title: "Fix crash", SourceFile: "bug_001.md"},
	}
	featureTasks := []parser.Task{
		{ID: "TASK-001", Title: "Feature task", SourceFile: "feature_001.md"},
	}
	// Bugs first, then features — matching work loop ordering
	tasks := append(bugTasks, featureTasks...)
	m := newListModel(tasks, "claude")
	m.Width = 120
	m.Height = 40
	content := m.viewList()

	bugPos := strings.Index(content, "BUG-001-001")
	featurePos := strings.Index(content, "TASK-001")
	if bugPos < 0 || featurePos < 0 {
		t.Fatalf("expected both tasks in TUI content, got:\n%s", content)
	}
	if bugPos > featurePos {
		t.Errorf("expected bug task before feature task in TUI, got:\n%s", content)
	}
}

func TestListTUIShowsBugSourceFile(t *testing.T) {
	tasks := []parser.Task{
		{ID: "BUG-001-001", Title: "Fix crash", SourceFile: "/path/to/bug_001.md"},
	}
	m := newListModel(tasks, "claude")
	m.Width = 120
	m.Height = 40
	content := m.viewList()

	if !strings.Contains(content, "bug_001.md") {
		t.Errorf("expected bug_001.md source file in TUI content, got:\n%s", content)
	}
}

func TestListTUIHeaderShowsIncompleteCount(t *testing.T) {
	tasks := []parser.Task{
		{ID: "BUG-001-001", Title: "Bug", SourceFile: "bug_001.md"},
		{ID: "TASK-001", Title: "Feature", SourceFile: "feature_001.md"},
		{ID: "TASK-002", Title: "Feature 2", SourceFile: "feature_001.md"},
	}
	m := newListModel(tasks, "claude")
	m.Width = 120
	m.Height = 40
	content := m.viewList()

	if !strings.Contains(content, "All incomplete tasks (3)") {
		t.Errorf("expected 'All incomplete tasks (3)' in header, got:\n%s", content)
	}
}

func TestListTUIShowsBlockedTasks(t *testing.T) {
	tasks := []parser.Task{
		{ID: "TASK-001", Title: "Workable task", SourceFile: "feature_001.md",
			Criteria: []parser.Criterion{{Text: "Do something", Checked: false}}},
		{ID: "TASK-002", Title: "Blocked task", SourceFile: "feature_001.md",
			Criteria: []parser.Criterion{{Text: "BLOCKED: waiting on something", Checked: false, Blocked: true}}},
	}
	m := newListModel(tasks, "claude")
	m.Width = 120
	m.Height = 40
	content := m.viewList()

	if !strings.Contains(content, "TASK-002") {
		t.Errorf("expected blocked TASK-002 in TUI content, got:\n%s", content)
	}
	if !strings.Contains(content, "⊘") {
		t.Errorf("expected ⊘ icon for blocked task in TUI content, got:\n%s", content)
	}
}

func TestListTUIScrolling(t *testing.T) {
	var tasks []parser.Task
	for i := 1; i <= 50; i++ {
		tasks = append(tasks, parser.Task{
			ID:         fmt.Sprintf("TASK-%03d", i),
			Title:      fmt.Sprintf("Task number %d", i),
			SourceFile: "feature_001.md",
		})
	}
	m := newListModel(tasks, "claude")
	m.Width = 120
	m.Height = 15

	m.Cursor = 10
	m.ensureCursorVisible()
	content := m.viewList()

	if !strings.Contains(content, "TASK-011") {
		t.Errorf("expected TASK-011 visible after scrolling, got:\n%s", content)
	}
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
	writeListPlan(t, dir, "feature_001.md", plan)
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
	writeListPlan(t, dir, "feature_001.md", plan)
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
