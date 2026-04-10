<!-- maggus-id: d37de463-f804-423d-b56e-73631751c7f5 -->
# Feature 041: Ctrl+S to Save Config

## Introduction

Add `Ctrl+S` as a keyboard shortcut to save the currently active config tab in the config editor TUI. Currently, saving requires navigating to the "Save project config" or "Save global config" button and pressing Enter. `Ctrl+S` is a universal save shortcut that users expect.

### Architecture Context

- **Vision alignment:** "Rich terminal UI" — standard keyboard shortcuts improve usability
- **Components involved:** `cmd/config.go` (config editor model, Update handler)
- **New patterns:** None — just a key binding addition

## Goals

- Users can press `Ctrl+S` anywhere in the config editor to save the active tab's config
- Visual feedback confirms the save succeeded (same "Saved project config" / "Saved global config" status text)

## User Stories

### TASK-041-001: Add Ctrl+S shortcut to config editor
**Description:** As a user editing config settings, I want to press Ctrl+S to save immediately so I don't have to navigate to the save button.

**Token Estimate:** ~15k tokens
**Predecessors:** none
**Successors:** none
**Parallel:** no

**Acceptance Criteria:**
- [x] Pressing `Ctrl+S` in the config editor triggers `configActionSaveProject` when the Project tab (tab 0) is active
- [x] Pressing `Ctrl+S` in the config editor triggers `configActionSaveGlobal` when the Global tab (tab 1) is active
- [x] The status text shows "Saved project config" or "Saved global config" on success, same as the button
- [x] The shortcut works regardless of cursor position (not just when on the save button)
- [x] The shortcut works when the tab bar is focused
- [x] The footer hint in the config view includes `ctrl+s: save`
- [x] `go vet ./...` and `go test ./...` pass
- [x] Unit test verifies that `ctrl+s` on tab 0 triggers project save and on tab 1 triggers global save

## Task Dependency Graph

```
TASK-041-001 (single task)
```

| Task | Estimate | Predecessors | Parallel | Model |
|------|----------|--------------|----------|-------|
| TASK-041-001 | ~15k | none | no | — |

**Total estimated tokens:** ~15k

## Functional Requirements

- FR-1: `Ctrl+S` on the Project tab saves `.maggus/config.yml`
- FR-2: `Ctrl+S` on the Global tab saves `~/.maggus/config.yml`
- FR-3: Status feedback is identical to pressing the save button

## Non-Goals

- No auto-save on exit
- No undo/revert functionality
- No "unsaved changes" indicator

## Open Questions

None.
