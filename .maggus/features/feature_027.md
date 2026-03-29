<!-- maggus-id: cec9f993-9dc5-49b8-9d1c-f70ed6c15d85 -->
# Feature 027: Daemon State Singleton Service

## Introduction

When navigating from the status view back to the main menu, the daemon running indicator (e.g. "● daemon running (PID 1234)") is blank until the first 500ms poll tick fires. This is jarring — the user just left a view that was actively monitoring the daemon and the menu appears to have lost that knowledge.

The root cause: the menu and status view are separate sequential `tea.NewProgram` instances within the same process. Each time the menu is created, it starts with zero state and must wait for the first `pollMenuDaemonTick` to fire before populating the daemon indicator.

This feature introduces a **process-level `DaemonStateCache` singleton** — initialized once in `root.go`, persisting across all sequential TUI program instances. It watches `.maggus/daemon.pid` via fsnotify (already a dependency) for file-system events and notifies subscribers via channels. No polling for daemon running state.

### Architecture Context

- **No VISION.md or ARCHITECTURE.md present** — design follows patterns already established in the codebase.
- **Components involved:** `root.go` (process lifecycle), `menu_model/update.go` (menu consumer), `status_model/update.go` (status consumer), `status_runlog.go` (existing I/O functions to reuse).
- **Existing pattern to replicate:** `listenForWatcherUpdate` + `watcherCh chan string` — the status view already uses fsnotify + a channel listener for feature file changes. The daemon cache uses the exact same pattern.
- **New component introduced:** `DaemonStateCache` in `daemon_state_cache.go`.

## Goals

- Eliminate the daemon indicator delay when returning to the menu from status view
- Remove all polling for daemon running state (PID + running flag) — use fsnotify file-watch only
- Provide a single source of truth for daemon running state shared across all TUI views in the process
- Keep the status view's 200ms log poll for `RunID`, `LogPath`, `CurrentFeature`, `CurrentTask` — these require log file I/O and are status-view-specific

## Tasks

### TASK-027-001: Implement DaemonStateCache
**Description:** As a developer, I want a `DaemonStateCache` struct in a new file so that daemon PID/running state is loaded once, cached in memory, and updated via fsnotify without any polling.

**Token Estimate:** ~50k tokens
**Predecessors:** none
**Successors:** TASK-027-002, TASK-027-003
**Parallel:** no — other tasks depend on this

