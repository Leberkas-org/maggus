package gitbranch

import (
	"os/exec"
	"strings"
	"testing"
)

// ── Pure name-generation tests ────────────────────────────────────────────────

func TestPlanBranchName(t *testing.T) {
	tests := []struct {
		planNum string
		want    string
	}{
		{"038", "feature/feat-038"},
		{"001", "feature/feat-001"},
		{"100", "feature/feat-100"},
		{"000", "feature/feat-000"},
	}
	for _, tt := range tests {
		got := PlanBranchName(tt.planNum)
		if got != tt.want {
			t.Errorf("PlanBranchName(%q) = %q, want %q", tt.planNum, got, tt.want)
		}
	}
}

func TestBugPlanBranchName(t *testing.T) {
	tests := []struct {
		planNum string
		want    string
	}{
		{"001", "fix/bug-001"},
		{"042", "fix/bug-042"},
		{"000", "fix/bug-000"},
	}
	for _, tt := range tests {
		got := BugPlanBranchName(tt.planNum)
		if got != tt.want {
			t.Errorf("BugPlanBranchName(%q) = %q, want %q", tt.planNum, got, tt.want)
		}
	}
}

func TestPlanNumFromTaskID(t *testing.T) {
	tests := []struct {
		taskID string
		want   string
	}{
		{"TASK-038-003", "038"},
		{"TASK-001-001", "001"},
		{"TASK-038", "038"},
		{"TASK-100", "100"},
		{"BUG-001-003", "001"},
		{"BUG-042-001", "042"},
		{"BUG-001", "001"},
		{"INVALID", ""},
		{"", ""},
	}
	for _, tt := range tests {
		got := PlanNumFromTaskID(tt.taskID)
		if got != tt.want {
			t.Errorf("PlanNumFromTaskID(%q) = %q, want %q", tt.taskID, got, tt.want)
		}
	}
}

func TestPlanBranchNameFromTaskID(t *testing.T) {
	tests := []struct {
		taskID string
		want   string
	}{
		{"TASK-038-003", "feature/feat-038"},
		{"TASK-001-001", "feature/feat-001"},
		{"TASK-038", "feature/feat-038"},
		{"BUG-001-003", "fix/bug-001"},
		{"BUG-042-001", "fix/bug-042"},
		{"INVALID", "feature/feat-000"},
		{"", "feature/feat-000"},
	}
	for _, tt := range tests {
		got := PlanBranchNameFromTaskID(tt.taskID)
		if got != tt.want {
			t.Errorf("PlanBranchNameFromTaskID(%q) = %q, want %q", tt.taskID, got, tt.want)
		}
	}
}

func TestIsPlanBranch(t *testing.T) {
	tests := []struct {
		branch string
		want   bool
	}{
		// Current format: feature/feat-NNN and fix/bug-NNN
		{"feature/feat-038", true},
		{"feature/feat-001", true},
		{"feature/feat-000", true},
		{"fix/bug-001", true},
		{"fix/bug-042", true},
		// Current format with -plan suffix (parallel mode)
		{"feature/feat-038-plan", true},
		{"feature/feat-001-plan", true},
		{"fix/bug-001-plan", true},
		{"fix/bug-042-plan", true},
		// Legacy format: feature/maggus-NNN and bugfix/maggus-bug-NNN
		{"feature/maggus-038-plan", true},
		{"feature/maggus-001-plan", true},
		{"feature/maggus-000-plan", true},
		{"bugfix/maggus-bug-001-plan", true},
		{"bugfix/maggus-bug-042-plan", true},
		{"feature/maggus-038", true},
		{"feature/maggus-001", true},
		{"feature/maggus-000", true},
		{"bugfix/maggus-bug-001", true},
		{"bugfix/maggus-bug-042", true},
		// Not plan branches
		{"feature/maggus-038/task-003", false},
		{"feature/maggus-001/task-001", false},
		{"master", false},
		{"main", false},
		{"feature/something-else", false},
		{"feature/maggus-", false},    // no digits
		{"bugfix/maggus-bug-", false}, // no digits
		{"feature/maggus-abc", false}, // non-digit suffix
		{"feature/feat-", false},      // no digits
		{"fix/bug-", false},           // no digits
	}
	for _, tt := range tests {
		got := IsPlanBranch(tt.branch)
		if got != tt.want {
			t.Errorf("IsPlanBranch(%q) = %v, want %v", tt.branch, got, tt.want)
		}
	}
}

// ── ShouldCreatePlanBranch tests ─────────────────────────────────────────────

