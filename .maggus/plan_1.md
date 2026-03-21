# Plan: Rework Work Command — Remove Sub-Menu, Add Stop Picker

## Introduction

The work command has accumulated a sub-menu (task count selector + worktree toggle) that is no longer needed now that the tool has matured. The y/n stop confirmation (Alt+S) is also too simplistic — users need to choose *where* to stop, not just confirm a binary. This plan removes the work sub-menu, makes `work` always process all tasks by default, replaces the y/n stop confirmation with an interactive stop picker, and removes the "Run Again" option from the summary screen.

## Goals

- Simplify the work command entry: launching `work` immediately starts processing all workable tasks
- Replace the y/n stop confirmation with a stop picker that lets users choose where to stop
- Remove the work sub-menu from the main menu (worktree remains a CLI flag only)
- Remove "Run Again" from the summary screen — users start a new run manually
- Keep `--count` and `--task` CLI flags for power users

## User Stories

### TASK-001: Remove the Work Sub-Menu from the Main Menu
**Description:** As a user, I want `work` to launch immediately from the main menu without a sub-menu so that I can start working faster.

**Acceptance Criteria:**
- [x] Selecting "work" in the main menu launches the work command directly (no sub-menu)
- [x] The `buildSubMenus()` map no longer contains a "work" entry
- [x] The `buildArgs()` function no longer handles the "work" case
- [x] When launched from the menu without flags, work defaults to processing all workable tasks (count=999 or equivalent)
- [x] The worktree sub-menu remains unchanged (only work sub-menu is removed)
- [x] Menu description for "work" is updated to reflect the new behavior (e.g. "Work through all tasks in the plan")
- [x] Typecheck/lint passes (`go vet ./...`, `go fmt ./...`)
- [x] Existing tests pass (`go test ./...`)

### TASK-002: Change Default Task Count to All
**Description:** As a user, I want `work` to default to processing all workable tasks so that I don't have to specify a count every time.

**Acceptance Criteria:**
- [x] When `--count` is not specified, the work command processes all workable tasks (not just 1 or 3)
- [x] `--count` flag still works when explicitly provided (e.g. `--count 3` processes 3 tasks)
- [x] `--task` flag still works to target a specific task
- [x] The banner/header view reflects the total workable task count accurately
- [x] Typecheck/lint passes
- [x] Existing tests pass

### TASK-003: Replace y/n Stop Confirmation with Stop Picker
**Description:** As a user, I want Alt+S to open an interactive stop picker so that I can choose exactly when to stop working.

**Acceptance Criteria:**
- [ ] Pressing Alt+S during work opens a stop picker overlay/modal instead of the y/n prompt
- [ ] The stop picker shows these options:
  - "After current task" — stops after the currently running task completes
  - One entry per remaining upcoming task (e.g. "After TASK-005: Title") — stops after that specific task completes
  - "Complete the plan" — continues working through all tasks (cancel/dismiss the stop)
- [ ] The user can navigate the picker with arrow keys (up/down) and confirm with Enter
- [ ] Pressing Escape or Alt+S again closes the picker without changing anything
- [ ] When a stop point is selected, the `stopAfterTask` flag and/or a `stopAtTaskID` field is set accordingly
- [ ] The status bar or header shows the selected stop point (e.g. "Stopping after TASK-005")
- [ ] If already stopped, opening the picker again allows changing or cancelling the stop point
- [ ] The work loop respects the selected stop point (stops after the chosen task, not just the current one)
- [ ] Typecheck/lint passes
- [ ] Existing tests pass

### TASK-004: Pass Remaining Tasks to TUI for Stop Picker
**Description:** As a developer, I want the TUI to know about remaining upcoming tasks so that the stop picker can list them.

**Acceptance Criteria:**
- [ ] The TUI model receives a list of upcoming/remaining workable tasks (IDs + titles)
- [ ] This list is updated each time a new task starts (via `IterationStartMsg` or a new message type)
- [ ] The list excludes the currently running task and already-completed tasks
- [ ] The stop picker reads from this list to build its menu options
- [ ] Typecheck/lint passes
- [ ] Existing tests pass

### TASK-005: Remove "Run Again" from Summary Screen
**Description:** As a user, I want the summary screen to only show "Exit" so that the experience is clean — I'll start a new run when I'm ready.

**Acceptance Criteria:**
- [ ] The summary screen no longer shows the "Run again" menu option
- [ ] The summary screen shows only "Exit" (or auto-exits, press any key)
- [ ] The `RunAgainResult` type and `editingCount` logic are removed from `summaryState`
- [ ] The run-again loop in `work.go` is removed or simplified (no outer loop needed)
- [ ] Pressing Q, Esc, Enter, or Ctrl+C on the summary screen exits
- [ ] Typecheck/lint passes
- [ ] Existing tests pass

