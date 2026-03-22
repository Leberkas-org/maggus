# Bug: Progress bar shrinks and last task skipped when files added during active run

## Summary

When new feature or bug files are added while `maggus work` is running, the progress bar briefly shows the correct higher total then shrinks back to the stale count. In some timing windows, the newly-added tasks are never worked on and the run exits prematurely.

## Steps to Reproduce

1. Run `maggus work` with at least 2 workable tasks
2. While a task is being worked on, add a new feature or bug file with workable tasks
3. Observe: progress bar jumps to a higher total (e.g. "1/5") then shrinks back (e.g. "1/3") once the current task completes
4. To reproduce the premature exit: add a new file in the brief window after the last task's agent finishes but before the next loop iteration begins — the run exits without working on the new task

## Expected Behavior

- Progress bar should stabilise at the highest observed total and never shrink
- All workable tasks discovered during the run (including newly-added files) should be worked on before the run exits

## Root Cause

Two separate defects in the work loop / TUI interaction:

**Defect 1 — progress bar shrinks (work_task.go:186)**

`completeTask` sends `ProgressMsg{Current: i+1, Total: count}` where `count` was computed as `displayCount` at the **top** of the current loop iteration (before `runTask` executed):

```go
// work_loop.go:280-289
displayCount = i + remaining  // computed from `tasks` snapshot BEFORE agent runs

result := runTask(params.tc, tasks, i, displayCount)
// ...
// inside completeTask (work_task.go:186):
tc.p.Send(runner.ProgressMsg{Current: i + 1, Total: count})  // count == old displayCount
```

`runTask` calls `parseAllTasks` **after** the agent runs (`work_task.go:115`), so `result.tasks` already includes the new files. But `ProgressMsg` still uses the pre-agent `count`. When the TUI receives this, it overwrites the file-watcher-updated `totalIters` with the stale value, causing the visible shrink.

The bar corrects itself on the next `IterationStartMsg` (which recomputes `displayCount` from the refreshed `tasks`), creating a bounce on every task completion while new files exist.

**Defect 2 — premature exit (work_loop.go:284-287)**

The unlimited-mode exit guard:

```go
remaining := countWorkable(tasks)
if remaining == 0 {
    break
}
```

uses `tasks` from the **previous iteration's** `parseAllTeset` result. If the user adds a new file in the window **after** the last `parseAllTasks` call returns (all original tasks done, `remaining = 0`) but **before** the loop checks this condition, the loop breaks without ever discovering the new tasks. The file watcher fires asynchronously and shows the new tasks in the TUI header, but the work goroutine has already decided to exit.

## User Stories

### BUG-005-001: Fix progress bar shrink by computing ProgressMsg total from refreshed task list

**Description:** As a user, I want the progress bar to reflect the true remaining task count so it never shrinks when new files are added during a run.

**Acceptance Criteria:**
- [x] In `completeTask` (`work_task.go:186`), compute the `ProgressMsg` total as `(i + 1) + countWorkable(parsedTasks)` instead of `count`
- [x] In bounded mode (non-unlimited), cap the new total at `params.count` so it does not exceed the user-requested limit
- [x] Progress bar never decreases when new files are added during an active unlimited run
- [x] Progress bar still counts up correctly when no new files are added
- [x] No regression in bounded mode (`maggus work N`) progress display
- [x] `go vet ./...` and `go test ./...` pass

### BUG-005-002: Fix premature exit when files added after last parseAllTasks

**Description:** As a user, I want `maggus work` to pick up newly-added tasks even if they arrive in the window just after the last task's re-parse, so no workable task is ever silently skipped.

**Acceptance Criteria:**
- [ ] When `remaining == 0` in the unlimited loop, do a fresh `parseAllTasks` re-parse before breaking to confirm no new tasks appeared since the last iteration
- [ ] If the fresh re-parse finds workable tasks, update `tasks` and continue the loop instead of breaking
- [ ] If the fresh re-parse also finds `remaining == 0`, break normally
- [ ] Tasks added by the user while the last original task is executing are worked on in the same run
- [ ] No regression in normal exit behaviour when all tasks are genuinely complete
- [ ] `go vet ./...` and `go test ./...` pass
