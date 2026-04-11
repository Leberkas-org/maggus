package gitbranch

import (
	"os/exec"
	"strings"
	"testing"
)

func TestIsProtected(t *testing.T) {
	defaultList := []string{"main", "master", "dev"}
	tests := []struct {
		branch string
		list   []string
		want   bool
	}{
		{"main", defaultList, true},
		{"master", defaultList, true},
		{"dev", defaultList, true},
		{"feature/foo", defaultList, false},
		{"fix/bar", defaultList, false},
		{"develop", defaultList, false},
		{"my-branch", defaultList, false},
		{"", defaultList, false},
		{"release", []string{"main", "release"}, true},
		{"main", []string{"main", "release"}, true},
		{"dev", []string{"main", "release"}, false},
	}

	for _, tt := range tests {
		got := IsProtected(tt.branch, tt.list)
		if got != tt.want {
			t.Errorf("IsProtected(%q, %v) = %v, want %v", tt.branch, tt.list, got, tt.want)
		}
	}
}

func TestFeatureBranchName(t *testing.T) {
	tests := []struct {
		taskID string
		want   string
	}{
		{"TASK-001", "feature/maggus-001/task-001"},
		{"TASK-003", "feature/maggus-003/task-003"},
		{"TASK-042", "feature/maggus-042/task-042"},
		{"TASK-100", "feature/maggus-100/task-100"},
		{"TASK-008", "feature/maggus-008/task-008"},
		{"TASK-038-003", "feature/maggus-038/task-003"},
		{"TASK-1-E05", "feature/maggus-1/task-e05"},
		{"TASK-2-A01", "feature/maggus-2/task-a01"},
		{"INVALID", "feature/maggus-000/task-000"},
	}

	for _, tt := range tests {
		got := FeatureBranchName(tt.taskID)
		if got != tt.want {
			t.Errorf("FeatureBranchName(%q) = %q, want %q", tt.taskID, got, tt.want)
		}
	}
}

func TestBranchName(t *testing.T) {
	tests := []struct {
		taskID string
		want   string
	}{
		// Bug task IDs
		{"BUG-001-001", "bugfix/maggus-bug-001/task-001"},
		{"BUG-002-003", "bugfix/maggus-bug-002/task-003"},
		{"BUG-123-456", "bugfix/maggus-bug-123/task-456"},
		{"BUG-001", "bugfix/maggus-bug-001/task-001"},
		// Feature task IDs (delegated to FeatureBranchName)
		{"TASK-001", "feature/maggus-001/task-001"},
		{"TASK-003", "feature/maggus-003/task-003"},
		{"TASK-1-E05", "feature/maggus-1/task-e05"},
		// Invalid
		{"INVALID", "feature/maggus-000/task-000"},
	}

	for _, tt := range tests {
		got := BranchName(tt.taskID)
		if got != tt.want {
			t.Errorf("BranchName(%q) = %q, want %q", tt.taskID, got, tt.want)
		}
	}
}

