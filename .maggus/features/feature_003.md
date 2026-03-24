# Feature 003: Daemon & Work Run Reliability Improvements

## Introduction

This feature addresses four reliability and correctness issues with the daemon and interactive work run:

1. **Agent output missing from run.log** — The structured run log captures task lifecycle and tool use events but silently drops the agent's actual text output. This makes post-mortem analysis impossible.
2. **Daemon log is empty** — The daemon uses `nullTUIModel` which discards all messages, so `daemon.log` contains nothing useful. It should contain the same structured events as a normal `run.log`.
3. **Daemon exits on no work** — When no workable tasks are found, the daemon process exits instead of waiting for new feature files to appear.
4. **Windows console windows** — On Windows, each task spawns a visible `cmd.exe`/console window. These must be suppressed and run fully in the background.
5. **No mutual exclusion** — A `maggus work` run and `maggus start` (daemon) can run simultaneously, causing task conflicts. Only one should be allowed at a time.

### Architecture Context

- **Components involved:** `internal/runlog`, `internal/agent` (claude.go + Windows procattr), `internal/filewatcher`, `cmd/daemon_start.go`, `cmd/daemon_tui.go`, `cmd/work.go`, `cmd/work_task.go`
- **Vision alignment:** Daemon reliability is core to the autonomous work loop — silent failures and ghost processes undermine the tool's purpose.
- **New patterns:** Work-run lock file (mirrors the existing daemon PID file pattern); daemon keep-alive loop with file watcher.

## Goals

- Agent text output is persisted to `run.log` in both interactive and daemon mode
- `daemon.log` contains the same structured log content as a normal `run.log`
- The daemon stays alive when there is no work, wakes up when feature files change
- No console/shell windows appear on Windows during daemon task execution
- Starting `maggus work` while a daemon is running (or vice versa) produces a clear error

## Tasks

### TASK-003-001: Add `Output()` method to runlog package
**Description:** As a developer diagnosing a past run, I want agent output text included in `run.log` so that I can reconstruct exactly what the agent said without re-running.

**Token Estimate:** ~15k tokens
**Predecessors:** none
**Successors:** TASK-003-002
**Parallel:** yes — can run alongside TASK-003-003, TASK-003-004

**Acceptance Criteria:**
- [x] `runlog.RunLog` has a new method `Output(taskID, text string)` that writes a structured log line at level `OUTPUT`
- [x] Log line format matches existing pattern: `YYYY-MM-DDTHH:MM:SSZ [OUTPUT] [TASK-NNN-NNN] <text>`
- [x] Long output text is written as-is (no truncation)
- [x] Existing runlog tests still pass; a new unit test covers the `Output()` method
- [x] `go test ./internal/runlog` passes

---

### TASK-003-002: Wire agent output text into run.log
**Description:** As a developer, I want agent output automatically written to `run.log` during every run so that both interactive and daemon modes capture the full agent conversation.

**Token Estimate:** ~45k tokens
**Predecessors:** TASK-003-001
**Successors:** none
**Parallel:** no

**Acceptance Criteria:**
- [x] `TUIModel` exposes a `SetOnOutput(fn func(taskID, text string))` callback, mirroring the existing `SetOnToolUse` pattern
- [x] `handleOutputMsg` in `tui_messages.go` invokes the callback when set
- [x] `nullTUIModel` in `daemon_tui.go` also stores and invokes an `onOutput` callback (same interface)
- [x] In `cmd/work_task.go` (or `cmd/work.go` where the TUI is wired), the callback is set to call `runLog.Output(currentTaskID, text)`
- [x] After a normal `maggus work` run, `run.log` contains `[OUTPUT]` lines with agent text
- [x] After a daemon run, `run.log` (inside the run directory) also contains `[OUTPUT]` lines
- [x] `go test ./...` passes

---

### TASK-003-003: Suppress Windows console windows for claude subprocess
**Description:** As a Windows user running the daemon, I want zero visible console windows during task execution so that the daemon truly runs silently in the background.

**Token Estimate:** ~20k tokens
**Predecessors:** none
**Successors:** none
**Parallel:** yes — can run alongside TASK-003-001, TASK-003-004
**Model:** haiku

**Acceptance Criteria:**
- [x] A new file `src/internal/agent/procattr_windows.go` (or the existing runner equivalent) adds `CREATE_NO_WINDOW (0x08000000)` to `SysProcAttr.CreationFlags` when building the `exec.Cmd` for the claude subprocess on Windows
- [x] A corresponding `procattr_other.go` provides a no-op / empty `SysProcAttr` for non-Windows platforms so the build stays cross-platform
- [x] No console window appears on Windows when running `maggus work` or the daemon
- [x] `go build ./...` succeeds on Windows (CreationFlags field exists in syscall package)
- [x] `go test ./...` passes

---

### TASK-003-004: Mutual exclusion — work-run lock file
**Description:** As a user, I want `maggus work` and `maggus start` to refuse to run if the other is already active, so that two agents never compete on the same task.

**Token Estimate:** ~40k tokens
**Predecessors:** none
**Successors:** TASK-003-005
**Parallel:** yes — can run alongside TASK-003-001, TASK-003-003

