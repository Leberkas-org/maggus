# Feature 002: Consistent 2x Theme & Working Directory Visibility

## Introduction

Extend the 2x color scheme (yellow border + logo when Claude is in 2x mode) to all views consistently, and improve the visibility of the current working directory in both the main menu and the work view.

Currently, the 2x theme only affects the main menu (logo color + border). The status view and work view don't respond to 2x state at all. The working directory is shown in muted gray in the menu and absent from the work view entirely.

## Goals

- Apply 2x border color consistently across menu, status, and work views
- Apply 2x logo color in the menu (already done — maintain existing behavior)
- Make the working directory more visually prominent in the main menu
- Show the working directory in the work view header

## Tasks

### TASK-002-001: Propagate 2x status to the status view
**Description:** As a user, I want the status view border to turn yellow when Claude is in 2x mode so that I have a consistent visual cue across all views.

**Token Estimate:** ~30k tokens
**Predecessors:** none
**Successors:** none
**Parallel:** yes — can run alongside TASK-002-002, TASK-002-003, TASK-002-004

**Acceptance Criteria:**
- [x] `statusModel` fetches 2x status asynchronously (same pattern as menu: `claude2x.FetchStatus()` in `Init()`)
- [x] Status view border uses `styles.ThemeColor(is2x)` instead of the default `Primary`
- [x] When 2x is active, the status view border is yellow; when not, it stays cyan
- [x] Non-border elements (tabs, task icons, progress bar) remain unchanged
- [x] Typecheck/lint passes
- [x] Unit tests are written and successful

### TASK-002-002: Propagate 2x status to the work view
**Description:** As a user, I want the work view border to turn yellow when Claude is in 2x mode, consistent with the menu.

**Token Estimate:** ~25k tokens
**Predecessors:** none
**Successors:** none
**Parallel:** yes — can run alongside TASK-002-001, TASK-002-003, TASK-002-004

**Acceptance Criteria:**
- [ ] `BannerInfo` already carries `TwoXExpiresIn` — use it to derive `is2x` state (non-empty = true)
- [ ] `renderView()` uses `styles.ThemeColor(is2x)` as the default border color (replacing the hardcoded `styles.Primary`)
- [ ] The existing "stop-after-task → yellow border" behavior is preserved (stop override still takes priority or combines naturally since both use `Warning`)
- [ ] `renderBannerView()` also uses the 2x-aware border color
- [ ] Non-border elements (spinner, tabs, task ID) remain cyan
- [ ] Typecheck/lint passes
- [ ] Unit tests are written and successful

### TASK-002-003: Improve working directory visibility in the main menu
**Description:** As a user, I want the working directory in the main menu to be more prominent so I can quickly confirm which project I'm working in.

**Token Estimate:** ~20k tokens
**Predecessors:** none
**Successors:** none
**Parallel:** yes — can run alongside TASK-002-001, TASK-002-002, TASK-002-004

**Acceptance Criteria:**
- [ ] Working directory line uses a bolder style (e.g., `styles.Primary` foreground or bold) instead of `styles.Muted`
- [ ] The directory path remains left-truncated with "..." for long paths
- [ ] The directory is still centered in the header area
- [ ] Typecheck/lint passes
- [ ] Unit tests are written and successful

### TASK-002-004: Show working directory in the work view header
**Description:** As a user, I want to see the current working directory in the work view header so I know which project the agent is working on.

**Token Estimate:** ~30k tokens
**Predecessors:** none
**Successors:** none
**Parallel:** yes — can run alongside TASK-002-001, TASK-002-002, TASK-002-003

**Acceptance Criteria:**
- [ ] `BannerInfo` gains a `CWD string` field populated from `os.Getwd()` at work command startup
- [ ] `renderHeaderInner()` displays the working directory on a new line between version/fingerprint and the 2x status line
- [ ] Directory path is left-truncated with "..." if it exceeds the available width
- [ ] Style matches the menu's improved CWD style (from TASK-002-003)
- [ ] The CWD line also appears in `renderBannerView()`
- [ ] Typecheck/lint passes
- [ ] Unit tests are written and successful

## Task Dependency Graph

```
TASK-002-001 (status 2x border)
TASK-002-002 (work 2x border)
TASK-002-003 (menu CWD style)
TASK-002-004 (work CWD display)
```

All tasks are fully independent — no dependencies.

| Task | Estimate | Predecessors | Parallel | Model |
|------|----------|--------------|----------|-------|
| TASK-002-001 | ~30k | none | yes (with all) | — |
| TASK-002-002 | ~25k | none | yes (with all) | — |
| TASK-002-003 | ~20k | none | yes (with all) | — |
| TASK-002-004 | ~30k | none | yes (with all) | — |

**Total estimated tokens:** ~105k

## Functional Requirements

- FR-1: When Claude is in 2x mode, the border of the status view must be yellow (ANSI color 3)
- FR-2: When Claude is in 2x mode, the border of the work view must be yellow (ANSI color 3), unless overridden by stop-after-task state
- FR-3: The working directory in the main menu must use a colored/bold style instead of muted gray
- FR-4: The work view header must display the current working directory between the version line and the 2x status line
- FR-5: Long directory paths must be left-truncated with "..." prefix in all views

## Non-Goals

- No changes to internal element colors (spinner, tabs, task IDs, progress bars remain cyan)
- No changes to the logo — it already switches correctly in the menu
- No 2x status fetching in the work view — it reuses the value passed through `BannerInfo`
- No changes to plain/non-TUI output modes

## Technical Considerations

- The status view currently doesn't have 2x state at all — it needs an async fetch in `Init()` and a new field on `statusModel`
- The work view already receives `TwoXExpiresIn` via `BannerInfo` — derive `is2x` from that (non-empty string = true)
- `FullScreenLeftColor` already exists and accepts a border color parameter — the work view already uses it for the stop-state override
- The status view uses `FullScreen()` which doesn't accept a color — it should switch to `FullScreenColor()`

## Success Metrics

- All three views (menu, status, work) show yellow borders when Claude is in 2x mode
- Working directory is immediately noticeable in the main menu without reading closely
- Working directory is visible in the work view header at a glance

## Open Questions

*None — all questions resolved.*
