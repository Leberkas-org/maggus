<!-- maggus-id: a47276b7-e773-4724-a1ee-7f81e69a6032 -->
# Feature 024: Truncate Task Title in Output Tab

## Introduction

In the Output tab of the status view, the "Task:" line in the rich snapshot view (`renderSnapshotInPane`) renders `snap.TaskTitle` without truncation. When the task title is long, it overflows the right pane width, breaking the layout.

### Architecture Context

- **Component touched:** `status_rightpane.go` — `renderSnapshotInPane()` line 142
- **Existing utility:** `styles.Truncate(text, maxW)` is already used throughout the file

## Goals

- Task title in the Output tab never overflows the pane width
- Fixed prefix (`"Task:"` label + task ID) is always fully visible; title fills the remaining space

## Tasks

### TASK-024-001: Truncate task title in snapshot view

**Description:** As a user, I want the task title in the Output tab to be truncated when it's too long so that the layout never breaks.

**Token Estimate:** ~15k tokens
**Predecessors:** none
**Successors:** none
**Parallel:** no
**Model:** haiku

**Acceptance Criteria:**
- [ ] In `renderSnapshotInPane()` (`status_rightpane.go` line ~142), compute the max available width for the title: `maxTitleW = width - 1 - lipgloss.Width(statusBoldStyle.Render("Task:")) - 4 - lipgloss.Width(statusCyanStyle.Render(snap.TaskID)) - 3` (the constants account for the leading space, 4-space gap, and `" - "` separator)
- [ ] `snap.TaskTitle` is wrapped with `styles.Truncate(snap.TaskTitle, maxTitleW)` before being written
- [ ] When `maxTitleW <= 0`, the title is omitted entirely (only `TaskID` is shown) rather than panicking
- [ ] `go build ./...` passes

## Task Dependency Graph

```
TASK-024-001
```

| Task | Estimate | Predecessors | Parallel | Model |
|------|----------|--------------|----------|-------|
| TASK-024-001 | ~15k | none | no | haiku |

**Total estimated tokens:** ~15k

## Functional Requirements

- FR-1: The Task title line in the Output tab must never exceed the pane width
- FR-2: The `"Task:"` label and task ID must always be fully visible; truncation applies only to the title
- FR-3: Truncation uses `…` suffix (handled by the existing `styles.Truncate` utility)

## Non-Goals

- No changes to the Status line or any other line in the snapshot view
- No wrapping or multi-line task title display

## Open Questions

None.
