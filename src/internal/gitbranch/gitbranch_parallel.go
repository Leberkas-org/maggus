package gitbranch

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/leberkas-org/maggus/internal/gitutil"
)

// planNumRe matches the leading plan number in TASK-NNN... or BUG-NNN...
var planNumRe = regexp.MustCompile(`^(?:TASK|BUG)-(\d+)`)

// planBranchRe matches plan-level branch names used in parallel mode.
// Matches current format: "feature/feat-NNN-plan" and "fix/bug-NNN-plan"
// (the "-plan" suffix avoids the git ref hierarchy conflict with task branches).
// Also matches legacy formats: "feature/maggus-NNN(-plan)?" and "bugfix/maggus-bug-NNN(-plan)?".
var planBranchRe = regexp.MustCompile(`^(?:feature/feat-\d+(?:-plan)?|fix/bug-\d+(?:-plan)?|feature/maggus-\d+(?:-plan)?|bugfix/maggus-bug-\d+(?:-plan)?)$`)

// PlanBranchName generates a plan-level feature branch name from a plan number.
// For example, "038" becomes "feature/feat-038".
func PlanBranchName(planNum string) string {
	return fmt.Sprintf("feature/feat-%s", strings.ToLower(planNum))
}

// BugPlanBranchName generates a plan-level bugfix branch name from a bug plan number.
// For example, "001" becomes "fix/bug-001".
func BugPlanBranchName(planNum string) string {
	return fmt.Sprintf("fix/bug-%s", strings.ToLower(planNum))
}

// PlanNumFromTaskID extracts the numeric plan number from a task or bug ID.
// For "TASK-038-003" or "TASK-038", returns "038".
// For "BUG-001-003" or "BUG-001", returns "001".
// Returns "" if the ID does not match the expected format.
func PlanNumFromTaskID(taskID string) string {
	m := planNumRe.FindStringSubmatch(taskID)
	if m == nil {
		return ""
	}
	return m[1]
}

// PlanBranchNameFromTaskID returns the plan-level branch name for the plan containing taskID.
// For "TASK-038-003" returns "feature/feat-038".
// For "BUG-001-003" returns "fix/bug-001".
// Returns "feature/feat-000" for unrecognised IDs.
func PlanBranchNameFromTaskID(taskID string) string {
	planNum := PlanNumFromTaskID(taskID)
	if planNum == "" {
		return "feature/feat-000"
	}
	if strings.HasPrefix(taskID, "BUG-") {
		return BugPlanBranchName(planNum)
	}
	return PlanBranchName(planNum)
}

// IsPlanBranch reports whether branch is a plan-level branch
// (i.e. "feature/maggus-NNN" or "bugfix/maggus-bug-NNN").
func IsPlanBranch(branch string) bool {
	return planBranchRe.MatchString(branch)
}

// parallelPlanBranchName returns the integration branch name used in parallel mode.
// It uses a "-plan" suffix (e.g. "feature/feat-038-plan") rather than the flat
// "feature/feat-038" to avoid a git ref hierarchy conflict: git cannot have
// refs/heads/feature/feat-038 (a file) and refs/heads/feature/feat-038/task-003
// (a file inside a directory of the same name) simultaneously.
func parallelPlanBranchName(taskID string) string {
	planNum := PlanNumFromTaskID(taskID)
	if planNum == "" {
		return "feature/feat-000-plan"
	}
	if strings.HasPrefix(taskID, "BUG-") {
		return fmt.Sprintf("fix/bug-%s-plan", planNum)
	}
	return fmt.Sprintf("feature/feat-%s-plan", planNum)
}

// ShouldCreatePlanBranch returns true when currentBranch is in protectedList,
// indicating that a new plan branch should be created rather than working
// directly on the current branch.
func ShouldCreatePlanBranch(currentBranch string, protectedList []string) bool {
	return IsProtected(currentBranch, protectedList)
}

// EnsurePlanBranch ensures the repository in workDir is checked out on the
// parallel-mode integration branch for the plan containing taskID.
//
// If the current branch is not in protectedList, the function returns the
// current branch unchanged — no plan branch is created.
//
// If the current branch is protected, a plan branch is created (or switched to).
// The integration branch uses a "-plan" suffix (e.g. "feature/feat-038-plan")
// so it does not conflict with task branches ("feature/feat-038/task-003").
//
// If git is not available or workDir is not a repository, a warning message is
// returned without an error, mirroring the behaviour of EnsureFeatureBranch.
func EnsurePlanBranch(workDir, taskID string, protectedList []string) (branch, msg string, err error) {
	current, err := currentBranch(workDir)
	if err != nil {
		return "", fmt.Sprintf("Warning: could not detect git branch: %v. Continuing without branch switching.", err), nil
	}

	if !ShouldCreatePlanBranch(current, protectedList) {
		return current, fmt.Sprintf("On branch %s (not protected, staying here)", current), nil
	}

	target := parallelPlanBranchName(taskID)

	if current == target {
		return current, fmt.Sprintf("Already on plan branch %s", current), nil
	}

	if err := createAndCheckout(workDir, target); err != nil {
		return "", "", fmt.Errorf("create plan branch %s: %w", target, err)
	}

	return target, fmt.Sprintf("Switched from %s to plan branch %s", current, target), nil
}

// CreateBranchFrom creates a git branch newBranch pointing at fromBranch.
// If newBranch already exists the call is a no-op and nil is returned.
// This is used in parallel mode to create task branches off the plan branch
// before handing them to gitworktree.CreateWorktree.
func CreateBranchFrom(repoRoot, newBranch, fromBranch string) error {
	if branchExists(repoRoot, newBranch) {
		return nil
	}
	cmd := gitutil.Command("branch", newBranch, fromBranch)
	cmd.Dir = repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git branch %s %s: %w: %s", newBranch, fromBranch, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// EnsureTaskBranchFromBase creates or checks out the task branch for taskID in workDir,
// branching off baseBranch if the task branch does not yet exist.
//
// In parallel mode, use this for Parallel:no tasks that execute in the main worktree:
// the main worktree is on the plan branch, and each sequential task needs its own
// task branch based off that plan branch.
func EnsureTaskBranchFromBase(workDir, taskID, baseBranch string) (branch, msg string, err error) {
	target := BranchName(taskID)

	if !branchExists(workDir, target) {
		cmd := gitutil.Command("branch", target, baseBranch)
		cmd.Dir = workDir
		if out, cmdErr := cmd.CombinedOutput(); cmdErr != nil {
			return "", "", fmt.Errorf("create task branch %s off %s: %w: %s", target, baseBranch, cmdErr, strings.TrimSpace(string(out)))
		}
	}

	cmd := gitutil.Command("checkout", target)
	cmd.Dir = workDir
	if out, cmdErr := cmd.CombinedOutput(); cmdErr != nil {
		return "", "", fmt.Errorf("checkout %s: %w: %s", target, cmdErr, strings.TrimSpace(string(out)))
	}

	return target, fmt.Sprintf("Switched to task branch %s (based on %s)", target, baseBranch), nil
}
