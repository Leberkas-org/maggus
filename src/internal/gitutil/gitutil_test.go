package gitutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestRepoURL_WithRemote(t *testing.T) {
	dir := t.TempDir()

	// Initialize a bare git repo so we can set a remote
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init")
	run("config", "user.email", "test@test.com")
	run("config", "user.name", "Test")
	run("remote", "add", "origin", "https://github.com/example/repo.git")

	url := RepoURL(dir)
	if url != "https://github.com/example/repo.git" {
		t.Errorf("got %q, want %q", url, "https://github.com/example/repo.git")
	}
}

func TestRepoURL_NoRemote(t *testing.T) {
	dir := t.TempDir()

	cmd := exec.Command("git", "init")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}

	url := RepoURL(dir)
	if url != "" {
		t.Errorf("expected empty string, got %q", url)
	}
}

func TestRepoURL_NotARepo(t *testing.T) {
	dir := t.TempDir()
	// No git init — not a repo
	_ = os.WriteFile(filepath.Join(dir, "file.txt"), []byte("hello"), 0o644)

	url := RepoURL(dir)
	if url != "" {
		t.Errorf("expected empty string for non-repo dir, got %q", url)
	}
}
