# Feature 001: Daemon-First UX Polish

## Introduction

Streamline the Maggus experience so the daemon is always running in the background, the primary view is a unified screen combining live log output with feature/task management, the run log is stored as JSONL for easy parsing, daemon-mode token usage is tracked, and log display is color-coded for readability.

### Architecture Context

- **Components touched:** `cmd/menu.go`, `cmd/root.go` (auto-start + menu cleanup), `cmd/status.go` + `cmd/status_runlog.go` (unified screen + log parsing), `internal/runlog/runlog.go` (JSONL format), `cmd/daemon_tui.go` (token tracking), `internal/tui/styles` (colors)
- **No new packages required** — changes are contained to existing components
- **Breaking change:** run.log format changes from plain text to JSONL; old logs won't be parsed by the new reader (acceptable — logs are ephemeral)

## Goals

- Daemon starts silently when Maggus launches (no user action required)
- Quitting the menu always prompts "Stop daemon? [y/N]" if daemon is running
- A unified screen replaces the separate status/work/start-daemon menu entries; `work` and `start daemon` are removed from the menu
- Run logs are JSONL for structured, easy downstream parsing
- Token usage is tracked and saved when tasks complete via daemon
- Log display uses subtle color-coding with dimmed timestamps and highlighted task IDs

## Tasks

### TASK-001-001: Auto-start daemon silently on Maggus launch
**Description:** As a user, I want the daemon to start automatically when I open Maggus so that I never need to manually start it before work begins.

**Token Estimate:** ~25k tokens
**Predecessors:** none
**Successors:** none
**Parallel:** yes — can run alongside all other tasks

**Acceptance Criteria:**
- [x] On Maggus startup (menu init), if no daemon PID file exists or the process is not alive, the daemon is started silently in the background using the existing daemon start logic
- [x] No output or prompt is shown to the user during auto-start
- [x] If the daemon is already running, no second instance is started
- [x] If the daemon fails to start (e.g. no Claude available), a non-fatal warning is shown but the menu still opens
- [x] Existing mutual exclusion (daemon.pid / work.pid) is respected

---

### TASK-001-002: "Stop daemon?" prompt on menu quit
**Description:** As a user, I want to be asked whether to stop the daemon when I exit Maggus so that I can choose between leaving it running or shutting it down cleanly.

**Token Estimate:** ~20k tokens
**Predecessors:** none
**Successors:** none
**Parallel:** yes — can run alongside all other tasks

**Acceptance Criteria:**
- [x] When the user quits the main menu (q or Ctrl+C) and the daemon is currently running, a confirmation prompt is shown: `Stop daemon? [y/N]`
- [x] Default answer is N (daemon keeps running) — pressing Enter without input exits without stopping
- [x] Answering y stops the daemon using the existing stop logic before exiting
- [x] If the daemon is not running, no prompt is shown and the app exits immediately
- [x] The prompt is non-blocking and keyboard-driven (fits within the existing Bubbletea model)

---

### TASK-001-003: Convert run.log to JSONL format
**Description:** As a developer/tool, I want run events stored as JSONL so that logs are easy to parse, filter, and extend without fragile string parsing.

**Token Estimate:** ~50k tokens
**Predecessors:** none
**Successors:** TASK-001-005
**Parallel:** yes — can run alongside 001, 002, 004

**Acceptance Criteria:**
- [ ] `internal/runlog/runlog.go` emits one JSON object per line (JSONL) instead of plain text
- [ ] Each entry has at minimum: `ts` (RFC3339), `level` (`info`/`output`/`error`), `event` (e.g. `feature_start`, `task_start`, `task_complete`, `task_failed`, `tool_use`, `output`, `info`), plus event-specific fields (e.g. `feature_id`, `task_id`, `title`, `commit`, `tool`, `description`, `text`)
- [ ] Example entries:
  ```jsonl
  {"ts":"2026-03-24T13:00:00Z","level":"info","event":"task_start","task_id":"TASK-001-001","title":"Do something"}
  {"ts":"2026-03-24T13:00:05Z","level":"info","event":"tool_use","task_id":"TASK-001-001","tool":"Read","description":"Read config.go"}
  {"ts":"2026-03-24T13:00:10Z","level":"output","event":"output","task_id":"TASK-001-001","text":"Agent output here"}
  {"ts":"2026-03-24T13:00:12Z","level":"info","event":"task_complete","task_id":"TASK-001-001","commit":"abc1234"}
  ```
- [ ] All existing call sites (`FeatureStart`, `TaskStart`, `ToolUse`, `Output`, etc.) continue to work with the same method signatures — only the output format changes
- [ ] `status_runlog.go` (`parseLogForCurrentState`, `readLastNLogLines`) is updated to parse JSONL instead of plain text; non-JSON lines are silently skipped to handle mixed-format files gracefully
- [ ] `go test ./...` passes

---

### TASK-001-004: Fix daemon-mode token usage tracking
**Description:** As a user, I want token usage to be recorded when tasks complete via the daemon so that my usage history is complete regardless of how work was triggered.

**Token Estimate:** ~35k tokens
**Predecessors:** none
**Successors:** none
**Parallel:** yes — can run alongside all other tasks

**Acceptance Criteria:**
- [ ] When a task completes via the daemon keep-alive loop, token usage (input tokens, output tokens, cost if available) is written to the appropriate `usage_*.jsonl` file under `.maggus/`
- [ ] The daemon token tracking uses the same `internal/usage` package and file format as the `work` command
- [ ] Usage is attributed to the same command source identifier that `work` uses
- [ ] Token counts are sourced from the streaming JSON events parsed by the agent runner (same source as the TUI token counter)
- [ ] After a daemon-run task completes, the `.maggus/usage_*.jsonl` file contains a new entry for that task

