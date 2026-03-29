<!-- maggus-id: e840d971-4c02-4252-8ac8-0dca3db0bad9 -->
# Feature 019: Tree View for Left Pane

## Introduction

Replace the flat feature/bug list in the left pane with a collapsible tree view. Each feature/bug row can be expanded to reveal its tasks as indented child rows. Navigation uses up/down to move through all visible rows and left/right to collapse/expand. The currently-active feature and task (as reported by the daemon) are highlighted with an animated spinner. Collapse state persists for the session.

## Goals

- Show tasks as collapsible children under each feature/bug in the left pane
- Let users navigate all visible rows (features and tasks) with up/down arrow keys
- Use left/right arrows to collapse/expand feature/bug rows (like a file-tree browser)
- Clearly indicate which feature and which task the daemon is currently processing via an animated spinner
- Show a progress summary (`done/total`) on each feature/bug row
- Remember collapse state for the session (survive plan reloads)

## Tasks

### TASK-019-001: Tree item model and state
**Description:** As a developer, I want a unified tree-item data model so that the left pane can represent both plan rows and task rows in a single navigable list.

**Token Estimate:** ~40k tokens
**Predecessors:** none
**Successors:** TASK-019-002, TASK-019-003
**Parallel:** no

**Acceptance Criteria:**
- [x] A `treeItem` struct is defined in `status_model.go` (or a new `status_tree.go`). It must carry: `kind` (plan or task), the `parser.Plan` it belongs to, and optionally the `parser.Task` (for task rows)
- [x] `statusModel` gains an `expandedPlans map[string]bool` field (keyed by `plan.ID`) — starts empty (all collapsed by default, per choice 1D: no initial expansion)
- [x] `statusModel` gains a `treeCursor int` field — replaces `planCursor` as the primary navigation index
- [x] A `buildTreeItems()` method on `statusModel` returns a `[]treeItem` reflecting the current expand state: for each visible plan, always emit one plan-row; if that plan's ID is in `expandedPlans`, also emit one task-row per task in `plan.Tasks` (all tasks, not just incomplete ones)
- [x] `selectedPlan()` is updated (or a new helper added) to derive the currently selected `parser.Plan` from `treeCursor` by walking `buildTreeItems()`
- [x] `rebuildForSelectedPlan()` continues to work correctly using the new selected-plan logic
- [x] `reloadPlans()` preserves the `expandedPlans` map across reloads (do not reset it)
- [x] All existing tests pass: `cd src && go test ./...`
- [x] `cd src && go vet ./...` reports no issues

---

### TASK-019-002: Render tree in left pane
**Description:** As a user, I want to see features and bugs rendered as a collapsible tree so that I can visually understand task structure and active progress at a glance.

**Token Estimate:** ~70k tokens
**Predecessors:** TASK-019-001
**Successors:** TASK-019-004
**Parallel:** yes — can run alongside TASK-019-003

**Acceptance Criteria:**
- [ ] `renderLeftPane` is rewritten to iterate over `buildTreeItems()` instead of `visiblePlans()`
- [ ] **Plan row** layout (left to right): cursor indicator (`▸` or space, 1 char) + expand/collapse icon (`▶` collapsed, `▼` expanded, space if no tasks, 1 char) + space + title (truncated) + progress badge (`done/total` in muted style) + approval badge (existing `✓`/`○` logic, right-aligned)
- [ ] **Task row** layout: 3-space indent + task ID (e.g. `TASK-019-001`, truncated) + space + task title fragment (truncated to remaining width)
- [ ] The plan row that contains the daemon's `CurrentFeature` shows an animated spinner character (from `styles.SpinnerFrames[m.spinnerFrame]`) immediately before the title — only while `m.daemon.Running`
- [ ] The task row whose task ID matches `m.daemon.CurrentTask` shows the same spinner character before the task ID — only while `m.daemon.Running`
- [ ] If no spinner is active (daemon stopped), neither row shows a spinner character; layout remains stable (no width jump)
- [ ] Selected row (at `treeCursor`) is highlighted with `selectedBg` background, same as before
- [ ] Features and bugs remain in separate sections with the existing `─` divider between them
- [ ] The progress badge format is `N/T` where N = `plan.DoneCount()` and T = `len(plan.Tasks)`; displayed in muted style; hidden if `len(plan.Tasks) == 0`
- [ ] All existing tests pass: `cd src && go test ./...`
- [ ] `cd src && go vet ./...` reports no issues

---

### TASK-019-003: Keyboard navigation for tree
**Description:** As a user, I want to navigate the tree with arrow keys so that I can move between features and tasks without leaving the left pane.

**Token Estimate:** ~45k tokens
**Predecessors:** TASK-019-001
**Successors:** TASK-019-004
**Parallel:** yes — can run alongside TASK-019-002

