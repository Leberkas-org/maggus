package cmd

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leberkas-org/maggus/internal/agent"
)

func TestReleaseCommandRegistered(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "release" {
			found = true
			break
		}
	}
	if !found {
		t.Error("release command not registered on rootCmd")
	}
}

func TestReleaseCommandHasModelFlag(t *testing.T) {
	flag := releaseCmd.Flags().Lookup("model")
	if flag == nil {
		t.Error("release command should have --model flag")
	}
}

func TestBuildReleaseMd(t *testing.T) {
	summary := "- Added new feature X\n- Fixed bug Y"
	changelog := "## v1.0.0\n\n### Features\n\n- feat: add X\n"

	result := buildReleaseMd(summary, changelog)

	if !strings.Contains(result, "# Release Notes") {
		t.Error("expected '# Release Notes' header")
	}
	if !strings.Contains(result, "## Summary") {
		t.Error("expected '## Summary' header")
	}
	if !strings.Contains(result, "## Changelog") {
		t.Error("expected '## Changelog' header")
	}
	if !strings.Contains(result, summary) {
		t.Error("expected summary content in output")
	}
	if !strings.Contains(result, changelog) {
		t.Error("expected changelog content in output")
	}
}

func TestBuildReleasePrompt(t *testing.T) {
	changelog := "### Features\n- feat: something\n"
	notes := "## TASK-001: Title\n- Did something"
	diffStat := " file.go | 10 ++++\n 1 file changed"

	prompt := buildReleasePrompt(changelog, notes, diffStat)

	if !strings.Contains(prompt, "Conventional Changelog") {
		t.Error("expected changelog section in prompt")
	}
	if !strings.Contains(prompt, changelog) {
		t.Error("expected changelog content in prompt")
	}
	if !strings.Contains(prompt, "Rough Release Notes") {
		t.Error("expected release notes section in prompt")
	}
	if !strings.Contains(prompt, notes) {
		t.Error("expected notes content in prompt")
	}
	if !strings.Contains(prompt, "Diff Summary") {
		t.Error("expected diff stat section in prompt")
	}
}

func TestBuildReleasePromptNoNotes(t *testing.T) {
	changelog := "### Features\n- feat: something\n"
	prompt := buildReleasePrompt(changelog, "", "")

	if strings.Contains(prompt, "Rough Release Notes") {
		t.Error("should not include release notes section when notes are empty")
	}
	if strings.Contains(prompt, "Diff Summary") {
		t.Error("should not include diff stat section when stat is empty")
	}
}

