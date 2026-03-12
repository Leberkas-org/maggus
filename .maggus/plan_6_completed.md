# Plan: Graceful Shutdown Fix & UI Rework

## Introduction

Two improvements to `maggus work`: (1) Fix Ctrl+C which currently does nothing — the process must be killed via Task Manager. (2) Rework the TUI so it never scrolls, uses a fixed layout with a header showing progress/version/host fingerprint, and handles commit output in a scrolling region that doesn't push the rest of the UI off-screen.

## Goals

- Ctrl+C reliably stops the current Claude run and exits cleanly on all platforms
- Second Ctrl+C force-kills immediately (no stuck process)
- The `maggus work` UI occupies a fixed terminal area that never scrolls
- Header shows: version, host fingerprint (persistent GUID), progress bar with task count
- Current task is visible below the header at all times
- Commit messages after each iteration appear in a bounded scrolling region (like tool history) that does not push content above it off-screen
- Output, tools, extras, model, and elapsed time remain as they are today

## User Stories

### TASK-001: Fix Ctrl+C signal handling in work command
**Description:** As a user, I want Ctrl+C to immediately stop the current Claude run so that I don't have to kill the process via Task Manager.

**Acceptance Criteria:**
- [x] `signal.NotifyContext` uses `syscall.SIGINT` and `syscall.SIGTERM` (on non-Windows) instead of just `os.Interrupt`
- [x] Signal context is created before any blocking I/O in the work loop
- [x] After context cancellation, `stop()` is called to reset signal handling to default so a second Ctrl+C terminates the process immediately (force-quit pattern from graceful-shutdown-guide.md)
- [x] The force-quit goroutine (`<-ctx.Done(); stop()`) is started in `RunClaude` before `cmd.Start()`
- [x] On cancellation, shutdown feedback ("Shutting down...") is printed synchronously in the main goroutine (not a background goroutine that might race with exit)
- [x] No `os.Exit()` calls — use `return` so defers run cleanly
- [x] Platform-specific signal lists: Windows gets `os.Interrupt` only; Unix/macOS gets `syscall.SIGINT, syscall.SIGTERM`
- [x] Typecheck/lint passes (`go vet ./...`)
- [x] Unit tests are written and successful

### TASK-002: Create host fingerprint package
**Description:** As a user, I want a persistent machine identifier shown in the UI header so that I can distinguish runs from different machines.

**Acceptance Criteria:**
- [x] New package `internal/fingerprint` provides a `Get() (string, error)` function that returns a stable UUID
- [x] On first call, generates a new UUID v4 and writes it to a platform-specific path
- [x] On subsequent calls, reads and returns the existing UUID
- [x] Storage paths: Windows → `C:\Program Files\maggus\fingerprint`, Linux → `/usr/local/share/maggus/fingerprint`, macOS → `/Library/Application Support/maggus/fingerprint`
- [x] If the directory does not exist, it is created (with appropriate permissions)
- [x] If writing to the system path fails (e.g. no admin/root), falls back to user-level path: Windows → `%APPDATA%\maggus\fingerprint`, Linux/macOS → `~/.maggus/fingerprint`
- [x] UUID format is standard 8-4-4-4-12 (e.g. `a1b2c3d4-e5f6-7890-abcd-ef1234567890`)
- [x] No external UUID library — use `crypto/rand` to generate UUID v4
- [x] Typecheck/lint passes
- [x] Unit tests are written and successful

### TASK-003: Introduce bubbletea TUI framework
**Description:** As a developer, I want the display layer to use bubbletea so that the UI has proper fixed-layout rendering, resize handling, and clean terminal management.

**Acceptance Criteria:**
- [x] `github.com/charmbracelet/bubbletea` and `github.com/charmbracelet/lipgloss` are added as dependencies
- [x] New file `internal/runner/tui.go` contains the bubbletea Model, Init, Update, View implementation
- [x] The old `display` struct and its methods (`renderLocked`, spinner goroutine, ANSI cursor manipulation) are replaced by the bubbletea model
- [x] The bubbletea program uses `tea.WithAltScreen()` so the TUI occupies a fixed screen that never scrolls
- [x] The bubbletea program uses `tea.WithMouseCellMotion()` is NOT used (no mouse needed)
- [x] The model receives events via `tea.Msg` types: tool updates, output updates, status changes, tick (for spinner + elapsed time)
- [x] `RunClaude` starts the bubbletea program, feeds it messages from the stream parser, and waits for it to quit
- [x] Ctrl+C is handled by bubbletea's built-in `tea.KeyCtrlC` message type, which sends a quit signal and triggers the context cancellation from TASK-001
- [x] Terminal is always restored cleanly on exit (bubbletea handles this, but verify with force-kill scenarios)
- [x] Typecheck/lint passes

