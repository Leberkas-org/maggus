<!-- maggus-id: d6f9f3f3-bfa3-4aca-9b55-4b2cceae30b8 -->
<!-- maggus-id: 20260326-152242-feature-003 -->
# Feature 003: Status View — Tab Cleanup, Unified Header, and Daemon Controls

## Introduction

Several polish and usability improvements to the split-pane status view:

1. **Tab key removal** — Tab as a pane-switch toggle is confusing since `1–5` already covers all navigation. Tab becomes a no-op in split mode.
2. **Unified header** — The left pane header (`Features & Bugs`) currently looks different from the right pane's tab bar. Reformatting it as `1 Features & Bugs` (same number-prefix + label style) makes the whole `1–5` navigation feel like one coherent tab strip.
3. **Daemon status indicator** — The left pane header area shows whether the daemon is running or stopped, so the user always has that context at a glance.
4. **Daemon start/stop controls** — A single key (`s`) starts the daemon when it's stopped. When it's running, pressing `s` opens a small overlay asking whether to stop gracefully (after the current task finishes) or kill immediately.

### Architecture Context

- **Components involved:** `cmd/status_update.go` (key handling), `cmd/status_leftpane.go` (left pane render), `cmd/status_rightpane.go` (tab bar render for reference), `cmd/status_view.go` (footer hints), `cmd/daemon_start.go` / `cmd/daemon_stop.go` (daemon control)
- **No new files** — all changes within existing files
- **New state fields** on `statusModel`: `daemonStopOverlay bool` to track whether the stop-mode selection dialog is visible

## Goals

- `1–5` is the sole navigation mechanism; Tab does nothing in split mode
- Left pane header visually matches the right pane tab bar format
- Daemon running/stopped state is always visible in the left pane header area
- User can start or stop the daemon without leaving the status view
- When stopping, user explicitly chooses graceful (after current task) vs immediate kill

## Tasks

### TASK-003-001: Remove Tab toggle from split-pane key handler and update footer
**Description:** As a user navigating the split-pane status view, I want Tab to do nothing so that `1–5` is the only navigation method and Tab never accidentally shifts my focus.

**Token Estimate:** ~15k tokens
**Predecessors:** none
**Successors:** TASK-003-004
**Parallel:** yes — can run alongside TASK-003-002

**Acceptance Criteria:**
- [x] In `src/cmd/status_update.go` `updateList()`, the `if key == "tab"` block that toggles `m.leftFocused` in split mode is removed (the `m.width > 0 && m.height > 0` branch); pressing Tab in split mode is a no-op
- [x] The compact/legacy fallback inside the same `if key == "tab"` block (`m.showLog = true`) may be kept or removed — it is no longer the documented entry path and can be silently dropped
- [x] In `src/cmd/status_view.go`, all footer hint strings that contain `"tab: switch pane"` or `"tab:"` are removed and replaced with the correct updated hint (e.g. remove the tab hint entirely since `1–5` makes it redundant)
- [x] Pressing Tab while in the left or right pane has no visible effect

---

### TASK-003-002: Unify left pane header to match right pane tab bar style
**Description:** As a user of the status view, I want the left pane header to look like the right pane's tab labels so that `1 Features & Bugs  2 Output  3 Feature Details  4 Current Task  5 Metrics` feels like one unified tab strip.

**Token Estimate:** ~30k tokens
**Predecessors:** none
**Successors:** TASK-003-003
**Parallel:** yes — can run alongside TASK-003-001

