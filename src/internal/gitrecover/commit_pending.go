package gitrecover

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/leberkas-org/maggus/internal/gitbranch"
	"github.com/leberkas-org/maggus/internal/gitcommit"
	"github.com/leberkas-org/maggus/internal/gitmerge"
	"github.com/leberkas-org/maggus/internal/gitutil"
	"github.com/leberkas-org/maggus/internal/parser"
	"github.com/leberkas-org/maggus/internal/stores"
)

// safetyGatePatterns are the internal paths that must never be included in
// recovery commits. The list mirrors the one used in gitcommit.CommitIteration.
var safetyGatePatterns = []string{
	"COMMIT.md",
	".maggus/runs/",
	".maggus/MEMORY.md",
	".maggus/RELEASE_NOTES.md",
}

// commitPending detects and commits any pending agent work that was left behind
// by an interrupted daemon run.
//
// Decision tree:
//  1. COMMIT.md present → commit it via gitcommit.CommitIteration.
//  2. COMMIT.md absent + dirty tree + current branch is a task branch:
//     a. If the current task ID matches the next workable task: skip (resume).
//     b. Otherwise: safety-gate unstage + git add -A + recovery commit.
//  3. All other cases → no-op (returns nil).
//
// Returns a slice of human-readable log messages for every action taken.
func commitPending(repoDir string, featureStore stores.FeatureStore, bugStore stores.BugStore) ([]string, error) {
	commitPath := filepath.Join(repoDir, "COMMIT.md")

	_, statErr := os.Stat(commitPath)
	if statErr != nil && !os.IsNotExist(statErr) {
		return nil, fmt.Errorf("commitPending: stat COMMIT.md: %w", statErr)
	}

	if statErr == nil {
		// COMMIT.md exists — delegate to CommitIteration.
		result, err := gitcommit.CommitIteration(repoDir, "")
		if err != nil {
			return nil, fmt.Errorf("commitPending: commit COMMIT.md: %w", err)
		}
		if !result.Committed {
			return nil, nil
		}
		return []string{"committed pending work: " + firstLineOf(result.Message)}, nil
	}

	// COMMIT.md absent — check tree dirtiness.
	dirty, err := isWorkingTreeDirty(repoDir)
	if err != nil {
		return nil, fmt.Errorf("commitPending: %w", err)
	}
	if !dirty {
		return nil, nil // clean tree, nothing to do
	}

	current, err := currentGitBranch(repoDir)
	if err != nil {
		return nil, fmt.Errorf("commitPending: %w", err)
	}
	if !gitbranch.IsTaskBranch(current) {
		return nil, nil // not a task branch, no-op
	}

	currentTaskID := gitmerge.TaskIDFromBranch(current)

	nextTask, err := findNextWorkableTask(featureStore, bugStore)
	if err != nil {
		return nil, fmt.Errorf("commitPending: find next workable task: %w", err)
	}

	// Resume scenario: same task is still next — leave the dirty state intact.
	if nextTask != nil && nextTask.ID == currentTaskID {
		return nil, nil
	}

	// Recovery commit: unstage internals, stage everything, commit.
	for _, pattern := range safetyGatePatterns {
		unstage := gitutil.Command("reset", "HEAD", "--", pattern)
		unstage.Dir = repoDir
		_, _ = unstage.CombinedOutput() // ignore errors (files may not be staged)
	}

	addCmd := gitutil.Command("add", "-A")
	addCmd.Dir = repoDir
	if out, err := addCmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("commitPending: git add -A: %w: %s", err, strings.TrimSpace(string(out)))
	}

	msg := fmt.Sprintf("maggus: recover uncommitted changes from %s", current)
	commitCmd := gitutil.Command("commit", "-m", msg)
	commitCmd.Dir = repoDir
	out, err := commitCmd.CombinedOutput()
	outStr := strings.TrimSpace(string(out))
	if err != nil {
		if strings.Contains(outStr, "nothing to commit") || strings.Contains(outStr, "nothing added to commit") {
			return nil, nil
		}
		return nil, fmt.Errorf("commitPending: recovery commit: %w: %s", err, outStr)
	}

	return []string{fmt.Sprintf("recovered uncommitted changes from %s", current)}, nil
}

// isWorkingTreeDirty reports whether the working tree has any uncommitted changes
// (staged or unstaged).
func isWorkingTreeDirty(repoDir string) (bool, error) {
	cmd := gitutil.Command("status", "--porcelain")
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("git status --porcelain: %w", err)
	}
	return strings.TrimSpace(string(out)) != "", nil
}

// findNextWorkableTask returns the first workable task across all bug and feature plans,
// with bugs taking priority over features (matching the daemon's work-loop order).
func findNextWorkableTask(featureStore stores.FeatureStore, bugStore stores.BugStore) (*parser.Task, error) {
	bugPlans, err := bugStore.LoadAll(false)
	if err != nil {
		return nil, fmt.Errorf("load bug plans: %w", err)
	}
	for _, plan := range bugPlans {
		if t := parser.FindNextIncomplete(plan.Tasks); t != nil {
			return t, nil
		}
	}

	featurePlans, err := featureStore.LoadAll(false)
	if err != nil {
		return nil, fmt.Errorf("load feature plans: %w", err)
	}
	for _, plan := range featurePlans {
		if t := parser.FindNextIncomplete(plan.Tasks); t != nil {
			return t, nil
		}
	}

	return nil, nil
}

// firstLineOf returns the first non-empty line from s.
func firstLineOf(s string) string {
	for _, line := range strings.Split(s, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			return line
		}
	}
	return s
}
