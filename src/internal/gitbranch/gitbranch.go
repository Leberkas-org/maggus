package gitbranch

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/leberkas-org/maggus/internal/gitutil"
)

// taskSegRe matches TASK-<plan>[−<task>] and captures plan and optional task segments.
var taskSegRe = regexp.MustCompile(`^TASK-([^-]+)(?:-(.+))?$`)

// bugSegRe matches BUG-<plan>[−<task>] and captures plan number and optional task segment.
var bugSegRe = regexp.MustCompile(`^BUG-(\d+)(?:-(.+))?$`)

// IsProtected returns true if the branch name is in the protected list.
func IsProtected(branch string, protectedList []string) bool {
	for _, p := range protectedList {
		if p == branch {
			return true
		}
	}
	return false
}

// BranchName generates a hierarchical branch name from a task ID.
// BUG-NNN-MMM produces "bugfix/maggus-bug-NNN/task-MMM".
// TASK-NNN-MMM produces "feature/maggus-NNN/task-MMM".
func BranchName(taskID string) string {
	if m := bugSegRe.FindStringSubmatch(taskID); m != nil {
		planNum := strings.ToLower(m[1])
		taskSeg := m[2]
		if taskSeg == "" {
			taskSeg = planNum
		} else {
			taskSeg = strings.ToLower(taskSeg)
		}
		return fmt.Sprintf("bugfix/maggus-bug-%s/task-%s", planNum, taskSeg)
	}
	return FeatureBranchName(taskID)
}

// FeatureBranchName generates a hierarchical feature branch name from a task ID.
// For example, "TASK-038-003" becomes "feature/maggus-038/task-003",
// and "TASK-003" (single-segment) becomes "feature/maggus-003/task-003".
func FeatureBranchName(taskID string) string {
	m := taskSegRe.FindStringSubmatch(taskID)
	if m == nil {
		return "feature/maggus-000/task-000"
	}
	planNum := strings.ToLower(m[1])
	taskSeg := m[2]
	if taskSeg == "" {
		taskSeg = planNum
	} else {
		taskSeg = strings.ToLower(taskSeg)
	}
	return fmt.Sprintf("feature/maggus-%s/task-%s", planNum, taskSeg)
}

// EnsureFeatureBranch checks the current branch and creates a feature branch if on a protected branch.
// Returns the branch name that is now checked out and any messages to display.
// If git is not available or the directory is not a repo, it returns a warning message and empty branch.
func EnsureFeatureBranch(workDir string, taskID string, protectedList []string) (branch string, msg string, err error) {
	current, err := currentBranch(workDir)
	if err != nil {
		return "", fmt.Sprintf("Warning: could not detect git branch: %v. Continuing without branch switching.", err), nil
	}

	if !IsProtected(current, protectedList) {
		return current, fmt.Sprintf("On branch %s (not protected, staying here)", current), nil
	}

	target := BranchName(taskID)
	if err := createAndCheckout(workDir, target); err != nil {
		return "", "", fmt.Errorf("create feature branch %s: %w", target, err)
	}

	return target, fmt.Sprintf("Switched from protected branch %s to %s", current, target), nil
}

func currentBranch(dir string) (string, error) {
	cmd := gitutil.Command("rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func branchExists(dir string, branch string) bool {
	cmd := gitutil.Command("rev-parse", "--verify", "--quiet", branch)
	cmd.Dir = dir
	return cmd.Run() == nil
}

func createAndCheckout(dir string, branch string) error {
	args := []string{"checkout"}
	if !branchExists(dir, branch) {
		args = append(args, "-b")
	}
	args = append(args, branch)

	cmd := gitutil.Command(args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// DeleteBranch deletes branchName in the repository at repoDir using git branch -d.
// It only deletes fully-merged branches; unmerged branches and non-existent branches
// are rejected with an error. Cleanup callers should treat errors as advisory.
func DeleteBranch(repoDir, branchName string) error {
	cmd := gitutil.Command("branch", "-d", branchName)
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git branch -d %s: %w: %s", branchName, err, strings.TrimSpace(string(out)))
	}
	return nil
}
