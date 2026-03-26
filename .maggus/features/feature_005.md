<!-- maggus-id: 2e8559ed-05cc-45fa-85d1-cd6c5d52e1d2 -->
<!-- maggus-id: 20260326-170556-feature-005 -->
# Feature 005: Status UI — Full-width footer separator with unified shared footer

## Introduction

The `status` split-pane view currently has a visual inconsistency: the left pane renders a horizontal border line (`─────────────────┴`) above the shared footer, but the right pane renders nothing on that same row, leaving the separator incomplete. Additionally, the Tab 2 (Feature Details) task list and detail sub-view each embed their own inline footer hints, duplicating (and diverging from) the shared footer logic in `statusSplitFooter()`.

This feature:
1. Extends the footer separator line across the full width by adding a matching `─` border line to the right pane
2. Uses the existing `┴` junction character (already in the left pane) to naturally connect both halves
3. Consolidates all embedded tab footers into the shared `statusSplitFooter()` function

### Architecture Context

- **Components touched:** `src/cmd/status_rightpane.go`, `src/cmd/status_view.go`, `src/cmd/status_update.go`
- **Pattern:** All three files are part of the Bubble Tea TUI following the split model/update/view pattern defined in CLAUDE.md
- **No new components** — this is a rendering cleanup within existing files

## Goals

- Render a clean full-width `─────────────────┴────────────────────────────────────` separator line spanning both panes above the footer
- Ensure the separator uses the same theme color as the left pane border (`styles.ThemeColor(m.is2x)`)
- Remove all inline footer hints embedded inside tab content renderers
- Consolidate all key hints into `statusSplitFooter()`, which already switches on `leftFocused` and `activeTab`

## Tasks

### TASK-005-001: Add right pane footer separator line

**Description:** As a user, I want a full-width separator line above the footer so that the visual divider spans both the left and right panes consistently.

**Token Estimate:** ~15k tokens
**Predecessors:** none
**Successors:** none
**Parallel:** yes — can run alongside TASK-005-002
**Model:** haiku

**Acceptance Criteria:**
- [x] `renderRightPane` in `status_rightpane.go` appends `"\n" + borderLine` after `lipgloss.NewStyle().Width(width).Height(height).Render(full)`
- [x] `borderLine` is `strings.Repeat(borderStyle.Render("─"), width)` where `borderStyle` uses `lipgloss.NewStyle().Foreground(styles.ThemeColor(m.is2x))`
- [x] The right pane now produces `height + 1` lines total (matching the left pane)
- [x] When rendered, the full separator row reads `─────────────┴──────────────────────────────────────` with `┴` at the left/right pane boundary
- [x] `rightPaneContentHeight()` and `outputTabScrollableLines()` are NOT changed (the internal content area is unchanged)
- [x] `go build ./...` passes
- [x] `go test ./...` passes

---

### TASK-005-002: Remove embedded footer from Tab 2 task list, update shared footer

**Description:** As a user, I want the key hints for the Feature Details task list to appear in the shared footer bar so that hints are always in a consistent location.

**Token Estimate:** ~25k tokens
**Predecessors:** none
**Successors:** TASK-005-003
**Parallel:** yes — can run alongside TASK-005-001
**Model:** haiku

**Acceptance Criteria:**
- [ ] In `renderTab2TaskList` (`status_rightpane.go`): remove `footerStr`, `footerLines := 1`, the `footerLines` deduction from `listH`, and the `+ "\n" + footerStr` at the return
- [ ] `renderTab2TaskList` returns `lipgloss.NewStyle().Width(width).Height(height).Render(sb.String())` using the full `height` (no footer reservation)
- [ ] `statusSplitFooter()` (`status_view.go`) case `activeTab == 1` (non-detail mode) now includes a tab-switching hint; the text must include at minimum: navigate hint, enter hint, tab/pane-switch hint, and q: exit — e.g. `"↑/↓ navigate  enter: detail  tab: switch pane  1: left  2-5: tabs  q: exit"`
- [ ] No duplicate footer appears in the task list area
- [ ] `go build ./...` passes
- [ ] `go test ./...` passes

---

### TASK-005-003: Move Tab 2 detail sub-view footer into shared footer

**Description:** As a user, I want the detail view key hints (scroll, blocked criteria actions, etc.) to appear in the shared footer bar so that the right pane content area is used fully and hints are always in one place.

