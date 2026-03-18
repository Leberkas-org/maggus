package gitsync

import (
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
