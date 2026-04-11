<!-- maggus-id: 44fb9f9a-cb67-4ed6-8e37-52c5483bfc74 -->
# Bug: Task Details tab always shows the first workable task, ignoring tree selection

## Summary

The Task Details tab in the status view always displays the globally next workable task (the one the daemon would pick up next), not the task currently selected in the left pane tree. Navigating between tasks in the tree does not update the Task Details content.

## Steps to Reproduce

1. Open `maggus` → status view
2. Expand a feature with multiple tasks
3. Navigate to any task other than the first workable one
4. Switch to the Task Details tab (key `4` or the relevant tab number)
5. Observe: the detail shows the first workable task, not the selected one
6. Navigate up/down between tasks
7. Observe: the detail content does not change

## Expected Behavior

The Task Details tab should show the description and acceptance criteria of whichever task is selected in the left pane tree. When no task is selected (a feature row is highlighted), it should fall back to the next workable task.

## Root Cause

Three issues compose to produce this bug:

### 1. `renderCurrentTaskTab` uses `m.nextTaskID` instead of the selected task

`status_rightpane.go:265-273`: `renderCurrentTaskTab` checks `m.nextTaskID` (the global next workable task set by `findNextTask()`) and renders `m.currentTaskViewport` which was loaded from the same global ID. It never reads `m.selectedTask()`.

### 2. `loadCurrentTaskDetail` loads from `m.nextTaskID`

`status_rightpane.go:258-261`: `loadCurrentTaskDetail` passes `m.nextTaskID` and `m.nextTaskFile` to `renderCurrentTaskContent`. It should use `m.selectedTask()` when a task is selected in the tree.

### 3. `loadCurrentTaskDetail` is not called on cursor movement within a plan

`status_update.go:601-628`: The up/down key handlers only call `rebuildRightPane()` when `m.selectedPlan().ID` changes (moving between features). When moving between tasks within the same feature, neither `rebuildRightPane` nor `loadCurrentTaskDetail` is called — the viewport content stays stale.

## User Stories

### BUG-030-001: Make Task Details tab follow tree selection

**Description:** As a user browsing tasks in the status view, I want the Task Details tab to show the selected task's details so I can review any task, not just the next workable one.

**Acceptance Criteria:**
- [ ] `loadCurrentTaskDetail()` uses `m.selectedTask()` when a task row is selected in the tree, falling back to `m.nextTaskID` when a feature row is selected or no task exists
- [ ] `renderCurrentTaskTab` shows "No task selected" (not "No pending tasks") when nothing applies
- [ ] `rebuildRightPane()` calls `loadCurrentTaskDetail()` so plan changes refresh the viewport
- [ ] Up/down cursor movement within the same plan calls `loadCurrentTaskDetail()` so task-to-task navigation updates the viewport
- [ ] PgUp/PgDn, Home/End navigation also triggers `loadCurrentTaskDetail()` when the selected item changes
- [ ] No regression: the next-task arrow marker (`→`) in the left pane still points to the globally next workable task
- [ ] `go vet ./...` and `go test ./...` pass