### TASK-004: Build the header section
**Description:** As a user, I want to see the maggus version, host fingerprint, and task progress at the top of the screen so I always know the run status at a glance.

**Acceptance Criteria:**
- [x] Header is rendered at the top of the TUI view
- [x] Header shows: `Maggus vX.Y.Z` on the left, host fingerprint (truncated or full UUID) on the right
- [x] Below the version line: progress bar in format `[████████░░░░] 3/10 Tasks` showing current iteration vs total count
- [x] Progress bar updates after each iteration completes
- [x] Header is visually separated from the content below (e.g. a horizontal line or color contrast)
- [x] Version and fingerprint are passed into the TUI model at construction time
- [x] Typecheck/lint passes

### TASK-005: Build the task info section
**Description:** As a user, I want to see the current task ID and title below the header so I know what maggus is working on.

**Acceptance Criteria:**
- [x] Section below the header shows: `TASK-NNN: Task Title`
- [x] Task info updates when the work loop moves to the next task
- [x] The task info is sent to the TUI model as a message when each iteration starts
- [x] Visually distinct from the header (e.g. different color or indentation)
- [x] Typecheck/lint passes

### TASK-006: Build the main status section (output, tools, extras, model, elapsed)
**Description:** As a user, I want the existing status display (output, tools, extras, model, elapsed time) to work within the new fixed-layout TUI.

**Acceptance Criteria:**
- [x] The status section renders: spinner + status, output (last line), tool history (last 10), extras, model, elapsed time — same information as today
- [x] Spinner animates via bubbletea tick messages (100ms interval)
- [x] Tool history shows the last 10 tools with `│` prefix and `▶` for the most recent, same as current behavior
- [x] The section occupies a fixed vertical space (status + output + 10 tool lines + extras + model + elapsed = predictable height)
- [x] All stream events (assistant text, tool_use, result) update the model via messages, same parsing logic as today
- [x] Typecheck/lint passes

### TASK-007: Build the commit message scrolling region
**Description:** As a user, I want commit messages to appear in a small scrolling area that does not push the rest of the UI off-screen, similar to how tool history works.

**Acceptance Criteria:**
- [x] After each iteration, the commit result message is sent to the TUI model
- [x] A "Recent Commits" section at the bottom shows the last 3-5 commit messages (one line each, truncated to terminal width)
- [x] New commit messages push older ones up within this bounded region
- [x] The commit section does not cause any content above it to move or scroll
- [x] Between iterations (while Claude is running), the commit section retains previous messages
- [x] The section is visually labeled (e.g. `Commits:` header) and uses subdued colors
- [x] Typecheck/lint passes

### TASK-008: Wire everything together in the work command
**Description:** As a developer, I want the work command to use the new TUI, passing all required data (version, fingerprint, progress, task info, commit results) so that the full experience works end-to-end.

**Acceptance Criteria:**
- [x] `cmd/work.go` calls `fingerprint.Get()` at startup and passes the result to the TUI
- [x] The startup banner (version, model, iterations, branch, run ID, permissions warning, 3-second countdown) is shown BEFORE entering the bubbletea alt-screen — it stays in the normal terminal scrollback
- [x] After the countdown, the bubbletea TUI takes over the full screen
- [x] Progress (current iteration, total count) is updated via messages to the TUI model at each loop iteration
- [x] Task info is updated via messages at each loop iteration
- [x] Commit results are sent to the TUI after each `gitcommit.CommitIteration`
- [x] When the work loop ends (all tasks done, interrupted, or count reached), the bubbletea program is quit and the summary banner is printed in normal terminal mode
- [x] The 3-second abort countdown still works with Ctrl+C (before TUI starts)
- [x] Typecheck/lint passes

