# Feature 007: Split Config TUI into Project / Global Tabs

## Introduction

The `maggus config` screen currently renders all settings in one long scrollable list with a "Project" section and a "Global" section separated by a visual header. This feature splits them into two distinct tabs — **Project** and **Global** — so the user sees only the relevant settings at a time. Tabs are rendered with a border/underline-style indicator and can be switched via left/right arrow (when on the tab bar) or the number keys `1` and `2` from anywhere.

### Architecture Context

- **Components involved:** `cmd/config.go` only — `configModel`, `newConfigModel`, `Update`, `View`, `buildConfig`, `executeAction`
- **No new packages required** — pure TUI refactor within the existing Bubble Tea model
- **Pattern:** Follows the existing `configRow` / cursor model; adds an `activeTab int` field and splits the single `rows []configRow` into `projectRows` and `globalRows`

## Goals

- Project and Global settings are never shown on screen at the same time
- Tab switching is fast and keyboard-driven (no mouse required)
- Left/right arrows retain their existing meaning (cycle option values) when focused on a setting row; they switch tabs only when focused on the tab bar row
- `1` and `2` always jump directly to the respective tab regardless of cursor position
- Active tab is visually distinct via a border/underline style on the tab bar
- All existing config functionality (save, edit in editor, cycle options) is preserved

## Tasks

### TASK-007-001: Refactor `configModel` to use per-tab row slices

**Description:** As a developer, I want the config model to store Project and Global rows separately so that each tab can render and operate on its own independent set of rows.

**Token Estimate:** ~50k tokens
**Predecessors:** none
**Successors:** TASK-007-002
**Parallel:** no

**Acceptance Criteria:**
- [ ] `configModel` in `src/cmd/config.go` gains an `activeTab int` field (0 = Project, 1 = Global)
- [ ] The single `rows []configRow` field is replaced by `projectRows []configRow` and `globalRows []configRow`
- [ ] A helper method `func (m *configModel) activeRows() *[]configRow` returns a pointer to the currently active slice so all cursor operations work on the right set without duplicating logic
- [ ] `newConfigModel` populates `projectRows` with all current Project-section rows (Agent, Model, Worktree, Auto-branch, Check sync, Protected branches, Sound, On task complete, On run complete, On error, On complete Feature, On complete Bug, Save project config button, Edit project file button) and `globalRows` with all Global-section rows (Auto-update, Save global config button, Edit global file button)
- [ ] The `section` header field is removed from the first row of each slice — the tab bar replaces section headers, so no `section:` string is needed on any row
- [ ] `optionByLabel` searches only the active tab's rows (or both, in order) — must still find any label regardless of which tab is active, so `buildConfig` works correctly when called from either tab's save action
- [ ] `buildConfig` is unchanged in logic — it still reads all option rows from both slices to construct the full `config.Config`
- [ ] `cursor` is reset to 0 whenever the active tab changes
- [ ] `globalAutoUpdateIdx` is updated to index into `globalRows` instead of `rows`
- [ ] All existing `config_test.go` tests pass
- [ ] `go build ./...` passes

---

### TASK-007-002: Render tab bar and update key handling

**Description:** As a user, I want to see a clearly labelled tab bar at the top of the config screen and be able to switch between Project and Global using left/right arrows (on the tab bar) or the `1`/`2` keys from anywhere, so that navigation is fast and discoverable.

**Token Estimate:** ~55k tokens
**Predecessors:** TASK-007-001
**Successors:** none
**Parallel:** no

**Acceptance Criteria:**

**Tab bar rendering:**
- [ ] The `View()` method renders a tab bar above the row list. Example layout (exact styling may vary as long as it is clear):
  ```
  ┌─────────┐  Global
  │ Project │
  └─────────┘
  ```
  Active tab has a visible border (lipgloss `Border` or `Underline` style); inactive tab is muted. The border style must be consistent with the existing `styles` package colours.
- [ ] The tab bar is rendered as a special first visual element — it is NOT a `configRow` entry in the slice; it is hardcoded in `View()` above the row list
- [ ] When the cursor is at position `-1` or a dedicated "tab bar focused" state, the tab bar is highlighted to show keyboard focus. Use a separate `tabFocused bool` field on the model, set to `true` when the user navigates up past row 0

