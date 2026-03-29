<!-- maggus-id: 94198141-4c7c-4a17-b679-32ba9555a005 -->
# Feature 032: PageUp/PageDown hop between plan rows in the status left pane

## Introduction

In the status view's left pane, `↑`/`↓` moves one row at a time — including through
individual task rows when a plan is expanded. There is no way to quickly jump between
plans (features/bugs) without stepping through every expanded task row one by one.

This feature binds `pgup` and `pgdn` to "jump to previous/next plan row", skipping over
task rows and separator rows entirely. The behaviour mirrors `↑`/`↓` in every other
respect: the right pane is refreshed when the selected plan changes and the tree scroll
offset is clamped to keep the cursor visible.

### Architecture Context

- **Components involved:** `cmd/status_update.go` (key handling in `updateList`),
  `cmd/status_tree.go` (tree item types — read-only), `cmd/status_view.go` (footer hint)
- **New patterns introduced:** none — extends the existing left-pane cursor movement pattern

## Goals

- `pgdn` moves the cursor to the **next** `treeItemKindPlan` row below the current cursor,
  regardless of whether the current cursor is on a plan row or a task row.
- `pgup` moves the cursor to the **previous** `treeItemKindPlan` row above the current
  cursor, regardless of whether the current cursor is on a plan row or a task row.
- If no next/previous plan row exists (cursor is already at the last/first plan), the
  cursor stays on the current plan row — it does **not** wrap around.
- The footer hint is updated to document the new keys.

## Tasks

### TASK-032-001: Add pgup/pgdn plan-hopping to the status left pane
**Description:** As a user, I want `pgup` and `pgdn` to jump between plan (feature/bug)
rows in the status view left pane so I can navigate large task trees quickly without
stepping through every expanded task row.

**Token Estimate:** ~30k tokens
**Predecessors:** none
**Successors:** none
**Parallel:** no

**Acceptance Criteria:**
- [ ] Two pure helper functions are added — `findNextPlanRow(items []treeItem, cursor int) int`
  and `findPrevPlanRow(items []treeItem, cursor int) int`:
  - `findNextPlanRow` returns the index of the first `treeItemKindPlan` row whose index
    is **strictly greater** than `cursor`; returns `cursor` if none exists
  - `findPrevPlanRow` returns the index of the last `treeItemKindPlan` row whose index
    is **strictly less** than `cursor`; returns `cursor` if none exists
  - Separator rows (`treeItemKindSeparator`) are never returned by either helper
- [ ] In `updateList` (`status_update.go`), `pgdn` and `pgup` key cases are added to the
  left-pane focused block (alongside `up`/`down`/`home`/`end`):
  - `pgdn` → set `m.treeCursor = findNextPlanRow(items, m.treeCursor)`, then call
    `m.clampTreeScroll()`, `m.syncPlanCursorFromTreeCursor()`, and `m.rebuildRightPane()`
    if the selected plan changed
  - `pgup` → set `m.treeCursor = findPrevPlanRow(items, m.treeCursor)`, then the same
    three follow-up calls
- [ ] When the cursor is already on the last plan row, `pgdn` leaves the cursor unchanged
- [ ] When the cursor is already on the first plan row, `pgup` leaves the cursor unchanged
- [ ] When the cursor is on a task row, `pgdn` jumps to the next plan row below the parent
  plan (not back to the parent itself); `pgup` jumps to the parent plan row (the first
  plan row strictly above the cursor)
- [ ] The left-pane footer hint (`statusSplitFooter` in `status_view.go`) is updated to
  include `pgup/pgdn: prev/next feature`
- [ ] Unit tests for `findNextPlanRow` and `findPrevPlanRow` cover: cursor on plan row,
  cursor on task row, cursor at first plan, cursor at last plan, list with separator,
  list with no expanded tasks
- [ ] `go build ./...` passes
- [ ] `go test ./...` passes

## Task Dependency Graph

```
TASK-032-001
```

| Task | Estimate | Predecessors | Parallel | Model |
|------|----------|--------------|----------|-------|
| TASK-032-001 | ~30k | none | no | — |

**Total estimated tokens:** ~30k

## Functional Requirements

- FR-1: Pressing `pgdn` when the left pane is focused MUST move the cursor to the next
  `treeItemKindPlan` row below the current cursor position.
- FR-2: Pressing `pgup` when the left pane is focused MUST move the cursor to the previous
  `treeItemKindPlan` row above the current cursor position.
- FR-3: If no such row exists in the requested direction, the cursor MUST remain unchanged.
- FR-4: `pgup`/`pgdn` MUST NOT be handled when the left pane is not focused (i.e. when
  `m.leftFocused` is false) — the existing right-pane scroll behaviour for those keys
  (if any) must be unaffected.
- FR-5: After moving, `clampTreeScroll` MUST be called so the cursor stays visible.
- FR-6: If the newly selected plan differs from the previously selected plan,
  `rebuildRightPane` MUST be called to keep the right pane in sync.
- FR-7: The footer hint for the left pane MUST be updated to show `pgup/pgdn: prev/next feature`.

## Non-Goals

- No change to `↑`/`↓` behaviour.
- No wrapping: `pgdn` at the last plan does not jump to the first.
- No change to right-pane key handling — `pgup`/`pgdn` in the right pane (if already
  bound there) are unaffected.
- No animation or visual transition between plan rows.

## Technical Considerations

- `buildTreeItems()` is cheap (allocates one slice); calling it twice in a single Update
  cycle (once for the old cursor, once for clamping) is acceptable.
- `treeItemKindSeparator` rows must be skipped: a separator is neither a plan nor a task
  and should never be the target of navigation.
- The helpers should be defined alongside the existing `skipSeparatorUp` /
  `skipSeparatorDown` helpers in `status_update.go` for consistency.

## Success Metrics

- In a status view with multiple expanded features, `pgdn` reaches each feature header
  in one keystroke regardless of how many task rows each feature contains.
- `go test ./...` passes with the new unit tests for the helper functions.

## Open Questions

_(none — all resolved before saving)_
