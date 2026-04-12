package gitrecover

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leberkas-org/maggus/internal/runlog"
)

// --- helpers ---

func initRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
		{"git", "commit", "--allow-empty", "-m", "initial commit"},
	}
	for _, args := range cmds {
		runCmd(t, repo, args...)
	}
	return repo
}

func runCmd(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v failed: %v\n%s", args, err, out)
	}
}

func createWorktreeDir(t *testing.T, repo, name, branch string) string {
	t.Helper()
	wtDir := filepath.Join(repo, ".maggus", "worktrees", name)
	if err := os.MkdirAll(filepath.Dir(wtDir), 0o755); err != nil {
		t.Fatal(err)
	}
	runCmd(t, repo, "git", "worktree", "add", "-b", branch, wtDir)
	return wtDir
}

func branchExists(t *testing.T, repo, branch string) bool {
	t.Helper()
	cmd := exec.Command("git", "rev-parse", "--verify", "--quiet", branch)
	cmd.Dir = repo
	return cmd.Run() == nil
}

func worktreeDirExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// writeWorkersIndex writes a fake state-workers.json under .maggus/runs/.
func writeWorkersIndex(t *testing.T, repoDir string, workers []runlog.WorkerIndexEntry) {
	t.Helper()
	runsDir := filepath.Join(repoDir, ".maggus", "runs")
	if err := os.MkdirAll(runsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(struct {
		Workers []runlog.WorkerIndexEntry `json:"workers"`
	}{Workers: workers})
	if err != nil {
		t.Fatal(err)
	}
	indexPath := filepath.Join(runsDir, "state-workers.json")
	if err := os.WriteFile(indexPath, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// --- tests ---

// TestCleanOrphanedWorktrees_OrphanRemoved verifies that a worktree under
// .maggus/worktrees/ is removed when no active worker exists for its task.
func TestCleanOrphanedWorktrees_OrphanRemoved(t *testing.T) {
	repo := initRepo(t)

	// Create a worktree under .maggus/worktrees/ (non-task branch to keep it simple).
	wtPath := createWorktreeDir(t, repo, "wt-orphan", "feature/orphan")

	logs, err := cleanOrphanedWorktrees(repo)
	if err != nil {
		t.Fatalf("cleanOrphanedWorktrees: %v", err)
	}

	// The worktree directory must be gone.
	if worktreeDirExists(wtPath) {
		t.Error("expected orphaned worktree directory to be removed")
	}

	// A log message mentioning "removed" must be present.
	if !containsSubstring(logs, "removed orphaned worktree") {
		t.Errorf("expected log about removal, got: %v", logs)
	}
}

// TestCleanOrphanedWorktrees_MergedBranchDeleted verifies that after worktree
// removal the task branch is also deleted when it is fully merged into HEAD.
func TestCleanOrphanedWorktrees_MergedBranchDeleted(t *testing.T) {
	repo := initRepo(t)
	// Capture default branch name before checking out any other branch.
	defaultBranch := detectDefaultBranch(t, repo)

	taskBranch := "feature/maggus-001/task-001"

	// Create the task branch off HEAD and add a commit so it has unique history.
	runCmd(t, repo, "git", "checkout", "-b", taskBranch)
	writeFile(t, filepath.Join(repo, "task.txt"), "task work")
	runCmd(t, repo, "git", "add", ".")
	runCmd(t, repo, "git", "commit", "-m", "task commit")

	// Merge the task branch back into the default branch so it is an ancestor of HEAD.
	runCmd(t, repo, "git", "checkout", defaultBranch)
	runCmd(t, repo, "git", "merge", "--no-ff", taskBranch)

	// Now create a worktree pointing to the (already-merged) task branch.
	wtPath := filepath.Join(repo, ".maggus", "worktrees", "task-001")
	if err := os.MkdirAll(filepath.Dir(wtPath), 0o755); err != nil {
		t.Fatal(err)
	}
	runCmd(t, repo, "git", "worktree", "add", wtPath, taskBranch)

	logs, err := cleanOrphanedWorktrees(repo)
	if err != nil {
		t.Fatalf("cleanOrphanedWorktrees: %v", err)
	}

	// Worktree must be removed.
	if worktreeDirExists(wtPath) {
		t.Error("expected worktree directory to be removed")
	}

	// Task branch must be deleted (was an ancestor of HEAD).
	if branchExists(t, repo, taskBranch) {
		t.Errorf("expected merged task branch %q to be deleted", taskBranch)
	}

	// Log messages must mention both the worktree removal and branch deletion.
	if !containsSubstring(logs, "removed orphaned worktree") {
		t.Errorf("expected removal log, got: %v", logs)
	}
	if !containsSubstring(logs, "deleted merged branch") {
		t.Errorf("expected branch deletion log, got: %v", logs)
	}
}

// TestCleanOrphanedWorktrees_DivergentBranchPreserved verifies that a task branch
// with unmerged work is NOT deleted; only the worktree is removed.
func TestCleanOrphanedWorktrees_DivergentBranchPreserved(t *testing.T) {
	repo := initRepo(t)
	// Capture default branch name before checking out any other branch.
	defaultBranch := detectDefaultBranch(t, repo)

	taskBranch := "feature/maggus-002/task-001"

	// Create the task branch and add a commit that is NOT merged into the default branch.
	runCmd(t, repo, "git", "checkout", "-b", taskBranch)
	writeFile(t, filepath.Join(repo, "task2.txt"), "divergent work")
	runCmd(t, repo, "git", "add", ".")
	runCmd(t, repo, "git", "commit", "-m", "divergent commit")

	// Go back to the default branch WITHOUT merging.
	runCmd(t, repo, "git", "checkout", defaultBranch)

	// Create a worktree on the (unmerged) task branch.
	wtPath := filepath.Join(repo, ".maggus", "worktrees", "task-002")
	if err := os.MkdirAll(filepath.Dir(wtPath), 0o755); err != nil {
		t.Fatal(err)
	}
	runCmd(t, repo, "git", "worktree", "add", wtPath, taskBranch)

	logs, err := cleanOrphanedWorktrees(repo)
	if err != nil {
		t.Fatalf("cleanOrphanedWorktrees: %v", err)
	}

	// Worktree must be removed.
	if worktreeDirExists(wtPath) {
		t.Error("expected worktree directory to be removed")
	}

	// Task branch must still exist (divergent work preserved).
	if !branchExists(t, repo, taskBranch) {
		t.Errorf("expected divergent task branch %q to be preserved", taskBranch)
	}

	// A warning about the preserved branch must appear in the logs.
	if !containsSubstring(logs, "warning") || !containsSubstring(logs, "divergent") {
		t.Errorf("expected warning about divergent branch, got: %v", logs)
	}
}

// TestCleanOrphanedWorktrees_ActiveWorkerSkipped verifies that a worktree whose
// task ID shows status "working" in the worker index is left untouched.
func TestCleanOrphanedWorktrees_ActiveWorkerSkipped(t *testing.T) {
	repo := initRepo(t)

	taskBranch := "feature/maggus-003/task-001"
	taskID := "TASK-003-001"

	// Create a worktree under .maggus/worktrees/.
	wtPath := filepath.Join(repo, ".maggus", "worktrees", "task-003")
	if err := os.MkdirAll(filepath.Dir(wtPath), 0o755); err != nil {
		t.Fatal(err)
	}
	runCmd(t, repo, "git", "worktree", "add", "-b", taskBranch, wtPath)

	// Write a workers index that marks this task as actively running.
	writeWorkersIndex(t, repo, []runlog.WorkerIndexEntry{
		{TaskID: taskID, TaskTitle: "test task", Status: "working"},
	})

	logs, err := cleanOrphanedWorktrees(repo)
	if err != nil {
		t.Fatalf("cleanOrphanedWorktrees: %v", err)
	}

	// Worktree must NOT have been removed.
	if !worktreeDirExists(wtPath) {
		t.Error("active worktree should not be removed")
	}

	// A skip message must appear in the logs.
	if !containsSubstring(logs, "skipping active worktree") {
		t.Errorf("expected skip log, got: %v", logs)
	}
}

// TestCleanOrphanedWorktrees_IgnoresNonMaggusWorktrees verifies that worktrees
// outside .maggus/worktrees/ are not touched.
func TestCleanOrphanedWorktrees_IgnoresNonMaggusWorktrees(t *testing.T) {
	repo := initRepo(t)

	// Create a worktree that is NOT under .maggus/worktrees/.
	externalWt := filepath.Join(t.TempDir(), "external-wt")
	runCmd(t, repo, "git", "worktree", "add", "-b", "feature/external", externalWt)

	logs, err := cleanOrphanedWorktrees(repo)
	if err != nil {
		t.Fatalf("cleanOrphanedWorktrees: %v", err)
	}

	// The external worktree must still exist.
	if !worktreeDirExists(externalWt) {
		t.Error("external worktree must not be removed by cleanOrphanedWorktrees")
	}

	// No removal logs expected.
	if containsSubstring(logs, "removed orphaned worktree") {
		t.Errorf("should not log removal for external worktree, got: %v", logs)
	}
}

// TestCleanOrphanedWorktrees_RemovesEmptyWorktreesDir verifies that the
// .maggus/worktrees/ directory is removed when it becomes empty after cleanup.
func TestCleanOrphanedWorktrees_RemovesEmptyWorktreesDir(t *testing.T) {
	repo := initRepo(t)

	// Create and then clean a single worktree so the directory becomes empty.
	createWorktreeDir(t, repo, "wt-empty", "feature/emptied")

	if _, err := cleanOrphanedWorktrees(repo); err != nil {
		t.Fatalf("cleanOrphanedWorktrees: %v", err)
	}

	wtBaseDir := filepath.Join(repo, ".maggus", "worktrees")
	if _, err := os.Stat(wtBaseDir); !os.IsNotExist(err) {
		t.Errorf("expected .maggus/worktrees/ to be removed when empty, stat err = %v", err)
	}
}

// --- helpers ---

func containsSubstring(lines []string, sub string) bool {
	for _, l := range lines {
		if strings.Contains(l, sub) {
			return true
		}
	}
	return false
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

// detectDefaultBranch returns the current branch name (the one the repo was
// initialised on), typically "master" or "main".
func detectDefaultBranch(t *testing.T, repo string) string {
	t.Helper()
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = repo
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("detect default branch: %v", err)
	}
	return strings.TrimSpace(string(out))
}
