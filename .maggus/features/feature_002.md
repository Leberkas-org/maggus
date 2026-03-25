<!-- maggus-id: 6451e052-a09a-48a3-abc9-fa03a8ac4c4d -->
# Feature 002: Enhanced Run Log TUI

## Introduction

Enhance the `maggus work` TUI run view with three improvements: fix a height calculation bug in the detail panel, replace the flat Progress tab with a 3-zone layout (top status, middle scrollable tools, bottom stats), and track active run elapsed time excluding idle gaps between tasks.

### Architecture Context

- **Components involved:** `internal/runner/tui_render.go` (rendering), `internal/runner/tui.go` (model state), `internal/runner/tui_keys.go` (key handling), `internal/runner/tui_tokens.go` (token tracking), `internal/runner/tui_messages.go` (message handlers)
- **No new components:** All changes are within the existing `runner` package TUI code
- **Existing patterns:** The TUI uses bubbletea with message-driven state updates, lipgloss for styling, and a tabbed layout inside a bordered fullscreen box

## Goals

- Fix the 2-line height mismatch between `detailAvailableHeight()` and the `renderDetailPanel()` call in `renderView()`
- Replace the Progress tab's flat layout with a 3-zone design: top (status/task/output), middle (scrollable compact tool list), bottom (accumulated stats)
- Show per-model accumulated token usage in the bottom stats zone
- Track and display active run elapsed time that excludes idle time between tasks (e.g. daemon waiting)
- Show average time per completed task in the stats zone

## Tasks

### TASK-002-001: Fix height calculation bug in detail panel
**Description:** As a user, I want the detail panel to render at the correct height so that content does not overflow or leave unexpected gaps at the bottom of the terminal.

**Token Estimate:** ~15k tokens
**Predecessors:** none
**Successors:** TASK-002-003
**Parallel:** yes -- can run alongside TASK-002-002

**Acceptance Criteria:**
- [x] In `renderView()` (tui_render.go line 668), the call `m.renderDetailPanel(innerW, innerH-8)` is changed to pass a height consistent with `detailAvailableHeight()` which reserves 10 lines (not 8)
- [x] `detailAvailableHeight()` and the actual height passed to `renderDetailPanel()` use the same calculation, eliminating the mismatch
- [x] The detail panel (tab 1) no longer overflows or leaves a 2-line gap at the bottom on standard terminal sizes (80x24, 120x40)
- [x] Existing detail scroll behavior still works correctly (auto-scroll, manual scroll, home/end)
- [x] Unit tests in `tui_render_test.go` are updated or added to verify height calculation consistency
- [x] `go vet ./...` passes

### TASK-002-002: Track active run elapsed time excluding idle
**Description:** As a user, I want the "Run elapsed" time to only count time spent actively working on tasks, so that idle gaps (e.g. daemon waiting for new tasks, between-task sync checks) are excluded from the displayed run time.

**Token Estimate:** ~30k tokens
**Predecessors:** none
**Successors:** TASK-002-003
**Parallel:** yes -- can run alongside TASK-002-001

**Acceptance Criteria:**
- [x] `TUIModel` has new fields to track active run time: an accumulated `activeRunDuration` (time.Duration) and a `taskActiveStart` (time.Time) that records when the current task began
- [x] On `IterationStartMsg`: `taskActiveStart` is set to `time.Now()`; if a previous task was active, its elapsed time is added to `activeRunDuration`
- [x] On task completion (when status becomes "Done" or "Failed" or "Interrupted"): the current task's elapsed time is added to `activeRunDuration`
- [x] A method `ActiveRunElapsed() time.Duration` returns `activeRunDuration` plus the current task's in-progress time (if a task is active)
- [x] The existing `runStartTime` field and wall-clock `runElapsed` remain available (not removed) for reference
- [x] Unit tests verify: active elapsed increases only during task execution, idle gaps are excluded, multiple task transitions accumulate correctly
- [x] `go vet ./...` passes

### TASK-002-003: Refactor Progress tab to 3-zone layout
**Description:** As a user, I want the Progress tab to show a fixed top zone (status, task info, output), a scrollable middle zone (compact tool list), and a fixed bottom zone (stats including per-model tokens and average time per task) so that I can see all key information at a glance while scrolling through tool usage.

**Token Estimate:** ~75k tokens
**Predecessors:** TASK-002-001, TASK-002-002
**Successors:** none
**Parallel:** no
**Model:** opus

