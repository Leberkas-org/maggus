<!-- maggus-id: 925429b7-d862-4dcf-a0d0-247090fc1c9b -->
# Feature 056: Feature Output Tab — Per-Task Capping, Output Scroll, Tab Keys, and Tab Order

## Introduction

The feature output tab (`renderFeatureOutputTab`) currently builds a flat combined list of every tool entry from every task and lets the user scroll through the whole thing. Several UX problems make it hard to use:

1. **Too much data, no per-task budget.** A feature with 8 tasks, each with 60 tool calls, dumps 480 scroll lines into the view. There is no overview — you can't see what all tasks did at a glance.
2. **No way to scroll the output content.** Pressing Up/Down moves the left-pane cursor (feature/task selection), not the output scroll offset. Users on the Output tab have no keyboard way to scroll the content they are looking at.
3. **Tab switching conflicts with navigation.** Switching between right-pane tabs requires number keys or arrows that also move the left-pane cursor.
4. **Wrong default tab for completed tasks.** When a completed task is selected, the Detail/Summary tab is shown first. The Output tab — which contains the actual work history — should be first so it's immediately visible.

This feature fixes all four UX gaps:
- Each task is allocated a fixed line budget (`max(contentH / taskCount, 7)`) and shows its last N tool entries within that budget.
- `[N tools]` is shown in every task header so users can see how much work each task did.
- `Shift+↑` / `Shift+↓` scrolls the output content independently of the left-pane cursor.
- `Tab` / `Shift+Tab` cycles between right-pane tabs.
- Output is the first tab in the task view tab order so it is the default when selecting any task.

### Architecture Context

- **Vision alignment:** Observability of agent work is a core product value. This feature makes it practical to review a whole feature's work in one screen.
- **Components involved:** `cmd/status_feature_output.go` (render), `cmd/status_task_output.go` (shared helpers), status view key handler (update logic), status view model (tab index state).
- **New patterns:** None — extends existing `m.logScroll`, `m.logAutoScroll`, and tab-index patterns already established in the status view.

## Goals

- Show a compact, scannable summary of all tasks in the feature output tab — each task gets a proportional line budget, minimum 7 lines.
- Always show the *last* N tool entries per task (most recent activity visible without scrolling).
- Display `[N tools]` in every task header so users know total activity even when truncated.
- Allow scrolling the output area with `Shift+↑` / `Shift+↓` without interfering with left-pane navigation.
- Allow switching right-pane tabs with `Tab` (forward) and `Shift+Tab` (backward).
- Output is always the first tab in the task view so it is the default when selecting a task.

## Tasks

### TASK-056-001: Per-task line capping and tool count in feature output header

**Description:** As a user, I want each task in the feature output tab to show its last N tool entries (where N is derived from available height and task count) so I can see all tasks at a glance without scrolling through hundreds of lines.

**Token Estimate:** ~40k tokens
**Predecessors:** none
**Successors:** none
**Parallel:** yes — can run alongside TASK-056-002

**Acceptance Criteria:**
- [x] In `renderFeatureOutputTab`, compute `linesPerTask = max(contentH / max(len(plan.Tasks), 1), 7)` before building `allLines`
- [x] For each task with tool entries, only the last `linesPerTask` entries are passed to `buildToolLines` (i.e. `snap.ToolEntries[max(0, total-linesPerTask):]`); the full `total` count is retained separately for display
- [x] Tasks with no snapshot or zero tool entries show only the header line and no tool lines beneath (no placeholder, no empty lines — the header alone occupies the row)
- [x] `buildFeatureTaskHeader` signature is updated to accept `totalTools int`; when `totalTools > 0`, the meta section always includes `[N tools]` (e.g. `[80 tools]`); when `totalTools == 0`, no tool count is shown
- [x] The tool count renders in the same dimmed meta style as the existing token/cost/duration fields
- [x] The existing `renderScrollableToolList` call is unchanged — it still receives `allLines` and `contentH`; the scroll mechanism works over the now-shorter combined list
- [x] `go vet ./...` and `go test ./...` pass

---

### TASK-056-002: Key bindings — output scroll (Shift+↑/↓) and tab cycling (Tab/Shift+Tab)

**Description:** As a user, I want to scroll the output content with `Shift+↑` / `Shift+↓` and switch right-pane tabs with `Tab` / `Shift+Tab`, so I can navigate the status view without conflicting with left-pane cursor movement.

**Token Estimate:** ~45k tokens
**Predecessors:** none
**Successors:** none
**Parallel:** yes — can run alongside TASK-056-001

**Acceptance Criteria:**
- [x] `Shift+↑` decrements `m.logScroll` by 1 (clamped to 0); sets `m.logAutoScroll = false`; does NOT move the left-pane cursor
- [x] `Shift+↓` increments `m.logScroll` by 1; does NOT move the left-pane cursor; when the new offset equals the maximum scrollable offset (all content visible), re-enables `m.logAutoScroll = true`
- [x] `Shift+↑` / `Shift+↓` are active whenever the right pane shows an Output tab (both feature-level and task-level output views); they are no-ops on non-output tabs so they don't accidentally fire
- [x] `Tab` advances the right-pane tab index by 1, wrapping from the last tab back to the first
- [x] `Shift+Tab` moves the right-pane tab index back by 1, wrapping from the first tab to the last
- [x] `Tab` / `Shift+Tab` work regardless of which tab is currently active
- [x] Existing number-key and arrow-key tab switching (if any) is preserved — `Tab`/`Shift+Tab` are additive
- [x] A help hint for the new keys is visible somewhere in the status view (footer or key legend), e.g. `shift+↑↓ scroll  tab switch tab`
- [x] `go vet ./...` and `go test ./...` pass

---