---

### TASK-001-005: Build unified status/log TUI screen
**Description:** As a user, I want a single screen that shows the live daemon log and lets me manage features/bugs so that I don't need to switch between separate commands.

**Token Estimate:** ~100k tokens
**Predecessors:** TASK-001-003
**Successors:** TASK-001-006
**Parallel:** no — requires JSONL log parser from TASK-001-003
**Model:** opus

**Acceptance Criteria:**
- [ ] The existing "status", "work", and "start daemon" menu entries are replaced by a single entry (name: "status") that opens the unified screen
- [ ] **Default view:** Full-screen live log panel showing the last N lines of the current run's JSONL log, auto-scrolling as new entries arrive, with 200ms polling
- [ ] **Toggle:** Pressing Tab (or a labeled key shown in the footer) switches to the feature/bug management view (equivalent to current status tabs: Features / Bugs, task list, detail pane)
- [ ] Pressing Tab again switches back to the log view
- [ ] A one-line daemon status bar is always visible at the top: shows daemon PID + current task (or "idle" / "stopped")
- [ ] The screen renders correctly at typical terminal widths (80–220 cols)
- [ ] Ctrl+C / q exits the screen and returns to the main menu (does not stop the daemon)
- [ ] The existing standalone `status` command (CLI: `maggus status`) still works and behaves as before (no regression)

---

### TASK-001-006: Color-coded log display
**Description:** As a user, I want the log panel to use color-coding so that I can quickly scan log output and distinguish event types at a glance.

**Token Estimate:** ~30k tokens
**Predecessors:** TASK-001-005
**Successors:** none
**Parallel:** no — requires unified screen from TASK-001-005

**Acceptance Criteria:**
- [ ] Timestamps are rendered dimmed/muted (low contrast, subordinate to content)
- [ ] Task IDs (e.g. `TASK-001-001`) are highlighted — bold or teal accent matching the existing Maggus color palette
- [ ] `tool_use` events show the tool name (e.g. `[Read]`, `[Edit]`) in a distinct accent color
- [ ] `output` events render in default/white (full contrast — this is the most important content)
- [ ] `error` / `task_failed` events are rendered in red
- [ ] `info` events (feature start/complete, task start/complete) are rendered in muted/gray
- [ ] Colors use the existing `internal/tui/styles` package — no new color constants are introduced without good reason
- [ ] Color rendering degrades gracefully (plain text) if the terminal does not support ANSI colors

---

## Task Dependency Graph

```
TASK-001-001 ─────────────────────────────────────────────────────────┐
TASK-001-002 ─────────────────────────────────────────────────────────┤ (independent)
TASK-001-003 ──→ TASK-001-005 ──→ TASK-001-006
TASK-001-004 ─────────────────────────────────────────────────────────┘
```

| Task | Estimate | Predecessors | Parallel | Model |
|------|----------|--------------|----------|-------|
| TASK-001-001 | ~25k | none | yes | — |
| TASK-001-002 | ~20k | none | yes | — |
| TASK-001-003 | ~50k | none | yes | — |
| TASK-001-004 | ~35k | none | yes | — |
| TASK-001-005 | ~100k | 003 | no | opus |
| TASK-001-006 | ~30k | 005 | no | — |

**Total estimated tokens:** ~260k

## Functional Requirements

- FR-1: The daemon must start automatically and silently on Maggus launch if not already running
- FR-2: Quitting the main menu must prompt "Stop daemon? [y/N]" when the daemon is active; default is N (keep running)
- FR-3: The "work" and "start daemon" menu entries are removed; the unified screen is the sole entry point for working and monitoring
- FR-4: The unified screen shows a live JSONL log by default; Tab toggles to the feature/bug management view
- FR-5: Run logs must be written as JSONL with fields: `ts`, `level`, `event`, and event-specific payload fields
- FR-6: Token usage from daemon-executed tasks must be persisted to `usage_*.jsonl` using the same format as the `work` command
- FR-7: Log lines must be color-coded: timestamps dimmed, task IDs highlighted, tool names accented, output full-brightness, errors red, info muted

## Non-Goals

- No changes to the `work` command's underlying logic or CLI flags (it remains available as a direct CLI command, just removed from the interactive menu)
- No changes to feature/bug parsing logic
- No backward compatibility for old plain-text run.log files (non-JSON lines are skipped silently)
- No new remote log shipping or external observability integrations
- No changes to how the daemon keep-alive loop selects or executes tasks

## Technical Considerations

- **JSONL backward compat:** Old run.log files (plain text) will not parse under the new reader. Graceful fallback: skip non-JSON lines silently so mixed-format files don't crash the parser.
- **Bubbletea quit flow:** The "stop daemon?" prompt requires careful handling — `tea.Quit` must be deferred until after the user responds. Use a confirmation state in the menu model (same pattern as delete confirmation in `tasklist.go`).
- **Token tracking in daemon:** The `nullTUIModel` in `daemon_tui.go` receives tool/output callbacks — token counts come from `result` events in the stream JSON. Accumulate and flush to `internal/usage` at task completion.
- **Menu cleanup:** Removing `work` and `start daemon` from the menu item list in `menu.go` — ensure any keyboard shortcut bindings for those items are also cleaned up.

## Success Metrics

- Opening Maggus and having the daemon already running requires zero manual steps
- A user can see live task progress and manage features/bugs from a single screen
- Token usage history is complete for all work done via daemon
- Log output is scannable at a glance due to color-coding
