<!-- maggus-id: 5dae4ef4-a7f0-4e2b-ab03-1a7af12eb48b -->
# Feature 026: Completed Task Indicator in Left Pane Tree

## Introduction

In the left pane tree, task rows have no visual distinction between complete and incomplete tasks. A task is complete when all its acceptance criteria are checked (`t.IsComplete() == true`). This feature adds a `✓` indicator in the spinner slot (green) for completed tasks, making progress immediately visible without changing the overall row styling.

### Architecture Context

- **Component touched:** `status_leftpane.go` — task row rendering (the `spinStr` block, ~line 282)
- **Existing method:** `task.IsComplete()` already available from the parser package
- **Pattern:** The right pane tab 2 task list already uses `✓` + `statusGreenStyle` for complete tasks — this mirrors that convention in the left pane

## Goals

- Completed tasks show a green `✓` in the spinner column
- Incomplete tasks remain unchanged (space in the spinner column)

## Tasks

### TASK-026-001: Add ✓ indicator for completed tasks in tree

**Description:** As a user, I want completed tasks to show a `✓` so I can see at a glance which tasks are done.

**Token Estimate:** ~15k tokens
**Predecessors:** none
**Successors:** none
**Parallel:** no
**Model:** haiku

**Acceptance Criteria:**
- [x] In `renderLeftPane()` (`status_leftpane.go`), within the task row block, the `spinStr` logic is extended: if `task.IsComplete()` is true AND the task is not the currently active daemon task, render `spinStr` as `✓` in `greenStyle`
- [x] Active daemon task (spinner) takes priority: if `m.daemon.Running && m.daemon.CurrentTask == task.ID`, show the spinner as before regardless of completion state
- [x] The `✓` respects the selection background: use `addBg(greenStyle).Render("✓")` when the row is selected, `greenStyle.Render("✓")` otherwise
- [x] Task ID and title rendering is unchanged (still `mutedStyle`)
- [x] `go build ./...` passes

## Task Dependency Graph

```
TASK-026-001
```

| Task | Estimate | Predecessors | Parallel | Model |
|------|----------|--------------|----------|-------|
| TASK-026-001 | ~15k | none | no | haiku |

**Total estimated tokens:** ~15k

## Functional Requirements

- FR-1: A task row where `task.IsComplete() == true` must display `✓` in green in the spinner column
- FR-2: A task row where `task.IsComplete() == false` must display a space in the spinner column (unchanged)
- FR-3: If the task is the active daemon task, the spinner character takes priority over `✓`
- FR-4: The `✓` background must match the row selection background when the row is selected

## Non-Goals

- No color change to task ID or title text
- No changes to plan rows
- No strikethrough or dimming of completed task text

## Open Questions

None.
