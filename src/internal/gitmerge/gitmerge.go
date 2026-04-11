package gitmerge

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/leberkas-org/maggus/internal/gitbranch"
	"github.com/leberkas-org/maggus/internal/gitutil"
	"github.com/leberkas-org/maggus/internal/gitworktree"
)

// MergeConflictError is returned when merging a task branch encounters conflicts.
type MergeConflictError struct {
	FeatureBranch string
	TaskBranch    string
}

func (e *MergeConflictError) Error() string {
	return fmt.Sprintf("merge conflict merging %s into %s", e.TaskBranch, e.FeatureBranch)
}

// MergeTaskBranch merges taskBranch into featureBranch in the repository at repoRoot
// using a standard merge commit (no rebase, no fast-forward squash).
//
// On success the task branch is deleted and its worktree (if any) is removed.
// On conflict the merge is aborted, a BLOCKED criterion is injected into the
// task's plan file, and a *MergeConflictError is returned. The worktree is
// preserved so the developer can inspect the changes.
func MergeTaskBranch(repoRoot, featureBranch, taskBranch string) error {
	if err := checkout(repoRoot, featureBranch); err != nil {
		return err
	}

	cmd := gitutil.Command("merge", "--no-ff", "--no-edit", taskBranch)
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		return handleMergeFailure(repoRoot, featureBranch, taskBranch, out, err)
	}

	return cleanup(repoRoot, taskBranch)
}

func checkout(repoRoot, branch string) error {
	cmd := gitutil.Command("checkout", branch)
	cmd.Dir = repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("checkout %s: %w: %s", branch, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func handleMergeFailure(repoRoot, featureBranch, taskBranch string, mergeOutput []byte, mergeErr error) error {
	// Check whether a merge is actually in progress (MERGE_HEAD exists).
	// If not, this was a non-conflict failure (e.g. branch not found).
	checkCmd := gitutil.Command("rev-parse", "-q", "--verify", "MERGE_HEAD")
	checkCmd.Dir = repoRoot
	if checkCmd.Run() != nil {
		return fmt.Errorf("merge %s into %s: %w: %s",
			taskBranch, featureBranch, mergeErr, strings.TrimSpace(string(mergeOutput)))
	}

	// Conflict — abort the merge.
	abortCmd := gitutil.Command("merge", "--abort")
	abortCmd.Dir = repoRoot
	_ = abortCmd.Run()

	// Best-effort: inject a BLOCKED criterion into the task's plan file.
	_ = injectBlockedCriterion(repoRoot, featureBranch, taskBranch)

	return &MergeConflictError{FeatureBranch: featureBranch, TaskBranch: taskBranch}
}

func cleanup(repoRoot, taskBranch string) error {
	// Remove the worktree first so the branch is no longer checked out anywhere.
	if wtPath := findWorktreeForBranch(repoRoot, taskBranch); wtPath != "" {
		if err := gitworktree.RemoveWorktree(repoRoot, wtPath); err != nil {
			return fmt.Errorf("remove worktree for %s: %w", taskBranch, err)
		}
	}

	// Delete the task branch (safe because it is fully merged).
	cmd := gitutil.Command("branch", "-d", taskBranch)
	cmd.Dir = repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("delete branch %s: %w: %s", taskBranch, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// findWorktreeForBranch returns the filesystem path of the worktree that has
// branch checked out, or "" if none is found.
func findWorktreeForBranch(repoRoot, branch string) string {
	wts, err := gitworktree.ListWorktrees(repoRoot)
	if err != nil {
		return ""
	}
	for _, wt := range wts {
		if wt.Branch == branch {
			return wt.Path
		}
	}
	return ""
}

// --- BLOCKED criterion injection ---

var taskHeadingRe = regexp.MustCompile(`^###\s+((?:TASK|BUG)-[\w-]+?):\s`)

// featureTaskBranchRe matches task branches in the new hierarchical format:
// feature/maggus-<plan>/task-<task>
var featureTaskBranchRe = regexp.MustCompile(`^feature/maggus-([^/]+)/task-(.+)$`)

// bugTaskBranchRe matches bug task branches in the new hierarchical format:
// bugfix/maggus-bug-<plan>/task-<task>
var bugTaskBranchRe = regexp.MustCompile(`^bugfix/maggus-bug-([^/]+)/task-(.+)$`)

// taskIDFromBranch derives the task ID from a task branch name.
//
//	feature/maggus-038/task-003      ->  TASK-038-003
//	bugfix/maggus-bug-001/task-003   ->  BUG-001-003
func taskIDFromBranch(branch string) string {
	if m := featureTaskBranchRe.FindStringSubmatch(branch); m != nil {
		return "TASK-" + strings.ToUpper(m[1]) + "-" + strings.ToUpper(m[2])
	}
	if m := bugTaskBranchRe.FindStringSubmatch(branch); m != nil {
		return "BUG-" + strings.ToUpper(m[1]) + "-" + strings.ToUpper(m[2])
	}
	return ""
}

// planFileFromTaskID returns the plan file path for the given task ID,
// or "" if the file does not exist on disk.
func planFileFromTaskID(repoRoot, taskID string) string {
	planNum := gitbranch.PlanNumFromTaskID(taskID)
	if planNum == "" {
		return ""
	}

	var path string
	if strings.HasPrefix(taskID, "BUG-") {
		path = filepath.Join(repoRoot, ".maggus", "bugs", "bug_"+planNum+".md")
	} else {
		path = filepath.Join(repoRoot, ".maggus", "features", "feature_"+planNum+".md")
	}

	if _, err := os.Stat(path); err != nil {
		return ""
	}
	return path
}

func injectBlockedCriterion(repoRoot, featureBranch, taskBranch string) error {
	taskID := taskIDFromBranch(taskBranch)
	if taskID == "" {
		return fmt.Errorf("cannot derive task ID from branch %s", taskBranch)
	}

	planFile := planFileFromTaskID(repoRoot, taskID)
	if planFile == "" {
		return fmt.Errorf("plan file not found for task %s", taskID)
	}

	blockedText := fmt.Sprintf(
		"BLOCKED: Merge conflict merging %s into %s \u2014 resolve manually, then uncheck this criterion",
		taskBranch, featureBranch,
	)

	return addCriterionToTask(planFile, taskID, blockedText)
}

// addCriterionToTask appends an unchecked criterion line after the last existing
// criterion in the specified task section of the plan file.
func addCriterionToTask(planFile, taskID, text string) error {
	data, err := os.ReadFile(planFile)
	if err != nil {
		return fmt.Errorf("read %s: %w", planFile, err)
	}

	lines := strings.Split(string(data), "\n")
	insertIdx := -1
	inTask := false

	for i, line := range lines {
		if m := taskHeadingRe.FindStringSubmatch(line); m != nil {
			if m[1] == taskID {
				inTask = true
			} else if inTask {
				break // entered the next task section
			}
			continue
		}
		if inTask {
			if strings.HasPrefix(line, "## ") {
				break // left the tasks section entirely
			}
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "- [") {
				insertIdx = i
			}
		}
	}

	if insertIdx < 0 {
		return fmt.Errorf("no criteria found for task %s in %s", taskID, planFile)
	}

	newLine := "- [ ] " + text
	result := make([]string, 0, len(lines)+1)
	result = append(result, lines[:insertIdx+1]...)
	result = append(result, newLine)
	result = append(result, lines[insertIdx+1:]...)

	return os.WriteFile(planFile, []byte(strings.Join(result, "\n")), 0o644)
}
