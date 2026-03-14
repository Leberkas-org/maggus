# Plan: Redesign status view with plan tabs and scoped task list

## Introduction

The current status view shows all plans and all tasks in a single long scrollable list. This makes it hard to focus on a specific plan when there are many. The redesign puts plan selection at the top as a horizontal tab bar, with the task list below scoped to the selected plan. Plans are navigated via `ctrl+tab/ctrl+shift+tab`, tasks via arrow keys.

## Goals

- Replace the flat status layout with a tabbed plan selector at the top and a scoped task list below
- Each plan tab shows the plan name and `done/total` progress count
- The selected plan shows a progress bar below the tab row
- Task list only shows tasks from the selected plan, navigable with arrow keys
- Maintain all existing functionality: detail view, `alt+a` toggle, `alt+r` run, `alt+bksp` delete

## User Stories

### TASK-001: Plan tab bar at the top of the status view
**Description:** As a user, I want to see all plans as horizontal tabs at the top of the status view, so I can quickly see which plans exist and switch between them.

**Acceptance Criteria:**
- [x] The top of the status view shows a horizontal row of plan tabs
- [x] Each tab displays the plan filename (without `.md` extension) and progress as `done/total` (e.g. `plan_1 3/5`)
- [x] The currently selected plan tab is visually highlighted (bold, primary color)
- [x] Unselected tabs are rendered in muted style
- [x] Completed plan tabs (from `_completed.md` files) are only shown when `showAll` is true
- [x] If tabs overflow the terminal width, they wrap to the next line or truncate gracefully
- [x] Pressing `ctrl+shift+tab` selects the previous plan tab (wraps around)
- [x] Pressing `ctrl+tab` selects the next plan tab (wraps around)
- [x] `go build ./...` and `go test ./...` pass

### TASK-002: Progress bar for the selected plan
**Description:** As a user, I want to see a progress bar for the currently selected plan, so I get a quick visual sense of how far along it is.

**Acceptance Criteria:**
- [x] Below the tab row, a progress bar is rendered for the selected plan
- [x] The progress bar uses the existing `styles.ProgressBar` helper
- [x] Summary stats are scoped to the selected plan: `done/total tasks · N pending · N blocked`
- [x] The progress bar and stats update when switching plans via `ctrl+tab/ctrl+shift+tab`
- [x] `go build ./...` and `go test ./...` pass

### TASK-003: Scoped task list for the selected plan
**Description:** As a user, I want the task list to only show tasks from the selected plan, so I can focus on one plan at a time without visual noise from other plans.

**Acceptance Criteria:**
- [x] The task list below the progress bar only shows tasks from the currently selected plan
- [x] By default (showAll=false), completed tasks within the plan are hidden
- [x] Tasks are navigable with arrow keys (up/down/j/k), with cursor wrapping
- [x] Switching plans via `ctrl+tab/ctrl+shift+tab` resets the task cursor to 0 and rebuilds the task list
- [x] Blocked tasks show the `⚠` icon and are colored red; the next workable task shows `→` in cyan; pending tasks show `○` in muted
- [x] When `showAll` is toggled via `alt+a`, completed tasks (green `✓`) appear/disappear and plans reload from disk
- [x] The task list is scrollable via viewport when it exceeds the available height
- [x] `go build ./...` and `go test ./...` pass

### TASK-004: Preserve existing interactions in new layout
**Description:** As a user, I want all existing status interactions (detail view, run, delete, toggle) to work in the new layout.

**Acceptance Criteria:**
- [x] Pressing `enter` on a task opens the detail view (same as before, with criteria mode)
- [x] Pressing `alt+r` dispatches work on the selected task
- [x] Pressing `alt+bksp` shows the delete confirmation for the selected task
- [x] Pressing `alt+a` toggles `showAll`, reloads from disk, and rebuilds both tabs and task list
- [x] The footer shows all available keybindings including `ctrl+tab/ctrl+shift+tab: switch plan`
- [x] `esc`/`q` exits the status view
- [x] `go build ./...` and `go test ./...` pass

### TASK-005: Remove old status layout
**Description:** As a developer, I want to remove the old status rendering code that is no longer needed after the redesign.

**Acceptance Criteria:**
- [ ] The old `viewStatus()` method rendering flat plan sections and the plans table at the bottom is removed
- [ ] The old plans table rendering code (progress bars per plan at the bottom) is removed
- [ ] No dead code remains from the old layout
- [ ] The `--plain` output (`renderStatusPlain`) is unchanged — it keeps the old flat format for scripting
- [ ] `go build ./...` and `go test ./...` pass

## Functional Requirements

- FR-1: Plan tabs are rendered horizontally at the top, each showing `filename done/total`
- FR-2: The selected tab is highlighted; unselected tabs are muted
- FR-3: `ctrl+tab` and `ctrl+shift+tab` cycle through plan tabs (wrapping at boundaries)
- FR-4: A progress bar and scoped summary stats are shown below the tabs for the selected plan
- FR-5: The task list only shows tasks from the selected plan
- FR-6: Arrow keys navigate the task list; `ctrl+tab/ctrl+shift+tab` navigate the plan tabs — these are independent
- FR-7: Switching plans resets the task cursor to 0
- FR-8: `alt+a` toggles completed tasks and completed plans, reloading from disk
- FR-9: All existing interactions (detail, run, delete) work on the task under the cursor
- FR-10: `--plain` output format is not changed

## Non-Goals

- No drag-and-drop or reordering of plan tabs
- No renaming plans from the status view
- No splitting the view into multiple panes (just tabs + list)
- No changes to the detail view internals (criteria mode, action picker, etc.)

## Technical Considerations

- The `statusModel` needs a new `selectedPlan` index field that tracks which plan tab is active
- The `selectableTasks` list should be rebuilt from `plans[selectedPlan].tasks` whenever the plan changes
- The tab bar rendering should handle long filenames gracefully — consider truncating to a max width per tab
- The existing `viewStatus()` method will be replaced entirely — the new layout is fundamentally different
- Tasks 001-003 can be developed incrementally on top of the existing model; TASK-005 cleans up afterward

## Success Metrics

- The status view feels focused — you see one plan's tasks at a time instead of everything
- Switching between plans is instant and intuitive via `ctrl+tab/ctrl+shift+tab`
- All existing keyboard shortcuts still work without relearning
