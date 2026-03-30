<!-- maggus-id: 8e9104a5-6a90-412e-b173-5ce6929d3c40 -->
# Bug: Daemon keeps running after stop-after-task signal is consumed

## Summary

When the stop-after-task signal fires and the current task finishes, the daemon loops back and starts a new work cycle instead of exiting. The daemon should halt after completing the task it was on when the signal arrived.

## Related

- **Commit:** 087f192 (feat(TASK-031-001): Add stop-after-task signal file mechanism)
- **Commit:** 57d9266 (feat(TASK-031-002): Fix stop-after-task key bindings)

## Steps to Reproduce

1. Start the daemon: `maggus daemon`
2. Wait for a task to begin executing
3. Press the stop-after-task keybind (or create the sentinel file manually)
4. Observe: the current task completes, but the daemon immediately picks up the next task and continues running

## Expected Behavior

After the current task completes, the daemon should exit cleanly rather than looping back to check for more work.

## Root Cause

`runOneDaemonCycle` (`src/cmd/daemon_keepalive.go:173`) unconditionally returns `true` at line 352, regardless of whether the stop-after-task signal was triggered during the cycle.

Back in `runDaemonLoop` (line 105), `hadWork == true` causes an immediate `continue`, bypassing the file-watcher wait and starting another cycle. Because the stop-after-task signal only sets `stopFlagAtomic` (line 303) — it does **not** cancel `workCtx` — the context-done check at line 92 also passes, and the daemon spins into a new cycle unimpeded.

The `stopFlagAtomic` value is fully consumed inside `runOneDaemonCycle` to halt the work goroutine between tasks, but its value is never read again after `p.Run()` returns (line 347). There is no path by which a stop-after-task event can propagate back to `runDaemonLoop` and cause it to exit.

```
// daemon_keepalive.go
347:    _, tuiErr := p.Run()
348:    if tuiErr != nil {
349:        return true, fmt.Errorf("TUI error: %w", tuiErr)
350:    }
351:
352:    return true, nil   // <-- always true; stopFlagAtomic is never checked here
```

## User Stories

### BUG-015-001: Propagate stop-after-task signal back to the daemon loop

**Description:** As a user, I want the daemon to stop after the current task when I request it, so that the daemon respects my intent to halt work cleanly.

**Acceptance Criteria:**
- [x] After a stop-after-task signal is consumed and the current task finishes, `runDaemonLoop` exits without starting a new cycle
- [x] The daemon exits with a clean (nil) error in this case
- [x] Normal daemon behaviour (looping to pick up more work) is unaffected when no stop signal was sent
- [x] The existing `workCtx` cancellation path (OS signal / stop file) still works correctly
- [x] No regression in stop-after-task behaviour in the interactive status view
- [x] `go vet ./...` and `go test ./...` pass
