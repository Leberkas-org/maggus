# Bug: Work command flags persist between menu-driven runs, causing "no tasks" after --task

## Summary

After running a specific task via `maggus work --task TASK-ID` (either from the status screen or CLI), all subsequent `maggus work` runs (count=0) immediately exit with "Task TASK-ID not found or already complete" — even though open tasks exist. Restarting maggus clears the issue. The `taskFlag` package variable retains its value between menu-driven command invocations.

## Related

- **File:** `src/cmd/dispatch.go` (dispatchWork sets --task flag)
- **File:** `src/cmd/root.go:65-74` (menu loop reuses cobra commands without resetting flags)

## Steps to Reproduce

1. Run `maggus` (opens main menu)
2. Select "status" to open the interactive status view
3. Navigate to an open task and press Alt+R to run it
4. `dispatchWork(taskID)` is called at `status.go:701`, which parses `--task TASK-ID` on the work cobra command
5. Wait for the task to complete — control returns to status, then back to the menu
6. Select "work" from the menu (default, count=0 meaning "all tasks")
7. Observe: work exits immediately — prints "Task TASK-ID not found or already complete"
8. Restart maggus → "work" works normally again

## Expected Behavior

After completing a `--task` run, returning to the menu and selecting "work" should process all open tasks as normal, ignoring the previous `--task` value.

## Root Cause

The work command flags are declared as package-level variables at `src/cmd/work.go:31-39`:

```go
var (
    countFlag       int
    noBootstrapFlag bool
    modelFlag       string
    agentFlag       string
    taskFlag        string      // <-- persists between runs
    worktreeFlag    bool
    noWorktreeFlag  bool
)
```

When the menu loop at `src/cmd/root.go:65-74` dispatches a command, it calls `sub.ParseFlags(remaining)` on the same cobra command instance. Cobra's `ParseFlags` only updates flags that are present in the argument list — **flags not in the new args retain their previous values**.

The chain of events:

1. `dispatchWork("TASK-001")` at `dispatch.go:5` parses `["work", "--task", "TASK-001"]` → sets `taskFlag = "TASK-001"`
2. Work completes, returns to menu
3. Menu selects "work" with args `["work", "--count", "999"]` at `root.go:65-70`
4. `ParseFlags` sees `--count 999` but no `--task` → **`taskFlag` stays "TASK-001"**
5. `findInitialTask()` at `work_loop.go:107` sees `taskFlag != ""`, looks for "TASK-001"
6. TASK-001 is already complete → returns nil → "not found or already complete"
7. Loop exits with nothing done

The same issue affects all work command flags (`modelFlag`, `agentFlag`, `worktreeFlag`, etc.) — they all persist between menu-driven runs. The `taskFlag` is just the most visible because it causes an immediate exit.

## User Stories

### BUG-003-001: Reset work command flags before each menu-driven invocation

**Description:** As a user, I want work command flags to be reset to defaults between menu-driven runs so that a previous `--task` run doesn't poison subsequent `work` invocations.

**Acceptance Criteria:**
- [x] All work command flag variables (`taskFlag`, `countFlag`, `modelFlag`, `agentFlag`, `noBootstrapFlag`, `worktreeFlag`, `noWorktreeFlag`) are reset to their zero/default values before `ParseFlags` in both `root.go:70` (menu loop) and `dispatch.go:9` (dispatchWork)
- [x] After running `maggus work --task TASK-ID` from the status screen, selecting "work" from the menu processes all open tasks normally
- [x] After running `maggus work --task TASK-ID` from the menu, selecting "work" again processes all open tasks normally
- [x] Explicit flags passed in a new invocation still take effect (e.g. `--model opus` still works)
- [x] The `defaultTaskCount` constant is used when resetting `countFlag` (not hardcoded 0)
- [x] No regression in direct CLI usage (`maggus work --task X` from shell)
- [x] No regression in menu-driven work with explicit count
- [x] `go vet ./...` and `go test ./...` pass
