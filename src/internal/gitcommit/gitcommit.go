package gitcommit

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

const commitFile = "COMMIT.md"

var coAuthoredByRe = regexp.MustCompile(`(?mi)^Co-Authored-By:.*$\n?`)

// StripCoAuthoredBy removes all Co-Authored-By lines from the given text.
func StripCoAuthoredBy(text string) string {
	result := coAuthoredByRe.ReplaceAllString(text, "")
	return strings.TrimRight(result, "\n") + "\n"
}

// Result represents the outcome of a commit attempt.
type Result struct {
	Committed bool
	Message   string
}

// CommitIteration checks for COMMIT.md in workDir, strips Co-Authored-By lines,
// runs git commit -F COMMIT.md, and deletes COMMIT.md on success.
// Returns a Result indicating what happened.
func CommitIteration(workDir string) (Result, error) {
	commitPath := filepath.Join(workDir, commitFile)

	data, err := os.ReadFile(commitPath)
	if os.IsNotExist(err) {
		return Result{
			Committed: false,
			Message:   "Warning: COMMIT.md not found, agent may not have made changes",
		}, nil
	}
	if err != nil {
		return Result{}, fmt.Errorf("read COMMIT.md: %w", err)
	}

	// Strip Co-Authored-By lines and write back
	cleaned := StripCoAuthoredBy(string(data))
	if err := os.WriteFile(commitPath, []byte(cleaned), 0644); err != nil {
		return Result{}, fmt.Errorf("write cleaned COMMIT.md: %w", err)
	}

	// Safety gate: ensure internal files are never included in the commit.
	// The agent may have staged them via `git add .` or `git add -A`.
	for _, pattern := range []string{commitFile, ".maggus/runs/", ".maggus/MEMORY.md", ".maggus/RELEASE_NOTES.md"} {
		unstage := exec.Command("git", "reset", "HEAD", "--", pattern)
		unstage.Dir = workDir
		unstage.CombinedOutput() // ignore errors (files may not be staged)
	}

	// Run git commit
	cmd := exec.Command("git", "commit", "-F", commitFile)
	cmd.Dir = workDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return Result{}, fmt.Errorf("git commit failed: %s", strings.TrimSpace(string(out)))
	}

	// Delete COMMIT.md after successful commit
	if err := os.Remove(commitPath); err != nil {
		return Result{}, fmt.Errorf("remove COMMIT.md: %w", err)
	}

	return Result{
		Committed: true,
		Message:   strings.TrimSpace(string(out)),
	}, nil
}
