<!-- maggus-id: e0b7a922-182e-4271-90f8-fda82dc7aacf -->
# Bug: Status TUI does not update when daemon transitions from idle to active

## Summary

When the daemon is idle (previous run done) and then starts working on a newly approved feature, the status TUI stays frozen on the previous run's "Done" state. Output is only visible after quitting to the main menu and re-entering the status screen, which re-initialises the watcher and picks up current state.

## Steps to Reproduce

1. Run `maggus status` — observe a previous run shown as "Done"
2. Daemon is idle (no approved features)
3. Approve a feature — daemon wakes and starts working
4. Observe: status TUI still shows the old "Done" state
5. Press Esc → return to menu → re-enter status
6. Observe: current run output is now visible

## Expected Behavior

The status TUI should update within seconds of the daemon starting a new run, without requiring re-entry.

## Root Cause

`LogFileWatcher` (`src/cmd/log_watcher.go`) watches two things:
1. Write events on the active `.log` file (task output)
2. Create events for new `.log` files in `.maggus/runs/`

When the daemon transitions from idle to active, the first thing it writes is the **snapshot** (`state.json`) via `nullTUIModel.writeSnapshot()` (`src/cmd/daemon_tui.go:137-159`). The task `.log` file only receives writes later — once Claude Code starts producing actual output, which can take several seconds or longer.

`state.json` is at `.maggus/runs/state.json` — a fixed path. It is **never added to the fsnotify watcher** in `NewLogFileWatcher`. On Linux, watching `.maggus/runs/` only fires Create/Delete events for files in the directory, not Write events for existing files. So Write events to `state.json` are invisible to the watcher on all platforms.

Result: `listenForLogFileUpdate` stays blocked waiting for a Write event that doesn't arrive until Claude Code has been running for some time. The TUI remains on the stale "Done" snapshot throughout.

When the user re-enters the status screen, `NewLogFileWatcher` is called fresh — it finds the current log file and `state.json` immediately, showing the correct state.

## User Stories

### BUG-013-001: Watch state.json for Write events in LogFileWatcher

**Description:** As a user watching the status TUI, I want the display to update immediately when the daemon starts a new run so I don't have to re-enter the screen to see current output.

**Acceptance Criteria:**
- [x] `NewLogFileWatcher` in `src/cmd/log_watcher.go` adds `.maggus/runs/state.json` to the fsnotify watcher at init time (ignore error if file doesn't exist yet — daemon may not have run yet)
- [x] `LogFileWatcher` stores the `stateJsonPath` and watches it for Write events
- [x] `handleEvent` fires `logFileUpdateMsg{}` on Write events to `state.json`, using the same non-blocking send pattern as the existing handlers
- [x] If `state.json` does not exist at watcher creation time, it is added when first seen (Create event in `.maggus/runs/` for `state.json`)
- [x] When the daemon starts a new run from idle, the status TUI updates within one snapshot write cycle (no re-entry needed)
- [x] No regression: Write events on the active task log file and Create events for new task log files still fire correctly
- [x] `go vet ./...` and `go test ./...` pass
