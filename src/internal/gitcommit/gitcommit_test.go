package gitcommit

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestStripCoAuthoredBy(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "removes single co-authored-by line",
			input: "Fix bug in parser\n\nCo-Authored-By: Claude <noreply@anthropic.com>\n",
			want:  "Fix bug in parser\n",
		},
		{
			name:  "removes multiple co-authored-by lines",
			input: "Add feature\n\nCo-Authored-By: Alice <a@b.com>\nCo-Authored-By: Bob <b@c.com>\n",
			want:  "Add feature\n",
		},
		{
			name:  "case insensitive",
			input: "Fix thing\n\nco-authored-by: Claude <noreply@anthropic.com>\n",
			want:  "Fix thing\n",
		},
		{
			name:  "no co-authored-by lines",
			input: "Simple commit message\n",
			want:  "Simple commit message\n",
		},
		{
			name:  "preserves other content",
			input: "TASK-004: Git commit\n\nDetails here.\n\nCo-Authored-By: Claude <noreply@anthropic.com>\n",
			want:  "TASK-004: Git commit\n\nDetails here.\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StripCoAuthoredBy(tt.input)
			if got != tt.want {
				t.Errorf("StripCoAuthoredBy() = %q, want %q", got, tt.want)
			}
		})
	}
}

// initGitRepo creates a temporary git repo for testing.
func initGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
	}

	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git setup %v failed: %s", args, out)
		}
	}

	// Create an initial commit so we have a HEAD
	initial := filepath.Join(dir, "README.md")
	os.WriteFile(initial, []byte("# Test\n"), 0644)
	run(t, dir, "git", "add", ".")
	run(t, dir, "git", "commit", "-m", "initial commit")

	return dir
}

func run(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v failed: %s", args, out)
	}
}

func TestCommitIteration_NoCOMMITFile(t *testing.T) {
	dir := initGitRepo(t)

	result, err := CommitIteration(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Committed {
		t.Error("expected Committed=false when COMMIT.md doesn't exist")
	}
	if !strings.Contains(result.Message, "Warning") {
		t.Error("expected warning message")
	}
}

func TestCommitIteration_Success(t *testing.T) {
	dir := initGitRepo(t)

	// Create a staged file
	testFile := filepath.Join(dir, "test.txt")
	os.WriteFile(testFile, []byte("hello\n"), 0644)
	run(t, dir, "git", "add", "test.txt")

	// Create COMMIT.md with Co-Authored-By
	commitMsg := "Add test file\n\nCo-Authored-By: Claude <noreply@anthropic.com>\n"
	os.WriteFile(filepath.Join(dir, commitFile), []byte(commitMsg), 0644)

	result, err := CommitIteration(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Committed {
		t.Error("expected Committed=true")
	}

	// Verify COMMIT.md was deleted
	if _, err := os.Stat(filepath.Join(dir, commitFile)); !os.IsNotExist(err) {
		t.Error("expected COMMIT.md to be deleted after successful commit")
	}

	// Verify commit message doesn't contain Co-Authored-By
	cmd := exec.Command("git", "log", "-1", "--format=%B")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git log failed: %v", err)
	}
	if strings.Contains(string(out), "Co-Authored-By") {
		t.Error("commit message should not contain Co-Authored-By")
	}
	if !strings.Contains(string(out), "Add test file") {
		t.Error("commit message should contain the original message")
	}
}

func TestCommitIteration_UnstagesMaggusRuns(t *testing.T) {
	dir := initGitRepo(t)

	// Create .maggus/runs/ files and stage them
	runsDir := filepath.Join(dir, ".maggus", "runs", "20260312")
	os.MkdirAll(runsDir, 0755)
	os.WriteFile(filepath.Join(runsDir, "iteration-01.md"), []byte("log\n"), 0644)
	run(t, dir, "git", "add", ".maggus/runs/")

	// Create .maggus/MEMORY.md and stage it
	os.WriteFile(filepath.Join(dir, ".maggus", "MEMORY.md"), []byte("memory\n"), 0644)
	run(t, dir, "git", "add", ".maggus/MEMORY.md")

	// Create a real change to commit
	os.WriteFile(filepath.Join(dir, "feature.go"), []byte("package main\n"), 0644)
	run(t, dir, "git", "add", "feature.go")

	// Create COMMIT.md
	os.WriteFile(filepath.Join(dir, commitFile), []byte("feat: add feature\n"), 0644)

	result, err := CommitIteration(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Committed {
		t.Error("expected Committed=true")
	}

	// Verify .maggus/runs/ files were NOT committed
	cmd := exec.Command("git", "show", "--name-only", "--format=", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git show failed: %v", err)
	}
	committed := string(out)
	if strings.Contains(committed, ".maggus/runs/") {
		t.Error(".maggus/runs/ files should not be in the commit")
	}
	if strings.Contains(committed, ".maggus/MEMORY.md") {
		t.Error(".maggus/MEMORY.md should not be in the commit")
	}
	if !strings.Contains(committed, "feature.go") {
		t.Error("feature.go should be in the commit")
	}
}

func TestCommitIteration_NothingStaged(t *testing.T) {
	dir := initGitRepo(t)

	// Create COMMIT.md but don't stage anything
	os.WriteFile(filepath.Join(dir, commitFile), []byte("Empty commit\n"), 0644)
	run(t, dir, "git", "add", commitFile)

	// Unstage it so nothing is staged
	run(t, dir, "git", "reset", "HEAD", commitFile)

	_, err := CommitIteration(dir)
	if err == nil {
		t.Error("expected error when nothing is staged")
	}
	if !strings.Contains(err.Error(), "git commit failed") {
		t.Errorf("expected git commit failure, got: %v", err)
	}
}
