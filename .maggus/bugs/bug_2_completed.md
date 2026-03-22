# Bug: Work loop ignores new tasks detected by file watcher when using default count

## Summary

When running `maggus work` (no explicit count), the file watcher correctly detects new feature/bug files mid-run and updates the TUI progress bar, but the actual work loop exits after completing the original number of workable tasks. New tasks are displayed in the progress bar but never executed.

## Related

- **Commit:** 0437be2 (Live progress bar and bug hint updates on file changes, TASK-004-002)
- **Commit:** 25aefb4 (Integrated file watcher into work TUI, TASK-004-001)

## Steps to Reproduce

1. Create a feature file with 2 tasks in `.maggus/features/`
2. Run `maggus work` (default count — resolves to 2 via `capCount`)
3. While iteration 1 is running, drop a new feature file with 3 tasks into `.maggus/features/`
4. Observe: TUI progress bar updates from "1/2" to "1/5" (correct)
5. Observe: after iteration 2 completes, the loop exits — tasks 3, 4, 5 are never processed

## Expected Behavior

When using `maggus work` (count=0, meaning "all"), the loop should continue processing tasks as long as workable tasks exist, including tasks from files added mid-run. When using an explicit count (`maggus work 3`), the original limit should be respected.

## Root Cause

The iteration count is frozen at startup and never updated when the file watcher detects new tasks.

**`src/cmd/work_loop.go:150-164`** — `capCount()` computes the workable task count once at init:

```go
func capCount(tasks []parser.Task, count int) int {
    // ...
    workable := 0
    for i := range tasks {
        if tasks[i].IsWorkable() {
            workable++
        }
    }
    if count <= 0 || workable < count {
        return workable  // frozen value
    }
    return count
}
```

**`src/cmd/work_loop.go:257`** — the main loop uses this frozen count:

```go
for i := 0; i < params.count; i++ {
```

`params.count` is never updated during the loop. The TUI side (`src/internal/runner/tui_messages.go:132`) correctly updates `m.totalIters` on file changes, but this is purely cosmetic — the actual loop bound in `runWorkGoroutine` is disconnected from the TUI state.

The disconnect is between two systems:
1. **TUI layer** (`tui_messages.go:105-145`): re-parses tasks on file change, updates `totalIters` = `currentIter + remaining workable`
2. **Loop layer** (`work_loop.go:257`): iterates `i < params.count` which was set once at startup

There is no feedback path from the TUI's file-change handler back to the loop's iteration bound.

## User Stories

### BUG-002-001: Make default-count work loop run until tasks are exhausted

**Description:** As a user, I want `maggus work` (no explicit count) to keep processing tasks as long as workable tasks exist, so that new tasks added mid-run via file watcher are automatically picked up and executed.

**Acceptance Criteria:**
- [x] When count is 0 (default/"all"), the loop continues as long as `findNextWorkableTask` returns a task, rather than using a fixed iteration limit
- [x] When an explicit count is provided (`maggus work 3`), the original limit is still respected — the loop stops after exactly N tasks regardless of new files
- [x] The `--task TASKID` flag still runs exactly 1 task
- [x] Progress bar total in the TUI stays accurate (reflects actual remaining workable tasks)
- [x] The stop command (`Alt+S`) still works correctly in both modes
- [x] The "stop at task" variant (`Alt+S` with task selection) still works correctly
- [x] No regression in summary screen (completed count, remaining tasks list)
- [x] `go vet ./...` and `go test ./...` pass