### TASK-056-003: Make Output the first tab in the task view tab order

**Description:** As a user, I want the Output tab to be the first (leftmost) tab when I select a task, so the work history is immediately visible without having to switch tabs.

**Token Estimate:** ~20k tokens
**Predecessors:** none
**Successors:** none
**Parallel:** yes — can run alongside TASK-056-001 and TASK-056-002

**Acceptance Criteria:**
- [x] In the task view (when any task row is selected in the left pane — running, completed, pending, or blocked), the Output tab is at position 0 in the tab bar
- [x] The tab previously at position 0 (Detail or Summary) moves to position 1 or later; all other tabs shift accordingly
- [x] The default active tab when navigating to any task row is Output (index 0) — no special case needed since reordering achieves this
- [x] The feature view tab order (when a feature row is selected) is not changed by this task — only the task-level tabs are reordered
- [x] `Tab` / `Shift+Tab` cycling (from TASK-056-002) continues to work correctly after the reorder
- [x] Existing number-key tab shortcuts (e.g. `1` for first tab) map to the new order correctly
- [x] `go vet ./...` and `go test ./...` pass

---

## Task Dependency Graph

```
TASK-056-001  (independent)
TASK-056-002  (independent)
TASK-056-003  (independent)
```

| Task | Estimate | Predecessors | Parallel | Model |
|---|---|---|---|---|
| TASK-056-001 | ~40k | none | yes (with 002, 003) | — |
| TASK-056-002 | ~45k | none | yes (with 001, 003) | — |
| TASK-056-003 | ~20k | none | yes (with 001, 002) | haiku |

**Total estimated tokens:** ~105k

## Functional Requirements

- FR-1: `linesPerTask = max(floor(contentH / taskCount), 7)` where `contentH` is the height of the content area passed to `renderFeatureOutputTab` and `taskCount` is `len(plan.Tasks)`.
- FR-2: For each task, only the *last* `linesPerTask` entries of `snap.ToolEntries` are rendered. The total entry count (`len(snap.ToolEntries)`) is always shown in the header as `[N tools]`.
- FR-3: Tasks with no snapshot data (not started, blocked, or pending) render a header line only — no tool lines, no placeholder text.
- FR-4: Pressing `Shift+↑` while on the Output tab decrements `m.logScroll` by 1 (floor: 0) and disables auto-scroll.
- FR-5: Pressing `Shift+↓` while on the Output tab increments `m.logScroll` by 1 and re-enables auto-scroll when the offset reaches the scrollable maximum.
- FR-6: `Shift+↑` / `Shift+↓` must not move the left-pane cursor. The key event must be consumed before it reaches cursor-movement handling.
- FR-7: `Tab` cycles the right-pane tab index forward (wrapping); `Shift+Tab` cycles it backward (wrapping).
- FR-8: The key legend or footer must display hints for the new bindings when they are active.
- FR-9: In the task view tab bar, the Output tab must appear at index 0 (first position). The previous first tab moves to index 1.

## Non-Goals

- Per-task individual scroll (each task section is not independently scrollable — the whole list scrolls as one).
- Configurable `linesPerTask` minimum (7 is hardcoded).
- Mouse scroll support.
- Applying the line cap to the single-task output tab (`renderCompletedTaskOutput`) — that view has a fixed layout and its own height budget.
- Reordering the feature-row tab bar (only the task-row tab bar is reordered).

## Technical Considerations

- `linesPerTask` should be computed using integer division with the Go `max()` builtin (available since Go 1.21) or an inline `if` guard; avoid float division.
- `buildFeatureTaskHeader` is currently called with `(task, snap, isRunning, width)`. Adding `totalTools int` as a parameter is a simple signature extension; there is only one call site in `renderFeatureOutputTab`.
- The key handler for the status view is likely in `cmd/status_update.go` (per the Bubble Tea split pattern). Check for existing `logScroll` key handling there and add `Shift+↑` / `Shift+↓` near it.
- Tea key names for shifted arrows: `"shift+up"` and `"shift+down"` in Bubble Tea's `tea.KeyMsg.String()` representation.
- The right-pane tab index is likely an `int` field on `statusModel`. Wrap-around: `(current + 1) % tabCount` forward, `(current - 1 + tabCount) % tabCount` backward. Verify the total tab count constant/variable name before implementing.
- `m.logAutoScroll` re-enable on `Shift+↓`: the maximum scroll offset is `max(totalLines - viewHeight, 0)`. Since this value is computed inside `renderScrollableToolList`, the key handler cannot compute it directly. A practical approach: always increment on `Shift+↓`, let `renderScrollableToolList` clamp it, and re-enable auto-scroll only when `m.logScroll` is already at or past the max. Alternatively, check `m.logScroll >= totalLines - viewHeight` using stored state. Pick whichever is cleanest given the existing code.
- For TASK-056-003: the tab labels slice and the `switch m.activeTab` render dispatch must both be updated atomically. Find the tab label definition (likely a `[]string` or `const` block near the status model) and the switch/if-chain that renders each tab — reorder both to match. Any existing `case 0:` → Output, `case 1:` → Detail mapping must be updated. Also update any place where the active tab is initialised or reset on navigation (e.g. `m.activeTab = 0` on task selection) — since Output moves to 0, this reset already becomes correct.

## Success Metrics

- A feature with 10 tasks and 80 tool calls each is readable in a single screen without scrolling — each task shows its last N entries in a proportional column.
- `[80 tools]` appears in each task header.
- `Shift+↑` / `Shift+↓` scrolls the output content; Up/Down continues to move the left-pane cursor.
- `Tab` / `Shift+Tab` cycles through Output, Plan, Detail, and any other right-pane tabs.
- Selecting any task row shows the Output tab first by default.

## Open Questions

_(none)_
