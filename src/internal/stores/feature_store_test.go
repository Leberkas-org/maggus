package stores

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leberkas-org/maggus/internal/parser"
)

// featureFixture creates a temp dir with .maggus/features/ and writes the given
// feature file content to feature_001.md. Returns the temp dir path.
func featureFixture(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	featDir := filepath.Join(dir, ".maggus", "features")
	if err := os.MkdirAll(featDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(featDir, "feature_001.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

const simpleFeature = `# Feature 001: Test Feature

### TASK-001-001: Do a thing
**Description:** Some task.
**Acceptance Criteria:**
- [ ] criterion one
- [ ] criterion two
`

const blockedFeature = `# Feature 001: Test Feature

### TASK-001-001: Do a thing
**Acceptance Criteria:**
- [ ] BLOCKED: some blocker
- [ ] normal criterion
`

const completedFeature = `# Feature 001: Test Feature

### TASK-001-001: Do a thing
**Acceptance Criteria:**
- [x] criterion one
- [x] criterion two
`

func TestFileFeatureStore_LoadAll(t *testing.T) {
	dir := featureFixture(t, simpleFeature)
	store := NewFileFeatureStore(dir)

	plans, err := store.LoadAll(false)
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(plans) != 1 {
		t.Fatalf("expected 1 plan, got %d", len(plans))
	}
	p := plans[0]
	if p.ID != "feature_001" {
		t.Errorf("ID = %q, want feature_001", p.ID)
	}
	if p.IsBug {
		t.Error("IsBug should be false for feature files")
	}
	if p.Completed {
		t.Error("Completed should be false")
	}
	if len(p.Tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(p.Tasks))
	}
	if p.Tasks[0].ID != "TASK-001-001" {
		t.Errorf("task ID = %q, want TASK-001-001", p.Tasks[0].ID)
	}
}

func TestFileFeatureStore_LoadAll_includeCompleted(t *testing.T) {
	dir := featureFixture(t, simpleFeature)
	featDir := filepath.Join(dir, ".maggus", "features")

	// Add a _completed.md file
	completedPath := filepath.Join(featDir, "feature_001_completed.md")
	if err := os.WriteFile(completedPath, []byte(completedFeature), 0o644); err != nil {
		t.Fatal(err)
	}

	store := NewFileFeatureStore(dir)

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

func TestFileFeatureStore_GlobFiles(t *testing.T) {
	dir := featureFixture(t, simpleFeature)
	store := NewFileFeatureStore(dir)

	files, err := store.GlobFiles(false)
	if err != nil {
		t.Fatalf("GlobFiles: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if !strings.HasSuffix(files[0], "feature_001.md") {
		t.Errorf("unexpected file path: %s", files[0])
	}
}

func TestFileFeatureStore_MarkCompleted(t *testing.T) {
	dir := featureFixture(t, completedFeature)
	store := NewFileFeatureStore(dir)

	completed, err := store.MarkCompleted("rename")
	if err != nil {
		t.Fatalf("MarkCompleted: %v", err)
	}
	if len(completed) != 1 {
		t.Fatalf("expected 1 completed file, got %d", len(completed))
	}

	// Original file should no longer exist; _completed.md should exist
	original := filepath.Join(dir, ".maggus", "features", "feature_001.md")
	renamed := filepath.Join(dir, ".maggus", "features", "feature_001_completed.md")

	if _, err := os.Stat(original); !os.IsNotExist(err) {
		t.Error("original file should have been renamed away")
	}
	if _, err := os.Stat(renamed); os.IsNotExist(err) {
		t.Error("_completed.md file should exist after rename")
	}
}

func TestFileFeatureStore_DeleteTask(t *testing.T) {
	dir := featureFixture(t, simpleFeature)
	featFile := filepath.Join(dir, ".maggus", "features", "feature_001.md")
	store := NewFileFeatureStore(dir)

	if err := store.DeleteTask(featFile, "TASK-001-001"); err != nil {
		t.Fatalf("DeleteTask: %v", err)
	}

	tasks, err := parser.ParseFile(featFile)
	if err != nil {
		t.Fatalf("ParseFile after delete: %v", err)
	}
	if len(tasks) != 0 {
		t.Errorf("expected 0 tasks after delete, got %d", len(tasks))
	}
}

func TestFileFeatureStore_UnblockCriterion(t *testing.T) {
	dir := featureFixture(t, blockedFeature)
	featFile := filepath.Join(dir, ".maggus", "features", "feature_001.md")
	store := NewFileFeatureStore(dir)

	c := parser.Criterion{Text: "BLOCKED: some blocker", Checked: false, Blocked: true}
	if err := store.UnblockCriterion(featFile, c); err != nil {
		t.Fatalf("UnblockCriterion: %v", err)
	}

	tasks, err := parser.ParseFile(featFile)
	if err != nil {
		t.Fatalf("ParseFile after unblock: %v", err)
	}
	if tasks[0].IsBlocked() {
		t.Error("task should no longer be blocked after UnblockCriterion")
	}
}

func TestFileFeatureStore_ResolveCriterion(t *testing.T) {
	dir := featureFixture(t, blockedFeature)
	featFile := filepath.Join(dir, ".maggus", "features", "feature_001.md")
	store := NewFileFeatureStore(dir)

	c := parser.Criterion{Text: "BLOCKED: some blocker", Checked: false, Blocked: true}
	if err := store.ResolveCriterion(featFile, c); err != nil {
		t.Fatalf("ResolveCriterion: %v", err)
	}

	tasks, err := parser.ParseFile(featFile)
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

func TestFileFeatureStore_DeleteCriterion(t *testing.T) {
	dir := featureFixture(t, blockedFeature)
	featFile := filepath.Join(dir, ".maggus", "features", "feature_001.md")
	store := NewFileFeatureStore(dir)

	c := parser.Criterion{Text: "BLOCKED: some blocker", Checked: false, Blocked: true}
	if err := store.DeleteCriterion(featFile, c); err != nil {
		t.Fatalf("DeleteCriterion: %v", err)
	}

	tasks, err := parser.ParseFile(featFile)
	if err != nil {
		t.Fatalf("ParseFile after delete criterion: %v", err)
	}
	for _, cr := range tasks[0].Criteria {
		if strings.Contains(cr.Text, "BLOCKED:") {
			t.Errorf("deleted criterion still present: %s", cr.Text)
		}
	}
}
