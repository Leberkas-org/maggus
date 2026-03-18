package gitsync

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// initBareAndClone creates a bare "remote" repo and a clone of it.
// Returns (cloneDir, bareDir). Both are inside t.TempDir().
func initBareAndClone(t *testing.T) (string, string) {
	t.Helper()
	base := t.TempDir()
	bare := filepath.Join(base, "remote.git")
	clone := filepath.Join(base, "local")

	run(t, base, "git", "init", "--bare", bare)
	run(t, base, "git", "clone", bare, clone)
	run(t, clone, "git", "config", "user.email", "test@test.com")
	run(t, clone, "git", "config", "user.name", "Test")
	run(t, clone, "git", "commit", "--allow-empty", "-m", "init")
	run(t, clone, "git", "push", "-u", "origin", "HEAD")

	return clone, bare
}

// commitInDir makes an empty commit in the given directory.
func commitInDir(t *testing.T, dir, msg string) {
	t.Helper()
	run(t, dir, "git", "commit", "--allow-empty", "-m", msg)
}

// run executes a command and fails the test on error.
func run(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v failed: %v\n%s", args, err, out)
	}
	return string(out)
}

func TestFetchRemote_Success(t *testing.T) {
	clone, _ := initBareAndClone(t)
	err := FetchRemote(clone)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFetchRemote_NotARepo(t *testing.T) {
	tmp := t.TempDir()
	err := FetchRemote(tmp)
	if err == nil {
		t.Fatal("expected error for non-repo directory")
	}
}

func TestRemoteStatus_UpToDate(t *testing.T) {
	clone, _ := initBareAndClone(t)

	status, err := RemoteStatus(clone)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !status.HasRemote {
		t.Fatal("expected HasRemote=true")
	}
	if status.Ahead != 0 {
		t.Errorf("Ahead = %d, want 0", status.Ahead)
	}
	if status.Behind != 0 {
		t.Errorf("Behind = %d, want 0", status.Behind)
	}
	if status.RemoteBranch == "" {
		t.Error("expected non-empty RemoteBranch")
	}
}

func TestRemoteStatus_AheadOnly(t *testing.T) {
	clone, _ := initBareAndClone(t)

	// Make local commits without pushing
	commitInDir(t, clone, "local1")
	commitInDir(t, clone, "local2")

	status, err := RemoteStatus(clone)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !status.HasRemote {
		t.Fatal("expected HasRemote=true")
	}
	if status.Ahead != 2 {
		t.Errorf("Ahead = %d, want 2", status.Ahead)
	}
	if status.Behind != 0 {
		t.Errorf("Behind = %d, want 0", status.Behind)
	}
}

func TestRemoteStatus_BehindOnly(t *testing.T) {
	clone, bare := initBareAndClone(t)

	// Push commits from a second clone to simulate remote changes
	base := filepath.Dir(bare)
	other := filepath.Join(base, "other")
	run(t, base, "git", "clone", bare, other)
	run(t, other, "git", "config", "user.email", "test@test.com")
	run(t, other, "git", "config", "user.name", "Test")
	commitInDir(t, other, "remote1")
	commitInDir(t, other, "remote2")
	commitInDir(t, other, "remote3")
	run(t, other, "git", "push")

	// Fetch in original clone so it knows about remote changes
	run(t, clone, "git", "fetch")

	status, err := RemoteStatus(clone)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !status.HasRemote {
		t.Fatal("expected HasRemote=true")
	}
	if status.Ahead != 0 {
		t.Errorf("Ahead = %d, want 0", status.Ahead)
	}
	if status.Behind != 3 {
		t.Errorf("Behind = %d, want 3", status.Behind)
	}
}

func TestRemoteStatus_BothAheadAndBehind(t *testing.T) {
	clone, bare := initBareAndClone(t)

	// Push commits from a second clone
	base := filepath.Dir(bare)
	other := filepath.Join(base, "other")
	run(t, base, "git", "clone", bare, other)
	run(t, other, "git", "config", "user.email", "test@test.com")
	run(t, other, "git", "config", "user.name", "Test")
	commitInDir(t, other, "remote1")
	run(t, other, "git", "push")

	// Make local commits
	commitInDir(t, clone, "local1")
	commitInDir(t, clone, "local2")

	// Fetch to know about remote changes
	run(t, clone, "git", "fetch")

	status, err := RemoteStatus(clone)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !status.HasRemote {
		t.Fatal("expected HasRemote=true")
	}
	if status.Ahead != 2 {
		t.Errorf("Ahead = %d, want 2", status.Ahead)
	}
	if status.Behind != 1 {
		t.Errorf("Behind = %d, want 1", status.Behind)
	}
}

func TestRemoteStatus_NoUpstream(t *testing.T) {
	tmp := t.TempDir()
	run(t, tmp, "git", "init")
	run(t, tmp, "git", "config", "user.email", "test@test.com")
	run(t, tmp, "git", "config", "user.name", "Test")
	run(t, tmp, "git", "commit", "--allow-empty", "-m", "init")

	status, err := RemoteStatus(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.HasRemote {
		t.Error("expected HasRemote=false for repo with no upstream")
	}
	if status.Ahead != 0 || status.Behind != 0 {
		t.Errorf("expected zero counts, got ahead=%d behind=%d", status.Ahead, status.Behind)
	}
}

func TestRemoteStatus_DetachedHEAD(t *testing.T) {
	clone, _ := initBareAndClone(t)

	// Detach HEAD
	run(t, clone, "git", "checkout", "--detach", "HEAD")

	status, err := RemoteStatus(clone)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.HasRemote {
		t.Error("expected HasRemote=false for detached HEAD")
	}
}

func TestFetchRemote_InvalidDir(t *testing.T) {
	err := FetchRemote(filepath.Join(os.TempDir(), "nonexistent-dir-gitsync-test"))
	if err == nil {
		t.Fatal("expected error for nonexistent directory")
	}
}

// writeFile creates a file with the given content in dir.
func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestWorkingTreeStatus_CleanTree(t *testing.T) {
	clone, _ := initBareAndClone(t)

	wt, err := WorkingTreeStatus(clone)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wt.HasUncommittedChanges {
		t.Error("expected HasUncommittedChanges=false")
	}
	if wt.HasUntrackedFiles {
		t.Error("expected HasUntrackedFiles=false")
	}
	if wt.TotalModified != 0 {
		t.Errorf("TotalModified = %d, want 0", wt.TotalModified)
	}
	if len(wt.ModifiedFiles) != 0 {
		t.Errorf("ModifiedFiles = %v, want empty", wt.ModifiedFiles)
	}
}

func TestWorkingTreeStatus_StagedChanges(t *testing.T) {
	clone, _ := initBareAndClone(t)

	writeFile(t, clone, "staged.txt", "hello")
	run(t, clone, "git", "add", "staged.txt")

	wt, err := WorkingTreeStatus(clone)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !wt.HasUncommittedChanges {
		t.Error("expected HasUncommittedChanges=true for staged file")
	}
	if wt.HasUntrackedFiles {
		t.Error("expected HasUntrackedFiles=false")
	}
	if wt.TotalModified != 1 {
		t.Errorf("TotalModified = %d, want 1", wt.TotalModified)
	}
}

func TestWorkingTreeStatus_UnstagedChanges(t *testing.T) {
	clone, _ := initBareAndClone(t)

	// Create a tracked file, commit it, then modify it
	writeFile(t, clone, "tracked.txt", "original")
	run(t, clone, "git", "add", "tracked.txt")
	commitInDir(t, clone, "add tracked file")
	writeFile(t, clone, "tracked.txt", "modified")

	wt, err := WorkingTreeStatus(clone)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !wt.HasUncommittedChanges {
		t.Error("expected HasUncommittedChanges=true for unstaged modification")
	}
	if wt.HasUntrackedFiles {
		t.Error("expected HasUntrackedFiles=false")
	}
	if wt.TotalModified != 1 {
		t.Errorf("TotalModified = %d, want 1", wt.TotalModified)
	}
}

func TestWorkingTreeStatus_UntrackedFiles(t *testing.T) {
	clone, _ := initBareAndClone(t)

	writeFile(t, clone, "newfile.txt", "untracked")

	wt, err := WorkingTreeStatus(clone)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wt.HasUncommittedChanges {
		t.Error("expected HasUncommittedChanges=false for untracked-only")
	}
	if !wt.HasUntrackedFiles {
		t.Error("expected HasUntrackedFiles=true")
	}
	if wt.TotalModified != 1 {
		t.Errorf("TotalModified = %d, want 1", wt.TotalModified)
	}
}

func TestWorkingTreeStatus_MixedState(t *testing.T) {
	clone, _ := initBareAndClone(t)

	// Create and commit a tracked file, then modify it (unstaged change)
	writeFile(t, clone, "tracked.txt", "original")
	run(t, clone, "git", "add", "tracked.txt")
	commitInDir(t, clone, "add tracked")
	writeFile(t, clone, "tracked.txt", "modified")

	// Staged new file (added but not committed)
	writeFile(t, clone, "staged.txt", "staged")
	run(t, clone, "git", "add", "staged.txt")

	// Untracked
	writeFile(t, clone, "untracked.txt", "new")

	wt, err := WorkingTreeStatus(clone)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !wt.HasUncommittedChanges {
		t.Error("expected HasUncommittedChanges=true")
	}
	if !wt.HasUntrackedFiles {
		t.Error("expected HasUntrackedFiles=true")
	}
	if wt.TotalModified != 3 {
		t.Errorf("TotalModified = %d, want 3", wt.TotalModified)
	}
	if len(wt.ModifiedFiles) != 3 {
		t.Errorf("len(ModifiedFiles) = %d, want 3", len(wt.ModifiedFiles))
	}
}

func TestWorkingTreeStatus_CapsAt10(t *testing.T) {
	clone, _ := initBareAndClone(t)

	// Create 15 untracked files
	for i := 0; i < 15; i++ {
		writeFile(t, clone, fmt.Sprintf("file%02d.txt", i), "content")
	}

	wt, err := WorkingTreeStatus(clone)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wt.TotalModified != 15 {
		t.Errorf("TotalModified = %d, want 15", wt.TotalModified)
	}
	if len(wt.ModifiedFiles) != 10 {
		t.Errorf("len(ModifiedFiles) = %d, want 10 (capped)", len(wt.ModifiedFiles))
	}
}

// --- Pull tests ---

func TestPull_Success(t *testing.T) {
	clone, bare := initBareAndClone(t)

	// Push a commit from another clone
	base := filepath.Dir(bare)
	other := filepath.Join(base, "other")
	run(t, base, "git", "clone", bare, other)
	run(t, other, "git", "config", "user.email", "test@test.com")
	run(t, other, "git", "config", "user.name", "Test")
	commitInDir(t, other, "remote change")
	run(t, other, "git", "push")

	// Fetch so clone knows about remote
	run(t, clone, "git", "fetch")

	err := Pull(clone)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify we are now up-to-date
	status, _ := RemoteStatus(clone)
	if status.Behind != 0 {
		t.Errorf("Behind = %d after pull, want 0", status.Behind)
	}
}

func TestPull_FailsOnConflict(t *testing.T) {
	clone, bare := initBareAndClone(t)

	// Create a tracked file and push it
	writeFile(t, clone, "conflict.txt", "original")
	run(t, clone, "git", "add", "conflict.txt")
	commitInDir(t, clone, "add conflict.txt")
	run(t, clone, "git", "push")

	// Modify from another clone and push
	base := filepath.Dir(bare)
	other := filepath.Join(base, "other")
	run(t, base, "git", "clone", bare, other)
	run(t, other, "git", "config", "user.email", "test@test.com")
	run(t, other, "git", "config", "user.name", "Test")
	writeFile(t, other, "conflict.txt", "remote version")
	run(t, other, "git", "add", "conflict.txt")
	commitInDir(t, other, "modify conflict.txt remotely")
	run(t, other, "git", "push")

	// Modify locally (divergent change)
	writeFile(t, clone, "conflict.txt", "local version")
	run(t, clone, "git", "add", "conflict.txt")
	commitInDir(t, clone, "modify conflict.txt locally")

	run(t, clone, "git", "fetch")

	err := Pull(clone)
	if err == nil {
		t.Fatal("expected error for conflicting pull")
	}
}

func TestPullRebase_Success(t *testing.T) {
	clone, bare := initBareAndClone(t)

	// Push from another clone
	base := filepath.Dir(bare)
	other := filepath.Join(base, "other")
	run(t, base, "git", "clone", bare, other)
	run(t, other, "git", "config", "user.email", "test@test.com")
	run(t, other, "git", "config", "user.name", "Test")
	commitInDir(t, other, "remote change")
	run(t, other, "git", "push")

	// Make a non-conflicting local commit
	writeFile(t, clone, "local.txt", "local content")
	run(t, clone, "git", "add", "local.txt")
	commitInDir(t, clone, "local non-conflicting")

	run(t, clone, "git", "fetch")

	err := PullRebase(clone)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify we are now up-to-date (ahead due to our rebased commit)
	status, _ := RemoteStatus(clone)
	if status.Behind != 0 {
		t.Errorf("Behind = %d after pull --rebase, want 0", status.Behind)
	}
}

func TestPullRebase_FailsOnConflict(t *testing.T) {
	clone, bare := initBareAndClone(t)

	writeFile(t, clone, "conflict.txt", "original")
	run(t, clone, "git", "add", "conflict.txt")
	commitInDir(t, clone, "add conflict.txt")
	run(t, clone, "git", "push")

	base := filepath.Dir(bare)
	other := filepath.Join(base, "other")
	run(t, base, "git", "clone", bare, other)
	run(t, other, "git", "config", "user.email", "test@test.com")
	run(t, other, "git", "config", "user.name", "Test")
	writeFile(t, other, "conflict.txt", "remote version")
	run(t, other, "git", "add", "conflict.txt")
	commitInDir(t, other, "modify remotely")
	run(t, other, "git", "push")

	writeFile(t, clone, "conflict.txt", "local version")
	run(t, clone, "git", "add", "conflict.txt")
	commitInDir(t, clone, "modify locally")

	run(t, clone, "git", "fetch")

	err := PullRebase(clone)
	if err == nil {
		t.Fatal("expected error for conflicting rebase")
	}
}

func TestForcePull_Success(t *testing.T) {
	clone, bare := initBareAndClone(t)

	// Push from another clone
	base := filepath.Dir(bare)
	other := filepath.Join(base, "other")
	run(t, base, "git", "clone", bare, other)
	run(t, other, "git", "config", "user.email", "test@test.com")
	run(t, other, "git", "config", "user.name", "Test")
	commitInDir(t, other, "remote change")
	run(t, other, "git", "push")

	// Make a local divergent commit
	commitInDir(t, clone, "local change to be discarded")

	err := ForcePull(clone, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should be in sync now (local commit discarded)
	status, _ := RemoteStatus(clone)
	if status.Ahead != 0 {
		t.Errorf("Ahead = %d after force pull, want 0", status.Ahead)
	}
	if status.Behind != 0 {
		t.Errorf("Behind = %d after force pull, want 0", status.Behind)
	}
}

func TestForcePull_RefusesWithoutConfirmation(t *testing.T) {
	clone, _ := initBareAndClone(t)

	err := ForcePull(clone, false)
	if err == nil {
		t.Fatal("expected error when confirm=false")
	}
	if !errors.Is(err, ErrForcePullNotConfirmed) {
		t.Errorf("expected ErrForcePullNotConfirmed, got: %v", err)
	}
}

func TestForcePull_DiscardsLocalCommits(t *testing.T) {
	clone, bare := initBareAndClone(t)

	// Push from another clone
	base := filepath.Dir(bare)
	other := filepath.Join(base, "other")
	run(t, base, "git", "clone", bare, other)
	run(t, other, "git", "config", "user.email", "test@test.com")
	run(t, other, "git", "config", "user.name", "Test")
	writeFile(t, other, "remote.txt", "from remote")
	run(t, other, "git", "add", "remote.txt")
	commitInDir(t, other, "remote commit")
	run(t, other, "git", "push")

	// Create divergent local commit
	writeFile(t, clone, "local.txt", "local only")
	run(t, clone, "git", "add", "local.txt")
	commitInDir(t, clone, "local commit to be discarded")

	err := ForcePull(clone, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Remote file should be present
	if _, err := os.Stat(filepath.Join(clone, "remote.txt")); os.IsNotExist(err) {
		t.Error("expected remote.txt to exist after force pull")
	}

	// Local commit's file should be gone (reset discards committed changes)
	if _, err := os.Stat(filepath.Join(clone, "local.txt")); !os.IsNotExist(err) {
		t.Error("expected local.txt to be removed by force pull")
	}
}

func TestStashAndPull_Success(t *testing.T) {
	clone, bare := initBareAndClone(t)

	// Create a tracked file
	writeFile(t, clone, "file.txt", "original")
	run(t, clone, "git", "add", "file.txt")
	commitInDir(t, clone, "add file")
	run(t, clone, "git", "push")

	// Push from another clone (non-conflicting remote change)
	base := filepath.Dir(bare)
	other := filepath.Join(base, "other")
	run(t, base, "git", "clone", bare, other)
	run(t, other, "git", "config", "user.email", "test@test.com")
	run(t, other, "git", "config", "user.name", "Test")
	writeFile(t, other, "remote.txt", "remote content")
	run(t, other, "git", "add", "remote.txt")
	commitInDir(t, other, "add remote.txt")
	run(t, other, "git", "push")

	// Make local uncommitted changes
	writeFile(t, clone, "file.txt", "local modification")

	run(t, clone, "git", "fetch")

	err := StashAndPull(clone)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Local changes should be restored
	content, _ := os.ReadFile(filepath.Join(clone, "file.txt"))
	if string(content) != "local modification" {
		t.Errorf("expected local changes restored, got: %s", string(content))
	}

	// Remote changes should be present
	if _, err := os.Stat(filepath.Join(clone, "remote.txt")); os.IsNotExist(err) {
		t.Error("expected remote.txt to exist after pull")
	}
}

func TestStashAndPull_ConflictReturnsSpecificError(t *testing.T) {
	clone, bare := initBareAndClone(t)

	// Create a tracked file
	writeFile(t, clone, "file.txt", "original")
	run(t, clone, "git", "add", "file.txt")
	commitInDir(t, clone, "add file")
	run(t, clone, "git", "push")

	// Modify from another clone and push
	base := filepath.Dir(bare)
	other := filepath.Join(base, "other")
	run(t, base, "git", "clone", bare, other)
	run(t, other, "git", "config", "user.email", "test@test.com")
	run(t, other, "git", "config", "user.name", "Test")
	writeFile(t, other, "file.txt", "remote version of file")
	run(t, other, "git", "add", "file.txt")
	commitInDir(t, other, "modify file remotely")
	run(t, other, "git", "push")

	// Make conflicting local uncommitted changes
	writeFile(t, clone, "file.txt", "local conflicting version")

	run(t, clone, "git", "fetch")

	err := StashAndPull(clone)
	if err == nil {
		t.Fatal("expected error for conflicting stash pop")
	}

	var conflictErr *ErrStashPopConflict
	if !errors.As(err, &conflictErr) {
		t.Errorf("expected *ErrStashPopConflict, got: %T: %v", err, err)
	}
}

func TestStashAndPull_PullFailsRestoresStash(t *testing.T) {
	// Create a repo with no remote to make pull fail
	tmp := t.TempDir()
	run(t, tmp, "git", "init")
	run(t, tmp, "git", "config", "user.email", "test@test.com")
	run(t, tmp, "git", "config", "user.name", "Test")
	writeFile(t, tmp, "file.txt", "content")
	run(t, tmp, "git", "add", "file.txt")
	run(t, tmp, "git", "commit", "-m", "init")

	// Make a local change to stash
	writeFile(t, tmp, "file.txt", "modified")

	err := StashAndPull(tmp)
	if err == nil {
		t.Fatal("expected error when pull fails (no remote)")
	}

	// Local changes should still be present (stash was restored)
	content, _ := os.ReadFile(filepath.Join(tmp, "file.txt"))
	if string(content) != "modified" {
		t.Errorf("expected stash to be restored, got: %s", string(content))
	}
}
