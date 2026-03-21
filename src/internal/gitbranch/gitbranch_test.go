package gitbranch

import (
	"os/exec"
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
		{"TASK-001", "feature/maggustask-001"},
		{"TASK-003", "feature/maggustask-003"},
		{"TASK-042", "feature/maggustask-042"},
		{"TASK-100", "feature/maggustask-100"},
		{"TASK-008", "feature/maggustask-008"},
		{"TASK-1-E05", "feature/maggustask-1-e05"},
		{"TASK-2-A01", "feature/maggustask-2-a01"},
		{"INVALID", "feature/maggustask-000"},
	}

	for _, tt := range tests {
		got := FeatureBranchName(tt.taskID)
		if got != tt.want {
			t.Errorf("FeatureBranchName(%q) = %q, want %q", tt.taskID, got, tt.want)
		}
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
			if branch != "feature/maggustask-003" {
				t.Errorf("branch = %q, want %q", branch, "feature/maggustask-003")
			}
			if msg == "" {
				t.Error("expected a message about switching branches")
			}

			// Verify we're actually on the new branch
			got := getCurrentBranch(t, tmp)
			if got != "feature/maggustask-003" {
				t.Errorf("actual git branch = %q, want %q", got, "feature/maggustask-003")
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
	if branch != "feature/maggustask-005" {
		t.Errorf("branch = %q, want %q", branch, "feature/maggustask-005")
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
