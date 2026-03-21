package runner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeFeatureFile creates a minimal feature file with the given tasks.
// Each task is a pair of (id, status) where status is "workable", "done", or "blocked".
func writeFeatureFile(t *testing.T, dir string, filename string, tasks []struct{ id, status string }) {
	t.Helper()
	featDir := filepath.Join(dir, ".maggus", "features")
	if err := os.MkdirAll(featDir, 0o755); err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	b.WriteString("# Feature\n\n")
	for _, task := range tasks {
		b.WriteString("### " + task.id + ": Test task\n")
		switch task.status {
		case "done":
			b.WriteString("- [x] criterion\n")
		case "blocked":
			b.WriteString("- [ ] BLOCKED: waiting on something\n")
		default: // workable
			b.WriteString("- [ ] criterion\n")
		}
	}
	if err := os.WriteFile(filepath.Join(featDir, filename), []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}

// writeBugFile creates a minimal bug file with the given tasks.
func writeBugFile(t *testing.T, dir string, filename string, tasks []struct{ id, status string }) {
	t.Helper()
	bugDir := filepath.Join(dir, ".maggus", "bugs")
	if err := os.MkdirAll(bugDir, 0o755); err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	b.WriteString("# Bug\n\n")
	for _, task := range tasks {
		b.WriteString("### " + task.id + ": Test bug\n")
		switch task.status {
		case "done":
			b.WriteString("- [x] criterion\n")
		case "blocked":
			b.WriteString("- [ ] BLOCKED: waiting on something\n")
		default: // workable
			b.WriteString("- [ ] criterion\n")
		}
	}
	if err := os.WriteFile(filepath.Join(bugDir, filename), []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestHandleFileChange_UpdatesTotalIters(t *testing.T) {
	dir := t.TempDir()

	writeFeatureFile(t, dir, "feature_001.md", []struct{ id, status string }{
		{"TASK-001-001", "workable"},
		{"TASK-001-002", "workable"},
		{"TASK-001-003", "done"},
	})

	m := &TUIModel{
		workDir:     dir,
		currentIter: 1,
		totalIters:  2,
	}

	m.handleFileChange()

	// 2 workable tasks + currentIter(1) = 3
	if m.totalIters != 3 {
		t.Errorf("totalIters = %d, want 3", m.totalIters)
	}
	if m.currentIter != 1 {
		t.Errorf("currentIter changed to %d, want 1 (should not change)", m.currentIter)
	}
	if m.activeBugs != 0 {
		t.Errorf("activeBugs = %d, want 0", m.activeBugs)
	}
}

func TestHandleFileChange_CountsBugsAsTasks(t *testing.T) {
	dir := t.TempDir()

	writeFeatureFile(t, dir, "feature_001.md", []struct{ id, status string }{
		{"TASK-001-001", "workable"},
	})
	writeBugFile(t, dir, "bug_001.md", []struct{ id, status string }{
		{"BUG-001-001", "workable"},
		{"BUG-001-002", "workable"},
	})

	m := &TUIModel{
		workDir:     dir,
		currentIter: 2,
		totalIters:  5,
	}

	m.handleFileChange()

	// 3 workable (1 feature + 2 bugs) + currentIter(2) = 5
	if m.totalIters != 5 {
		t.Errorf("totalIters = %d, want 5", m.totalIters)
	}
	if m.activeBugs != 2 {
		t.Errorf("activeBugs = %d, want 2", m.activeBugs)
	}
}

func TestHandleFileChange_CurrentIterNotChanged(t *testing.T) {
	dir := t.TempDir()

	writeFeatureFile(t, dir, "feature_001.md", []struct{ id, status string }{
		{"TASK-001-001", "done"},
	})

	m := &TUIModel{
		workDir:     dir,
		currentIter: 5,
		totalIters:  10,
	}

	m.handleFileChange()

	// 0 workable + currentIter(5) = 5
	if m.totalIters != 5 {
		t.Errorf("totalIters = %d, want 5", m.totalIters)
	}
	if m.currentIter != 5 {
		t.Errorf("currentIter = %d, want 5", m.currentIter)
	}
}

func TestHandleFileChange_BlockedTasksNotCounted(t *testing.T) {
	dir := t.TempDir()

	writeFeatureFile(t, dir, "feature_001.md", []struct{ id, status string }{
		{"TASK-001-001", "workable"},
		{"TASK-001-002", "blocked"},
	})
	writeBugFile(t, dir, "bug_001.md", []struct{ id, status string }{
		{"BUG-001-001", "blocked"},
	})

	m := &TUIModel{
		workDir:     dir,
		currentIter: 0,
		totalIters:  3,
	}

	m.handleFileChange()

	// 1 workable feature + 0 workable bugs + currentIter(0) = 1
	if m.totalIters != 1 {
		t.Errorf("totalIters = %d, want 1", m.totalIters)
	}
	if m.activeBugs != 0 {
		t.Errorf("activeBugs = %d, want 0", m.activeBugs)
	}
}

func TestHandleFileChange_EmptyWorkDir(t *testing.T) {
	m := &TUIModel{
		workDir:     "",
		currentIter: 1,
		totalIters:  5,
	}

	m.handleFileChange()

	// No change when workDir is empty
	if m.totalIters != 5 {
		t.Errorf("totalIters = %d, want 5 (should not change)", m.totalIters)
	}
}

func TestHandleFileChange_NewFileAdded(t *testing.T) {
	dir := t.TempDir()

	writeFeatureFile(t, dir, "feature_001.md", []struct{ id, status string }{
		{"TASK-001-001", "workable"},
	})

	m := &TUIModel{
		workDir:     dir,
		currentIter: 1,
		totalIters:  2,
	}

	m.handleFileChange()
	if m.totalIters != 2 {
		t.Errorf("before adding file: totalIters = %d, want 2", m.totalIters)
	}

	// Simulate adding a new feature file
	writeFeatureFile(t, dir, "feature_002.md", []struct{ id, status string }{
		{"TASK-002-001", "workable"},
		{"TASK-002-002", "workable"},
	})

	m.handleFileChange()

	// 3 workable + currentIter(1) = 4
	if m.totalIters != 4 {
		t.Errorf("after adding file: totalIters = %d, want 4", m.totalIters)
	}
}

func TestBugHintLine_Rendered(t *testing.T) {
	m := TUIModel{
		version:    "1.0.0",
		totalIters: 5,
		currentIter: 2,
		activeBugs: 3,
		width:      80,
		height:     40,
	}

	header := m.renderHeaderInner(80)

	if !strings.Contains(header, "3 bugs active") {
		t.Errorf("expected '3 bugs active' in header, got:\n%s", header)
	}
}

func TestBugHintLine_SingleBug(t *testing.T) {
	m := TUIModel{
		version:    "1.0.0",
		totalIters: 5,
		currentIter: 2,
		activeBugs: 1,
		width:      80,
		height:     40,
	}

	header := m.renderHeaderInner(80)

	if !strings.Contains(header, "1 bug active") {
		t.Errorf("expected '1 bug active' in header, got:\n%s", header)
	}
	// Should not contain plural form
	if strings.Contains(header, "bugs active") {
		t.Errorf("expected singular '1 bug active', not plural, got:\n%s", header)
	}
}

func TestBugHintLine_Hidden_WhenNoBugs(t *testing.T) {
	m := TUIModel{
		version:    "1.0.0",
		totalIters: 5,
		currentIter: 2,
		activeBugs: 0,
		width:      80,
		height:     40,
	}

	header := m.renderHeaderInner(80)

	if strings.Contains(header, "bug") {
		t.Errorf("expected no bug hint when activeBugs=0, got:\n%s", header)
	}
}
