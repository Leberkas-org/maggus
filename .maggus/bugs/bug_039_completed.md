<!-- maggus-id: 66c4fdcc-91d2-456b-8ffd-6d9b82873db8 -->
# Bug: Output tab shows wrong data, "Done" for running tasks, or nothing — desync between snapshot and JSONL log

## Summary

The Output tab in the status TUI is unreliable: it shows "Done" for running tasks, nothing for tasks with data, or data from the wrong task. The root cause is that the TUI uses two independent data sources (JSONL log for spinner/running detection, state.json snapshot for output rendering) that can disagree about the current task state.

## Steps to Reproduce

1. Start the daemon with an approved feature
2. Open status view, expand the feature, select the running task
3. Switch to Output tab
4. Observe any of: "No tool invocations yet", wrong task's data, "Done" status while spinner shows running

## Expected Behavior

The Output tab should show live tool invocations for the running task, in sync with the spinner indicator.

## Root Cause

The pipeline has TWO INDEPENDENT SOURCES OF TRUTH that aren't synchronized:

### Source 1: JSONL log → `m.daemon.CurrentTask` (determines spinner)

`parseLogForCurrentState()` in `status_runlog.go` scans the last 200 lines of the JSONL log file for the most recent `task_start` event. This sets `m.daemon.CurrentTask`. The `isTaskRunning()` method uses this to show the spinner.

### Source 2: state.json → `m.snapshot` (determines output content)

`runlog.ReadSnapshot()` reads `.maggus/runs/state.json`, written by the daemon's `nullTUIModel.writeSnapshot()`. This snapshot contains `task_id`, tool entries, tokens, etc. The Output tab renders from this snapshot.

### The desync

These can disagree because:
1. **Task ID empty in snapshot:** `nullTUIModel.taskID` is set when `IterationStartMsg` fires, but `writeSnapshot()` can be called before that (on status changes), producing a snapshot with `task_id: ""`.
2. **Stale snapshot after task completion:** When a task completes, the snapshot's `status` becomes "Done" but `m.daemon.CurrentTask` may still point to the task until the next JSONL entry updates it.
3. **No TaskID validation in rendering:** `snapshotForSelectedTask()` returns `m.snapshot` without checking that `snap.TaskID == selectedTask.ID`. If the snapshot is for a different task (or empty), wrong data is shown.
4. **Worker index stale entries:** Dispatched workers can leave `status: "failed"` entries in `state-workers.json` that never get cleaned up, confusing the worker snapshot lookup.

### Specific scenarios

**"Done" for running task:** JSONL log has `task_start` → spinner shows. But state.json was written during the PREVIOUS task's completion → snapshot status is "Done".

**Empty output for running task:** state.json has `task_id: ""` because `writeSnapshot()` was called before `IterationStartMsg` set the task ID.

**Wrong task's output:** `snapshotForSelectedTask()` returns `m.snapshot` which contains task A's data, but the user selected task B.

## User Stories

### BUG-039-001: Use snapshot as single source of truth for task running state

**Description:** As a developer, I want the TUI to use a single consistent data source for determining task state so the spinner and Output tab always agree.

**Acceptance Criteria:**
- [x] `isTaskRunning()` checks `m.snapshot.TaskID == taskID` AND `m.snapshot.Status` is not terminal (not "Done"/"Failed"/"Interrupted") as its PRIMARY check, falling back to JSONL parsing only when no snapshot exists
- [x] `parseLogForCurrentState()` is kept as a fallback for `daemon.CurrentFeature` (feature-level tracking) but NOT used for task-level running detection
- [x] When the snapshot has `task_id: ""`, `isTaskRunning()` returns false (no false positives)
- [x] Spinner and Output tab always agree: if spinner shows, Output tab has data; if no data, no spinner
- [x] `go vet ./...` and `go test ./...` pass

### BUG-039-002: Validate snapshot TaskID matches selected task before rendering

**Description:** As a user, I want the Output tab to only show data for the task I selected, not stale data from a different task.

**Acceptance Criteria:**
- [x] `snapshotForSelectedTask()` checks that the returned snapshot's `TaskID` matches the selected task's ID
- [x] If `m.snapshot.TaskID != selectedTask.ID`, returns nil (not the wrong snapshot)
- [x] For parallel mode, `m.workerSnapshots[task.ID]` still takes precedence when available
- [x] When no matching snapshot exists, the Output tab shows "Waiting for agent output..." (not stale data)
- [x] `go vet ./...` and `go test ./...` pass

### BUG-039-003: Ensure nullTUIModel always writes task_id to snapshot

**Description:** As a developer, I want every snapshot write to include the current task_id so the TUI always has correct task identification.

**Acceptance Criteria:**
- [x] In `nullTUIModel.writeSnapshot()` (`daemon_tui.go`), `snap.TaskID` is always set from `m.taskID` — verify this is never empty during active work
- [x] `IterationStartMsg` handler sets `m.taskID` BEFORE the first `writeSnapshot()` call in the same message cycle
- [x] If `m.taskID` is empty when `writeSnapshot()` is called, the snapshot includes a field indicating "no active task" (not silently empty)
- [x] `go vet ./...` and `go test ./...` pass

### BUG-039-004: Clean up stale worker index entries

**Description:** As a user, I want the workers index to not contain stale entries from previous dispatched workers so the TUI doesn't show ghost workers.

**Acceptance Criteria:**
- [x] When the daemon starts a new work cycle, it removes completed/failed entries from `state-workers.json` that are older than 5 minutes
- [x] When a dispatched worker completes (success or failure), it removes its own entry from the workers index OR updates status to done/failed (already partially done)
- [x] `refreshWorkerSnapshots()` skips entries where the per-worker snapshot file doesn't exist
- [x] The status TUI never shows ghost workers from previous runs
- [x] `go vet ./...` and `go test ./...` pass

## Task Dependency Graph

```
BUG-039-001 ──→ BUG-039-002 ──┐
BUG-039-003 ───────────────────├──→ (done)
BUG-039-004 ───────────────────┘
```

| Task | Predecessors | Parallel |
|------|-------------|----------|
| BUG-039-001 | none | yes (with 003, 004) |
| BUG-039-002 | 001 | yes (with 003, 004) |
| BUG-039-003 | none | yes (with 001, 004) |
| BUG-039-004 | none | yes (with 001, 003) |
