package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/leberkas-org/maggus/internal/approval"
	"github.com/leberkas-org/maggus/internal/config"
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
		{ID: "BUG-001-002", Title: "Fixed", Criteria: []parser.Criterion{{Text: "B", Checked: true}}},                             // complete
		{ID: "TASK-001-001", Title: "Add", Criteria: []parser.Criterion{{Text: "C", Checked: false}}},                             // workable
		{ID: "TASK-001-002", Title: "Block", Criteria: []parser.Criterion{{Text: "BLOCKED: dep", Checked: false, Blocked: true}}}, // blocked
		{ID: "TASK-001-003", Title: "Ign", Criteria: []parser.Criterion{{Text: "D", Checked: false}}, Ignored: true},              // ignored
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

// TestProgressTotal_UnlimitedMode verifies the progress total is computed from
// the refreshed task list and never shrinks when new workable tasks appear.
func TestProgressTotal_UnlimitedMode(t *testing.T) {
	// Simulate: iteration i=0 completed, 2 workable tasks remain.
	parsedTasks := []parser.Task{
		// task that was just completed (now complete)
		{ID: "TASK-001-001", Title: "Done", Criteria: []parser.Criterion{{Text: "A", Checked: true}}},
		// 2 remaining workable tasks
		{ID: "TASK-001-002", Title: "Next1", Criteria: []parser.Criterion{{Text: "B", Checked: false}}},
		{ID: "TASK-001-003", Title: "Next2", Criteria: []parser.Criterion{{Text: "C", Checked: false}}},
	}

	i := 0        // first iteration (0-based)
	maxCount := 0 // unlimited

	progressTotal := (i + 1) + countWorkable(parsedTasks)
	if maxCount > 0 && progressTotal > maxCount {
		progressTotal = maxCount
	}

	// Expected: 1 completed + 2 remaining = 3
	if progressTotal != 3 {
		t.Errorf("progressTotal = %d, want 3", progressTotal)
	}
}

// TestProgressTotal_NewFilesAdded verifies bar does not shrink when extra tasks appear.
func TestProgressTotal_NewFilesAdded(t *testing.T) {
	// Before iteration: displayCount was 2 (i=0, remaining=2 => i+remaining=2).
	// After iteration: 3 workable tasks exist (a new file was added).
	parsedTasks := []parser.Task{
		{ID: "TASK-001-001", Title: "Done", Criteria: []parser.Criterion{{Text: "A", Checked: true}}},
		{ID: "TASK-001-002", Title: "Next1", Criteria: []parser.Criterion{{Text: "B", Checked: false}}},
		{ID: "TASK-001-003", Title: "Next2", Criteria: []parser.Criterion{{Text: "C", Checked: false}}},
		{ID: "TASK-001-004", Title: "New", Criteria: []parser.Criterion{{Text: "D", Checked: false}}}, // newly added
	}

	i := 0
	oldDisplayCount := 2 // what count was before runTask (stale)
	maxCount := 0        // unlimited

	progressTotal := (i + 1) + countWorkable(parsedTasks)
	if maxCount > 0 && progressTotal > maxCount {
		progressTotal = maxCount
	}

	// New total (4) should be greater than the stale displayCount (2).
	if progressTotal <= oldDisplayCount {
		t.Errorf("progressTotal %d should exceed stale displayCount %d", progressTotal, oldDisplayCount)
	}
	if progressTotal != 4 {
		t.Errorf("progressTotal = %d, want 4", progressTotal)
	}
}

// TestProgressTotal_BoundedModeCap verifies the total is capped at user-requested count.
func TestProgressTotal_BoundedModeCap(t *testing.T) {
	// User ran `maggus work 2`, but 3 workable tasks remain after the agent run.
	parsedTasks := []parser.Task{
		{ID: "TASK-001-002", Title: "Next1", Criteria: []parser.Criterion{{Text: "B", Checked: false}}},
		{ID: "TASK-001-003", Title: "Next2", Criteria: []parser.Criterion{{Text: "C", Checked: false}}},
		{ID: "TASK-001-004", Title: "Extra", Criteria: []parser.Criterion{{Text: "D", Checked: false}}},
	}

	i := 0
	maxCount := 2 // user requested only 2 tasks

	progressTotal := (i + 1) + countWorkable(parsedTasks)
	if maxCount > 0 && progressTotal > maxCount {
		progressTotal = maxCount
	}

	// Should be capped at 2, not (1+3)=4.
	if progressTotal != 2 {
		t.Errorf("progressTotal = %d, want 2 (bounded cap)", progressTotal)
	}
}

