<!-- maggus-id: 69277adf-0ca0-4caf-8f28-6d316cf8db44 -->
# Bug: Parallel tasks stuck in boot loop — git checkout fails when branch is in worktree

## Summary

Every parallel task fails immediately at startup with a git error, causing the daemon to loop endlessly. No agent is ever launched. The root cause is `RunTaskWorker` unconditionally calling `EnsureTaskBranchFromBase` in the main repo, but the task branch is already checked out in a worktree at that point — git rejects the checkout.

## Related

- **Feature:** feature_052.md (TASK-052-002: Add parallel task dispatch to orchestrator)

## Steps to Reproduce

1. Have a feature with tasks marked `**Parallel:** yes` (e.g. TASK-054-002 and TASK-054-003 in `feature_054.md`)
2. Approve the feature (`maggus approve`)
3. Start the daemon (`maggus start`)
4. Observe the daemon entering a continuous restart loop — workers appear to "boot" but never run the agent

## Expected Behavior

Parallel tasks run concurrently in isolated git worktrees. The agent is invoked, commits are made, and results are merged back into the plan branch.

## Root Cause

`runWorktreeTask` (`src/cmd/orchestrator_parallel.go:202`) creates a task branch and worktree before calling `RunTaskWorker`:

```go
// Creates branch e.g. "feature/feat-054/task-002" pointing at plan branch
gitbranch.CreateBranchFrom(cfg.RepoDir, taskBranch, cfg.PlanBranch)

// Creates worktree at .maggus/worktrees/TASK-054-002 checked out on that branch
gitworktree.CreateWorktree(cfg.RepoDir, worktreePath, taskBranch)

// Worker called with WorkDir=worktreePath, RepoDir=mainRepo, PlanBranch=planBranch
RunTaskWorker(WorkerConfig{..., WorkDir: worktreePath, PlanBranch: cfg.PlanBranch, ...})
```

Then `RunTaskWorker` (`src/cmd/worker.go:138-141`) unconditionally calls:

```go
if cfg.PlanBranch != "" {
    if _, _, err := gitbranch.EnsureTaskBranchFromBase(cfg.RepoDir, cfg.Task.ID, cfg.PlanBranch); err != nil {
        return workerFail(&result, cfg, fmt.Sprintf("create task branch: %v", err))
    }
}
```

`EnsureTaskBranchFromBase` (`src/internal/gitbranch/gitbranch_parallel.go:155-161`) sees the branch already exists and skips creating it, but then runs:

```go
cmd := gitutil.Command("checkout", target)  // "feature/feat-054/task-002"
cmd.Dir = workDir                           // = cfg.RepoDir = MAIN REPO
```

This `git checkout` in the main repo fails:

```
fatal: 'feature/feat-054/task-002' is already checked out at '<worktree path>'
```

Git refuses to check out a branch that is already live in another worktree.

`RunTaskWorker` returns a failure, `runWorktreeTask` marks the task as `failedIDs[task.ID] = true`, and the orchestrator exits without running any agent. **On the next daemon cycle, `failedIDs` is reset** (a new `Orchestrator` is created in `runOneDaemonCycle`), the tasks appear workable again, and the identical failure repeats — creating the boot loop.

## User Stories

### BUG-044-001: Skip EnsureTaskBranchFromBase when running in worktree mode

**Description:** As a user, I want parallel tasks to actually start the agent so that they can be worked on concurrently.

**Acceptance Criteria:**
- [x] `RunTaskWorker` skips the `EnsureTaskBranchFromBase` call when `cfg.WorkDir != cfg.RepoDir` (i.e. when running inside a git worktree, not the main repo)
- [x] Parallel tasks TASK-054-002 and TASK-054-003 launch without a git checkout error
- [x] The agent is invoked in each worktree and can commit its work
- [x] Sequential tasks (non-worktree) still call `EnsureTaskBranchFromBase` as before — no regression
- [x] Orphaned task branches from previous failed cycles are cleaned up by `runWorktreeTask` on the failure path (branch created but checkout failed → add `gitbranch.DeleteBranch` to the failure path in `runWorktreeTask`)
- [x] `go vet ./...` and `go test ./...` pass
