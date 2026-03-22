# Bug: Infinite work loop stops after initial feature file and progress bar fluctuates

## Summary

When running with auto-work enabled, the work loop exits after processing the tasks from the initial feature file instead of continuing with newly added feature files. Additionally, the progress bar grows and shrinks spuriously whenever a file change is detected during task execution.

## Steps to Reproduce

**Progress bar grow/shrink:**
1. Run `maggus work` with unlimited mode (no count flag) while feature files exist
2. While a task is running, add a new feature file (or have the agent create one)
3. Observe: the progress bar total increases by `N+1` instead of `N`
4. When the current task completes, the total snaps back down by 1

**Stops after initial feature file:**
1. Enable auto-work in config (enabled or delayed)
2. Drop a new feature file into `.maggus/features/`
3. Auto-work fires: maggus starts and processes all tasks from the initial feature file
4. During the run, or immediately after, a second feature file exists/appears
5. Observe: maggus stops at the summary screen after the first feature file — it does not continue to the second file

## Expected Behavior

- Progress bar total should increase by exactly `N` when `N` new tasks are detected
- In unlimited / auto-work mode, maggus should continue processing new feature files until no workable tasks remain

## Root Cause

### Bug 1 — Progress bar off-by-one (tui_messages.go:132)

`handleFileChange()` computes the total as:

```go
m.totalIters = m.currentIter + workableBugs + workableFeatures
```

`currentIter` is 1-based (e.g. `2` when running the second task). `workable` counts ALL remaining tasks including the currently-executing one (not yet committed). So both the current task and the "current position" counter are summed, inflating the total by 1.

The rest of the codebase uses the invariant `total = i + remaining` where `i` is 0-based and `remaining` includes the current task. Expressed in terms of `currentIter`: `total = (currentIter - 1) + workable`.

After the task completes, `ProgressMsg` is sent with `progressTotal = (i+1) + countWorkable(parsedTasks)` where `parsedTasks` has the just-completed task marked done — so the total is 1 less than what `handleFileChange` had set. This is the shrink.

### Bug 2 — Auto-work dispatches `--count 999` instead of `--count 0` (menu.go:430, 454)

The auto-work trigger in the menu dispatches the work command with `--count 999`:

```go
// menu.go:430
m.args = []string{"--count", "999"}
// menu.go:454
m.args = []string{"--count", "999"}
```

In `work.go`, the unlimited flag is set as:

```go
unlimited: wc.count == 0 && taskFlag == "",
```

Because `count = 999 ≠ 0`, `unlimited = false`. Then `capCount()` limits `loopCount` to `countWorkable(initialTasks)` (e.g., 3 if there are 3 tasks). The loop runs exactly 3 iterations and exits, regardless of any new feature files added during the run.

With `--count 0`, `unlimited = true` and the loop re-parses after each task, picking up newly added feature files until no workable tasks remain.

## User Stories

### BUG-001-001: Fix progress bar off-by-one in handleFileChange

**Description:** As a user, I want the progress bar to show a stable total that increases correctly when new tasks are detected, without flickering up then back down.

**Acceptance Criteria:**
- [x] `handleFileChange` uses `m.totalIters = (m.currentIter - 1) + workableBugs + workableFeatures`
- [x] Adding N new tasks while a task is executing increases `totalIters` by exactly N
- [x] `totalIters` does not decrease when the current task completes (progress only moves forward)
- [x] No regression in file-watcher notification logic
- [x] `go vet ./...` and `go test ./...` pass

### BUG-001-002: Fix auto-work to use unlimited mode (--count 0)

**Description:** As a user with auto-work enabled, I want maggus to continue processing all feature files that exist or are added during a run, not just the ones present at the start.

**Acceptance Criteria:**
- [ ] Auto-work dispatches `--count 0` (or omits `--count`) instead of `--count 999`
- [ ] When auto-work fires, the work loop continues to pick up new feature files added during the run
- [ ] After processing all workable tasks, the loop terminates normally and returns to the menu
- [ ] Both `AutoWorkEnabled` and `AutoWorkDelayed` paths use the corrected count
- [ ] No regression in menu-driven dispatch behavior
- [ ] `go vet ./...` and `go test ./...` pass
