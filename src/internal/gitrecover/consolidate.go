package gitrecover

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/leberkas-org/maggus/internal/config"
	"github.com/leberkas-org/maggus/internal/gitbranch"
	"github.com/leberkas-org/maggus/internal/gitmerge"
	"github.com/leberkas-org/maggus/internal/gitutil"
)

// consolidateBranches detects when the repository is on a task branch and
// merges it back into the nearest non-task, non-protected ancestor branch.
//
// Steps:
//  1. If the current branch is not a task branch, or is protected: return nil (skip).
//  2. Find the merge target by listing branches that are ancestors of HEAD
//     (git branch --merged HEAD), filtering out task branches and protected
//     branches, and picking the one closest to HEAD.
//  3. If no merge target exists: create an integration branch ("feature/maggus"
//     for TASK- IDs, "bug/maggus" for BUG- IDs) off the first existing
//     protected branch from cfg.
//  4. Check out the merge target and run git merge --no-ff <current-task-branch>.
//  5. Delete sibling task branches (same prefix) that are now ancestors of HEAD.
//
// Returns log messages for each action taken. On merge conflict the merge is
// aborted and a descriptive error is returned; the repository is not left in a
// mid-merge state.
func consolidateBranches(repoDir string, cfg config.Config) ([]string, error) {
	current, err := currentGitBranch(repoDir)
	if err != nil {
		return nil, fmt.Errorf("consolidateBranches: get current branch: %w", err)
	}

	protectedList := cfg.Git.ProtectedBranchList()

	// Skip if not a task branch or if (somehow) the task branch is protected.
	if !gitbranch.IsTaskBranch(current) || gitbranch.IsProtected(current, protectedList) {
		return nil, nil
	}

	taskID := gitmerge.TaskIDFromBranch(current)
	prefix := gitbranch.TaskPrefixFromBranch(current)

	mergeTarget, created, err := findOrCreateMergeTarget(repoDir, current, taskID, protectedList)
	if err != nil {
		return nil, err
	}

	var logs []string
	if created {
		logs = append(logs, fmt.Sprintf("created integration branch %s", mergeTarget))
	}

	// Check out the merge target.
	if err := checkoutRef(repoDir, mergeTarget); err != nil {
		return logs, fmt.Errorf("consolidateBranches: checkout %s: %w", mergeTarget, err)
	}

	// Merge the task branch.
	if err := mergeNoFF(repoDir, mergeTarget, current); err != nil {
		return logs, err
	}
	logs = append(logs, fmt.Sprintf("merged %s into %s", current, mergeTarget))

	// Delete sibling task branches that are now fully merged into HEAD.
	siblingLogs := deleteAncestorSiblings(repoDir, prefix)
	logs = append(logs, siblingLogs...)

	return logs, nil
}

// findOrCreateMergeTarget returns the branch that the current task branch
// should be merged into. It first searches for a non-task, non-protected
// ancestor of HEAD. If none is found it creates a fresh integration branch
// (feature/maggus or bug/maggus) off the first existing protected branch.
func findOrCreateMergeTarget(repoDir, currentBranch, taskID string, protectedList []string) (string, bool, error) {
	merged, err := branchesMergedIntoHEAD(repoDir)
	if err != nil {
		return "", false, err
	}

	var candidates []string
	for _, b := range merged {
		if b == currentBranch {
			continue
		}
		if gitbranch.IsTaskBranch(b) {
			continue
		}
		if gitbranch.IsProtected(b, protectedList) {
			continue
		}
		candidates = append(candidates, b)
	}

	if len(candidates) > 0 {
		return pickClosestToHEAD(repoDir, candidates), false, nil
	}

	// No suitable ancestor — create an integration branch off the first
	// existing protected branch.
	intBranch := integrationBranchName(taskID)

	var base string
	for _, p := range protectedList {
		if localBranchExists(repoDir, p) {
			base = p
			break
		}
	}
	if base == "" {
		return "", false, fmt.Errorf("consolidateBranches: no existing protected branch to base integration branch %s on", intBranch)
	}

	// CreateBranchFrom is a no-op if the branch already exists.
	if err := gitbranch.CreateBranchFrom(repoDir, intBranch, base); err != nil {
		return "", false, fmt.Errorf("consolidateBranches: create %s off %s: %w", intBranch, base, err)
	}

	return intBranch, true, nil
}

// integrationBranchName returns the fallback integration branch name for the
// given task ID: "feature/maggus" for TASK- IDs, "bug/maggus" for BUG- IDs.
func integrationBranchName(taskID string) string {
	if strings.HasPrefix(taskID, "BUG-") {
		return "bug/maggus"
	}
	return "feature/maggus"
}

