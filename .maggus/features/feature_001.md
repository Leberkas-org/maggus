<!-- maggus-id: e6cb73e5-a85b-4b84-bd6d-ce89a7db38a4 -->
# Feature 001: Discord Rich Presence Integration

## Introduction

Add Discord Rich Presence support to Maggus so that it appears as a "game" in the user's Discord profile. When someone views the user's Discord status, they see "Running Maggus" — and expanding the status reveals the feature title and current task being worked on (e.g. "User Auth — TASK-003-002: Add login page").

This uses Discord's local IPC protocol (no bot, no network auth) and is opt-in via config.

### Architecture Context

- **Components involved:** The work loop (`cmd/work.go`), the TUI model (`internal/runner/tui.go`), and the bubbletea message system (`internal/agent/messages.go`)
- **New component:** A new `internal/discord` package implementing minimal Discord Rich Presence IPC directly (no external dependency)
- **Integration point:** The TUI already receives `IterationStartMsg` (with task ID, title, feature file) and `StatusMsg` (agent activity). The discord package subscribes to these same messages to update presence.
- **Config:** Extends `internal/config` to support a `discord_presence: true` opt-in flag
- **Assets:** Maggus logo exists at `src/winres/icon.png` and `docs/avatar.png` — will be uploaded to the Discord Developer Portal as the application icon

## Goals

- Show Maggus as a "game" in Discord profile when `maggus work` is running
- Display feature title and current task in the expanded presence view
- Opt-in via config — zero impact on users who don't enable it
- Gracefully degrade when Discord is not running
- No external Go dependencies for Discord IPC — minimal self-contained implementation

## Tasks

### TASK-001-001: Create Discord Application & upload assets
**Description:** As a maintainer, I want a Discord Application registered in the Discord Developer Portal so that Rich Presence has an Application ID and the Maggus logo appears in user profiles.

**Token Estimate:** ~10k tokens
**Predecessors:** none
**Successors:** TASK-001-003
**Parallel:** yes — can run alongside TASK-001-002
**Model:** haiku — just documentation and a constant

**Acceptance Criteria:**
- [~] ⚠️ BLOCKED: Discord Application is created at discord.com/developers/applications — requires manual human interaction with the Discord Developer Portal
- [~] ⚠️ BLOCKED: Maggus logo uploaded as the application icon (from `docs/avatar.png` or `src/winres/icon.png`) — requires manual upload in Discord Developer Portal
- [~] ⚠️ BLOCKED: At least one Rich Presence asset uploaded (key: "maggus_logo" for the large image) — requires manual upload in Discord Developer Portal
- [x] Application ID is documented and added as a constant in the new `internal/discord` package
- [x] A brief section in the README or a `docs/discord-setup.md` explains how this was set up (for future maintainers)

### TASK-001-002: Add `discord_presence` config option
**Description:** As a user, I want a `discord_presence` option in `.maggus/config.yml` so that I can opt-in to Discord Rich Presence.

**Token Estimate:** ~25k tokens
**Predecessors:** none
**Successors:** TASK-001-003
**Parallel:** yes — can run alongside TASK-001-001

**Acceptance Criteria:**
- [x] `internal/config` struct has a `DiscordPresence bool` field (yaml: `discord_presence`)
- [x] Defaults to `false` (off by default)
- [x] Parsing correctly reads `discord_presence: true` from config.yml
- [x] Unit tests cover: missing key (defaults false), explicit true, explicit false
- [x] `go vet ./...` passes

### TASK-001-003: Implement `internal/discord` Rich Presence package
**Description:** As a developer, I want a `discord` package that manages Discord Rich Presence via direct IPC so that other parts of Maggus can update the presence state without external dependencies.

**Token Estimate:** ~75k tokens
**Predecessors:** TASK-001-001, TASK-001-002
**Successors:** TASK-001-004
**Parallel:** no
**Model:** opus — IPC protocol implementation, platform-specific pipes, connection management

**Acceptance Criteria:**
- [x] New package `internal/discord` with a `Presence` type
- [x] Implements Discord IPC protocol directly — no external library dependency (uses Unix socket on Linux/macOS, named pipe `\\.\pipe\discord-ipc-0` on Windows)
- [x] `Connect()` method — connects to Discord IPC; returns nil error silently if Discord is not running
- [x] `Update(state PresenceState)` method — sets activity with: details (feature title + task info), state ("Running Maggus"), large image key ("maggus_logo"), timestamps
- [x] `Close()` method — clears presence and disconnects cleanly
- [x] `PresenceState` struct has fields: `TaskID`, `TaskTitle`, `FeatureTitle`, `StartTime`
- [x] Platform-specific IPC connection files: `ipc_windows.go` and `ipc_unix.go` (build tags)
- [x] If Discord disconnects mid-session, logs once and stops trying (no retry spam)
- [x] Unit tests for state formatting logic and message serialization (not IPC itself — that requires Discord running)
- [x] `go vet ./...` passes

