package cmd

import (
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/leberkas-org/maggus/internal/parser"
)

func newTestComponent(tasks []parser.Task) *taskListComponent {
	return &taskListComponent{
		Tasks:       tasks,
		Width:       80,
		Height:      24,
		HeaderLines: 2,
		AgentName:   "test-agent",
	}
}

func sampleTasks(n int) []parser.Task {
	tasks := make([]parser.Task, n)
	for i := range tasks {
		tasks[i] = parser.Task{
			ID:    fmt.Sprintf("TASK-%03d", i+1),
			Title: fmt.Sprintf("Task %d", i+1),
		}
	}
	return tasks
}

// --- Cursor movement wrapping ---

func TestTaskListComponent_CursorDown_WrapsToTop(t *testing.T) {
	c := newTestComponent(sampleTasks(3))
	c.Cursor = 2 // last item

	key := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}
	c.Update(key)

	if c.Cursor != 0 {
		t.Errorf("Cursor = %d, want 0 (should wrap to top)", c.Cursor)
	}
}

func TestTaskListComponent_CursorUp_WrapsToBottom(t *testing.T) {
	c := newTestComponent(sampleTasks(3))
	c.Cursor = 0 // first item

	key := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}}
	c.Update(key)

	if c.Cursor != 2 {
		t.Errorf("Cursor = %d, want 2 (should wrap to bottom)", c.Cursor)
	}
}

func TestTaskListComponent_CursorDown_Advances(t *testing.T) {
	c := newTestComponent(sampleTasks(5))
	c.Cursor = 1

	key := tea.KeyMsg{Type: tea.KeyDown}
	c.Update(key)

	if c.Cursor != 2 {
		t.Errorf("Cursor = %d, want 2", c.Cursor)
	}
}

func TestTaskListComponent_CursorUp_Retreats(t *testing.T) {
	c := newTestComponent(sampleTasks(5))
	c.Cursor = 3

	key := tea.KeyMsg{Type: tea.KeyUp}
	c.Update(key)

	if c.Cursor != 2 {
		t.Errorf("Cursor = %d, want 2", c.Cursor)
	}
}

func TestTaskListComponent_Home_GoesToFirst(t *testing.T) {
	c := newTestComponent(sampleTasks(5))
	c.Cursor = 4

	key := tea.KeyMsg{Type: tea.KeyHome}
	c.Update(key)

	if c.Cursor != 0 {
		t.Errorf("Cursor = %d, want 0", c.Cursor)
	}
}

func TestTaskListComponent_End_GoesToLast(t *testing.T) {
	c := newTestComponent(sampleTasks(5))
	c.Cursor = 0

	key := tea.KeyMsg{Type: tea.KeyEnd}
	c.Update(key)

	if c.Cursor != 4 {
		t.Errorf("Cursor = %d, want 4", c.Cursor)
	}
}

func TestTaskListComponent_EmptyTasks_CursorNoOp(t *testing.T) {
	c := newTestComponent(nil)

	key := tea.KeyMsg{Type: tea.KeyDown}
	_, action := c.Update(key)

	if c.Cursor != 0 {
		t.Errorf("Cursor = %d, want 0 on empty tasks", c.Cursor)
	}
	if action != taskListNone {
		t.Errorf("action = %d, want taskListNone", action)
	}
}

// --- ensureCursorVisible ---

func TestTaskListComponent_EnsureCursorVisible_CursorAboveViewport(t *testing.T) {
	c := newTestComponent(sampleTasks(20))
	c.ScrollOffset = 10
	c.Cursor = 5

	c.ensureCursorVisible()

	if c.ScrollOffset != 5 {
		t.Errorf("ScrollOffset = %d, want 5 (should scroll up to cursor)", c.ScrollOffset)
	}
}

