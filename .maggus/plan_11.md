# Plan: TUI Overhaul — Polished Full-Screen Experience

## Introduction

Rethink the Maggus TUI to provide a polished, consistent visual experience from start to finish across all commands. Currently, only the `work` command uses the bubbletea alt-screen TUI — other commands (`status`, `list`, `blocked`) use raw `fmt.Printf` with ANSI escape codes, and the `work` command dumps raw git output to the terminal after the TUI exits. The goal is to make every command render through lipgloss-styled bubbletea views, eliminate all raw/ugly shell output, and add configurable sound notifications for key events.

## Goals

- Every Maggus command renders inside a bubbletea alt-screen TUI with lipgloss styling
- The `work` command shows a styled summary screen after completion and waits for a keypress before exiting (no raw git output ever leaks to the terminal)
- All git operations (push, branch creation) happen inside the TUI with status updates — never raw output
- Configurable sound notifications: task completion, run completion, and error (default off)
- Consistent visual language across all commands using shared lipgloss styles and components
- The 3-second startup countdown is removed — `work` starts immediately (Ctrl+C works anytime)

## User Stories

### TASK-001: Create shared TUI style package
**Description:** As a developer, I want a shared style package with reusable lipgloss styles, color palette, and layout helpers so that all commands have a consistent visual language.

**Acceptance Criteria:**
- [x] New package at `src/internal/tui/styles/styles.go`
- [x] Defines a color palette with named constants: `Primary` (cyan), `Success` (green), `Warning` (yellow), `Error` (red), `Muted` (gray), `Accent` (blue/purple)
- [x] Provides reusable lipgloss styles: `Title`, `Subtitle`, `Label`, `Value`, `Separator`, `Box`, `StatusBar`
- [x] `Title` style: bold, primary color, used for section headers
- [x] `Box` style: bordered box using lipgloss.Border for framing content sections
- [x] `Separator(width int) string` helper that renders a styled horizontal rule
- [x] `ProgressBar(done, total, width int) string` helper that renders a styled progress bar (replacing the duplicated implementations in status.go and tui.go)
- [x] `Truncate(text string, maxWidth int) string` helper (replacing the local `truncate` in tui.go)
- [x] Removes the raw ANSI escape code constants from `src/cmd/status.go` (`colorGreen`, `colorCyan`, etc.) — all styling goes through lipgloss
- [x] Unit tests for ProgressBar and Truncate helpers
- [x] Typecheck/lint passes (`go vet ./...`)

### TASK-002: Refactor `work` TUI — remove countdown, capture all output
**Description:** As a user, I want `maggus work` to start immediately (no 3-second countdown) and never show raw git output, so the experience is clean from start to finish.

**Acceptance Criteria:**
- [x] The 3-second pause/countdown in `src/cmd/work.go` (lines 166-178) is removed entirely
- [x] The startup banner that was printed with `fmt.Printf` before the TUI starts is moved inside the TUI as the initial view (rendered via bubbletea, not raw print)
- [x] Git push output (currently `push.Stdout = os.Stdout`, lines 344-351) is captured and displayed as a status update inside the TUI instead of raw terminal output
- [x] Branch creation messages (from `gitbranch.EnsureFeatureBranch`) are displayed as TUI status messages, not raw `fmt.Println`
- [x] All `fmt.Printf`/`fmt.Println` calls in the work command are replaced with TUI messages or removed
- [x] The TUI starts immediately when `maggus work` is invoked and stays in alt-screen until the user dismisses the summary
- [x] Typecheck/lint passes (`go vet ./...`)

### TASK-003: Add post-completion summary screen to `work` TUI
**Description:** As a user, I want to see a styled summary screen when work completes, showing what was accomplished, and press a key to exit — so the terminal stays clean.

**Acceptance Criteria:**
- [x] When all iterations complete (or are interrupted), the TUI transitions to a "summary" view instead of quitting
- [x] The summary view shows: run ID, branch, model, total elapsed time, tasks completed vs. total, commit range (start..end), and a list of commits made
- [x] If there are remaining incomplete tasks, the summary shows the count and first few task titles
- [x] The summary view shows push status: "Pushed to origin/branch-name" or "Push failed: reason"
- [x] The summary is rendered inside a lipgloss box with the shared styles from TASK-001
- [x] At the bottom: "Press any key to exit" prompt
- [x] Pressing any key (or Ctrl+C) exits the TUI and returns to the shell cleanly (no trailing output)
- [x] The git push happens in the background while the summary is displayed, with a spinner/status update
- [x] Typecheck/lint passes (`go vet ./...`)

### TASK-004: Restyle `status` command with lipgloss TUI
**Description:** As a user, I want `maggus status` to render a polished, styled view using lipgloss instead of raw ANSI codes.

