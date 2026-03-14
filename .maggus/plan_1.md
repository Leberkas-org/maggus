# Plan: Streamline list and status TUI commands

## Introduction

The `list` and `status` commands currently require a sub-menu dialog in the TUI main menu before launching. This adds unnecessary friction. The goal is to make both commands launch directly — `list` always shows all incomplete tasks (including blocked) in a scrollable viewport, and `status` defaults to unfinished plans with an in-view `alt+a` toggle to show/hide completed plans. Additionally, the standalone `blocked` command is removed and its functionality is integrated into the task detail view, allowing users to manage blocked criteria in-place.

## Goals

- Remove sub-menu dialogs for `list` and `status` from the main menu — they launch directly
- `list` shows all incomplete tasks (workable + blocked) with a scrollable viewport
- `status` defaults to showing only unfinished plans, with `alt+a` to toggle completed plans in-place (reloading from disk)
- `--plain` and `--all` CLI flags remain unchanged for scripting use
- Remove the standalone `blocked` command; integrate blocked-task management into the task detail view

## User Stories

### TASK-001: Remove sub-menu dialogs for list and status
**Description:** As a user, I want `list` and `status` to launch directly from the main menu without a settings dialog, so I get to the information faster.

**Acceptance Criteria:**
- [ ] Selecting "list" in the main menu launches the list TUI directly (no sub-menu)
- [ ] Selecting "status" in the main menu launches the status TUI directly (no sub-menu)
- [ ] The `buildSubMenus()` map no longer contains entries for "list" or "status"
- [ ] The `buildArgs()` switch no longer contains cases for "list" or "status"
- [ ] CLI flags (`--plain`, `--all`, `--count`) still work when running `maggus list` or `maggus status` directly from the command line
- [ ] `go build ./...` and `go test ./...` pass

### TASK-002: List command shows all incomplete tasks with scrollable viewport
**Description:** As a user, I want the `list` command to show all incomplete tasks (including blocked ones) in a scrollable list, so I can browse the full backlog without a count cap.

**Acceptance Criteria:**
- [ ] In TUI mode (no `--plain`), `list` shows all incomplete tasks (both workable and blocked) — no count cap
- [ ] Blocked tasks are visually distinguishable from workable tasks (e.g. different icon or color)
- [ ] The task list uses a viewport-based scrollable list: the cursor moves freely, and the visible window scrolls when the cursor reaches the edge
- [ ] The header shows the total count of displayed tasks (e.g. "All incomplete tasks (12)")
- [ ] Existing detail view (enter), run (alt+r), and delete (alt+backspace) still work
- [ ] `--plain` mode still works and respects `--count` and `--all` flags as before
- [ ] `go build ./...` and `go test ./...` pass

### TASK-003: Status command defaults to unfinished plans with alt+a toggle
**Description:** As a user, I want the `status` command to show only unfinished plans by default, with `alt+a` to toggle showing completed plans, so I can focus on active work but still check history when needed.

**Acceptance Criteria:**
- [ ] In TUI mode, `status` defaults to `showAll: false` (only unfinished plans visible)
- [ ] Pressing `alt+a` toggles `showAll` and reloads plan data from disk (picks up external changes)
- [ ] After toggling, the selectable task list is rebuilt and cursor is reset to 0
- [ ] The footer shows the `alt+a` hint (e.g. "alt+a: show all" or "alt+a: hide completed")
- [ ] `--plain` mode still works and respects `--all` flag as before
- [ ] `--all` CLI flag still works for TUI mode (starts with showAll: true)
- [ ] `go build ./...` and `go test ./...` pass

### TASK-004: Integrate blocked-task management into the detail view
**Description:** As a user, I want to manage blocked criteria directly from the task detail view (in both `list` and `status`), so I don't need a separate `blocked` command.

**Acceptance Criteria:**
- [ ] The task detail view (opened via `enter` in both `list` and `status`) supports two modes: **scroll mode** (default) and **criteria mode**
- [ ] Pressing `tab` or `b` in the detail view toggles into criteria mode, where the cursor moves between blocked criteria
- [ ] In criteria mode, pressing `enter` on a blocked criterion shows an inline action picker with three options: Unblock (remove BLOCKED: prefix), Resolve (delete the criterion line), Skip
- [ ] After an action is performed, the plan file is updated on disk and the detail view refreshes to reflect the change
- [ ] The footer updates to show mode-specific hints: scroll mode shows "tab: manage blocked", criteria mode shows "enter: action | tab: scroll mode | esc: back"
- [ ] If a task has no blocked criteria, pressing `tab`/`b` does nothing (or shows a brief message)
- [ ] The standalone `blocked` command is removed from `cmd/blocked.go` and from the main menu
- [ ] The `unblockCriterion` and `resolveCriterion` helper functions are moved to a shared location (e.g. `internal/parser`) or kept in a shared file accessible by both `list` and `status`
- [ ] `go build ./...` and `go test ./...` pass

## Functional Requirements

- FR-1: The main menu must not show a sub-menu for `list` or `status` — both launch directly
- FR-2: `list` TUI mode must display all incomplete tasks (IsComplete() == false), regardless of blocked status
- FR-3: The `list` task list must be scrollable via a bubbles/viewport when the list exceeds the visible area
- FR-4: `status` TUI must start with `showAll: false` unless `--all` is passed
- FR-5: `status` must support `alt+a` to toggle between showing and hiding completed plans, reloading from disk on each toggle
- FR-6: All existing CLI flags (`--plain`, `--all`, `--count`) must continue to work for non-TUI usage
- FR-7: The task detail view must support an inline criteria mode for managing blocked criteria (unblock, resolve, skip)
- FR-8: The standalone `blocked` command must be removed; its functionality is absorbed into the detail view

## Non-Goals

- No changes to the `--plain` output format for either command
- No changes to the delete confirmation behavior
- No pagination with page numbers — use viewport scrolling instead
- No `alt+a` toggle for the `list` command
- No inline editing of non-blocked criteria (only blocked criteria are actionable)

## Technical Considerations

- The `list` command currently filters with `t.IsWorkable()` which excludes blocked tasks — change to `!t.IsComplete()` for TUI mode
- For the scrollable list in `list`, use `bubbles/viewport` to wrap the rendered task lines, similar to how the detail view already works
- The `status` toggle needs to call `parsePlans()` and `findNextTask()` again on each toggle to pick up disk changes
- When removing sub-menus from `buildSubMenus`, also clean up `buildArgs` cases to avoid dead code
- The `unblockCriterion` and `resolveCriterion` functions from `blocked.go` need to be reusable — move to `internal/parser` or a shared file in `cmd/`
- The detail view in both `list.go` and `status.go` share the same `renderDetailContent` pattern — consider extracting the criteria-mode logic into a shared component to avoid duplication
- TASK-004 depends on TASK-002 and TASK-003 being done first (the detail view needs to be stable before adding criteria mode)

## Success Metrics

- Both commands launch from the main menu with zero intermediate dialogs
- Users can scroll through large task lists without truncation
- The `alt+a` toggle in status is discoverable via the footer hint and responsive
- Blocked criteria can be managed directly from the detail view without leaving the current command
- The standalone `blocked` command is no longer needed — one fewer top-level command to learn