**Acceptance Criteria:**
- [x] A new helper `workPIDPath()` in `cmd/daemon.go` (or a new `cmd/workpid.go`) returns the path `.maggus/work.pid`
- [x] When `maggus work` starts (in `cmd/work.go` `RunE`), it checks whether `daemon.pid` exists and the daemon process is alive; if so, it exits with a clear error message: `"daemon is running (PID NNN) — stop it first with 'maggus stop'"`
- [x] When `maggus work` starts, it writes its own PID to `work.pid`; on exit (normal or error), it removes `work.pid`
- [x] When `maggus start` (daemon) is launched, it checks whether `work.pid` exists and the work process is alive; if so, it exits with a clear error: `"a work run is active (PID NNN) — wait for it to finish"`
- [x] Stale PID files (process no longer running) are silently removed, not treated as conflicts
- [x] `go test ./...` passes

---

### TASK-003-005: Daemon keep-alive with feature-file watcher and timeout fallback
**Description:** As a user, I want the daemon to stay running when there is no current work, automatically resuming when new feature files appear or after a timeout, so I never have to restart it manually.

**Token Estimate:** ~75k tokens
**Predecessors:** TASK-003-004
**Successors:** none
**Parallel:** no

**Acceptance Criteria:**
- [ ] When the work loop finishes with reason `no tasks` (or equivalent), the daemon does NOT exit; instead it enters a wait state
- [ ] In wait state, the daemon watches `.maggus/features/` for any file creation, modification, or deletion using the existing `filewatcher` package
- [ ] A configurable timeout (default 5 minutes) acts as a fallback poll interval — if no file change is detected within that window, the daemon re-checks for work anyway (prevents watcher misses)
- [ ] When a file change is detected (or the timeout fires), the daemon re-runs the full work loop from the top
- [ ] The timeout value is configurable in `.maggus/config.yml` under a `daemon_poll_interval` key (e.g., `"5m"`, `"30s"`); if absent, defaults to `5m`
- [ ] If the SIGTERM/stop signal arrives while the daemon is in wait state, it exits cleanly (no zombie process)
- [ ] Daemon log includes a `[INFO] no work found, watching for changes (timeout: 5m)` line when entering wait state
- [ ] Daemon log includes a `[INFO] file change detected: <path>` or `[INFO] poll timeout reached, rechecking` line when waking up
- [ ] `go test ./...` passes

---

## Task Dependency Graph

```
TASK-003-001 ──→ TASK-003-002
TASK-003-003
TASK-003-004 ──→ TASK-003-005
```

| Task | Estimate | Predecessors | Parallel | Model |
|------|----------|--------------|----------|-------|
| TASK-003-001 | ~15k | none | yes (with 003, 004) | — |
| TASK-003-002 | ~45k | 001 | no | — |
| TASK-003-003 | ~20k | none | yes (with 001, 004) | haiku |
| TASK-003-004 | ~40k | none | yes (with 001, 003) | — |
| TASK-003-005 | ~75k | 004 | no | — |

**Total estimated tokens:** ~195k

## Functional Requirements

- FR-1: `runlog.RunLog.Output(taskID, text)` writes a log line at level `OUTPUT` in the standard timestamped format
- FR-2: Every agent text block received via `agent.OutputMsg` triggers the `onOutput` callback and is written to `run.log`
- FR-3: Both `TUIModel` and `nullTUIModel` implement the `SetOnOutput` callback interface
- FR-4: The claude subprocess on Windows is started with `CREATE_NO_WINDOW` in `SysProcAttr.CreationFlags`; non-Windows builds compile cleanly with a no-op equivalent
- FR-5: `maggus work` writes its PID to `.maggus/work.pid` on start and removes it on exit
- FR-6: `maggus work` checks for a live daemon (via `daemon.pid`) and aborts with a descriptive error if one is found
- FR-7: `maggus start` checks for a live work run (via `work.pid`) and aborts with a descriptive error if one is found
- FR-8: Stale PID files from crashed processes are automatically cleaned up (not treated as live conflicts)
- FR-9: When no work is found, the daemon enters a wait loop instead of exiting
- FR-10: The wait loop uses `filewatcher` on `.maggus/features/` and wakes up on any file event
- FR-11: A timeout fallback (default 5 minutes, configurable via `daemon_poll_interval` in `config.yml`) re-triggers the work check even if no file event fires
- FR-12: A SIGTERM received during the wait loop causes a clean, immediate exit

## Non-Goals

- No changes to the interactive TUI rendering (tabs, layout, colours)
- No changes to how commits are made or feature files are parsed
- No distributed locking — this mutual exclusion is local to one machine only
- No daemon auto-restart on crash (out of scope)
- No streaming daemon.log output to the terminal via `maggus attach` (existing feature, unchanged)

## Technical Considerations

- The `filewatcher` package is already used in `internal/runner/tui.go` — reuse it rather than introducing a new dependency
- `CREATE_NO_WINDOW` is `0x08000000` in `golang.org/x/sys/windows` or `syscall`; check which import is already used in `runner/procattr_windows.go` and stay consistent
- The `daemon_poll_interval` config field should be parsed as a `time.Duration` string; use `time.ParseDuration` and fall back to `5 * time.Minute` on parse error or missing field
- The work-PID file must be removed even if `work.go` exits via `os.Exit`, a panic, or a signal — use `defer` plus a signal handler (consistent with how daemon PID is managed today)
- The wait loop in the daemon must respect the existing graceful-shutdown channel already threaded through the daemon's context

## Success Metrics

- `daemon.log` and `run.log` both contain `[OUTPUT]` lines after a task completes
- Zero console windows appear on Windows when the daemon processes a task
- The daemon process stays alive for 10+ minutes with no feature files and resumes within 2 seconds of a new feature file being written
- Running `maggus work` while the daemon is active prints a clear error and exits non-zero
- Running `maggus start` while a work run is active prints a clear error and exits non-zero

## Open Questions

_None — all questions resolved._
