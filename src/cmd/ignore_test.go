package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// setupIgnoreDir creates a temp dir with .maggus/features/ ready for feature files.
func setupIgnoreDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".maggus", "features"), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// writeIgnoreFeature writes a feature file into .maggus/features/.
func writeIgnoreFeature(t *testing.T, dir, filename, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, ".maggus", "features", filename), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// newTestCmd creates a cobra.Command with captured stdout/stderr for testing.
func newTestCmd(t *testing.T) (*cobra.Command, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	cmd := &cobra.Command{}
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	return cmd, &stdout, &stderr
}

// --- findFeatureFile tests ---

func TestFindFeatureFile_Active(t *testing.T) {
	dir := setupIgnoreDir(t)
	writeIgnoreFeature(t, dir, "feature_003.md", "# Feature 003")

	file, state, err := findFeatureFile(dir, "003")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state != featureStateActive {
		t.Errorf("expected featureStateActive, got %d", state)
	}
	if !strings.HasSuffix(file, "feature_003.md") {
		t.Errorf("expected file ending with feature_003.md, got %s", file)
	}
}

func TestFindFeatureFile_Ignored(t *testing.T) {
	dir := setupIgnoreDir(t)
	writeIgnoreFeature(t, dir, "feature_005_ignored.md", "# Feature 005 ignored")

	_, state, err := findFeatureFile(dir, "005")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state != featureStateIgnored {
		t.Errorf("expected featureStateIgnored, got %d", state)
	}
}

func TestFindFeatureFile_Completed(t *testing.T) {
	dir := setupIgnoreDir(t)
	writeIgnoreFeature(t, dir, "feature_007_completed.md", "# Feature 007 completed")

	_, state, err := findFeatureFile(dir, "007")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state != featureStateCompleted {
		t.Errorf("expected featureStateCompleted, got %d", state)
	}
}

func TestFindFeatureFile_NotFound(t *testing.T) {
	dir := setupIgnoreDir(t)

	_, state, err := findFeatureFile(dir, "099")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state != featureStateNotFound {
		t.Errorf("expected featureStateNotFound, got %d", state)
	}
}

func TestFindFeatureFile_NoPartialMatch(t *testing.T) {
	dir := setupIgnoreDir(t)
	writeIgnoreFeature(t, dir, "feature_030.md", "# Feature 030")

	// Looking for feature 003 should NOT match feature_030
	_, state, err := findFeatureFile(dir, "003")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state != featureStateNotFound {
		t.Errorf("expected featureStateNotFound for partial ID, got %d", state)
	}
}

func TestFindFeatureFile_NoPartialMatchReverse(t *testing.T) {
	dir := setupIgnoreDir(t)
	writeIgnoreFeature(t, dir, "feature_003.md", "# Feature 003")

	// Looking for feature 030 should NOT match feature_003
	_, state, err := findFeatureFile(dir, "030")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state != featureStateNotFound {
		t.Errorf("expected featureStateNotFound for partial ID 30, got %d", state)
	}
}

// --- runIgnoreFeature tests ---