func TestShouldCreatePlanBranch(t *testing.T) {
	protected := []string{"main", "master", "dev"}
	tests := []struct {
		branch string
		list   []string
		want   bool
	}{
		{"main", protected, true},
		{"master", protected, true},
		{"dev", protected, true},
		{"feature/my-work", protected, false},
		{"fix/bug-123", protected, false},
		{"feature/feat-038-plan", protected, false},
		{"", protected, false},
		{"release", []string{"main", "release"}, true},
		{"main", []string{"main", "release"}, true},
		{"dev", []string{"main", "release"}, false},
	}
	for _, tt := range tests {
		got := ShouldCreatePlanBranch(tt.branch, tt.list)
		if got != tt.want {
			t.Errorf("ShouldCreatePlanBranch(%q, %v) = %v, want %v", tt.branch, tt.list, got, tt.want)
		}
	}
}

// ── EnsurePlanBranch integration tests ───────────────────────────────────────

func TestEnsurePlanBranch_AlreadyOnPlanBranch(t *testing.T) {
	tmp := t.TempDir()
	initGitRepoWithBranch(t, tmp, "master")
	checkoutBranch(t, tmp, "feature/feat-038-plan")

	// Not protected → stays on current branch (passthrough)
	branch, msg, err := EnsurePlanBranch(tmp, "TASK-038-003", []string{"master"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if branch != "feature/feat-038-plan" {
		t.Errorf("branch = %q, want %q", branch, "feature/feat-038-plan")
	}
	if msg == "" {
		t.Error("expected non-empty message")
	}
	if got := getCurrentBranch(t, tmp); got != "feature/feat-038-plan" {
		t.Errorf("git branch = %q, want %q", got, "feature/feat-038-plan")
	}
}

func TestEnsurePlanBranch_OnProtectedBranch(t *testing.T) {
	tmp := t.TempDir()
	initGitRepoWithBranch(t, tmp, "master")

	branch, msg, err := EnsurePlanBranch(tmp, "TASK-038-003", []string{"main", "master", "dev"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if branch != "feature/feat-038-plan" {
		t.Errorf("branch = %q, want %q", branch, "feature/feat-038-plan")
	}
	if msg == "" {
		t.Error("expected non-empty message")
	}
	if got := getCurrentBranch(t, tmp); got != "feature/feat-038-plan" {
		t.Errorf("git branch = %q, want %q", got, "feature/feat-038-plan")
	}
}

func TestEnsurePlanBranch_NonProtectedBranch_Passthrough(t *testing.T) {
	tmp := t.TempDir()
	initGitRepoWithBranch(t, tmp, "master")
	checkoutBranch(t, tmp, "feature/other-work")

	// feature/other-work is not in the protected list → stays on current branch
	branch, msg, err := EnsurePlanBranch(tmp, "TASK-038-003", []string{"main", "master", "dev"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if branch != "feature/other-work" {
		t.Errorf("branch = %q, want %q", branch, "feature/other-work")
	}
	if msg == "" {
		t.Error("expected non-empty message")
	}
	if got := getCurrentBranch(t, tmp); got != "feature/other-work" {
		t.Errorf("git branch = %q, want %q", got, "feature/other-work")
	}
}

func TestEnsurePlanBranch_ExistingPlanBranch_SwitchesBack(t *testing.T) {
	// Plan branch was created earlier; we went back to master and now re-enter.
	tmp := t.TempDir()
	initGitRepoWithBranch(t, tmp, "master")
	protected := []string{"main", "master", "dev"}
	checkoutBranch(t, tmp, "feature/feat-038-plan")

	cmd := exec.Command("git", "checkout", "master")
	cmd.Dir = tmp
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("checkout master: %v\n%s", err, out)
	}

	branch, msg, err := EnsurePlanBranch(tmp, "TASK-038-003", protected)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if branch != "feature/feat-038-plan" {
		t.Errorf("branch = %q, want %q", branch, "feature/feat-038-plan")
	}
	if msg == "" {
		t.Error("expected non-empty message")
	}
	if got := getCurrentBranch(t, tmp); got != "feature/feat-038-plan" {
		t.Errorf("git branch = %q, want %q", got, "feature/feat-038-plan")
	}
}

func TestEnsurePlanBranch_NotAGitRepo(t *testing.T) {
	tmp := t.TempDir() // no git init

	branch, msg, err := EnsurePlanBranch(tmp, "TASK-038-003", []string{"main", "master", "dev"})
	if err != nil {
		t.Fatalf("should not return error for non-git dir, got: %v", err)
	}
	if branch != "" {
		t.Errorf("branch should be empty for non-git dir, got %q", branch)
	}
	if msg == "" {
		t.Error("expected a warning message for non-git dir")
	}
}

func TestEnsurePlanBranch_BugTask_Protected(t *testing.T) {
	tmp := t.TempDir()
	initGitRepoWithBranch(t, tmp, "master")

	branch, msg, err := EnsurePlanBranch(tmp, "BUG-001-003", []string{"main", "master", "dev"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if branch != "fix/bug-001-plan" {
		t.Errorf("branch = %q, want %q", branch, "fix/bug-001-plan")
	}
	if msg == "" {
		t.Error("expected non-empty message")
	}
	if got := getCurrentBranch(t, tmp); got != "fix/bug-001-plan" {
		t.Errorf("git branch = %q, want %q", got, "fix/bug-001-plan")
	}
}

func TestEnsurePlanBranch_BugTask_NonProtected(t *testing.T) {
	tmp := t.TempDir()
	initGitRepoWithBranch(t, tmp, "master")
	checkoutBranch(t, tmp, "my-bugfix-branch")

	// On a non-protected branch → stays on current branch
	branch, msg, err := EnsurePlanBranch(tmp, "BUG-001-003", []string{"main", "master", "dev"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if branch != "my-bugfix-branch" {
		t.Errorf("branch = %q, want %q", branch, "my-bugfix-branch")
	}
	if msg == "" {
		t.Error("expected non-empty message")
	}
	if got := getCurrentBranch(t, tmp); got != "my-bugfix-branch" {
		t.Errorf("git branch = %q, want %q", got, "my-bugfix-branch")
	}
}

func TestEnsurePlanBranch_CoexistsWithTaskBranches(t *testing.T) {
	// Verify the plan branch (-plan suffix) can coexist with task branches
	// (feature/feat-038/task-*) without a git ref hierarchy conflict.
	tmp := t.TempDir()
	initGitRepoWithBranch(t, tmp, "master")

	// Create the plan branch.
	planBranch, _, err := EnsurePlanBranch(tmp, "TASK-038-003", []string{"main", "master", "dev"})
	if err != nil {
		t.Fatalf("EnsurePlanBranch: %v", err)
	}
	if planBranch != "feature/feat-038-plan" {
		t.Errorf("planBranch = %q, want %q", planBranch, "feature/feat-038-plan")
	}

	// Creating task branches alongside the plan branch must not fail.
	if err := CreateBranchFrom(tmp, "feature/feat-038/task-001", planBranch); err != nil {
		t.Fatalf("CreateBranchFrom task-001: %v", err)
	}
	if err := CreateBranchFrom(tmp, "feature/feat-038/task-002", planBranch); err != nil {
		t.Fatalf("CreateBranchFrom task-002: %v", err)
	}

	// All three refs must exist.
	for _, ref := range []string{"feature/feat-038-plan", "feature/feat-038/task-001", "feature/feat-038/task-002"} {
		cmd := exec.Command("git", "rev-parse", "--verify", ref)
		cmd.Dir = tmp
		if err := cmd.Run(); err != nil {
			t.Errorf("ref %q should exist after creation, got error: %v", ref, err)
		}
	}
}

// ── CreateBranchFrom integration tests ───────────────────────────────────────

func TestCreateBranchFrom_NewBranch(t *testing.T) {
	tmp := t.TempDir()
	initGitRepoWithBranch(t, tmp, "master")

	if err := CreateBranchFrom(tmp, "feature/new-task", "master"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cmd := exec.Command("git", "rev-parse", "--verify", "feature/new-task")
	cmd.Dir = tmp
	if err := cmd.Run(); err != nil {
		t.Error("branch feature/new-task should exist after CreateBranchFrom")
	}
}

func TestCreateBranchFrom_ExistingBranch_NoOp(t *testing.T) {
	tmp := t.TempDir()
	initGitRepoWithBranch(t, tmp, "master")
	checkoutBranch(t, tmp, "feature/existing")

	cmd := exec.Command("git", "checkout", "master")
	cmd.Dir = tmp
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("checkout master: %v\n%s", err, out)
	}

	if err := CreateBranchFrom(tmp, "feature/existing", "master"); err != nil {
		t.Fatalf("unexpected error on existing branch: %v", err)
	}
}

func TestCreateBranchFrom_BranchCreatedOffCorrectBase(t *testing.T) {
	// Verify the new branch points at fromBranch's HEAD, not the current HEAD.
	// Note: we use "feature/base-038" as the base (not "feature/maggus-038") because
	// git cannot have refs/heads/feature/maggus-038 (file) and
	// refs/heads/feature/maggus-038/task-003 (file in dir) simultaneously.
	tmp := t.TempDir()
	initGitRepoWithBranch(t, tmp, "master")
	checkoutBranch(t, tmp, "feature/base-038")
	addEmptyCommit(t, tmp, "base branch commit")

	// HEAD is now 1 commit ahead of master on feature/base-038.
	baseHead := getBranchHead(t, tmp, "feature/base-038")
	masterHead := getBranchHead(t, tmp, "master")
	if baseHead == masterHead {
		t.Fatal("test setup error: base branch not ahead of master")
	}

	// Create task branch off base branch while still checked out on base branch.
	if err := CreateBranchFrom(tmp, "feature/maggus-038/task-003", "feature/base-038"); err != nil {
		t.Fatalf("CreateBranchFrom failed: %v", err)
	}

	taskHead := getBranchHead(t, tmp, "feature/maggus-038/task-003")
	if taskHead != baseHead {
		t.Errorf("task branch HEAD %q != base branch HEAD %q; not based on base branch", taskHead, baseHead)
	}
}

// ── EnsureTaskBranchFromBase integration tests ────────────────────────────────

func TestEnsureTaskBranchFromBase_NewBranch(t *testing.T) {
	// Note: we use master as base (not "feature/maggus-038") because git cannot
	// have refs/heads/feature/maggus-038 and refs/heads/feature/maggus-038/task-003
	// as refs simultaneously (file vs directory conflict in .git/refs/heads/).
	tmp := t.TempDir()
	initGitRepoWithBranch(t, tmp, "master")

	branch, msg, err := EnsureTaskBranchFromBase(tmp, "TASK-038-003", "master")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if branch != "feature/maggus-038/task-003" {
		t.Errorf("branch = %q, want %q", branch, "feature/maggus-038/task-003")
	}
	if msg == "" {
		t.Error("expected non-empty message")
	}
	if got := getCurrentBranch(t, tmp); got != "feature/maggus-038/task-003" {
		t.Errorf("git branch = %q, want %q", got, "feature/maggus-038/task-003")
	}
}

func TestEnsureTaskBranchFromBase_ExistingBranch(t *testing.T) {
	// Note: the task branch is created directly off master to avoid the git ref
	// conflict that would occur if "feature/maggus-038" (plan branch) and
	// "feature/maggus-038/task-003" (task branch) both existed as refs.
	tmp := t.TempDir()
	initGitRepoWithBranch(t, tmp, "master")
	checkoutBranch(t, tmp, "feature/maggus-038/task-003")

	cmd := exec.Command("git", "checkout", "master")
	cmd.Dir = tmp
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("checkout master: %v\n%s", err, out)
	}

	branch, _, err := EnsureTaskBranchFromBase(tmp, "TASK-038-003", "master")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if branch != "feature/maggus-038/task-003" {
		t.Errorf("branch = %q, want %q", branch, "feature/maggus-038/task-003")
	}
	if got := getCurrentBranch(t, tmp); got != "feature/maggus-038/task-003" {
		t.Errorf("git branch = %q, want %q", got, "feature/maggus-038/task-003")
	}
}

func TestEnsureTaskBranchFromBase_BranchCreatedOffBase(t *testing.T) {
	// Verify the task branch is created at baseBranch's HEAD.
	// Note: we use a non-conflicting base ("feature/base-038") because git cannot
	// have refs/heads/feature/maggus-038 and refs/heads/feature/maggus-038/task-003
	// as refs simultaneously.
	tmp := t.TempDir()
	initGitRepoWithBranch(t, tmp, "master")
	checkoutBranch(t, tmp, "feature/base-038")
	addEmptyCommit(t, tmp, "base branch commit")

	baseHead := getBranchHead(t, tmp, "feature/base-038")
	masterHead := getBranchHead(t, tmp, "master")
	if baseHead == masterHead {
		t.Fatal("test setup error: base branch not ahead of master")
	}

	_, _, err := EnsureTaskBranchFromBase(tmp, "TASK-038-003", "feature/base-038")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	taskHead := getBranchHead(t, tmp, "feature/maggus-038/task-003")
	if taskHead != baseHead {
		t.Errorf("task branch HEAD %q != base branch HEAD %q; not based on base branch", taskHead, baseHead)
	}
}

func TestEnsureTaskBranchFromBase_BugTask(t *testing.T) {
	// Note: we use master as base to avoid the git ref conflict between
	// "bugfix/maggus-bug-001" (plan branch) and "bugfix/maggus-bug-001/task-003" (task branch).
	tmp := t.TempDir()
	initGitRepoWithBranch(t, tmp, "master")

	branch, msg, err := EnsureTaskBranchFromBase(tmp, "BUG-001-003", "master")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if branch != "bugfix/maggus-bug-001/task-003" {
		t.Errorf("branch = %q, want %q", branch, "bugfix/maggus-bug-001/task-003")
	}
	if msg == "" {
		t.Error("expected non-empty message")
	}
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func addEmptyCommit(t *testing.T, dir, msg string) {
	t.Helper()
	cmd := exec.Command("git", "commit", "--allow-empty", "-m", msg)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("commit: %v\n%s", err, out)
	}
}

func getBranchHead(t *testing.T, dir, branch string) string {
	t.Helper()
	cmd := exec.Command("git", "rev-parse", branch)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("rev-parse %s: %v", branch, err)
	}
	return strings.TrimSpace(string(out))
}
