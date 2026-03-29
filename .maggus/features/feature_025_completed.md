<!-- maggus-id: 5077d4a2-f0b6-4660-acee-0c92b90acc8d -->
# Feature 025: Fix Tree Scroll Overhead Miscalculation

## Introduction

`treeAvailableHeight()` uses `const treeOverhead = 4` to estimate how many item rows fit in the left pane. The actual overhead is 6 (5 fixed header lines + the `−1` offset from `innerH−1` passed to `renderLeftPane`). This makes the scroll window 2 rows larger than reality, so the cursor drifts off-screen before scrolling triggers.

### Architecture Context

- **Component touched:** `status_update.go` — `treeAvailableHeight()`, `const treeOverhead`
- **Root cause:** `treeAvailableHeight` uses `innerH` directly, but `renderLeftPane` receives `innerH−1`; and header line count was underestimated at 4 instead of 5

## Goals

- Cursor is always visible when navigating the tree
- The scroll window exactly matches the rendered item area

## Tasks

### TASK-025-001: Fix treeOverhead constant

**Description:** As a user, I want the tree to scroll correctly so the selected item is never hidden below the visible area.

**Token Estimate:** ~15k tokens
**Predecessors:** none
**Successors:** none
**Parallel:** no
**Model:** haiku

**Acceptance Criteria:**
- [x] In `treeAvailableHeight()` (`status_update.go`), `const treeOverhead` is changed from `4` to `6`
- [x] A comment explains the breakdown: `// innerH-1 (renderLeftPane receives innerH-1) + 5 header lines (label + sep + empty + daemon + sep)`
- [x] Navigating down through a list longer than the pane height keeps the cursor visible at all times with ~2 rows of context below
- [x] `go build ./...` passes

## Task Dependency Graph

```
TASK-025-001
```

| Task | Estimate | Predecessors | Parallel | Model |
|------|----------|--------------|----------|-------|
| TASK-025-001 | ~15k | none | no | haiku |

**Total estimated tokens:** ~15k

## Functional Requirements

- FR-1: `treeAvailableHeight()` must return `innerH − 6` (accounting for `−1` passed to `renderLeftPane` and 5 header lines)
- FR-2: The cursor must never be rendered outside the visible item area during normal up/down navigation

## Non-Goals

- No changes to the scroll padding (2-line context) or any other scroll behavior
- No changes to overlay mode rendering

## Open Questions

None.
