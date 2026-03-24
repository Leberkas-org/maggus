package gitbranch

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

var taskIDSuffixRe = regexp.MustCompile(`^TASK-(.+)$`)
var bugIDSuffixRe = regexp.MustCompile(`^BUG-(\d+)`)

// IsProtected returns true if the branch name is in the protected list.
func IsProtected(branch string, protectedList []string) bool {
	for _, p := range protectedList {
		if p == branch {
			return true
		}
	}
	return false
}

// BranchName generates a branch name from a task ID.
// BUG-NNN task IDs produce "bugfix/maggus-bug-NNN" branches.
// TASK-NNN task IDs produce "feature/maggustask-NNN" branches.
func BranchName(taskID string) string {
	if m := bugIDSuffixRe.FindStringSubmatch(taskID); m != nil {
		return fmt.Sprintf("bugfix/maggus-bug-%s", strings.ToLower(m[1]))
	}
	return FeatureBranchName(taskID)
}

// FeatureBranchName generates a feature branch name from a task ID.
// For example, "TASK-003" becomes "feature/maggustask-003",
// and "TASK-1-E05" becomes "feature/maggustask-1-e05".
func FeatureBranchName(taskID string) string {
	m := taskIDSuffixRe.FindStringSubmatch(taskID)
	if m == nil {
		return "feature/maggustask-000"
	}
	return fmt.Sprintf("feature/maggustask-%s", strings.ToLower(m[1]))
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
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = dir
	setProcAttr(cmd)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func branchExists(dir string, branch string) bool {
	cmd := exec.Command("git", "rev-parse", "--verify", "--quiet", branch)
	cmd.Dir = dir
	setProcAttr(cmd)
	return cmd.Run() == nil
}

func createAndCheckout(dir string, branch string) error {
	args := []string{"checkout"}
	if !branchExists(dir, branch) {
		args = append(args, "-b")
	}
	args = append(args, branch)

	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	setProcAttr(cmd)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
