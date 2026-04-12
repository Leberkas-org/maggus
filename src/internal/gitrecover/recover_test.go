package gitrecover

import (
	"os"
	"path/filepath"
	"testing"
)

// TestRecoverDirtyState_CleanRepo verifies that RecoverDirtyState is a no-op and
// completes quickly on a clean repository with no COMMIT.md and no orphaned worktrees.
func TestRecoverDirtyState_CleanRepo(t *testing.T) {
	repo := initRepoOnMain(t)

	featureStore, bugStore := emptyStores()

	logs, err := RecoverDirtyState(repo, defaultCfg(), featureStore, bugStore)
	if err != nil {
		t.Fatalf("RecoverDirtyState on clean repo: %v", err)
	}
	if len(logs) != 0 {
		t.Errorf("expected no log messages for clean repo, got: %v", logs)
	}
}

// TestRecoverDirtyState_Step2FailureDoesNotPreventStep3 verifies that when step 2
// (consolidateBranches) fails with a merge conflict, step 3 (cleanOrphanedWorktrees)
// still runs and cleans up orphaned worktrees.
func TestRecoverDirtyState_Step2FailureDoesNotPreventStep3(t *testing.T) {
	repo := initRepoOnMain(t)

	// Create feature/maggus with a divergent change to produce a merge conflict.
	runCmd(t, repo, "git", "checkout", "-b", "feature/maggus")
	writeFile(t, filepath.Join(repo, "conflict.txt"), "integration version")
	runCmd(t, repo, "git", "add", ".")
	runCmd(t, repo, "git", "commit", "-m", "integration change")

	// Create task branch off main with a conflicting change.
	// feature/maggus and this task branch have diverged from main, so the
	// merge attempt in consolidateBranches will conflict.
	runCmd(t, repo, "git", "checkout", "main")
	runCmd(t, repo, "git", "checkout", "-b", "feature/maggus-001/task-001")
	writeFile(t, filepath.Join(repo, "conflict.txt"), "task version")
	runCmd(t, repo, "git", "add", ".")
	runCmd(t, repo, "git", "commit", "-m", "task change")

	// Create an orphaned worktree using a task-branch name so it is filtered out
	// as a merge candidate by consolidateBranches, yet cleaned up by step 3.
	wtPath := filepath.Join(repo, ".maggus", "worktrees", "wt-orphan")
	if err := os.MkdirAll(filepath.Dir(wtPath), 0o755); err != nil {
		t.Fatal(err)
	}
	runCmd(t, repo, "git", "worktree", "add", "-b", "feature/maggus-002/task-001", wtPath)

	featureStore, bugStore := emptyStores()

	logs, err := RecoverDirtyState(repo, defaultCfg(), featureStore, bugStore)

	// Step 2 must have returned an error (merge conflict).
	if err == nil {
		t.Error("expected error from step 2 (merge conflict), got nil")
	}

	// Step 3 must have run: orphaned worktree should be removed despite step 2 failing.
	if worktreeDirExists(wtPath) {
		t.Error("step 3 should have run despite step 2 failure: orphaned worktree must be removed")
	}

	// Log from step 3 must appear in the accumulated messages.
	if !containsSubstring(logs, "removed orphaned worktree") {
		t.Errorf("expected step 3 log message, got: %v", logs)
	}
}

// TestRecoverDirtyState_AllStepsRun verifies that log messages from multiple
// active steps are all accumulated in the return value.
func TestRecoverDirtyState_AllStepsRun(t *testing.T) {
	repo := initRepoOnMain(t)

	// Create a task branch so step 2 has something to do.
	runCmd(t, repo, "git", "checkout", "-b", "feature/maggus-003/task-001")
	writeFile(t, filepath.Join(repo, "task.txt"), "task work")
	runCmd(t, repo, "git", "add", ".")
	runCmd(t, repo, "git", "commit", "-m", "task work")

	// Create an orphaned worktree for step 3 to clean up.
	wtPath := createWorktreeDir(t, repo, "wt-all-steps", "feature/orphan-all")

	featureStore, bugStore := emptyStores()

	logs, _ := RecoverDirtyState(repo, defaultCfg(), featureStore, bugStore)

	// Step 3 must have removed the orphaned worktree.
	if worktreeDirExists(wtPath) {
		t.Error("step 3 should have removed the orphaned worktree")
	}

	// At least one log message expected (from step 2 or 3).
	if len(logs) == 0 {
		t.Error("expected at least one log message from the active steps")
	}
}
