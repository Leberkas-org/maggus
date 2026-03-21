package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/leberkas-org/maggus/internal/parser"
	"github.com/leberkas-org/maggus/internal/tasklock"
)

func TestFindTaskByID_Found(t *testing.T) {
	tasks := []parser.Task{
		{ID: "TASK-001", Title: "First", Criteria: []parser.Criterion{{Text: "A", Checked: false}}},
		{ID: "TASK-002", Title: "Second", Criteria: []parser.Criterion{{Text: "B", Checked: false}}},
		{ID: "TASK-003", Title: "Third", Criteria: []parser.Criterion{{Text: "C", Checked: false}}},
	}

	got := findTaskByID(tasks, "TASK-002")
	if got == nil {
		t.Fatal("expected non-nil task, got nil")
	}
	if got.ID != "TASK-002" {
		t.Errorf("expected TASK-002, got %s", got.ID)
	}
	if got.Title != "Second" {
		t.Errorf("expected title 'Second', got %q", got.Title)
	}
}

func TestFindTaskByID_CompleteReturnsNil(t *testing.T) {
	tasks := []parser.Task{
		{ID: "TASK-001", Title: "Done", Criteria: []parser.Criterion{{Text: "A", Checked: true}}},
	}

	got := findTaskByID(tasks, "TASK-001")
	if got != nil {
		t.Errorf("expected nil for complete task, got %+v", got)
	}
}

func TestFindTaskByID_NotFoundReturnsNil(t *testing.T) {
	tasks := []parser.Task{
		{ID: "TASK-001", Title: "First", Criteria: []parser.Criterion{{Text: "A", Checked: false}}},
	}

	got := findTaskByID(tasks, "TASK-999")
	if got != nil {
		t.Errorf("expected nil for missing task, got %+v", got)
	}
}

func TestFindTaskByID_EmptySlice(t *testing.T) {
	got := findTaskByID(nil, "TASK-001")
	if got != nil {
		t.Errorf("expected nil for empty slice, got %+v", got)
	}
}

func TestFindNextUnlocked_ReturnsFirstWorkable(t *testing.T) {
	dir := t.TempDir()

	tasks := []parser.Task{
		// Complete task — should be skipped
		{ID: "TASK-001", Title: "Done", Criteria: []parser.Criterion{{Text: "A", Checked: true}}},
		// Workable and unlocked — should be returned
		{ID: "TASK-002", Title: "Next", Criteria: []parser.Criterion{{Text: "B", Checked: false}}},
		// Workable and unlocked — should not be returned (TASK-002 comes first)
		{ID: "TASK-003", Title: "After", Criteria: []parser.Criterion{{Text: "C", Checked: false}}},
	}

	got := findNextUnlocked(tasks, dir)
	if got == nil {
		t.Fatal("expected non-nil task, got nil")
	}
	if got.ID != "TASK-002" {
		t.Errorf("expected TASK-002, got %s", got.ID)
	}
}

func TestFindNextUnlocked_SkipsLockedTasks(t *testing.T) {
	dir := t.TempDir()

	// Create lock for TASK-002
	locksDir := filepath.Join(dir, ".maggus", "locks")
	if err := os.MkdirAll(locksDir, 0755); err != nil {
		t.Fatal(err)
	}
	lockPath := filepath.Join(locksDir, "TASK-002.lock")
	if err := os.WriteFile(lockPath, []byte("run_id: test\n"), 0644); err != nil {
		t.Fatal(err)
	}

	tasks := []parser.Task{
		{ID: "TASK-001", Title: "Done", Criteria: []parser.Criterion{{Text: "A", Checked: true}}},
		{ID: "TASK-002", Title: "Locked", Criteria: []parser.Criterion{{Text: "B", Checked: false}}},
		{ID: "TASK-003", Title: "Free", Criteria: []parser.Criterion{{Text: "C", Checked: false}}},
	}

	got := findNextUnlocked(tasks, dir)
	if got == nil {
		t.Fatal("expected non-nil task, got nil")
	}
	if got.ID != "TASK-003" {
		t.Errorf("expected TASK-003 (skipping locked TASK-002), got %s", got.ID)
	}
}

func TestFindNextUnlocked_AllLockedReturnsNil(t *testing.T) {
	dir := t.TempDir()

	// Lock both workable tasks
	locksDir := filepath.Join(dir, ".maggus", "locks")
	if err := os.MkdirAll(locksDir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"TASK-001", "TASK-002"} {
		lock, err := tasklock.Acquire(dir, id, "test-run")
		if err != nil {
			t.Fatalf("failed to acquire lock for %s: %v", id, err)
		}
		_ = lock // keep lock held (file exists)
	}

	tasks := []parser.Task{
		{ID: "TASK-001", Title: "Locked1", Criteria: []parser.Criterion{{Text: "A", Checked: false}}},
		{ID: "TASK-002", Title: "Locked2", Criteria: []parser.Criterion{{Text: "B", Checked: false}}},
	}

	got := findNextUnlocked(tasks, dir)
	if got != nil {
		t.Errorf("expected nil when all tasks are locked, got %+v", got)
	}
}

func TestFindNextUnlocked_SkipsBlockedTasks(t *testing.T) {
	dir := t.TempDir()

	tasks := []parser.Task{
		{ID: "TASK-001", Title: "Blocked", Criteria: []parser.Criterion{
			{Text: "BLOCKED: needs something", Checked: false, Blocked: true},
		}},
		{ID: "TASK-002", Title: "Workable", Criteria: []parser.Criterion{{Text: "B", Checked: false}}},
	}

	got := findNextUnlocked(tasks, dir)
	if got == nil {
		t.Fatal("expected non-nil task, got nil")
	}
	if got.ID != "TASK-002" {
		t.Errorf("expected TASK-002 (skipping blocked TASK-001), got %s", got.ID)
	}
}

func TestFindNextUnlocked_EmptySlice(t *testing.T) {
	dir := t.TempDir()

	got := findNextUnlocked(nil, dir)
	if got != nil {
		t.Errorf("expected nil for empty slice, got %+v", got)
	}
}

func TestIsTaskAtOrPastTarget(t *testing.T) {
	tasks := []parser.Task{
		{ID: "TASK-001", Title: "First"},
		{ID: "TASK-002", Title: "Second"},
		{ID: "TASK-003", Title: "Third"},
		{ID: "TASK-004", Title: "Fourth"},
	}

	tests := []struct {
		name      string
		completed string
		target    string
		want      bool
	}{
		{"completed equals target", "TASK-002", "TASK-002", true},
		{"completed after target", "TASK-003", "TASK-002", true},
		{"completed before target", "TASK-001", "TASK-003", false},
		{"empty completed", "", "TASK-002", false},
		{"empty target", "TASK-002", "", false},
		{"both empty", "", "", false},
		{"unknown completed", "TASK-999", "TASK-002", false},
		{"unknown target", "TASK-002", "TASK-999", false},
		{"last task completed past earlier target", "TASK-004", "TASK-001", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isTaskAtOrPastTarget(tasks, tt.completed, tt.target)
			if got != tt.want {
				t.Errorf("isTaskAtOrPastTarget(tasks, %q, %q) = %v, want %v",
					tt.completed, tt.target, got, tt.want)
			}
		})
	}
}