func TestTaskListComponent_EnsureCursorVisible_CursorBelowViewport(t *testing.T) {
	c := newTestComponent(sampleTasks(20))
	c.ScrollOffset = 0
	c.Cursor = 15

	c.ensureCursorVisible()

	visible := c.visibleTaskLines()
	expected := c.Cursor - visible + 1
	if c.ScrollOffset != expected {
		t.Errorf("ScrollOffset = %d, want %d (should scroll down to show cursor)", c.ScrollOffset, expected)
	}
}

func TestTaskListComponent_EnsureCursorVisible_CursorInView(t *testing.T) {
	c := newTestComponent(sampleTasks(20))
	c.ScrollOffset = 5
	c.Cursor = 7

	origOffset := c.ScrollOffset
	c.ensureCursorVisible()

	if c.ScrollOffset != origOffset {
		t.Errorf("ScrollOffset changed from %d to %d, should stay unchanged when cursor is in view", origOffset, c.ScrollOffset)
	}
}

func TestTaskListComponent_EnsureCursorVisible_NegativeOffset(t *testing.T) {
	c := newTestComponent(sampleTasks(5))
	c.ScrollOffset = -3
	c.Cursor = 0

	c.ensureCursorVisible()

	if c.ScrollOffset < 0 {
		t.Errorf("ScrollOffset = %d, should not be negative", c.ScrollOffset)
	}
}

// --- visibleTaskLines ---

func TestTaskListComponent_VisibleTaskLines_NormalSize(t *testing.T) {
	c := newTestComponent(sampleTasks(5))
	c.Width = 80
	c.Height = 24
	c.HeaderLines = 2

	visible := c.visibleTaskLines()

	// FullScreenInnerSize(80, 24): innerH = 24 - 2*2 - 2 = 18
	// avail = 18 - headerLines(2) - footerLines(1) = 15
	if visible != 15 {
		t.Errorf("visibleTaskLines = %d, want 15", visible)
	}
}

func TestTaskListComponent_VisibleTaskLines_DifferentHeaderLines(t *testing.T) {
	c := newTestComponent(sampleTasks(5))
	c.Width = 80
	c.Height = 24
	c.HeaderLines = 5

	visible := c.visibleTaskLines()

	// innerH = 18, avail = 18 - 5 - 1 = 12
	if visible != 12 {
		t.Errorf("visibleTaskLines = %d, want 12", visible)
	}
}

func TestTaskListComponent_VisibleTaskLines_MinimumOne(t *testing.T) {
	c := newTestComponent(sampleTasks(5))
	c.Width = 20
	c.Height = 8 // very small
	c.HeaderLines = 10

	visible := c.visibleTaskLines()

	if visible < 1 {
		t.Errorf("visibleTaskLines = %d, should be at least 1", visible)
	}
}

// --- Detail view open/close ---

func TestTaskListComponent_OpenDetail(t *testing.T) {
	c := newTestComponent(sampleTasks(3))
	c.Cursor = 1

	c.openDetail()

	if !c.ShowDetail {
		t.Error("ShowDetail should be true after openDetail")
	}
	if !c.detailReady {
		t.Error("detailReady should be true after openDetail")
	}
}

func TestTaskListComponent_CloseDetail(t *testing.T) {
	c := newTestComponent(sampleTasks(3))
	c.openDetail()
	c.Detail.criteriaMode = true

	c.closeDetail()

	if c.ShowDetail {
		t.Error("ShowDetail should be false after closeDetail")
	}
	if c.detailReady {
		t.Error("detailReady should be false after closeDetail")
	}
	if c.Detail.criteriaMode {
		t.Error("criteriaMode should be false after closeDetail")
	}
}

func TestTaskListComponent_EnterOpensDetail(t *testing.T) {
	c := newTestComponent(sampleTasks(3))

	key := tea.KeyMsg{Type: tea.KeyEnter}
	c.Update(key)

	if !c.ShowDetail {
		t.Error("Enter key should open detail view")
	}
}