func TestRunIgnoreFeature_ActiveRenames(t *testing.T) {
	dir := setupIgnoreDir(t)
	writeIgnoreFeature(t, dir, "feature_001.md", "# Feature 001")

	cmd, stdout, _ := newTestCmd(t)
	err := runIgnoreFeature(cmd, dir, "001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Original should be gone
	if _, err := os.Stat(filepath.Join(dir, ".maggus", "features", "feature_001.md")); !os.IsNotExist(err) {
		t.Error("feature_001.md should have been renamed")
	}
	// Ignored file should exist
	if _, err := os.Stat(filepath.Join(dir, ".maggus", "features", "feature_001_ignored.md")); err != nil {
		t.Error("feature_001_ignored.md should exist")
	}
	if !strings.Contains(stdout.String(), "Ignored feature 001") {
		t.Errorf("expected success message, got: %s", stdout.String())
	}
}

func TestRunIgnoreFeature_AlreadyIgnored(t *testing.T) {
	dir := setupIgnoreDir(t)
	writeIgnoreFeature(t, dir, "feature_002_ignored.md", "# Feature 002 ignored")

	cmd, stdout, _ := newTestCmd(t)
	err := runIgnoreFeature(cmd, dir, "002")
	if err != nil {
		t.Fatalf("expected nil error for idempotent ignore, got: %v", err)
	}
	if !strings.Contains(stdout.String(), "already ignored") {
		t.Errorf("expected 'already ignored' message, got: %s", stdout.String())
	}
}

func TestRunIgnoreFeature_CompletedReturnsError(t *testing.T) {
	dir := setupIgnoreDir(t)
	writeIgnoreFeature(t, dir, "feature_004_completed.md", "# Feature 004 completed")

	cmd, _, _ := newTestCmd(t)
	err := runIgnoreFeature(cmd, dir, "004")
	if err == nil {
		t.Fatal("expected error for completed feature")
	}
	if !strings.Contains(err.Error(), "already completed") {
		t.Errorf("expected 'already completed' error, got: %v", err)
	}
}

func TestRunIgnoreFeature_MissingReturnsError(t *testing.T) {
	dir := setupIgnoreDir(t)

	cmd, _, _ := newTestCmd(t)
	err := runIgnoreFeature(cmd, dir, "099")
	if err == nil {
		t.Fatal("expected error for missing feature")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

// --- runIgnoreTask tests ---

const sampleFeatureWithTask = `# Feature

### TASK-007: Sample task
- [ ] Do something
- [ ] Do something else

### TASK-008: Another task
- [ ] More work
`

const sampleFeatureWithIgnoredTask = `# Feature

### IGNORED TASK-007: Sample task
- [ ] Do something
`

func TestRunIgnoreTask_RewritesHeading(t *testing.T) {
	dir := setupIgnoreDir(t)
	writeIgnoreFeature(t, dir, "feature_001.md", sampleFeatureWithTask)

	cmd, stdout, _ := newTestCmd(t)
	err := runIgnoreTask(cmd, dir, "TASK-007")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify file content was rewritten
	data, err := os.ReadFile(filepath.Join(dir, ".maggus", "features", "feature_001.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "### IGNORED TASK-007:") {
		t.Errorf("expected IGNORED heading, got:\n%s", string(data))
	}
	// TASK-008 should be unchanged
	if !strings.Contains(string(data), "### TASK-008:") {
		t.Errorf("TASK-008 should be unchanged, got:\n%s", string(data))
	}
	if !strings.Contains(stdout.String(), "Ignored task TASK-007") {
		t.Errorf("expected success message, got: %s", stdout.String())
	}
}

func TestRunIgnoreTask_AlreadyIgnored(t *testing.T) {
	dir := setupIgnoreDir(t)
	writeIgnoreFeature(t, dir, "feature_001.md", sampleFeatureWithIgnoredTask)

	cmd, stdout, _ := newTestCmd(t)
	err := runIgnoreTask(cmd, dir, "TASK-007")
	if err != nil {
		t.Fatalf("expected nil for idempotent ignore, got: %v", err)
	}
	if !strings.Contains(stdout.String(), "already ignored") {
		t.Errorf("expected 'already ignored' message, got: %s", stdout.String())
	}
}

func TestRunIgnoreTask_BareIDNormalizes(t *testing.T) {
	dir := setupIgnoreDir(t)
	writeIgnoreFeature(t, dir, "feature_001.md", sampleFeatureWithTask)

	cmd, stdout, _ := newTestCmd(t)
	// Pass bare "007" instead of "TASK-007"
	err := runIgnoreTask(cmd, dir, "007")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".maggus", "features", "feature_001.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "### IGNORED TASK-007:") {
		t.Errorf("expected IGNORED heading after bare ID, got:\n%s", string(data))
	}
	if !strings.Contains(stdout.String(), "Ignored task TASK-007") {
		t.Errorf("expected success message, got: %s", stdout.String())
	}
}

func TestRunIgnoreTask_MissingReturnsError(t *testing.T) {
	dir := setupIgnoreDir(t)
	writeIgnoreFeature(t, dir, "feature_001.md", sampleFeatureWithTask)

	cmd, _, _ := newTestCmd(t)
	err := runIgnoreTask(cmd, dir, "TASK-999")
	if err == nil {
		t.Fatal("expected error for missing task")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

func TestRunIgnoreTask_NoFeatureFiles(t *testing.T) {
	dir := setupIgnoreDir(t)

	cmd, _, _ := newTestCmd(t)
	err := runIgnoreTask(cmd, dir, "TASK-001")
	if err == nil {
		t.Fatal("expected error when no feature files exist")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

// --- rewriteTaskHeading tests ---

func TestRewriteTaskHeading_AtomicWrite(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "feature.md")
	content := "# Feature\n\n### TASK-010: Test task\n- [ ] criterion\n"
	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	err := rewriteTaskHeading(filePath, "TASK-010", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the temp file is cleaned up (no .tmp left behind)
	tmpFile := filePath + ".tmp"
	if _, err := os.Stat(tmpFile); !os.IsNotExist(err) {
		t.Error("temp file should not exist after successful rename")
	}

	// Verify content was rewritten
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "### IGNORED TASK-010:") {
		t.Errorf("expected IGNORED heading, got:\n%s", string(data))
	}
}

func TestRewriteTaskHeading_RemoveIgnored(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "feature.md")
	content := "# Feature\n\n### IGNORED TASK-010: Test task\n- [ ] criterion\n"
	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	err := rewriteTaskHeading(filePath, "TASK-010", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "### TASK-010: Test task") {
		t.Errorf("expected non-ignored heading, got:\n%s", string(data))
	}
	if strings.Contains(string(data), "IGNORED") {
		t.Errorf("expected IGNORED to be removed, got:\n%s", string(data))
	}
}

func TestRewriteTaskHeading_TaskNotFound(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "feature.md")
	content := "# Feature\n\n### TASK-010: Test task\n"
	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	err := rewriteTaskHeading(filePath, "TASK-999", false)
	if err == nil {
		t.Fatal("expected error for missing task heading")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

func TestRewriteTaskHeading_PreservesOtherContent(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "feature.md")
	content := "# Feature\n\nSome intro text.\n\n### TASK-001: First\n- [ ] a\n\n### TASK-002: Second\n- [ ] b\n"
	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	err := rewriteTaskHeading(filePath, "TASK-001", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !strings.Contains(s, "### IGNORED TASK-001: First") {
		t.Errorf("TASK-001 should be ignored")
	}
	if !strings.Contains(s, "### TASK-002: Second") {
		t.Errorf("TASK-002 should be unchanged")
	}
	if !strings.Contains(s, "Some intro text.") {
		t.Errorf("intro text should be preserved")
	}
}
