package gitrecover

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leberkas-org/maggus/internal/config"
)

// initRepoOnMain initialises a git repo with an explicit "main" default branch
// and an initial commit. This is separate from initRepo (in gitrecover_test.go)
// which does not pin the default branch name.
func initRepoOnMain(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	cmds := [][]string{
		{"git", "init", "-b", "main"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
	}
	for _, args := range cmds {
		runCmd(t, repo, args...)
	}
	writeFile(t, filepath.Join(repo, "init.txt"), "initial")
	runCmd(t, repo, "git", "add", ".")
	runCmd(t, repo, "git", "commit", "-m", "initial commit")
	return repo
}

// currentBranchOfRepo returns the current branch name of the repo at dir.
func currentBranchOfRepo(t *testing.T, dir string) string {
	t.Helper()
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("get current branch: %v", err)
	}
	return strings.TrimSpace(string(out))
}

func defaultCfg() config.Config {
	return config.Config{
		Git: config.GitConfig{
			ProtectedBranches: []string{"main", "master", "dev"},
		},
	}
}

// --- tests ---

// TestConsolidateBranches_NotTaskBranch verifies that the function is a no-op
// when the current branch is not a task branch.
func TestConsolidateBranches_NotTaskBranch(t *testing.T) {
	repo := initRepoOnMain(t)
	// Repo is on "main" (not a task branch).

	logs, err := consolidateBranches(repo, defaultCfg())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if logs != nil {
		t.Errorf("expected nil logs for non-task branch, got: %v", logs)
	}
}

// TestConsolidateBranches_BasicMerge verifies that the function merges the
// current task branch into its integration branch ancestor.
func TestConsolidateBranches_BasicMerge(t *testing.T) {
	repo := initRepoOnMain(t)

	// Create an integration branch. Note: we use "feature/plan-001" (not
	// "feature/maggus-001") to avoid the git ref hierarchy conflict where
	// refs/heads/feature/maggus-001 (file) and
	// refs/heads/feature/maggus-001/task-001 (file-in-dir) cannot coexist.
	runCmd(t, repo, "git", "checkout", "-b", "feature/plan-001")

	// Create task branch off the integration branch.
	runCmd(t, repo, "git", "checkout", "-b", "feature/maggus-001/task-001")
	writeFile(t, filepath.Join(repo, "task.txt"), "task work")
	runCmd(t, repo, "git", "add", ".")
	runCmd(t, repo, "git", "commit", "-m", "task work")

	// Call consolidate while on the task branch.
	logs, err := consolidateBranches(repo, defaultCfg())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(logs) == 0 {
		t.Error("expected log messages, got none")
	}

	// Should now be on the integration branch.
	cur := currentBranchOfRepo(t, repo)
	if cur != "feature/plan-001" {
		t.Errorf("current branch = %q, want %q", cur, "feature/plan-001")
	}

	// Merged file should exist on the integration branch.
	if _, err := os.Stat(filepath.Join(repo, "task.txt")); err != nil {
		t.Errorf("task.txt should exist after merge: %v", err)
	}

	// Task branch should be deleted (it is now an ancestor of HEAD).
	if branchExists(t, repo, "feature/maggus-001/task-001") {
		t.Error("task branch should be deleted after consolidation")
	}
}

