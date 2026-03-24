package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leberkas-org/maggus/internal/parser"
)

func TestCriteriaAction_String(t *testing.T) {
	tests := []struct {
		action criteriaAction
		want   string
	}{
		{criteriaActionUnblock, "Unblock"},
		{criteriaActionResolve, "Resolve"},
		{criteriaActionDelete, "Delete"},
		{criteriaActionSkip, "Skip"},
		{criteriaAction(99), ""},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := tt.action.String()
			if got != tt.want {
				t.Errorf("criteriaAction(%d).String() = %q, want %q", tt.action, got, tt.want)
			}
		})
	}
}

func TestCriteriaAction_Description(t *testing.T) {
	tests := []struct {
		action criteriaAction
		want   string
	}{
		{criteriaActionUnblock, "Remove BLOCKED: prefix, keep unchecked"},
		{criteriaActionResolve, "Mark as done (remove block + check)"},
		{criteriaActionDelete, "Remove criterion entirely"},
		{criteriaActionSkip, "Do nothing"},
		{criteriaAction(99), ""},
	}
	for _, tt := range tests {
		t.Run(tt.action.String(), func(t *testing.T) {
			got := tt.action.Description()
			if got != tt.want {
				t.Errorf("criteriaAction(%d).Description() = %q, want %q", tt.action, got, tt.want)
			}
		})
	}
}

func TestCriteriaActions_AllPresent(t *testing.T) {
	if len(criteriaActions) != 4 {
		t.Errorf("criteriaActions has %d entries, want 4", len(criteriaActions))
	}
	expected := []criteriaAction{criteriaActionUnblock, criteriaActionResolve, criteriaActionDelete, criteriaActionSkip}
	for i, want := range expected {
		if criteriaActions[i] != want {
			t.Errorf("criteriaActions[%d] = %d, want %d", i, criteriaActions[i], want)
		}
	}
}

func TestDetailState_InitCriteriaMode_WithBlocked(t *testing.T) {
	task := parser.Task{
		Criteria: []parser.Criterion{
			{Text: "done", Checked: true},
			{Text: "BLOCKED: something", Blocked: true},
			{Text: "pending"},
			{Text: "BLOCKED: another", Blocked: true},
		},
	}

	ds := &detailState{}
	ok := ds.initCriteriaMode(task)

	if !ok {
		t.Fatal("initCriteriaMode should return true when blocked criteria exist")
	}
	if !ds.criteriaMode {
		t.Error("criteriaMode should be true")
	}
	if ds.criteriaCursor != 0 {
		t.Errorf("criteriaCursor = %d, want 0", ds.criteriaCursor)
	}
	if ds.showActionPicker {
		t.Error("showActionPicker should be false")
	}
	if ds.actionCursor != 0 {
		t.Errorf("actionCursor = %d, want 0", ds.actionCursor)
	}
	if len(ds.blockedIndices) != 2 {
		t.Fatalf("blockedIndices len = %d, want 2", len(ds.blockedIndices))
	}
	if ds.blockedIndices[0] != 1 || ds.blockedIndices[1] != 3 {
		t.Errorf("blockedIndices = %v, want [1, 3]", ds.blockedIndices)
	}
}

func TestDetailState_InitCriteriaMode_NoBlocked(t *testing.T) {
	task := parser.Task{
		Criteria: []parser.Criterion{
			{Text: "done", Checked: true},
			{Text: "pending"},
		},
	}

	ds := &detailState{}
	ok := ds.initCriteriaMode(task)

	if ok {
		t.Fatal("initCriteriaMode should return false when no blocked criteria")
	}
	if ds.criteriaMode {
		t.Error("criteriaMode should be false")
	}
}

func TestDetailState_InitCriteriaMode_EmptyCriteria(t *testing.T) {
	task := parser.Task{}
	ds := &detailState{}
	ok := ds.initCriteriaMode(task)

	if ok {
		t.Fatal("initCriteriaMode should return false for empty criteria")
	}
}

func TestDetailState_ExitCriteriaMode(t *testing.T) {
	ds := &detailState{
		criteriaMode:     true,
		criteriaCursor:   2,
		showActionPicker: true,
		actionCursor:     1,
		blockedIndices:   []int{1, 3, 5},
	}

	ds.exitCriteriaMode()

	if ds.criteriaMode {
		t.Error("criteriaMode should be false")
	}
	if ds.criteriaCursor != 0 {
		t.Errorf("criteriaCursor = %d, want 0", ds.criteriaCursor)
	}
	if ds.showActionPicker {
		t.Error("showActionPicker should be false")
	}
	if ds.actionCursor != 0 {
		t.Errorf("actionCursor = %d, want 0", ds.actionCursor)
	}
	if ds.blockedIndices != nil {
		t.Errorf("blockedIndices = %v, want nil", ds.blockedIndices)
	}
}

