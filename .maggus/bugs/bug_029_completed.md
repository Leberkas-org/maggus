<!-- maggus-id: 2194d594-f0ea-4bda-bf05-f7d699cd0e14 -->
# Bug: Left pane tree cuts off last item when all tasks are expanded

## Summary

When expanding features in the status view left pane (showing all tasks), the last task row is clipped and not visible. The scroll math allocates one more row than actually fits, so the bottom item is always hidden behind the border.

## Steps to Reproduce

1. Open `maggus` → status view
2. Press `alt+a` to show completed tasks (if needed to get a long list)
3. Expand a feature with many tasks (press right arrow)
4. Scroll to the bottom with `End` or repeated `Down`
5. Observe: the last task in the tree is not visible

## Expected Behavior

All items in the tree should be reachable and visible when scrolled to the bottom.

## Root Cause

`treeAvailableHeight()` in `src/cmd/status_update.go:260-269` uses `treeOverhead = 6`, but the correct value is `7`.

The calculation: `renderLeftPane` receives `innerH - 2` as its height (from `viewStatusSplit` at `status_view.go:156`). The left pane has 5 fixed header lines (Items label, separator, empty line, daemon status, separator). So available tree rows = `(innerH - 2) - 5 = innerH - 7`. But the code computes `innerH - 6`, which is one too many — the scroll system thinks there's room for one more row than actually renders.

The trim at `status_leftpane.go:309` (`if len(lines) > height { lines = lines[:height] }`) silently clips the overflow, hiding the last item.

## User Stories

### BUG-029-001: Fix treeAvailableHeight off-by-one

**Description:** As a user navigating the status view tree, I want to see all items including the last one so nothing is hidden.

**Acceptance Criteria:**
- [ ] `treeOverhead` constant in `treeAvailableHeight()` is `7` (was `6`)
- [ ] Comment accurately describes the calculation: renderLeftPane receives `innerH-2`, then 5 header lines
- [ ] Scrolling to the bottom of a fully expanded tree shows the last item
- [ ] No regression in scroll behavior (cursor clamping, context lines above/below)
- [ ] `go vet ./...` and `go test ./...` pass
