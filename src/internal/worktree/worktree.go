package worktree

import (
	"fmt"
	"os/exec"
	"strings"
)

// Create creates a new git worktree at worktreeDir on a new branch.
// It runs "git worktree add <worktreeDir> -b <branch>" from repoDir.
func Create(repoDir, worktreeDir, branch string) error {
	cmd := exec.Command("git", "worktree", "add", worktreeDir, "-b", branch)
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git worktree add %s -b %s: %w: %s", worktreeDir, branch, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// Remove removes an existing git worktree at worktreeDir.
// It runs "git worktree remove <worktreeDir> --force" from repoDir.
func Remove(repoDir, worktreeDir string) error {
	cmd := exec.Command("git", "worktree", "remove", worktreeDir, "--force")
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git worktree remove %s: %w: %s", worktreeDir, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// List returns the paths of all worktrees for the repository at repoDir.
// It runs "git worktree list --porcelain" and parses the output.
func List(repoDir string) ([]string, error) {
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git worktree list: %w", err)
	}

	var paths []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if path, ok := strings.CutPrefix(line, "worktree "); ok {
			paths = append(paths, path)
		}
	}
	return paths, nil
}
