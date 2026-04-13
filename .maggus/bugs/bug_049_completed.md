<!-- maggus-id: 323e60ff-3c39-4735-ac85-fcf7dae1604d -->
# Bug: Next sequential task shows "Preparing" for entire duration of a parallel batch

## Summary

When a parallel batch starts, the main `state.json` snapshot was already written as "Preparing" for the next sequential task. Because `sendIterationStart` is never called before `runParallelBatch`, the snapshot never transitions away from "Preparing" until the sequential task actually starts. The user sees "Preparing" for the entire duration of the parallel batch — which can be long — with no indication that they are simply waiting for parallel work to complete.

## Steps to Reproduce

1. Have a feature with at least one parallel task (003) and one sequential task (002) that will run after it
2. Run `maggus start`
3. Watch `maggus status` while the parallel task runs
4. Observe: main pane shows "Preparing" for the sequential task the entire time

## Expected Behavior

The main pane status should read "Waiting" (or similar) to clearly communicate that the next task is queued and held until the parallel batch finishes — not that startup is in progress.

## Root Cause

`src/cmd/daemon_keepalive.go:306` writes "Preparing" for the first workable task before the orchestrator starts. When the orchestrator runs a parallel batch (`orchestrator.go:351`), it never calls `sendIterationStart` or sends any message that would cause `nullTUIModel.writeSnapshot()` to update the status. The snapshot stays at "Preparing" for the duration of the batch.

## User Stories

### BUG-049-001: Update main snapshot status to "Waiting for X" when a parallel batch is running

**Description:** As a user, I want the main status pane to show what the next task is waiting for when it is queued behind a running parallel batch, so I can tell the difference between "startup in progress" and "waiting for parallel tasks to finish".

**Acceptance Criteria:**
- [x] Before calling `runParallelBatch` in `orchestrator.go`, if there is a pending sequential task, send a snapshot update that changes the status from "Preparing" to a descriptive waiting message
- [x] The status names the parallel tasks being awaited, e.g. `"Waiting for TASK-008-003"` for a single task or `"Waiting for TASK-008-003, TASK-008-004"` for multiple
- [x] After the parallel batch completes and the sequential task starts, `sendIterationStart` transitions the status to "Starting" as normal
- [x] If there is no pending sequential task after the parallel batch (all remaining work is parallel), the update is skipped
- [x] Sequential-only features are unaffected
- [x] `go vet ./...` and `go test ./...` pass
