<!-- maggus-id: fd0e80b0-5f13-4843-a44d-994ecd8a2c34 -->
# Feature 028: Eliminate Remaining Polling

## Introduction

Two time-based polling loops remain in the codebase after the `DaemonStateCache` refactor replaced daemon-state polling in the status and menu TUIs. This feature removes both remaining polls and replaces them with event-driven patterns already established in the codebase.

### Architecture Context

- **Pattern in use:** `DaemonStateCache` (fsnotify + subscriber channels) already exists in `src/cmd/daemon_state_cache.go` and is consumed by status and menu TUIs
- **Components touched:** `src/cmd/repos.go` (repos TUI), `src/cmd/status_runlog.go` + `src/cmd/status_update.go` (status TUI log panel)
- **No new patterns introduced:** both tasks follow existing `DaemonStateCache` and `filewatcher` conventions

## Goals

- Remove the 500ms `reposDaemonTickMsg` polling loop in the repos TUI
- Remove the 200ms `logPollTickMsg` polling loop in the status TUI log panel
- Keep spinners, countdown timers, and the Windows stop-file ticker unchanged (those are UI animations or platform-required, not data polling)

## Tasks

### TASK-028-001: Replace repos TUI daemon polling with per-repo DaemonStateCache

**Description:** As a user of the repos screen, I want daemon running state to update instantly when a daemon starts or stops so that I don't see stale indicators for up to 500ms.

**Token Estimate:** ~60k tokens
**Predecessors:** none
**Successors:** none
**Parallel:** yes — can run alongside TASK-028-002

**Acceptance Criteria:**
- [x] One `DaemonStateCache` is created per repo entry when `reposModel` initialises (one per `globalconfig.Repository` in `m.repos`)
- [x] `daemonRunning[i]` is pre-populated from `cache.Get()` immediately at init — no flicker on first render
- [x] Each cache's subscriber channel is listened to with a per-repo Tea Cmd that delivers a `reposDaemonUpdateMsg{idx int, state daemonPIDState}` message
- [x] On receiving `reposDaemonUpdateMsg`, the model updates `daemonRunning[idx]` and re-schedules the listener for that specific cache channel (same fan-in pattern used in `status_update.go`)
- [x] `pollReposDaemonTick`, `reposDaemonTickMsg`, and `refreshDaemonStatus` are deleted
- [x] All `DaemonStateCache` instances are stopped (`.Stop()`) when the repos model exits or the program ends — no goroutine leaks
- [x] If a repo path has no `.maggus/` directory, `NewDaemonStateCache` returns an error — handle gracefully (log the error, leave `daemonRunning[i] = false`, skip subscription)
- [x] `go test ./cmd/...` passes (or new tests cover the fan-in path if no existing tests cover it)

**Key files:**
- `src/cmd/repos.go` — remove tick, add cache slice + subscriber fan-in
- `src/cmd/daemon_state_cache.go` — reuse as-is (no changes)

---

### TASK-028-002: Replace log file polling with fsnotify-based log watcher

**Description:** As a user watching the status TUI log panel, I want log lines to appear as soon as the daemon writes them rather than waiting up to 200ms, and I want the panel to not wake up at all when the daemon is idle.

**Token Estimate:** ~80k tokens
**Predecessors:** none
**Successors:** none
**Parallel:** yes — can run alongside TASK-028-001

**Acceptance Criteria:**
- [ ] A new `LogFileWatcher` struct (or equivalent) is created in `src/cmd/status_runlog.go` (or a new `src/cmd/log_watcher.go` if it keeps the existing file under 500 lines)
- [ ] `LogFileWatcher` uses `fsnotify` to watch `.maggus/runs/` for Create events (detects new run log files) and the currently active log file for Write events
- [ ] When a Write event fires on the active log file, a `logFileUpdateMsg` is delivered to the status TUI (same blocking-channel pattern as `listenForDaemonCacheUpdate`)
- [ ] When a Create event for a new `.log` file arrives in `.maggus/runs/`, `findLatestRunLog` is called, and if the active path changes the watcher swaps to the new file (stop watching old path, add new path)
- [ ] `logPollTickMsg` and `logPollTick` are deleted; all `case logPollTickMsg:` branches in `status_update.go` are replaced with `case logFileUpdateMsg:`
- [ ] `LogFileWatcher` has a `Stop()` method that closes the fsnotify watcher and its goroutine cleanly
- [ ] If fsnotify is unavailable or fails to init, fall back to the existing 200ms poll (keep `logPollTick` reachable as a fallback path, guarded by the error from `NewLogFileWatcher`)
- [ ] `go test ./cmd/...` passes

**Key files:**
- `src/cmd/status_runlog.go` — add `LogFileWatcher`, remove `logPollTick`/`logPollTickMsg`
- `src/cmd/status_update.go` — swap `logPollTickMsg` case for `logFileUpdateMsg`, init/stop watcher

---

## Task Dependency Graph

```
TASK-028-001 (repos polling)    ─── independent
TASK-028-002 (log polling)      ─── independent
```

| Task | Estimate | Predecessors | Parallel | Model |
|------|----------|--------------|----------|-------|
| TASK-028-001 | ~60k | none | yes (with 002) | — |
| TASK-028-002 | ~80k | none | yes (with 001) | — |

**Total estimated tokens:** ~140k

## Functional Requirements

- FR-1: The repos TUI must update `daemonRunning[i]` within one fsnotify event delivery of the actual daemon.pid change — no 500ms delay
- FR-2: Initial render of the repos TUI must show correct daemon running state immediately (populated from `cache.Get()` before first event)
- FR-3: The status TUI log panel must refresh within one fsnotify Write event of the daemon appending a log line
- FR-4: When a new run starts (new `.log` file created), the log watcher must automatically switch to the new file without requiring a TUI restart
- FR-5: All background goroutines introduced by both tasks must be stopped cleanly when their owning model exits

## Non-Goals

- The 500ms stop-file ticker in `daemon_keepalive.go` — intentional Windows behavior, do not touch
- Spinner animations (`spinnerTickMsg`, `tickMsg`, `updateTickMsg`, `syncTickMsg`) — UI animations, not polling
- Countdown timers (`claude2xTickMsg`) — UI timers, not polling
- Status-clear timers (`reposStatusClearMsg`) — one-shot UI timer, not polling

## Technical Considerations

- `DaemonStateCache` fan-in for N repos: the simplest pattern is one Tea Cmd per repo (each blocks on its own channel). On `reposDaemonUpdateMsg`, re-schedule only that repo's listener. This avoids a reflect-based select or goroutine fan-in.
- fsnotify on Windows: the existing codebase already uses fsnotify (in `daemon_state_cache.go` and `filewatcher/filewatcher.go`) on Windows without issues — no platform guards needed.
- Log file path changes: watch `.maggus/runs/` as a directory (catches Create events for new `.log` files). Also explicitly add the active log file path for Write events. On path swap, call `watcher.Remove(oldPath)` then `watcher.Add(newPath)`.
- File size limit: `status_runlog.go` is currently ~150 lines. Adding `LogFileWatcher` should stay within the 500-line limit; if it grows large, split into `log_watcher.go`.

## Verification

```bash
cd src && go build ./...
cd src && go test ./...
# Manual: run `maggus status` and start/stop a daemon — verify log panel updates immediately
# Manual: open repos screen — verify daemon indicators are correct on first render
```

## Open Questions

_(none)_