// pickClosestToHEAD returns the candidate branch whose tip is fewest commits
// behind HEAD (i.e. most recently diverged from the current branch).
func pickClosestToHEAD(repoDir string, candidates []string) string {
	best := candidates[0]
	bestDist := commitsBehindHEAD(repoDir, best)
	for _, c := range candidates[1:] {
		d := commitsBehindHEAD(repoDir, c)
		if d < bestDist {
			bestDist = d
			best = c
		}
	}
	return best
}

// deleteAncestorSiblings finds all local branches whose name starts with
// prefix+"/" and deletes those that are fully merged into the current HEAD.
// Errors from individual deletions are logged as warnings rather than
// propagated, so a failed deletion does not abort the cleanup.
func deleteAncestorSiblings(repoDir, prefix string) []string {
	all, err := allLocalBranches(repoDir)
	if err != nil {
		return nil
	}

	merged, err := branchesMergedIntoHEAD(repoDir)
	if err != nil {
		return nil
	}
	mergedSet := make(map[string]bool, len(merged))
	for _, b := range merged {
		mergedSet[b] = true
	}

	var logs []string
	sibPrefix := prefix + "/"
	for _, b := range all {
		if !strings.HasPrefix(b, sibPrefix) {
			continue
		}
		if !mergedSet[b] {
			continue
		}
		if err := gitbranch.DeleteBranch(repoDir, b); err != nil {
			logs = append(logs, fmt.Sprintf("warning: could not delete %s: %v", b, err))
			continue
		}
		logs = append(logs, fmt.Sprintf("deleted merged task branch %s", b))
	}
	return logs
}

// mergeNoFF runs git merge --no-ff --no-edit sourceBranch in repoDir.
// On conflict the merge is aborted and an error is returned. The repo is
// guaranteed to not be in a mid-merge state when an error is returned.
func mergeNoFF(repoDir, targetBranch, sourceBranch string) error {
	cmd := gitutil.Command("merge", "--no-ff", "--no-edit", sourceBranch)
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}

	// Check whether a real merge conflict is in progress (MERGE_HEAD exists).
	checkCmd := gitutil.Command("rev-parse", "-q", "--verify", "MERGE_HEAD")
	checkCmd.Dir = repoDir
	if checkCmd.Run() == nil {
		// Conflict — abort to leave the repo in a clean state.
		abortCmd := gitutil.Command("merge", "--abort")
		abortCmd.Dir = repoDir
		_ = abortCmd.Run()
		return fmt.Errorf("merge conflict merging %s into %s: resolve manually", sourceBranch, targetBranch)
	}

	// Non-conflict failure (e.g. branch not found).
	return fmt.Errorf("merge %s into %s: %w: %s", sourceBranch, targetBranch, err, strings.TrimSpace(string(out)))
}

// currentGitBranch returns the abbreviated ref name of HEAD (the current branch).
func currentGitBranch(repoDir string) (string, error) {
	cmd := gitutil.Command("rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// checkoutRef runs git checkout branch in repoDir.
func checkoutRef(repoDir, branch string) error {
	cmd := gitutil.Command("checkout", branch)
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// branchesMergedIntoHEAD returns all local branch names whose tip is reachable
// from HEAD (i.e. git branch --merged HEAD).
func branchesMergedIntoHEAD(repoDir string) ([]string, error) {
	cmd := gitutil.Command("branch", "--merged", "HEAD")
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git branch --merged HEAD: %w", err)
	}
	return parseBranchList(string(out)), nil
}

// allLocalBranches returns all local branch names.
func allLocalBranches(repoDir string) ([]string, error) {
	cmd := gitutil.Command("branch")
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git branch: %w", err)
	}
	return parseBranchList(string(out)), nil
}

// parseBranchList parses the output of git branch (or git branch --merged) into
// a slice of branch names, stripping the leading "* " or "  " markers.
func parseBranchList(output string) []string {
	var branches []string
	for _, line := range strings.Split(output, "\n") {
		b := strings.TrimSpace(line)
		// Remove the "* " current-branch marker if present after trimming.
		b = strings.TrimPrefix(b, "* ")
		if b != "" {
			branches = append(branches, b)
		}
	}
	return branches
}

// localBranchExists reports whether branch exists in the local repository.
func localBranchExists(repoDir, branch string) bool {
	cmd := gitutil.Command("rev-parse", "--verify", "--quiet", branch)
	cmd.Dir = repoDir
	return cmd.Run() == nil
}

// commitsBehindHEAD returns the number of commits in HEAD that are not in
// branch (i.e. how far ahead HEAD is of branch). Returns math.MaxInt on error.
func commitsBehindHEAD(repoDir, branch string) int {
	cmd := gitutil.Command("rev-list", "--count", branch+"..HEAD")
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		return math.MaxInt
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return math.MaxInt
	}
	return n
}
