<!-- maggus-id: 554526be-a3cc-4a97-9358-361b16b94c3d -->
# Bug: Update view clips top 4 content lines after changelog loads

## Summary

When navigating to the update view, the top 4 content lines (title, separator, blank line, current version) are not visible. The first visible line is the auto-update setting row. The view displays correctly for a brief moment during the checking phase, then breaks once the changelog is loaded and the view transitions to `phaseConfirm`.

## Steps to Reproduce

1. Run `maggus` and navigate to the update view (or run `maggus update` standalone)
2. Observe the view briefly during the checking phase — it renders correctly
3. Wait for the version check to complete with an available update (changelog loads)
4. Observe: the first visible line is `      Auto:  off / notify / auto` — the title, separator, blank line, and current version line are all off-screen above

## Expected Behavior

The first visible line should be `Maggus Update` (the title), with `scrollOffset = 0`.

## Root Cause

The transition from `phaseChecking` to `phaseConfirm` changes `viewportHeight()` from `innerH - 2` to `innerH - 3` (because `footerLines` goes from 1 to 2). At the same time, the rendered content grows significantly with the changelog body.

The viewport slicing in `View()` (`src/cmd/update.go:469–492`) uses `offset = m.scrollOffset` which is reset to 0 in the `updateCheckMsg` handler. Mathematically, with `offset = 0`, the title should always be `lines[0]` and visible. Static analysis cannot explain the observed clipping.

**Candidate causes, ranked by likelihood:**

1. **`Height(innerH)` is the outer box height, not inner.** In `FullScreenLeftColor` (`src/internal/tui/styles/styles.go:170–173`), `Box.Height(innerH).Render(body)` is called with `innerH = height - 2`. If lipgloss interprets `Height()` as the *total* height including borders, the inner content space is `innerH - 2 = height - 4`. The body passed in is `vp + footerLines` lines = `(innerH - footerLines - 1) + footerLines = innerH - 1 = height - 3` lines, which overflows the inner space by 1 line. Whether lipgloss clips from top or bottom determines where content is lost.

2. **Off-by-one in `viewportHeight()` gap constant.** The hardcoded `gap := 1` at `update.go:308` causes `vp = innerH - footerLines - 1`. When `footerLines = 2` (confirm phase), `vp = innerH - 3`. Meanwhile `FullScreenLeftColor` recomputes its own gap dynamically as `innerH - contentLines - footerLineCount`. This double-accounting of the gap may produce a mismatched rendering once content is long enough to trigger viewport slicing.

3. **`scrollOffset` is non-zero despite the reset.** Something between `updateCheckMsg` handler (`scrollOffset = 0`) and the first `View()` call for `phaseConfirm` may be setting `scrollOffset` to a non-zero value. Candidates: a late-firing tick that triggers an indirect `clampUpdateScroll`, or a Bubble Tea rendering order issue where the `WindowSizeMsg` seeded by `navigateTo` arrives *after* `updateCheckMsg`.

**Key code locations:**
- `viewportHeight()` — `src/cmd/update.go:302–314`
- Viewport slicing — `src/cmd/update.go:469–492`
- `FullScreenLeftColor` box height — `src/internal/tui/styles/styles.go:169–173`
- `fullScreenInner` height formula — `src/internal/tui/styles/styles.go:199–208`
- `updateCheckMsg` handler — `src/cmd/update.go:160–173`

## User Stories

### BUG-022-001: Fix update view top clipping when changelog loads

**Description:** As a user, I want to see the full update view from the title line so that I can read version information and the changelog from the top.

**Acceptance Criteria:**
- [x] When navigating to the update view (from menu or standalone), the first visible line is `Maggus Update` with `scrollOffset = 0`
- [x] Investigate `scrollOffset` value at first render of `phaseConfirm` by adding a temporary assertion or log; confirm it is 0
- [x] Verify whether `Height(innerH)` in `FullScreenLeftColor` refers to inner or outer height, and correct the formula if needed (compare with how status view uses the same function)
- [~] ⚠️ BLOCKED: If the gap double-accounting is the cause, remove the `gap := 1` from `viewportHeight()` and rely solely on `FullScreenLeftColor`'s dynamic gap — Not applicable: root cause was word-wrap overflow causing BubbleTea to drop top lines, not gap double-accounting. The gap := 1 is correct and needed.
- [x] The update view renders correctly at multiple terminal sizes (24, 40, 60 row terminals)
- [x] Footer (hints line and menu in confirm phase) remains visible and pinned to the bottom
- [x] Scrolling still works correctly in `phaseConfirm` with a long changelog
- [x] No regression in status view, config view, or repos view
- [x] `go vet ./...` and `go test ./...` pass
