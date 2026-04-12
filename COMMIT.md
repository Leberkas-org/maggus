feat(TASK-051-002, TASK-051-003): Rewire sequential and parallel daemons to use unified worker

TASK-051-002 (sequential):
- `runTask` in `run_task.go` now calls `RunTaskWorker` for the complete task
  lifecycle (branch → agent → pre-commit ops → commit → merge-back → cleanup)
- Added `PreCommit func(workDir string)` callback to `WorkerConfig` so callers
  can inject pre-commit operations (MarkCompleted, hooks, git-add .maggus/)
  without the worker needing to know about them
- Added `buildPreCommitFn(tc taskContext)` helper for sequential task execution
- Removed `completeTask` function (worker now owns commit+post-commit)
- Added `planBranch string` to `taskContext` and `runLoopParams`; threaded from
  `daemon_keepalive.go` → `runLoopParams` → `tc.planBranch` → `RunTaskWorker`
- Sequential mode calls `EnsurePlanBranch` instead of `setupBranch`; resolves
  bug_041 (sequential tasks now always merge back and delete their task branch)

TASK-051-003 (parallel):
- `runSingleTask` in `run_parallel.go` replaced with calls to `RunTaskWorker`
- Parallel concurrency and worktree management remain in the orchestrator
- Merge serialization (`o.mu.Lock()`) preserved via `MergeMu` passed to worker
- `MergeTaskBranch` switched from merge-commit to rebase+fast-forward for linear history
- Removed `spinnerTicking` field from `statusModel`; tick loop now always runs

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>
