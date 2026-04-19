package git

import (
	"fmt"
	"strings"
	"testing"
)

type mockCmd struct {
	runs    []string
	outputs map[string]string
	errors  map[string]error
}

func newMockCmd() *mockCmd {
	return &mockCmd{
		outputs: make(map[string]string),
		errors:  make(map[string]error),
	}
}

func (m *mockCmd) key(args []string) string {
	return strings.Join(args, " ")
}

func (m *mockCmd) Run(_ string, args ...string) error {
	m.runs = append(m.runs, m.key(args))
	if err, ok := m.errors[m.key(args)]; ok {
		return err
	}
	return nil
}

func (m *mockCmd) Output(_ string, args ...string) (string, error) {
	m.runs = append(m.runs, m.key(args))
	if err, ok := m.errors[m.key(args)]; ok {
		return "", err
	}
	if out, ok := m.outputs[m.key(args)]; ok {
		return out, nil
	}
	return "", nil
}

func (m *mockCmd) CombinedOutput(_ string, args ...string) (string, error) {
	return m.Output("", args...)
}

func TestCurrentBranch(t *testing.T) {
	cmd := newMockCmd()
	cmd.outputs["rev-parse --abbrev-ref HEAD"] = "feature/test"
	o := New(cmd, nil)

	branch, err := o.CurrentBranch("/repo")
	if err != nil {
		t.Fatal(err)
	}
	if branch != "feature/test" {
		t.Errorf("got %q, want %q", branch, "feature/test")
	}
}

func TestBranchExists(t *testing.T) {
	cmd := newMockCmd()
	cmd.errors["rev-parse --verify refs/heads/nope"] = fmt.Errorf("not found")
	o := New(cmd, nil)

	if !o.BranchExists("/repo", "main") {
		t.Error("expected main to exist (no error)")
	}
	if o.BranchExists("/repo", "nope") {
		t.Error("expected nope to not exist")
	}
}

func TestCreateBranch_Protected(t *testing.T) {
	cmd := newMockCmd()
	o := New(cmd, []string{"main", "master"})

	if err := o.CreateBranch("/repo", "main", "HEAD"); err == nil {
		t.Error("expected error for protected branch")
	}
	if err := o.CreateBranch("/repo", "feature/new", "HEAD"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDeleteBranch_Protected(t *testing.T) {
	cmd := newMockCmd()
	o := New(cmd, []string{"main"})

	if err := o.DeleteBranch("/repo", "main"); err == nil {
		t.Error("expected error for protected branch")
	}
}

func TestIsProtected(t *testing.T) {
	o := New(newMockCmd(), []string{"main", "master", "dev"})
	if !o.IsProtected("main") {
		t.Error("main should be protected")
	}
	if o.IsProtected("feature/x") {
		t.Error("feature/x should not be protected")
	}
}

func TestHasChanges(t *testing.T) {
	cmd := newMockCmd()
	o := New(cmd, nil)

	cmd.outputs["status --porcelain"] = ""
	if o.HasChanges("/repo") {
		t.Error("no output = no changes")
	}

	cmd.outputs["status --porcelain"] = " M file.go"
	if !o.HasChanges("/repo") {
		t.Error("output = has changes")
	}
}

func TestCommit(t *testing.T) {
	cmd := newMockCmd()
	cmd.outputs["rev-parse HEAD"] = "abc123"
	o := New(cmd, nil)

	hash, err := o.Commit("/repo", "test commit")
	if err != nil {
		t.Fatal(err)
	}
	if hash != "abc123" {
		t.Errorf("hash = %q, want abc123", hash)
	}
}

func TestListWorktrees(t *testing.T) {
	cmd := newMockCmd()
	cmd.outputs["worktree list --porcelain"] = "worktree /repo\nbranch refs/heads/main\n\nworktree /repo/.maggus/worktrees/TASK-001\nbranch refs/heads/feature/F-12/TASK-001\n"
	o := New(cmd, nil)

	trees, err := o.ListWorktrees("/repo")
	if err != nil {
		t.Fatal(err)
	}
	if len(trees) != 2 {
		t.Fatalf("got %d worktrees, want 2", len(trees))
	}
	if trees[0].Branch != "main" {
		t.Errorf("tree[0].Branch = %q, want main", trees[0].Branch)
	}
	if trees[1].Branch != "feature/F-12/TASK-001" {
		t.Errorf("tree[1].Branch = %q, want feature/F-12/TASK-001", trees[1].Branch)
	}
}

func TestRemoteExists(t *testing.T) {
	cmd := newMockCmd()
	o := New(cmd, nil)

	cmd.outputs["remote"] = ""
	if o.RemoteExists("/repo") {
		t.Error("empty output = no remote")
	}

	cmd.outputs["remote"] = "origin"
	if !o.RemoteExists("/repo") {
		t.Error("origin output = remote exists")
	}
}

func TestDefaultBranch_Fallback(t *testing.T) {
	cmd := newMockCmd()
	cmd.errors["symbolic-ref refs/remotes/origin/HEAD"] = fmt.Errorf("not set")
	// main exists
	o := New(cmd, nil)

	branch, err := o.DefaultBranch("/repo")
	if err != nil {
		t.Fatal(err)
	}
	if branch != "main" {
		t.Errorf("got %q, want main", branch)
	}
}
