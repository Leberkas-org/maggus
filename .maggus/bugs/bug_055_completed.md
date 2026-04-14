<!-- maggus-id: fbcec0f0-7e7d-474a-8b1e-204fb7107e2f -->
# Bug: MergeTaskBranch fails when task branch is checked out in a worktree

## Summary

When a previously-blocked (or any parallel) task is unblocked and runs in a git worktree, the merge step crashes with `fatal: 'feature/maggus-NNN/task-MMM' is already checked out at '.maggus/worktrees/TASK-NNN-MMM'`. The merge never completes and the task is marked as failed.

## Steps to Reproduce

1. Run maggus on a project with at least one parallel task
2. Let a task run in its worktree (normal parallel execution)
3. Observe the merge step after the agent finishes:
   ```
   merge: checkout feature/maggus-016/task-002: exit status 128:
   fatal: 'feature/maggus-016/task-002' is already checked out at
   'C:/diit/waire.devicehub/.maggus/worktrees/TASK-016-002'
   ```

## Expected Behavior

The task branch should be rebased onto the plan branch and fast-forwarded successfully, without trying to check it out again in the main repo.

## Root Cause

`gitmerge.MergeTaskBranch` (`src/internal/gitmerge/gitmerge.go:33`) is called with `cfg.RepoDir` as its working directory from `workerMerge` in `src/cmd/worker.go:229`:

```go
func workerMerge(cfg WorkerConfig, taskBranch string) error {
    ...
    return gitmerge.MergeTaskBranch(cfg.RepoDir, cfg.PlanBranch, taskBranch)
}
```

Its first step is `checkout(repoRoot, taskBranch)` at `gitmerge.go:35`, which runs:

```
git checkout feature/maggus-016/task-002
```

…in the **main repo**. But for parallel tasks the orchestrator already checked that branch out in a worktree at `.maggus/worktrees/TASK-016-002` (`orchestrator_parallel.go:302`). Git forbids a branch from being active in two places at once, so the checkout exits 128.

The rebase command that follows (`gitmerge.go:39`) also sets `rebaseCmd.Dir = repoRoot`, so even if the checkout were somehow skipped, the rebase would run against the wrong working tree.

The fix: before attempting `checkout(repoRoot, taskBranch)`, `MergeTaskBranch` should call `gitworktree.ListWorktrees(repoRoot)` to check whether the task branch is already active in a registered worktree. If it is, skip the checkout and use that worktree's path as the working directory for the rebase. The feature branch checkout and ff-merge continue from `repoRoot` as today. No signature change is needed; no callers need updating.

## User Stories

### BUG-055-001: Auto-detect worktree in MergeTaskBranch so parallel merges work

**Description:** As a maggus user, I want parallel tasks to merge successfully after the agent finishes, so that completed work is integrated into the plan branch without errors.

**Acceptance Criteria:**
- [x] At the start of `MergeTaskBranch`, `gitworktree.ListWorktrees(repoRoot)` is called to find any worktree where `taskBranch` is already checked out
- [x] If such a worktree is found, the `checkout(repoRoot, taskBranch)` call is skipped and `rebaseCmd.Dir` is set to that worktree's path
- [x] If no such worktree is found, behavior is unchanged — checkout the task branch in `repoRoot` and rebase there
- [x] `handleRebaseFailure` uses the same resolved directory (worktree path or `repoRoot`) for REBASE_HEAD checks and abort
- [x] The signature of `MergeTaskBranch` is unchanged; no callers need updating
- [x] A parallel task that completes in a worktree merges cleanly into the plan branch without the "already checked out" error
- [x] No regression in sequential (non-worktree) task execution
- [x] `go vet ./...` and `go test ./...` pass
