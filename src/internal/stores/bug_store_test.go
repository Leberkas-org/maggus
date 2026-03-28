package stores

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leberkas-org/maggus/internal/parser"
)

// bugFixture creates a temp dir with .maggus/bugs/ and writes the given
// bug file content to bug_001.md. Returns the temp dir path.
func bugFixture(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	bugDir := filepath.Join(dir, ".maggus", "bugs")
	if err := os.MkdirAll(bugDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(bugDir, "bug_001.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

const simpleBug = `# Bug 001: Test Bug

### BUG-001-001: Fix a thing
**Description:** Some bug.
**Acceptance Criteria:**
- [ ] criterion one
- [ ] criterion two
`

const blockedBug = `# Bug 001: Test Bug

### BUG-001-001: Fix a thing
**Acceptance Criteria:**
- [ ] BLOCKED: some blocker
- [ ] normal criterion
`

const completedBug = `# Bug 001: Test Bug

### BUG-001-001: Fix a thing
**Acceptance Criteria:**
- [x] criterion one
- [x] criterion two
`

// legacyBug uses legacy TASK-NNN format (single number) to test migration.
const legacyBug = `# Bug 001: Test Bug

### TASK-001: Old style task
**Acceptance Criteria:**
- [ ] criterion one
`

func TestFileBugStore_LoadAll(t *testing.T) {
	dir := bugFixture(t, simpleBug)
	store := NewFileBugStore(dir)

	plans, err := store.LoadAll(false)
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(plans) != 1 {
		t.Fatalf("expected 1 plan, got %d", len(plans))
	}
	p := plans[0]
	if p.ID != "bug_001" {
		t.Errorf("ID = %q, want bug_001", p.ID)
	}
	if !p.IsBug {
		t.Error("IsBug should be true for bug files")
	}
	if p.Completed {
		t.Error("Completed should be false")
	}
	if len(p.Tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(p.Tasks))
	}
	if p.Tasks[0].ID != "BUG-001-001" {
		t.Errorf("task ID = %q, want BUG-001-001", p.Tasks[0].ID)
	}
}

func TestFileBugStore_LoadAll_includeCompleted(t *testing.T) {
	dir := bugFixture(t, simpleBug)
	bugDir := filepath.Join(dir, ".maggus", "bugs")

	// Add a _completed.md file
	completedPath := filepath.Join(bugDir, "bug_001_completed.md")
	if err := os.WriteFile(completedPath, []byte(completedBug), 0o644); err != nil {
		t.Fatal(err)
	}

	store := NewFileBugStore(dir)

	// Without completed
	plans, err := store.LoadAll(false)
	if err != nil {
		t.Fatalf("LoadAll(false): %v", err)
	}
	if len(plans) != 1 {
		t.Errorf("LoadAll(false): expected 1, got %d", len(plans))
	}

	// With completed
	plans, err = store.LoadAll(true)
	if err != nil {
		t.Fatalf("LoadAll(true): %v", err)
	}
	if len(plans) != 2 {
		t.Errorf("LoadAll(true): expected 2, got %d", len(plans))
	}
}

func TestFileBugStore_LoadAll_legacyMigration(t *testing.T) {
	dir := bugFixture(t, legacyBug)
	store := NewFileBugStore(dir)

	plans, err := store.LoadAll(false)
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(plans) != 1 {
		t.Fatalf("expected 1 plan, got %d", len(plans))
	}
	if len(plans[0].Tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(plans[0].Tasks))
	}
	// After migration, TASK-001-001 should be rewritten to BUG-001-001
	taskID := plans[0].Tasks[0].ID
	if !strings.HasPrefix(taskID, "BUG-") {
		t.Errorf("expected migrated BUG-NNN-NNN ID, got %q", taskID)
	}
}

func TestFileBugStore_GlobFiles(t *testing.T) {
	dir := bugFixture(t, simpleBug)
	store := NewFileBugStore(dir)

	files, err := store.GlobFiles(false)
	if err != nil {
		t.Fatalf("GlobFiles: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if !strings.HasSuffix(files[0], "bug_001.md") {
		t.Errorf("unexpected file path: %s", files[0])
	}
}

func TestFileBugStore_MarkCompleted(t *testing.T) {
	dir := bugFixture(t, completedBug)
	store := NewFileBugStore(dir)

	completed, err := store.MarkCompleted("rename")
	if err != nil {
		t.Fatalf("MarkCompleted: %v", err)
	}
	if len(completed) != 1 {
		t.Fatalf("expected 1 completed file, got %d", len(completed))
	}

	// Original file should no longer exist; _completed.md should exist
	original := filepath.Join(dir, ".maggus", "bugs", "bug_001.md")
	renamed := filepath.Join(dir, ".maggus", "bugs", "bug_001_completed.md")

	if _, err := os.Stat(original); !os.IsNotExist(err) {
		t.Error("original file should have been renamed away")
	}
	if _, err := os.Stat(renamed); os.IsNotExist(err) {
		t.Error("_completed.md file should exist after rename")
	}
}

func TestFileBugStore_DeleteTask(t *testing.T) {
	dir := bugFixture(t, simpleBug)
	bugFile := filepath.Join(dir, ".maggus", "bugs", "bug_001.md")
	store := NewFileBugStore(dir)

	if err := store.DeleteTask(bugFile, "BUG-001-001"); err != nil {
		t.Fatalf("DeleteTask: %v", err)
	}

	tasks, err := parser.ParseFile(bugFile)
	if err != nil {
		t.Fatalf("ParseFile after delete: %v", err)
	}
	if len(tasks) != 0 {
		t.Errorf("expected 0 tasks after delete, got %d", len(tasks))
	}
}

func TestFileBugStore_UnblockCriterion(t *testing.T) {
	dir := bugFixture(t, blockedBug)
	bugFile := filepath.Join(dir, ".maggus", "bugs", "bug_001.md")
	store := NewFileBugStore(dir)

	c := parser.Criterion{Text: "BLOCKED: some blocker", Checked: false, Blocked: true}
	if err := store.UnblockCriterion(bugFile, c); err != nil {
		t.Fatalf("UnblockCriterion: %v", err)
	}

	tasks, err := parser.ParseFile(bugFile)
	if err != nil {
		t.Fatalf("ParseFile after unblock: %v", err)
	}
	if tasks[0].IsBlocked() {
		t.Error("task should no longer be blocked after UnblockCriterion")
	}
}

func TestFileBugStore_ResolveCriterion(t *testing.T) {
	dir := bugFixture(t, blockedBug)
	bugFile := filepath.Join(dir, ".maggus", "bugs", "bug_001.md")
	store := NewFileBugStore(dir)

	c := parser.Criterion{Text: "BLOCKED: some blocker", Checked: false, Blocked: true}
	if err := store.ResolveCriterion(bugFile, c); err != nil {
		t.Fatalf("ResolveCriterion: %v", err)
	}

	tasks, err := parser.ParseFile(bugFile)
	if err != nil {
		t.Fatalf("ParseFile after resolve: %v", err)
	}
	// The blocker criterion should now be checked
	found := false
	for _, cr := range tasks[0].Criteria {
		if cr.Text == "some blocker" && cr.Checked {
			found = true
		}
	}
	if !found {
		t.Error("resolved criterion should be checked and unblocked")
	}
}

func TestFileBugStore_DeleteCriterion(t *testing.T) {
	dir := bugFixture(t, blockedBug)
	bugFile := filepath.Join(dir, ".maggus", "bugs", "bug_001.md")
	store := NewFileBugStore(dir)

	c := parser.Criterion{Text: "BLOCKED: some blocker", Checked: false, Blocked: true}
	if err := store.DeleteCriterion(bugFile, c); err != nil {
		t.Fatalf("DeleteCriterion: %v", err)
	}

	tasks, err := parser.ParseFile(bugFile)
	if err != nil {
		t.Fatalf("ParseFile after delete criterion: %v", err)
	}
	for _, cr := range tasks[0].Criteria {
		if strings.Contains(cr.Text, "BLOCKED:") {
			t.Errorf("deleted criterion still present: %s", cr.Text)
		}
	}
}
