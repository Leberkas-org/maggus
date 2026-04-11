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
		{"038", "feature/maggus-038"},
		{"001", "feature/maggus-001"},
		{"100", "feature/maggus-100"},
		{"000", "feature/maggus-000"},
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
		{"001", "bugfix/maggus-bug-001"},
		{"042", "bugfix/maggus-bug-042"},
		{"000", "bugfix/maggus-bug-000"},
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
		{"TASK-038-003", "feature/maggus-038"},
		{"TASK-001-001", "feature/maggus-001"},
		{"TASK-038", "feature/maggus-038"},
		{"BUG-001-003", "bugfix/maggus-bug-001"},
		{"BUG-042-001", "bugfix/maggus-bug-042"},
		{"INVALID", "feature/maggus-000"},
		{"", "feature/maggus-000"},
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
	}
	for _, tt := range tests {
		got := IsPlanBranch(tt.branch)
		if got != tt.want {
			t.Errorf("IsPlanBranch(%q) = %v, want %v", tt.branch, got, tt.want)
		}
	}
}

// ── EnsurePlanBranch integration tests ───────────────────────────────────────

func TestEnsurePlanBranch_AlreadyOnPlanBranch(t *testing.T) {
	tmp := t.TempDir()
	initGitRepoWithBranch(t, tmp, "master")
	checkoutBranch(t, tmp, "feature/maggus-038")

	branch, msg, err := EnsurePlanBranch(tmp, "TASK-038-003")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if branch != "feature/maggus-038" {
		t.Errorf("branch = %q, want %q", branch, "feature/maggus-038")
	}
	if msg == "" {
		t.Error("expected non-empty message")
	}
	if got := getCurrentBranch(t, tmp); got != "feature/maggus-038" {
		t.Errorf("git branch = %q, want %q", got, "feature/maggus-038")
	}
}

func TestEnsurePlanBranch_OnProtectedBranch(t *testing.T) {
	tmp := t.TempDir()
	initGitRepoWithBranch(t, tmp, "master")

	branch, msg, err := EnsurePlanBranch(tmp, "TASK-038-003")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if branch != "feature/maggus-038" {
		t.Errorf("branch = %q, want %q", branch, "feature/maggus-038")
	}
	if msg == "" {
		t.Error("expected non-empty message")
	}
	if got := getCurrentBranch(t, tmp); got != "feature/maggus-038" {
		t.Errorf("git branch = %q, want %q", got, "feature/maggus-038")
	}
}

func TestEnsurePlanBranch_OnOtherBranch(t *testing.T) {
	tmp := t.TempDir()
	initGitRepoWithBranch(t, tmp, "master")
	checkoutBranch(t, tmp, "feature/other-work")

	branch, msg, err := EnsurePlanBranch(tmp, "TASK-038-003")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if branch != "feature/maggus-038" {
		t.Errorf("branch = %q, want %q", branch, "feature/maggus-038")
	}
	if msg == "" {
		t.Error("expected non-empty message")
	}
	if got := getCurrentBranch(t, tmp); got != "feature/maggus-038" {
		t.Errorf("git branch = %q, want %q", got, "feature/maggus-038")
	}
}

func TestEnsurePlanBranch_ExistingPlanBranch_SwitchesBack(t *testing.T) {
	// Plan branch was created earlier; we went back to master and now re-enter.
	tmp := t.TempDir()
	initGitRepoWithBranch(t, tmp, "master")
	checkoutBranch(t, tmp, "feature/maggus-038")

	cmd := exec.Command("git", "checkout", "master")
	cmd.Dir = tmp
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("checkout master: %v\n%s", err, out)
	}

	branch, msg, err := EnsurePlanBranch(tmp, "TASK-038-003")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if branch != "feature/maggus-038" {
		t.Errorf("branch = %q, want %q", branch, "feature/maggus-038")
	}
	if msg == "" {
		t.Error("expected non-empty message")
	}
	if got := getCurrentBranch(t, tmp); got != "feature/maggus-038" {
		t.Errorf("git branch = %q, want %q", got, "feature/maggus-038")
	}
}

func TestEnsurePlanBranch_NotAGitRepo(t *testing.T) {
	tmp := t.TempDir() // no git init

	branch, msg, err := EnsurePlanBranch(tmp, "TASK-038-003")
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

func TestEnsurePlanBranch_BugTask(t *testing.T) {
	tmp := t.TempDir()
	initGitRepoWithBranch(t, tmp, "master")

	branch, msg, err := EnsurePlanBranch(tmp, "BUG-001-003")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if branch != "bugfix/maggus-bug-001" {
		t.Errorf("branch = %q, want %q", branch, "bugfix/maggus-bug-001")
	}
	if msg == "" {
		t.Error("expected non-empty message")
	}
	if got := getCurrentBranch(t, tmp); got != "bugfix/maggus-bug-001" {
		t.Errorf("git branch = %q, want %q", got, "bugfix/maggus-bug-001")
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