func TestRunReleaseNoCommits(t *testing.T) {
	// Create a temporary git repo with a tag at HEAD (no commits since tag)
	dir := t.TempDir()
	gitInit(t, dir)
	gitCommit(t, dir, "initial commit")
	gitTag(t, dir, "v0.1.0")

	var buf bytes.Buffer
	cmd := *releaseCmd
	cmd.SetOut(&buf)

	err := runRelease(&cmd, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(buf.String(), "No changes since last tag.") {
		t.Errorf("expected 'No changes since last tag.' message, got:\n%s", buf.String())
	}
}

func TestRunReleaseWritesFile(t *testing.T) {
	dir := t.TempDir()
	gitInit(t, dir)
	gitCommit(t, dir, "initial commit")
	gitTag(t, dir, "v0.1.0")
	gitCommit(t, dir, "feat: add new feature")

	// Write rough release notes
	maggusDir := filepath.Join(dir, ".maggus")
	os.MkdirAll(maggusDir, 0o755)
	os.WriteFile(filepath.Join(maggusDir, "RELEASE_NOTES.md"), []byte("## TASK-001\n- Added feature\n"), 0o644)

	// Mock the Claude invocation
	origRunner := runAgentOnce
	runAgentOnce = func(ctx context.Context, a agent.Agent, prompt string, model string) (string, error) {
		return "- Added a great new feature for users", nil
	}
	defer func() { runAgentOnce = origRunner }()

	var buf bytes.Buffer
	cmd := *releaseCmd
	cmd.SetOut(&buf)

	err := runRelease(&cmd, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Wrote") {
		t.Errorf("expected 'Wrote' message, got:\n%s", output)
	}
	if !strings.Contains(output, "## Summary") {
		t.Errorf("expected summary preview in output, got:\n%s", output)
	}

	// Verify RELEASE.md was written
	data, err := os.ReadFile(filepath.Join(dir, "RELEASE.md"))
	if err != nil {
		t.Fatalf("RELEASE.md should exist: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "# Release Notes") {
		t.Error("RELEASE.md should contain '# Release Notes'")
	}
	if !strings.Contains(content, "## Summary") {
		t.Error("RELEASE.md should contain '## Summary'")
	}
	if !strings.Contains(content, "## Changelog") {
		t.Error("RELEASE.md should contain '## Changelog'")
	}
	if !strings.Contains(content, "Added a great new feature") {
		t.Error("RELEASE.md should contain the AI summary")
	}

	// Verify RELEASE_NOTES.md was cleared
	if _, err := os.Stat(filepath.Join(maggusDir, "RELEASE_NOTES.md")); !os.IsNotExist(err) {
		t.Error("RELEASE_NOTES.md should be deleted after release")
	}
	if !strings.Contains(output, "Cleared .maggus/RELEASE_NOTES.md for next release cycle.") {
		t.Errorf("expected cleared message, got:\n%s", output)
	}
}

func TestRunReleaseNoReleaseNotes(t *testing.T) {
	dir := t.TempDir()
	gitInit(t, dir)
	gitCommit(t, dir, "initial commit")
	gitTag(t, dir, "v0.1.0")
	gitCommit(t, dir, "feat: add something")

	// No .maggus/RELEASE_NOTES.md — should not error

	origRunner := runAgentOnce
	runAgentOnce = func(ctx context.Context, a agent.Agent, prompt string, model string) (string, error) {
		return "- Summary", nil
	}
	defer func() { runAgentOnce = origRunner }()

	var buf bytes.Buffer
	cmd := *releaseCmd
	cmd.SetOut(&buf)

	err := runRelease(&cmd, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if strings.Contains(output, "Cleared .maggus/RELEASE_NOTES.md") {
		t.Error("should not print cleared message when file didn't exist")
	}
}

func TestRunReleaseOutputFormat(t *testing.T) {
	// Verify the structure of the generated RELEASE.md
	summary := "- Feature A added\n- Bug B fixed"
	changelog := "## v1.0.0\n\n### Features\n\n- feat: A\n\n### Bug Fixes\n\n- fix: B\n"

	md := buildReleaseMd(summary, changelog)

	// Check ordering: Summary before Changelog
	summaryIdx := strings.Index(md, "## Summary")
	changelogIdx := strings.Index(md, "## Changelog")
	if summaryIdx >= changelogIdx {
		t.Error("Summary section should appear before Changelog section")
	}
}

// git helpers for tests
func gitInit(t *testing.T, dir string) {
	t.Helper()
	run(t, dir, "git", "init")
	run(t, dir, "git", "config", "user.email", "test@test.com")
	run(t, dir, "git", "config", "user.name", "Test")
}

func gitCommit(t *testing.T, dir string, msg string) {
	t.Helper()
	// Create or modify a file to have something to commit
	f := filepath.Join(dir, "file.txt")
	content := msg + "\n"
	if data, err := os.ReadFile(f); err == nil {
		content = string(data) + content
	}
	os.WriteFile(f, []byte(content), 0o644)
	run(t, dir, "git", "add", "-A")
	run(t, dir, "git", "commit", "-m", msg)
}

func gitTag(t *testing.T, dir string, tag string) {
	t.Helper()
	run(t, dir, "git", "tag", tag)
}

func run(t *testing.T, dir string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v failed: %v\n%s", name, args, err, out)
	}
}
