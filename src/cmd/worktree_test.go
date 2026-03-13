package cmd

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// initWorktreeTestRepo creates a temp git repo with one commit for worktree tests.
func initWorktreeTestRepo(t *testing.T) string {
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

func TestWorktreeListEmpty(t *testing.T) {
	dir := initWorktreeTestRepo(t)

	var buf bytes.Buffer
	worktreeListCmd.SetOut(&buf)
	defer worktreeListCmd.SetOut(nil)

	if err := runWorktreeList(worktreeListCmd, dir); err != nil {
		t.Fatalf("runWorktreeList: %v", err)
	}

	if !strings.Contains(buf.String(), "No worktrees found") {
		t.Errorf("expected 'No worktrees found', got: %s", buf.String())
	}
}

func TestWorktreeListWithWorktree(t *testing.T) {
	dir := initWorktreeTestRepo(t)

	// Create .maggus-work directory with a worktree.
	wtPath := filepath.Join(dir, maggusWorkDir, "run-123")
	cmd := exec.Command("git", "worktree", "add", wtPath, "-b", "feature/test-wt")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git worktree add: %v: %s", err, out)
	}

	var buf bytes.Buffer
	worktreeListCmd.SetOut(&buf)
	defer worktreeListCmd.SetOut(nil)

	if err := runWorktreeList(worktreeListCmd, dir); err != nil {
		t.Fatalf("runWorktreeList: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "run-123") {
		t.Errorf("expected run ID 'run-123' in output, got: %s", output)
	}
	if !strings.Contains(output, "feature/test-wt") {
		t.Errorf("expected branch name in output, got: %s", output)
	}
}

func TestWorktreeCleanEmpty(t *testing.T) {
	dir := initWorktreeTestRepo(t)

	var buf bytes.Buffer
	worktreeCleanCmd.SetOut(&buf)
	defer worktreeCleanCmd.SetOut(nil)

	if err := runWorktreeClean(worktreeCleanCmd, dir); err != nil {
		t.Fatalf("runWorktreeClean: %v", err)
	}

	if !strings.Contains(buf.String(), "No worktrees found") {
		t.Errorf("expected 'No worktrees found', got: %s", buf.String())
	}
}

func TestWorktreeCleanRemovesWorktrees(t *testing.T) {
	dir := initWorktreeTestRepo(t)

	// Create two worktrees.
	for _, name := range []string{"run-aaa", "run-bbb"} {
		wtPath := filepath.Join(dir, maggusWorkDir, name)
		cmd := exec.Command("git", "worktree", "add", wtPath, "-b", "feature/"+name)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git worktree add %s: %v: %s", name, err, out)
		}
	}

	// Create some lock files.
	locksDir := filepath.Join(dir, ".maggus", "locks")
	os.MkdirAll(locksDir, 0755)
	os.WriteFile(filepath.Join(locksDir, "TASK-001.lock"), []byte("test"), 0644)

	var buf bytes.Buffer
	worktreeCleanCmd.SetOut(&buf)
	defer worktreeCleanCmd.SetOut(nil)

	if err := runWorktreeClean(worktreeCleanCmd, dir); err != nil {
		t.Fatalf("runWorktreeClean: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Cleaned 2 worktree(s)") {
		t.Errorf("expected 'Cleaned 2 worktree(s)', got: %s", output)
	}
	if !strings.Contains(output, "Cleaned lock files") {
		t.Errorf("expected 'Cleaned lock files', got: %s", output)
	}

	// Worktree directories should be gone.
	for _, name := range []string{"run-aaa", "run-bbb"} {
		wtPath := filepath.Join(dir, maggusWorkDir, name)
		if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
			t.Errorf("worktree %s still exists after clean", name)
		}
	}

	// Lock files should be gone.
	lockPath := filepath.Join(locksDir, "TASK-001.lock")
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Error("lock file still exists after clean")
	}
}

func TestWorktreeCleanDeletesBranches(t *testing.T) {
	dir := initWorktreeTestRepo(t)

	wtPath := filepath.Join(dir, maggusWorkDir, "run-xyz")
	cmd := exec.Command("git", "worktree", "add", wtPath, "-b", "feature/maggustask-099")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git worktree add: %v: %s", err, out)
	}

	var buf bytes.Buffer
	worktreeCleanCmd.SetOut(&buf)
	defer worktreeCleanCmd.SetOut(nil)

	if err := runWorktreeClean(worktreeCleanCmd, dir); err != nil {
		t.Fatalf("runWorktreeClean: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "feature/maggustask-099") {
		t.Errorf("expected branch name in output, got: %s", output)
	}

	// Branch should be deleted.
	branchCmd := exec.Command("git", "branch", "--list", "feature/maggustask-099")
	branchCmd.Dir = dir
	branchOut, _ := branchCmd.Output()
	if strings.Contains(string(branchOut), "feature/maggustask-099") {
		t.Error("branch still exists after clean")
	}
}