### TASK-006: Update Work Loop to Support Stop-At-Task
**Description:** As a developer, I want the work loop to stop at a specific task (not just "after current") so that the stop picker's "After TASK-X" option works correctly.

**Acceptance Criteria:**
- [ ] The work loop checks a `stopAtTaskID` field (in addition to the existing `stopFlag`)
- [ ] When `stopAtTaskID` is set, the loop continues until that task completes, then stops
- [ ] When `stopAfterTask` is set (no specific ID), the loop stops after the current task (existing behavior)
- [ ] The stop reason in the summary correctly reflects "Stopped by User" for both cases
- [ ] If the target task is skipped (blocked/already done), the loop stops at the next completed task after it or at the target point in sequence
- [ ] Typecheck/lint passes
- [ ] Existing tests pass

### TASK-007: Render Stop Picker UI
**Description:** As a user, I want the stop picker to look clean and integrated with the existing TUI so that it feels like a native part of the work view.

**Acceptance Criteria:**
- [ ] The stop picker renders as an overlay or replaces the current tab content when active
- [ ] Each option shows the task ID and title (truncated to fit the terminal width)
- [ ] The currently selected option is highlighted (matching the existing TUI style)
- [ ] "After current task" is the first option and is pre-selected
- [ ] "Complete the plan" is the last option
- [ ] The footer/status bar shows navigation hints (e.g. "↑/↓ select · enter confirm · esc cancel")
- [ ] If a stop point was previously set, the picker highlights/marks that option
- [ ] The stop indicator in the header/status bar shows the selected stop point while work continues
- [ ] Typecheck/lint passes
- [ ] Existing tests pass

### TASK-008: Clean Up Removed Code and Update Tests
**Description:** As a developer, I want dead code from the old work sub-menu and run-again flow removed so that the codebase stays clean.

**Acceptance Criteria:**
- [ ] `RunAgainResult` struct is removed
- [ ] `summaryState.editingCount`, `summaryState.countInput`, `summaryState.menuChoice`, `summaryState.runAgain` fields are removed
- [ ] `handleSummaryKeys` is simplified (no menu navigation, just exit on any key/esc/q)
- [ ] `renderSummaryMenu` is simplified or removed (no menu to render)
- [ ] Old stop confirmation code (`confirmingStop`, `handleStopConfirmation`) is removed
- [ ] Work sub-menu entries in `buildSubMenus` and `buildArgs` for "work" are removed
- [ ] All existing tests still pass after cleanup
- [ ] No compiler warnings or unused imports remain
- [ ] Typecheck/lint passes

## Functional Requirements

- FR-1: Selecting "work" from the main menu must launch the work command immediately with no sub-menu
- FR-2: The work command must default to processing all workable tasks when no `--count` flag is given
- FR-3: `--count N` and `--task TASK-NNN` flags must continue to work as before
- FR-4: Alt+S during work must open an interactive stop picker with: "After current task", one entry per remaining task, and "Complete the plan"
- FR-5: The stop picker must support arrow key navigation and Enter/Escape to confirm/cancel
- FR-6: The work loop must support stopping after a specific task (not just the current one)
- FR-7: The summary screen must only allow exiting (no "Run again")
- FR-8: A visible indicator must show the selected stop point while work continues
- FR-9: The worktree toggle remains accessible only via the `--worktree` CLI flag

## Non-Goals

- No task reordering or drag-and-drop in the stop picker
- No mid-task pause/resume functionality
- No changes to the list command or its Alt+R "run task" feature
- No changes to the sync conflict resolution flow
- No changes to worktree sub-menu behavior
- No auto-exit from the summary screen (user still needs to press a key to exit)

## Technical Considerations

- The stop picker needs access to the remaining task list, which currently lives in the work loop goroutine. A new message type or shared state will be needed to pass this to the TUI.
- The `stopFlag` is currently an `atomic.Bool` shared between the TUI and work goroutine. The stop-at-task feature needs a similar thread-safe mechanism (e.g. `atomic.Value` storing a task ID string).
- The stop picker overlay should not interfere with the running agent — it's purely a UI concern that sets a flag checked between tasks.
- Task ordering in the picker should match the order the work loop will process them.

## Success Metrics

- Work command launches in one step from the menu (no sub-menu)
- Users can precisely control where to stop using the Alt+S picker
- Summary screen is simpler with just an exit option
- All existing tests pass after changes
- No regressions in worktree mode, sync handling, or Ctrl+C behavior

## Open Questions

- Should the stop picker also show blocked tasks (grayed out) for context, or only workable tasks?
- When the stop point task gets completed, should we show a brief "Reached stop point" message before transitioning to summary?