**Acceptance Criteria:**
- [ ] `up` / `k` moves `treeCursor` up by one through all visible tree items (wraps from top to bottom)
- [ ] `down` / `j` moves `treeCursor` down by one through all visible tree items (wraps from bottom to top)
- [ ] `right` / `l` on a **plan row**: adds the plan's ID to `expandedPlans` (expands it); rebuilds tree; does nothing if already expanded or if the plan has no tasks
- [ ] `left` / `h` on a **plan row**: removes the plan's ID from `expandedPlans` (collapses it); rebuilds tree; does nothing if already collapsed
- [ ] `left` / `h` on a **task row**: collapses the parent plan (same effect as pressing left on the plan row), then moves `treeCursor` to that plan row
- [ ] After any cursor movement, `rebuildForSelectedPlan()` is called if the selected plan changed, so the right pane stays in sync
- [ ] All pre-existing key bindings (Tab, Enter, `a`, `d`, `?`, etc.) continue to work correctly
- [ ] When the left pane is not focused (`m.leftFocused == false`), left/right arrows are NOT consumed by tree navigation (they should pass through or be ignored, same as before)
- [ ] All existing tests pass: `cd src && go test ./...`
- [ ] `cd src && go vet ./...` reports no issues

---

### TASK-019-004: Integration and polish
**Description:** As a user, I want the tree view to feel seamless — correct startup state, correct plan selection when the daemon is active, and no layout regressions.

**Token Estimate:** ~30k tokens
**Predecessors:** TASK-019-002, TASK-019-003
**Successors:** none
**Parallel:** no

**Acceptance Criteria:**
- [ ] On startup, all plans are collapsed (no plan in `expandedPlans`)
- [ ] When the daemon is running and `m.daemon.CurrentFeature` changes (detected in `logPollTickMsg` handler), the corresponding plan is automatically expanded (added to `expandedPlans`) so the active task row becomes visible
- [ ] When a plan reload happens (`reloadPlans`), `treeCursor` is clamped to the new tree length so it never goes out of bounds
- [ ] The daemon status line at the top of the left pane continues to show the running indicator and current feature/task text (existing behavior, no regression)
- [ ] With the left pane focused, navigating up/down through task rows correctly updates the selected plan in the right pane (right pane shows the parent plan's details, not a blank screen)
- [ ] No visual artifacts: tree rows have consistent width, approval badge stays right-aligned, selected-row highlight fills the full content width
- [ ] All existing tests pass: `cd src && go test ./...`
- [ ] `cd src && go vet ./...` reports no issues

## Task Dependency Graph

```
TASK-019-001 ──→ TASK-019-002 ──→ TASK-019-004
             └─→ TASK-019-003 ──┘
```

| Task | Estimate | Predecessors | Parallel | Model |
|------|----------|--------------|----------|-------|
| TASK-019-001 | ~40k | none | no | — |
| TASK-019-002 | ~70k | 001 | yes (with 003) | — |
| TASK-019-003 | ~45k | 001 | yes (with 002) | — |
| TASK-019-004 | ~30k | 002, 003 | no | — |

**Total estimated tokens:** ~185k

## Functional Requirements

- FR-1: The left pane must render a tree where plan rows are top-level nodes and task rows are indented children
- FR-2: Each plan row must display an expand/collapse icon (`▶`/`▼`) when the plan has tasks
- FR-3: Each plan row must display a progress summary in the format `N/T` (e.g. `3/7`) in muted style
- FR-4: Right-arrow or `l` on a collapsed plan row must expand it; left-arrow or `h` must collapse it
- FR-5: Left-arrow or `h` on a task row must collapse its parent plan and move focus to the plan row
- FR-6: Up/down navigation must traverse all visible rows (both plan rows and task rows)
- FR-7: When the daemon is running, the plan row matching `daemon.CurrentFeature` must display an animated spinner before its title
- FR-8: When the daemon is running, the task row matching `daemon.CurrentTask` must display an animated spinner before its task ID
- FR-9: The `expandedPlans` map must survive `reloadPlans()` calls (collapse state persists across file-watch reloads)
- FR-10: When `daemon.CurrentFeature` changes, that plan must be automatically expanded so its active task row is visible
- FR-11: Features and bugs must remain in separate sections with a `─` divider (existing behavior preserved)
- FR-12: The approval badge (`✓`/`○`) must remain right-aligned on plan rows

## Non-Goals

- Task rows are not independently selectable for right-pane detail (right pane always shows the parent plan's details)
- No drag-and-drop or multi-select
- No task-level actions (approve, delete) from within the tree
- No persistence of collapse state to disk across sessions
- No filtering or searching within the tree

## Technical Considerations

- `buildTreeItems()` must be cheap to call — it will be invoked on every `View()` render. Avoid allocating more than one slice per call.
- The spinner character width must be accounted for in the row layout. Reserve the spinner column unconditionally (1 char) and render either the spinner or a space — this prevents layout jitter as the spinner appears/disappears.
- `treeCursor` replaces `planCursor` as the primary index. `planCursor` can be removed or kept as a derived value — choose whichever causes fewer blast-radius changes in `status_update.go`.
- The `buildSelectableTasksForFeature` function (used by the right-pane task list) is separate from the tree task rows — do not merge them. Task rows in the tree show all tasks; the right-pane selectable list continues to respect `showAll`.
- File size rule: if `status_leftpane.go` or `status_model.go` grow beyond 500 lines after changes, split by responsibility.

## Success Metrics

- Opening the status view with an active daemon: the running feature is automatically expanded and the active task row is visible with a spinner
- Pressing right on any feature expands it instantly with no flicker
- Pressing left on a task row collapses the parent and lands on the plan row
- Up/down navigation moves smoothly through tasks and across feature boundaries
