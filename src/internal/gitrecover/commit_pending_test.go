package gitrecover

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leberkas-org/maggus/internal/parser"
	"github.com/leberkas-org/maggus/internal/stores"
)

// makeWorkableTask returns a workable parser.Task with the given ID.
func makeWorkableTask(id string) parser.Task {
	return parser.Task{
		ID: id,
		Criteria: []parser.Criterion{
			{Text: "do something", Checked: false, Blocked: false},
		},
	}
}

// emptyStores returns empty in-memory feature and bug stores.
func emptyStores() (stores.FeatureStore, stores.BugStore) {
	return stores.NewMemFeatureStore(nil), stores.NewMemBugStore(nil)
}

// storesWithTask returns stores where the next workable task has the given ID.
func storesWithTask(taskID string) (stores.FeatureStore, stores.BugStore) {
	plan := parser.Plan{
		ID:   "feature_001",
		File: "/fake/feature_001.md",
		Tasks: []parser.Task{makeWorkableTask(taskID)},
	}
	return stores.NewMemFeatureStore([]parser.Plan{plan}), stores.NewMemBugStore(nil)
}

// commitCount returns the number of commits in the repo.
func commitCount(t *testing.T, repo string) int {
	t.Helper()
	cmd := exec.Command("git", "rev-list", "--count", "HEAD")
	cmd.Dir = repo
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git rev-list --count HEAD: %v", err)
	}
	count := 0
	for _, c := range strings.TrimSpace(string(out)) {
		if c >= '0' && c <= '9' {
			count = count*10 + int(c-'0')
		}
	}
	return count
}

// isClean reports whether the repo has no uncommitted changes.
func isClean(t *testing.T, repo string) bool {
	t.Helper()
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = repo
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git status --porcelain: %v", err)
	}
	return strings.TrimSpace(string(out)) == ""
}

// TestCommitPending_CommitMDPresent verifies that when COMMIT.md exists, the
// pending changes are committed using gitcommit.CommitIteration, COMMIT.md is
// removed, and a log message is returned.
func TestCommitPending_CommitMDPresent(t *testing.T) {
	repo := initRepo(t)
	before := commitCount(t, repo)

	// Stage a file so there is something to commit.
	writeFile(t, filepath.Join(repo, "work.txt"), "agent work")
	runCmd(t, repo, "git", "add", "work.txt")

	// Write a minimal COMMIT.md.
	writeFile(t, filepath.Join(repo, "COMMIT.md"), "feat: agent completed task\n")

	featureStore, bugStore := emptyStores()

	logs, err := commitPending(repo, featureStore, bugStore)
	if err != nil {
		t.Fatalf("commitPending: %v", err)
	}

	// A log message must be present.
	if len(logs) == 0 {
		t.Error("expected log messages, got none")
	}

	// COMMIT.md must be removed after the commit.
	if _, err := os.Stat(filepath.Join(repo, "COMMIT.md")); !os.IsNotExist(err) {
		t.Error("COMMIT.md should be removed after successful commit")
	}

	// A new commit must have been created.
	if got := commitCount(t, repo); got <= before {
		t.Errorf("expected more than %d commits after COMMIT.md scenario, got %d", before, got)
	}

	// The commit message must mention the task.
	if !containsSubstring(logs, "committed pending work") {
		t.Errorf("expected 'committed pending work' in log, got: %v", logs)
	}
}

