<!-- maggus-id: efc849ce-8b70-4772-a76d-797966e79f66 -->
# Feature 007: Show Completed Features in Status Left Pane

## Introduction

The status view's left pane already supports showing completed features via an `alt+a` toggle (`showAll` flag), but the shortcut is never displayed in the footer — making it completely undiscoverable. Users have no way of knowing it exists unless they read the source code.

This feature adds `alt+a: show done` to the left-pane footer hint, but only when there are actually completed features (or bugs) present, so it doesn't clutter the footer when there is nothing to toggle.

### Architecture Context

- **Vision alignment:** Reduces friction in the core feedback loop — users should be able to see what's been done without leaving the status view.
- **Components involved:** `cmd/status_view.go` (footer hint logic in `statusSplitFooter()`), `cmd/status_model.go` (helper to detect completed plans).
- **New patterns:** None. Purely additive to the existing footer string.

## Goals

- Make `alt+a` discoverable by showing it in the left-pane footer when completed features/bugs exist.
- Keep the footer uncluttered when there are no completed items to show.
- No changes to toggle behaviour, visual state, or any other part of the status view.

## Tasks

### TASK-007-001: Add `alt+a` hint to the left-pane footer

**Description:** As a user, I want the footer to tell me I can press `alt+a` to show completed features so that I can discover the toggle without reading source code.

**Token Estimate:** ~20k tokens
**Predecessors:** none
**Successors:** none
**Parallel:** no

**Acceptance Criteria:**
- [x] `cmd/status_model.go`: a helper method `func (m statusModel) hasCompletedPlans() bool` is added that returns `true` if any plan in `m.plans` has `Completed == true`.
- [x] `cmd/status_view.go`: in `statusSplitFooter()`, the `m.leftFocused` branch appends `  alt+a: show done` to the footer string when `m.hasCompletedPlans()` returns `true` AND `m.showAll` is `false`.
- [x] When `m.showAll` is `true` (completed items are already visible), the hint changes to `  alt+a: hide done` so the user knows they can toggle back.
- [x] When `m.hasCompletedPlans()` returns `false`, neither hint appears.
- [x] `cmd/status_test.go`: unit tests cover the three states: no completed plans (no hint), completed plans with `showAll=false` (`show done` hint), completed plans with `showAll=true` (`hide done` hint).
- [x] `go build ./...` and `go test ./...` pass from `src/`.

## Task Dependency Graph

```
TASK-007-001
```

| Task | Estimate | Predecessors | Parallel | Model |
|------|----------|--------------|----------|-------|
| TASK-007-001 | ~20k | none | no | haiku |

**Total estimated tokens:** ~20k

## Functional Requirements

- FR-1: When the left pane has focus and `m.plans` contains at least one plan with `Completed == true` and `m.showAll == false`, the footer must include the text `alt+a: show done`.
- FR-2: When the left pane has focus and `m.plans` contains at least one plan with `Completed == true` and `m.showAll == true`, the footer must include the text `alt+a: hide done`.
- FR-3: When `m.plans` contains no completed plans, neither `alt+a` hint appears in the footer regardless of `showAll`.
- FR-4: The hint must not appear when the right pane has focus.
- FR-5: No changes to the `alt+a` key handler logic in `status_update.go`.

## Non-Goals

- Changing the default value of `showAll` (it stays `false`).
- Adding any visual indicator to the left pane header or rows when `showAll` is active.
- Modifying how completed plans are loaded or filtered.
- Adding any new keyboard shortcuts.

## Technical Considerations

- `statusSplitFooter()` in `status_view.go:190` builds the footer string. The `m.leftFocused` branch at line 197 is the only place to modify.
- `hasCompletedPlans()` should be a pure method on `statusModel` — no side effects. It just iterates `m.plans` (the full unfiltered list).
- The left-focused footer string currently ends with `q: exit`. Append the hint after that, separated by two spaces, so it reads: `... q: exit  alt+a: show done`.

## Success Metrics

- A user who opens `maggus status` with completed features present can discover and use `alt+a` without any external documentation.
- Footer length stays within terminal width for typical terminal sizes (80+ columns) — the added hint is 18 characters max.

## Open Questions

_(none)_
