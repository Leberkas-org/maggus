# Feature 001: Daemon Controls in Main Menu

## Introduction

Add daemon lifecycle management and log attachment directly into the main interactive menu. Currently, users must use separate `maggus start` / `maggus stop` CLI commands to manage the daemon. This feature brings those controls into the menu as a single context-sensitive toggle item, adds a persistent daemon status indicator in the menu header, and provides an "attach" option to view the daemon's live log output without leaving the menu workflow.

### Architecture Context

- **Vision alignment:** Reduces context-switching for users who primarily use the interactive menu as their entry point into Maggus.
- **Components involved:**
  - `cmd/menu.go` — main menu model, rendering, and item handling
  - `cmd/daemon_start.go` / `cmd/daemon_stop.go` — existing start/stop logic to reuse
  - `cmd/daemon.go` — PID helpers (`readDaemonPID`, `removeDaemonPID`)
  - `cmd/status_runlog.go` — `loadDaemonStatus()` and `daemonStatus` struct already used by the status TUI
  - `cmd/status.go` — status TUI entry point, to be invoked with live log pre-opened
- **New patterns:** Periodic polling tick in the menu model (mirrors the pattern already used in the status TUI at 200ms intervals).
- **Existing CLI commands:** `maggus start` / `maggus stop` are kept as-is; menu is an additional surface.

## Goals

- Show a live daemon status line ("● daemon running (PID X)" / "○ daemon not running") persistently in the main menu header.
- Provide a single toggle menu item that starts the daemon when it is not running and stops it when it is.
- Provide an "attach" menu item (only visible when daemon is running) that opens the status TUI with the live log panel pre-opened.
- Poll daemon state automatically so the menu stays in sync without user action.

## Tasks

### TASK-001-001: Add daemon state polling to menu model

**Description:** As a menu user, I want the menu to automatically know whether the daemon is running so that the header and menu items always reflect the current state.

**Token Estimate:** ~25k tokens
**Predecessors:** none
**Successors:** TASK-001-002, TASK-001-003, TASK-001-004
**Parallel:** no — this is the foundation for all other tasks

**Acceptance Criteria:**
- [x] A `daemonStatus` field (type `daemonStatus` from `status_runlog.go`) is added to `menuModel` in `menu.go`
- [x] A `menuDaemonTickMsg` message type is defined (mirrors the pattern in `status_runlog.go`)
- [x] A `pollMenuDaemonTick()` command fires every 500ms and returns `menuDaemonTickMsg`
- [x] The tick is started in `Init()` and re-queued in `Update()` on each `menuDaemonTickMsg`
- [x] On each tick, `loadDaemonStatus(dir)` is called and the result stored in `model.daemon`
- [x] No visual changes yet — this task is data plumbing only
- [x] `go test ./...` passes (no regressions)

---

### TASK-001-002: Render daemon status line in menu header

**Description:** As a menu user, I want to see whether the daemon is running at a glance so that I don't have to run a separate command to check.

**Token Estimate:** ~20k tokens
**Predecessors:** TASK-001-001
**Successors:** none
**Parallel:** yes — can run alongside TASK-001-003 and TASK-001-004 once TASK-001-001 is merged

**Acceptance Criteria:**
- [x] The menu `View()` renders a daemon status line above (or below) the existing feature summary header
- [x] When running: shows `● daemon running (PID 12345)` styled in cyan (match the style used in `status.go`)
- [x] When not running: shows `○ daemon not running` styled in a muted/dim color
- [x] The status line updates automatically as daemon state changes (driven by the poll from TASK-001-001)
- [x] Layout does not break at narrow terminal widths (clips gracefully)
- [x] `go test ./...` passes

---

### TASK-001-003: Add start/stop daemon toggle menu item

**Description:** As a menu user, I want a single menu item that starts or stops the daemon so that I can manage the daemon without leaving the menu or running separate CLI commands.

**Token Estimate:** ~40k tokens
**Predecessors:** TASK-001-001
**Successors:** none
**Parallel:** yes — can run alongside TASK-001-002 and TASK-001-004 once TASK-001-001 is merged