// TestCommitPending_DirtyMismatched verifies the recovery-commit path: dirty
// working tree, on a task branch whose task ID differs from the next workable
// task in the stores.
func TestCommitPending_DirtyMismatched(t *testing.T) {
	repo := initRepo(t)
	before := commitCount(t, repo)

	// Switch to a task branch for TASK-001-001.
	runCmd(t, repo, "git", "checkout", "-b", "feature/maggus-001/task-001")

	// Create an unstaged dirty file.
	writeFile(t, filepath.Join(repo, "dirty.txt"), "some agent work")

	// Next workable task is TASK-001-002 — mismatch with current branch.
	featureStore, bugStore := storesWithTask("TASK-001-002")

	logs, err := commitPending(repo, featureStore, bugStore)
	if err != nil {
		t.Fatalf("commitPending: %v", err)
	}

	// A recovery log must be present.
	if !containsSubstring(logs, "recovered uncommitted changes") {
		t.Errorf("expected recovery log, got: %v", logs)
	}

	// Working tree must be clean after recovery.
	if !isClean(t, repo) {
		t.Error("working tree should be clean after recovery commit")
	}

	// A new commit must have been created.
	if got := commitCount(t, repo); got <= before {
		t.Errorf("expected more than %d commits after recovery, got %d", before, got)
	}

	// The commit message on HEAD must be the recovery message.
	cmd := exec.Command("git", "log", "-1", "--format=%s")
	cmd.Dir = repo
	out, _ := cmd.Output()
	subject := strings.TrimSpace(string(out))
	if !strings.Contains(subject, "maggus: recover uncommitted changes from") {
		t.Errorf("recovery commit message = %q, want prefix 'maggus: recover uncommitted changes from'", subject)
	}
}

// TestCommitPending_DirtyMatched verifies the resume scenario: dirty working
// tree, on a task branch whose task ID matches the next workable task — no commit
// is made and the dirty state is preserved.
func TestCommitPending_DirtyMatched(t *testing.T) {
	repo := initRepo(t)
	before := commitCount(t, repo)

	// Switch to a task branch for TASK-001-001.
	runCmd(t, repo, "git", "checkout", "-b", "feature/maggus-001/task-001")

	// Create an unstaged dirty file.
	writeFile(t, filepath.Join(repo, "resumable.txt"), "in-progress work")

	// Next workable task is also TASK-001-001 — same task, resume scenario.
	featureStore, bugStore := storesWithTask("TASK-001-001")

	logs, err := commitPending(repo, featureStore, bugStore)
	if err != nil {
		t.Fatalf("commitPending: %v", err)
	}

	// No log messages: nothing was done.
	if len(logs) != 0 {
		t.Errorf("expected nil logs for resume scenario, got: %v", logs)
	}

	// Working tree must still be dirty (resume requires the files to be there).
	if isClean(t, repo) {
		t.Error("working tree should remain dirty in the resume scenario")
	}

	// No new commit must have been created.
	if got := commitCount(t, repo); got != before {
		t.Errorf("expected %d commits (no new commit), got %d", before, got)
	}
}

// TestCommitPending_CleanRepo verifies that a clean repository with no COMMIT.md
// is a no-op.
func TestCommitPending_CleanRepo(t *testing.T) {
	repo := initRepo(t)
	before := commitCount(t, repo)

	featureStore, bugStore := emptyStores()

	logs, err := commitPending(repo, featureStore, bugStore)
	if err != nil {
		t.Fatalf("commitPending: %v", err)
	}

	if len(logs) != 0 {
		t.Errorf("expected nil logs for clean repo, got: %v", logs)
	}

	if got := commitCount(t, repo); got != before {
		t.Errorf("expected %d commits (no change), got %d", before, got)
	}
}

// TestCommitPending_NotTaskBranch verifies that a dirty tree on a non-task branch
// is a no-op (not our business to commit).
func TestCommitPending_NotTaskBranch(t *testing.T) {
	repo := initRepo(t)
	before := commitCount(t, repo)

	// Repo starts on master/main — not a task branch.
	writeFile(t, filepath.Join(repo, "dirty.txt"), "some work")

	featureStore, bugStore := emptyStores()

	logs, err := commitPending(repo, featureStore, bugStore)
	if err != nil {
		t.Fatalf("commitPending: %v", err)
	}

	if len(logs) != 0 {
		t.Errorf("expected nil logs for non-task branch, got: %v", logs)
	}

	if got := commitCount(t, repo); got != before {
		t.Errorf("expected %d commits (no change), got %d", before, got)
	}
}
