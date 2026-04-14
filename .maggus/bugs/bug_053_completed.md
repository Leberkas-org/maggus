<!-- maggus-id: 90d18d33-e50a-4c0b-a691-394d5d816a6c -->
# Bug: b shortcut doesn't unblock, footer wraps, b/e missing from F1, editor blocks

## Summary

Four UI issues in the status view: (1) Pressing `b` on a blocked task should immediately unblock all its blocked criteria — instead it opens a navigation-based criteria mode that is broken and unwanted. (2) The footer status bar wraps to a second line when many hints are visible. (3) The `b` and `e` shortcuts are absent from the F1 help popup. (4) Pressing `e` suspends the TUI while the editor is open — it should open in the background without blocking.

## Steps to Reproduce

### Unblocking
1. Have a feature with a blocked task (`BLOCKED:` criterion)
2. Run `maggus status`
3. Navigate to the blocked task in the tree
4. Press `b`
5. Observe: a criteria navigation mode opens instead of immediately unblocking

### Footer wrapping
1. Run `maggus status` with a blocked task selected
2. Observe: the status bar footer wraps to two lines, breaking the layout

### F1 popup missing b/e
1. Press `F1` in status view
2. Observe: `b` (unblock) and `e` (edit file) are not listed under Actions

### Editor blocks TUI
1. Select a plan file in the tree
2. Press `e`
3. Observe: TUI suspends until the editor closes; the editor should open without pausing the status view

## Expected Behavior

- `b` immediately removes the `BLOCKED:` prefix from all blocked criteria on the selected task and reloads plans; no navigation or confirmation required
- Footer fits on one line
- F1 popup lists `b` and `e` under Actions
- `e` opens the file in the background; status view stays live

## Root Cause

### Unblocking
The `b` key handler (`src/cmd/status_update.go:884`) opens a detail view and enters a navigation-based criteria mode. This mode is complex, broken (tree navigation keys intercept before the component gets them), and not the desired UX. The correct fix is to remove all criteria mode code and make `b` directly call `store.UnblockCriterion` for every blocked criterion on the selected task, then reload plans.

Code to remove:
- `b` key handler in `status_update.go` (lines ~884–907) — replace with a direct unblock-all loop
- `updateCriteriaMode` and `updateActionPicker` functions in `src/cmd/tasklist.go`
- `tab`/`b` handler inside the detail view in `tasklist.go` that enters criteria mode
- `criteriaMode`, `criteriaCursor`, `blockedIndices`, `showActionPicker`, `actionCursor`, `noBlockedMsg` fields from `detailState` in `src/cmd/detail.go`
- `initCriteriaMode`, `exitCriteriaMode` methods in `detail.go`
- `performAction` method and all `criteriaAction` types/constants in `detail.go`
- Criteria mode rendering block in `renderDetailContent` (`detail.go`)
- `renderInlineActionPicker` function in `detail.go`
- Criteria mode footer hints in `statusSplitFooter` (`status_view.go`)

### Footer wrapping
`statusSplitFooter()` (`src/cmd/status_view.go:172`) assembles up to ~10 hint segments separated by two spaces. At 80–120 columns with all hints visible the string exceeds the terminal width and wraps.

### b/e missing from F1
`statusHelpSections` in `src/cmd/status_help.go:24` does not include `b` or `e` in the Actions section.

### Editor blocks
The `e` key handler (`src/cmd/status_update.go:908`) returns an `execProcessMsg`, which the app router handles via `tea.ExecProcess` — a full TUI suspend. Should use `cmd.Start()` (fire-and-forget) instead.

## User Stories

### BUG-053-001: Replace criteria mode with direct unblock-all on b

**Description:** As a user, I want pressing `b` on a blocked task to immediately remove all `BLOCKED:` prefixes so I can unblock tasks in one keystroke with no navigation.

**Acceptance Criteria:**
- [x] All criteria mode code is removed: `updateCriteriaMode`, `updateActionPicker`, `initCriteriaMode`, `exitCriteriaMode`, `performAction`, all `criteriaAction` types, `renderInlineActionPicker`, and criteria mode fields in `detailState`
- [x] The `b` key handler in `updateList` calls `store.UnblockCriterion` for every criterion where `c.Blocked == true` on the selected task
- [x] After unblocking, plans are reloaded and the tree cursor is preserved so the task row stays selected
- [x] `b` is a no-op when the selected task has no blocked criteria
- [x] The footer hint `b: unblock` still only shows when the selected task is blocked
- [x] The criteria mode footer hints (`↑/↓: navigate/scroll · enter: action · tab: scroll mode · q: back`) are removed from `statusSplitFooter`
- [x] No regression in the regular detail view (opened via `enter`) or the `tab` key in the detail view
- [x] `go vet ./...` and `go test ./...` pass

### BUG-053-002: Shorten the status bar footer to prevent wrapping

**Description:** As a user, I want the footer hints to fit on one line so the layout stays clean.

**Acceptance Criteria:**
- [x] The footer bar never wraps to a second line at 80-column terminal width with all hints visible
- [x] Hints are either abbreviated or only the most contextually relevant ones shown (e.g. omit obvious/redundant hints when screen space is tight)
- [x] `go vet ./...` and `go test ./...` pass

### BUG-053-003: Add b and e to the F1 help popup

**Description:** As a user, I want the F1 help popup to list all available shortcuts including `b` (unblock) and `e` (edit file).

**Acceptance Criteria:**
- [x] `b` — "Unblock all blocked criteria" appears in the Actions section of `statusHelpSections` (`src/cmd/status_help.go`)
- [x] `e` — "Open plan file in editor" appears in the Actions section
- [x] Both entries are visible in the rendered F1 popup at normal terminal sizes
- [x] `go vet ./...` and `go test ./...` pass

### BUG-053-005: Remove broken background dimming when F1 help popup is open

**Description:** As a user, I want the F1 help popup to overlay the status view cleanly without a broken dim effect on the background.

**Acceptance Criteria:**
- [x] In `viewStatus()` (`src/cmd/status_view.go`), remove the `lipgloss.NewStyle().Faint(true).Render(bg)` call — pass `bg` directly to `styles.OverlayCenter` instead of `dimmedBg`
- [x] The status view background is fully visible (not faint/dimmed) when the F1 popup is open
- [x] `go vet ./...` and `go test ./...` pass

### BUG-053-004: Open editor non-blocking so TUI stays live

**Description:** As a user, I want pressing `e` to open my editor without suspending the status view.

**Acceptance Criteria:**
- [x] The `e` key handler in `src/cmd/status_update.go` launches the editor with `cmd.Start()` (non-blocking) instead of returning an `execProcessMsg`
- [x] The status TUI remains interactive while the editor is open
- [x] No zombie processes or resource leaks (fire-and-forget is acceptable; the editor process owns its own lifecycle)
- [x] `go vet ./...` and `go test ./...` pass
