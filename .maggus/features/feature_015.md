<!-- maggus-id: 0ec6e534-5ae0-46f0-a444-3e7747e01e4b -->
# Feature 015: Consistent Key Navigation — ESC for Cancel Only, Q for Back/Quit

## Introduction

Standardize keyboard navigation across all TUI views. Currently ESC is used inconsistently: sometimes it goes back, sometimes it quits, sometimes it cancels a dialog. This creates a confusing experience. The new rule is simple: ESC only cancels modal dialogs/pickers; `q` is the only key that navigates back or quits. `ctrl+c` is also removed from quit.

## Goals

- ESC never closes a view or navigates between screens
- ESC may still cancel modal dialogs and pickers (e.g. approve picker, file browser, confirmation prompts)
- `q` is the sole key for going back and for quitting
- `ctrl+c` is removed from all explicit quit handlers (OS-level SIGINT may still terminate the process)
- All footer/hint text reflects the new key mappings

## Tasks

### TASK-015-001: Fix ESC key handling in `cmd/` package
**Description:** As a user, I want consistent key navigation in all cmd-level TUI screens so that ESC never unexpectedly closes a view or quits the app.

**Token Estimate:** ~60k tokens
**Predecessors:** none
**Successors:** none
**Parallel:** yes — can run alongside TASK-015-002

**Files to change:**

| File | Line(s) | Current behavior | Required change |
|------|---------|-----------------|-----------------|
| `cmd/config.go` | 330 | `"q", "ctrl+c", "esc"` → quit | Remove `"esc"` and `"ctrl+c"` from case; keep `"q"` only. Update footer hint `"q/esc: exit"` → `"q: exit"` |
| `cmd/gitsync.go` | 273 | `tea.KeyEsc` in Menu state → `tea.Quit` | Remove `tea.KeyEsc` case (ESC should not quit) |
| `cmd/gitsync.go` | 248, 258 | `"q", "esc"` / `"n", "N", "esc"` in DirtyOnly/ConfirmForce | Keep ESC — these are dialog cancel/confirmation screens, not view navigation |
| `cmd/menu_update.go` | 218 | `"q", "esc", "ctrl+c"` → quit or stop confirm | Remove `"esc"` and `"ctrl+c"`; keep `"q"` only |
| `cmd/menu_update.go` | 273 | `"esc", "q"` in submenu → back to main menu | Remove `"esc"`; keep `"q"` only. Update hint text |
| `cmd/menu_update.go` | 329 | `tea.KeyEnter, tea.KeyEscape` in stop daemon confirm → cancel | Keep `tea.KeyEscape` — this is a dialog cancel, not view navigation |
| `cmd/repos.go` | 177 | `"q", "esc", "ctrl+c"` → quit | Remove `"esc"` and `"ctrl+c"`; keep `"q"` only. Update hint text |
| `cmd/tasklist.go` | 194 | `"q", "esc", "ctrl+c"` in list → quit | Remove `"esc"` and `"ctrl+c"`; keep `"q"` only |
| `cmd/tasklist.go` | 226 | `"esc", "backspace"` in detail → close detail | Remove `"esc"`; keep `"backspace"` (or add `"q"` if not already present). Update hint text |
| `cmd/detail.go` | 277 | Footer hint: `"esc: cancel"` in action picker | Keep — action picker is a dialog, ESC cancel is correct |
| `cmd/detail.go` | 279 | Footer hint: `"esc: back"` in blocked criteria mode | Change hint to `"q: back"` and update key handler to use `"q"` not `"esc"` |
| `cmd/detail.go` | 288 | Footer hint: `"esc: back"` in scrollable detail view | Change hint to `"q: back"` and update key handler to use `"q"` not `"esc"` |
| `cmd/status_update.go` | 410 | ESC not in detail → quit | Remove ESC from quit path; only `"q"` should quit |
| `cmd/status_update.go` | 432 | `"q", "esc", "ctrl+c"` → quit | Remove `"esc"` and `"ctrl+c"`; keep `"q"` only. Update hint text |
| `cmd/update.go` | 197, 209 | `tea.KeyEscape` → quit in checking/downloading/confirm phases | Remove `tea.KeyEscape` from quit; keep `tea.KeyCtrlC` removal as well |
| `cmd/approve.go` | 236 | `"esc", "q", "ctrl+c"` → cancel picker | Keep ESC (dialog cancel). Remove `"ctrl+c"`. Update hint `"Esc/q to cancel"` → `"Esc/q to cancel"` (ESC is valid here, hint stays) |
| `cmd/prompt_picker.go` | 77 | `"q", "esc", "ctrl+c"` → cancel picker | Keep ESC (dialog cancel). Remove `"ctrl+c"` from explicit handler |