**Acceptance Criteria:**
- [ ] In `src/cmd/status_leftpane.go` `renderLeftPane()`, the header line is changed from `" FEATURES & BUGS"` (all-caps plain text) to the format `<dim>1</dim> <label>Features & Bugs</label>` — matching the number-prefix + label convention used by `renderRightPaneTabBar()`
- [ ] When `m.leftFocused` is true: the number `1` is rendered faint/dim (matching the right pane's dimmed number prefixes), and `Features & Bugs` is rendered bold + underline + primary color (matching the active tab style in the right pane)
- [ ] When `m.leftFocused` is false: both the number and label are rendered in `styles.Muted` (matching the right pane's inactive tab style)
- [ ] The horizontal separator line immediately below the header is unchanged
- [ ] The `strings.ToUpper` call is removed — the new label is title-case `Features & Bugs`

---

### TASK-003-003: Add daemon status indicator to left pane header area
**Description:** As a user of the status view, I want to see immediately whether the daemon is running or stopped directly in the left pane header, so I don't have to look elsewhere.

**Token Estimate:** ~25k tokens
**Predecessors:** TASK-003-002
**Successors:** TASK-003-004
**Parallel:** no — builds directly on the header layout established in TASK-003-002

**Acceptance Criteria:**
- [ ] In `src/cmd/status_leftpane.go` `renderLeftPane()`, a second line is added immediately below the tab-style header (before the `─────` separator), showing the daemon status
- [ ] When `m.daemon.Running` is true: the indicator reads `● Running` rendered in `styles.Success` (green)
- [ ] When `m.daemon.Running` is false: the indicator reads `○ Stopped` rendered in `styles.Muted`
- [ ] When `m.daemon.Running` is true and `m.daemon.CurrentTask` is non-empty, the indicator appends the current task in muted text, truncated to fit the available content width: e.g. `● Running  TASK-001-002`
- [ ] The left pane's content area height calculation accounts for the extra line (the `statusHeaderLines` constant or equivalent height budget is updated if needed to prevent layout overflow)
- [ ] The `─────` separator line still appears immediately below the daemon status line

---

### TASK-003-004: Daemon start/stop key with stop-mode selection overlay
**Description:** As a user of the status view, I want to press `s` to start the daemon when it is stopped, or to open a stop-mode chooser when it is running, so I can control the daemon without leaving the status view.

**Token Estimate:** ~55k tokens
**Predecessors:** TASK-003-001, TASK-003-003
**Successors:** none
**Parallel:** no — depends on key handler cleanup (001) and the header/status indicator (003)
**Model:** opus

**Acceptance Criteria:**
- [ ] A new boolean field `daemonStopOverlay bool` is added to `statusModel` in `src/cmd/status_model.go`
- [ ] In `src/cmd/status_update.go` `updateList()`, pressing `s` when `m.daemon.Running` is false invokes the daemon start logic (reuse or call the same path as `daemon_start.go`) and returns; the daemon status indicator updates on the next poll tick
- [ ] In `src/cmd/status_update.go` `updateList()`, pressing `s` when `m.daemon.Running` is true sets `m.daemonStopOverlay = true` and returns — this opens the stop-mode chooser
- [ ] A new key handler `updateStatusDaemonStopOverlay(msg tea.KeyMsg)` (or equivalent inline) handles keys while `m.daemonStopOverlay` is true:
  - `t` or `T` — graceful stop (stop after current task): invokes the graceful stop path from `daemon_stop.go`, sets `m.daemonStopOverlay = false`
  - `k` or `K` — immediate kill: invokes the forceful stop/kill path from `daemon_stop.go`, sets `m.daemonStopOverlay = false`
  - `esc` or `ctrl+c` — cancel: sets `m.daemonStopOverlay = false`, no daemon action taken
- [ ] The overlay is checked before the normal key dispatch in `Update()`, similar to how `m.confirmDeleteFeature` is checked today
- [ ] In `src/cmd/status_view.go`, the overlay is rendered as a one- or two-line prompt in the footer when `m.daemonStopOverlay` is true: e.g. `Stop daemon:  t stop after task  k kill now  esc cancel` using `styles.StatusBar`
- [ ] Footer hints for the left pane focused state (when overlay is not shown) are updated to include `s: start` or `s: stop` depending on `m.daemon.Running`
- [ ] All existing keys (`q`, `esc`, `alt+p`, etc.) continue to work normally when the overlay is not open

## Task Dependency Graph

```
TASK-003-001 ──────────────────┐
TASK-003-002 ──→ TASK-003-003 ──→ TASK-003-004
```

| Task | Estimate | Predecessors | Parallel | Model |
|------|----------|--------------|----------|-------|
| TASK-003-001 | ~15k | none | yes (with 002) | — |
| TASK-003-002 | ~30k | none | yes (with 001) | — |
| TASK-003-003 | ~25k | 002 | no | — |
| TASK-003-004 | ~55k | 001, 003 | no | opus |

**Total estimated tokens:** ~125k

## Functional Requirements

- FR-1: Pressing Tab in split-pane mode must have no effect; `1–5` is the only pane/tab navigation
- FR-2: The left pane header must display as `1 Features & Bugs` using faint number + bold+underline+primary label when focused, and fully muted when unfocused
- FR-3: A daemon status line (`● Running` / `○ Stopped`) must appear in the left pane header area at all times
- FR-4: When the daemon is running and a current task is known, the task ID must be shown next to the running indicator (truncated to fit)
- FR-5: Pressing `s` when the daemon is stopped must start the daemon
- FR-6: Pressing `s` when the daemon is running must open a stop-mode overlay — no immediate action is taken until the user chooses
- FR-7: In the stop-mode overlay, `t` triggers graceful stop (after current task), `k` triggers immediate kill, `esc` cancels
- FR-8: The stop-mode overlay must replace the normal footer hint line while it is open
- FR-9: The footer must show `s: start` or `s: stop` as a hint based on `m.daemon.Running`

## Non-Goals

- No changes to the daemon's internal behavior — only the TUI control surface changes
- No new daemon subcommands or CLI flags
- No changes to the right pane tabs (Output, Feature Details, Current Task, Metrics)
- No changes to `--show-log` compact mode behavior beyond Tab key removal
- No confirmation for starting the daemon (start is always immediate)

## Technical Considerations

- `daemonStopOverlay` should be checked at the top of `Update()` alongside `confirmDeleteFeature` and `ConfirmDelete`, before routing to `updateList()`
- The daemon start invocation should reuse the existing start path in `daemon_start.go` — check whether it can be called as a function or must be exec'd as a subprocess; prefer function call if available
- `statusHeaderLines` constant (currently `11`) may need incrementing by 1 to account for the new daemon status line in TASK-003-003 — verify against the actual render to avoid off-by-one clipping
- The `renderRightPaneTabBar()` in `status_rightpane.go` is the visual reference for TASK-003-002: faint number + active style or inactive style

## Success Metrics

- Pressing Tab in the status view does nothing — no accidental pane jumps
- The full tab strip reads visually as `1 Features & Bugs  2 Output  3 Feature Details  4 Current Task  5 Metrics`
- A user can tell at a glance whether the daemon is running without pressing any key
- A user can start or stop the daemon with 2 keystrokes (`s` + `t`/`k`) without leaving the status view

## Open Questions

*(none)*
