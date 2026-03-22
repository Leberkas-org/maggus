package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/leberkas-org/maggus/internal/parser"
)

// mockPrinter captures Println/Printf calls for testing.
type mockPrinter struct {
	messages []string
}

func (m *mockPrinter) Println(args ...interface{}) {
	for _, a := range args {
		if s, ok := a.(string); ok {
			m.messages = append(m.messages, s)
		}
	}
}

func (m *mockPrinter) Printf(format string, args ...interface{}) {}

// TestMergeBugAndFeatureTasks_BugsFirst verifies bugs appear before features.
func TestMergeBugAndFeatureTasks_BugsFirst(t *testing.T) {
	bugs := []parser.Task{
		{ID: "BUG-001-001", Title: "Fix crash"},
		{ID: "BUG-001-002", Title: "Fix leak"},
	}
	features := []parser.Task{
		{ID: "TASK-001-001", Title: "Add feature"},
		{ID: "TASK-001-002", Title: "Add another"},
	}

	merged := mergeBugAndFeatureTasks(bugs, features)

	if len(merged) != 4 {
		t.Fatalf("expected 4 tasks, got %d", len(merged))
	}
	if merged[0].ID != "BUG-001-001" {
		t.Errorf("expected first task to be BUG-001-001, got %s", merged[0].ID)
	}
	if merged[1].ID != "BUG-001-002" {
		t.Errorf("expected second task to be BUG-001-002, got %s", merged[1].ID)
	}
	if merged[2].ID != "TASK-001-001" {
		t.Errorf("expected third task to be TASK-001-001, got %s", merged[2].ID)
	}
	if merged[3].ID != "TASK-001-002" {
		t.Errorf("expected fourth task to be TASK-001-002, got %s", merged[3].ID)
	}
}

// TestMergeBugAndFeatureTasks_NoBugs verifies features-only case works.
func TestMergeBugAndFeatureTasks_NoBugs(t *testing.T) {
	features := []parser.Task{
		{ID: "TASK-001-001", Title: "Add feature"},
	}
	merged := mergeBugAndFeatureTasks(nil, features)
	if len(merged) != 1 || merged[0].ID != "TASK-001-001" {
		t.Errorf("expected single feature task, got %v", merged)
	}
}

// TestMergeBugAndFeatureTasks_NoFeatures verifies bugs-only case works.
func TestMergeBugAndFeatureTasks_NoFeatures(t *testing.T) {
	bugs := []parser.Task{
		{ID: "BUG-001-001", Title: "Fix crash"},
	}
	merged := mergeBugAndFeatureTasks(bugs, nil)
	if len(merged) != 1 || merged[0].ID != "BUG-001-001" {
		t.Errorf("expected single bug task, got %v", merged)
	}
}

// TestMergeBugAndFeatureTasks_Empty verifies empty case returns empty.
func TestMergeBugAndFeatureTasks_Empty(t *testing.T) {
	merged := mergeBugAndFeatureTasks(nil, nil)
	if len(merged) != 0 {
		t.Errorf("expected empty slice, got %v", merged)
	}
}

// TestFindInitialTask_BugTaskFlag verifies --task flag works with BUG- IDs.
func TestFindInitialTask_BugTaskFlag(t *testing.T) {
	// Set the taskFlag to a bug task ID.
	oldFlag := taskFlag
	taskFlag = "BUG-001-001"
	defer func() { taskFlag = oldFlag }()

	tasks := []parser.Task{
		{ID: "BUG-001-001", Title: "Fix crash", Criteria: []parser.Criterion{{Text: "A", Checked: false}}},
		{ID: "TASK-001-001", Title: "Add feature", Criteria: []parser.Criterion{{Text: "B", Checked: false}}},
	}

	mp := &mockPrinter{}
	next, done := findInitialTask(mp, tasks)
	if done {
		t.Fatal("expected done=false")
	}
	if next == nil {
		t.Fatal("expected non-nil task")
	}
	if next.ID != "BUG-001-001" {
		t.Errorf("expected BUG-001-001, got %s", next.ID)
	}
}