func TestDetailState_PerformAction_Skip(t *testing.T) {
	task := parser.Task{
		Criteria: []parser.Criterion{
			{Text: "BLOCKED: something", Blocked: true},
		},
	}
	ds := &detailState{
		blockedIndices: []int{0},
		criteriaCursor: 0,
	}

	modified, err := ds.performAction(task, criteriaActionSkip)
	if err != nil {
		t.Fatalf("performAction(skip) error: %v", err)
	}
	if modified {
		t.Error("skip should not modify")
	}
}

func TestDetailState_PerformAction_OutOfBounds(t *testing.T) {
	task := parser.Task{
		Criteria: []parser.Criterion{
			{Text: "BLOCKED: something", Blocked: true},
		},
	}
	ds := &detailState{
		blockedIndices: []int{0},
		criteriaCursor: 5, // out of bounds
	}

	modified, err := ds.performAction(task, criteriaActionUnblock)
	if err != nil {
		t.Fatalf("performAction error: %v", err)
	}
	if modified {
		t.Error("out of bounds cursor should not modify")
	}
}

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

func TestDetailState_PerformAction_Unblock(t *testing.T) {
	dir := t.TempDir()
	setupPlanDir(t, dir)
	planContent := "### TASK-001: Test\n\n- [ ] BLOCKED: needs fix\n- [ ] other criterion\n"
	writePlanFile(t, dir, "plan_1.md", planContent)
	planFile := filepath.Join(dir, ".maggus", "plan_1.md")

	task := parser.Task{
		SourceFile: planFile,
		Criteria: []parser.Criterion{
			{Text: "BLOCKED: needs fix", Blocked: true},
			{Text: "other criterion"},
		},
	}
	ds := &detailState{
		blockedIndices: []int{0},
		criteriaCursor: 0,
	}

	modified, err := ds.performAction(task, criteriaActionUnblock)
	if err != nil {
		t.Fatalf("performAction(unblock) error: %v", err)
	}
	if !modified {
		t.Error("unblock should return modified=true")
	}

	data, _ := os.ReadFile(planFile)
	if strings.Contains(string(data), "BLOCKED:") {
		t.Error("file should not contain BLOCKED: after unblock")
	}
	if !strings.Contains(string(data), "- [ ] needs fix") {
		t.Error("file should contain unblocked criterion")
	}
}

func TestDetailState_PerformAction_Resolve(t *testing.T) {
	dir := t.TempDir()
	setupPlanDir(t, dir)
	planContent := "### TASK-001: Test\n\n- [ ] BLOCKED: needs fix\n"
	writePlanFile(t, dir, "plan_1.md", planContent)
	planFile := filepath.Join(dir, ".maggus", "plan_1.md")

	task := parser.Task{
		SourceFile: planFile,
		Criteria: []parser.Criterion{
			{Text: "BLOCKED: needs fix", Blocked: true},
		},
	}
	ds := &detailState{
		blockedIndices: []int{0},
		criteriaCursor: 0,
	}

	modified, err := ds.performAction(task, criteriaActionResolve)
	if err != nil {
		t.Fatalf("performAction(resolve) error: %v", err)
	}
	if !modified {
		t.Error("resolve should return modified=true")
	}

	data, _ := os.ReadFile(planFile)
	if !strings.Contains(string(data), "- [x] needs fix") {
		t.Errorf("file should contain resolved criterion, got:\n%s", string(data))
	}
}

