package gitrecover

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/leberkas-org/maggus/internal/gitbranch"
	"github.com/leberkas-org/maggus/internal/gitmerge"
	"github.com/leberkas-org/maggus/internal/gitutil"
	"github.com/leberkas-org/maggus/internal/gitworktree"
	"github.com/leberkas-org/maggus/internal/runlog"
)

// cleanOrphanedWorktrees removes leftover worktrees from .maggus/worktrees/ and
// safely deletes their branches when fully merged. Worktrees whose task IDs have
// status "working" in the worker index are skipped to avoid interrupting active runs.
//
// For each orphaned worktree:
//   - The worktree is removed via gitworktree.RemoveWorktree.
//   - If the worktree's branch is a task branch and is an ancestor of HEAD, the branch
//     is deleted via gitbranch.DeleteBranch.
//   - If the branch has divergent (unmerged) work, only the worktree is removed; the
//     branch is preserved and a warning is included in the returned log messages.
//
// After cleanup the .maggus/worktrees/ directory is removed if it is empty.
// Returns log messages for every action taken and any warnings encountered.
func cleanOrphanedWorktrees(repoDir string) ([]string, error) {
	var logs []string

	worktrees, err := gitworktree.ListWorktrees(repoDir)
	if err != nil {
		return nil, fmt.Errorf("cleanOrphanedWorktrees: list worktrees: %w", err)
	}

	maggusWtDir := filepath.Join(repoDir, ".maggus", "worktrees")

	// Build a set of task IDs that currently have an active (working) worker.
	workers := runlog.ReadWorkersIndex(repoDir)
	activeTaskIDs := make(map[string]bool, len(workers))
	for _, w := range workers {
		if w.Status == "working" {
			activeTaskIDs[w.TaskID] = true
		}
	}

	for _, wt := range worktrees {
		// Only process worktrees under .maggus/worktrees/.
		if !isUnderDir(wt.Path, maggusWtDir) {
			continue
		}

		// Skip worktrees that have an active worker.
		if wt.Branch != "" {
			taskID := gitmerge.TaskIDFromBranch(wt.Branch)
			if taskID != "" && activeTaskIDs[taskID] {
				logs = append(logs, fmt.Sprintf("skipping active worktree %s (worker for task %s is running)", wt.Path, taskID))
				continue
			}
		}

		// Remove the orphaned worktree.
		if err := gitworktree.RemoveWorktree(repoDir, wt.Path); err != nil {
			logs = append(logs, fmt.Sprintf("warning: failed to remove worktree %s: %v", wt.Path, err))
			continue
		}
		logs = append(logs, fmt.Sprintf("removed orphaned worktree %s", wt.Path))

		// Decide what to do with the branch.
		if wt.Branch == "" || !gitbranch.IsTaskBranch(wt.Branch) {
			continue
		}

		if isBranchAncestorOfHEAD(repoDir, wt.Branch) {
			if err := gitbranch.DeleteBranch(repoDir, wt.Branch); err != nil {
				logs = append(logs, fmt.Sprintf("warning: failed to delete merged branch %s: %v", wt.Branch, err))
			} else {
				logs = append(logs, fmt.Sprintf("deleted merged branch %s", wt.Branch))
			}
		} else {
			logs = append(logs, fmt.Sprintf("warning: branch %s has divergent work; branch preserved", wt.Branch))
		}
	}

	// Remove .maggus/worktrees/ if it is now empty.
	if err := removeIfEmpty(maggusWtDir); err != nil {
		logs = append(logs, fmt.Sprintf("warning: could not remove %s: %v", maggusWtDir, err))
	}

	return logs, nil
}

// isUnderDir reports whether path is a direct or indirect descendant of dir
// (not dir itself). Both paths are cleaned before comparison.
func isUnderDir(path, dir string) bool {
	rel, err := filepath.Rel(filepath.Clean(dir), filepath.Clean(path))
	if err != nil {
		return false
	}
	// "." means path == dir; ".." prefix means path is above dir.
	return rel != "." && !strings.HasPrefix(rel, "..")
}

// isBranchAncestorOfHEAD returns true if the tip of branch is reachable from
// HEAD (i.e. the branch was already merged into the current branch).
func isBranchAncestorOfHEAD(repoDir, branch string) bool {
	cmd := gitutil.Command("merge-base", "--is-ancestor", branch, "HEAD")
	cmd.Dir = repoDir
	return cmd.Run() == nil
}

// removeIfEmpty removes dir only when it exists and contains no entries.
// Returns nil when the directory does not exist.
func removeIfEmpty(dir string) error {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return os.Remove(dir)
	}
	return nil
}