// TestConsolidateBranches_NoAncestor_CreatesIntegrationBranch verifies that
// when no non-task, non-protected ancestor branch exists, the function creates
// "feature/maggus" (for TASK- IDs) off the first existing protected branch.
func TestConsolidateBranches_NoAncestor_CreatesIntegrationBranch(t *testing.T) {
	repo := initRepoOnMain(t)

	// Create task branch directly off main (no dedicated integration branch).
	runCmd(t, repo, "git", "checkout", "-b", "feature/maggus-001/task-001")
	writeFile(t, filepath.Join(repo, "task.txt"), "task work")
	runCmd(t, repo, "git", "add", ".")
	runCmd(t, repo, "git", "commit", "-m", "task work")

	logs, err := consolidateBranches(repo, defaultCfg())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(logs) == 0 {
		t.Error("expected log messages, got none")
	}

	// "feature/maggus" should have been created (TASK- prefix → feature).
	if !branchExists(t, repo, "feature/maggus") {
		t.Error("feature/maggus integration branch should have been created")
	}

	// Should now be on feature/maggus.
	cur := currentBranchOfRepo(t, repo)
	if cur != "feature/maggus" {
		t.Errorf("current branch = %q, want %q", cur, "feature/maggus")
	}

	// Task branch should be deleted (it was merged into feature/maggus).
	if branchExists(t, repo, "feature/maggus-001/task-001") {
		t.Error("task branch should be deleted after consolidation")
	}

	// At least one log message should mention the integration branch.
	found := false
	for _, l := range logs {
		if strings.Contains(l, "feature/maggus") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected log message mentioning feature/maggus, got: %v", logs)
	}
}

// TestConsolidateBranches_BugNoAncestor_CreatesBugIntegrationBranch verifies
// that a bug task branch (BUG- prefix) creates "bug/maggus" instead of
// "feature/maggus".
func TestConsolidateBranches_BugNoAncestor_CreatesBugIntegrationBranch(t *testing.T) {
	repo := initRepoOnMain(t)

	// Create bug task branch directly off main.
	runCmd(t, repo, "git", "checkout", "-b", "bugfix/maggus-bug-001/task-001")
	writeFile(t, filepath.Join(repo, "bugfix.txt"), "bugfix work")
	runCmd(t, repo, "git", "add", ".")
	runCmd(t, repo, "git", "commit", "-m", "bugfix work")

	_, err := consolidateBranches(repo, defaultCfg())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// "bug/maggus" should have been created (BUG- prefix → bug).
	if !branchExists(t, repo, "bug/maggus") {
		t.Error("bug/maggus integration branch should have been created")
	}

	cur := currentBranchOfRepo(t, repo)
	if cur != "bug/maggus" {
		t.Errorf("current branch = %q, want %q", cur, "bug/maggus")
	}
}

