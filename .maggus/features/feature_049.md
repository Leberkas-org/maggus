<!-- maggus-id: 4762383f-abc2-4a4b-85cf-9b88a11b11f3 -->
# Feature 049: Case-Insensitive TUI Keyboard Shortcuts

## Introduction

Make all single-letter keyboard shortcuts in the Bubble Tea TUI work regardless of Caps Lock state. Terminals cannot detect Caps Lock — when active, pressing `q` sends `Q`, which doesn't match `case "q":` and the shortcut silently fails. This is a recurring usability annoyance.

### Architecture Context

- **Vision alignment:** Improves the TUI experience — core interaction surface of maggus
- **Components involved:** All key handler functions in `src/cmd/` (status, tasklist, config, approve, repos, menu, prompt picker)
- **Approach:** Add a `normalizeKey` helper in `src/cmd/` and apply it at the top of every `switch msg.String()` block. The one exception is the vim-style `g`/`G` distinction in `status_update.go` (go-to-top vs go-to-bottom) which stays case-sensitive.

## Goals

- Every single-letter shortcut works identically whether Caps Lock is on or off
- Preserve the intentional `g` (top) / `G` (bottom) case distinction
- Minimal, non-invasive change — one helper function, mechanical application

## Tasks

### TASK-049-001: Add normalizeKey helper and apply to all key handlers
**Description:** As a user, I want all keyboard shortcuts to work regardless of Caps Lock state so that I don't get silently ignored input.

**Token Estimate:** ~40k tokens
**Predecessors:** none
**Successors:** none
**Parallel:** no

Add a package-level helper function `normalizeKey` in `src/cmd/` that lowercases single ASCII letter strings (length 1, A-Z). Multi-character strings (like `ctrl+c`, `alt+p`, `enter`, `esc`) pass through unchanged.

Then apply it in every key handler function listed below. The pattern is: replace `msg.String()` with `normalizeKey(msg)` (or assign `key := normalizeKey(msg)` then switch on `key`).

**Special case:** In `status_update.go` `updateList()`, the `g`/`G` scroll block (lines ~615-632) must NOT use the normalizer. Handle this by either:
- Switching on `msg.String()` for just that block before the normalized switch, or
- Checking `msg.String()` directly for `"G"` before falling into the normalized path

**Files and functions to update:**

| File | Function | Keys affected |
|------|----------|---------------|
| `status_update.go` | `updateList()` | `j`, `k`, `h`, `l`, `a`, `x`, `s`, `q` (but NOT `g`/`G`) |
| `status_update.go` | `updateStatusConfirmDelete()` | `y`, `n` (already paired — normalizer makes the uppercase cases redundant, clean them up) |
| `status_update.go` | `updateStatusConfirmDeleteFeature()` | `y`, `n` (same as above) |
| `status_update.go` | `updateStatusDaemonStopOverlay()` | `s`, `k` (already paired — clean up) |
| `status_update.go` | `updateExitDaemonOverlay()` | `d`, `s`, `k`, `q` (already paired — clean up) |
| `tasklist.go` | `updateListNav()` | `j`, `k`, `q` |
| `tasklist.go` | `updateDetail()` | `q`, `b` |
| `tasklist.go` | `updateCriteriaMode()` | `j`, `k`, `q` |
| `tasklist.go` | `updateActionPicker()` | `j`, `k` |
| `tasklist.go` | `updateConfirmDelete()` | `y`, `n` (already paired — clean up) |
| `config.go` | `configModel.Update()` | `q`, `j`, `k`, `h`, `l` |
| `approve.go` | `pickerModel.Update()` | `j`, `k`, `q` |
| `prompt_picker.go` | `promptPickerModel.Update()` | `q`, `j`, `k` |
| `menu_update.go` | `updateMainMenu()` | `q` |
| `menu_update.go` | `updateSubMenu()` | `q`, `j`, `k`, `h`, `l` |
| `repos.go` | `updateList()` | `q`, `j`, `k`, `n`, `d`, `s`, `a` |
| `repos.go` | `updateConfirmInit()` | `y`, `n` |

**Acceptance Criteria:**
- [x] `normalizeKey` helper exists in `src/cmd/` — lowercases single ASCII letters, passes everything else through
- [x] Unit test for `normalizeKey`: verifies `"Q"` -> `"q"`, `"q"` -> `"q"`, `"ctrl+c"` -> `"ctrl+c"`, `"G"` -> `"g"`, `"enter"` -> `"enter"`
- [x] All 17 key handler functions listed above use `normalizeKey` for their switch
- [x] `g`/`G` distinction in `updateList()` is preserved — `g` still goes to top, `G` still goes to bottom
- [x] Redundant uppercase cases (`"Y"`, `"N"`, `"S"`, `"K"`, `"D"`, `"Q"`) removed from handlers that previously duplicated both forms — the normalizer makes them unnecessary
- [x] `go build ./...` succeeds
- [x] `go test ./...` passes
- [x] `go vet ./...` clean

## Task Dependency Graph

```
TASK-049-001
```

| Task | Estimate | Predecessors | Parallel | Model |
|------|----------|--------------|----------|-------|
| TASK-049-001 | ~40k | none | no | — |

**Total estimated tokens:** ~40k

## Functional Requirements

- FR-1: Pressing `q` or `Q` (Caps Lock) must both trigger quit/back in every TUI screen
- FR-2: Pressing `j`/`k` or `J`/`K` must both navigate up/down in every list
- FR-3: Pressing `y`/`n` or `Y`/`N` must both work in every confirmation dialog
- FR-4: Pressing `g` must go to top, pressing `G` (Shift+g) must go to bottom — this is the only case-sensitive binding
- FR-5: Multi-character keys (`ctrl+c`, `alt+p`, `enter`, `esc`, `backspace`) must not be affected by normalization
- FR-6: Number keys (`1`-`5` for tab switching) must not be affected

## Non-Goals

- No new keybindings or shortcut changes beyond case normalization
- No changes to the rune-based switch in `menu_update.go` `updateConfirmStopDaemon()` (it already handles both cases via rune matching)
- No Caps Lock detection (impossible in terminals)

## Technical Considerations

- The helper should operate on `tea.KeyMsg` (not raw string) so callers stay clean: `normalizeKey(msg)` rather than `normalizeKey(msg.String())`
- For the `g`/`G` exception: check `msg.String()` for `"G"` before the normalized switch in `updateList()`. This keeps the special case explicit and localized.

## Success Metrics

- All single-letter shortcuts work with Caps Lock on
- No behavioral regression — `g`/`G` distinction preserved, multi-key combos unaffected

## Open Questions

None — scope is fully defined.
