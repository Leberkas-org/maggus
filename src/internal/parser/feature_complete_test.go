package parser

import (
	"os"
	"path/filepath"
	"testing"
)

// helper: write a file with given content into dir
func writeTestFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("writeTestFile %s: %v", name, err)
	}
}

// allCompleteFeatureContent is a feature file where all tasks are complete.
const allCompleteFeatureContent = `# Feature 001: Complete Feature

### TASK-001-001: Something done

**Acceptance Criteria:**
- [x] Done criterion
`

// hasIncompleteFeatureContent is a feature file with one incomplete task.
const hasIncompleteFeatureContent = `# Feature 002: Incomplete Feature

### TASK-002-001: Something not done

**Acceptance Criteria:**
- [x] Done criterion
- [ ] Not done criterion
`

// --- IsFeatureComplete ---

func TestIsFeatureComplete_CompletedFileExists(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "feature_001_completed.md", allCompleteFeatureContent)

	if !IsFeatureComplete(dir, "001") {
		t.Error("expected true when _completed.md exists")
	}
}

func TestIsFeatureComplete_ActiveFileAllTasksComplete(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "feature_001.md", allCompleteFeatureContent)

	if !IsFeatureComplete(dir, "001") {
		t.Error("expected true when all tasks in active file are complete")
	}
}

func TestIsFeatureComplete_ActiveFileHasIncompleteTask(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "feature_002.md", hasIncompleteFeatureContent)

	if IsFeatureComplete(dir, "002") {
		t.Error("expected false when active file has an incomplete task")
	}
}

func TestIsFeatureComplete_NeitherFileExists(t *testing.T) {
	dir := t.TempDir()

	if !IsFeatureComplete(dir, "099") {
		t.Error("expected true when neither file exists (missing = treat as complete)")
	}
}

func TestIsFeatureComplete_CompletedFilePreferredOverActiveFile(t *testing.T) {
	// _completed.md takes priority: return true even if active file has incomplete tasks.
	dir := t.TempDir()
	writeTestFile(t, dir, "feature_003_completed.md", "# some content")
	writeTestFile(t, dir, "feature_003.md", hasIncompleteFeatureContent)

	if !IsFeatureComplete(dir, "003") {
		t.Error("expected true: _completed.md exists, feature is complete regardless of active file")
	}
}

func TestIsFeatureComplete_ActiveFileEmptyTasks(t *testing.T) {
	// A feature file with no tasks is treated as complete.
	dir := t.TempDir()
	writeTestFile(t, dir, "feature_004.md", "# Feature 004: Empty\n\nNo tasks here.\n")

	if !IsFeatureComplete(dir, "004") {
		t.Error("expected true when active file has no tasks (nothing incomplete)")
	}
}

// --- CrossFeaturePredecessorsSatisfied ---

func TestCrossFeaturePredecessorsSatisfied_NilPredecessors(t *testing.T) {
	dir := t.TempDir()
	task := Task{CrossFeaturePredecessors: nil}

	if !task.CrossFeaturePredecessorsSatisfied(dir) {
		t.Error("expected true when CrossFeaturePredecessors is nil")
	}
}

func TestCrossFeaturePredecessorsSatisfied_EmptyPredecessors(t *testing.T) {
	dir := t.TempDir()
	task := Task{CrossFeaturePredecessors: []CrossFeatureRef{}}

	if !task.CrossFeaturePredecessorsSatisfied(dir) {
		t.Error("expected true when CrossFeaturePredecessors is empty slice")
	}
}

func TestCrossFeaturePredecessorsSatisfied_SingleRefComplete(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "feature_006_completed.md", "# completed")

	task := Task{
		CrossFeaturePredecessors: []CrossFeatureRef{
			{FeatureNums: []int{6}, Label: "DTOs"},
		},
	}

	if !task.CrossFeaturePredecessorsSatisfied(dir) {
		t.Error("expected true: feature 006 is complete")
	}
}

func TestCrossFeaturePredecessorsSatisfied_SingleRefIncomplete(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "feature_007.md", hasIncompleteFeatureContent)

	task := Task{
		CrossFeaturePredecessors: []CrossFeatureRef{
			{FeatureNums: []int{7}, Label: "Something"},
		},
	}

	if task.CrossFeaturePredecessorsSatisfied(dir) {
		t.Error("expected false: feature 007 is not complete")
	}
}

func TestCrossFeaturePredecessorsSatisfied_RangeRefAllComplete(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "feature_004_completed.md", "# completed")
	writeTestFile(t, dir, "feature_005_completed.md", "# completed")
	writeTestFile(t, dir, "feature_006_completed.md", "# completed")

	task := Task{
		CrossFeaturePredecessors: []CrossFeatureRef{
			{FeatureNums: []int{4, 5, 6}, Label: "all"},
		},
	}

	if !task.CrossFeaturePredecessorsSatisfied(dir) {
		t.Error("expected true: all features in range are complete")
	}
}

func TestCrossFeaturePredecessorsSatisfied_RangeRefOneIncomplete(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "feature_004_completed.md", "# completed")
	writeTestFile(t, dir, "feature_005.md", hasIncompleteFeatureContent)
	writeTestFile(t, dir, "feature_006_completed.md", "# completed")

	task := Task{
		CrossFeaturePredecessors: []CrossFeatureRef{
			{FeatureNums: []int{4, 5, 6}, Label: "all"},
		},
	}

	if task.CrossFeaturePredecessorsSatisfied(dir) {
		t.Error("expected false: feature 005 in the range is not complete")
	}
}

func TestCrossFeaturePredecessorsSatisfied_MultipleRefsOneSatisfiedOneNot(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "feature_010_completed.md", "# completed")
	writeTestFile(t, dir, "feature_011.md", hasIncompleteFeatureContent)

	task := Task{
		CrossFeaturePredecessors: []CrossFeatureRef{
			{FeatureNums: []int{10}, Label: ""},
			{FeatureNums: []int{11}, Label: ""},
		},
	}

	if task.CrossFeaturePredecessorsSatisfied(dir) {
		t.Error("expected false: feature 011 is not complete")
	}
}
