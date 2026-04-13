<!-- maggus-id: 0cbc4e9e-0693-4dfe-807e-7e26d4b7fb94 -->
# Feature 057: Keyboard Shortcut Help Popup (F1)

## Introduction

The status view footer packs an increasing number of keyboard hints into a single line. As new shortcuts are added (Shift+↑/↓ scroll, Tab/Shift+Tab, dispatch keys, etc.) the footer becomes unreadably crowded. Users have no way to discover or remember all available shortcuts without reading the source code.

This feature adds a floating help popup triggered by `F1`. Pressing `F1` overlays a centered, bordered modal box on top of the current status view — the background remains visible but dimmed beneath it. The modal lists all keyboard shortcuts grouped by category. Pressing `F1` again or `Esc` closes it.

The implementation requires three pieces:
1. A help content builder that produces the formatted shortcut list.
2. An ANSI-safe overlay compositor utility that places one rendered string on top of another without corrupting escape sequences.
3. Wiring the flag, key handler, and overlay call into the status view.

### Architecture Context

- **Vision alignment:** Discoverability is part of a good operator experience. This removes a usability barrier for new users and surfaces hidden shortcuts for experienced ones.
- **Components involved:** `cmd/status_view.go` (View dispatch), `cmd/status_update.go` (key handling), `cmd/status_model.go` (model field), `internal/tui/styles/` (new overlay utility).
- **New patterns:** ANSI overlay compositor (`OverlayCenter`) — a pure function that composites two rendered ANSI strings. Goes in `internal/tui/styles/overlay.go` alongside existing layout helpers.
- **Existing modal pattern:** The delete-confirmation and daemon-stop overlays use a full-screen bool-flag takeover (`styles.FullScreenColor`). This feature follows the same bool-flag pattern but adds a true overlay renderer instead of a full-screen replacement.

## Goals

- All status view keyboard shortcuts visible in one place, reachable with `F1`.
- Modal floats over the dimmed (faint) background — not a full-screen replacement.
- `Esc` or `F1` closes the modal; all other keys are consumed while it is open (no accidental navigation).
- The footer gains a `F1: help` hint; other footer hints can be shortened or removed as the popup takes over their documentation role.

## Tasks

### TASK-057-001: Help modal content builder

**Description:** As a user, I want the help popup to show all status view keyboard shortcuts in a clear, categorised layout so I can quickly find the binding I need.

**Token Estimate:** ~35k tokens
**Predecessors:** none
**Successors:** TASK-057-003
**Parallel:** yes — can run alongside TASK-057-002

**Acceptance Criteria:**
- [x] New function `buildHelpModal(width, height int) string` in `cmd/` (e.g. `status_help.go`) that returns a fully styled, border-framed string ready to be overlaid
- [x] Shortcuts are grouped into labelled sections, at minimum:
  - **Navigation** — `↑/↓`, `pgup/pgdn`, `enter`, `esc`, `q`
  - **Tabs** — `tab`, `shift+tab`, number keys
  - **Output** — `shift+↑/↓` scroll
  - **Actions** — `a` approve, `d` delete, `s` dispatch, `alt+s` stop-after-task, `ctrl+c` kill
  - **Daemon** — start/stop hints
- [x] Each row shows key on the left, description on the right, aligned in two columns
- [x] The modal box has a rounded or normal lipgloss border with a title (e.g. `" Keyboard Shortcuts "`)
- [x] Modal width is capped at `min(width-8, 72)` columns; height is capped at `height-6` rows; content scrolls if there are more rows than fit (use simple top-truncation — no scroll bar needed for the first iteration)
- [x] The rendered string has consistent padding inside the border
- [x] `go vet ./...` passes

---

### TASK-057-002: ANSI overlay compositor utility

**Description:** As a developer, I want a pure utility function that places a rendered modal string on top of a rendered background string (both ANSI-encoded) without corrupting either string's escape sequences, so the help modal can float over the dimmed status view.

**Token Estimate:** ~50k tokens
**Predecessors:** none
**Successors:** TASK-057-003
**Parallel:** yes — can run alongside TASK-057-001
**Model:** opus

**Acceptance Criteria:**
- [x] New file `internal/tui/styles/overlay.go` with exported function:
  ```go
  // OverlayCenter places fg centered over bg, both of which are fully rendered
  // ANSI strings. termW and termH are the terminal dimensions. Lines in bg that
  // fall within the fg bounding box are replaced (left and right flanks of the
  // bg line are preserved). Returns the composited string.
  func OverlayCenter(bg, fg string, termW, termH int) string
  ```
