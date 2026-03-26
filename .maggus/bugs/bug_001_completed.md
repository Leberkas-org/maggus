<!-- maggus-id: e34d367a-846f-4c2c-b2bb-61ead2583ac6 -->
# Bug: Status view shows blank frame on startup instead of split-pane

## Summary

When `maggus status` opens, the first rendered frame is blank — the split-pane (including the left feature-list pane) only appears after Bubble Tea delivers its first `WindowSizeMsg`. The main menu avoids this with `xterm.GetSize` in `newMenuModel`, but `newStatusModel` never received the same fix.

## Related

- **Commit:** fac891b (fix(status): remove stale guard so split-pane renders on first frame)

## Steps to Reproduce

1. Run `maggus status`
2. Observe the first frame — screen is blank (no left pane, no right pane)
3. Wait for `WindowSizeMsg` (or press a key to force a redraw)
4. Split-pane now renders correctly

## Expected Behavior

The split-pane should be visible on the very first frame, exactly as the main menu is.

## Root Cause

`newStatusModel` (`src/cmd/status_model.go:97`) initializes `width` and `height` to zero. `viewStatus()` returns `""` when either is zero, so the first frame is blank.

`newMenuModel` (`src/cmd/menu_model.go:264`) already solves this by calling `xterm.GetSize` and setting `width`/`height` directly in the struct literal — with NO call to `HandleResize` or any component resize method:

```go
termW, termH, _ := xterm.GetSize(int(os.Stdout.Fd()))
return menuModel{
    ...
    width:  termW,
    height: termH,
}
```

**Important:** The fix must ONLY set `m.width` and `m.height`. Calling `HandleResize` or `resizeCurrentTaskViewport` at init time sets `taskListComponent.Width`/`Height` to the full terminal dimensions, which corrupts height calculations for the split-pane sub-components (the header disappears because the content area is sized for a full-screen box, not a split pane). Those methods should only run from the `WindowSizeMsg` handler where Bubble Tea provides the correct alt-screen dimensions.

## User Stories

### BUG-001-001: Pre-populate terminal dimensions in newStatusModel

**Description:** As a user, I want `maggus status` to render the full split-pane layout on the first frame, matching the behaviour of the main menu.

**Acceptance Criteria:**
- [x] `newStatusModel` calls `xterm.GetSize(int(os.Stdout.Fd()))` and sets `m.width` and `m.height` from the result (same pattern as `newMenuModel`)
- [x] `HandleResize` and `resizeCurrentTaskViewport` are NOT called in `newStatusModel` — only from the `WindowSizeMsg` handler (calling them early corrupts the split-pane height calculations)
- [x] The split-pane (left pane + right pane) is visible on the very first rendered frame
- [x] `WindowSizeMsg` still updates dimensions and sub-component sizes correctly (no regression)
- [x] `go vet ./...` and `go test ./...` pass