**Acceptance Criteria:**
- [x] New file `src/cmd/daemon_state_cache.go` created in `package cmd`
- [x] `daemonPIDState` struct defined with `PID int` and `Running bool` fields
- [x] `DaemonStateCache` struct defined with mutex-protected state, fsnotify watcher, subscriber list, and done channel
- [x] `NewDaemonStateCache(dir string) (*DaemonStateCache, error)` constructor watches the `.maggus/` directory (not the file directly, so it works even if `daemon.pid` doesn't exist yet)
- [x] Constructor calls `reload()` synchronously before returning so first `Get()` is always populated
- [x] `reload()` calls existing `readDaemonPID(dir)` and `isProcessRunning(pid)` (already in `package cmd`); only notifies subscribers when state actually changes
- [x] Background `loop()` goroutine filters fsnotify events to `daemon.pid` filename only; calls `reload()` on `Create | Write | Remove | Rename`
- [x] `Get() daemonPIDState` returns cached state under RLock (zero I/O)
- [x] `Subscribe() chan daemonPIDState` returns a buffered (size 1) channel and registers it
- [x] `Unsubscribe(ch chan daemonPIDState)` removes the channel from the subscriber list
- [x] `Stop()` closes the done channel, waits for the loop goroutine, and closes all remaining subscriber channels
- [x] `notify()` fans out to all subscriber channels using non-blocking sends (drops stale updates if channel is full)
- [x] `daemonCacheUpdateMsg` Bubble Tea message type defined
- [x] `listenForDaemonCacheUpdate(ch <-chan daemonPIDState) tea.Cmd` defined (mirrors `listenForWatcherUpdate` pattern exactly)
- [x] `go build ./...` passes

### TASK-027-002: Wire cache into root.go and menu
**Description:** As a developer, I want the `DaemonStateCache` initialized in `root.go` and the menu model to read from it so that the menu shows the correct daemon state on its very first rendered frame.

**Token Estimate:** ~40k tokens
**Predecessors:** TASK-027-001
**Successors:** TASK-027-004
**Parallel:** no

**Acceptance Criteria:**
- [x] `var daemonCache *DaemonStateCache` package-level var added in `root.go`
- [x] `daemonCache` initialized before the `for` loop in `runMenu()` with `defer daemonCache.Stop()`
- [x] If `NewDaemonStateCache` returns an error, `daemonCache` is nil and all call sites handle nil gracefully (no panic)
- [x] `menuModel` has `daemonCacheCh chan daemonPIDState` field
- [x] `newMenuModel()` subscribes via `daemonCache.Subscribe()` and immediately calls `daemonCache.Get()` to pre-populate `m.daemon.PID` and `m.daemon.Running`
- [x] `menuDaemonTickMsg` type deleted from `menu_update.go`
- [x] `pollMenuDaemonTick()` function deleted from `menu_update.go`
- [x] `menuModel.Init()` replaces `pollMenuDaemonTick()` with `listenForDaemonCacheUpdate(m.daemonCacheCh)`
- [x] `menuModel.Update()` handles `daemonCacheUpdateMsg`: updates `m.daemon.PID` and `m.daemon.Running`, returns `listenForDaemonCacheUpdate(m.daemonCacheCh)`
- [x] After `p.Run()` returns in `root.go`'s menu loop, `daemonCache.Unsubscribe(final.daemonCacheCh)` is called before the next iteration
- [x] `go build ./...` passes
- [x] Existing menu tests pass (`go test -run TestMenu ./cmd/...` or equivalent)

### TASK-027-003: Wire cache into status view
**Description:** As a developer, I want the status view to receive daemon PID/running state from the cache so that polling for those fields is eliminated and the status view responds instantly to daemon start/stop events.

**Token Estimate:** ~45k tokens
**Predecessors:** TASK-027-001
**Successors:** TASK-027-004
**Parallel:** yes — can run alongside TASK-027-002 (touches different files)

**Acceptance Criteria:**
- [ ] `statusModel` has `daemonCacheCh chan daemonPIDState` field
- [ ] In `runStatus()` (status_cmd.go): subscribe via `daemonCache.Subscribe()` and pre-populate `m.daemon.PID` and `m.daemon.Running` before `tea.NewProgram`
- [ ] `statusModel.Init()` adds `listenForDaemonCacheUpdate(m.daemonCacheCh)` to its batch
- [ ] `statusModel.Update()` handles `daemonCacheUpdateMsg`: updates `m.daemon.PID` and `m.daemon.Running`; if daemon just stopped (`prevRunning && !m.daemon.Running`), sets `m.snapshot = nil` immediately
- [ ] `daemonCacheUpdateMsg` returns `listenForDaemonCacheUpdate(m.daemonCacheCh)` to keep listening
- [ ] `logPollTickMsg` case in `status_update.go` no longer calls `loadDaemonStatus()` or reads PID/running; only reads log-file fields: calls `findLatestRunLog`, `readLastNLogLines`, `parseLogForCurrentState` to update `m.daemon.RunID`, `m.daemon.LogPath`, `m.daemon.CurrentFeature`, `m.daemon.CurrentTask`
- [ ] Snapshot read logic in `logPollTickMsg` unchanged
- [ ] After `prog.Run()` returns in `runStatus()`, `daemonCache.Unsubscribe(m.daemonCacheCh)` is called
- [ ] `go build ./...` passes

### TASK-027-004: Cleanup — remove loadDaemonStatus
**Description:** As a developer, I want the now-unused `loadDaemonStatus()` function removed so the codebase doesn't carry dead code.

**Token Estimate:** ~15k tokens
**Predecessors:** TASK-027-002, TASK-027-003
**Successors:** none
**Parallel:** no
**Model:** haiku

**Acceptance Criteria:**
- [ ] `loadDaemonStatus()` function deleted from `status_runlog.go`
- [ ] All sub-functions it used (`readDaemonPID`, `findLatestRunLog`, `readLastNLogLines`, `parseLogForCurrentState`) are kept — they are still called directly
- [ ] `go build ./...` passes with no unused function warnings
- [ ] `go test ./...` passes

## Task Dependency Graph

```
TASK-027-001 ──→ TASK-027-002 ──→ TASK-027-004
             └──→ TASK-027-003 ──┘
```

| Task | Estimate | Predecessors | Parallel | Model |
|------|----------|--------------|----------|-------|
| TASK-027-001 | ~50k | none | no | — |
| TASK-027-002 | ~40k | 001 | yes (with 003) | — |
| TASK-027-003 | ~45k | 001 | yes (with 002) | — |
| TASK-027-004 | ~15k | 002, 003 | no | haiku |

**Total estimated tokens:** ~150k

## Functional Requirements

- FR-1: When the user presses `q` in the status view and the menu renders, the daemon indicator must show the correct state (running or not) on the **first** rendered frame — no blank state, no delay
- FR-2: The `DaemonStateCache` must not use a polling timer for PID/running state — detection must be event-driven via fsnotify
- FR-3: The cache must handle the case where `.maggus/daemon.pid` does not exist at startup (daemon not running); `Get()` must return `{PID: 0, Running: false}`
- FR-4: If `NewDaemonStateCache` fails (e.g. `.maggus/` directory doesn't exist), all consumers must fall back gracefully to zero-value state — no crash
- FR-5: When an external process starts or stops the daemon (outside the TUI), the cache must detect the change via fsnotify and notify all subscribers
- FR-6: The 200ms `logPollTickMsg` in the status view must continue to update `RunID`, `LogPath`, `CurrentFeature`, `CurrentTask` — only the PID/running portion is replaced

## Non-Goals

- No changes to how the daemon process itself is started or stopped
- No changes to the 200ms log/snapshot poll in the status view (only the PID/running I/O is removed from it)
- No IPC between separate `maggus` process invocations (the singleton is in-process only)
- No changes to the `daemonStatus` struct shape (other consumers like tests that create it directly are unaffected)

## Technical Considerations

- **fsnotify is already a dependency** — `m.watcher *fsnotify.Watcher` and `m.watcherCh chan string` exist in `statusModel`. Replicate this pattern exactly for the cache.
- **Watch the directory, not the file** — watching `.maggus/` handles both the case where `daemon.pid` already exists and where it will be created later. File-level watches fail if the file doesn't exist.
- **Windows**: fsnotify may fire multiple events for a single write (e.g. `Create` + `Write`). The `changed` guard in `reload()` ensures only real state transitions trigger notifications.
- **Subscriber channel buffer = 1** with non-blocking sends: prevents the cache goroutine from blocking on a slow subscriber. Stale updates are silently dropped — the subscriber will get the next one.
- **`readDaemonPID` and `isProcessRunning`** are already in `package cmd` — no need to move or duplicate them.
- **`listenForDaemonCacheUpdate`** must re-register itself after each message (same as `listenForWatcherUpdate`) — the returned `tea.Cmd` must always call `listenForDaemonCacheUpdate` again in the Update handler.

## Success Metrics

- Zero visible delay on daemon indicator when returning to menu from status view
- No polling goroutines for daemon running state (verifiable by removing 500ms tick and confirming daemon start/stop is still detected)
- All existing tests pass

## Open Questions

_(none)_
