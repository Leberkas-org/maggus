<!-- maggus-id: 402f3cd1-f616-40ec-938b-8b92dcbecdac -->
# Bug: Status view shows blank output panel for several seconds after navigating back from the menu

## Summary

When the user navigates away from the status view (detach → main menu) and then returns
while the daemon is mid-task, the output panel and task indicator are blank for several
seconds. After some time the view catches up and shows the correct running state. Nothing
is actually lost — the delay is a display-only stall.

## Steps to Reproduce

1. Open maggus and navigate to the status view while a daemon is running a task.
2. Press `q` (or the back key) to return to the main menu while the task is in progress.
3. Immediately navigate back to the status view.
4. Observe: the daemon status line shows "● Running" but the output panel, current
   feature/task indicator, and snapshot data are all blank.
5. Wait a few seconds (until the daemon next writes to its log file).
6. Observe: the view suddenly catches up and shows the correct state.

## Expected Behavior

On re-entering the status view, the output panel and task indicator should immediately
reflect the daemon's current state (same as they would on a cold-start `maggus status`).

## Root Cause

`buildStatusModel()` (`cmd/app_model.go:306–365`) seeds `sm.daemon.PID` and
`sm.daemon.Running` from `daemonCache.Get()`, but does **not** populate the
log-derived fields:

```
sm.daemon.RunID          // always ""
sm.daemon.LogPath        // always ""
sm.daemon.CurrentFeature // always ""
sm.daemon.CurrentTask    // always ""
sm.snapshot              // always nil
```

These fields are only set inside the `logFileUpdateMsg` handler
(`status_update.go:95–160`), which runs `findLatestRunLog`, `readLastNLogLines`,
`parseLogForCurrentState`, and `runlog.ReadSnapshot`.

The `logFileUpdateMsg` is delivered by the `LogFileWatcher` — but only when fsnotify
detects an actual file **write** or **create** event. On re-entry the watcher was just
created; it misses any writes that happened while the screen was inactive. The next event
only arrives when the daemon next appends to its log file, which can be seconds later.

The `logPollTick()` fallback (used when fsnotify is unavailable) does NOT have this
problem: it fires unconditionally after 200ms, giving an immediate `logFileUpdateMsg`
regardless of file activity.

A secondary contributor: `statusModel.Init()` (`status_update.go:19–41`) does not
fire an immediate `logFileUpdateMsg` when `logWatcherCh` is not nil. The initial
read therefore depends entirely on the first fsnotify event.

## User Stories

### BUG-014-001: Seed log-derived daemon fields at status model construction time

**Description:** As a user, I want the status view to show the current running task
immediately on re-entry so I don't see a blank output panel while the daemon is active.

**Acceptance Criteria:**
- [x] `buildStatusModel()` in `cmd/app_model.go` calls `findLatestRunLog(dir)` after
  constructing the model and seeds `sm.daemon.RunID` and `sm.daemon.LogPath`
- [x] When `sm.daemon.Running` is true and `sm.daemon.RunID != ""`, `buildStatusModel()`
  also calls `runlog.ReadSnapshot(dir)` and stores the result in `sm.snapshot`
- [x] When a log path is found, `buildStatusModel()` calls `readLastNLogLines` and
  `parseLogForCurrentState` to populate `sm.daemon.CurrentFeature` and
  `sm.daemon.CurrentTask`
- [x] Navigating away from the status view while a task is running and immediately
  returning shows the correct feature name, task ID, and output panel content without
  any blank-state delay
- [x] When the daemon is not running, these fields remain empty (no behaviour change
  for the stopped-daemon case)
- [x] No regression: `go vet ./...` and `go test ./...` pass

### BUG-014-002: Fire an immediate logFileUpdateMsg on Init when using LogFileWatcher

**Description:** As a developer, I want `statusModel.Init()` to trigger an immediate
log read on startup so the view is never blank on first render, regardless of whether
the fsnotify path or the poll-tick fallback is used.

**Acceptance Criteria:**
- [x] `statusModel.Init()` in `cmd/status_update.go` includes
  `func() tea.Msg { return logFileUpdateMsg{} }` in its `tea.Batch` call regardless
  of whether `logWatcherCh` is nil (i.e. it fires for both the `LogFileWatcher` path
  and the `logPollTick` fallback path)
- [x] The existing `listenForLogFileUpdate(m.logWatcherCh)` call is kept so subsequent
  fsnotify events continue to be processed
- [x] On cold-start `maggus status` (where the daemon may not be running), this extra
  message results in a no-op read and no visible change in behaviour
- [x] No regression: `go vet ./...` and `go test ./...` pass
