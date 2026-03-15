# Plan: Overhaul Work View with Bordered Layout and Tool Detail Panel

## Introduction

The work view (`maggus work`) currently renders as flat, unstyled text — no border, no structure. All other views (status, summary) already use the shared `FullScreenLeft` bordered box layout. This plan brings the work view in line with the rest of the UI and adds a toggleable right-side detail panel showing a structured, scrollable log of tool invocations. The existing left-side summary (status, tools list, tokens, elapsed) stays as-is but moves inside the bordered box. Commits move into a tab system alongside the main progress view.

## Goals

- Wrap the work view in the same bordered `FullScreenLeft` box used by status/summary
- Add a toggleable right-side detail panel (Alt+I) showing a rolling structured log per tool invocation
- Keep the existing left-side summary layout (status, tool list, tokens, model, elapsed) intact
- Move the commits section into a tab bar (Progress | Commits)
- Make the tool detail panel scrollable independently while execution continues
- Maintain all current functionality (Ctrl+C interrupt, spinner, progress bar, token tracking)

## User Stories

### TASK-001: Wrap work view in bordered full-screen box
**Description:** As a user, I want the work view to render inside a bordered box so that it matches the visual style of the status and summary views.

**Acceptance Criteria:**
- [x] The `renderView()` method uses `styles.FullScreenLeft` (or equivalent) to wrap content in a rounded-border box
- [x] The banner view (`renderBannerView`) also renders inside the bordered box
- [x] The header (version, fingerprint, progress bar) renders inside the box as a top section
- [x] The box fills the terminal with the standard 2-char margin on each side
- [x] A footer bar at the bottom shows available keybindings (e.g., `alt+i detail · ctrl+c stop`)
- [x] Window resize (`tea.WindowSizeMsg`) correctly updates both width and height, and the box reflows
- [x] Typecheck and vet pass (`go vet ./...`)

### TASK-002: Add tab bar with Progress and Commits tabs
**Description:** As a user, I want a tab bar in the work view so that I can switch between the main progress view and a commits view without cluttering the main display.

**Acceptance Criteria:**
- [x] A horizontal tab bar renders below the header/progress bar area, showing "Progress" and "Commits" tabs
- [x] The currently active tab is bold + primary color; inactive tabs are muted
- [x] Tabs are separated by `│` characters, matching the status view convention
- [x] The "Progress" tab shows the existing status/tools/tokens/elapsed content
- [x] The "Commits" tab shows the list of recent commits (moved from the bottom of the progress view)
- [x] The commits tab shows a "(N)" count badge next to the tab label when commits exist
- [x] Tab switching works via number keys (1/2) or left/right arrow keys
- [x] The tab state persists across ticker updates (does not reset on each render)
- [x] Typecheck and vet pass (`go vet ./...`)

### TASK-003: Add tool detail panel (right side, toggleable with Alt+I)
**Description:** As a user, I want to toggle a detail panel on the right side of the work view that shows a rolling structured log of each tool invocation, so I can see what Claude is doing in more detail.

**Acceptance Criteria:**
- [x] Pressing Alt+I toggles a right-side detail panel on/off
- [x] When the detail panel is hidden, the left panel uses the full box width (current behavior)
- [x] When the detail panel is visible, the layout splits into left (summary, ~40% width) and right (detail, ~60% width) separated by a vertical `│` divider
- [x] The detail panel shows a rolling log where each tool invocation gets a structured section:
  - Header line: tool type icon + tool description + timestamp (e.g., `▶ Read: src/cmd/work.go  12:34:05`)
  - Indented detail lines showing parameters (file path, command, pattern, etc.) when available
  - A subtle separator (`·····`) between tool entries
- [x] The detail panel auto-scrolls to the latest entry as new tools are invoked
- [x] The footer bar updates to reflect the current keybindings (shows `alt+i hide detail` when panel is open)
- [x] Typecheck and vet pass (`go vet ./...`)

### TASK-004: Enrich tool messages with structured metadata
**Description:** As a developer, I want tool messages to carry structured metadata (tool type, parameters, timestamp) so that the detail panel can render rich per-tool sections.