**Acceptance Criteria:**
- [x] In all cmd/ views, pressing ESC does not close the view or navigate back
- [x] In all cmd/ views, pressing ESC does not quit the application
- [x] Pressing `q` navigates back from submenus and detail views
- [x] Pressing `q` quits top-level views (config, repos, status)
- [x] ESC still cancels modal dialogs: approve picker, prompt picker, stop-daemon confirmation, gitsync confirmation dialogs
- [x] `ctrl+c` is removed from all explicit quit/cancel handlers in `cmd/`
- [x] All footer hint text in `cmd/` reflects the new mappings (no "esc: back", no "esc: exit")
- [x] `go vet ./cmd/...` passes with no errors

---

### TASK-015-002: Fix ESC key handling in `internal/` packages
**Description:** As a user, I want consistent key navigation in the runner and internal TUI components so that ESC never closes a view or quits.

**Token Estimate:** ~40k tokens
**Predecessors:** none
**Successors:** none
**Parallel:** yes — can run alongside TASK-015-001

**Files to change:**

| File | Line(s) | Current behavior | Required change |
|------|---------|-----------------|-----------------|
| `internal/runner/tui_summary.go` | 108 | `tea.KeyEscape, tea.KeyCtrlC, tea.KeyEnter` → quit | Remove `tea.KeyEscape` and `tea.KeyCtrlC`; keep `tea.KeyEnter` and add `'q'` case |
| `internal/runner/tui_keys.go` | 116 | `tea.KeyEscape` → close stop picker | Keep — stop picker is a dialog/overlay, ESC cancel is correct |
| `internal/runner/tui_sync.go` | 151 | `"n", "N", "esc"` in confirmForce → cancel force pull | Keep ESC — this is a dialog cancel |
| `internal/runner/tui_sync.go` | 171 | `tea.KeyEsc` in menu/abort path → abort sync + quit | Remove `tea.KeyEsc`; abort/quit only via `'q'`. Update any hint text |
| `internal/tui/filebrowser/filebrowser.go` | 187 | `tea.KeyEsc` → cancel browser | Keep — file browser is a modal dialog, ESC cancel is correct |

**Acceptance Criteria:**
- [ ] In the runner summary screen, ESC does not exit; `q` or Enter exits
- [ ] In the runner sync screen menu state, ESC does not abort; `q` aborts
- [ ] ESC still cancels the stop picker overlay (dialog cancel behavior preserved)
- [ ] ESC still cancels the force-pull confirmation dialog in tui_sync.go
- [ ] ESC still cancels the file browser
- [ ] `ctrl+c` is removed from the runner summary quit handler
- [ ] All hint text in `internal/` reflects the new mappings
- [ ] `go vet ./internal/...` passes with no errors

---

## Task Dependency Graph

```
TASK-015-001 ──┐
               └──→ (both complete independently, no shared dependencies)
TASK-015-002 ──┘
```

| Task | Estimate | Predecessors | Parallel | Model |
|------|----------|--------------|----------|-------|
| TASK-015-001 | ~60k | none | yes (with 002) | — |
| TASK-015-002 | ~40k | none | yes (with 001) | — |

**Total estimated tokens:** ~100k

## Functional Requirements

- FR-1: ESC must never trigger view dismissal or back-navigation in any screen
- FR-2: ESC must never trigger application quit in any screen
- FR-3: ESC may trigger cancel on modal dialogs and pickers (approve picker, prompt picker, file browser, confirmation dialogs)
- FR-4: `q` must be the single key responsible for both "go back" and "quit" at the top level
- FR-5: `ctrl+c` must be removed from all explicit quit/cancel key handlers (OS SIGINT may still terminate the process)
- FR-6: Footer/hint text must be consistent: show `q` for back/quit, show `esc` only where ESC is a valid dialog cancel
- FR-7: `backspace` may remain as an additional "back" key in detail views if already present

## Non-Goals

- Not changing the overall quit/back UX beyond key mappings
- Not adding new navigation gestures or shortcuts
- Not changing `ctrl+c` OS signal handling (only explicit key handler removal)
- Not modifying non-TUI code paths

## Technical Considerations

- Two implementation styles exist in the codebase — string-based (`msg.String() == "esc"`), type-based (`msg.Type == tea.KeyEscape`), and rune-based (`msg.Runes[0] == 'q'`). Both must be covered when searching for ESC usages.
- When removing ESC from a `case "esc", "q", "ctrl+c":` block, ensure `"q"` remains in the case or is handled separately.
- Removing `ctrl+c` from explicit handlers does not prevent SIGINT from terminating the process — the OS-level behavior is unchanged.
- After changes, verify no "esc" string or `tea.KeyEsc`/`tea.KeyEscape` constant appears in any back/quit path. A `grep -r "KeyEsc\|\"esc\"" src/` can help audit.

## Success Metrics

- Zero ESC key handlers remain in any back-navigation or quit path
- All footer hints correctly reflect the active keys for each action
- `go test ./...` passes after changes
