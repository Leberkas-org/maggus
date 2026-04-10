package gitworktree

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// initGitRepo initialises a git repository in dir with one empty commit.
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

func TestCreateWorktree_NewBranch(t *testing.T) {
	repo := t.TempDir()
	initGitRepo(t, repo)

	wt := filepath.Join(t.TempDir(), "wt1")
	branch := "feature/task-001"

	if err := CreateWorktree(repo, wt, branch); err != nil {
		t.Fatalf("CreateWorktree failed: %v", err)
	}

	// Worktree directory should exist.
	if _, err := os.Stat(wt); err != nil {
		t.Fatalf("worktree directory not created: %v", err)
	}

	// The branch should be checked out inside the worktree.
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = wt
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("rev-parse in worktree: %v", err)
	}
	if got := strings.TrimSpace(string(out)); got != branch {
		t.Errorf("worktree branch = %q, want %q", got, branch)
	}
}

func TestCreateWorktree_ExistingBranch(t *testing.T) {
	repo := t.TempDir()
	initGitRepo(t, repo)

	// Create the branch in the main repo first.
	branch := "feature/preexisting"
	cmd := exec.Command("git", "branch", branch)
	cmd.Dir = repo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git branch: %v\n%s", err, out)
	}

	wt := filepath.Join(t.TempDir(), "wt2")
	if err := CreateWorktree(repo, wt, branch); err != nil {
		t.Fatalf("CreateWorktree on existing branch failed: %v", err)
	}

	if _, err := os.Stat(wt); err != nil {
		t.Fatalf("worktree directory not created: %v", err)
	}
}

func TestRemoveWorktree(t *testing.T) {
	repo := t.TempDir()
	initGitRepo(t, repo)

	wt := filepath.Join(t.TempDir(), "wt3")
	if err := CreateWorktree(repo, wt, "feature/remove-me"); err != nil {
		t.Fatalf("CreateWorktree: %v", err)
	}

	if err := RemoveWorktree(repo, wt); err != nil {
		t.Fatalf("RemoveWorktree failed: %v", err)
	}

	// Directory should be gone after removal.
	if _, err := os.Stat(wt); !os.IsNotExist(err) {
		t.Errorf("expected worktree directory to be removed, stat err = %v", err)
	}
}

func TestListWorktrees_IncludesMain(t *testing.T) {
	repo := t.TempDir()
	initGitRepo(t, repo)

	wts, err := ListWorktrees(repo)
	if err != nil {
		t.Fatalf("ListWorktrees failed: %v", err)
	}
	if len(wts) < 1 {
		t.Fatal("expected at least the main worktree")
	}
}

func TestListWorktrees_AfterCreate(t *testing.T) {
	repo := t.TempDir()
	initGitRepo(t, repo)

	wt := filepath.Join(t.TempDir(), "wt4")
	if err := CreateWorktree(repo, wt, "feature/listed"); err != nil {
		t.Fatalf("CreateWorktree: %v", err)
	}

	wts, err := ListWorktrees(repo)
	if err != nil {
		t.Fatalf("ListWorktrees failed: %v", err)
	}
	if len(wts) < 2 {
		t.Fatalf("expected at least 2 worktrees, got %d", len(wts))
	}

	found := false
	for _, w := range wts {
		if w.Branch == "feature/listed" {
			found = true
			if w.HEAD == "" {
				t.Error("WorktreeInfo.HEAD should not be empty")
			}
			if w.Path == "" {
				t.Error("WorktreeInfo.Path should not be empty")
			}
		}
	}
	if !found {
		t.Errorf("created worktree branch not found in list: %+v", wts)
	}
}

func TestListWorktrees_AfterRemove(t *testing.T) {
	repo := t.TempDir()
	initGitRepo(t, repo)

	wt := filepath.Join(t.TempDir(), "wt5")
	if err := CreateWorktree(repo, wt, "feature/removed"); err != nil {
		t.Fatalf("CreateWorktree: %v", err)
	}

	if err := RemoveWorktree(repo, wt); err != nil {
		t.Fatalf("RemoveWorktree: %v", err)
	}

	wts, err := ListWorktrees(repo)
	if err != nil {
		t.Fatalf("ListWorktrees after remove failed: %v", err)
	}
	for _, w := range wts {
		if w.Branch == "feature/removed" {
			t.Errorf("removed worktree still present in list: %+v", w)
		}
	}
}

func TestListWorktrees_PrunesStale(t *testing.T) {
	repo := t.TempDir()
	initGitRepo(t, repo)

	wt := filepath.Join(t.TempDir(), "wt6")
	if err := CreateWorktree(repo, wt, "feature/stale"); err != nil {
		t.Fatalf("CreateWorktree: %v", err)
	}

	// Simulate a stale worktree by deleting the directory without using git.
	if err := os.RemoveAll(wt); err != nil {
		t.Fatalf("RemoveAll: %v", err)
	}

	// ListWorktrees must prune the stale entry and not return it.
	wts, err := ListWorktrees(repo)
	if err != nil {
		t.Fatalf("ListWorktrees failed: %v", err)
	}
	for _, w := range wts {
		if w.Branch == "feature/stale" {
			t.Errorf("stale worktree not pruned, still present: %+v", w)
		}
	}
}

func TestParsePorcelain(t *testing.T) {
	input := "worktree /path/to/main\nHEAD abc123\nbranch refs/heads/main\n\nworktree /path/to/feat\nHEAD def456\nbranch refs/heads/feature/foo\n\n"

	wts := parsePorcelain(input)
	if len(wts) != 2 {
		t.Fatalf("expected 2 worktrees, got %d", len(wts))
	}

	if wts[0].Path != "/path/to/main" {
		t.Errorf("wts[0].Path = %q, want %q", wts[0].Path, "/path/to/main")
	}
	if wts[0].HEAD != "abc123" {
		t.Errorf("wts[0].HEAD = %q, want %q", wts[0].HEAD, "abc123")
	}
	if wts[0].Branch != "main" {
		t.Errorf("wts[0].Branch = %q, want %q", wts[0].Branch, "main")
	}

	if wts[1].Path != "/path/to/feat" {
		t.Errorf("wts[1].Path = %q, want %q", wts[1].Path, "/path/to/feat")
	}
	if wts[1].HEAD != "def456" {
		t.Errorf("wts[1].HEAD = %q, want %q", wts[1].HEAD, "def456")
	}
	if wts[1].Branch != "feature/foo" {
		t.Errorf("wts[1].Branch = %q, want %q", wts[1].Branch, "feature/foo")
	}
}

func TestParsePorcelain_DetachedHEAD(t *testing.T) {
	input := "worktree /path/to/detached\nHEAD abc789\ndetached\n\n"

	wts := parsePorcelain(input)
	if len(wts) != 1 {
		t.Fatalf("expected 1 worktree, got %d", len(wts))
	}
	if wts[0].Branch != "" {
		t.Errorf("detached HEAD worktree should have empty Branch, got %q", wts[0].Branch)
	}
	if wts[0].HEAD != "abc789" {
		t.Errorf("HEAD = %q, want %q", wts[0].HEAD, "abc789")
	}
}

func TestParsePorcelain_NoTrailingNewline(t *testing.T) {
	// Some git versions omit the final blank line.
	input := "worktree /path/to/main\nHEAD abc123\nbranch refs/heads/main"

	wts := parsePorcelain(input)
	if len(wts) != 1 {
		t.Fatalf("expected 1 worktree, got %d", len(wts))
	}
}
