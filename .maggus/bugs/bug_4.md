# Bug: Summary screen progress bar updates when new files are created

## Summary

When new feature or bug files are created while the summary screen is visible, the progress bar in the header re-renders with a higher total (e.g. "3/3 Tasks" → "3/5 Tasks"). The summary screen should display a frozen snapshot of the completed run.

## Steps to Reproduce

1. Run `maggus work` and let all tasks complete so the summary screen appears
2. While the summary screen is visible, create a new feature or bug file in `.maggus/`
3. Observe: the progress bar in the summary header updates from e.g. `[████████████████████] 3/3 Tasks` to `[████████████░░░░░░░░] 3/5 Tasks`

## Expected Behavior

The progress bar and task counts on the summary screen should not change. The summary is a post-run snapshot. The user must start a new run from the main menu to work on new tasks.

## Root Cause

`tui.go:353-355` handles `FileChangeMsg` unconditionally:

```go
case FileChangeMsg:
    cmd := m.handleFileChange()
    return m, tea.Batch(cmd, listenForWatcherUpdate(m.watcherCh))
```

`handleFileChange()` (`tui_messages.go:132`) always updates `m.totalIters`:

```go
m.totalIters = m.currentIter + workableBugs + workableFeatures
```

The summary view's header is rendered by `renderSummaryView` → `m.renderHeaderInner(innerW)` (`tui_render.go:109-115`), which reads `m.totalIters` to draw the progress bar. Because `FileChangeMsg` updates `totalIters` while the summary is showing, the frozen summary becomes live again.

The fix is to short-circuit `handleFileChange()` (or the entire `FileChangeMsg` branch) when `m.summary.show` is true.

## User Stories

### BUG-004-001: Skip file-change processing while summary screen is active

**Description:** As a user, I want the summary screen to display a frozen snapshot of the completed run so that creating new files does not alter the displayed progress bar or task counts.

**Acceptance Criteria:**
- [ ] `FileChangeMsg` is a no-op (returns early with only `listenForWatcherUpdate`) when `m.summary.show` is true
- [ ] Progress bar in summary header stays at the completed count (e.g. 3/3) after new files are added
- [ ] Notification banner for new tasks is not shown while summary is visible
- [ ] After starting a new run, the file watcher resumes normal behaviour (progress bar updates as tasks complete)
- [ ] No regression in file-change updates during an active run
- [ ] `go vet ./...` and `go test ./...` pass
