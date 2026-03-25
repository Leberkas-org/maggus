<!-- maggus-id: fe57d428-fc6c-41c7-b1ef-8f0786e3ae32 -->
# Bug: Daemon wait loop uses polling fallback instead of pure fsnotify

## Summary

The daemon `waitForChanges` function sets a 5-minute poll timer alongside the fsnotify watcher. When no files change, the timer fires and logs "poll timeout reached, rechecking" every 5 minutes. The daemon should rely solely on filesystem notifications with no periodic polling.

## Steps to Reproduce

1. Run `maggus daemon` with no pending work
2. Wait 5 minutes without modifying any feature/bug files
3. Observe log: "poll timeout reached, rechecking"
4. This repeats every 5 minutes indefinitely

## Expected Behavior

The daemon blocks on fsnotify events only. No periodic polling, no "poll timeout reached" log entries. The daemon wakes only when a feature/bug file is actually created, modified, or deleted — or when a shutdown signal is received.

## Root Cause

`waitForChanges` in `src/cmd/daemon_keepalive.go:125-166` creates both an fsnotify watcher **and** a `time.NewTimer(timeout)` on every call. The `select` at line 158 races the two:

```go
timer := time.NewTimer(timeout)
defer timer.Stop()

select {
case <-ctx.Done():
    return wakeSignal, ""
case evt := <-wakeCh:
    return wakeFileChange, evt.path
case <-timer.C:
    return wakeTimeout, ""
}
```

The timer fires after `DaemonPollIntervalDuration()` (default 5 minutes, configured via `daemon_poll_interval` in `config.yml`). This means the daemon never truly sleeps on fsnotify alone — it always wakes on the poll timeout even when fsnotify is healthy.

Additionally, the watcher is recreated from scratch on every wait cycle (line 144, deferred close at line 149–153). This is wasteful but not the primary bug.

Secondary references:
- `src/internal/config/config.go:140` — `DaemonPollInterval` config field
- `src/internal/config/config.go:147-161` — `DefaultDaemonPollInterval` constant (5 minutes) and duration parser
- `src/cmd/daemon_keepalive.go:72` — poll interval read from config
- `src/cmd/daemon_keepalive.go:109` — "poll timeout reached" log line

## User Stories

### BUG-004-001: Remove poll timer from daemon wait loop

**Description:** As a user, I want the daemon to block purely on fsnotify events so that it doesn't wake up and re-scan every 5 minutes when there's no work.

**Acceptance Criteria:**
- [x] `waitForChanges` no longer creates a `time.NewTimer` for periodic polling
- [x] The `select` in `waitForChanges` only listens for `ctx.Done()` and the fsnotify wake channel
- [x] `wakeTimeout` reason and the `"poll timeout reached"` log line are removed
- [x] The `DaemonPollInterval` config field, `DefaultDaemonPollInterval` constant, and `DaemonPollIntervalDuration()` method are removed from `src/internal/config/config.go`
- [x] The `daemon_poll_interval` YAML key is documented as removed (or silently ignored for backward compat)
- [x] Related tests in `src/internal/config/config_test.go` (`TestDaemonPollIntervalDuration_*`, `TestLoad_DaemonPollInterval`) are removed
- [x] The log message at line 100 no longer references a timeout duration
- [x] No regression: daemon still wakes on file create/write/remove/rename in `.maggus/features/` and `.maggus/bugs/`
- [x] No regression: daemon still shuts down cleanly on signal or stop file
- [x] `go vet ./...` and `go test ./...` pass

### BUG-004-002: Persist filewatcher across wait cycles

**Description:** As a user, I want the filesystem watcher to remain active across wait cycles so that file changes during a work cycle are not missed.

**Acceptance Criteria:**
- [ ] The `filewatcher.Watcher` is created once in `runDaemonLoop` and reused across iterations
- [ ] `waitForChanges` accepts the existing watcher instead of creating a new one
- [ ] The watcher is closed on daemon shutdown (defer in `runDaemonLoop`)
- [ ] No regression: daemon still detects file changes in `.maggus/features/` and `.maggus/bugs/`
- [ ] No regression: daemon still shuts down cleanly on signal or stop file
- [ ] `go vet ./...` and `go test ./...` pass
