<!-- maggus-id: 90b5568d-02be-426b-b9b7-ddabd76e7dd6 -->
# Bug: Spinners and elapsed times keep running after task completion

## Summary

When all tasks finish, the tree spinners keep spinning and the run/task elapsed times in the Metrics tab keep incrementing. They should freeze at the completion moment and the spinner should stop animating.

## Steps to Reproduce

1. Run `maggus status` while a daemon is active
2. Wait for the current task to complete (status transitions to Done/Failed/Interrupted)
3. Observe the left pane tree — task spinners are still spinning
4. Open the Metrics tab — run elapsed and task elapsed times are still ticking upward

## Expected Behavior

- Tree spinners stop animating when the snapshot status is `"Done"`, `"Failed"`, or `"Interrupted"`
- Run elapsed and task elapsed times freeze at the final value when the run is no longer active

## Root Cause

Two independent issues both caused by `spinnerTickMsg` continuing to fire every 80ms regardless of run state.

**Issue 1 — Spinner tick never stops**
`src/cmd/status_update.go:71-77`:

```go
case spinnerTickMsg:
    if m.daemon.Running && m.snapshot != nil {
        m.spinnerFrame = (m.spinnerFrame + 1) % len(styles.SpinnerFrames)
        return m, spinnerTick()
    }
    // Keep ticking even when idle so the spinner starts immediately when daemon resumes.
    return m, spinnerTick()
```

The else branch **unconditionally re-schedules** `spinnerTick()`. This means the 80ms timer never stops — it keeps firing even after `snap.Status` is `"Done"`. The `snap.Status` field is already checked elsewhere (e.g. `status_rightpane.go:129-139`) to display `✓`/`✗`/`⊘`, but that same check is not applied to stop the tick.

**Issue 2 — Elapsed times use `time.Since()` unconditionally**
`src/cmd/status_rightpane.go:263-277`:

```go
if t, err := time.Parse(time.RFC3339, snap.RunStartedAt); err == nil {
    runElapsed = formatHumanDuration(time.Since(t))
}
// ...
if t, err := time.Parse(time.RFC3339, snap.TaskStartedAt); err == nil {
    taskElapsed = formatHumanDuration(time.Since(t))
}
```

`time.Since()` is called on every render regardless of `snap.Status`. When the run is done, the elapsed time should be frozen. The snapshot has no `CompletedAt` field, but `snap.Status` is available and sufficient: when it is a terminal state, elapsed = time between `StartedAt` and the moment the status changed — which for display purposes can be approximated as the last elapsed value seen before status became terminal, or calculated by storing a freeze time in the model.

The simplest fix: in `status_update.go`, when `snap.Status` is a terminal state (`"Done"`, `"Failed"`, `"Interrupted"`), stop advancing `spinnerFrame` **and** stop re-scheduling `spinnerTick()`. The spinner will stop firing, View() will stop being called on tick, and elapsed times will naturally freeze at the last rendered value. If any other trigger (e.g. window resize, log update) causes a re-render, elapsed times would tick again — so a more robust fix also freezes the elapsed time in `renderSnapshotInPane` when `snap.Status` is terminal.

## User Stories

### BUG-011-001: Stop spinner tick and freeze elapsed times when run is done

**Description:** As a user watching the status TUI, I want spinners and elapsed times to stop updating once a run finishes so that I can clearly see the final duration without confusion.

**Acceptance Criteria:**
- [x] In `status_update.go`, when `snap.Status` is `"Done"`, `"Failed"`, or `"Interrupted"`, `spinnerTick()` is NOT re-scheduled (the tick loop stops)
- [x] In `status_rightpane.go` `renderSnapshotInPane`, run elapsed and task elapsed times are frozen (do not call `time.Since()`) when `snap.Status` is a terminal state (`"Done"`, `"Failed"`, `"Interrupted"`)
- [x] Tree spinners in the left pane stop animating when the run is in a terminal state
- [x] When a new run starts (daemon resumes), the spinner tick re-starts correctly
- [x] Existing spinner behavior (spinning while active, `✓`/`✗`/`⊘` icon when done) is unchanged
- [x] No regression in log panel refresh or daemon state updates
- [x] `go vet ./...` and `go test ./...` pass
