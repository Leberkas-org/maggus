package runtracker

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestNew_CreatesRunDirectory(t *testing.T) {
	tmp := t.TempDir()
	initGitRepo(t, tmp)

	run, err := New(tmp, "claude", 3)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	// Verify RUN_ID format (YYYYMMDD-HHMMSS)
	if len(run.ID) != 15 || run.ID[8] != '-' {
		t.Errorf("unexpected RUN_ID format: %q", run.ID)
	}

	// Verify directory was created
	info, err := os.Stat(run.Dir)
	if err != nil {
		t.Fatalf("run dir not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("run dir is not a directory")
	}

	// Verify run.md exists with expected content
	runMd, err := os.ReadFile(filepath.Join(run.Dir, "run.md"))
	if err != nil {
		t.Fatalf("run.md not created: %v", err)
	}

	content := string(runMd)
	for _, want := range []string{
		"# Run Log",
		"**RUN_ID:**",
		"**Branch:**",
		"**Model:** claude",
		"**Iterations:** 3",
		"**Start Commit:**",
		"**Start Time:**",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("run.md missing %q", want)
		}
	}
}

func TestIterationLogPath(t *testing.T) {
	run := &Run{
		ID:  "20260309-120000",
		Dir: filepath.Join("some", "dir", "20260309-120000"),
	}

	tests := []struct {
		iter int
		want string
	}{
		{1, filepath.Join("some", "dir", "20260309-120000", "iteration-01.md")},
		{5, filepath.Join("some", "dir", "20260309-120000", "iteration-05.md")},
		{12, filepath.Join("some", "dir", "20260309-120000", "iteration-12.md")},
	}

	for _, tt := range tests {
		got := run.IterationLogPath(tt.iter)
		if got != tt.want {
			t.Errorf("IterationLogPath(%d) = %q, want %q", tt.iter, got, tt.want)
		}
	}
}

func TestFinalize_AppendsEndMetadata(t *testing.T) {
	tmp := t.TempDir()
	initGitRepo(t, tmp)

	run, err := New(tmp, "claude", 1)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	if err := run.Finalize(tmp); err != nil {
		t.Fatalf("Finalize() error: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(run.Dir, "run.md"))
	if err != nil {
		t.Fatalf("read run.md: %v", err)
	}

	text := string(content)
	for _, want := range []string{
		"## End",
		"**End Time:**",
		"**End Commit:**",
		"**Commit Range:**",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("finalized run.md missing %q", want)
		}
	}
}

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
