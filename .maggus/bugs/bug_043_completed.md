<!-- maggus-id: c6bdce4a-c553-47b8-8212-a143db91007b -->
# Bug: "Stopping after task" indicator disappears before daemon exits

## Summary

After sending the "stop after task" signal, the TUI's "⏳ Stopping after task…" indicator vanishes prematurely — immediately when the sentinel file is removed — rather than persisting until the daemon process actually exits. The daemon removes `daemon.stop-after-task` right before it exits, causing the cache to report `StoppingAfterTask=false` while the process is still alive (for milliseconds), creating a brief "● Running" flash before "○ not running" appears.

## Steps to Reproduce

1. Run `maggus start` with at least one active task
2. Open `maggus status`
3. Press the stop key to open the stop overlay
4. Press `s` (stop after task)
5. Observe: "⏳ Stopping after task…" appears in the footer and "● Running (stopping after task)" in the header
6. When the current task finishes, both indicators disappear momentarily (show "● Running") before the daemon exits

## Expected Behavior

The "stopping after task" indicator remains visible from when the signal is sent until the daemon process has fully exited. Transitions directly from "stopping after task" → "not running" with no intermediate "running" flash.

## Root Cause

The bug is in `src/cmd/status_update.go` at the `daemonCacheUpdateMsg` handler (line 98):

```go
case daemonCacheUpdateMsg:
    prevRunning := m.daemon.Running
    m.daemon.PID = msg.State.PID
    m.daemon.Running = msg.State.Running
    m.daemonStoppingAfterTask = msg.State.StoppingAfterTask  // ← always overwritten
    ...
```

The sentinel lifecycle is:

1. User sends signal → `daemon.stop-after-task` created
2. Cache detects creation → `StoppingAfterTask=true` → TUI shows "stopping"
3. Task completes → `runOneDaemonCycle` calls `removeStopAfterTaskFile(dir)` and returns `errStopAfterTask`
4. Cache detects *removal* via fsnotify → emits `daemonCacheUpdateMsg{StoppingAfterTask: false, Running: true}`
5. Line 98 unconditionally writes `m.daemonStoppingAfterTask = false` → indicator disappears
6. Milliseconds later daemon exits → `Running: false` arrives → "not running"

The window between steps 4 and 6 is very small but perceptible. If the task was short (completed before the 500ms polling goroutine fired), step 3 can happen via the "no work found" sentinel check in `runDaemonLoop` (lines 117–121 of `daemon_keepalive.go`) even more quickly.

The fix is to stop clearing `daemonStoppingAfterTask` on `StoppingAfterTask=false` updates while the daemon is still running. Only clear it when `Running` transitions to `false`.

## User Stories

### BUG-043-001: Keep "stopping after task" indicator visible until daemon exits

**Description:** As a user, I want the "stopping after task" indicator to remain visible from the moment I send the signal until the daemon process fully exits, so that I'm not confused by a brief flash of "● Running" as the daemon winds down.

**Acceptance Criteria:**
- [x] In `status_update.go`, the `daemonCacheUpdateMsg` handler sets `m.daemonStoppingAfterTask = true` when `msg.State.StoppingAfterTask` is true
- [x] `m.daemonStoppingAfterTask` is cleared to `false` only when `msg.State.Running == false`
- [x] When the daemon is still running but `StoppingAfterTask=false` (transition window between sentinel removal and process exit), `daemonStoppingAfterTask` is left unchanged
- [x] The optimistic set (`m.daemonStoppingAfterTask = true`) in `updateStatusDaemonStopOverlay` still takes effect immediately on "s" keypress
- [x] Sending "stop after task" and waiting for the daemon to finish shows: "stopping after task" → "not running" with no intermediate "● Running" flash
- [x] No regression in the kill-now path (`k` / `ctrl+c`): when the daemon is force-killed, `Running=false` arrives and `daemonStoppingAfterTask` is cleared correctly
- [x] `go vet ./...` and `go test ./...` pass