// TestConsolidateBranches_SiblingCleanup verifies that sibling task branches
// that are already merged into the integration branch get deleted.
func TestConsolidateBranches_SiblingCleanup(t *testing.T) {
	repo := initRepoOnMain(t)

	// Create integration branch.
	runCmd(t, repo, "git", "checkout", "-b", "feature/plan-001")

	// Create task-001, commit, and merge it into the integration branch.
	runCmd(t, repo, "git", "checkout", "-b", "feature/maggus-001/task-001")
	writeFile(t, filepath.Join(repo, "task1.txt"), "task1 work")
	runCmd(t, repo, "git", "add", ".")
	runCmd(t, repo, "git", "commit", "-m", "task-001 work")
	runCmd(t, repo, "git", "checkout", "feature/plan-001")
	runCmd(t, repo, "git", "merge", "--no-ff", "--no-edit", "feature/maggus-001/task-001")

	// Create task-002 off the integration branch, add a commit, stay on it.
	runCmd(t, repo, "git", "checkout", "-b", "feature/maggus-001/task-002")
	writeFile(t, filepath.Join(repo, "task2.txt"), "task2 work")
	runCmd(t, repo, "git", "add", ".")
	runCmd(t, repo, "git", "commit", "-m", "task-002 work")

	// Call consolidate while on task-002.
	logs, err := consolidateBranches(repo, defaultCfg())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Both task branches should be deleted.
	if branchExists(t, repo, "feature/maggus-001/task-001") {
		t.Error("task-001 should have been deleted (was already merged)")
	}
	if branchExists(t, repo, "feature/maggus-001/task-002") {
		t.Error("task-002 should have been deleted (just merged)")
	}

	// Should be on the integration branch.
	cur := currentBranchOfRepo(t, repo)
	if cur != "feature/plan-001" {
		t.Errorf("current branch = %q, want %q", cur, "feature/plan-001")
	}

	// Both task files should be present.
	if _, err := os.Stat(filepath.Join(repo, "task1.txt")); err != nil {
		t.Errorf("task1.txt should exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, "task2.txt")); err != nil {
		t.Errorf("task2.txt should exist: %v", err)
	}

	// Log messages should mention at least two deletions.
	deletions := 0
	for _, l := range logs {
		if strings.Contains(l, "deleted") {
			deletions++
		}
	}
	if deletions < 2 {
		t.Errorf("expected at least 2 deletion log messages, got %d in: %v", deletions, logs)
	}
}

// TestConsolidateBranches_MergeConflict verifies that a merge conflict is
// handled gracefully: the merge is aborted and a descriptive error is returned
// without leaving the repo in a mid-merge state.
//
// Setup: both the existing "feature/maggus" integration branch and the task
// branch independently add conflict.txt with different content, branching from
// the same base commit. From the task branch, "feature/maggus" is NOT an
// ancestor of HEAD (it diverged), so consolidateBranches takes the
// no-ancestor path, finds "feature/maggus" already exists (CreateBranchFrom
// is a no-op), checks it out, and attempts the merge — which conflicts.
func TestConsolidateBranches_MergeConflict(t *testing.T) {
	repo := initRepoOnMain(t)

	// Create feature/maggus as a pre-existing integration branch and make a
	// change to conflict.txt so it diverges from main.
	runCmd(t, repo, "git", "checkout", "-b", "feature/maggus")
	writeFile(t, filepath.Join(repo, "conflict.txt"), "integration version")
	runCmd(t, repo, "git", "add", ".")
	runCmd(t, repo, "git", "commit", "-m", "integration change")

	// Create task branch directly off main (common ancestor with feature/maggus
	// is main, so feature/maggus is NOT an ancestor of task-001) and add a
	// conflicting change to conflict.txt.
	runCmd(t, repo, "git", "checkout", "main")
	runCmd(t, repo, "git", "checkout", "-b", "feature/maggus-001/task-001")
	writeFile(t, filepath.Join(repo, "conflict.txt"), "task version")
	runCmd(t, repo, "git", "add", ".")
	runCmd(t, repo, "git", "commit", "-m", "task change")

	// Confirm setup: feature/maggus must NOT be an ancestor of task-001.
	ancestorCheck := exec.Command("git", "merge-base", "--is-ancestor", "feature/maggus", "HEAD")
	ancestorCheck.Dir = repo
	if ancestorCheck.Run() == nil {
		t.Fatal("test setup error: feature/maggus should not be an ancestor of task-001")
	}

	// Call consolidate on task-001. Since no non-protected ancestor exists,
	// the code takes the no-ancestor path: feature/maggus already exists so
	// CreateBranchFrom is a no-op, then the merge conflicts.
	_, err := consolidateBranches(repo, defaultCfg())
	if err == nil {
		t.Fatal("expected error for merge conflict, got nil")
	}

	// Repo must NOT be in mid-merge state (merge must have been aborted).
	checkMerge := exec.Command("git", "rev-parse", "-q", "--verify", "MERGE_HEAD")
	checkMerge.Dir = repo
	if checkMerge.Run() == nil {
		t.Error("repo should not be in mid-merge state (merge should have been aborted)")
	}

	// conflict.txt should be at the integration version (the state of
	// feature/maggus after the merge abort).
	content, readErr := os.ReadFile(filepath.Join(repo, "conflict.txt"))
	if readErr != nil {
		t.Fatalf("read conflict.txt: %v", readErr)
	}
	if string(content) != "integration version" {
		t.Errorf("conflict.txt should be at integration version after abort, got: %q", content)
	}
}
