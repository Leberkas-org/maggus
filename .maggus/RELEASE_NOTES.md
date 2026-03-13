## TASK-008: Parse and track token usage from Claude Code output

- The work TUI now displays cumulative token usage (input/output) in the status area during execution
- The post-completion summary screen shows total token usage and a per-task breakdown
- Token counts use a compact format with `k` suffix for thousands (e.g., "12.3k in / 8.1k out")
- Shows "N/A" gracefully when running with older Claude CLI versions that don't report usage

## TASK-007: Add configurable sound notifications

- Optional sound notifications when tasks complete, the run finishes, or errors occur — plays a terminal bell (BEL character)
- Disabled by default; enable via `notifications.sound: true` in `.maggus/config.yml`
- Individual event types (`on_task_complete`, `on_run_complete`, `on_error`) can be toggled independently

## TASK-006: Restyle `blocked` command with lipgloss TUI

- `maggus blocked` wizard now uses lipgloss styling instead of raw ANSI escape codes for a polished, consistent look
- Action picker menu items are styled with semantic colors (green for unblock, yellow for resolve, red for abort)
- The wizard summary is rendered inside a styled bordered box

## TASK-005: Restyle `list` command with lipgloss TUI

- `maggus list` now renders in a full-screen TUI with lipgloss styling inside a bordered box
- The next task (first in the list) is highlighted with primary color and an arrow indicator
- The `--plain` flag still outputs unformatted text for scripting/piping

## TASK-004: Restyle `status` command with lipgloss TUI

- `maggus status` now renders in a polished full-screen TUI with a lipgloss-styled box frame
- Task icons are styled: ✓ (green) for complete, ⚠ (red) for blocked, ○ (gray) for pending, → (cyan) for next-up
- The `--plain` flag still outputs unformatted text for scripting/piping

## TASK-003: Add post-completion summary screen to `work` TUI

- After `maggus work` completes, a styled summary screen shows run ID, branch, model, elapsed time, tasks completed, commit range, and remaining tasks
- Git push runs in the background with a spinner while the summary is displayed
- Press any key to exit cleanly back to the shell

## TASK-002: Refactor `work` TUI — remove countdown, capture all output

- `maggus work` now starts immediately with no 3-second countdown delay
- Startup banner, branch creation messages, and git push output are all displayed inside the TUI instead of raw terminal output
- After work completes, the TUI shows a "done" screen and waits for a keypress before exiting — no raw output leaks to the terminal

## TASK-002: Add release notes accumulation to work loop

- The work loop now instructs the agent to append user-visible change notes to `.maggus/RELEASE_NOTES.md` after each task
- `.maggus/RELEASE_NOTES.md` is automatically gitignored and excluded from commits

## TASK-001: Create shared TUI style package

- Added shared lipgloss style package for consistent visual styling across all TUI commands
- Refactored `maggus status` to use lipgloss instead of raw ANSI escape codes