- [x] The function splits both strings by `\n` and computes the centered position: `startX = (termW - fgWidth) / 2`, `startY = (termH - fgHeight) / 2`
- [x] For each row covered by `fg`: the corresponding `bg` line is composited as `left(bgLine, startX) + fgLine + right(bgLine, startX+fgWidth)` where `left`/`right` are visual-column slices using `charmbracelet/x/ansi` (already a project dependency)
- [x] Rows outside the `fg` bounding box are passed through unchanged
- [x] If `startX < 0` or `startY < 0` (modal larger than terminal) the function falls back to returning `fg` directly (centered via `lipgloss.Place`)
- [x] The function is pure (no side effects, no global state) and has no knowledge of the status view
- [x] Unit tests cover: modal exactly fitting terminal, modal smaller than terminal (standard case), modal larger than terminal (fallback), bg line shorter than expected (edge case)
- [x] `go vet ./...` and `go test ./...` pass

---

### TASK-057-003: Status view wiring — F1 toggle, dim background, close on Esc

**Description:** As a user, I want pressing `F1` in the status view to open the shortcut help popup floating over the dimmed current view, and pressing `F1` or `Esc` to close it.

**Token Estimate:** ~35k tokens
**Predecessors:** TASK-057-001, TASK-057-002
**Successors:** none
**Parallel:** no

**Acceptance Criteria:**
- [ ] `statusModel` has a new field `showHelp bool`
- [ ] In `statusModel.Update()`, before all other key handling: if `key == "f1"` toggle `m.showHelp`; if `m.showHelp` is true, consume all other keys (return without further processing) except `"esc"` which sets `m.showHelp = false`
- [ ] In `statusModel.View()`, after building the normal view string `bg`:
  - If `m.showHelp` is false: return `bg` as normal (no change to existing render path)
  - If `m.showHelp` is true:
    1. Apply `lipgloss.NewStyle().Faint(true).Render(bg)` to dim the background
    2. Call `buildHelpModal(m.width, m.height)` to produce the modal string
    3. Return `styles.OverlayCenter(dimmedBg, modal, m.width, m.height)`
- [ ] `showHelp` is reset to `false` whenever `statusModel` navigates away (e.g. on `navigateBackMsg` or screen switch), so the popup does not persist across navigation
- [ ] The status footer adds `F1: help` to its hint text; existing hints that are now covered by the popup can be shortened
- [ ] `go vet ./...` and `go test ./...` pass

---

## Task Dependency Graph

```
TASK-057-001 ──┬──→ TASK-057-003
TASK-057-002 ──┘
```

| Task | Estimate | Predecessors | Parallel | Model |
|---|---|---|---|---|
| TASK-057-001 | ~35k | none | yes (with 002) | — |
| TASK-057-002 | ~50k | none | yes (with 001) | opus |
| TASK-057-003 | ~35k | 001, 002 | no | — |

**Total estimated tokens:** ~120k

## Functional Requirements

- FR-1: Pressing `F1` in the status view opens the help popup; pressing `F1` again or `Esc` closes it.
- FR-2: While the popup is open, all keys except `F1` and `Esc` are consumed — no navigation, no accidental actions.
- FR-3: The background view is rendered normally and then dimmed (`Faint` style) before compositing.
- FR-4: The modal box is centered in the terminal.
- FR-5: The modal lists all status view shortcuts grouped by category with aligned two-column formatting.
- FR-6: The `OverlayCenter` function correctly preserves ANSI escape sequences in both the background and the modal string.
- FR-7: The footer displays a `F1: help` hint at all times in the status view.

## Non-Goals

- Help popups for other views (menu, config, repos) — status view only in this iteration.
- Scrolling within the help popup — content is capped to fit; overflow is top-truncated.
- Mouse support for closing the popup.
- Animated open/close transitions.

## Technical Considerations

- **ANSI column slicing:** `charmbracelet/x/ansi` (v0.11.6, already a dependency) provides `ansi.Truncate(s, width, tail)` for left slices and may provide `ansi.Cut` or similar for right slices. If only `Truncate` is available, the right flank of each bg line can be extracted by stripping and re-measuring. Check the available API in `go doc github.com/charmbracelet/x/ansi` before implementing. An alternative: use `lipgloss`'s `Width()` measurement only for positioning, and reconstruct visible content via rune iteration with ANSI state tracking.
- **Faint on pre-styled strings:** Applying `lipgloss.NewStyle().Faint(true).Render(bg)` wraps the entire background in a single faint ANSI attribute. This works correctly in most terminals because the faint attribute is inherited until reset, and the existing reset sequences in `bg` will locally override it. Test in the target terminal to confirm visual result.
- **`buildHelpModal` location:** Place in a new file `cmd/status_help.go` to keep the 500-line-per-file rule and separate content from wiring.
- **Modal width computation:** `lipgloss.Width(fgLine)` gives the visual width of the widest modal line; use this as `fgWidth` in `OverlayCenter` rather than a hardcoded constant.
- **Reset on navigate:** The `showHelp = false` reset can be placed in the `navigateBackMsg` handler or in a shared `resetOverlayState()` helper if the pattern is reused later.

## Success Metrics

- `F1` opens a readable popup listing all shortcuts, over a visibly dimmed (but recognisable) status view background.
- The footer is less crowded — `F1: help` replaces several inline hints.
- `Esc` or `F1` closes the popup immediately with no side effects.

## Open Questions

_(none)_
