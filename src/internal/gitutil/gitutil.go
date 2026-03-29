package gitutil

import (
	"os/exec"
	"strings"
)

// Command creates an *exec.Cmd for a git subprocess with the given arguments.
// On Windows it applies CREATE_NO_WINDOW to suppress console window flicker.
func Command(args ...string) *exec.Cmd {
	cmd := exec.Command("git", args...)
	setProcAttr(cmd)
	return cmd
}

// RepoURL returns the git remote origin URL for the repository in dir.
// Returns trimmed output of `git config --get remote.origin.url`, or "" on error.
func RepoURL(dir string) string {
	cmd := Command("config", "--get", "remote.origin.url")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