func TestTaskListComponent_EscClosesDetail(t *testing.T) {
	c := newTestComponent(sampleTasks(3))
	c.openDetail()

	key := tea.KeyMsg{Type: tea.KeyEsc}
	_, action := c.Update(key)

	if c.ShowDetail {
		t.Error("Esc key should close detail view")
	}
	if action != taskListNone {
		t.Errorf("action = %d, want taskListNone", action)
	}
}

func TestTaskListComponent_BackspaceClosesDetail(t *testing.T) {
	c := newTestComponent(sampleTasks(3))
	c.openDetail()

	key := tea.KeyMsg{Type: tea.KeyBackspace}
	c.Update(key)

	if c.ShowDetail {
		t.Error("Backspace should close detail view")
	}
}

func TestTaskListComponent_EnterOnEmptyTasks_NoDetail(t *testing.T) {
	c := newTestComponent(nil)

	key := tea.KeyMsg{Type: tea.KeyEnter}
	c.Update(key)

	if c.ShowDetail {
		t.Error("Enter on empty task list should not open detail")
	}
}

// --- Criteria mode enter/exit lifecycle ---

func TestTaskListComponent_CriteriaMode_EnterExit(t *testing.T) {
	tasks := []parser.Task{
		{
			ID:    "TASK-001",
			Title: "Test",
			Criteria: []parser.Criterion{
				{Text: "BLOCKED: something", Blocked: true},
				{Text: "normal"},
			},
		},
	}
	c := newTestComponent(tasks)
	c.openDetail()

	// Press tab to enter criteria mode
	tabKey := tea.KeyMsg{Type: tea.KeyTab}
	c.Update(tabKey)

	if !c.Detail.criteriaMode {
		t.Error("Tab should enter criteria mode when blocked criteria exist")
	}
	if len(c.Detail.blockedIndices) != 1 {
		t.Errorf("blockedIndices len = %d, want 1", len(c.Detail.blockedIndices))
	}

	// Press tab again to exit criteria mode
	c.Update(tabKey)

	if c.Detail.criteriaMode {
		t.Error("Tab should exit criteria mode")
	}
}

func TestTaskListComponent_CriteriaMode_NoBlockedCriteria(t *testing.T) {
	tasks := []parser.Task{
		{
			ID:    "TASK-001",
			Title: "Test",
			Criteria: []parser.Criterion{
				{Text: "normal"},
			},
		},
	}
	c := newTestComponent(tasks)
	c.openDetail()

	tabKey := tea.KeyMsg{Type: tea.KeyTab}
	c.Update(tabKey)

	if c.Detail.criteriaMode {
		t.Error("Should not enter criteria mode when no blocked criteria")
	}
	if !c.Detail.noBlockedMsg {
		t.Error("noBlockedMsg should be true when no blocked criteria")
	}
}

func TestTaskListComponent_CriteriaMode_Navigation(t *testing.T) {
	tasks := []parser.Task{
		{
			ID:    "TASK-001",
			Title: "Test",
			Criteria: []parser.Criterion{
				{Text: "BLOCKED: first", Blocked: true},
				{Text: "normal"},
				{Text: "BLOCKED: second", Blocked: true},
			},
		},
	}
	c := newTestComponent(tasks)
	c.openDetail()

	// Enter criteria mode
	tabKey := tea.KeyMsg{Type: tea.KeyTab}
	c.Update(tabKey)

	if c.Detail.criteriaCursor != 0 {
		t.Fatalf("criteriaCursor = %d, want 0", c.Detail.criteriaCursor)
	}

	// Move down
	downKey := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}
	c.Update(downKey)

	if c.Detail.criteriaCursor != 1 {
		t.Errorf("criteriaCursor = %d, want 1 after down", c.Detail.criteriaCursor)
	}

	// Move up
	upKey := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}}
	c.Update(upKey)

	if c.Detail.criteriaCursor != 0 {
		t.Errorf("criteriaCursor = %d, want 0 after up", c.Detail.criteriaCursor)
	}
}

