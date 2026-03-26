<!-- maggus-id: 7b141fa7-4e19-45cf-8850-a137ae87b021 -->
<!-- maggus-id: 20260326-143930-feature-002 -->
# Feature 002: Status View — Fix Initial View & Improve Focus Clarity

## Introduction

When navigating to the status view from the main menu, users land on the legacy log-panel view (`--show-log` mode) instead of the new split-pane layout. They must press Tab once to dismiss the log view and reveal the split pane. Additionally, once in the split pane, it is not visually clear which pane is active — the only indicator is the color of the thin `│` divider, which is easy to miss.

This feature fixes both issues:
1. Remove `--show-log` from the menu's status launch so the split pane is shown immediately
2. Make pane focus visually obvious via header color change and a contextual footer
3. Clean up the now-redundant Tab toggle in the legacy log view handler

### Architecture Context

- **Components involved:** `cmd/menu_update.go` (menu launch), `cmd/status_leftpane.go` (left pane render), `cmd/status_rightpane.go` (right pane render), `cmd/status_view.go` (split-pane layout), `cmd/status_update.go` (key handling)
- **Pattern:** Split-pane TUI with `leftFocused bool` state; focus changes routing of key input
- **No new components** — all changes are within existing files

## Goals

- Status opens directly in split-pane layout when launched from the main menu
- The focused pane is immediately obvious from its header color (primary when focused, muted when not)
- The right pane's tab bar is dimmed when focus is on the left pane, normal when focus is on the right
- A one-line contextual footer shows the relevant key hints for whichever pane is active
- The `--show-log` CLI flag continues to work; its Tab handler is simplified (Tab no longer toggles back to split pane)

## Tasks

### TASK-002-001: Remove `--show-log` from menu launch
**Description:** As a user navigating from the main menu, I want the status view to open directly in split-pane mode so I don't have to press Tab to reach the main view.

**Token Estimate:** ~10k tokens
**Predecessors:** none
**Successors:** none
**Parallel:** yes — can run alongside TASK-002-002

**Acceptance Criteria:**
- [x] In `src/cmd/menu_update.go`, the block `if item.name == "status" { m.args = []string{"--show-log"} }` is removed
- [x] Launching status from the menu opens the split-pane view immediately (no log view shown first)
- [x] `maggus status --show-log` still works when passed on the CLI directly

---

### TASK-002-002: Make split-pane focus visually clear
**Description:** As a user in the split-pane status view, I want the focused pane to be visually obvious so I know which pane responds to my keystrokes.

**Token Estimate:** ~35k tokens
**Predecessors:** none
**Successors:** TASK-002-003
**Parallel:** yes — can run alongside TASK-002-001

**Acceptance Criteria:**
- [ ] In `src/cmd/status_leftpane.go`: the "FEATURES & BUGS" header text renders in `styles.ThemeColor(m.is2x)` (primary color) when `m.leftFocused` is true, and in `styles.Muted` when false
- [ ] In `src/cmd/status_rightpane.go`: `renderRightPaneTabBar()` dims all tab labels (faint/muted) when `m.leftFocused` is true; renders normally (active tab bold+underline+primary, inactive muted) when `m.leftFocused` is false
- [ ] With left pane focused: left header is primary color, right tab bar is all-muted
- [ ] With right pane focused: left header is muted, right tab bar is normal
- [ ] The `│` divider color behavior is unchanged (it can stay as a secondary indicator)

---

### TASK-002-003: Add contextual footer hints to split-pane view
**Description:** As a user in the split-pane status view, I want a footer showing relevant key hints for the currently focused pane so I know what actions are available.

**Token Estimate:** ~25k tokens
**Predecessors:** TASK-002-002
**Successors:** none
**Parallel:** no — depends on the focus state work in TASK-002-002

