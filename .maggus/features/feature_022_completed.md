<!-- maggus-id: 429ced32-bbc1-49ab-8c25-87304040031d -->
# Feature 022: Home/End Key Navigation in Left Pane Tree

## Introduction

The left pane tree supports up/down navigation but has no way to jump directly to the first or last item. Adding Home and End key support lets users instantly reach either end of the list without repeatedly pressing arrow keys.

## Goals

- `Home` jumps the tree cursor to the first item (index 0)
- `End` jumps the tree cursor to the last item (last index in the visible tree)
- Both keys sync the plan cursor so the right pane updates correctly

## Tasks

### TASK-022-001: Handle Home and End keys in left pane tree navigation

**Description:** As a user navigating the left pane tree, I want to press Home to jump to the first item and End to jump to the last item so I can quickly reach either end of the list.

**Token Estimate:** ~10k tokens
**Predecessors:** none
**Successors:** none
**Parallel:** no
**Model:** haiku

**Acceptance Criteria:**
- [x] In `updateList` in `src/cmd/status_update.go`, a `case "home":` block sets `m.treeCursor = 0` when the left pane is focused and the tree has items, then calls `m.syncPlanCursorFromTreeCursor()`
- [x] A `case "end":` block sets `m.treeCursor = len(items) - 1` when the left pane is focused and the tree has items, then calls `m.syncPlanCursorFromTreeCursor()`
- [x] Both cases are guarded by `m.leftFocused` (same guard as the up/down cases)
- [x] Both cases are no-ops when `len(items) == 0`
- [x] `go build ./...` passes
- [x] `go vet ./...` passes

## Task Dependency Graph

```
TASK-022-001
```

| Task | Estimate | Predecessors | Parallel | Model |
|------|----------|--------------|----------|-------|
| TASK-022-001 | ~10k | none | no | haiku |

**Total estimated tokens:** ~10k

## Functional Requirements

- FR-1: Pressing `Home` while the left pane is focused moves `treeCursor` to 0 and syncs the plan cursor
- FR-2: Pressing `End` while the left pane is focused moves `treeCursor` to `len(items)-1` and syncs the plan cursor
- FR-3: Both keys are ignored when the right pane is focused

## Non-Goals

- No Home/End handling for the right pane or any other component
- No scroll-to-cursor logic (the tree already handles scrolling via cursor position)

## Technical Considerations

- Add the two cases alongside the existing `"up", "k"` and `"down", "j"` cases in `updateList` (`src/cmd/status_update.go`)
- The `items` slice is already built via `m.buildTreeItems()` earlier in that function — reuse it
- The bubbletea key string for the Home key is `"home"` and End is `"end"`

## Success Metrics

- Pressing Home from any position instantly selects the first tree item
- Pressing End from any position instantly selects the last tree item

## Open Questions

_None._