func TestEnsureFeatureBranch_BugTask(t *testing.T) {
	tmp := t.TempDir()
	initGitRepoWithBranch(t, tmp, "main")

	branch, msg, err := EnsureFeatureBranch(tmp, "BUG-001-001", []string{"main", "master", "dev"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if branch != "bugfix/maggus-bug-001/task-001" {
		t.Errorf("branch = %q, want %q", branch, "bugfix/maggus-bug-001/task-001")
	}
	if msg == "" {
		t.Error("expected a message about switching branches")
	}

	got := getCurrentBranch(t, tmp)
	if got != "bugfix/maggus-bug-001/task-001" {
		t.Errorf("actual git branch = %q, want %q", got, "bugfix/maggus-bug-001/task-001")
	}
}

func TestEnsureFeatureBranch_NonProtected(t *testing.T) {
	tmp := t.TempDir()
	initGitRepo(t, tmp)
	checkoutBranch(t, tmp, "feature/existing")

	branch, msg, err := EnsureFeatureBranch(tmp, "TASK-003", []string{"main", "master", "dev"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if branch != "feature/existing" {
		t.Errorf("branch = %q, want %q", branch, "feature/existing")
	}
	if msg == "" {
		t.Error("expected a message about staying on current branch")
	}
}

func TestEnsureFeatureBranch_Protected(t *testing.T) {
	for _, protected := range []string{"main", "master", "dev"} {
		t.Run(protected, func(t *testing.T) {
			tmp := t.TempDir()
			initGitRepoWithBranch(t, tmp, protected)

			branch, msg, err := EnsureFeatureBranch(tmp, "TASK-003", []string{"main", "master", "dev"})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if branch != "feature/maggus-003/task-003" {
				t.Errorf("branch = %q, want %q", branch, "feature/maggus-003/task-003")
			}
			if msg == "" {
				t.Error("expected a message about switching branches")
			}

			// Verify we're actually on the new branch
			got := getCurrentBranch(t, tmp)
			if got != "feature/maggus-003/task-003" {
				t.Errorf("actual git branch = %q, want %q", got, "feature/maggus-003/task-003")
			}
		})
	}
}

func TestEnsureFeatureBranch_NotAGitRepo(t *testing.T) {
	tmp := t.TempDir() // no git init

	branch, msg, err := EnsureFeatureBranch(tmp, "TASK-001", []string{"main", "master", "dev"})
	if err != nil {
		t.Fatalf("should not return error for non-git dir, got: %v", err)
	}
	if branch != "" {
		t.Errorf("branch should be empty for non-git dir, got %q", branch)
	}
	if msg == "" {
		t.Error("expected a warning message for non-git dir")
	}
}

func TestEnsureFeatureBranch_CustomProtectedList(t *testing.T) {
	tmp := t.TempDir()
	initGitRepoWithBranch(t, tmp, "release")

	branch, _, err := EnsureFeatureBranch(tmp, "TASK-005", []string{"release", "staging"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if branch != "feature/maggus-005/task-005" {
		t.Errorf("branch = %q, want %q", branch, "feature/maggus-005/task-005")
	}
}

func TestEnsureFeatureBranch_ExistingBranch_CalledTwice(t *testing.T) {
	tmp := t.TempDir()
	initGitRepoWithBranch(t, tmp, "master")
	protected := []string{"main", "master", "dev"}

	// First call: creates the branch
	branch1, msg1, err := EnsureFeatureBranch(tmp, "TASK-007", protected)
	if err != nil {
		t.Fatalf("first call: unexpected error: %v", err)
	}
	if branch1 != "feature/maggus-007/task-007" {
		t.Errorf("first call: branch = %q, want %q", branch1, "feature/maggus-007/task-007")
	}
	if msg1 == "" {
		t.Error("first call: expected a message")
	}

	// Go back to protected branch
	cmd := exec.Command("git", "checkout", "master")
	cmd.Dir = tmp
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("checkout master: %v\n%s", err, out)
	}

	// Second call: should switch to existing branch, not fail
	branch2, msg2, err := EnsureFeatureBranch(tmp, "TASK-007", protected)
	if err != nil {
		t.Fatalf("second call: unexpected error: %v", err)
	}
	if branch2 != "feature/maggus-007/task-007" {
		t.Errorf("second call: branch = %q, want %q", branch2, "feature/maggus-007/task-007")
	}
	if msg2 == "" {
		t.Error("second call: expected a message")
	}

	got := getCurrentBranch(t, tmp)
	if got != "feature/maggus-007/task-007" {
		t.Errorf("actual git branch = %q, want %q", got, "feature/maggus-007/task-007")
	}
}

func TestEnsureFeatureBranch_ExistingBranch_ReturnsCorrectMessage(t *testing.T) {
	tmp := t.TempDir()
	initGitRepoWithBranch(t, tmp, "master")
	protected := []string{"main", "master", "dev"}

	// Create the branch first
	_, _, err := EnsureFeatureBranch(tmp, "TASK-008", protected)
	if err != nil {
		t.Fatalf("setup: unexpected error: %v", err)
	}

	// Go back to protected branch
	cmd := exec.Command("git", "checkout", "master")
	cmd.Dir = tmp
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("checkout master: %v\n%s", err, out)
	}

	// Switch to existing branch
	branch, msg, err := EnsureFeatureBranch(tmp, "TASK-008", protected)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if branch != "feature/maggus-008/task-008" {
		t.Errorf("branch = %q, want %q", branch, "feature/maggus-008/task-008")
	}
	if msg == "" {
		t.Error("expected a message about switching branches")
	}
	// Message should mention switching from protected branch
	if !strings.Contains(msg, "master") || !strings.Contains(msg, "feature/maggus-008/task-008") {
		t.Errorf("message should mention both branches, got: %q", msg)
	}
}

func TestDeleteBranch_Normal(t *testing.T) {
	tmp := t.TempDir()
	initGitRepoWithBranch(t, tmp, "master")

	// Create a feature branch and add a commit.
	checkoutBranch(t, tmp, "feature/cleanup-test")
	cmd := exec.Command("git", "commit", "--allow-empty", "-m", "feature commit")
	cmd.Dir = tmp
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("commit on feature branch: %v\n%s", err, out)
	}

	// Merge into master, then switch back.
	cmd = exec.Command("git", "checkout", "master")
	cmd.Dir = tmp
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("checkout master: %v\n%s", err, out)
	}
	cmd = exec.Command("git", "merge", "--no-ff", "feature/cleanup-test")
	cmd.Dir = tmp
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("merge: %v\n%s", err, out)
	}

	// Branch is fully merged — DeleteBranch should succeed.
	if err := DeleteBranch(tmp, "feature/cleanup-test"); err != nil {
		t.Fatalf("DeleteBranch: %v", err)
	}

	// Verify the branch is gone.
	if branchExists(tmp, "feature/cleanup-test") {
		t.Error("branch should not exist after DeleteBranch")
	}
}

func TestDeleteBranch_AlreadyDeleted(t *testing.T) {
	tmp := t.TempDir()
	initGitRepoWithBranch(t, tmp, "master")

	// Trying to delete a non-existent branch must return an error.
	if err := DeleteBranch(tmp, "feature/does-not-exist"); err == nil {
		t.Error("expected error when deleting non-existent branch, got nil")
	}
}

func TestDeleteBranch_Unmerged(t *testing.T) {
	tmp := t.TempDir()
	initGitRepoWithBranch(t, tmp, "master")

	// Create a branch with an unmerged commit.
	checkoutBranch(t, tmp, "feature/unmerged")
	cmd := exec.Command("git", "commit", "--allow-empty", "-m", "unmerged commit")
	cmd.Dir = tmp
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("commit on feature branch: %v\n%s", err, out)
	}

	// Switch back to master without merging.
	cmd = exec.Command("git", "checkout", "master")
	cmd.Dir = tmp
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("checkout master: %v\n%s", err, out)
	}

	// DeleteBranch uses -d (not -D): must refuse to delete an unmerged branch.
	if err := DeleteBranch(tmp, "feature/unmerged"); err == nil {
		t.Error("expected error when deleting unmerged branch with -d, got nil")
	}
}

func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
		{"git", "commit", "--allow-empty", "-m", "init"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git setup %v failed: %v\n%s", args, err, out)
		}
	}
}

func initGitRepoWithBranch(t *testing.T, dir string, branch string) {
	t.Helper()
	cmds := [][]string{
		{"git", "init", "-b", branch},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
		{"git", "commit", "--allow-empty", "-m", "init"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git setup %v failed: %v\n%s", args, err, out)
		}
	}
}

func checkoutBranch(t *testing.T, dir string, branch string) {
	t.Helper()
	cmd := exec.Command("git", "checkout", "-b", branch)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("checkout branch %s failed: %v\n%s", branch, err, out)
	}
}

func getCurrentBranch(t *testing.T, dir string) string {
	t.Helper()
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("get current branch: %v", err)
	}
	return string(out[:len(out)-1]) // trim newline
}