**Acceptance Criteria:**
- [x] `src/cmd/status.go` is refactored to use a bubbletea model with alt-screen
- [x] The status view uses shared styles from `src/internal/tui/styles/`
- [x] Plan progress bars use the shared `ProgressBar` helper
- [x] Task list uses styled icons: checkmark (success color) for complete, warning icon (error color) for blocked, circle (muted) for pending, arrow (primary) for next-up
- [x] The view is framed in a lipgloss box
- [x] The `--plain` flag still works — when set, skips the TUI and prints plain-text output (for scripting/piping)
- [x] Pressing `q`, `Esc`, or any key exits the view
- [x] Typecheck/lint passes (`go vet ./...`)

### TASK-005: Restyle `list` command with lipgloss TUI
**Description:** As a user, I want `maggus list` to render a styled task list using lipgloss instead of raw ANSI codes.

**Acceptance Criteria:**
- [x] `src/cmd/list.go` is refactored to use a bubbletea model with alt-screen
- [x] The task list uses shared styles from `src/internal/tui/styles/`
- [x] The first task (next up) is highlighted with primary color
- [x] Each task shows its number, ID, and title in a clean layout
- [x] Header shows count information styled with `Title` style
- [x] The `--plain` flag still works — when set, skips the TUI and prints plain-text output
- [x] Pressing `q`, `Esc`, or any key exits the view
- [x] Typecheck/lint passes (`go vet ./...`)

### TASK-006: Restyle `blocked` command with lipgloss TUI
**Description:** As a user, I want `maggus blocked` to render its interactive wizard with lipgloss styling instead of raw ANSI codes.

**Acceptance Criteria:**
- [x] `src/cmd/blocked.go` is refactored to use lipgloss styles from the shared package instead of raw ANSI escape codes
- [x] The `actionPickerModel` view uses lipgloss styles for the menu items (green for unblock, yellow for resolve, red for abort)
- [x] The task detail view (`renderBlockedTaskDetail`) uses lipgloss styles and the shared `Separator` helper
- [x] The summary at the end uses a styled box
- [x] The raw `colorGreen`, `colorRed`, etc. constants are no longer used (can be removed if status.go also stops using them)
- [x] Interactive behavior (up/down/enter navigation) is unchanged
- [x] Typecheck/lint passes (`go vet ./...`)

### TASK-007: Add configurable sound notifications
**Description:** As a user, I want optional sound notifications when tasks complete, runs finish, or errors occur, so I can walk away and be alerted.

**Acceptance Criteria:**
- [x] New package at `src/internal/notify/notify.go`
- [x] `Config` struct in `src/internal/config/config.go` gains a `Notifications` section:
  ```yaml
  notifications:
    sound: false           # master toggle (default: false)
    on_task_complete: true  # play sound after each task (default: true when sound is enabled)
    on_run_complete: true   # play sound when run finishes (default: true when sound is enabled)
    on_error: true          # play sound on errors (default: true when sound is enabled)
  ```
- [x] The `notify` package provides `PlayTaskComplete()`, `PlayRunComplete()`, and `PlayError()` functions
- [x] Sound is played by writing a BEL character (`\a`) to the terminal — this is cross-platform (Windows, macOS, Linux) and requires no external dependencies or sound files
- [x] Each function checks the config before playing — if disabled, it's a no-op
- [x] The work loop calls `PlayTaskComplete()` after each successful commit, `PlayRunComplete()` when the run finishes, and `PlayError()` on task failures
- [x] When `sound: false` (the default), no sound is ever played
- [x] Unit tests verify that notification functions respect the config toggle
- [x] Typecheck/lint passes (`go vet ./...`)

### TASK-008: Parse and track token usage from Claude Code output
**Description:** As a user, I want the work TUI and summary screen to show token usage (input/output tokens) per task and cumulative for the run, so I can monitor how much each session costs.

**Acceptance Criteria:**
- [ ] The `streamEvent` struct in `src/internal/runner/runner.go` is extended to parse a `usage` field from `result` events. Claude Code's stream-json `result` event includes `"usage": {"input_tokens": N, "output_tokens": N}` — this data is currently ignored
- [ ] New message type `UsageMsg` with `InputTokens` and `OutputTokens` fields, sent to the TUI when a `result` event contains usage data
- [ ] The TUI model in `runner/tui.go` tracks per-iteration usage and cumulative usage across all iterations
- [ ] The work TUI displays cumulative token usage in the header or status area (e.g., "Tokens: 12.3k in / 8.1k out")
- [ ] Token counts are formatted with `k` suffix for thousands (e.g., `1500` → `1.5k`, `234` → `234`)
- [ ] The post-completion summary screen (TASK-003) displays: total input tokens, total output tokens, and per-task breakdown
- [ ] If Claude Code does not return usage data (e.g., older CLI version), the TUI gracefully shows "N/A" instead of zeros
- [ ] Unit tests for token formatting helper and usage accumulation
- [ ] Typecheck/lint passes (`go vet ./...`)