**Acceptance Criteria:**
- [ ] **Top zone** (fixed, always visible): spinner + status, task ID + title, last output line, progress bar (same elements as current header + first lines of Progress tab)
- [ ] **Middle zone** (scrollable): compact tool list showing all tools in format `icon Type: Description  timestamp` (e.g. `📖 Read: src/cmd/work.go  14:32:05`), one line per tool, scrollable with arrow keys
- [ ] Middle zone auto-scrolls to the latest tool entry (same as current detail panel auto-scroll behavior)
- [ ] Middle zone supports manual scroll with up/down arrow keys; scrolling up disables auto-scroll, scrolling to bottom re-enables it
- [ ] Middle zone shows scroll indicator (e.g. `[1-10 of 47]`) when content overflows
- [ ] **Bottom zone** (fixed, always visible): model name, extras (skills/MCPs), commits count, elapsed time (task + active run + avg per task), accumulated token usage with per-model breakdown, cost
- [ ] Per-model token display format: one line per model, e.g. `opus: 45.2k in / 12.1k out  ·  sonnet: 8k in / 2k out`
- [ ] Average time per task is calculated from completed tasks in `tokens.usages` history
- [ ] Active run elapsed (from TASK-002-002) is displayed instead of wall-clock run elapsed
- [ ] Height calculation for the 3 zones is correct: top and bottom zones have fixed heights, middle zone gets the remaining space
- [ ] The Detail tab (tab 1) continues to work as before with full tool parameters and its own scroll state
- [ ] Tab switching (left/right arrows, number keys) still works; scroll keys work on tab 0 (middle zone) and tab 1 (detail panel) independently
- [ ] The middle zone scroll state is separate from the detail tab scroll state (independent `detailScrollOffset` vs new `progressScrollOffset`)
- [ ] Footer keybindings update to show scroll hints when on the Progress tab (tab 0)
- [ ] Rendering looks correct on standard terminal sizes (80x24, 120x40, 200x50)
- [ ] `go vet ./...` passes

## Task Dependency Graph

```
TASK-002-001 ──→ TASK-002-003
TASK-002-002 ──┘
```

| Task | Estimate | Predecessors | Parallel | Model |
|------|----------|--------------|----------|-------|
| TASK-002-001 | ~15k | none | yes (with 002) | -- |
| TASK-002-002 | ~30k | none | yes (with 001) | -- |
| TASK-002-003 | ~75k | 001, 002 | no | opus |

**Total estimated tokens:** ~120k

## Functional Requirements

- FR-1: The detail panel height calculation must be consistent between `detailAvailableHeight()` and the value passed to `renderDetailPanel()`
- FR-2: The Progress tab (tab 0) must display three fixed zones: top (status/task/output), middle (scrollable compact tools), bottom (stats)
- FR-3: The middle tool zone must show one tool per line in format: `icon Type: Description  timestamp`
- FR-4: The middle tool zone must support scrolling with up/down arrow keys and auto-scroll to latest
- FR-5: The bottom stats zone must show per-model accumulated token usage, one line per model
- FR-6: The bottom stats zone must show average elapsed time per completed task
- FR-7: Run elapsed time must exclude idle time between tasks (only active task execution time counts)
- FR-8: The Detail tab (tab 1) must continue to show full tool parameters with its own independent scroll state
- FR-9: All existing keybindings (tab switching, stop picker, ctrl+c) must remain functional

## Non-Goals

- No changes to the Detail tab (tab 1) rendering or behavior
- No changes to the Task tab (tab 2) or Commits tab (tab 3)
- No changes to the banner view or summary view
- No new tabs or removal of existing tabs
- No changes to the stop picker overlay
- No changes to how tool events are parsed from Claude Code's streaming JSON

## Technical Considerations

- The 3-zone layout needs careful height math: `topHeight` and `bottomHeight` are fixed based on content lines, `middleHeight = innerH - topHeight - bottomHeight - tabBar - footer`
- The middle zone reuses the same scroll/clamp pattern as the existing detail panel but with its own offset field (`progressScrollOffset`)
- Active elapsed tracking requires detecting task-active vs task-idle transitions; `IterationStartMsg` marks task start, and the token `saveAndReset` call (triggered by the next `IterationStartMsg` or summary) marks task end
- Per-model token data already exists in `tokenState.totalModelUsage` — just needs rendering
- Average time per task can be derived from `tokenState.usages` (which has `StartTime` and `EndTime` per task)

## Success Metrics

- Height bug is eliminated — detail panel renders cleanly without overflow on all common terminal sizes
- Progress tab shows all three zones simultaneously — no need to switch tabs to see tools AND stats
- Run elapsed time accurately reflects only active work time
- Per-model token breakdown is visible at a glance during the run

## Open Questions

None — all resolved.