// TestFindNextIncomplete_BugsBeforeFeatures verifies that FindNextIncomplete
// returns the first bug task when bugs appear before features in the merged list.
func TestFindNextIncomplete_BugsBeforeFeatures(t *testing.T) {
	tasks := []parser.Task{
		{ID: "BUG-001-001", Title: "Fix crash", Criteria: []parser.Criterion{{Text: "A", Checked: false}}},
		{ID: "TASK-001-001", Title: "Add feature", Criteria: []parser.Criterion{{Text: "B", Checked: false}}},
	}

	next := parser.FindNextIncomplete(tasks)
	if next == nil {
		t.Fatal("expected non-nil task")
	}
	if next.ID != "BUG-001-001" {
		t.Errorf("expected BUG-001-001, got %s", next.ID)
	}
}

// TestFindNextIncomplete_CompleteBugsSkipsToFeatures verifies that completed
// bugs are skipped and the first workable feature is returned.
func TestFindNextIncomplete_CompleteBugsSkipsToFeatures(t *testing.T) {
	tasks := []parser.Task{
		{ID: "BUG-001-001", Title: "Fixed", Criteria: []parser.Criterion{{Text: "A", Checked: true}}},
		{ID: "TASK-001-001", Title: "Add feature", Criteria: []parser.Criterion{{Text: "B", Checked: false}}},
	}

	next := parser.FindNextIncomplete(tasks)
	if next == nil {
		t.Fatal("expected non-nil task")
	}
	if next.ID != "TASK-001-001" {
		t.Errorf("expected TASK-001-001, got %s", next.ID)
	}
}

// TestCapCount_MergedTasks verifies capCount counts workable tasks from both sources.
func TestCapCount_MergedTasks(t *testing.T) {
	oldFlag := taskFlag
	taskFlag = ""
	defer func() { taskFlag = oldFlag }()

	tasks := []parser.Task{
		{ID: "BUG-001-001", Title: "Fix", Criteria: []parser.Criterion{{Text: "A", Checked: false}}},
		{ID: "BUG-001-002", Title: "Fixed", Criteria: []parser.Criterion{{Text: "B", Checked: true}}}, // complete
		{ID: "TASK-001-001", Title: "Add", Criteria: []parser.Criterion{{Text: "C", Checked: false}}},
		{ID: "TASK-001-002", Title: "Block", Criteria: []parser.Criterion{{Text: "BLOCKED: dep", Checked: false, Blocked: true}}}, // blocked
	}

	// 0 means "all workable" — should return 2 (one bug + one feature).
	got := capCount(tasks, 0)
	if got != 2 {
		t.Errorf("capCount(tasks, 0) = %d, want 2", got)
	}

	// Explicit count higher than workable.
	got = capCount(tasks, 10)
	if got != 2 {
		t.Errorf("capCount(tasks, 10) = %d, want 2", got)
	}

	// Explicit count lower than workable.
	got = capCount(tasks, 1)
	if got != 1 {
		t.Errorf("capCount(tasks, 1) = %d, want 1", got)
	}
}

// TestCapCount_BugTaskFlag verifies --task flag returns 1 for bug tasks.
func TestCapCount_BugTaskFlag(t *testing.T) {
	oldFlag := taskFlag
	taskFlag = "BUG-001-001"
	defer func() { taskFlag = oldFlag }()

	tasks := []parser.Task{
		{ID: "BUG-001-001", Title: "Fix", Criteria: []parser.Criterion{{Text: "A", Checked: false}}},
		{ID: "TASK-001-001", Title: "Add", Criteria: []parser.Criterion{{Text: "B", Checked: false}}},
	}

	got := capCount(tasks, 0)
	if got != 1 {
		t.Errorf("capCount with --task flag = %d, want 1", got)
	}
}