func TestTaskListComponent_CriteriaMode_EscClosesDetail(t *testing.T) {
	tasks := []parser.Task{
		{
			ID:    "TASK-001",
			Title: "Test",
			Criteria: []parser.Criterion{
				{Text: "BLOCKED: something", Blocked: true},
			},
		},
	}
	c := newTestComponent(tasks)
	c.openDetail()

	tabKey := tea.KeyMsg{Type: tea.KeyTab}
	c.Update(tabKey)

	escKey := tea.KeyMsg{Type: tea.KeyEsc}
	c.Update(escKey)

	if c.ShowDetail {
		t.Error("Esc in criteria mode should close detail view")
	}
}

// --- Quit action ---

func TestTaskListComponent_QuitKeys(t *testing.T) {
	quitKeys := []tea.KeyMsg{
		{Type: tea.KeyRunes, Runes: []rune{'q'}},
		{Type: tea.KeyEsc},
		{Type: tea.KeyCtrlC},
	}

	for _, key := range quitKeys {
		c := newTestComponent(sampleTasks(3))
		_, action := c.Update(key)
		if action != taskListQuit {
			t.Errorf("key %q: action = %d, want taskListQuit", key.String(), action)
		}
	}
}

// --- Run action ---

func TestTaskListComponent_AltR_ReturnsRunAction(t *testing.T) {
	c := newTestComponent(sampleTasks(3))
	c.Cursor = 1

	key := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}, Alt: true}
	_, action := c.Update(key)

	if action != taskListRun {
		t.Errorf("action = %d, want taskListRun", action)
	}
	if c.RunTaskID != "TASK-002" {
		t.Errorf("RunTaskID = %q, want TASK-002", c.RunTaskID)
	}
}

func TestTaskListComponent_AltR_EmptyTasks(t *testing.T) {
	c := newTestComponent(nil)

	key := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}, Alt: true}
	_, action := c.Update(key)

	if action != taskListNone {
		t.Errorf("action = %d, want taskListNone on empty tasks", action)
	}
}

// --- Delete confirmation ---

func TestTaskListComponent_AltBackspace_OpensDeleteConfirm(t *testing.T) {
	c := newTestComponent(sampleTasks(3))

	key := tea.KeyMsg{Type: tea.KeyBackspace, Alt: true}
	c.Update(key)

	if !c.ConfirmDelete {
		t.Error("Alt+Backspace should open delete confirmation")
	}
}

func TestTaskListComponent_DeleteConfirm_Cancel(t *testing.T) {
	c := newTestComponent(sampleTasks(3))
	c.ConfirmDelete = true

	key := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}}
	_, action := c.Update(key)

	if c.ConfirmDelete {
		t.Error("'n' should cancel delete confirmation")
	}
	if action != taskListNone {
		t.Errorf("action = %d, want taskListNone", action)
	}
}

func TestTaskListComponent_DeleteConfirm_EscCancels(t *testing.T) {
	c := newTestComponent(sampleTasks(3))
	c.ConfirmDelete = true

	key := tea.KeyMsg{Type: tea.KeyEsc}
	c.Update(key)

	if c.ConfirmDelete {
		t.Error("Esc should cancel delete confirmation")
	}
}

// --- Detail view PageUp/PageDown navigation ---

func TestTaskListComponent_DetailView_PageDown(t *testing.T) {
	c := newTestComponent(sampleTasks(5))
	c.Cursor = 0
	c.openDetail()

	key := tea.KeyMsg{Type: tea.KeyPgDown}
	c.Update(key)

	if c.Cursor != 1 {
		t.Errorf("Cursor = %d, want 1 after PageDown in detail", c.Cursor)
	}
}

func TestTaskListComponent_DetailView_PageUp(t *testing.T) {
	c := newTestComponent(sampleTasks(5))
	c.Cursor = 2
	c.openDetail()

	key := tea.KeyMsg{Type: tea.KeyPgUp}
	c.Update(key)

	if c.Cursor != 1 {
		t.Errorf("Cursor = %d, want 1 after PageUp in detail", c.Cursor)
	}
}