**Acceptance Criteria:**
- [ ] A new menu item is added to `allMenuItems` with shortcut `d`
- [ ] When daemon is **not** running: item label is `"start daemon"` with description `"Start the work loop as a background daemon"`
- [ ] When daemon **is** running: item label is `"stop daemon"` with description `"Stop the running daemon gracefully"`
- [ ] The item label and description update automatically based on `model.daemon.Running` (no restart required)
- [ ] Selecting "start daemon" launches the daemon using the existing `startCmd` logic (reuse or inline the same approach as `daemon_start.go` — generate a run ID, launch detached process, write PID file)
- [ ] Selecting "stop daemon" stops the daemon using the existing `stopCmd` logic (reuse `daemon_stop.go` — send graceful signal, wait up to 10s, force-kill if needed)
- [ ] After start/stop, the next poll tick updates the header status line to reflect the new state
- [ ] The item is positioned logically in the menu (e.g., after "work", before "status")
- [ ] `go test ./...` passes

---

### TASK-001-004: Add attach menu item (visible only when daemon is running)

**Description:** As a menu user, I want an "attach" option when the daemon is running so that I can watch what the daemon is doing without switching to a separate terminal command.

**Token Estimate:** ~30k tokens
**Predecessors:** TASK-001-001
**Successors:** none
**Parallel:** yes — can run alongside TASK-001-002 and TASK-001-003 once TASK-001-001 is merged

**Acceptance Criteria:**
- [ ] A new `"attach"` menu item is added with shortcut `l` (for "log")
- [ ] The item is **only rendered/selectable** when `model.daemon.Running` is true; hidden otherwise
- [ ] Selecting "attach" invokes the status TUI (`statusCmd`) with the live log panel already open (equivalent to pressing `l` immediately after opening status)
- [ ] This is implemented either by: (a) adding a `--show-log` flag to `statusCmd` that opens the log panel on init, or (b) passing an environment variable or model init option that the status TUI reads on startup
- [ ] When the user exits the status TUI (q / Ctrl+C), they are returned to the main menu
- [ ] The attach item description reads: `"Watch daemon output (live log)"`
- [ ] `go test ./...` passes

---

## Task Dependency Graph

```
TASK-001-001 ──→ TASK-001-002
             ──→ TASK-001-003
             ──→ TASK-001-004
```

| Task | Estimate | Predecessors | Parallel | Model |
|------|----------|--------------|----------|-------|
| TASK-001-001 | ~25k | none | no (foundation) | — |
| TASK-001-002 | ~20k | 001 | yes (with 003, 004) | — |
| TASK-001-003 | ~40k | 001 | yes (with 002, 004) | — |
| TASK-001-004 | ~30k | 001 | yes (with 002, 003) | — |

**Total estimated tokens:** ~115k

## Functional Requirements

- FR-1: The main menu must poll daemon state every 500ms and update header and menu items accordingly.
- FR-2: The daemon status line must display PID when daemon is running.
- FR-3: A single menu item with shortcut `d` must toggle between "start daemon" and "stop daemon" based on current state.
- FR-4: Starting the daemon from the menu must use the same detached-process mechanism as `maggus start`.
- FR-5: Stopping the daemon from the menu must use the same graceful-shutdown mechanism as `maggus stop` (10s timeout, then force-kill).
- FR-6: The "attach" item (shortcut `l`) must be hidden when no daemon is running.
- FR-7: The "attach" item must open the status TUI with the live log panel already visible.
- FR-8: Returning from the status TUI after attach must bring the user back to the main menu.
- FR-9: The existing `maggus start` and `maggus stop` CLI commands must continue to work unchanged.

## Non-Goals

- No new daemon management backend — reuse existing start/stop logic entirely.
- No streaming daemon stdout directly into the menu view (use the status TUI for that).
- No daemon restart option in this iteration.
- No multiple daemon support (one daemon per repo, as today).
- No changes to the daemon's internal behavior or log format.

## Technical Considerations

- `loadDaemonStatus()` and `daemonStatus` struct are already implemented in `cmd/status_runlog.go` — reuse directly, no duplication.
- The 500ms poll interval for the menu is intentionally slower than the status TUI's 200ms, since the menu header is lower priority.
- For TASK-001-004, prefer the `--show-log` flag approach (option a) as it keeps the status TUI self-contained and testable independently.
- The "attach" shortcut `l` may conflict with an existing binding — verify `allMenuItems` shortcuts before implementation and adjust if needed.
- Menu item visibility based on daemon state should use the same conditional rendering pattern as the `init` item (which is only shown when `.maggus` is not initialized).

## Success Metrics

- A user can start, stop, and attach to the daemon entirely from the main menu without running any separate CLI command.
- The daemon status line in the header is always accurate within 500ms of a state change.
- No regressions in existing `maggus start` / `maggus stop` behavior.

## Open Questions

_(none — all questions resolved)_