**Acceptance Criteria:**
- [ ] The `agent.ToolMsg` struct is extended with fields: `Type` (string, e.g. "Read", "Bash", "Grep"), `Params` (map[string]string for key details), and `Timestamp` (time.Time)
- [ ] The `DescribeToolUse()` function (or the calling code in `claude.go`) populates these new fields when parsing tool_use blocks from the streaming JSON
- [ ] Existing `Description` field remains for backward compatibility with the left-side tool list
- [ ] The TUI model stores the enriched tool messages (not just description strings) for the detail panel to consume
- [ ] The `toolHistory` in `TUIModel` stores full `agent.ToolMsg` structs (or a new `ToolEntry` struct) instead of plain strings
- [ ] The existing left-side tool list still renders from `Description` as before
- [ ] Unit tests for `DescribeToolUse` still pass
- [ ] Typecheck and vet pass (`go vet ./...`)

### TASK-005: Make detail panel scrollable
**Description:** As a user, I want to scroll through the tool detail panel history while execution continues, so I can review earlier tool invocations.

**Acceptance Criteria:**
- [ ] When the detail panel is visible, Up/Down arrow keys scroll the detail panel viewport
- [ ] Home/End keys jump to the top/bottom of the detail log
- [ ] When scrolled to the bottom (default), new tool entries auto-scroll the view
- [ ] When the user has scrolled up, auto-scroll is paused — new entries appear but the viewport stays in place
- [ ] A scroll indicator shows position (e.g., `[3-15 of 42]`) at the top-right of the detail panel
- [ ] Pressing `End` or scrolling to the bottom re-enables auto-scroll
- [ ] Arrow keys only affect the detail panel when it is visible; otherwise they pass through to tab switching
- [ ] Typecheck and vet pass (`go vet ./...`)

### TASK-006: Integration testing and polish
**Description:** As a developer, I want to verify the full work view renders correctly across all states (banner, progress, summary) and that all interactions work together.

**Acceptance Criteria:**
- [ ] The banner view renders inside the bordered box with correct padding
- [ ] Switching tabs while the detail panel is open works correctly (tabs affect left panel only)
- [ ] The summary view continues to render in its own bordered box (not affected by work view changes)
- [ ] Ctrl+C still works in all states (banner, progress with/without detail panel, summary)
- [ ] The detail panel toggle state resets between iterations (or persists — pick one and be consistent)
- [ ] No visual glitches when terminal is narrower than 80 columns (graceful degradation)
- [ ] `go build ./...` succeeds
- [ ] `go vet ./...` passes
- [ ] `go test ./...` passes

## Functional Requirements

- FR-1: The work view must render inside a `styles.FullScreenLeft` bordered box, matching the status view's visual style
- FR-2: A tab bar must allow switching between "Progress" and "Commits" sections using number keys or arrow keys
- FR-3: Alt+I must toggle a right-side detail panel that shows structured per-tool log entries
- FR-4: Each tool entry in the detail panel must show: tool type, description, timestamp, and available parameters
- FR-5: The detail panel must be independently scrollable with Up/Down/Home/End keys
- FR-6: Auto-scroll must be active by default and pause when the user scrolls up
- FR-7: The footer must show context-sensitive keybindings based on current state
- FR-8: All existing functionality (Ctrl+C, spinner, progress bar, token tracking, commit tracking) must continue to work
- FR-9: Terminal resize must correctly reflow the bordered layout including the split panel

## Non-Goals

- No changes to the summary view (post-completion screen) — it already has a bordered box
- No changes to the status command view
- No mouse interaction support
- No persistent detail panel state across `maggus work` invocations
- No filtering or searching within the detail panel
- No changes to how the agent/runner streams or parses Claude Code output (only how it's displayed)

## Technical Considerations

- The `TUIModel` struct in `runner/tui.go` is the single bubbletea model — all changes go here
- Use `lipgloss.JoinHorizontal` for the left/right panel split
- The detail panel needs its own scroll offset tracking (similar to how status view handles its scrollable task list)
- `agent.ToolMsg` changes in `agent/messages.go` must not break existing consumers
- Tab state and detail panel visibility are new fields on `TUIModel`
- The `renderView()` method will need to be split into sub-methods: `renderLeftPanel()`, `renderDetailPanel()`, `renderTabBar()`, `renderFooter()`
- Consider the `charmbracelet/viewport` component for the scrollable detail panel, or implement simple offset-based scrolling as the status view does

## Success Metrics

- The work view visually matches the bordered style of the status view
- Tool detail panel provides meaningful at-a-glance insight into what Claude is doing
- Existing users see an improved but familiar layout — no learning curve for the left-side summary
- No regressions in existing work loop functionality

## Open Questions

- Should the detail panel width ratio (40/60) be configurable or fixed?
- Should the detail panel remember its scroll position when toggled off and back on?
- Should there be a visual indicator on the left-side tool list showing which tool is "selected" in the detail panel?
