<!-- maggus-id: 6b8d614a-1a4c-4332-bcf4-b4256da0b495 -->
# Bug: Approving a feature does not wake an idle daemon

## Summary

When the daemon is idle (no approved work) and a feature is approved via `maggus approve`, the daemon does not start working on it. It stays idle indefinitely until manually restarted.

## Steps to Reproduce

1. Start daemon with `maggus start` — ensure it has no approved features (idles immediately)
2. Run `maggus approve` and approve a feature
3. Observe: daemon does not start working on the newly approved feature

## Expected Behavior

Approving a feature should wake the idle daemon and cause it to begin working on the first approved task within a few seconds.

## Root Cause

**TOCTOU race: file-change events are silently dropped while the daemon's `send` function is nil.**

`src/cmd/daemon_keepalive.go:85` creates the filewatcher with `nil` send:
```go
fw, fwErr := filewatcher.New(dir, nil, 500*time.Millisecond)
```

`src/cmd/daemon_keepalive.go:149-160` — the send function is only set inside `waitForChanges()`:
```go
fw.SetSend(func(msg any) { ... })
defer fw.SetSend(nil)  // cleared when waitForChanges returns
```

This means:
- **During any work cycle** (`runOneDaemonCycle`), `send` is `nil`. Any fsnotify events fired during a cycle — including a Write to `feature_approvals.yml` — are received by the filewatcher's internal loop but silently discarded.
- **After the cycle finishes** with no work found and `waitForChanges` is entered, the send function is finally set. But the approval event already fired and was dropped. There is nothing left to wake the daemon.

The concrete failure sequence:
1. Daemon calls `runOneDaemonCycle` → `buildApprovedPlans` → no approved features → returns `false`
2. **At this moment**, user runs `maggus approve` → writes `.maggus/feature_approvals.yml`
3. Filewatcher debounce fires → calls `send(UpdateMsg{...})` → `send` is still `nil` → **event dropped**
4. Daemon enters `waitForChanges` → sets send function → blocks on `<-wakeCh`
5. No more events arrive → daemon waits forever

This race window is small (~0–500ms between `runOneDaemonCycle` returning and `fw.SetSend` being called), but approval is a manual user action that can coincide with it. Additionally, once the daemon has done real work and returns to idle, the prior `defer fw.SetSend(nil)` means the entire next work cycle runs with `send = nil`, widening the window.

`waitForChanges` has no fallback timeout — if the triggering event is missed, it blocks indefinitely on `<-ctx.Done()` or `<-wakeCh`, neither of which will fire.

## User Stories

### BUG-012-001: Add periodic wake-up fallback to waitForChanges

**Description:** As a user, I want the idle daemon to re-check for work periodically so that missed file-change events (due to the TOCTOU race) are recovered within a short time.

**Acceptance Criteria:**
- [x] `waitForChanges` in `src/cmd/daemon_keepalive.go` adds a `time.After(30 * time.Second)` case to its select — when it fires, return `wakeFileChange` with path `""` (treated the same as a real file event: daemon re-checks for work)
- [x] The 30-second interval is defined as a named constant (e.g. `daemonIdlePollInterval`) at the top of `daemon_keepalive.go`
- [x] Daemon still wakes immediately on real file-change events (existing behaviour unchanged)
- [x] Daemon still shuts down cleanly on context cancellation (existing behaviour unchanged)
- [x] Approving a feature while the daemon is idle causes it to start working within 30 seconds even if the fsnotify event was missed
- [x] No regression in daemon start/stop behaviour
- [x] `go vet ./...` and `go test ./...` pass
