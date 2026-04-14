package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leberkas-org/maggus/internal/parser"
)

func setupPlanDir(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, ".maggus"), 0o755); err != nil {
		t.Fatal(err)
	}
}

func writePlanFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, ".maggus", name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestReloadTask_Found(t *testing.T) {
	dir := t.TempDir()
	setupPlanDir(t, dir)
	planContent := "### TASK-001: First task\n\n- [ ] criterion A\n\n### TASK-002: Second task\n\n- [x] criterion B\n"
	writePlanFile(t, dir, "plan_1.md", planContent)
	planFile := filepath.Join(dir, ".maggus", "plan_1.md")

	task := reloadTask(planFile, "TASK-001")
	if task == nil {
		t.Fatal("reloadTask should find TASK-001")
	}
	if task.ID != "TASK-001" {
		t.Errorf("task.ID = %q, want TASK-001", task.ID)
	}
	if task.Title != "First task" {
		t.Errorf("task.Title = %q, want 'First task'", task.Title)
	}
}

func TestReloadTask_NotFound(t *testing.T) {
	dir := t.TempDir()
	setupPlanDir(t, dir)
	planContent := "### TASK-001: First task\n\n- [ ] criterion A\n"
	writePlanFile(t, dir, "plan_1.md", planContent)
	planFile := filepath.Join(dir, ".maggus", "plan_1.md")

	task := reloadTask(planFile, "TASK-999")
	if task != nil {
		t.Errorf("reloadTask should return nil for missing task, got %v", task)
	}
}

func TestReloadTask_InvalidFile(t *testing.T) {
	task := reloadTask("/nonexistent/path/plan.md", "TASK-001")
	if task != nil {
		t.Error("reloadTask should return nil for invalid file")
	}
}

func TestDetailFooter_Scrollable(t *testing.T) {
	footer := detailFooter(true)

	if !strings.Contains(footer, "scroll") {
		t.Errorf("scrollable footer should contain 'scroll', got: %s", footer)
	}
	if !strings.Contains(footer, "prev/next task") {
		t.Errorf("footer should contain 'prev/next task', got: %s", footer)
	}
}

func TestDetailFooter_NotScrollable(t *testing.T) {
	footer := detailFooter(false)

	// No scroll hint when not scrollable, but should have other parts
	if strings.Contains(footer, "↑/↓: scroll") {
		t.Errorf("non-scrollable footer should not contain '↑/↓: scroll', got: %s", footer)
	}
	if !strings.Contains(footer, "prev/next task") {
		t.Errorf("footer should contain 'prev/next task', got: %s", footer)
	}
}

func TestRenderDetailContent_BasicTask(t *testing.T) {
	task := parser.Task{
		ID:          "TASK-001",
		Title:       "Test task",
		SourceFile:  "/some/path/plan_1.md",
		Description: "A test description",
		Criteria: []parser.Criterion{
			{Text: "First criterion", Checked: true},
			{Text: "Second criterion"},
			{Text: "BLOCKED: Third", Blocked: true},
		},
	}

	content := renderDetailContent(task)

	if !strings.Contains(content, "TASK-001") {
		t.Error("content should contain task ID")
	}
	if !strings.Contains(content, "Test task") {
		t.Error("content should contain task title")
	}
	if !strings.Contains(content, "plan_1.md") {
		t.Error("content should contain plan filename")
	}
	if !strings.Contains(content, "A test description") {
		t.Error("content should contain description")
	}
	if !strings.Contains(content, "First criterion") {
		t.Error("content should contain first criterion")
	}
	if !strings.Contains(content, "Second criterion") {
		t.Error("content should contain second criterion")
	}
	if !strings.Contains(content, "Third") {
		t.Error("content should contain blocked criterion text")
	}
	if !strings.Contains(content, "3 total, 1 done, 1 blocked") {
		t.Error("content should contain correct criteria counts")
	}
}

func TestRenderDetailContent_CompleteTask(t *testing.T) {
	task := parser.Task{
		ID:         "TASK-002",
		Title:      "Done task",
		SourceFile: "/plan_1.md",
		Criteria: []parser.Criterion{
			{Text: "A", Checked: true},
			{Text: "B", Checked: true},
		},
	}

	content := renderDetailContent(task)
	if !strings.Contains(content, "Complete") {
		t.Error("complete task should show Complete status")
	}
}

func TestRenderDetailContent_BlockedTask(t *testing.T) {
	task := parser.Task{
		ID:         "TASK-003",
		Title:      "Blocked task",
		SourceFile: "/plan_1.md",
		Criteria: []parser.Criterion{
			{Text: "BLOCKED: something", Blocked: true},
		},
	}

	content := renderDetailContent(task)
	if !strings.Contains(content, "Blocked") {
		t.Error("blocked task should show Blocked status")
	}
}

func TestRenderDetailContent_NoDescription(t *testing.T) {
	task := parser.Task{
		ID:         "TASK-005",
		Title:      "No desc",
		SourceFile: "/plan_1.md",
		Criteria: []parser.Criterion{
			{Text: "criterion"},
		},
	}

	content := renderDetailContent(task)
	if strings.Contains(content, "Description") {
		t.Error("should not show Description heading when description is empty")
	}
}

func TestRenderDetailContent_NoCriteria(t *testing.T) {
	task := parser.Task{
		ID:         "TASK-006",
		Title:      "No criteria",
		SourceFile: "/plan_1.md",
	}

	content := renderDetailContent(task)
	if strings.Contains(content, "Acceptance Criteria") {
		t.Error("should not show Acceptance Criteria heading when empty")
	}
}
