<!-- maggus-id: 780a75b4-4b22-4fd6-8e86-2de0780c44e7 -->
# Feature 020: Improve Tree Selection Visibility in Left Pane

## Introduction

The current tree selection indicator in the left pane uses a `▸` cursor character as the primary visual cue for the focused row. This is subtle and hard to notice. This feature replaces the cursor character with a proper highlight: the cursor column is removed entirely (layout shifts left), and selected rows render their text in boosted foreground colors while keeping the existing dark background (`#1f2937`). Both plan rows and task rows receive the same treatment.

## Goals

- Remove the `▸` cursor character and its reserved column from the layout
- Make the selected row visually obvious using foreground color changes
- Preserve per-item color semantics: completed items stay muted, bug items stay red, normal items go white/bright
- Apply consistent selection style to both plan rows and task rows

## Tasks

### TASK-020-001: Remove cursor column and apply selection foreground colors

**Description:** As a user navigating the left pane tree, I want the selected row to be clearly highlighted with foreground color changes instead of a hard-to-see cursor character, so that I always know which item is focused.

**Token Estimate:** ~25k tokens
**Predecessors:** none
**Successors:** none
**Parallel:** no

**Acceptance Criteria:**
- [x] The `cursorChar` variable and its rendering logic are removed from plan rows
- [x] The 1-char cursor column is gone — `expandIcon` is now the leftmost character in a plan row
- [x] Layout widths and `titleMaxW` calculation are updated to account for the removed column (subtract 3 fixed instead of 4)
- [x] When a plan row is selected and the left pane is focused, the title foreground is boosted: white (`#ffffff`) for normal plans, muted (unchanged) for completed plans, red/error (unchanged) for bug plans
- [x] When a plan row is selected and the left pane is focused, the expand/collapse icon foreground is boosted to white
- [x] Task rows: when selected and focused, the task ID and task title text are rendered in white instead of muted
- [x] When the left pane is NOT focused, selection highlight (background) still shows but foreground colors are not boosted
- [x] The existing `selectedBg` background (`#1f2937`) is unchanged
- [x] No layout jitter or misalignment in plan or task rows at any pane width
- [x] `go build ./...` passes
- [x] `go vet ./...` passes

## Task Dependency Graph

```
TASK-020-001
```

| Task | Estimate | Predecessors | Parallel | Model |
|------|----------|--------------|----------|-------|
| TASK-020-001 | ~25k | none | no | — |

**Total estimated tokens:** ~25k

## Functional Requirements

- FR-1: The cursor character `▸` must no longer appear in any tree row
- FR-2: The reserved 1-char cursor column is eliminated; plan row layout starts with the expand/collapse icon
- FR-3: A selected, focused plan row with a normal (non-completed, non-bug) title must render that title in bright white
- FR-4: A selected, focused plan row with a completed plan must keep the title in muted color (no boost)
- FR-5: A selected, focused plan row for a bug plan must keep the title in error/red color (no boost)
- FR-6: A selected, focused task row must render both task ID and task title in bright white instead of muted
- FR-7: When the left pane loses focus, selected rows show only the background highlight — no foreground boost
- FR-8: The `cursorStyle` lipgloss style that was only used for the cursor char may be removed if unused elsewhere

## Non-Goals

- No changes to the right pane or any other UI component
- No changes to keyboard navigation logic or the `treeCursor` state
- No changes to the background color (`#1f2937`)
- No new color theme tokens — use `#ffffff` (bright white) inline for the boosted foreground

## Technical Considerations

- All changes are isolated to `src/cmd/status_leftpane.go`
- The fixed left overhead for plan rows changes from `4` to `3` (cursor removed, layout: `expandIcon(1) + space(1) + spinner(1)`)
- The `cursorStyle` variable was used exclusively for the cursor char; if it's not referenced elsewhere after this change, remove it
- Boost logic: wrap the already-computed `titleStr` in a white style when `isSelected && m.leftFocused && !plan.Completed && !plan.IsBug`
- For task rows: apply white style to `taskIDStr` and `taskTitleStr` when `isSelected && m.leftFocused`

## Success Metrics

- Selected row is immediately obvious at a glance without needing to spot a small triangle
- Color semantics remain intact (completed = dim, bug = red)
- Zero regressions in layout or build

## Open Questions

_None._