### TASK-001-004: Integrate presence updates into the work loop TUI
**Description:** As a user, I want my Discord status to automatically update as Maggus works through tasks so that my teammates can see what Maggus is doing.

**Token Estimate:** ~50k tokens
**Predecessors:** TASK-001-003
**Successors:** TASK-001-005
**Parallel:** no

**Acceptance Criteria:**
- [ ] When `discord_presence: true` in config, a `discord.Presence` is created and connected at work loop start
- [ ] On `IterationStartMsg`: presence updates with feature title + task ID + task title
- [ ] Details line format: "Feature Title — TASK-NNN-XXX: Task Title"
- [ ] State line: "Running Maggus"
- [ ] On work loop exit (normal completion, Ctrl+C, or error): presence is cleared via `Close()`
- [ ] When `discord_presence` is false or missing: no Discord code is initialized at all
- [ ] When Discord is not running: no errors shown, maggus works exactly as before
- [ ] Presence shows elapsed time (Discord "elapsed" timestamp from iteration start)
- [ ] `go vet ./...` passes

### TASK-001-005: End-to-end testing and polish
**Description:** As a developer, I want to verify the full integration works correctly so that we can ship with confidence.

**Token Estimate:** ~25k tokens
**Predecessors:** TASK-001-004
**Successors:** none
**Parallel:** no

**Acceptance Criteria:**
- [ ] Manual test: run `maggus work` with `discord_presence: true` and Discord open — presence appears with Maggus logo
- [ ] Manual test: presence shows "Running Maggus" as state and feature/task as details
- [ ] Manual test: presence updates when task changes
- [ ] Manual test: presence clears on Ctrl+C and on normal completion
- [ ] Manual test: run with Discord closed — no errors, maggus works normally
- [ ] Manual test: run with `discord_presence: false` — no presence appears
- [ ] All existing tests still pass (`go test ./...`)
- [ ] `go fmt ./... && go vet ./...` passes

## Task Dependency Graph

```
TASK-001-001 ──→ TASK-001-003 ──→ TASK-001-004 ──→ TASK-001-005
TASK-001-002 ──┘
```

| Task | Estimate | Predecessors | Parallel | Model |
|------|----------|--------------|----------|-------|
| TASK-001-001 | ~10k | none | yes (with 002) | haiku |
| TASK-001-002 | ~25k | none | yes (with 001) | — |
| TASK-001-003 | ~75k | 001, 002 | no | opus |
| TASK-001-004 | ~50k | 003 | no | — |
| TASK-001-005 | ~25k | 004 | no | — |

**Total estimated tokens:** ~185k

## Functional Requirements

- FR-1: When `discord_presence: true` is set in `.maggus/config.yml` and Discord is running, Maggus must register as a Rich Presence activity
- FR-2: The presence state line must show "Running Maggus"
- FR-3: The presence details line must show the feature title and current task (e.g. "User Auth — TASK-003-002: Add login page")
- FR-4: The presence large image must display the Maggus logo (asset key "maggus_logo")
- FR-5: The presence must show elapsed time since the current task started
- FR-6: The presence must update when Maggus moves to a new task
- FR-7: The presence must be cleared when Maggus exits (normally or via Ctrl+C)
- FR-8: When Discord is not running, Maggus must silently continue without errors or warnings
- FR-9: When `discord_presence` is false or absent, no Discord connection must be attempted
- FR-10: Discord IPC must be implemented directly without external Go dependencies

## Non-Goals

- No Discord bot integration (no messages, no webhooks — Rich Presence only)
- No per-command presence (only `maggus work` for now)
- No real-time tool/thinking status in the presence details (just task-level info)
- No small image overlays for agent status — just the large Maggus logo
- No user-configurable presence text or images
- No Discord Application ID configurability (hardcoded for now)
- No external Go library for Discord IPC

## Technical Considerations

- Discord Rich Presence uses local IPC — no network or authentication needed
- Platform-specific IPC: Unix uses `/tmp/discord-ipc-0` socket, Windows uses `\\.\pipe\discord-ipc-0` named pipe
- The IPC protocol sends JSON payloads with a binary header (opcode + length) — straightforward to implement in ~100-200 lines
- The TUI uses bubbletea — presence updates should be triggered from the `Update()` method when relevant messages arrive, not from a separate goroutine polling
- `IterationStartMsg` already carries `TaskID`, `TaskTitle`, `FeatureFile` — feature title can be derived from the parser
- The Discord Application ID is a public value (not a secret) — safe to hardcode as a constant
- Existing platform-specific pattern in the codebase: `runner/procattr_windows.go` and `procattr_other.go` — follow the same build tag convention for IPC

## Success Metrics

- Discord profile shows "Running Maggus" with the Maggus logo while `maggus work` is active
- Expanding the status shows the feature title and current task
- Presence disappears within seconds of Maggus exiting
- Zero impact on users who don't enable the feature
- No new external Go dependencies added

## Open Questions

None — all resolved.