// TestInitIteration_MergedBugsAndFeatures verifies initIteration parses both
// bugs and features, returning bugs first in the task list.
func TestInitIteration_MergedBugsAndFeatures(t *testing.T) {
	oldFlag := taskFlag
	taskFlag = ""
	defer func() { taskFlag = oldFlag }()

	dir := t.TempDir()

	// Create bug file.
	bugsDir := filepath.Join(dir, ".maggus", "bugs")
	if err := os.MkdirAll(bugsDir, 0755); err != nil {
		t.Fatal(err)
	}
	bugContent := `# Bug 1
## Tasks
### BUG-001-001: Fix crash
**Acceptance Criteria:**
- [ ] Fix the crash
`
	if err := os.WriteFile(filepath.Join(bugsDir, "bug_1.md"), []byte(bugContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create feature file.
	featDir := filepath.Join(dir, ".maggus", "features")
	if err := os.MkdirAll(featDir, 0755); err != nil {
		t.Fatal(err)
	}
	featContent := `# Feature 1
## Tasks
### TASK-001-001: Add feature
**Acceptance Criteria:**
- [ ] Add the feature
`
	if err := os.WriteFile(filepath.Join(featDir, "feature_1.md"), []byte(featContent), 0644); err != nil {
		t.Fatal(err)
	}

	mp := &mockPrinter{}
	setup, err := initIteration(mp, dir, "test-model", 0)
	if err != nil {
		t.Fatalf("initIteration error: %v", err)
	}
	if setup == nil {
		t.Fatal("expected non-nil setup")
	}

	// Verify bugs come first.
	if len(setup.tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(setup.tasks))
	}
	if setup.tasks[0].ID != "BUG-001-001" {
		t.Errorf("expected first task BUG-001-001, got %s", setup.tasks[0].ID)
	}
	if setup.tasks[1].ID != "TASK-001-001" {
		t.Errorf("expected second task TASK-001-001, got %s", setup.tasks[1].ID)
	}

	// Next task should be the bug.
	if setup.next.ID != "BUG-001-001" {
		t.Errorf("expected next task BUG-001-001, got %s", setup.next.ID)
	}

	// Count should be 2 (both workable).
	if setup.count != 2 {
		t.Errorf("expected count 2, got %d", setup.count)
	}
}

// TestInitIteration_BugTaskTargeting verifies --task flag works with BUG- IDs.
func TestInitIteration_BugTaskTargeting(t *testing.T) {
	oldFlag := taskFlag
	taskFlag = "BUG-001-001"
	defer func() { taskFlag = oldFlag }()

	dir := t.TempDir()

	bugsDir := filepath.Join(dir, ".maggus", "bugs")
	if err := os.MkdirAll(bugsDir, 0755); err != nil {
		t.Fatal(err)
	}
	bugContent := `# Bug 1
## Tasks
### BUG-001-001: Fix crash
**Acceptance Criteria:**
- [ ] Fix the crash
`
	if err := os.WriteFile(filepath.Join(bugsDir, "bug_1.md"), []byte(bugContent), 0644); err != nil {
		t.Fatal(err)
	}

	featDir := filepath.Join(dir, ".maggus", "features")
	if err := os.MkdirAll(featDir, 0755); err != nil {
		t.Fatal(err)
	}
	featContent := `# Feature 1
## Tasks
### TASK-001-001: Add feature
**Acceptance Criteria:**
- [ ] Add the feature
`
	if err := os.WriteFile(filepath.Join(featDir, "feature_1.md"), []byte(featContent), 0644); err != nil {
		t.Fatal(err)
	}

	mp := &mockPrinter{}
	setup, err := initIteration(mp, dir, "test-model", 0)
	if err != nil {
		t.Fatalf("initIteration error: %v", err)
	}
	if setup == nil {
		t.Fatal("expected non-nil setup")
	}

	if setup.next.ID != "BUG-001-001" {
		t.Errorf("expected targeted task BUG-001-001, got %s", setup.next.ID)
	}
	if setup.count != 1 {
		t.Errorf("expected count 1 with --task flag, got %d", setup.count)
	}
}

// TestInitIteration_OnlyFeaturesNoBugs verifies the old behavior still works.
func TestInitIteration_OnlyFeaturesNoBugs(t *testing.T) {
	oldFlag := taskFlag
	taskFlag = ""
	defer func() { taskFlag = oldFlag }()

	dir := t.TempDir()

	featDir := filepath.Join(dir, ".maggus", "features")
	if err := os.MkdirAll(featDir, 0755); err != nil {
		t.Fatal(err)
	}
	featContent := `# Feature 1
## Tasks
### TASK-001-001: Add feature
**Acceptance Criteria:**
- [ ] Add the feature
`
	if err := os.WriteFile(filepath.Join(featDir, "feature_1.md"), []byte(featContent), 0644); err != nil {
		t.Fatal(err)
	}

	mp := &mockPrinter{}
	setup, err := initIteration(mp, dir, "test-model", 0)
	if err != nil {
		t.Fatalf("initIteration error: %v", err)
	}
	if setup == nil {
		t.Fatal("expected non-nil setup")
	}
	if setup.next.ID != "TASK-001-001" {
		t.Errorf("expected TASK-001-001, got %s", setup.next.ID)
	}
}

// TestInitIteration_EmptyNoBugsNoFeatures verifies proper message when no files exist.
func TestInitIteration_EmptyNoBugsNoFeatures(t *testing.T) {
	oldFlag := taskFlag
	taskFlag = ""
	defer func() { taskFlag = oldFlag }()

	dir := t.TempDir()

	mp := &mockPrinter{}
	setup, err := initIteration(mp, dir, "test-model", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if setup != nil {
		t.Errorf("expected nil setup, got %+v", setup)
	}
	if len(mp.messages) == 0 {
		t.Fatal("expected informational message")
	}
	found := false
	for _, msg := range mp.messages {
		if msg == "No feature or bug files found." {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'No feature or bug files found.' message, got %v", mp.messages)
	}
}

// TestCountWorkable verifies countWorkable returns only workable (incomplete, not blocked, not ignored) tasks.
func TestCountWorkable(t *testing.T) {
	tasks := []parser.Task{
		{ID: "BUG-001-001", Title: "Fix", Criteria: []parser.Criterion{{Text: "A", Checked: false}}},
		{ID: "BUG-001-002", Title: "Fixed", Criteria: []parser.Criterion{{Text: "B", Checked: true}}},                        // complete
		{ID: "TASK-001-001", Title: "Add", Criteria: []parser.Criterion{{Text: "C", Checked: false}}},                          // workable
		{ID: "TASK-001-002", Title: "Block", Criteria: []parser.Criterion{{Text: "BLOCKED: dep", Checked: false, Blocked: true}}}, // blocked
		{ID: "TASK-001-003", Title: "Ign", Criteria: []parser.Criterion{{Text: "D", Checked: false}}, Ignored: true},            // ignored
	}

	got := countWorkable(tasks)
	if got != 2 {
		t.Errorf("countWorkable = %d, want 2", got)
	}

	// Empty list.
	if countWorkable(nil) != 0 {
		t.Errorf("countWorkable(nil) != 0")
	}
}

// TestFindTaskByID_BugID verifies findTaskByID works with BUG- prefixed IDs.
func TestFindTaskByID_BugID(t *testing.T) {
	tasks := []parser.Task{
		{ID: "BUG-001-001", Title: "Fix crash", Criteria: []parser.Criterion{{Text: "A", Checked: false}}},
		{ID: "TASK-001-001", Title: "Add feature", Criteria: []parser.Criterion{{Text: "B", Checked: false}}},
	}

	got := findTaskByID(tasks, "BUG-001-001")
	if got == nil {
		t.Fatal("expected non-nil task")
	}
	if got.ID != "BUG-001-001" {
		t.Errorf("expected BUG-001-001, got %s", got.ID)
	}
}
