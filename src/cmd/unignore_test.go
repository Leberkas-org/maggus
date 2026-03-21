package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- runUnignoreFeature tests ---

func TestRunUnignoreFeature_IgnoredRenamesBack(t *testing.T) {
	dir := setupIgnoreDir(t)
	writeIgnoreFeature(t, dir, "feature_005_ignored.md", "# Feature 005 ignored")

	cmd, stdout, _ := newTestCmd(t)
	err := runUnignoreFeature(cmd, dir, "005")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Ignored file should be gone
	if _, err := os.Stat(filepath.Join(dir, ".maggus", "features", "feature_005_ignored.md")); !os.IsNotExist(err) {
		t.Error("feature_005_ignored.md should have been renamed")
	}
	// Active file should exist
	if _, err := os.Stat(filepath.Join(dir, ".maggus", "features", "feature_005.md")); err != nil {
		t.Error("feature_005.md should exist after unignore")
	}
	if !strings.Contains(stdout.String(), "Unignored feature 005") {
		t.Errorf("expected success message, got: %s", stdout.String())
	}
}

func TestRunUnignoreFeature_ActiveReturnsError(t *testing.T) {
	dir := setupIgnoreDir(t)
	writeIgnoreFeature(t, dir, "feature_002.md", "# Feature 002")

	cmd, _, _ := newTestCmd(t)
	err := runUnignoreFeature(cmd, dir, "002")
	if err == nil {
		t.Fatal("expected error for active feature")
	}
	if !strings.Contains(err.Error(), "not currently ignored") {
		t.Errorf("expected 'not currently ignored' error, got: %v", err)
	}
}

func TestRunUnignoreFeature_CompletedReturnsError(t *testing.T) {
	dir := setupIgnoreDir(t)
	writeIgnoreFeature(t, dir, "feature_004_completed.md", "# Feature 004 completed")

	cmd, _, _ := newTestCmd(t)
	err := runUnignoreFeature(cmd, dir, "004")
	if err == nil {
		t.Fatal("expected error for completed feature")
	}
	if !strings.Contains(err.Error(), "cannot unignore a completed feature") {
		t.Errorf("expected 'cannot unignore a completed feature' error, got: %v", err)
	}
}

func TestRunUnignoreFeature_MissingReturnsError(t *testing.T) {
	dir := setupIgnoreDir(t)

	cmd, _, _ := newTestCmd(t)
	err := runUnignoreFeature(cmd, dir, "99")
	if err == nil {
		t.Fatal("expected error for missing feature")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

// --- runUnignoreTask tests ---

func TestRunUnignoreTask_RewritesHeading(t *testing.T) {
	dir := setupIgnoreDir(t)
	writeIgnoreFeature(t, dir, "feature_001.md", sampleFeatureWithIgnoredTask)

	cmd, stdout, _ := newTestCmd(t)
	err := runUnignoreTask(cmd, dir, "TASK-007")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify file content was rewritten
	data, err := os.ReadFile(filepath.Join(dir, ".maggus", "features", "feature_001.md"))
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
	writeIgnoreFeature(t, dir, "feature_001.md", sampleFeatureWithTask)

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
	writeIgnoreFeature(t, dir, "feature_001.md", sampleFeatureWithIgnoredTask)

	cmd, stdout, _ := newTestCmd(t)
	// Pass bare "007" instead of "TASK-007"
	err := runUnignoreTask(cmd, dir, "007")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".maggus", "features", "feature_001.md"))
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
	writeIgnoreFeature(t, dir, "feature_001.md", sampleFeatureWithIgnoredTask)

	cmd, _, _ := newTestCmd(t)
	err := runUnignoreTask(cmd, dir, "TASK-999")
	if err == nil {
		t.Fatal("expected error for missing task")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

func TestRunUnignoreTask_NoFeatureFiles(t *testing.T) {
	dir := setupIgnoreDir(t)

	cmd, _, _ := newTestCmd(t)
	err := runUnignoreTask(cmd, dir, "TASK-001")
	if err == nil {
		t.Fatal("expected error when no feature files exist")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}
