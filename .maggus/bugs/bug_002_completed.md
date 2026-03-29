<!-- maggus-id: 979800a6-34d3-4116-8e2d-4e0b9c6152d7 -->
# Bug: Task row spinner/check missing space separator and truncates one char too late

## Summary

In the left pane tree, task rows render the spinner (or ✓) directly adjacent to the task ID with no separating space. The truncation width budget also does not account for this missing space, causing task titles to occupy one character more than intended.

## Steps to Reproduce

1. Run `maggus status` with expanded plans so task rows are visible
2. Observe a running task row — the spinner character is immediately followed by the task ID: `⠋TASK-001 Title`
3. Observe a completed task row — the checkmark is immediately followed by the task ID: `✓TASK-001 Title`

## Expected Behavior

A space should appear between the spinner/checkmark and the task ID:
`⠋ TASK-001 Title`
`✓ TASK-001 Title`

The title truncation should also be tightened by one character to account for the extra space in the layout.

## Root Cause

**Missing space in row construction — `status_leftpane.go` line 318:**

```go
rowContent := bgStr("   ") + spinStr + taskIDRendered + bgStr(" ") + taskTitleRendered
```

`spinStr` is rendered directly against `taskIDRendered` with no space between them.

**Width budget off by one — `status_leftpane.go` line 290:**

```go
avail := contentW - 4
```

The comment on line 288 documents the layout as `indent(3) + spinner(1) + taskID + space(1) + taskTitle`. The fixed overhead is currently counted as 4 (indent + spinner). Adding the space between spinner and taskID raises the fixed overhead to 5, so `avail` must be `contentW - 5` to keep the total row width correct.

## User Stories

### BUG-002-001: Add space between spinner/check and task ID in tree rows

**Description:** As a user, I want the spinner and checkmark in task rows to be visually separated from the task ID so the tree is easier to read.

**Acceptance Criteria:**
- [x] In `renderLeftPane` (`status_leftpane.go` line 318), change `spinStr + taskIDRendered` to `spinStr + bgStr(" ") + taskIDRendered`
- [x] In the same function at line 290, change `avail := contentW - 4` to `avail := contentW - 5` to compensate for the extra space
- [x] Running task rows display as `⠋ TASK-NNN Title` (space after spinner)
- [x] Completed task rows display as `✓ TASK-NNN Title` (space after checkmark)
- [x] Idle task rows display as `  TASK-NNN Title` (two spaces: indent placeholder + gap)
- [x] Row total width still equals `contentW` (no overflow or missing padding)
- [x] No regression in tree scroll, cursor highlight, or selected-row styling
- [x] `go vet ./...` and `go test ./...` pass
