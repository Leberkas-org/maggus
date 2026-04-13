<!-- maggus-id: bd78f92a-50bc-468c-b3f9-b81cc56ec7ab -->
# Bug: Stop-after-task indicator disappears immediately and state is lost

## Summary

After triggering "stop after task" from the status TUI, the "(stopping after task)" indicator in the left pane vanishes within ~500 ms and never reappears, even though the daemon is still running a task and will stop after it. The stop signal itself is delivered to the daemon correctly, but the TUI loses visibility of the pending state almost immediately.

## Steps to Reproduce

1. Run `maggus status` while the daemon is actively running a task
2. Press `s` to open the stop overlay
3. Press `s` again to choose "stop after task"
4. Observe the left pane — "(stopping after task)" briefly appears (or may not appear at all) then disappears
5. The daemon does eventually stop after the task, but there is no visual feedback during the wait

## Expected Behavior

"● Running (stopping after task)" should remain visible in the left pane until the daemon process exits. The user needs to know the stop was acknowledged and that the daemon will not start another task.

## Root Cause

The polling goroutine in `runOneDaemonCycle` (`src/cmd/daemon_keepalive.go:385–401`) detects the sentinel file and **immediately deletes it** before the running task finishes:

```go
go func() {
    ticker := time.NewTicker(500 * time.Millisecond)
    defer ticker.Stop()
    stopAfterTaskFile := daemonStopAfterTaskFilePath(dir)
    for {
        select {
        case <-workCtx.Done():
            return
        case <-ticker.C:
            if _, err := os.Stat(stopAfterTaskFile); err == nil {
                removeStopAfterTaskFile(dir)   // ← deletes file here, task still running
                stopFlagAtomic.Store(true)
                return
            }
        }
    }
}()
```

`DaemonStateCache` (`src/cmd/daemon_state_cache.go`) watches `.maggus/` via fsnotify. It detects the deletion and sets `StoppingAfterTask = false`, which is pushed to the TUI as a `daemonCacheUpdateMsg`. The TUI handler (`src/cmd/status_update.go:98`) unconditionally overwrites `m.daemonStoppingAfterTask` with the incoming value:

```go
m.daemonStoppingAfterTask = msg.State.StoppingAfterTask
```

Result: the TUI optimistically sets `daemonStoppingAfterTask = true` when the user presses `s`, but the daemon removes the file within one 500 ms tick, `DaemonStateCache` reports `StoppingAfterTask = false`, and the TUI clears the indicator — even though `stopFlagAtomic` is now set and the daemon *will* stop after the task.

## User Stories

### BUG-042-001: Keep sentinel file alive until task completes

**Description:** As a user, I want the "(stopping after task)" indicator to stay visible until the daemon actually stops, so I have confidence my stop request was received.

**Acceptance Criteria:**
- [x] The polling goroutine in `runOneDaemonCycle` (`src/cmd/daemon_keepalive.go`) does **not** call `removeStopAfterTaskFile` — it only sets `stopFlagAtomic.Store(true)` and returns
- [x] `removeStopAfterTaskFile` is called after the orchestrator finishes and before returning `errStopAfterTask` (i.e. just before `return true, errStopAfterTask` at line 443)
- [x] `DaemonStateCache` continues to report `StoppingAfterTask = true` for the entire duration the task is running after the stop was requested
- [x] The TUI left pane shows "● Running (stopping after task)" continuously from the moment the user confirms stop until the daemon process exits
- [x] The existing idle-path cleanup (`runDaemonLoop` and `waitForChanges`) is not changed — those paths remove the file before the daemon returns, which is correct
- [x] Stale leftover file is still cleaned up at daemon startup (`removeStopAfterTaskFile` call at `daemon_keepalive.go:58`)
- [x] `go vet ./...` and `go test ./...` pass