**Key handling — switching tabs:**
- [ ] Pressing `1` from anywhere sets `activeTab = 0` (Project), resets cursor to 0, clears `tabFocused`
- [ ] Pressing `2` from anywhere sets `activeTab = 1` (Global), resets cursor to 0, clears `tabFocused`
- [ ] When `tabFocused` is `true`, pressing `left` or `right` (or `h`/`l`) switches to the other tab (wraps: Project ↔ Global), resets cursor to 0, keeps `tabFocused = true`
- [ ] When `tabFocused` is `false` and cursor is at row 0, pressing `up` (or `k`) sets `tabFocused = true` instead of wrapping to the last row
- [ ] When `tabFocused` is `true`, pressing `down` (or `j`) or `enter` clears `tabFocused` and sets cursor to 0 (first row of the active tab)

**Key handling — within a tab:**
- [ ] When `tabFocused` is `false`, `up`/`down` navigate rows within the active tab only (no cross-tab wrapping — hitting the bottom of Global does not jump to Project)
- [ ] When `tabFocused` is `false`, `left`/`right` on an option row still cycles values exactly as before
- [ ] `enter` on an action button still executes the action (save / edit)

**Footer:**
- [ ] The footer hint bar is updated to include tab-switching hints: `1/2: switch tab | up/down: navigate | left/right: change value | enter: select | q/esc: exit`

**Existing behaviour preserved:**
- [ ] Saving project config still writes `.maggus/config.yml` with all project settings
- [ ] Saving global config still writes `~/.maggus/config.yml` with global settings
- [ ] Status messages ("Saved project config", error text) still appear after save/edit actions
- [ ] Window resize still works correctly
- [ ] `go build ./...` passes
- [ ] `go test ./cmd` passes (all existing config tests pass)

---

## Task Dependency Graph

```
TASK-007-001 ──→ TASK-007-002
```

| Task | Estimate | Predecessors | Parallel | Model |
|------|----------|--------------|----------|-------|
| TASK-007-001 | ~50k | none | no | — |
| TASK-007-002 | ~55k | 001 | no | — |

**Total estimated tokens:** ~105k

## Functional Requirements

- FR-1: The config screen must display a tab bar at the top with two tabs: "Project" and "Global"
- FR-2: Only the rows belonging to the active tab are visible and navigable at any time
- FR-3: The active tab must be visually distinguished from the inactive tab using a border or underline style
- FR-4: Pressing `1` must always activate the Project tab; pressing `2` must always activate the Global tab
- FR-5: When the tab bar has focus, pressing left or right must switch to the other tab
- FR-6: Left/right arrows on a setting row must still cycle option values (unchanged behaviour)
- FR-7: Each tab retains its own save and edit-in-editor action buttons at the bottom of its row list
- FR-8: The footer help text must document tab switching keys
- FR-9: Switching tabs resets the cursor to the first row of the newly active tab

## Non-Goals

- No mouse support for tab clicking
- No more than two tabs — this is Project and Global only
- No animation or slide transition between tabs
- No persistence of cursor position per tab across tab switches (cursor always resets to 0)
- No keyboard shortcut other than `1`/`2` and left/right-on-tabbar for switching

## Design Considerations

- The tab bar should use the existing `styles` package for colours: active tab border in `styles.Primary`, inactive tab label in `styles.Muted`
- Lipgloss has built-in border styles (`lipgloss.RoundedBorder()`, `lipgloss.NormalBorder()`) — use whichever fits the existing visual style of the TUI
- The `styles.FullScreenColor` / `styles.Box` layout used in the existing `View()` must still wrap the full output correctly after the tab bar is prepended

## Technical Considerations

- `optionByLabel` currently searches `m.rows` linearly. After the split, it must search both `projectRows` and `globalRows` so that `buildConfig` (which reads all option values) works regardless of which tab was active at save time
- The `globalAutoUpdateIdx` field is used in `saveGlobalConfig()` to find the Auto-update row. After the split it indexes into `globalRows` — verify the index is updated in `newConfigModel`
- `tabFocused bool` is a clean way to represent "cursor is above the first row" without using a special sentinel cursor value like `-1`, which would require range checks throughout the `Update` method
- The existing `config_test.go` tests likely call `newConfigModel` and inspect `m.rows` — these tests will need updating to use `m.projectRows` / `m.globalRows` after TASK-007-001

## Success Metrics

- Opening `maggus config` shows the Project tab active by default with all project settings visible
- Pressing `2` immediately shows only the Global tab (Auto-update + save/edit buttons)
- Pressing `1` switches back to Project
- Navigating up past the first row focuses the tab bar; left/right then switches tabs
- All settings saved via the TUI are correctly written to their respective config files
