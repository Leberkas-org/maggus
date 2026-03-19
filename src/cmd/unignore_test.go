package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- runUnignorePlan tests ---

func TestRunUnignorePlan_IgnoredRenamesBack(t *testing.T) {
	dir := setupIgnoreDir(t)
	writeIgnorePlan(t, dir, "plan_5_ignored.md", "# Plan 5 ignored")

	cmd, stdout, _ := newTestCmd(t)
	err := runUnignorePlan(cmd, dir, "5")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Ignored file should be gone
	if _, err := os.Stat(filepath.Join(dir, ".maggus", "plan_5_ignored.md")); !os.IsNotExist(err) {
		t.Error("plan_5_ignored.md should have been renamed")
	}
	// Active file should exist
	if _, err := os.Stat(filepath.Join(dir, ".maggus", "plan_5.md")); err != nil {
		t.Error("plan_5.md should exist after unignore")
	}
	if !strings.Contains(stdout.String(), "Unignored plan 5") {
		t.Errorf("expected success message, got: %s", stdout.String())
	}
}

func TestRunUnignorePlan_ActiveReturnsError(t *testing.T) {
	dir := setupIgnoreDir(t)
	writeIgnorePlan(t, dir, "plan_2.md", "# Plan 2")

	cmd, _, _ := newTestCmd(t)
	err := runUnignorePlan(cmd, dir, "2")
	if err == nil {
		t.Fatal("expected error for active plan")
	}
	if !strings.Contains(err.Error(), "not currently ignored") {
		t.Errorf("expected 'not currently ignored' error, got: %v", err)
	}
}

func TestRunUnignorePlan_CompletedReturnsError(t *testing.T) {
	dir := setupIgnoreDir(t)
	writeIgnorePlan(t, dir, "plan_4_completed.md", "# Plan 4 completed")

	cmd, _, _ := newTestCmd(t)
	err := runUnignorePlan(cmd, dir, "4")
	if err == nil {
		t.Fatal("expected error for completed plan")
	}
	if !strings.Contains(err.Error(), "cannot unignore a completed plan") {
		t.Errorf("expected 'cannot unignore a completed plan' error, got: %v", err)
	}
}

func TestRunUnignorePlan_MissingReturnsError(t *testing.T) {
	dir := setupIgnoreDir(t)

	cmd, _, _ := newTestCmd(t)
	err := runUnignorePlan(cmd, dir, "99")
	if err == nil {
		t.Fatal("expected error for missing plan")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

// --- runUnignoreTask tests ---

func TestRunUnignoreTask_RewritesHeading(t *testing.T) {
	dir := setupIgnoreDir(t)
	writeIgnorePlan(t, dir, "plan_1.md", samplePlanWithIgnoredTask)

	cmd, stdout, _ := newTestCmd(t)
	err := runUnignoreTask(cmd, dir, "TASK-007")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify file content was rewritten
	data, err := os.ReadFile(filepath.Join(dir, ".maggus", "plan_1.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "### TASK-007:") {
		t.Errorf("expected non-ignored heading, got:\n%s", string(data))
	}
	if strings.Contains(string(data), "IGNORED") {
		t.Errorf("expected IGNORED to be removed, got:\n%s", string(data))
	}
	if !strings.Contains(stdout.String(), "Unignored task TASK-007") {
		t.Errorf("expected success message, got: %s", stdout.String())
	}
}

func TestRunUnignoreTask_NonIgnoredReturnsError(t *testing.T) {
	dir := setupIgnoreDir(t)
	writeIgnorePlan(t, dir, "plan_1.md", samplePlanWithTask)

	cmd, _, _ := newTestCmd(t)
	err := runUnignoreTask(cmd, dir, "TASK-007")
	if err == nil {
		t.Fatal("expected error for non-ignored task")
	}
	if !strings.Contains(err.Error(), "not currently ignored") {
		t.Errorf("expected 'not currently ignored' error, got: %v", err)
	}
}

func TestRunUnignoreTask_BareIDNormalizes(t *testing.T) {
	dir := setupIgnoreDir(t)
	writeIgnorePlan(t, dir, "plan_1.md", samplePlanWithIgnoredTask)

	cmd, stdout, _ := newTestCmd(t)
	// Pass bare "007" instead of "TASK-007"
	err := runUnignoreTask(cmd, dir, "007")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".maggus", "plan_1.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "### TASK-007:") {
		t.Errorf("expected non-ignored heading after bare ID, got:\n%s", string(data))
	}
	if !strings.Contains(stdout.String(), "Unignored task TASK-007") {
		t.Errorf("expected success message, got: %s", stdout.String())
	}
}

func TestRunUnignoreTask_MissingReturnsError(t *testing.T) {
	dir := setupIgnoreDir(t)
	writeIgnorePlan(t, dir, "plan_1.md", samplePlanWithIgnoredTask)

	cmd, _, _ := newTestCmd(t)
	err := runUnignoreTask(cmd, dir, "TASK-999")
	if err == nil {
		t.Fatal("expected error for missing task")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

func TestRunUnignoreTask_NoPlanFiles(t *testing.T) {
	dir := setupIgnoreDir(t)

	cmd, _, _ := newTestCmd(t)
	err := runUnignoreTask(cmd, dir, "TASK-001")
	if err == nil {
		t.Fatal("expected error when no plan files exist")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}
