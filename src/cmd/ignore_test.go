package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// setupIgnoreDir creates a temp dir with .maggus/ ready for plan files.
func setupIgnoreDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".maggus"), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// writeIgnorePlan writes a plan file into .maggus/.
func writeIgnorePlan(t *testing.T, dir, filename, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, ".maggus", filename), []byte(content), 0o644); err != nil {
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

// --- findPlanFile tests ---

func TestFindPlanFile_Active(t *testing.T) {
	dir := setupIgnoreDir(t)
	writeIgnorePlan(t, dir, "plan_3.md", "# Plan 3")

	file, state, err := findPlanFile(dir, "3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state != planStateActive {
		t.Errorf("expected planStateActive, got %d", state)
	}
	if !strings.HasSuffix(file, "plan_3.md") {
		t.Errorf("expected file ending with plan_3.md, got %s", file)
	}
}

func TestFindPlanFile_Ignored(t *testing.T) {
	dir := setupIgnoreDir(t)
	writeIgnorePlan(t, dir, "plan_5_ignored.md", "# Plan 5 ignored")

	_, state, err := findPlanFile(dir, "5")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state != planStateIgnored {
		t.Errorf("expected planStateIgnored, got %d", state)
	}
}

func TestFindPlanFile_Completed(t *testing.T) {
	dir := setupIgnoreDir(t)
	writeIgnorePlan(t, dir, "plan_7_completed.md", "# Plan 7 completed")

	_, state, err := findPlanFile(dir, "7")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state != planStateCompleted {
		t.Errorf("expected planStateCompleted, got %d", state)
	}
}

func TestFindPlanFile_NotFound(t *testing.T) {
	dir := setupIgnoreDir(t)

	_, state, err := findPlanFile(dir, "99")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state != planStateNotFound {
		t.Errorf("expected planStateNotFound, got %d", state)
	}
}

func TestFindPlanFile_NoPartialMatch(t *testing.T) {
	dir := setupIgnoreDir(t)
	writeIgnorePlan(t, dir, "plan_30.md", "# Plan 30")

	// Looking for plan 3 should NOT match plan_30
	_, state, err := findPlanFile(dir, "3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state != planStateNotFound {
		t.Errorf("expected planStateNotFound for partial ID, got %d", state)
	}
}

func TestFindPlanFile_NoPartialMatchReverse(t *testing.T) {
	dir := setupIgnoreDir(t)
	writeIgnorePlan(t, dir, "plan_3.md", "# Plan 3")

	// Looking for plan 30 should NOT match plan_3
	_, state, err := findPlanFile(dir, "30")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state != planStateNotFound {
		t.Errorf("expected planStateNotFound for partial ID 30, got %d", state)
	}
}

// --- runIgnorePlan tests ---

func TestRunIgnorePlan_ActiveRenames(t *testing.T) {
	dir := setupIgnoreDir(t)
	writeIgnorePlan(t, dir, "plan_1.md", "# Plan 1")

	cmd, stdout, _ := newTestCmd(t)
	err := runIgnorePlan(cmd, dir, "1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Original should be gone
	if _, err := os.Stat(filepath.Join(dir, ".maggus", "plan_1.md")); !os.IsNotExist(err) {
		t.Error("plan_1.md should have been renamed")
	}
	// Ignored file should exist
	if _, err := os.Stat(filepath.Join(dir, ".maggus", "plan_1_ignored.md")); err != nil {
		t.Error("plan_1_ignored.md should exist")
	}
	if !strings.Contains(stdout.String(), "Ignored plan 1") {
		t.Errorf("expected success message, got: %s", stdout.String())
	}
}

func TestRunIgnorePlan_AlreadyIgnored(t *testing.T) {
	dir := setupIgnoreDir(t)
	writeIgnorePlan(t, dir, "plan_2_ignored.md", "# Plan 2 ignored")

	cmd, stdout, _ := newTestCmd(t)
	err := runIgnorePlan(cmd, dir, "2")
	if err != nil {
		t.Fatalf("expected nil error for idempotent ignore, got: %v", err)
	}
	if !strings.Contains(stdout.String(), "already ignored") {
		t.Errorf("expected 'already ignored' message, got: %s", stdout.String())
	}
}

func TestRunIgnorePlan_CompletedReturnsError(t *testing.T) {
	dir := setupIgnoreDir(t)
	writeIgnorePlan(t, dir, "plan_4_completed.md", "# Plan 4 completed")

	cmd, _, _ := newTestCmd(t)
	err := runIgnorePlan(cmd, dir, "4")
	if err == nil {
		t.Fatal("expected error for completed plan")
	}
	if !strings.Contains(err.Error(), "already completed") {
		t.Errorf("expected 'already completed' error, got: %v", err)
	}
}

func TestRunIgnorePlan_MissingReturnsError(t *testing.T) {
	dir := setupIgnoreDir(t)

	cmd, _, _ := newTestCmd(t)
	err := runIgnorePlan(cmd, dir, "99")
	if err == nil {
		t.Fatal("expected error for missing plan")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

// --- runIgnoreTask tests ---

const samplePlanWithTask = `# Plan

### TASK-007: Sample task
- [ ] Do something
- [ ] Do something else

### TASK-008: Another task
- [ ] More work
`

const samplePlanWithIgnoredTask = `# Plan

### IGNORED TASK-007: Sample task
- [ ] Do something
`

func TestRunIgnoreTask_RewritesHeading(t *testing.T) {
	dir := setupIgnoreDir(t)
	writeIgnorePlan(t, dir, "plan_1.md", samplePlanWithTask)

	cmd, stdout, _ := newTestCmd(t)
	err := runIgnoreTask(cmd, dir, "TASK-007")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify file content was rewritten
	data, err := os.ReadFile(filepath.Join(dir, ".maggus", "plan_1.md"))
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
	writeIgnorePlan(t, dir, "plan_1.md", samplePlanWithIgnoredTask)

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
	writeIgnorePlan(t, dir, "plan_1.md", samplePlanWithTask)

	cmd, stdout, _ := newTestCmd(t)
	// Pass bare "007" instead of "TASK-007"
	err := runIgnoreTask(cmd, dir, "007")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".maggus", "plan_1.md"))
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
	writeIgnorePlan(t, dir, "plan_1.md", samplePlanWithTask)

	cmd, _, _ := newTestCmd(t)
	err := runIgnoreTask(cmd, dir, "TASK-999")
	if err == nil {
		t.Fatal("expected error for missing task")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

func TestRunIgnoreTask_NoPlanFiles(t *testing.T) {
	dir := setupIgnoreDir(t)

	cmd, _, _ := newTestCmd(t)
	err := runIgnoreTask(cmd, dir, "TASK-001")
	if err == nil {
		t.Fatal("expected error when no plan files exist")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

// --- rewriteTaskHeading tests ---

func TestRewriteTaskHeading_AtomicWrite(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "plan.md")
	content := "# Plan\n\n### TASK-010: Test task\n- [ ] criterion\n"
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
	filePath := filepath.Join(dir, "plan.md")
	content := "# Plan\n\n### IGNORED TASK-010: Test task\n- [ ] criterion\n"
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
	filePath := filepath.Join(dir, "plan.md")
	content := "# Plan\n\n### TASK-010: Test task\n"
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
	filePath := filepath.Join(dir, "plan.md")
	content := "# Plan\n\nSome intro text.\n\n### TASK-001: First\n- [ ] a\n\n### TASK-002: Second\n- [ ] b\n"
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
