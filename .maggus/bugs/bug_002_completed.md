<!-- maggus-id: 2cc0a61a-b54f-4ee1-bb2e-8df85a41e505 -->
<!-- maggus-id: 20260326-135011-bug-002 -->
# Bug: Number keys don't move focus to right pane in status split view

## Summary

Pressing `1`–`4` in the status split view switches the active tab in the right pane but does not move focus there. If the left pane is focused, the user remains in the left pane navigating the plan list — making the tab switch invisible and confusing.

## Steps to Reproduce

1. Run `maggus status`
2. Observe the split-pane view with left pane focused (default)
3. Press `2`, `3`, or `4`
4. Observe: the tab bar updates visually, but `↑/↓` still moves the plan cursor in the left pane

## Expected Behavior

Pressing a number key should move focus to the right pane and activate the corresponding tab. The user should be able to use `1` to jump to the left pane and `2`–`5` to jump to right-pane tabs 1–4 respectively.

## Root Cause

In `src/cmd/status_update.go:336-344`, the `case "1", "2", "3", "4":` handler sets `m.activeTab` but never sets `m.leftFocused = false`:

```go
// Keys 1–4 switch the right-pane active tab regardless of pane focus.
switch key {
case "1", "2", "3", "4":
    m.activeTab = int(key[0] - '1')   // tab switches...
    // ...but m.leftFocused is never changed → focus stays on left pane
    return m, nil
}
```

## User Stories

### BUG-002-001: Remap number keys so 1 focuses left pane and 2–5 focus right pane tabs

**Description:** As a user, I want to press a single number key to jump to any pane/tab so that I can navigate the status view without needing the Tab key.

**New key mapping:**
- `1` → focus left pane (set `m.leftFocused = true`)
- `2` → focus right pane + switch to tab 0 (Output)
- `3` → focus right pane + switch to tab 1 (Feature Details)
- `4` → focus right pane + switch to tab 2 (Current Task)
- `5` → focus right pane + switch to tab 3 (Metrics)

**Acceptance Criteria:**
- [x] Pressing `1` sets `m.leftFocused = true` and returns focus to the left pane
- [x] Pressing `2`–`5` sets `m.leftFocused = false` and sets `m.activeTab` to `key - '2'`
- [x] Pressing `2` (Output tab) still auto-scrolls the log to the bottom (`m.logAutoScroll = true`)
- [x] Tab bar in `status_rightpane.go` renders numbers `2`–`5` instead of `1`–`4` next to tab names
- [x] Footer hints in `status_view.go` are updated to reflect `1: left  2-5: tabs`
- [x] `go vet ./...` and `go test ./...` pass
- [x] No regression: Tab key still toggles focus between panes
