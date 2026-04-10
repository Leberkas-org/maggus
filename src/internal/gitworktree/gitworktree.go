package gitworktree

import (
	"bufio"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/leberkas-org/maggus/internal/gitutil"
)

// WorktreeInfo describes a git worktree entry.
type WorktreeInfo struct {
	Path   string
	Branch string
	HEAD   string
}

// CreateWorktree adds a worktree at path on branch.
// If branch does not exist it is created off HEAD.
// If path is already a registered worktree on the expected branch, the call is
// a no-op (idempotent success). If it is registered on a different branch, a
// descriptive error is returned.
func CreateWorktree(repoRoot, path, branch string) error {
	// Check for an already-registered worktree at path to make this call idempotent.
	existing, err := ListWorktrees(repoRoot)
	if err != nil {
		return fmt.Errorf("CreateWorktree: listing worktrees: %w", err)
	}
	cleanPath := filepath.Clean(path)
	for _, wt := range existing {
		if filepath.Clean(wt.Path) != cleanPath {
			continue
		}
		// Path is already registered.
		if wt.Branch == branch {
			return nil // idempotent success
		}
		return fmt.Errorf("worktree at %q is already registered on branch %q, not %q", path, wt.Branch, branch)
	}

	var args []string
	if branchExists(repoRoot, branch) {
		args = []string{"worktree", "add", path, branch}
	} else {
		args = []string{"worktree", "add", "-b", branch, path}
	}
	cmd := gitutil.Command(args...)
	cmd.Dir = repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git worktree add: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// RemoveWorktree removes the worktree at path unconditionally.
func RemoveWorktree(repoRoot, path string) error {
	cmd := gitutil.Command("worktree", "remove", "--force", path)
	cmd.Dir = repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git worktree remove: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// ListWorktrees returns all worktrees in the repository.
// Stale entries are pruned before listing.
func ListWorktrees(repoRoot string) ([]WorktreeInfo, error) {
	pruneCmd := gitutil.Command("worktree", "prune")
	pruneCmd.Dir = repoRoot
	if out, err := pruneCmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("git worktree prune: %w: %s", err, strings.TrimSpace(string(out)))
	}

	listCmd := gitutil.Command("worktree", "list", "--porcelain")
	listCmd.Dir = repoRoot
	out, err := listCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git worktree list: %w", err)
	}

	return parsePorcelain(string(out)), nil
}

func parsePorcelain(output string) []WorktreeInfo {
	var results []WorktreeInfo
	var current WorktreeInfo

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if current.Path != "" {
				results = append(results, current)
				current = WorktreeInfo{}
			}
			continue
		}
		if after, ok := strings.CutPrefix(line, "worktree "); ok {
			current.Path = after
		} else if after, ok := strings.CutPrefix(line, "HEAD "); ok {
			current.HEAD = after
		} else if after, ok := strings.CutPrefix(line, "branch "); ok {
			current.Branch = strings.TrimPrefix(after, "refs/heads/")
		}
	}
	// flush last block if there is no trailing blank line
	if current.Path != "" {
		results = append(results, current)
	}
	return results
}

func branchExists(dir, branch string) bool {
	cmd := gitutil.Command("rev-parse", "--verify", "--quiet", branch)
	cmd.Dir = dir
	return cmd.Run() == nil
}
