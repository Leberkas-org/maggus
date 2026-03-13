# Plan: Interactive Main Menu TUI

## Introduction

When `maggus` is invoked without any subcommand in an interactive terminal, it currently shows Cobra's default help text. This plan adds an interactive main menu that lists all available commands with descriptions, shows a quick plan summary in the header, and lets the user select a command with arrow keys. For commands with common options (like `--all`, `--plain`, `--count`), a sub-menu appears to configure those before launching.

## Goals

- Provide a discoverable entry point for new and returning users
- Show at-a-glance project status (plans, tasks, blocked count) in the menu header
- Allow launching any command directly from the menu, with common options configurable via sub-menus
- Only activate in interactive terminals; non-interactive invocations fall back to help text

## User Stories

### TASK-001: Render interactive main menu when no subcommand is given
**Description:** As a user, I want to see an interactive menu when I run `maggus` without arguments so that I can quickly pick a command.

**Acceptance Criteria:**
- [x] Running `maggus` with no subcommand in an interactive terminal launches a bubbletea TUI in alt-screen mode
- [x] The menu lists all commands: `work`, `list`, `status`, `blocked`, `clean`, `release`, `worktree`
- [x] Each command shows a short one-line description next to it
- [x] Arrow keys (up/down) move the selection cursor, Enter launches the selected command
- [x] `q`, `esc`, or `ctrl+c` exits the menu cleanly
- [x] Home/End keys jump to first/last menu item
- [x] Uses shared styles from `internal/tui/styles` for consistent look
- [x] Typecheck/lint passes (`go vet ./...`, `go fmt ./...`)

### TASK-002: Show plan summary banner in menu header
**Description:** As a user, I want to see a quick status summary above the menu so that I know the current state of my plans without running `status` first.

**Acceptance Criteria:**
- [x] Header shows app name and version (e.g., "Maggus v1.2.3")
- [x] Below the title, a summary line shows: number of plans, total tasks, completed tasks, blocked tasks (e.g., "3 plans · 12 tasks · 8 done · 2 blocked")
- [x] If no `.maggus/` directory or no plan files exist, the summary line shows "No plans found"
- [x] Summary is rendered using shared styles (muted for labels, colored for counts)
- [x] Typecheck/lint passes

### TASK-003: Add sub-menus for common command options
**Description:** As a user, I want to configure common options before launching a command so that I don't have to remember CLI flags.

**Acceptance Criteria:**
- [x] After selecting a command that has common options, a sub-menu appears instead of immediately launching
- [x] Sub-menus for each command with options:
  - `work`: option to set task count (1, 3, 5, 10, all) and worktree mode (on/off)
  - `list`: option to set count (5, 10, 20) or all, and plain mode (on/off)
  - `status`: option to toggle `--all` (show completed plans) and `--plain`
  - `blocked`: no sub-menu, launches directly
  - `clean`: no sub-menu, launches directly
  - `release`: no sub-menu, launches directly
  - `worktree`: sub-menu to pick `list` or `clean` subcommand
- [x] Each sub-menu item shows the current/default value
- [x] A "Run" option at the bottom of the sub-menu launches the command with selected options
- [x] `esc` in a sub-menu returns to the main menu without launching
- [x] Typecheck/lint passes

### TASK-004: Detect interactive terminal and fall back to help text
**Description:** As a user piping maggus output or running in CI, I want the default help text instead of a TUI so that non-interactive usage is not broken.

**Acceptance Criteria:**
- [ ] When stdout is not a terminal (e.g., piped to a file or another command), `maggus` with no subcommand prints the standard Cobra help text
- [ ] When stdout is a terminal, the interactive menu is shown
- [ ] Detection uses `os.Stdout.Fd()` with `term.IsTerminal()` or equivalent from the existing `x/term` dependency
- [ ] Existing `--help` flag behavior is unchanged (always prints help text, never launches TUI)
- [ ] Typecheck/lint passes
- [ ] Unit test verifies that the root command still produces help output when `RunE` is set (e.g., by checking output contains "Usage:")

### TASK-005: Launch selected command from the menu
**Description:** As a user, I want the selected command to actually run after I pick it from the menu so that the menu is functional end-to-end.

**Acceptance Criteria:**
- [ ] After the menu TUI exits, the selected command executes with the configured options
- [ ] The alt-screen is properly exited before the selected command starts (no rendering artifacts)
- [ ] If the selected command itself uses alt-screen (status, list, blocked), it works correctly after the menu exits
- [ ] If the user quits the menu without selecting (q/esc), no command is executed and maggus exits cleanly
- [ ] Typecheck/lint passes

## Functional Requirements

- FR-1: The root command must detect whether it is running in an interactive terminal before deciding to show the menu or help text
- FR-2: The menu must list all registered subcommands with their `Short` description from Cobra
- FR-3: Arrow key navigation must wrap around (down from last item goes to first, up from first goes to last)
- FR-4: The header must parse plan files to compute the summary — reuse existing `parsePlans` / `parser.GlobPlanFiles` logic
- FR-5: Sub-menus must build the appropriate `[]string` args to pass to the selected cobra command's `RunE`
- FR-6: The menu TUI must use the viewport scrolling pattern if the menu content exceeds terminal height

## Non-Goals

- No configuration file for menu items or ordering
- No custom keybinding support for the menu
- No persistent menu that returns after a command finishes (one-shot: pick, run, exit)
- No mouse click support for menu selection (arrow keys and enter only)
- No animation or transitions between menu and sub-menus

## Technical Considerations

- The `term.IsTerminal` function is available via `github.com/charmbracelet/x/term` which is already an indirect dependency
- Reuse the `parsePlans` and `findNextTask` helpers from `status.go` — consider extracting them to a shared location if needed, or just call them from the menu code
- The menu should set `rootCmd.RunE` conditionally in `init()` or use Cobra's `PersistentPreRunE` to intercept the no-subcommand case
- Launching the selected command can be done by calling `cmd.ExecuteContext()` or directly invoking the subcommand's `RunE` with constructed args

## Success Metrics

- Running `maggus` without args in a terminal shows the menu instantly (< 200ms to first render)
- Users can discover and launch any command without reading `--help`
- Non-interactive usage (CI, pipes) is completely unaffected

## Open Questions

- Should the menu remember the last-selected command for next invocation? (Probably not for v1)
- Should we show a "last run" timestamp or recent commit info in the header?
