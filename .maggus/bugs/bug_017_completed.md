<!-- maggus-id: 74b86c46-e813-4d20-b0b7-f2109c78c88c -->
# Bug: Stop-after-task ignored when daemon is idle (before and during waitForChanges)

## Summary

When the user activates stop-after-task while the daemon is idle (no workable tasks), the daemon does not stop. It continues waiting for file changes indefinitely — both if the signal arrives before entering `waitForChanges` and if it arrives while already blocking inside it.

## Steps to Reproduce

**Scenario A — signal before wait:**
1. Start `maggus daemon` with no workable tasks (all done or all blocked)
2. Open `maggus status` and press `s` to activate stop-after-task
3. Observe: daemon enters `waitForChanges` and stays alive

**Scenario B — signal during wait:**
1. Start `maggus daemon` with no workable tasks
2. Let the daemon enter its idle wait state
3. Press `s` in `maggus status` to activate stop-after-task
4. Observe: daemon does not react; waits up to 30 seconds for the next idle poll

## Expected Behavior

If stop-after-task is activated and no task is currently running, the daemon should exit immediately — regardless of whether the signal arrives before or during `waitForChanges`.

## Root Cause

The stop-after-task polling goroutine is spawned **inside** `runOneDaemonCycle` (`src/cmd/daemon_keepalive.go:301-317`), meaning it only runs when the daemon has found work and is about to execute tasks. The sentinel file `.maggus/daemon.stop-after-task` is only ever consumed inside a cycle that actually runs a task.

When there is no work, `runOneDaemonCycle` returns early at line 193 (`setup == nil`) or line 212-213 (`len(featureGroups) == 0`) with `(false, nil)` — before the polling goroutine is even created. The outer loop at `src/cmd/daemon_keepalive.go:118-127` then enters `waitForChanges`, which blocks on file-system events or a 30-second timeout. Neither of those paths checks for the stop-after-task sentinel file.

As a result:
- The sentinel file sits unread on disk (or is read only on the next cycle that happens to run a task)
- The daemon stays alive until a feature file changes or the 30-second idle poll fires
- Even then, it re-enters `runOneDaemonCycle`, still finds no work, and loops again — never consuming the stop signal

**Fix A — before entering wait (scenario A):** Check the sentinel file in the outer `runDaemonLoop` at `src/cmd/daemon_keepalive.go:118`, before calling `waitForChanges`:

```go
if hadWork {
    continue
}

// Exit immediately if stop-after-task was requested while idle.
if _, err := os.Stat(daemonStopAfterTaskFilePath(dir)); err == nil {
    removeStopAfterTaskFile(dir)
    return nil
}

wakeReason, wakePath := waitForChanges(fw, workCtx)
```

**Fix B — during wait (scenario B):** `waitForChanges` (`src/cmd/daemon_keepalive.go:146`) must also poll the sentinel file so it can wake early. Add a new `wakeStopAfterTask` reason and a ticker that polls the file on the same 500 ms interval already used elsewhere:

```go
stopAfterTaskTicker := time.NewTicker(500 * time.Millisecond)
defer stopAfterTaskTicker.Stop()

select {
case <-ctx.Done():
    return wakeSignal, ""
case evt := <-wakeCh:
    return wakeFileChange, evt.path
case <-stopAfterTaskTicker.C:
    if _, err := os.Stat(daemonStopAfterTaskFilePath(dir)); err == nil {
        return wakeStopAfterTask, ""
    }
case <-time.After(daemonIdlePollInterval):
    return wakeFileChange, ""
}
```

The caller in `runDaemonLoop` then handles `wakeStopAfterTask` by removing the file and returning nil.

## User Stories

### BUG-017-001: Exit immediately if stop-after-task is set before entering waitForChanges

**Description:** As a user, I want the daemon to stop immediately when I press stop-after-task and no task is running, so I don't have to wait for a file-change event or a 30-second timeout.

**Acceptance Criteria:**
- [x] In `runDaemonLoop`, after `runOneDaemonCycle` returns `(false, nil)`, check whether the sentinel file exists before calling `waitForChanges`
- [x] If the sentinel file is present, remove it and return nil (daemon exits cleanly)
- [x] When a task IS active, stop-after-task still waits for that task to finish (existing behavior preserved)
- [x] No regression in normal daemon work loop behavior
- [x] `go vet ./...` and `go test ./...` pass

### BUG-017-002: Wake waitForChanges immediately on stop-after-task

**Description:** As a user, I want the daemon to react within milliseconds if I press stop-after-task while it is already in its idle wait state, so I'm not stuck waiting up to 30 seconds.

**Acceptance Criteria:**
- [x] `waitForChanges` polls the stop-after-task sentinel file (500 ms interval) alongside the existing file-change watcher
- [x] When the sentinel file is detected inside `waitForChanges`, the function returns a new `wakeStopAfterTask` reason
- [x] The caller in `runDaemonLoop` handles `wakeStopAfterTask` by removing the sentinel file and returning nil
- [x] Normal file-change waking and context-cancellation shutdown are unaffected
- [x] `go vet ./...` and `go test ./...` pass
