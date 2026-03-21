# Plan: Fix Broken TUI Tab Switching After Refactor

## Introduction

The TUI tab switching in the work view stopped working after the refactoring commit `426494a` ("refactor(tui): delegate Update() to sub-component handlers"). The root cause is a value-receiver bug: `handleTabSwitch` and `handleDetailScroll` in `tui_keys.go` modify `m.activeTab` / scroll offsets on a **copy** of the model, but only return `(tea.Cmd, bool)` — the modified model is discarded by the caller.

## Goals

- Restore tab switching (arrow keys and number keys 1-4) in the work view
- Restore detail panel scrolling (up/down/pgup/pgdn/home/end keys)
- Fix the missing return statement in `tui.go` for `SummaryMsg`/`PushStatusMsg`/`QuitMsg` handling
- Ensure no other value-receiver mutations were introduced by the refactoring

## Root Cause Analysis

### Bug 1: `handleTabSwitch` value receiver discards mutations

**File:** `src/internal/runner/tui_keys.go:172`

```go
func (m TUIModel) handleTabSwitch(msg tea.KeyMsg) (tea.Cmd, bool) {
    // modifies m.activeTab on a COPY — caller never sees the change
}
```

Called from `handleKeyMsg` at line 50:
```go
if cmd, handled := m.handleTabSwitch(msg); handled {
    return m, cmd  // returns the ORIGINAL m, not the modified copy
}
```

### Bug 2: `handleDetailScroll` same issue

**File:** `src/internal/runner/tui_keys.go:145`

```go
func (m TUIModel) handleDetailScroll(msg tea.KeyMsg) (tea.Cmd, bool) {
    // modifies m.detailScrollOffset, m.detailAutoScroll on a COPY
}
```

### Bug 3: Missing return after `handleSummaryMsg`

**File:** `src/internal/runner/tui.go:234-235`

```go
case SummaryMsg, PushStatusMsg, QuitMsg:
    m.summary.handleSummaryMsg(msg, &m)
    // Missing: return m, nil
    // Falls through to the return at the end, which still works BUT
    // the old code returned early when handled==true
```

## User Stories

### TASK-001: Fix Tab Switching and Detail Scrolling in tui_keys.go

**Description:** As a user, I want to switch between tabs and scroll the detail panel during the work view so I can monitor different aspects of the running task.

**Acceptance Criteria:**
- [ ] `handleTabSwitch` is changed to a pointer receiver: `func (m *TUIModel) handleTabSwitch(msg tea.KeyMsg) (tea.Cmd, bool)`
- [ ] `handleDetailScroll` is changed to a pointer receiver: `func (m *TUIModel) handleDetailScroll(msg tea.KeyMsg) (tea.Cmd, bool)`
- [ ] `handleKeyMsg` is changed to a pointer receiver: `func (m *TUIModel) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd)` — required because it calls the above methods on `m`
- [ ] The call site in `tui.go` `Update()` is updated: `case tea.KeyMsg:` now calls `m.handleKeyMsg(msg)` with the pointer receiver (note: `Update()` itself must remain a value receiver per bubbletea's `tea.Model` interface, so dereference or assign back as needed)
- [ ] Tab switching works: left/right arrow keys and number keys 1-4 change the active tab
- [ ] Detail panel scrolling works: up/down/pgup/pgdn/home/end scroll the detail panel on tab 1
- [ ] Stop picker still works: Alt+S opens it, arrow keys navigate, Enter selects
- [ ] `go vet ./...` and `go test ./...` pass

### TASK-002: Fix Missing Return for Summary Message Handling

**Description:** As a developer, I want the summary message handler to return early after processing so that message handling follows the same pattern as the rest of the Update function.

**Acceptance Criteria:**
- [ ] In `tui.go`, the `case SummaryMsg, PushStatusMsg, QuitMsg:` block returns `m, nil` after calling `handleSummaryMsg` (restoring the early return that existed before the refactor)
- [ ] `go test ./...` passes

### TASK-003: Audit All Value-Receiver Methods for Mutation Bugs

**Description:** As a developer, I want to verify that no other value-receiver methods in the TUI package silently discard state mutations.

**Acceptance Criteria:**
- [ ] Review all `func (m TUIModel)` methods in `tui_keys.go`, `tui_messages.go`, `tui_render.go`, and `tui.go`
- [ ] Render methods (`renderXxx`) are confirmed safe as value receivers (they only read, never write)
- [ ] Read-only getters (`StopFlag`, `StopAtTaskIDFlag`, `TaskUsages`) are confirmed safe
- [ ] Any method that mutates `m` fields uses a pointer receiver `*TUIModel`
- [ ] `handleStopPicker` and `applyStopPickerSelection` are checked — they return `(tea.Model, tea.Cmd)` so the model propagates, but verify they correctly modify and return `m`
- [ ] `go vet ./...` and `go test ./...` pass

## Functional Requirements

- FR-1: All state-mutating methods on `TUIModel` must use pointer receivers to avoid silent data loss
- FR-2: The `Update()` method must remain a value receiver (required by `tea.Model` interface) but must ensure mutations from sub-handlers are propagated back through the return value
- FR-3: Every `case` branch in `Update()` that handles a message must explicitly return `m, cmd` — no implicit fall-through to the bottom return

## Non-Goals

- No changes to TUI rendering or layout
- No changes to the summary screen behavior
- No new features — this is purely a bug fix

## Technical Considerations

- Bubbletea's `tea.Model` interface requires `Update(msg tea.Msg) (tea.Model, tea.Cmd)` with a value receiver — this is non-negotiable. The pattern is to mutate the local `m` and return it.
- The refactored sub-handlers that return `(tea.Cmd, bool)` instead of `(tea.Model, tea.Cmd)` are the core issue — they break the bubbletea mutation-via-return pattern. Switching to pointer receivers fixes this because the caller's `m` is mutated in place.
- Alternative approach: change sub-handlers to return `(tea.Model, tea.Cmd)` instead of `(tea.Cmd, bool)`. This is more idiomatic bubbletea but requires more changes. Pointer receivers are simpler here since `Update()` already has `m` as a local copy.

## Success Metrics

- Tab switching works during the work view (arrow keys and number keys)
- Detail panel scrolling works on tab 1
- Stop picker continues to work
- No regression in summary screen behavior
