package gitsync

import (
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
