<!-- maggus-id: 7ea1558e-2b70-4f19-bc1d-e0ab09eb7e2c -->
# Bug: Daemon loops forever showing "Preparing" when a task's predecessor is blocked or skipped

## Summary

When a task has a `**Predecessors:**` pointing to a predecessor that is blocked or skipped (not workable), the daemon writes "Preparing" for that task every cycle but the orchestrator immediately finds nothing to run. The snapshot never updates, no progress is visible anywhere, and the daemon spins indefinitely.

## Steps to Reproduce

1. Have a feature file with two tasks where Task 2 has `**Predecessors:** TASK-NNN-001`
2. Mark Task 1 as blocked (`BLOCKED:` criterion) or skipped (`SKIPPED:` criterion with `[>]` checkbox)
3. Run `maggus start`
4. Observe `maggus status` — it shows "Preparing" forever with no tool invocations and no worker output

## Expected Behavior

If all tasks in a feature group are stuck (no task can run because predecessors are unsatisfied and not completable), the daemon should not spin endlessly showing "Preparing". It should either detect that no runnable tasks exist and idle cleanly, or skip over blocked predecessors and allow successor tasks to proceed.

## Root Cause

`countWorkable` (`src/cmd/run_loop.go:154`) and `firstWorkableTask` (`src/cmd/run_loop.go:214`) use `IsWorkable()` — which checks only `!IsComplete() && !IsBlocked() && !IsSkipped()`. Neither function checks predecessor satisfaction.

The orchestrator's `classifyWorkable` (`src/cmd/orchestrator_parallel.go:117`) **does** check predecessors via `predecessorsComplete`. A task whose predecessor is blocked or skipped is never added to `completedIDs` (only fully completed tasks — all criteria checked — are added at `src/cmd/orchestrator.go:292`), so `predecessorsComplete` permanently returns false for its successors.

This creates a permanent mismatch:

| Check | Sees blocked-predecessor task as... |
|---|---|
| `countWorkable` / `firstWorkableTask` | **workable** → group included, "Preparing" written |
| `classifyWorkable` | **unrunnable** → skipped, `par=[], seq=[]` → loop breaks |

Cycle flow:
1. `countWorkable > 0` → group kept, `firstWorkableTask` returns the stuck task → `"Preparing"` written to `state.json` (`daemon_keepalive.go:306`)
2. Orchestrator runs → `classifyWorkable` rejects all tasks → `runGroupTasks` returns with 0 completed
3. `nullTUIModel` receives no `IterationStartMsg`, no `agent.StatusMsg`, no tool events → `writeSnapshot()` never called → `"Preparing"` persists
4. Daemon cycle ends, idles, repeats → same result forever

No worker snapshots are written because no workers are ever launched.

## User Stories

### BUG-045-001: Treat blocked and skipped predecessors as satisfied in predecessorsComplete

**Description:** As a user, I want successor tasks to run even when their predecessor was blocked or skipped, so that an explicit skip or block on an earlier task does not permanently deadlock subsequent tasks.

**Acceptance Criteria:**
- [x] `predecessorsComplete` (`src/cmd/orchestrator_parallel.go:141`) treats predecessors that are blocked or skipped (in addition to completed) as satisfied
- [x] A helper set `skippedOrBlockedIDs` is built alongside `completedIDs` in `runGroupTasks` from tasks where `IsBlocked()` or `IsSkipped()` is true
- [x] When a task's predecessor is in `skippedOrBlockedIDs`, `predecessorsComplete` returns true
- [x] A feature where Task 1 is blocked and Task 2 lists Task 1 as predecessor now runs Task 2
- [x] A feature where Task 1 is skipped and Task 2 lists Task 1 as predecessor now runs Task 2
- [x] Sequential tasks with unsatisfied (not-complete, not-blocked, not-skipped) predecessors are still correctly held back
- [x] No regression in normal predecessor-ordering behavior
- [x] `go vet ./...` and `go test ./...` pass
