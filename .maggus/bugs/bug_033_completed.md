<!-- maggus-id: 3ee03a9b-47c5-4eb9-9583-ca1fe806394d -->
# Bug: Top outer border disappears when Task Details tab is selected

## Summary

Selecting the Task Details tab in the status view causes the top outer border of the TUI frame to disappear. The rest of the layout shifts, breaking the visual structure.

## Steps to Reproduce

1. Open `maggus` → status view
2. Select a task in the left pane tree
3. Switch to the Task Details tab
4. Observe: the top border of the outer frame disappears

## Expected Behavior

The outer border frame should remain intact on all tabs. The Task Details content should render within the right pane without affecting the outer frame.

## Root Cause

The right pane content overflows its allocated height, pushing the outer `Box` border off-screen. This is caused by multiple layers of `lipgloss.Height()` wrapping that don't constrain correctly when content is too tall.

The chain:
1. `renderCurrentTaskTab()` (`status_rightpane.go:272-282`) renders the viewport with `.Width(width).Height(height)` — lipgloss applies `Width` which can **word-wrap** long lines (task descriptions, acceptance criteria), adding extra newlines
2. `renderRightPane()` (`status_rightpane.go:82-83`) constructs `tabBar + "\n" + sep + "\n" + content` and wraps with `.Width(width).Height(height)` — another Height constraint on already-wrapped content
3. `viewStatusSplit()` (`status_view.go:159`) joins left and right panes with `lipgloss.JoinHorizontal`
4. `FullScreenLeftColor()` (`styles.go:169-175`) wraps in a `Box.Height(innerH)` — if the joined content exceeds `innerH` lines, the box renders taller than expected and the top border is pushed above the visible terminal area

The specific trigger for Task Details: the `currentTaskViewport.View()` returns content where long acceptance criteria lines wrap when lipgloss applies `.Width()`. The viewport itself constrains to `contentH` lines, but after lipgloss width-wrapping, the actual line count exceeds `contentH`. The extra lines cascade through each wrapping layer.

This is the same class of bug as bug_031 (completed task Output tab breaking layout) — both caused by lipgloss `.Width()` word-wrapping adding unexpected lines inside a `.Height()` constraint.

## Related

- **Bug:** bug_031 (completed task Output tab breaks layout — same root cause)

## User Stories

### BUG-033-001: Prevent content overflow in right pane tab rendering

**Description:** As a user, I want all right pane tabs to render within their allocated dimensions so the outer border frame is never broken.

**Acceptance Criteria:**
- [x] `renderCurrentTaskTab` does NOT apply `.Width().Height()` wrapping — it returns raw content and lets `renderRightPane` handle final sizing
- [x] `renderRightPane` applies `.MaxHeight(height)` (not `.Height()`) to the combined `tabBar + sep + content` so overflow is clipped rather than pushing the border
- [x] Alternatively: each tab renderer pre-truncates lines to `width` before returning, preventing lipgloss word-wrap from adding extra lines
- [x] The outer border frame remains visible on all tabs: Output, Summary, Details, Task Details, Metrics
- [x] Long acceptance criteria text in Task Details does not break the layout
- [x] Task descriptions with very long lines do not break the layout
- [x] No regression in other tab renderers (they all follow the same pattern)
- [x] `go vet ./...` and `go test ./...` pass