// TestProgressTotal_BoundedModeNoCap verifies bounded mode still shows correct total
// when the refreshed count is within the requested limit.
func TestProgressTotal_BoundedModeNoCap(t *testing.T) {
	// User ran `maggus work 3`, 1 workable task remains after first task.
	parsedTasks := []parser.Task{
		{ID: "TASK-001-002", Title: "Next1", Criteria: []parser.Criterion{{Text: "B", Checked: false}}},
	}

	i := 0
	maxCount := 3

	progressTotal := (i + 1) + countWorkable(parsedTasks)
	if maxCount > 0 && progressTotal > maxCount {
		progressTotal = maxCount
	}

	// (1+1)=2, capped at 3 → stays at 2.
	if progressTotal != 2 {
		t.Errorf("progressTotal = %d, want 2", progressTotal)
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

// TestParseAllTasksPicksUpNewFile verifies that parseAllTasks returns newly-added
// tasks when called a second time after a file is written to disk.
// This is the mechanism the unlimited-mode re-parse relies on to avoid
// premature exit when tasks are added during a run.
func TestParseAllTasksPicksUpNewFile(t *testing.T) {
	dir := t.TempDir()

	// Create a feature dir with one already-completed task.
	featDir := filepath.Join(dir, ".maggus", "features")
	if err := os.MkdirAll(featDir, 0755); err != nil {
		t.Fatal(err)
	}
	completedContent := `# Feature 1
## Tasks
### TASK-001-001: Done
**Acceptance Criteria:**
- [x] Already done
`
	if err := os.WriteFile(filepath.Join(featDir, "feature_1.md"), []byte(completedContent), 0644); err != nil {
		t.Fatal(err)
	}

	// First parse: no workable tasks.
	tasks, err := parseAllTasks(dir)
	if err != nil {
		t.Fatalf("parseAllTasks error: %v", err)
	}
	if countWorkable(tasks) != 0 {
		t.Fatalf("expected 0 workable tasks before new file, got %d", countWorkable(tasks))
	}

	// Simulate user adding a new feature file mid-run.
	newContent := `# Feature 2
## Tasks
### TASK-002-001: New task
**Acceptance Criteria:**
- [ ] Do the new thing
`
	if err := os.WriteFile(filepath.Join(featDir, "feature_2.md"), []byte(newContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Second parse: should find the new workable task.
	freshTasks, err := parseAllTasks(dir)
	if err != nil {
		t.Fatalf("parseAllTasks (fresh) error: %v", err)
	}
	if countWorkable(freshTasks) != 1 {
		t.Fatalf("expected 1 workable task after new file added, got %d", countWorkable(freshTasks))
	}
	if freshTasks[len(freshTasks)-1].ID != "TASK-002-001" {
		t.Errorf("expected new task TASK-002-001, got %s", freshTasks[len(freshTasks)-1].ID)
	}
}

// TestUnlimitedModeReparseSentinel verifies the re-parse-before-break logic:
// when remaining==0 but a fresh re-parse finds workable tasks, the loop
// should continue (remaining becomes positive), not break.
func TestUnlimitedModeReparseSentinel(t *testing.T) {
	// Simulate the re-parse decision: initial tasks list exhausted,
	// fresh re-parse finds a new workable task.
	exhausted := []parser.Task{
		{ID: "TASK-001-001", Title: "Done", Criteria: []parser.Criterion{{Text: "A", Checked: true}}},
	}
	if countWorkable(exhausted) != 0 {
		t.Fatalf("pre-condition: expected 0 workable tasks in exhausted list")
	}

	// Simulate fresh re-parse finding a new task.
	freshTasks := []parser.Task{
		{ID: "TASK-001-001", Title: "Done", Criteria: []parser.Criterion{{Text: "A", Checked: true}}},
		{ID: "TASK-002-001", Title: "New task", Criteria: []parser.Criterion{{Text: "B", Checked: false}}},
	}

	// Decision: if fresh re-parse has workable tasks, we should continue (not break).
	shouldContinue := countWorkable(freshTasks) > 0
	if !shouldContinue {
		t.Error("expected shouldContinue=true when fresh re-parse finds workable tasks")
	}

	// Verify remaining is updated after adopting fresh tasks.
	tasks := freshTasks
	remaining := countWorkable(tasks)
	if remaining != 1 {
		t.Errorf("expected remaining=1 after fresh re-parse, got %d", remaining)
	}
}

// TestUnlimitedModeReparseNoNewTasks verifies that when remaining==0 and the
// fresh re-parse also finds no workable tasks, the loop breaks normally.
func TestUnlimitedModeReparseNoNewTasks(t *testing.T) {
	exhausted := []parser.Task{
		{ID: "TASK-001-001", Title: "Done", Criteria: []parser.Criterion{{Text: "A", Checked: true}}},
	}

	// Simulate fresh re-parse also finding no workable tasks.
	freshTasks := []parser.Task{
		{ID: "TASK-001-001", Title: "Done", Criteria: []parser.Criterion{{Text: "A", Checked: true}}},
	}

	_ = exhausted // both lists exhausted

	// Decision: if fresh re-parse has no workable tasks, we should break.
	shouldContinue := countWorkable(freshTasks) > 0
	if shouldContinue {
		t.Error("expected shouldContinue=false when fresh re-parse also finds no workable tasks")
	}
}

// ─── Feature-centric tests ────────────────────────────────────────────────────

// incompleteTaskContent returns a feature file body with one incomplete task.
func incompleteTaskContent(taskID, taskTitle string) string {
	return "# Feature\n## Tasks\n### " + taskID + ": " + taskTitle + "\n**Acceptance Criteria:**\n- [ ] Done\n"
}

// incompleteBugContent returns a bug file body with one incomplete task.
func incompleteBugContent(taskID, taskTitle string) string {
	return "# Bug\n## Tasks\n### " + taskID + ": " + taskTitle + "\n**Acceptance Criteria:**\n- [ ] Done\n"
}

// writeApprovals writes an approval file granting approval to the given IDs.
func writeApprovals(t *testing.T, dir string, approvedIDs ...string) {
	t.Helper()
	maggusDir := filepath.Join(dir, ".maggus")
	if err := os.MkdirAll(maggusDir, 0755); err != nil {
		t.Fatal(err)
	}
	a := make(approval.Approvals)
	for _, id := range approvedIDs {
		a[id] = true
	}
	if err := approval.Save(dir, a); err != nil {
		t.Fatal(err)
	}
}

// TestBuildApprovedFeatureGroups_OptOutAllApproved verifies that opt-out mode
// approves all groups by default (no approval file needed).
func TestBuildApprovedFeatureGroups_OptOutAllApproved(t *testing.T) {
	dir := setupCleanDir(t)
	writeFeatureFile(t, dir, "feature_001.md", incompleteTaskContent("TASK-001-001", "Add feature"))
	writeFeatureFile(t, dir, "feature_002.md", incompleteTaskContent("TASK-002-001", "Add another"))

	cfg := config.Config{ApprovalMode: config.ApprovalModeOptOut}
	groups, err := buildApprovedFeatureGroups(dir, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(groups) != 2 {
		t.Errorf("opt-out: expected 2 groups, got %d", len(groups))
	}
}

// TestBuildApprovedFeatureGroups_OptInNoApprovals verifies that opt-in mode
// returns no groups when nothing has been approved.
func TestBuildApprovedFeatureGroups_OptInNoApprovals(t *testing.T) {
	dir := setupCleanDir(t)
	writeFeatureFile(t, dir, "feature_001.md", incompleteTaskContent("TASK-001-001", "Add feature"))

	cfg := config.Config{ApprovalMode: config.ApprovalModeOptIn}
	groups, err := buildApprovedFeatureGroups(dir, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(groups) != 0 {
		t.Errorf("opt-in with no approvals: expected 0 groups, got %d", len(groups))
	}
}

// TestBuildApprovedFeatureGroups_OptInPartialApproval verifies that only
// explicitly approved features are included in opt-in mode.
func TestBuildApprovedFeatureGroups_OptInPartialApproval(t *testing.T) {
	dir := setupCleanDir(t)
	writeFeatureFile(t, dir, "feature_001.md", incompleteTaskContent("TASK-001-001", "Add feature"))
	writeFeatureFile(t, dir, "feature_002.md", incompleteTaskContent("TASK-002-001", "Add another"))
	writeApprovals(t, dir, "feature_001")

	cfg := config.Config{ApprovalMode: config.ApprovalModeOptIn}
	groups, err := buildApprovedFeatureGroups(dir, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(groups) != 1 {
		t.Errorf("opt-in with one approval: expected 1 group, got %d", len(groups))
	}
	if groups[0].id != "feature_001" {
		t.Errorf("expected group id 'feature_001', got %q", groups[0].id)
	}
}

// TestBuildApprovedFeatureGroups_BugsFirst verifies that bug groups come before
// feature groups in the returned list.
func TestBuildApprovedFeatureGroups_BugsFirst(t *testing.T) {
	dir := setupCleanDir(t)
	writeFeatureFile(t, dir, "feature_001.md", incompleteTaskContent("TASK-001-001", "Add feature"))
	writeBugFile(t, dir, "bug_001.md", incompleteBugContent("BUG-001-001", "Fix crash"))
	// Approve both
	writeApprovals(t, dir, "feature_001", "bug_001")

	cfg := config.Config{ApprovalMode: config.ApprovalModeOptIn}
	groups, err := buildApprovedFeatureGroups(dir, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}
	if !groups[0].isBug {
		t.Errorf("expected first group to be a bug, got id=%q", groups[0].id)
	}
	if groups[1].isBug {
		t.Errorf("expected second group to be a feature, got id=%q", groups[1].id)
	}
}

// TestBuildApprovedFeatureGroups_EmptyDir verifies that an empty directory
// returns an empty list without error.
func TestBuildApprovedFeatureGroups_EmptyDir(t *testing.T) {
	dir := setupCleanDir(t)
	cfg := config.Config{ApprovalMode: config.ApprovalModeOptOut}
	groups, err := buildApprovedFeatureGroups(dir, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(groups) != 0 {
		t.Errorf("empty dir: expected 0 groups, got %d", len(groups))
	}
}

// TestFilterTasksBySourceFile verifies that only tasks from the matching source
// file are returned.
func TestFilterTasksBySourceFile(t *testing.T) {
	tasks := []parser.Task{
		{ID: "TASK-001-001", SourceFile: "/path/to/feature_001.md"},
		{ID: "TASK-001-002", SourceFile: "/path/to/feature_001.md"},
		{ID: "TASK-002-001", SourceFile: "/path/to/feature_002.md"},
	}

	got := filterTasksBySourceFile(tasks, "/path/to/feature_001.md")
	if len(got) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(got))
	}
	for _, t2 := range got {
		if t2.SourceFile != "/path/to/feature_001.md" {
			t.Errorf("unexpected source file %q", t2.SourceFile)
		}
	}
}

// TestFilterTasksBySourceFile_NoMatch verifies empty result when no tasks match.
func TestFilterTasksBySourceFile_NoMatch(t *testing.T) {
	tasks := []parser.Task{
		{ID: "TASK-001-001", SourceFile: "/path/to/feature_001.md"},
	}
	got := filterTasksBySourceFile(tasks, "/path/to/feature_999.md")
	if len(got) != 0 {
		t.Errorf("expected 0 tasks, got %d", len(got))
	}
}

// TestFindGroupForTask_Found verifies finding a group by task ID.
func TestFindGroupForTask_Found(t *testing.T) {
	groups := []featureGroup{
		{
			id: "feature_001",
			tasks: []parser.Task{
				{ID: "TASK-001-001", Criteria: []parser.Criterion{{Text: "A", Checked: false}}},
			},
		},
		{
			id: "feature_002",
			tasks: []parser.Task{
				{ID: "TASK-002-001", Criteria: []parser.Criterion{{Text: "B", Checked: false}}},
			},
		},
	}

	got := findGroupForTask(groups, "TASK-002-001")
	if got == nil {
		t.Fatal("expected non-nil group")
	}
	if got.id != "feature_002" {
		t.Errorf("expected feature_002, got %q", got.id)
	}
}

// TestFindGroupForTask_CompletedTaskNotFound verifies that completed tasks are
// not matched (they cannot be targeted by --task).
func TestFindGroupForTask_CompletedTaskNotFound(t *testing.T) {
	groups := []featureGroup{
		{
			id: "feature_001",
			tasks: []parser.Task{
				{ID: "TASK-001-001", Criteria: []parser.Criterion{{Text: "A", Checked: true}}}, // complete
			},
		},
	}

	got := findGroupForTask(groups, "TASK-001-001")
	if got != nil {
		t.Errorf("expected nil for completed task, got %+v", got)
	}
}

// TestFindGroupForTask_NotFound verifies nil when task ID is unknown.
func TestFindGroupForTask_NotFound(t *testing.T) {
	groups := []featureGroup{
		{
			id: "feature_001",
			tasks: []parser.Task{
				{ID: "TASK-001-001", Criteria: []parser.Criterion{{Text: "A", Checked: false}}},
			},
		},
	}

	got := findGroupForTask(groups, "TASK-999-001")
	if got != nil {
		t.Errorf("expected nil for unknown task, got %+v", got)
	}
}

// TestFirstWorkableTask_Found verifies the first workable task is returned.
func TestFirstWorkableTask_Found(t *testing.T) {
	groups := []featureGroup{
		{
			id: "feature_001",
			tasks: []parser.Task{
				{ID: "TASK-001-001", Criteria: []parser.Criterion{{Text: "A", Checked: true}}},  // complete
				{ID: "TASK-001-002", Criteria: []parser.Criterion{{Text: "B", Checked: false}}}, // workable
			},
		},
	}

	got := firstWorkableTask(groups)
	if got == nil {
		t.Fatal("expected non-nil task")
	}
	if got.ID != "TASK-001-002" {
		t.Errorf("expected TASK-001-002, got %q", got.ID)
	}
}

// TestFirstWorkableTask_Empty verifies nil is returned when no workable task exists.
func TestFirstWorkableTask_Empty(t *testing.T) {
	groups := []featureGroup{
		{
			id: "feature_001",
			tasks: []parser.Task{
				{ID: "TASK-001-001", Criteria: []parser.Criterion{{Text: "A", Checked: true}}},
			},
		},
	}

	got := firstWorkableTask(groups)
	if got != nil {
		t.Errorf("expected nil, got %+v", got)
	}
}

// TestBuildApprovedFeatureGroups_CompletedFileExcluded verifies that _completed.md
// files are not included in the group list.
func TestBuildApprovedFeatureGroups_CompletedFileExcluded(t *testing.T) {
	dir := setupCleanDir(t)

	// Completed file — should be excluded by GlobFeatureFiles.
	completedPath := filepath.Join(dir, ".maggus", "features", "feature_001_completed.md")
	content := "# Feature\n## Tasks\n### TASK-001-001: Done\n**Acceptance Criteria:**\n- [x] Done\n"
	if err := os.WriteFile(completedPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := config.Config{ApprovalMode: config.ApprovalModeOptOut}
	groups, err := buildApprovedFeatureGroups(dir, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(groups) != 0 {
		t.Errorf("expected 0 groups (completed file excluded), got %d", len(groups))
	}
}