func TestTaskListComponent_DetailView_PageDown_AtEnd(t *testing.T) {
	c := newTestComponent(sampleTasks(3))
	c.Cursor = 2
	c.openDetail()

	key := tea.KeyMsg{Type: tea.KeyPgDown}
	c.Update(key)

	if c.Cursor != 2 {
		t.Errorf("Cursor = %d, want 2 (should not advance past end)", c.Cursor)
	}
}

func TestTaskListComponent_DetailView_PageUp_AtStart(t *testing.T) {
	c := newTestComponent(sampleTasks(3))
	c.Cursor = 0
	c.openDetail()

	key := tea.KeyMsg{Type: tea.KeyPgUp}
	c.Update(key)

	if c.Cursor != 0 {
		t.Errorf("Cursor = %d, want 0 (should not retreat before start)", c.Cursor)
	}
}

// --- HandleResize ---

func TestTaskListComponent_HandleResize(t *testing.T) {
	c := newTestComponent(sampleTasks(3))
	c.HandleResize(120, 40)

	if c.Width != 120 {
		t.Errorf("Width = %d, want 120", c.Width)
	}
	if c.Height != 40 {
		t.Errorf("Height = %d, want 40", c.Height)
	}
}

func TestTaskListComponent_HandleResize_UpdatesDetailViewport(t *testing.T) {
	c := newTestComponent(sampleTasks(3))
	c.openDetail()

	c.HandleResize(120, 40)

	if !c.detailReady {
		t.Error("detailReady should remain true after resize in detail mode")
	}
}

// --- CurrentTask ---

func TestTaskListComponent_CurrentTask(t *testing.T) {
	tasks := sampleTasks(3)
	c := newTestComponent(tasks)
	c.Cursor = 1

	task := c.CurrentTask()
	if task == nil {
		t.Fatal("CurrentTask should not be nil")
	}
	if task.ID != "TASK-002" {
		t.Errorf("CurrentTask().ID = %q, want TASK-002", task.ID)
	}
}

func TestTaskListComponent_CurrentTask_Empty(t *testing.T) {
	c := newTestComponent(nil)

	task := c.CurrentTask()
	if task != nil {
		t.Error("CurrentTask should be nil for empty tasks")
	}
}

func TestTaskListComponent_CurrentTask_CursorOutOfBounds(t *testing.T) {
	c := newTestComponent(sampleTasks(2))
	c.Cursor = 5

	task := c.CurrentTask()
	if task != nil {
		t.Error("CurrentTask should be nil when cursor is out of bounds")
	}
}

// --- View ---

func TestTaskListComponent_View_ListMode_ReturnsEmpty(t *testing.T) {
	c := newTestComponent(sampleTasks(3))

	view := c.View()
	if view != "" {
		t.Errorf("View() in list mode should return empty string, got %q", view)
	}
}

func TestTaskListComponent_View_DetailMode_ReturnsContent(t *testing.T) {
	tasks := []parser.Task{
		{ID: "TASK-001", Title: "Test task", SourceFile: "/plan_1.md"},
	}
	c := newTestComponent(tasks)
	c.openDetail()

	view := c.View()
	if view == "" {
		t.Error("View() in detail mode should return non-empty string")
	}
}

func TestTaskListComponent_View_ConfirmDelete_ReturnsContent(t *testing.T) {
	tasks := []parser.Task{
		{ID: "TASK-001", Title: "Test task", SourceFile: "/plan_1.md"},
	}
	c := newTestComponent(tasks)
	c.ConfirmDelete = true

	view := c.View()
	if view == "" {
		t.Error("View() in confirm delete mode should return non-empty string")
	}
}

// --- Unhandled keys ---

func TestTaskListComponent_UnhandledKey(t *testing.T) {
	c := newTestComponent(sampleTasks(3))

	// 'x' is not a recognized key in list nav
	key := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}}
	_, action := c.Update(key)

	if action != taskListUnhandled {
		t.Errorf("action = %d, want taskListUnhandled for unrecognized key", action)
	}
}
