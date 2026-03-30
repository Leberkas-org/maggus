<!-- maggus-id: e700b299-69f9-403d-969b-368a2a03086d -->
# Feature 033: Stop-After-Task UX Refinements

## Introduction

Improve the stop-after-task signal flow so its behaviour matches user intent in two key scenarios:

1. **Status view exit path (`q` → `s`):** When the user chooses "stop after task" as part of quitting the status view, the app should navigate back to the main menu immediately. The daemon continues running in the background and stops on its own after the current task. The main menu should reflect this pending stop.

2. **Main menu exit while stopping after task:** If the daemon is already in "stopping after task" mode when the user quits the main menu, skip the stop/kill confirmation dialog and just exit the TUI, printing a brief CLI hint so the user knows the daemon is self-managing.

Additionally, the main menu daemon status line should display a "stopping after task" indicator so the state is visible without having to re-enter the status view.

### Architecture Context

- **No VISION.md / ARCHITECTURE.md** — proceeding from code investigation.
- **Components touched:** `DaemonStateCache` (`cmd/daemon_state_cache.go`), menu model/view/update (`cmd/menu_model.go`, `cmd/menu_view.go`, `cmd/menu_update.go`), status model (`cmd/status_model.go`, `cmd/status_update.go`).
- **Patterns in use:** `DaemonStateCache` with fsnotify already watches `.maggus/` directory and fans out `daemonPIDState` to subscribers; stop-after-task state is currently tracked only in-memory inside the status TUI model.
- **New pattern:** Extend `DaemonStateCache` to also surface `StoppingAfterTask bool` by checking sentinel file existence at notify time, making this state available to any subscriber (menu model included).

## Goals

- Main menu shows "stopping after task" in the daemon status line when the sentinel file is present.
- Quitting the main menu while stop-after-task is active skips the stop/kill overlay and exits cleanly with a one-line CLI hint.
- No change to the direct `s` keypress flow inside the status view (stays in status, shows indicator).
- The `q` → `s` path in the status exit overlay already navigates back; this feature ensures the main menu reflects the resulting state correctly.

## Tasks

### TASK-033-001: Extend DaemonStateCache to surface stop-after-task sentinel state

**Description:** As a developer, I want the `DaemonStateCache` to include `StoppingAfterTask bool` in its state notifications so that any subscriber (menu, future screens) can react to the sentinel file without duplicating file-watching logic.

**Token Estimate:** ~25k tokens
**Predecessors:** none
**Successors:** TASK-033-002, TASK-033-003
**Parallel:** no — foundational for both successors

**Acceptance Criteria:**
- [x] `daemonPIDState` struct gains a `StoppingAfterTask bool` field
- [x] `DaemonStateCache` checks for `.maggus/daemon.stop-after-task` existence each time it reloads state (same code path as PID check)
- [x] When the sentinel file appears or disappears, the cache emits a new notification to all subscribers
- [x] The cache correctly sets `StoppingAfterTask = false` when the daemon is not running (regardless of sentinel file)
- [x] Existing subscribers (status model) compile and function correctly — no regression in status view stop-after-task display
- [x] `go vet ./...` and `go test ./...` pass

---

### TASK-033-002: Update menu daemon status line to show "stopping after task"

**Description:** As a user, I want the main menu to show when the daemon is stopping after the current task so I don't need to re-enter the status view to check.

**Token Estimate:** ~20k tokens
**Predecessors:** TASK-033-001
**Successors:** none
**Parallel:** yes — can run alongside TASK-033-003

**Acceptance Criteria:**
- [ ] `menuModel` subscribes to the updated `daemonPIDState` and populates `daemon.StoppingAfterTask` (or an equivalent local field)
- [ ] `formatDaemonStatusLine()` in `menu_model.go` handles the stopping-after-task case:
  - When `Running == true && StoppingAfterTask == true`: renders `● daemon stopping after task (PID X)` in a visually distinct style (e.g., warning/yellow colour, or the same style as status view's `⏳ Stopping after task…`)
  - Existing `● daemon running` and `○ daemon not running` cases are unchanged
- [ ] When the daemon stops (PID file removed), the "stopping after task" display clears automatically via the cache notification
- [ ] `go vet ./...` and `go test ./...` pass

---

### TASK-033-003: Skip exit confirmation and print CLI hint when stop-after-task is active

**Description:** As a user, I want quitting the main menu to be instant when the daemon is already stopping after its current task, so I'm not asked to make a decision that's already been made.

**Token Estimate:** ~20k tokens
**Predecessors:** TASK-033-001
**Successors:** none
**Parallel:** yes — can run alongside TASK-033-002

**Acceptance Criteria:**
- [ ] In `menu_update.go`, when quit is triggered (`q` or exit menu item) and `daemonStoppingAfterTask == true`, the app does **not** show the `confirmStopDaemon` overlay
- [ ] Instead, the app exits the TUI and prints to stdout: `Daemon will stop after the current task completes.`
- [ ] Hint is printed via `tea.Println` (before `tea.Quit`) so it appears cleanly below the TUI
- [ ] When `daemonStoppingAfterTask == false` (daemon running normally), the existing stop/kill overlay behaviour is unchanged
- [ ] When daemon is not running at all, the existing direct-quit behaviour is unchanged
- [ ] `go vet ./...` and `go test ./...` pass

---

## Task Dependency Graph

```
TASK-033-001 ──→ TASK-033-002
             └─→ TASK-033-003
```

| Task | Estimate | Predecessors | Parallel | Model |
|------|----------|--------------|----------|-------|
| TASK-033-001 | ~25k | none | no | — |
| TASK-033-002 | ~20k | 001 | yes (with 003) | — |
| TASK-033-003 | ~20k | 001 | yes (with 002) | — |

**Total estimated tokens:** ~65k

## Functional Requirements

- FR-1: `DaemonStateCache` must detect `.maggus/daemon.stop-after-task` file presence and include it as `StoppingAfterTask bool` in every state notification.
- FR-2: The main menu daemon status line must render a distinct "stopping after task" variant when `Running == true && StoppingAfterTask == true`.
- FR-3: When the main menu receives a quit command and `StoppingAfterTask == true`, it must exit the TUI without showing the stop/kill overlay.
- FR-4: The CLI hint printed on exit must be exactly: `Daemon will stop after the current task completes.`
- FR-5: The direct `s` keypress path in the status view daemon stop overlay (not the exit overlay) must remain unchanged — it still sends the signal and stays in the status view.

## Non-Goals

- No changes to the status view stop-after-task display or keybindings.
- No changes to the `q` → `s` navigation in the status exit overlay (already navigates back to menu).
- No new stop-after-task trigger points beyond what already exists.
- No changes to the kill-daemon flow.

## Technical Considerations

- `DaemonStateCache` already uses `fsnotify` on `.maggus/` — file creation/deletion of `daemon.stop-after-task` will already trigger a reload event; the only change needed is to include the file existence check in the reload function.
- `tea.Println` queues output to appear after the alt-screen closes, which is the correct approach for the CLI hint.
- The `daemonStatus` struct (`status_runlog.go`) may need a `StoppingAfterTask bool` field added, or the menu model can use its own field sourced from `daemonPIDState` — whichever is cleaner given the existing data flow.

## Success Metrics

- After pressing `q` → `s` in the status view and returning to the main menu, the daemon status line reads "stopping after task" without any extra user action.
- Pressing `q` on the main menu in that state exits immediately with the hint printed — no overlay, no extra keypress.
