<!-- maggus-id: 4593fdcc-94ea-49e9-99ed-58d7515f1406 -->
# Feature 023: Left Pane Tree View Scrolling

## Introduction

The left pane tree has no scroll support — when the item list (plans + expanded task rows) is taller than the visible pane, items are silently cut off and unreachable. This feature adds cursor-follow scrolling to the tree: the view scrolls to keep the cursor visible, with a 2-line context buffer above and below.

### Architecture Context

- **Components touched:** `status_model.go` (new field), `status_update.go` (scroll helper + navigation wiring), `status_leftpane.go` (render update)
- **Pattern:** Mirrors existing scroll pattern used in the right-pane log/snapshot panel (`logScroll`, `maxLogScroll`)

## Goals

- The cursor is always visible in the left pane, even when the list is taller than the pane
- Expanding a plan keeps the cursor in view (task rows counted in height)
- 2 context lines of padding kept above and below the cursor when possible
- Home/End jump navigation also snaps the scroll to the correct position

## Tasks

### TASK-023-001: Add tree scroll offset and clamp helper

**Description:** As a developer, I want a `treeScrollOffset` field and a `clampTreeScroll()` helper so that the scroll position can be computed and clamped correctly.

**Token Estimate:** ~20k tokens
**Predecessors:** none
**Successors:** TASK-023-002
**Parallel:** no

**Acceptance Criteria:**
- [x] `treeScrollOffset int` field added to `statusModel` in `status_model.go`
- [x] `clampTreeScroll()` method added to `statusModel` in `status_update.go` with the following logic:
  - Compute `availH` = left pane inner height minus fixed header lines (the `[1] Items` label + separator + daemon status line + separator = 4 lines overhead; use `m.height` and `styles.FullScreenInnerSize` as reference, similar to how `rightPaneContentHeight()` works)
  - If `treeCursor < treeScrollOffset + 2`: pull offset up so cursor has 2 lines of context above (`treeScrollOffset = max(0, treeCursor - 2)`)
  - If `treeCursor >= treeScrollOffset + availH - 2`: push offset down so cursor has 2 lines of context below (`treeScrollOffset = treeCursor - availH + 3`)
  - Clamp `treeScrollOffset` to `[0, max(0, len(items) - availH)]`
- [x] `go build ./...` passes

### TASK-023-002: Wire scroll into navigation and render

**Description:** As a user, I want the tree to scroll automatically as I navigate so that I can always see the selected item and its context.

**Token Estimate:** ~45k tokens
**Predecessors:** TASK-023-001
**Successors:** none
**Parallel:** no
**Model:** opus

**Acceptance Criteria:**
- [ ] `clampTreeScroll()` is called after every cursor move in `updateList()` — up, down, home, end, shift+tab
- [ ] `clampTreeScroll()` is called after expand (`right`/`l`) and collapse (`left`/`h`) so newly revealed/hidden task rows don't leave the cursor off-screen
- [ ] `renderLeftPane()` in `status_leftpane.go` is updated to:
  - Call `m.buildTreeItems()` as before
  - Compute `availH` (same formula as in `clampTreeScroll`)
  - Slice the items slice: `visible := items[treeScrollOffset : min(treeScrollOffset+availH, len(items))]`
  - Render only the `visible` slice — remove the old trim/cut-off logic at the bottom of the function
  - Adjust cursor highlighting: the local index used for cursor comparison must account for the offset (`localIdx == treeCursor - treeScrollOffset`)
- [ ] With a list of 30+ tasks across multiple expanded plans, scrolling down with `j`/down arrow keeps the cursor always visible with ~2 lines of context below
- [ ] `go build ./...` passes

## Task Dependency Graph

```
TASK-023-001 ──→ TASK-023-002
```

| Task | Estimate | Predecessors | Parallel | Model |
|------|----------|--------------|----------|-------|
| TASK-023-001 | ~20k | none | no | — |
| TASK-023-002 | ~45k | 001 | no | opus |

**Total estimated tokens:** ~65k

## Functional Requirements

- FR-1: When the cursor moves down past `treeScrollOffset + availH - 2`, the view must scroll down so at least 2 rows remain visible below the cursor
- FR-2: When the cursor moves up past `treeScrollOffset + 2`, the view must scroll up so at least 2 rows remain visible above the cursor
- FR-3: Task rows under an expanded plan count as individual lines for scroll height calculation
- FR-4: Home key sets cursor to 0 and scroll offset to 0
- FR-5: End key sets cursor to last item and scroll offset to `max(0, len(items) - availH)`
- FR-6: After expand/collapse, scroll is re-clamped so the cursor row remains visible

## Non-Goals

- No scroll indicator or position hint in the UI
- No mouse scroll support
- No animation or smooth scrolling
- No changes to the right pane scroll behavior

## Technical Considerations

- `availH` must be computed consistently in both `clampTreeScroll()` and `renderLeftPane()` — consider extracting it as a helper `treeAvailableHeight() int` to avoid drift
- The existing old trim at the bottom of `renderLeftPane()` (the lines that cut off excess items) must be removed — the slice now handles this
- `treeScrollOffset` should be reset to 0 in any path that rebuilds the full plan list (e.g. `reloadPlans()`) to avoid stale offsets pointing past the end

## Open Questions

None.