**Token Estimate:** ~50k tokens
**Predecessors:** TASK-005-002
**Successors:** none
**Parallel:** no

**Acceptance Criteria:**
- [ ] In `renderTab2Detail` (`status_rightpane.go`): remove `footer := detailFooter(...)` and remove `+ "\n" + footer` from the return; the function now renders the full `height` into the viewport with no footer line reserved
- [ ] In `status_update.go` (the function that sizes `detailViewport`): remove the `-1` comment and change `vpH := contentH - 1` to `vpH := contentH`, then update the associated comment
- [ ] In `statusSplitFooter()` (`status_view.go`): add sub-state handling for `activeTab == 1` when `m.taskListComponent.ShowDetail` is true — replicate the 4 states from `detailFooter()`:
  - `criteriaMode && showActionPicker` → `"↑/↓: select action  enter: confirm  esc: cancel"`
  - `criteriaMode` → `"↑/↓: navigate blocked  enter: action  tab: scroll mode  esc: back"`
  - `ShowDetail && scrollable` → include scroll hint + pgup/pgdn + tab/manage blocked + alt+r/alt+bksp + esc/q hints
  - `ShowDetail` (not scrollable) → same minus scroll hint
- [ ] For the "scrollable" sub-state: scrollable is `c.detailViewport.TotalLineCount() > c.detailViewport.Height`
- [ ] No duplicate footer appears inside the detail view content area
- [ ] `detailFooter()` in `detail.go` is NOT deleted — it is still used by `tasklist.go` (standalone task list component)
- [ ] `go build ./...` passes
- [ ] `go test ./...` passes

---

## Task Dependency Graph

```
TASK-005-001  (independent)
TASK-005-002 ──→ TASK-005-003
```

| Task | Estimate | Predecessors | Parallel | Model |
|------|----------|--------------|----------|-------|
| TASK-005-001 | ~15k | none | yes (with 002) | haiku |
| TASK-005-002 | ~25k | none | yes (with 001) | haiku |
| TASK-005-003 | ~50k | 002 | no | — |

**Total estimated tokens:** ~90k

## Functional Requirements

- FR-1: The right pane must render a `─` border line as its last line, using `styles.ThemeColor(m.is2x)` for color, producing `height + 1` total lines (same structure as the left pane)
- FR-2: When joined horizontally, the left pane `─────────────┴` and right pane `──────────────────────────────────` must produce a single continuous separator row with the `┴` at the pane boundary column
- FR-3: `renderTab2TaskList` must use the full `height` parameter for content (no reserved footer line)
- FR-4: `statusSplitFooter()` for `activeTab == 1`, non-detail mode must include a tab-switch hint
- FR-5: `statusSplitFooter()` for `activeTab == 1`, detail mode must replicate all 4 states from `detailFooter()` (criteriaMode+actionPicker, criteriaMode, scrollable, not scrollable)
- FR-6: `detailViewport.Height` must be set to `contentH` (not `contentH - 1`) after removing the inline footer
- FR-7: `detailFooter()` in `detail.go` must not be deleted (still used by standalone task list)

## Non-Goals

- No changes to the outer box border rendering (no `├`/`┤` T-junctions at the outer box edges — not achievable without replacing lipgloss Box rendering)
- No changes to `status_logview.go` (separate full-screen view, not part of the split pane)
- No changes to `renderTab2ConfirmDelete` (no embedded footer there)
- No changes to `rightPaneContentHeight()` or `outputTabScrollableLines()` (TASK-005-001 appends the border after `Height(height).Render()`, so internal content area is unchanged)
- No changes to the left pane (`status_leftpane.go`) — already correct

## Technical Considerations

- The left pane appends its border line after `strings.Join(result, "\n")` (i.e. after the height-padded content). The right pane should mirror this: `Height(height).Render(full) + "\n" + borderLine`
- `statusSplitFooter()` needs access to `m.taskListComponent.detailViewport` to determine the "scrollable" sub-state. `taskListComponent` is embedded in `statusModel`, so `m.taskListComponent.detailViewport.TotalLineCount() > m.taskListComponent.detailViewport.Height` is accessible directly
- The `ShowDetail`, `criteriaMode`, and `showActionPicker` fields are on `taskListComponent` — accessible as `m.taskListComponent.ShowDetail`, etc.

## Open Questions

None.