### TASK-009: Update existing `work` TUI styles to use shared package
**Description:** As a developer, I want the existing `runner/tui.go` to use the shared style package so there's one source of truth for visual styling.

**Acceptance Criteria:**
- [ ] `src/internal/runner/tui.go` imports and uses styles from `src/internal/tui/styles/`
- [ ] The local style variables (`boldStyle`, `statusStyle`, `greenStyle`, `redStyle`, `cyanStyle`, `blueStyle`, `grayStyle`) are removed from `tui.go`
- [ ] The `renderHeader()` and `renderView()` methods use shared styles
- [ ] The progress bar in `renderHeader()` uses the shared `ProgressBar` helper
- [ ] The local `truncate` function is replaced with the shared `Truncate` helper
- [ ] Visual output is unchanged or improved — no regression in the work TUI appearance
- [ ] Typecheck/lint passes (`go vet ./...`)

## Functional Requirements

- FR-1: All commands (`work`, `status`, `list`, `blocked`) must render through lipgloss-styled bubbletea views
- FR-2: The `--plain` flag on `status` and `list` must bypass the TUI and output plain text for scripting
- FR-3: The `work` command must never print raw output to the terminal — all git operations, warnings, and status messages are rendered inside the TUI
- FR-4: The `work` command must show a summary screen after completion and wait for a keypress before exiting
- FR-5: The 3-second startup countdown is removed — `work` starts immediately
- FR-6: Sound notifications must default to off and be configurable via `.maggus/config.yml`
- FR-7: Sound must use BEL character (`\a`) — no external dependencies or sound files
- FR-8: All commands must share a single style package for consistent visual language
- FR-9: The `blocked` command's interactive navigation (up/down/enter) must continue to work identically
- FR-10: Git push must happen inside the TUI with progress indication, never raw output
- FR-11: Token usage (input/output) must be parsed from Claude Code's stream-json `result` events and displayed in both the live TUI and the post-completion summary

## Non-Goals

- No custom sound files or audio library integration — BEL character only
- No mouse support in the TUI
- No responsive/adaptive layouts (simple width-aware is enough)
- No theme customization by the user (single built-in theme)
- No animated transitions between views (keep it snappy)
- No color scheme configuration — the built-in palette is the palette

## Design Considerations

- **Color palette:** Use the Charm ecosystem's approach — a small set of named colors (Primary/Success/Warning/Error/Muted/Accent) mapped to ANSI 256 colors for broad terminal compatibility
- **Layout pattern:** Each command's TUI should follow: Header (command name + context) → Content → Footer (keybindings hint). Consistent structure makes navigation intuitive
- **Box borders:** Use lipgloss's `RoundedBorder` for content sections — it looks modern and works in most terminals
- **The `blocked` command** is already partially using bubbletea (for the action picker). The refactor should unify its raw ANSI output with the picker's bubbletea model into a single cohesive TUI

## Technical Considerations

- Bubbletea and lipgloss are already dependencies (`go.mod` has both). No new dependencies needed for styling
- The `bubbles` package (charmbracelet/bubbles) provides ready-made components like `viewport`, `table`, `spinner`, `progress` — consider adding it as a dependency if it simplifies implementation. But evaluate whether the overhead is worth it vs. custom lipgloss rendering
- The BEL character (`\a`) for sound works on Windows Terminal, iTerm2, Terminal.app, and most Linux terminal emulators. On some terminals it may flash the screen instead of playing a sound — this is acceptable
- For the summary screen, the TUI should transition from the "working" state to a "summary" state within the same bubbletea program — no need to exit and restart
- Git push should run in a goroutine while the summary screen is showing, sending a message to the TUI when complete. This keeps the UI responsive
- The `--plain` flag is important for CI and scripting. When set, commands should write to stdout without any ANSI codes or alt-screen, exactly as they do now (minus the raw escape codes — use plain text equivalents)

## Success Metrics

- Running any Maggus command produces a visually polished, consistent experience
- After `maggus work` completes, the terminal is clean — no raw git output, no garbled text
- Sound notifications play when enabled and are silent when disabled
- The `--plain` flag produces clean, parseable output for scripting
- All existing functionality works identically — only the visual presentation changes

## Open Questions

- Should `maggus clean` and `maggus release` (from plan_10) also get TUI views, or should those be added later as a follow-up?
- Should the summary screen show a diff stat (files changed, insertions, deletions)?
- Should there be a `--no-tui` global flag that forces plain output for all commands?
