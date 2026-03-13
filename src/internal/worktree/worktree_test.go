package worktree

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// initTestRepo creates a temporary bare-bones git repo with one commit.
func initTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	for _, args := range [][]string{
		{"init", "-b", "main"},
		{"config", "user.email", "test@test.com"},
		{"config", "user.name", "Test"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}

	// Create an initial commit so branches can be created.
	f := filepath.Join(dir, "README.md")
	if err := os.WriteFile(f, []byte("init"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"add", "README.md"},
		{"commit", "-m", "initial"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	return dir
}

func TestCreateAndList(t *testing.T) {
	repo := initTestRepo(t)
	wtDir := filepath.Join(repo, "wt-test")

	if err := Create(repo, wtDir, "feature/test-branch"); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Verify the worktree directory exists.
	if _, err := os.Stat(wtDir); os.IsNotExist(err) {
		t.Fatal("worktree directory was not created")
	}

	// List should include both the main repo and the new worktree.
	paths, err := List(repo)
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if len(paths) < 2 {
		t.Fatalf("expected at least 2 worktrees, got %d: %v", len(paths), paths)
	}

	found := false
	// Normalize paths for comparison (git on Windows returns forward slashes).
	normalizedWtDir := filepath.ToSlash(wtDir)
	for _, p := range paths {
		if filepath.ToSlash(p) == normalizedWtDir {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("worktree path %s not found in list: %v", wtDir, paths)
	}
}

func TestRemove(t *testing.T) {
	repo := initTestRepo(t)
	wtDir := filepath.Join(repo, "wt-remove")

	if err := Create(repo, wtDir, "feature/remove-branch"); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := Remove(repo, wtDir); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	// Directory should be gone.
	if _, err := os.Stat(wtDir); !os.IsNotExist(err) {
		t.Error("worktree directory still exists after Remove")
	}

	// List should only contain the main worktree.
	paths, err := List(repo)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	for _, p := range paths {
		if p == wtDir {
			t.Error("removed worktree still appears in list")
		}
	}
}

func TestCreateDuplicateBranch(t *testing.T) {
	repo := initTestRepo(t)
	wtDir1 := filepath.Join(repo, "wt-dup1")
	wtDir2 := filepath.Join(repo, "wt-dup2")

	if err := Create(repo, wtDir1, "feature/dup-branch"); err != nil {
		t.Fatalf("first Create: %v", err)
	}

	// Creating a second worktree with the same branch name should fail.
	err := Create(repo, wtDir2, "feature/dup-branch")
	if err == nil {
		t.Fatal("expected error when creating worktree with duplicate branch, got nil")
	}
	if !strings.Contains(err.Error(), "git worktree add") {
		t.Errorf("error should mention git worktree add, got: %v", err)
	}
}

func TestRemoveNonExistent(t *testing.T) {
	repo := initTestRepo(t)
	err := Remove(repo, filepath.Join(repo, "does-not-exist"))
	if err == nil {
		t.Fatal("expected error when removing non-existent worktree, got nil")
	}
	if !strings.Contains(err.Error(), "git worktree remove") {
		t.Errorf("error should mention git worktree remove, got: %v", err)
	}
}

func TestListEmptyRepo(t *testing.T) {
	repo := initTestRepo(t)

	// A repo with no extra worktrees should list at least the main one.
	paths, err := List(repo)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(paths) < 1 {
		t.Fatal("expected at least 1 worktree (main), got 0")
	}
}

func TestListInvalidDir(t *testing.T) {
	_, err := List("/nonexistent-dir-that-does-not-exist")
	if err == nil {
		t.Fatal("expected error for invalid directory, got nil")
	}
	if !strings.Contains(err.Error(), "git worktree list") {
		t.Errorf("error should mention git worktree list, got: %v", err)
	}
}

func TestCreateInvalidRepoDir(t *testing.T) {
	err := Create("/nonexistent-dir", "/tmp/wt", "branch")
	if err == nil {
		t.Fatal("expected error for invalid repo directory, got nil")
	}
}

func TestRemoveForceWithUncommittedChanges(t *testing.T) {
	repo := initTestRepo(t)
	wtDir := filepath.Join(repo, "wt-dirty")

	if err := Create(repo, wtDir, "feature/dirty-branch"); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Create an uncommitted file in the worktree.
	if err := os.WriteFile(filepath.Join(wtDir, "dirty.txt"), []byte("uncommitted"), 0644); err != nil {
		t.Fatal(err)
	}

	// Remove with --force should still succeed.
	if err := Remove(repo, wtDir); err != nil {
		t.Fatalf("Remove with dirty worktree: %v", err)
	}
}
