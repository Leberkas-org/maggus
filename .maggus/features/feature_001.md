# Feature 001: Multi-Repo Daemon Manager

## Introduction

When switching between repositories, daemons from previous repositories keep running silently. There is no way to see which repos have active daemons, no way to stop them from a central view, and no way to disable the auto-start behavior per repository. This feature integrates daemon visibility and control directly into the existing `maggus repos` TUI.

### Architecture Context

- **Vision alignment:** Reduces the surprise of silent background work across repos; gives the user explicit control over automation
- **Components involved:** `internal/globalconfig` (Repository struct), `cmd/repos.go` (repos TUI), `cmd/menu.go` + `cmd/daemon_start.go` (auto-start logic)
- **New patterns:** Daemon status polling inside the repos TUI; per-repo daemon ops from a non-current-repo context

## Goals

- Show live daemon status (running/stopped) for every tracked repository in the repos screen
- Allow starting/stopping a daemon and toggling auto-start for any repo from the repos screen
- Respect the per-repo auto-start flag when the menu auto-starts a daemon on load
- Preserve backwards compatibility: existing repos default to auto-start ON

## Tasks

### TASK-001-001: Add auto-start preference to Repository struct
**Description:** As a developer, I want the `Repository` struct to carry an auto-start preference so that the auto-start behavior can be persisted per repository.

**Token Estimate:** ~15k tokens
**Predecessors:** none
**Successors:** TASK-001-002, TASK-001-003
**Parallel:** no — other tasks depend on this
**Model:** haiku

**Acceptance Criteria:**
- [x] `Repository` struct in `internal/globalconfig/globalconfig.go` gains a field that stores whether auto-start is disabled for that repo
- [x] The field uses the negated form (`AutoStartDisabled bool`, yaml: `auto_start_disabled,omitempty`) so the Go zero value (`false`) means "auto-start enabled" — preserving existing behavior for all repos that don't have the field set
- [x] A helper method `IsAutoStartEnabled() bool` is added to `Repository` that returns `true` when `AutoStartDisabled == false`
- [x] Existing `repositories.yml` files without the field load correctly (field absent → auto-start ON)
- [x] `go build ./...` and `go test ./...` pass

### TASK-001-002: Update repos TUI with daemon status and controls
**Description:** As a user, I want the repos screen to show each repo's daemon status and let me start/stop daemons and toggle auto-start so that I can manage all my daemons from one place.

**Token Estimate:** ~80k tokens
**Predecessors:** TASK-001-001
**Successors:** none
**Parallel:** yes — can run alongside TASK-001-003

**Acceptance Criteria:**
- [x] The repos list in `cmd/repos.go` shows a status indicator next to each repo (e.g. `●` running / `○` stopped), using the same color conventions as the menu header
- [x] Status is computed by reading `<repo>/.maggus/daemon.pid` and checking whether the process is alive (reuse the existing PID-check logic from `daemon.go`)
- [x] The TUI polls daemon status every 500ms so the indicator updates live without user action
- [x] Auto-start state is shown per repo (e.g. `[auto]` badge or `[no auto]` muted label)
- [x] When a repo is selected/highlighted, the keybind help footer shows the available actions: `s` start/stop daemon · `a` toggle auto-start · `enter` switch to repo
- [x] Pressing `s` on a repo starts the daemon if stopped, or stops it gracefully if running (reuse `autoStartDaemon` / `stopDaemonGracefully` logic, targeting the selected repo's directory instead of cwd)
- [x] Pressing `a` on a repo toggles `AutoStartDisabled` and saves the global config immediately
- [x] Starting/stopping a repo's daemon does not require switching to that repo first
- [x] A brief status message (e.g. `"daemon started"` / `"daemon stopped"` / `"auto-start disabled"`) appears at the bottom of the screen for 2 seconds after each action
- [x] `go build ./...` and `go test ./...` pass

### TASK-001-003: Respect auto-start flag in menu auto-start logic
**Description:** As a user, I want the menu to skip auto-starting the daemon when I've disabled it for the current repo so that opening the menu doesn't override my preference.

**Token Estimate:** ~15k tokens
**Predecessors:** TASK-001-001
**Successors:** none
**Parallel:** yes — can run alongside TASK-001-002
**Model:** haiku

**Acceptance Criteria:**
- [ ] `autoStartDaemon()` in `cmd/daemon_start.go` loads the global config, finds the `Repository` entry matching the current working directory, and calls `IsAutoStartEnabled()` before proceeding
- [ ] If `IsAutoStartEnabled()` returns `false`, `autoStartDaemon()` returns immediately without starting or logging anything
- [ ] If no matching repo entry is found in global config (edge case), auto-start proceeds as before
- [ ] Existing behavior is unchanged for repos where `AutoStartDisabled` is not set
- [ ] `go build ./...` and `go test ./...` pass

## Task Dependency Graph

```
TASK-001-001 ──→ TASK-001-002
             └─→ TASK-001-003
```

| Task | Estimate | Predecessors | Parallel | Model |
|------|----------|--------------|----------|-------|
| TASK-001-001 | ~15k | none | no | haiku |
| TASK-001-002 | ~80k | 001 | yes (with 003) | — |
| TASK-001-003 | ~15k | 001 | yes (with 002) | haiku |

**Total estimated tokens:** ~110k

## Functional Requirements

- FR-1: The `Repository` struct must store auto-start preference in a backwards-compatible way — zero value means enabled
- FR-2: The repos TUI must display daemon running/stopped status for every tracked repository
- FR-3: The repos TUI must display auto-start on/off state for every tracked repository
- FR-4: The user must be able to start or stop a daemon for any tracked repo without switching to it first
- FR-5: The user must be able to toggle auto-start for any repo and have the change persisted immediately to `~/.maggus/repositories.yml`
- FR-6: `autoStartDaemon()` must check the current repo's `AutoStartDisabled` flag before starting
- FR-7: Daemon status indicators in the repos TUI must refresh live (≤500ms polling)

## Non-Goals

- No new top-level `maggus daemons` command — all controls live in the repos screen
- No daemon log viewer in the repos screen — that remains in the menu status panel
- No global "disable all auto-starts" toggle — per-repo only
- No changes to how daemons are identified or how PID files work

## Technical Considerations

- `autoStartDaemon()` and `stopDaemonGracefully()` currently assume cwd as the target directory; TASK-001-002 needs to invoke them with the selected repo's path — verify whether these functions accept a directory argument or need one added
- Daemon process checking is platform-specific (Windows: `WaitForSingleObject`, Unix: signal 0) — reuse existing helpers from `daemon.go` rather than reimplementing
- `omitempty` on `auto_start_disabled` means the field is omitted from YAML when `false`, keeping existing config files clean

## Open Questions

None.
