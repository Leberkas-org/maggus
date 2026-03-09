package gitbranch

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

// protectedBranches are branches that should not be committed to directly.
var protectedBranches = map[string]bool{
	"main":   true,
	"master": true,
	"dev":    true,
}

var taskNumberRe = regexp.MustCompile(`TASK-(\d+)`)

// IsProtected returns true if the branch name is a protected branch.
func IsProtected(branch string) bool {
	return protectedBranches[branch]
}

// FeatureBranchName generates a feature branch name from a task ID.
// For example, "TASK-003" becomes "feature/maggustask-003".
func FeatureBranchName(taskID string) string {
	m := taskNumberRe.FindStringSubmatch(taskID)
	if m == nil {
		return "feature/maggustask-000"
	}
	return fmt.Sprintf("feature/maggustask-%s", m[1])
}

// EnsureFeatureBranch checks the current branch and creates a feature branch if on a protected branch.
// Returns the branch name that is now checked out and any messages to display.
// If git is not available or the directory is not a repo, it returns a warning message and empty branch.
func EnsureFeatureBranch(workDir string, taskID string) (branch string, msg string, err error) {
	current, err := currentBranch(workDir)
	if err != nil {
		return "", fmt.Sprintf("Warning: could not detect git branch: %v. Continuing without branch switching.", err), nil
	}

	if !IsProtected(current) {
		return current, fmt.Sprintf("On branch %s (not protected, staying here)", current), nil
	}

	target := FeatureBranchName(taskID)
	if err := createAndCheckout(workDir, target); err != nil {
		return "", "", fmt.Errorf("create feature branch %s: %w", target, err)
	}

	return target, fmt.Sprintf("Switched from protected branch %s to %s", current, target), nil
}

func currentBranch(dir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func createAndCheckout(dir string, branch string) error {
	cmd := exec.Command("git", "checkout", "-b", branch)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