func TestDetailState_PerformAction_Delete(t *testing.T) {
	dir := t.TempDir()
	setupPlanDir(t, dir)
	planContent := "### TASK-001: Test\n\n- [ ] BLOCKED: remove me\n- [ ] keep me\n"
	writePlanFile(t, dir, "plan_1.md", planContent)
	planFile := filepath.Join(dir, ".maggus", "plan_1.md")

	task := parser.Task{
		SourceFile: planFile,
		Criteria: []parser.Criterion{
			{Text: "BLOCKED: remove me", Blocked: true},
			{Text: "keep me"},
		},
	}
	ds := &detailState{
		blockedIndices: []int{0},
		criteriaCursor: 0,
	}

	modified, err := ds.performAction(task, criteriaActionDelete)
	if err != nil {
		t.Fatalf("performAction(delete) error: %v", err)
	}
	if !modified {
		t.Error("delete should return modified=true")
	}

	data, _ := os.ReadFile(planFile)
	if strings.Contains(string(data), "remove me") {
		t.Error("file should not contain deleted criterion")
	}
	if !strings.Contains(string(data), "keep me") {
		t.Error("file should still contain other criterion")
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

func TestDetailFooter_CriteriaMode_ActionPicker(t *testing.T) {
	ds := &detailState{criteriaMode: true, showActionPicker: true}
	footer := detailFooter(ds, false)

	if !strings.Contains(footer, "select action") {
		t.Errorf("action picker footer should contain 'select action', got: %s", footer)
	}
}

func TestDetailFooter_CriteriaMode_Navigation(t *testing.T) {
	ds := &detailState{criteriaMode: true, showActionPicker: false}
	footer := detailFooter(ds, false)

	if !strings.Contains(footer, "navigate blocked") {
		t.Errorf("criteria mode footer should contain 'navigate blocked', got: %s", footer)
	}
}

func TestDetailFooter_ScrollMode_Scrollable(t *testing.T) {
	footer := detailFooter(nil, true)

	if !strings.Contains(footer, "scroll") {
		t.Errorf("scrollable footer should contain 'scroll', got: %s", footer)
	}
	if !strings.Contains(footer, "manage blocked") {
		t.Errorf("footer should contain 'manage blocked', got: %s", footer)
	}
}

func TestDetailFooter_ScrollMode_NotScrollable(t *testing.T) {
	footer := detailFooter(nil, false)

	// Should not have "↑/↓: scroll" but should have other parts
	if !strings.Contains(footer, "manage blocked") {
		t.Errorf("footer should contain 'manage blocked', got: %s", footer)
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

	content := renderDetailContent(task, nil)

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

	content := renderDetailContent(task, nil)
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

	content := renderDetailContent(task, nil)
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

	content := renderDetailContent(task, nil)
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

	content := renderDetailContent(task, nil)
	if strings.Contains(content, "Acceptance Criteria") {
		t.Error("should not show Acceptance Criteria heading when empty")
	}
}

func TestRenderDetailContent_NoBlockedMessage(t *testing.T) {
	task := parser.Task{
		ID:         "TASK-007",
		Title:      "Test",
		SourceFile: "/plan_1.md",
	}
	ds := &detailState{noBlockedMsg: true}

	content := renderDetailContent(task, ds)
	if !strings.Contains(content, "No blocked criteria") {
		t.Error("should show no-blocked message when noBlockedMsg is true")
	}
}

func TestRenderInlineActionPicker(t *testing.T) {
	// Test each cursor position highlights the correct action
	for cursor := 0; cursor < len(criteriaActions); cursor++ {
		result := renderInlineActionPicker(cursor)

		// Should contain all action names
		for _, a := range criteriaActions {
			if !strings.Contains(result, a.String()) {
				t.Errorf("cursor=%d: missing action %q", cursor, a.String())
			}
			if !strings.Contains(result, a.Description()) {
				t.Errorf("cursor=%d: missing description for %q", cursor, a.String())
			}
		}

		// Should contain exactly one "> " prefix
		lines := strings.Split(strings.TrimSpace(result), "\n")
		selectedCount := 0
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "> ") {
				selectedCount++
			}
		}
		if selectedCount != 1 {
			t.Errorf("cursor=%d: expected 1 selected line, got %d", cursor, selectedCount)
		}
	}
}

func TestRenderDetailContent_CriteriaMode_Highlight(t *testing.T) {
	task := parser.Task{
		ID:         "TASK-008",
		Title:      "Test",
		SourceFile: "/plan_1.md",
		Criteria: []parser.Criterion{
			{Text: "normal criterion"},
			{Text: "BLOCKED: first block", Blocked: true},
			{Text: "BLOCKED: second block", Blocked: true},
		},
	}
	ds := &detailState{
		criteriaMode:   true,
		blockedIndices: []int{1, 2},
		criteriaCursor: 0, // first blocked = index 1
	}

	content := renderDetailContent(task, ds)
	// The selected blocked criterion should have the ▸ marker
	if !strings.Contains(content, "▸") {
		t.Error("criteria mode should show ▸ marker for selected blocked criterion")
	}
}

func TestRenderDetailContent_CriteriaMode_ActionPicker(t *testing.T) {
	task := parser.Task{
		ID:         "TASK-009",
		Title:      "Test",
		SourceFile: "/plan_1.md",
		Criteria: []parser.Criterion{
			{Text: "BLOCKED: something", Blocked: true},
		},
	}
	ds := &detailState{
		criteriaMode:     true,
		blockedIndices:   []int{0},
		criteriaCursor:   0,
		showActionPicker: true,
		actionCursor:     0,
	}

	content := renderDetailContent(task, ds)
	// Should contain action picker options
	if !strings.Contains(content, "Unblock") {
		t.Error("action picker should contain Unblock option")
	}
	if !strings.Contains(content, "Resolve") {
		t.Error("action picker should contain Resolve option")
	}
}