### TASK-009: Unstage .maggus/runs files before committing
**Description:** As a user, I want `.maggus/runs/*` files to never be committed, even if the AI agent manually stages them with `git add .` or `git add -A`.

**Acceptance Criteria:**
- [x] `gitcommit.CommitIteration` unstages `.maggus/runs/` before committing, using `git reset HEAD -- .maggus/runs/` (same pattern already used for COMMIT.md)
- [x] Also unstage `.maggus/MEMORY.md` with the same approach (it is gitignored but could be force-staged)
- [x] The unstage commands run silently — errors are ignored (files may not be staged)
- [x] Existing unstage of COMMIT.md is preserved
- [x] Typecheck/lint passes
- [x] Unit tests are written and successful

### TASK-010: End-to-end testing and cleanup
**Description:** As a developer, I want to verify the full flow works and clean up any leftover code from the old display implementation.

**Acceptance Criteria:**
- [x] Old `display` struct, `newDisplay`, `renderLocked`, and ANSI cursor manipulation code are fully removed (no dead code)
- [x] `go build ./...` succeeds with no warnings
- [x] `go vet ./...` passes
- [x] `go test ./...` passes
- [x] The `stderrWriter`, `describeToolUse`, and `truncate` helpers are preserved (still needed by the stream parser)
- [x] No unused imports or variables remain

## Functional Requirements

- FR-1: Pressing Ctrl+C during a Claude run must cancel the subprocess within 1 second and return to the shell
- FR-2: Pressing Ctrl+C a second time during shutdown must force-kill immediately
- FR-3: The TUI must use an alternate screen buffer so normal terminal scrollback is not affected
- FR-4: The host fingerprint must be identical across restarts on the same machine
- FR-5: The host fingerprint must be different on different machines (UUID v4 collision probability is negligible)
- FR-6: If the system-level fingerprint path is not writable, the fallback user-level path must be used silently (no error shown to user)
- FR-7: The progress bar must accurately reflect completed iterations vs total requested count
- FR-8: The commit scrolling region must never exceed its allocated height (3-5 lines)
- FR-9: Terminal must be fully restored (cursor visible, alt screen exited) even on error or interrupt
- FR-10: Files under `.maggus/runs/` and `.maggus/MEMORY.md` must never be included in iteration commits, even if the AI agent explicitly stages them

## Non-Goals

- No mouse support in the TUI
- No interactive input during the work loop (it remains a fire-and-forget execution)
- No color theme customization or config options for the TUI layout
- No changes to `maggus list` or `maggus status` commands — only `maggus work` is affected
- No network-based fingerprint (e.g. MAC address) — just a random UUID persisted to disk

## Technical Considerations

- Bubbletea runs its own event loop on a goroutine. Stream events from Claude must be sent as `tea.Msg` via `p.Send()` from the scanner goroutine
- The bubbletea `Program` must be created with `tea.WithAltScreen()` for fixed layout
- On Windows, `C:\Program Files\maggus\` requires admin rights to write. The fallback to `%APPDATA%\maggus\` handles non-admin users. Consider trying the system path first and silently falling back
- `crypto/rand` for UUID generation avoids adding a dependency like `google/uuid`
- The `cmd.Cancel` + `taskkill /T /F` mechanism for killing the child process tree on Windows should continue to work with bubbletea — bubbletea's Ctrl+C handler can trigger context cancellation which triggers `cmd.Cancel`
- lipgloss can handle terminal width measurement and text truncation, replacing the manual `truncate()` and `termWidth()` helpers

## Success Metrics

- Ctrl+C exits the process within 2 seconds on Windows, Linux, and macOS
- The TUI never causes terminal scrolling during a multi-iteration run
- Host fingerprint remains stable across 10+ restarts
- Commit messages are visible but contained — they don't push the header or task info off-screen

## Open Questions

- Should the 3-second startup countdown also be rendered inside the bubbletea TUI, or is it fine in normal terminal mode before the alt-screen activates?
- Should the progress bar show overall plan progress (e.g. 15/30 total tasks across all plans) in addition to the current run's iteration progress?
- If bubbletea's alt-screen causes issues with Claude's stderr output (which currently goes to os.Stderr), should stderr be captured and shown in a TUI section instead of passed through?
