<!-- maggus-id: fe5fabba-fe15-4fec-b573-022fd0137039 -->
# Feature 042: Remove Left Pane Focus Mode — Always-Active Tree Navigation

## Introduction

After feature 040 (context-sensitive right pane), the status view still has two focus modes: left pane (key `1`, up/down navigates tree) and right pane (keys `2-5`, up/down scrolls tab content). This creates unnecessary cognitive overhead. Since the right pane is now fully driven by left pane selection, up/down should always navigate the tree regardless of which tab is active. Tab number keys simply switch what's displayed on the right — no focus change needed.

This is a follow-up iteration to feature 040 that simplifies the interaction model.

### Architecture Context

- **Vision alignment:** "Rich terminal UI" — simpler interaction model reduces learning curve
- **Components involved:** `cmd/status_update.go` (key handling), `cmd/status_view.go` (footer hints), `cmd/status_rightpane.go` (scroll handling)
- **Predecessor:** Feature 040 (context-sensitive right pane) must be completed first

## Goals

- Remove the left/right pane focus concept entirely
- Up/down always navigates the left pane tree, on every tab
- Tab content that needs scrolling uses dedicated keys (PgUp/PgDn or similar), not up/down
- Eliminate the `[1]` tab key since there's no left pane focus to switch to

## User Stories

### TASK-042-001: Remove leftFocused state and unify key handling
**Description:** As a user, I want up/down to always navigate the feature tree so I don't have to think about which pane is focused.

**Token Estimate:** ~50k tokens
**Predecessors:** none (depends on feature 040 being completed first)
**Successors:** TASK-042-002
**Parallel:** no
**Model:** opus

**Acceptance Criteria:**
- [x] The `leftFocused` field is removed from `statusModel`
- [x] Up/down keys always drive `treeCursor` navigation (same behavior as current left-focused mode)
- [x] Left/right keys still expand/collapse tree nodes
- [x] Tab number keys (`1`, `2`, `3`, etc.) switch the right pane tab without changing any focus state — they start at `1` now (no more `[1]` for left pane focus)
- [x] PgUp/PgDn on the left pane still jumps between features (existing behavior preserved)
- [x] The `enter` key still opens task detail view for the selected task
- [x] All key handling branches that check `m.leftFocused` are removed or refactored
- [x] `go vet ./...` and `go test ./...` pass
- [x] Existing tests that assert `leftFocused` behavior are updated

### TASK-042-002: Add content scrolling via dedicated keys
**Description:** As a user viewing the Output tab or Task Details tab, I want to scroll the content using dedicated keys so I can read long tool logs or acceptance criteria.

**Token Estimate:** ~30k tokens
**Predecessors:** TASK-042-001
**Successors:** TASK-042-003
**Parallel:** no

**Acceptance Criteria:**
- [x] Output tab (tool log): `Shift+Up` / `Shift+Down` scrolls the tool log (or alternatively `{` / `}` — pick a pair that doesn't conflict)
- [x] Output tab: `g` jumps to top, `G` jumps to bottom (same as current)
- [x] Output tab: auto-scroll behavior for running tasks is preserved
- [x] Task Details tab: `Shift+Up` / `Shift+Down` scrolls the viewport (same keys as Output)
- [x] Summary and Metrics tabs: no scrolling needed (content fits)
- [x] Scrolling keys are no-ops when the active tab has no scrollable content
- [x] `go vet ./...` and `go test ./...` pass

### TASK-042-003: Update footer hints and documentation
**Description:** As a user, I want the footer to show the correct key hints for the new unified model, and the docs to reflect the change.

**Token Estimate:** ~20k tokens
**Predecessors:** TASK-042-002
**Successors:** none
**Parallel:** no

**Acceptance Criteria:**
- [ ] Footer no longer shows `1-5: tabs` — instead shows `1-N: tabs` where N is the count of available right-pane tabs
- [ ] Footer shows scroll hint (e.g. `shift+↑/↓: scroll`) when the active tab has scrollable content
- [ ] Footer no longer mentions left/right pane focus
- [ ] `docs/reference/tui.md` is updated: remove any mention of left pane focus, document the always-active tree and tab content scroll keys
- [ ] `go vet ./...` and `go test ./...` pass

## Task Dependency Graph

```
TASK-042-001 ──→ TASK-042-002 ──→ TASK-042-003
```

| Task | Estimate | Predecessors | Parallel | Model |
|------|----------|--------------|----------|-------|
| TASK-042-001 | ~50k | none | no | opus |
| TASK-042-002 | ~30k | 001 | no | — |
| TASK-042-003 | ~20k | 002 | no | — |

**Total estimated tokens:** ~100k

## Functional Requirements

- FR-1: Up/down keys always navigate the left pane tree, regardless of active tab
- FR-2: Tab number keys switch the right pane display without changing focus
- FR-3: Content scrolling in Output and Task Details uses dedicated keys (not up/down)
- FR-4: The `[1]` left-pane-focus key is removed; tab numbers start at `1` for the first right-pane tab
- FR-5: All existing tree navigation (expand/collapse, PgUp/PgDn between features, Home/End) continues to work

## Non-Goals

- No changes to the context-sensitive tab mapping (that's feature 040)
- No changes to the left pane tree rendering
- No changes to approval, delete, or run-task actions

## Technical Considerations

- The `leftFocused` bool is checked in many places in `status_update.go` — removing it requires carefully auditing every branch
- The `statusSplitFooter()` method has separate branches for left vs right focus — these merge into one
- The task detail view (`ShowDetail`) currently has its own key handling — this needs to be considered (does the detail view still capture up/down for criteria navigation, or does it also use the new model?)
- The blocked criteria mode (`criteriaMode`) definitely needs its own key capture — that's a modal overlay and should keep its current behavior

## Open Questions

None.
