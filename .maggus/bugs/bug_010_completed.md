<!-- maggus-id: b16e4e9c-c69f-4cea-9609-1893f5ca0e70 -->
# Bug: Left pane scroll height off by one when bugs and features both visible

## Summary

When a repo contains both bug tickets and feature plans, a separator line `─` is rendered between them in the left pane tree. This separator is added inside the item render loop but is not counted by `treeAvailableHeight()`, causing the visible scroll window to be 1 row too short — the last item in the window gets trimmed.

## Steps to Reproduce

1. Open a repo that has both feature plans and bug tickets (`.maggus/features/` contains both `feature_*.md` and `bug_*.md` files, or bugs are present in the tree alongside features)
2. Have enough total items that scrolling is required
3. Scroll to the bottom of the list
4. Observe: the last item is cut off / one visible row at the bottom is always empty

## Expected Behavior

All `treeAvailableHeight()` rows should be occupied by items. The last item in the scroll window should be fully visible with no empty row at the bottom.

## Root Cause

`renderLeftPane` in `src/cmd/status_leftpane.go:167-169` conditionally appends a separator line between the bugs section and the features section:

```go
if !plan.IsBug && bugAdded && !bugSepAdded {
    bugSepAdded = true
    lines = append(lines, mutedStyle.Render(strings.Repeat("─", contentW-1)))
}
```

This separator is appended inside the item loop, so it consumes one row from the `height` budget. However, `treeAvailableHeight()` in `src/cmd/status_update.go:172-181` calculates the visible window size as `innerH - 6` (5 fixed header lines + the `innerH-1` adjustment), without knowing about this extra separator line.

As a result, `availH` items are sliced from `allItems`, but rendering emits `availH + 1` lines (items + separator). The trim at `status_leftpane.go:329` (`lines = lines[:height]`) then cuts the last item off.

The separator is not a tree item — it's injected ad-hoc during render. Because it lives outside `allItems`, neither the slice size nor `clampTreeScroll` knows it exists.

**Fix:** Add the separator as a dedicated `treeItemKindSeparator` entry in `buildTreeItems()`. This makes it a first-class tree item that is naturally included in the scroll window count, `treeAvailableHeight()`, and `clampTreeScroll()` — with no special-casing needed anywhere.

## User Stories

### BUG-010-001: Make the bugs/features separator a tree item

**Description:** As a user, I want the left pane to correctly show all visible items without the last row being cut off so that I can navigate the full list.

**Acceptance Criteria:**
- [x] A new `treeItemKindSeparator` kind is added to the tree item type in `status_leftpane.go` (or wherever `treeItemKind` is defined)
- [x] `buildTreeItems()` inserts a separator `treeItem` at the boundary between bugs and features instead of the ad-hoc injection in the render loop
- [x] The render loop in `renderLeftPane` handles `treeItemKindSeparator` by rendering the `─` separator line (same visual as before)
- [x] The ad-hoc separator injection at `status_leftpane.go:167-169` is removed
- [x] The separator item is not selectable (cursor navigation skips over it, or `clampTreeScroll` is separator-aware)
- [x] With both bugs and features present and scrolling active, all `treeAvailableHeight()` rows are occupied — no empty bottom row
- [x] No regression in repos with only features or only bugs (no separator rendered when only one type exists)
- [x] `go vet ./...` and `go test ./...` pass
