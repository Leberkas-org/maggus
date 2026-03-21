# Bug: Stop picker not scrollable and missing plan/feature grouping

## Summary

The Alt+S stop picker renders all remaining tasks directly without viewport scrolling. With 100+ tasks across multiple plans, items overflow the terminal and only the last entries are visible. Additionally, there is no indication of which feature/plan each task belongs to.

## Steps to Reproduce

1. Create multiple feature files with many tasks (100+ total across files)
2. Run `maggus work`
3. Press Alt+S to open the stop picker
4. Observe: only the last tasks are visible; earlier tasks are pushed above the terminal viewport
5. Observe: no feature file grouping — all tasks are a flat list

## Expected Behavior

- The stop picker should be scrollable, keeping the cursor visible within the terminal height
- Tasks should be grouped by feature file with a header line per group (e.g., `── feature_002.md ──`)

## Root Cause

### No scrolling

`renderStopPicker` at `src/internal/runner/tui_render.go:103-137` builds a `strings.Builder` with every item appended sequentially. There is no viewport, scroll offset, or visible-window calculation. The overlay replaces tab content at line 469, but the rendered string can exceed the available height. The cursor (`stopPickerCursor`) moves correctly in `handleStopPicker` at `tui_keys.go:77-106`, but there is no mechanism to scroll the rendered view to keep the cursor visible.

### No plan/feature grouping

`RemainingTask` (defined at `src/internal/runner/tui_summary.go:44-48`) only stores `ID` and `Title`:

```go
type RemainingTask struct {
    ID    string
    Title string
}
```

The source feature file is not carried through. In `sendIterationStart` at `src/cmd/work_task.go:204-241`, the task list is iterated but `tasks[ti].SourceFile` is never included in the `RemainingTask` struct.

## User Stories

### TASK-001: Add viewport scrolling to the stop picker

**Description:** As a user, I want the stop picker to scroll within the available terminal height so that I can navigate through all tasks even when there are 100+.

**Acceptance Criteria:**
- [ ] The stop picker renders only as many items as fit within the available inner height of the TUI
- [ ] A scroll offset tracks which slice of items is currently visible
- [ ] Moving the cursor past the visible top/bottom edge scrolls the view to keep the cursor visible
- [ ] The first item ("After current task") and last item ("Complete the plan") are reachable and visible when selected
- [ ] Scroll position indicators are shown when there are items above or below the visible area (e.g., `▲ more` / `▼ more`)
- [ ] No regression in stop picker selection, escape/close, or the active stop point marker (●)
- [ ] `go vet ./...` and `go test ./...` pass

### TASK-002: Add feature file grouping to the stop picker

**Description:** As a user, I want to see which feature/plan each task belongs to so that I can orient myself when scrolling through many tasks.

**Acceptance Criteria:**
- [ ] `RemainingTask` includes a `SourceFile` field (or equivalent) populated from the parsed task
- [ ] `sendIterationStart` in `src/cmd/work_task.go` passes `tasks[ti].SourceFile` into each `RemainingTask`
- [ ] The stop picker renders a non-selectable header line before each group of tasks from the same feature file (e.g., `── feature_002.md ──`)
- [ ] Header lines are styled distinctly (muted/dimmed) and the cursor skips over them during up/down navigation
- [ ] The header uses only the filename (not the full path) for readability
- [ ] Scrolling from TASK-001 correctly accounts for the extra header lines in height calculations
- [ ] No regression in stop picker selection, cursor movement, or stop point logic
- [ ] `go vet ./...` and `go test ./...` pass