**Acceptance Criteria:**
- [ ] `src/cmd/status_view.go` `viewStatusSplit()` passes a non-empty footer string to `styles.FullScreenLeftColor()`
- [ ] A helper method `statusSplitFooter()` (or inline logic) generates the footer based on `m.leftFocused` and `m.activeTab`
- [ ] Left pane focused footer: `↑/↓ navigate  enter: details  tab: switch pane  alt+p: approve  q: exit`
- [ ] Right pane focused, tab 0 (Output): `↑/↓ scroll  G: bottom  1-4: tabs  tab: switch pane  q: exit`
- [ ] Right pane focused, tab 1 (Feature Details): `↑/↓ navigate  enter: detail  1-4: tabs  tab: switch pane  q: exit`
- [ ] Right pane focused, tab 2 (Current Task): `↑/↓ scroll  1-4: tabs  tab: switch pane  q: exit`
- [ ] Right pane focused, tab 3 (Metrics): `1-4: tabs  tab: switch pane  q: exit`
- [ ] Footer is rendered using `styles.StatusBar` style for visual consistency with other views

---

### TASK-002-004: Remove Tab toggle from legacy log-view key handler
**Description:** As a developer maintaining the codebase, I want the legacy `showLog` key handler to be cleaned up now that Tab-to-split-pane is no longer needed from the menu path.

**Token Estimate:** ~10k tokens
**Predecessors:** TASK-002-001
**Successors:** none
**Parallel:** no — should be done after TASK-002-001 to confirm log view is no longer the default entry point

**Acceptance Criteria:**
- [ ] In `src/cmd/status_update.go` `updateList()`, the `case "tab": m.showLog = false` block (lines ~296–304) is removed from the `if m.showLog` handler
- [ ] When in `--show-log` mode, pressing Tab no longer switches to the split pane (Tab is now a no-op or consumed silently)
- [ ] `q`/`esc` still exits from `--show-log` mode
- [ ] `j`/`k`/`g`/`G` scroll keys still work in `--show-log` mode

## Task Dependency Graph

```
TASK-002-001 ──→ TASK-002-004
TASK-002-002 ──→ TASK-002-003
```

| Task | Estimate | Predecessors | Parallel | Model |
|------|----------|--------------|----------|-------|
| TASK-002-001 | ~10k | none | yes (with 002) | — |
| TASK-002-002 | ~35k | none | yes (with 001) | — |
| TASK-002-003 | ~25k | 002 | no | — |
| TASK-002-004 | ~10k | 001 | no | — |

**Total estimated tokens:** ~80k

## Functional Requirements

- FR-1: `maggus status` launched from the main menu must open in split-pane mode without any intermediate log view
- FR-2: The focused pane's left-pane header must use primary color when focused and muted color when not
- FR-3: The right pane's tab bar must be visually dimmed (all tabs faint/muted) when the left pane has focus
- FR-4: The split-pane footer must show key hints relevant to the currently focused pane and active tab
- FR-5: `maggus status --show-log` must still show the log view; pressing Tab while in that view no longer switches to split pane
- FR-6: All existing key behaviors in both panes must be preserved

## Non-Goals

- No changes to the log view's content or layout
- No changes to the right-pane tab content (Output, Feature Details, Current Task, Metrics)
- No changes to how Tab toggles `leftFocused` in split mode
- No changes to the `│` divider color behavior
- No other TUI views (menu, work, list) are affected

## Technical Considerations

- `styles.ThemeColor(m.is2x)` returns the primary color normally and yellow in 2x mode — use this for the focused left header to stay consistent with the border
- `renderRightPaneTabBar()` currently has no access to `leftFocused` — it is a method on `statusModel`, so `m.leftFocused` is accessible directly without changing the signature
- The footer string in `viewStatusSplit()` is currently `""` — changing it to a non-empty string will make `styles.FullScreenLeftColor` render a bottom status bar
- The `styles.StatusBar` style should be used for footer text to match existing footer rendering in other views (e.g. `viewLog()`)

## Success Metrics

- Users navigating from the main menu land directly in the split-pane view without needing to press Tab
- In a quick usability check, a user can immediately tell which pane is active without reading documentation

## Open Questions

*(none)*
