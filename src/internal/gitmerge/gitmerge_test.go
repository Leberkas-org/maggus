package gitmerge

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// --- test helpers ---

func initRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
	}
	for _, args := range cmds {
		run(t, repo, args...)
	}
	writeFile(t, filepath.Join(repo, "init.txt"), "initial")
	run(t, repo, "git", "add", ".")
	run(t, repo, "git", "commit", "-m", "initial commit")
	return repo
}

func run(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v failed: %v\n%s", args, err, out)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func branchExists(t *testing.T, repo, branch string) bool {
	t.Helper()
	cmd := exec.Command("git", "rev-parse", "--verify", "--quiet", branch)
	cmd.Dir = repo
	return cmd.Run() == nil
}

// --- tests ---

func TestMergeTaskBranch_FastForward(t *testing.T) {
	repo := initRepo(t)

	// Create and switch to feature branch.
	// Note: we use "feature/plan-038" (not "feature/maggus-038") because git cannot
	// have refs/heads/feature/maggus-038 and refs/heads/feature/maggus-038/task-001
	// as refs simultaneously (file vs directory conflict).
	run(t, repo, "git", "checkout", "-b", "feature/plan-038")

	// Create task branch off feature/plan-038.
	taskBranch := "feature/maggus-038/task-001"
	run(t, repo, "git", "checkout", "-b", taskBranch)
	writeFile(t, filepath.Join(repo, "task.txt"), "task content")
	run(t, repo, "git", "add", ".")
	run(t, repo, "git", "commit", "-m", "task commit")
	run(t, repo, "git", "checkout", "feature/plan-038")

	// Merge (feature branch has no new commits — fast-forward scenario).
	if err := MergeTaskBranch(repo, "feature/plan-038", taskBranch); err != nil {
		t.Fatalf("MergeTaskBranch: %v", err)
	}

	// Task branch should NOT be deleted — cleanup is now the caller's responsibility.
	if !branchExists(t, repo, taskBranch) {
		t.Error("task branch should still exist after MergeTaskBranch (cleanup is caller's responsibility)")
	}

	// The merged file should exist on the feature branch.
	if _, err := os.Stat(filepath.Join(repo, "task.txt")); err != nil {
		t.Errorf("task.txt should exist after merge: %v", err)
	}
}

