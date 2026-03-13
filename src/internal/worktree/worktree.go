package worktree

import (
	"fmt"
	"os/exec"
	"strings"
)

// Info holds detailed information about a git worktree.
type Info struct {
	Path   string
	Branch string
}

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

// ListDetailed returns detailed info (path + branch) for all worktrees.
func ListDetailed(repoDir string) ([]Info, error) {
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git worktree list: %w", err)
	}

	var result []Info
	var current Info
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if path, ok := strings.CutPrefix(line, "worktree "); ok {
			current = Info{Path: path}
		} else if branch, ok := strings.CutPrefix(line, "branch "); ok {
			current.Branch = branch
		} else if line == "" && current.Path != "" {
			result = append(result, current)
			current = Info{}
		}
	}
	// Handle last entry if no trailing newline.
	if current.Path != "" {
		result = append(result, current)
	}
	return result, nil
}

// Prune runs "git worktree prune" to clean up stale worktree references.
func Prune(repoDir string) error {
	cmd := exec.Command("git", "worktree", "prune")
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git worktree prune: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// DeleteBranch deletes a local git branch.
func DeleteBranch(repoDir, branch string) error {
	cmd := exec.Command("git", "branch", "-D", branch)
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git branch -D %s: %w: %s", branch, err, strings.TrimSpace(string(out)))
	}
	return nil
}
