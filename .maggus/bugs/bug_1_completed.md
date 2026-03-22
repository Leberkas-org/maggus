# Bug: EnsureFeatureBranch fails when target branch already exists

## Summary

`maggus work` crashes with an error when the target feature branch already exists (e.g., from a previous run). `createAndCheckout` uses `git checkout -b` which fails if the branch is already present, instead of switching to the existing branch.

## Steps to Reproduce

1. Run `maggus work` on a project with workable tasks while on a protected branch (e.g., `master`)
2. Let it create a feature branch (e.g., `feature/maggustask-005`) and complete at least one iteration
3. Switch back to `master` manually (`git checkout master`)
4. Run `maggus work` again — the next workable task still maps to the same branch name
5. Observe: maggus exits with error `ensure feature branch: create feature branch feature/maggustask-005: exit status 128: fatal: a branch named 'feature/maggustask-005' already exists`

## Expected Behavior

If the target branch already exists, maggus should check it out (`git checkout <branch>`) instead of trying to create it (`git checkout -b <branch>`). The work loop should continue normally.

## Root Cause

`createAndCheckout` in `src/internal/gitbranch/gitbranch.go:75-82` unconditionally runs `git checkout -b <branch>`:

```go
func createAndCheckout(dir string, branch string) error {
	cmd := exec.Command("git", "checkout", "-b", branch)
	// ...
}
```

The `-b` flag tells git to create a new branch and fails with exit code 128 if a branch with that name already exists. There is no pre-check for branch existence and no fallback to a plain `git checkout`.

The caller `EnsureFeatureBranch` (line 58) propagates this as a hard error, which causes `work_loop.go:473` to abort the work loop.

## User Stories

### BUG-001-001: Handle existing branch in createAndCheckout

**Description:** As a user, I want `maggus work` to switch to an existing feature branch instead of failing, so that I can restart work without manually checking out branches.

**Acceptance Criteria:**
- [x] `createAndCheckout` checks if the target branch already exists before creating it
- [x] If the branch exists, it runs `git checkout <branch>` (without `-b`)
- [x] If the branch does not exist, it runs `git checkout -b <branch>` (current behavior)
- [x] Add test: `EnsureFeatureBranch` succeeds when called twice for the same task on a protected branch
- [x] Add test: `EnsureFeatureBranch` switches to existing branch and returns correct branch name and message
- [x] No regression in existing branch creation behavior
- [x] `go vet ./...` and `go test ./...` pass