func TestMergeTaskBranch_ThreeWayMerge(t *testing.T) {
	repo := initRepo(t)

	// Create and switch to feature branch.
	// Note: we use "feature/plan-038" (not "feature/maggus-038") because git cannot
	// have refs/heads/feature/maggus-038 and refs/heads/feature/maggus-038/task-002
	// as refs simultaneously (file vs directory conflict).
	run(t, repo, "git", "checkout", "-b", "feature/plan-038")

	// Create task branch off feature/plan-038.
	taskBranch := "feature/maggus-038/task-002"
	run(t, repo, "git", "checkout", "-b", taskBranch)

	// Add commit on feature branch (diverge from task branch).
	run(t, repo, "git", "checkout", "feature/plan-038")
	writeFile(t, filepath.Join(repo, "feature.txt"), "feature content")
	run(t, repo, "git", "add", ".")
	run(t, repo, "git", "commit", "-m", "feature commit")

	// Add commit on task branch (different file — no conflict).
	run(t, repo, "git", "checkout", taskBranch)
	writeFile(t, filepath.Join(repo, "task.txt"), "task content")
	run(t, repo, "git", "add", ".")
	run(t, repo, "git", "commit", "-m", "task commit")
	run(t, repo, "git", "checkout", "feature/plan-038")

	// Merge.
	if err := MergeTaskBranch(repo, "feature/plan-038", taskBranch); err != nil {
		t.Fatalf("MergeTaskBranch: %v", err)
	}

	// Task branch should NOT be deleted — cleanup is now the caller's responsibility.
	if !branchExists(t, repo, taskBranch) {
		t.Error("task branch should still exist after MergeTaskBranch (cleanup is caller's responsibility)")
	}

	// Both files should exist on the feature branch.
	if _, err := os.Stat(filepath.Join(repo, "feature.txt")); err != nil {
		t.Errorf("feature.txt should exist after merge: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, "task.txt")); err != nil {
		t.Errorf("task.txt should exist after merge: %v", err)
	}
}

func TestMergeTaskBranch_Conflict(t *testing.T) {
	repo := initRepo(t)

	// Create and switch to feature branch.
	// Note: we use "feature/plan-038" (not "feature/maggus-038") because git cannot
	// have refs/heads/feature/maggus-038 and refs/heads/feature/maggus-038/task-003
	// as refs simultaneously (file vs directory conflict).
	run(t, repo, "git", "checkout", "-b", "feature/plan-038")

	// Set up plan file (commit it so it exists on both branches).
	planContent := "### TASK-038-003: Test task\n\n" +
		"**Acceptance Criteria:**\n" +
		"- [ ] First criterion\n" +
		"- [ ] Second criterion\n"
	planFile := filepath.Join(repo, ".maggus", "features", "feature_038.md")
	writeFile(t, planFile, planContent)
	run(t, repo, "git", "add", ".")
	run(t, repo, "git", "commit", "-m", "add plan file")

	// Create task branch off feature/plan-038.
	taskBranch := "feature/maggus-038/task-003"
	run(t, repo, "git", "checkout", "-b", taskBranch)

	// Add conflicting commit on task branch.
	writeFile(t, filepath.Join(repo, "conflict.txt"), "task version")
	run(t, repo, "git", "add", ".")
	run(t, repo, "git", "commit", "-m", "task change")

	// Switch back to feature branch and add a conflicting commit.
	run(t, repo, "git", "checkout", "feature/plan-038")
	writeFile(t, filepath.Join(repo, "conflict.txt"), "feature version")
	run(t, repo, "git", "add", ".")
	run(t, repo, "git", "commit", "-m", "feature change")

	// Merge — should fail with conflict.
	err := MergeTaskBranch(repo, "feature/plan-038", taskBranch)

	// Verify MergeConflictError.
	var conflictErr *MergeConflictError
	if !errors.As(err, &conflictErr) {
		t.Fatalf("expected *MergeConflictError, got: %v", err)
	}
	if conflictErr.FeatureBranch != "feature/plan-038" {
		t.Errorf("FeatureBranch = %q, want %q", conflictErr.FeatureBranch, "feature/plan-038")
	}
	if conflictErr.TaskBranch != taskBranch {
		t.Errorf("TaskBranch = %q, want %q", conflictErr.TaskBranch, taskBranch)
	}

	// Verify BLOCKED criterion was injected into the plan file.
	data, readErr := os.ReadFile(planFile)
	if readErr != nil {
		t.Fatal(readErr)
	}
	expectedBlocked := "BLOCKED: Merge conflict merging feature/maggus-038/task-003 into feature/plan-038"
	if !strings.Contains(string(data), expectedBlocked) {
		t.Errorf("expected BLOCKED criterion in plan file.\ngot:\n%s", data)
	}
	// The injected line should be an unchecked criterion.
	if !strings.Contains(string(data), "- [ ] "+expectedBlocked) {
		t.Errorf("BLOCKED line should be an unchecked criterion")
	}

	// Task branch should still exist (not deleted on conflict).
	if !branchExists(t, repo, taskBranch) {
		t.Error("task branch should NOT be deleted on conflict")
	}

	// Merge should be cleanly aborted — conflict.txt at feature version.
	content, _ := os.ReadFile(filepath.Join(repo, "conflict.txt"))
	if string(content) != "feature version" {
		t.Errorf("conflict.txt should be feature version after abort, got: %q", content)
	}
}

func TestMergeTaskBranch_WorktreeAlreadyCheckedOut(t *testing.T) {
	repo := initRepo(t)

	// Create the plan (feature) branch.
	// We use "feature/plan-055" (not "feature/maggus-055") to avoid the git ref
	// hierarchy conflict: refs/heads/feature/maggus-055 (file) cannot coexist with
	// refs/heads/feature/maggus-055/task-001 (inside a directory).
	run(t, repo, "git", "checkout", "-b", "feature/plan-055")

	// Create task branch off the plan branch with a commit.
	taskBranch := "feature/maggus-055/task-001"
	run(t, repo, "git", "checkout", "-b", taskBranch)
	writeFile(t, filepath.Join(repo, "task.txt"), "task content")
	run(t, repo, "git", "add", ".")
	run(t, repo, "git", "commit", "-m", "task commit")

	// Switch back to the plan branch in the main repo.
	run(t, repo, "git", "checkout", "feature/plan-055")

	// Simulate parallel execution: check out the task branch in a worktree.
	// This is what the orchestrator does before launching the agent.
	wtPath := filepath.Join(t.TempDir(), "worktree")
	run(t, repo, "git", "worktree", "add", wtPath, taskBranch)

	// MergeTaskBranch must succeed even though taskBranch is already checked out
	// in the worktree. Without the fix this returns "already checked out" exit 128.
	if err := MergeTaskBranch(repo, "feature/plan-055", taskBranch); err != nil {
		t.Fatalf("MergeTaskBranch: %v", err)
	}

	// The task file should now exist on the plan branch in the main repo.
	if _, err := os.Stat(filepath.Join(repo, "task.txt")); err != nil {
		t.Errorf("task.txt should exist on plan branch after merge: %v", err)
	}
}

// --- unit tests for helper functions ---

func TestTaskIDFromBranch(t *testing.T) {
	tests := []struct {
		branch string
		want   string
	}{
		{"feature/maggus-038/task-003", "TASK-038-003"},
		{"feature/maggus-001/task-001", "TASK-001-001"},
		{"bugfix/maggus-bug-001/task-003", "BUG-001-003"},
		{"bugfix/maggus-bug-001/task-001", "BUG-001-001"},
		{"main", ""},
		{"feature/other", ""},
	}
	for _, tt := range tests {
		if got := TaskIDFromBranch(tt.branch); got != tt.want {
			t.Errorf("TaskIDFromBranch(%q) = %q, want %q", tt.branch, got, tt.want)
		}
	}
}

func TestAddCriterionToTask(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "plan.md")

	original := "### TASK-001-001: First task\n\n" +
		"**Acceptance Criteria:**\n" +
		"- [ ] criterion A\n" +
		"- [x] criterion B\n" +
		"\n" +
		"### TASK-001-002: Second task\n\n" +
		"**Acceptance Criteria:**\n" +
		"- [ ] criterion C\n"

	writeFile(t, path, original)

	if err := addCriterionToTask(path, "TASK-001-001", "BLOCKED: something went wrong"); err != nil {
		t.Fatalf("addCriterionToTask: %v", err)
	}

	data, _ := os.ReadFile(path)
	content := string(data)

	// The new criterion should appear after criterion B (the last criterion in TASK-001-001).
	if !strings.Contains(content, "- [x] criterion B\n- [ ] BLOCKED: something went wrong\n") {
		t.Errorf("criterion not inserted at expected position.\ngot:\n%s", content)
	}

	// TASK-001-002's criteria should be unchanged.
	if !strings.Contains(content, "- [ ] criterion C") {
		t.Error("second task's criteria should be unchanged")
	}
}
