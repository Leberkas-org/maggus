<!-- maggus-id: 6bd05ed3-2517-4c7c-920e-649c93985ca5 -->
# Feature 021: Fix Tree Row Selection — Full-Row Background Highlight

## Introduction

Feature 020 introduced selection highlighting for tree rows in the left pane, but the background color only applies to the first character of each row. The root cause is a lipgloss rendering issue: `selectedBg.Render(rowContent)` wraps the whole row in a background escape (`\x1b[48;...m`), but each individually styled sub-element inside `rowContent` (expand icon, spinner, badge, etc.) emits its own `\x1b[0m` reset that kills the background mid-row. After the first inner reset, all subsequent characters render on the default (transparent) background.

The fix is to stop applying background at the outer level and instead embed the background color in every styled sub-element when that row is selected.

## Goals

- The entire selected row — including spaces, icons, spinner placeholder, badges, padding — renders on the dark background (`#1f2937`)
- No per-element color semantics are lost (green badge stays green, red bugs stay red, etc.)
- The fix applies to both plan rows and task rows

## Tasks

### TASK-021-001: Embed selection background in every sub-element style

**Description:** As a user navigating the tree, I want the full width of the selected row to have the dark background highlight so that the selection is visually unambiguous.

**Token Estimate:** ~30k tokens
**Predecessors:** none
**Successors:** none
**Parallel:** no

**Acceptance Criteria:**
- [x] A helper `withBg(style lipgloss.Style, bg lipgloss.Color) lipgloss.Style` (or inline) is used to copy any existing style and add the background color when the row is selected
- [x] For selected plan rows: `expandIcon`, `spinStr`, `titleStr`, `progBadge`, `badge`, and all literal space/padding strings are rendered with the background color included in their style
- [x] For selected task rows: `spinStr`, `taskIDRendered`, `taskTitleRendered`, the indent spaces (`"   "`), and the separating space are all rendered with the background color included
- [x] The outer `selectedBg.Render(padToWidth(rowContent, contentW))` call is removed; instead, `rowContent` itself is already fully colored and padded — just use `padToWidth(rowContent, contentW)` with no outer wrap
- [x] Plain spaces and padding (`strings.Repeat(" ", n)`) within a selected row are wrapped in a style that sets only the background color (no foreground change)
- [x] When the left pane is NOT focused, the selected row still shows the background on the full row (unfocused selection — no foreground boost, but full-width background)
- [x] Non-selected rows are unchanged — no background is set on any sub-element
- [x] The dark background color (`#1f2937`) is defined once as a named `const` or variable (e.g. `selectedBgColor`) and reused everywhere — not duplicated as a string literal
- [x] Visual result: the full row width from column 0 to `contentW-1` renders on the dark background with no gaps
- [x] `go build ./...` passes
- [x] `go vet ./...` passes

## Task Dependency Graph

```
TASK-021-001
```

| Task | Estimate | Predecessors | Parallel | Model |
|------|----------|--------------|----------|-------|
| TASK-021-001 | ~30k | none | no | — |

**Total estimated tokens:** ~30k

## Functional Requirements

- FR-1: Every styled sub-element in a selected row must include `Background(selectedBgColor)` in its lipgloss style
- FR-2: Every plain-text padding or space string in a selected row must be wrapped in `lipgloss.NewStyle().Background(selectedBgColor).Render(...)`
- FR-3: The `selectedBg.Render(...)` outer wrapper must be removed from both plan and task row rendering
- FR-4: The background color `#1f2937` must be stored in exactly one place (e.g. `selectedBgColor := lipgloss.Color("#1f2937")`) and referenced by all sub-element styles
- FR-5: Unfocused selected rows (left pane not focused) must still show full-row background with no gaps
- FR-6: Non-selected rows must not gain any background color

## Non-Goals

- No changes to which elements are white vs. colored on selection (that was feature 020)
- No changes to navigation logic, key bindings, or model state
- No changes to the right pane or any other component
- No new theme tokens in `internal/tui/styles` — keep the color local to `status_leftpane.go`

## Technical Considerations

- The pattern to apply: instead of `mutedStyle.Render(x)`, use `mutedStyle.Copy().Background(selBg).Render(x)` when `isSelected`
- Plain spaces: `strings.Repeat(" ", n)` → `lipgloss.NewStyle().Background(selBg).Render(strings.Repeat(" ", n))` when `isSelected`
- The indent prefix for task rows (`"   "`) must also be background-colored when selected
- `padToWidth` appends raw spaces — for selected rows, replace its usage with a version that pads using background-colored spaces, OR pad manually inline
- An alternative cleaner approach: after assembling `rowContent` with all sub-elements carrying the background, call `padToWidth` to add trailing spaces, then wrap only those trailing spaces in the background style. But embedding background in each sub-element is simpler and more robust.
- All changes are isolated to `src/cmd/status_leftpane.go`

## Success Metrics

- Moving the cursor through the tree shows a solid dark-background highlight bar from left edge to right border, with no gaps or color breaks
- Per-element foreground colors (green badge, red bug title, muted progress) remain visible on top of the dark background

## Open Questions

_None._
