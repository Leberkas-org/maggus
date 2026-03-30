<!-- maggus-id: 72bbd800-a661-40cd-b58d-226148185e85 -->
# Bug: Status view upper border scrolls off-screen due to height over-count by 1

## Summary

The status split-pane view renders 1 line taller than the terminal, causing the alt-screen to scroll and the top border of the outer box to be pushed off-screen. The footer is simultaneously clipped.

## Steps to Reproduce

1. Run `maggus status` (or navigate to the status screen from the menu)
2. Observe: the top rounded border (`╭───╮`) of the full-screen box is not visible — it has been scrolled off the top of the terminal

## Expected Behavior

The full-screen box should fit exactly within the terminal with both the top and bottom borders visible.

## Root Cause

The height mismatch is a chain of off-by-one errors across three files.

**Step 1 — `status_view.go:156–157`** passes `innerH - 1` to each pane:
```go
leftPane  := m.renderLeftPane(leftW, innerH-1)
rightPane := m.renderRightPane(rightW, innerH-1)
```
Intent: leave 1 slot for the footer that `FullScreenLeftColor` will append.

**Step 2 — Each pane appends an extra bottom border line on top of its allocated height.**

`status_rightpane.go:71–72`:
```go
rendered := lipgloss.NewStyle().Width(width).Height(height).Render(full)
borderLine := strings.Repeat(borderStyle.Render("─"), width)
return rendered + "\n" + borderLine   // +1 line beyond `height`
```

`status_leftpane.go:345–346`:
```go
lastLine := strings.Repeat(bChar, contentW) + borderStyle.Render("┴")
result = append(result, lastLine)      // +1 line beyond `height`
```

Each pane returns `(innerH - 1) + 1 = innerH` lines instead of the intended `innerH - 1`.

**Step 3 — `styles/styles.go:159` gap calculation in `FullScreenLeftColor`:**
```go
gap := innerH - contentLines - footerLineCount
// = innerH - innerH - 1 = -1  → clamped to 0
```
Because `contentLines == innerH` the gap is negative, so the footer is appended with zero spacing: `body` = `innerH + 1` lines.

**Step 4 — The box overflows the terminal.**

`Box.Height(innerH)` is a *minimum* in lipgloss; with `innerH + 1` lines of body content the box expands to `innerH + 1` interior + 2 border rows = `innerH + 3 = H + 1` lines.

`lipgloss.Place(W, H, Left, Top, box)` does not crop oversized content — it returns the full `H + 1` line string. The alt-screen terminal receives one extra line, scrolls, and the top border (line 0) disappears from view.

With `FullScreenMargin = 0`:
```
innerH       = H - 2
pane content = innerH - 1 + 1 = H - 2   (no net reduction)
body         = (H-2) + 1 footer = H - 1 lines
box total    = (H-1) + 2 borders = H + 1 lines   ← 1 too tall
```

## User Stories

### BUG-020-001: Fix height passed to status split panes to account for their bottom border and the footer

**Description:** As a user, I want the status view to render with both borders visible so the layout is correct on all terminal sizes.

**Acceptance Criteria:**
- [x] `viewStatusSplit()` passes `innerH - 2` to `renderLeftPane` and `renderRightPane` (accounting for the 1-line bottom border each pane appends **and** the 1-line footer in `FullScreenLeftColor`)
- [x] The top border of the full-screen box is visible when running `maggus status`
- [x] The footer key hints are visible at the bottom of the box
- [x] No content is clipped when the terminal is resized
- [x] No regression in left-pane plan list rendering or right-pane tab content
- [x] `go vet ./...` and `go test ./...` pass
