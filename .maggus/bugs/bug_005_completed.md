<!-- maggus-id: 5d301f1f-6ed5-426b-b2d3-18851018fdae -->
# Bug: Output tab and snapshot broken after runlog flat-file refactor

## Summary

After TASK-010-002 changed log files from per-run subdirectories (`.maggus/runs/<runID>/run.log`) to flat files (`.maggus/runs/<timestamp>.log`), the Output tab in `maggus status` shows "No active run" at all times. The rich live snapshot view (spinner, tool entries, token counts) is also gone. Both regressions stem from `findLatestRunLog` in `status_runlog.go` still looking for directories instead of files.

## Related

- **Commit:** ca6560a (feat(TASK-010-002): Refactor runlog.Open for flat per-plan log files with automatic pruning)
- **Commit:** 6f66419 (feat(TASK-010-003): Wire maggusID through work loop and daemon; remove run directory creation)

## Steps to Reproduce

1. Start the daemon: `maggus start`
2. Open `maggus status`
3. Press `2` to switch to the Output tab
4. Observe: "No active run" even while the daemon is actively running tasks

## Expected Behavior

The Output tab shows live JSONL log entries (tool use, output, task events) from the current daemon run. When the daemon is running, the snapshot view (spinner, tool list, token usage) renders in the Output tab.

## Root Cause

`findLatestRunLog` in `src/cmd/status_runlog.go:72-94` has two assumptions that broke after TASK-010-002:

**Assumption 1 — looks for directories:**
```go
for _, e := range entries {
    if e.IsDir() {               // line 80: skips flat .log files entirely
        dirs = append(dirs, e.Name())
    }
}
```
The new flat log files (`20260327-000328.log`) are not directories, so `dirs` is always empty and the function returns `"", ""`.

**Assumption 2 — constructs old nested path:**
```go
candidate := filepath.Join(runsDir, latest, "run.log")  // line 89: old format
```
Even if directories existed, this path is wrong. The new format is `runsDir/<timestamp>.log`.

**Cascade from the empty return:**
- `m.daemon.LogPath == ""` → `status_update.go:87` sets `m.logLines = nil` → Output tab renders "No active run"
- `m.daemon.RunID == ""` → `status_update.go:75` calls `runlog.ReadSnapshot(m.dir, "")` → path resolves to `.maggus/runs//state.json` → fails → `m.snapshot = nil` → snapshot view never renders

**Snapshot path is separate from log path:**
The snapshot is still written to `.maggus/runs/<runID>/state.json` via `WriteSnapshot` (which calls `MkdirAll` to create the subdirectory), where `runID` is the daemon's run ID flag (e.g. `d-20260327-000328`). The log file is the flat `.maggus/runs/<timestamp>.log`. They live in the same directory but have different naming schemes and cannot be derived from each other. `findLatestRunLog` must find each independently.

## User Stories

### BUG-005-001: Fix findLatestRunLog to locate flat log files and snapshot directory

**Description:** As a user, I want the Output tab in `maggus status` to display live log entries and the snapshot view while the daemon is running so I can monitor task progress without reading raw files.

**Acceptance Criteria:**
- [x] `findLatestRunLog` scans for flat `.log` files in `.maggus/runs/` (excluding `daemon.log`) instead of directories; returns the path to the latest one as `logPath`
- [x] `findLatestRunLog` independently scans for subdirectories in `.maggus/runs/` that contain `state.json`; returns the latest directory name as `runID`
- [x] `m.daemon.LogPath` is non-empty when `.maggus/runs/` contains flat log files
- [x] `m.daemon.RunID` is non-empty when `.maggus/runs/` contains a snapshot directory with `state.json`
- [x] Output tab shows formatted JSONL log entries while the daemon is running (or from the last run when idle)
- [x] Snapshot view (spinner, tool list, token counts) renders when the daemon is running and a snapshot exists
- [x] `renderDaemonStatusLine` still shows last run info (the RunID display at line 263 can show a shortened form of the log filename or the snapshot dir name — whichever is non-empty)
- [x] No regression on projects where `.maggus/runs/` has no log files yet (function returns `"", ""` gracefully)
- [x] `go vet ./...` and `go test ./...` pass
