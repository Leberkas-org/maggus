<!-- maggus-id: f64afc87-f7f7-4114-93c1-37d58e787e72 -->
<!-- maggus-id: 20260326-154615-feature-004 -->
# Feature 004: Status View — Daemon Stop Prompt on Exit

## Introduction

When a user presses `q`, `esc`, or `ctrl+c` to exit the status view while the daemon is running, the view exits silently — leaving the daemon running in the background. This is fine when auto-start is enabled (the daemon is expected to run continuously and will restart on its own). But when auto-start is **disabled** for this repository, the daemon is running because the user started it manually. Leaving the status view without being asked is surprising: the daemon keeps consuming resources with no easy way to notice.

This feature adds a one-line exit overlay that appears only when both conditions are true: the daemon is running, and auto-start is disabled for the current repository. The overlay lets the user choose between stopping gracefully (after the current task), killing immediately, or leaving the daemon running and just exiting.

### Architecture Context

- **Components involved:** `cmd/status_update.go` (exit key handling), `cmd/status_view.go` (overlay rendering), `cmd/status_model.go` (new state field), `cmd/daemon_stop.go` (stop/kill invocation), `internal/globalconfig/globalconfig.go` (auto-start check via `IsAutoStartEnabled()`)
- **Pattern:** Same overlay pattern used for `confirmDeleteFeature` and `daemonStopOverlay` (feature 003) — a boolean flag on `statusModel` that intercepts key input before normal routing
- **No new files** — all changes within existing files

## Goals

- When exiting the status view with the daemon running and auto-start disabled, the user is prompted before the view closes
- The user can choose: stop after current task, kill now, or leave running and exit
- When auto-start is enabled, the exit is immediate and unchanged — no prompt shown
- The check must cover all top-level exit paths: `q`, `esc`, and `ctrl+c`

## Tasks

### TASK-004-001: Add exit daemon prompt for auto-start-disabled repos
**Description:** As a user exiting the status view, I want to be asked whether to stop the daemon when I started it manually (auto-start off), so I don't accidentally leave it running without noticing.

**Token Estimate:** ~40k tokens
**Predecessors:** none
**Successors:** none
**Parallel:** no — single task

**Acceptance Criteria:**
- [x] A new boolean field `exitDaemonOverlay bool` is added to `statusModel` in `src/cmd/status_model.go`
- [x] A helper method `(m statusModel) shouldPromptOnExit() bool` is added; it returns `true` only when `m.daemon.Running` is true AND `globalconfig.Load()` finds the current repo (matched by absolute path of `m.dir`) with `IsAutoStartEnabled() == false`; if the repo is not found in the config or the config fails to load, it returns `false` (no prompt — safe default)
- [x] A helper method `(m statusModel) handleQuitRequest() (statusModel, tea.Cmd)` is added; it calls `shouldPromptOnExit()` — if true, sets `m.exitDaemonOverlay = true` and returns `m, nil`; if false, returns `m, tea.Quit`
- [x] All top-level `return m, tea.Quit` paths in `updateList()` that correspond to user-initiated exit (`q`, `esc`, `ctrl+c`) are replaced with `return m.handleQuitRequest()`; this includes:
  - the right-pane handler's `case "q", "esc", "ctrl+c"` branch
  - the left-pane fall-through to the task list component where `taskListQuit` or `taskListRun` actions cause quit
  - any other `tea.Quit` returns in `updateList()` that are triggered by the user pressing an exit key
- [x] `m.exitDaemonOverlay` is checked at the top of `Update()` (before `updateList()` is called), similar to how `m.confirmDeleteFeature` is checked; when true, a new handler `updateExitDaemonOverlay(msg tea.KeyMsg)` is called
- [x] `updateExitDaemonOverlay` handles:
  - `t` or `T` — invoke graceful stop (reuse the graceful stop path from `daemon_stop.go`), then return `m, tea.Quit`
  - `k` or `K` — invoke immediate kill (reuse the kill path from `daemon_stop.go`), then return `m, tea.Quit`
  - `l`, `L`, `enter`, `esc` — leave daemon running, set `m.exitDaemonOverlay = false`, return `m, tea.Quit`
  - `ctrl+c` — same as `esc`: leave running and quit
- [x] In `src/cmd/status_view.go`, when `m.exitDaemonOverlay` is true, the footer hint line is replaced with the overlay prompt text rendered using `styles.StatusBar`: `Daemon is running:  t stop after task  k kill now  l/esc leave running`
- [x] When `exitDaemonOverlay` is false, all existing footer hints are unchanged
- [x] `go vet ./...` passes

## Task Dependency Graph

```
TASK-004-001
```

| Task | Estimate | Predecessors | Parallel | Model |
|------|----------|--------------|----------|-------|
| TASK-004-001 | ~40k | none | no | — |

**Total estimated tokens:** ~40k

## Functional Requirements

- FR-1: When `q`, `esc`, or `ctrl+c` is pressed in the status view with the daemon running and auto-start disabled, the view must NOT quit immediately — the exit overlay must appear instead
- FR-2: The exit overlay must display in the footer bar as: `Daemon is running:  t stop after task  k kill now  l/esc leave running`
- FR-3: Pressing `t` in the overlay must trigger a graceful daemon stop and then exit the status view
- FR-4: Pressing `k` in the overlay must trigger an immediate daemon kill and then exit the status view
- FR-5: Pressing `l`, `enter`, `esc`, or `ctrl+c` in the overlay must leave the daemon running and exit the status view
- FR-6: If the daemon is not running, exit is immediate regardless of auto-start setting
- FR-7: If auto-start is enabled for the current repo, exit is immediate regardless of daemon running state
- FR-8: If the repo is not registered in the global config, or the global config fails to load, exit is immediate (no prompt)

## Non-Goals

- No changes to `q`/`esc` behavior in sub-states like the delete confirmation dialog or detail view — those sub-states close the sub-state first (existing behavior is correct)
- No changes to menu, work, list, or repos views
- No persistent setting to suppress the prompt — the condition (daemon running + auto-start disabled) is the natural gate
- No async stop — stop/kill is called synchronously before returning `tea.Quit`

## Technical Considerations

- `shouldPromptOnExit()` calls `globalconfig.Load()` at exit time (synchronously, once). This is a single YAML file read and is fast enough to not cause visible delay. Caching it at model init is unnecessary complexity.
- The absolute path comparison in `shouldPromptOnExit()` should use `filepath.Abs(m.dir)` — match the same pattern used in `autoStartDaemon()` in `daemon_start.go`
- The `handleQuitRequest()` helper centralizes the decision. All quit paths in `updateList()` that are user-initiated exits should call it. Programmatic quits (e.g. after a delete that empties the list) should still use `tea.Quit` directly — those are not user-initiated exits and the overlay would be confusing
- The stop/kill invocation in `updateExitDaemonOverlay` can share the same function used by feature 003's `daemonStopOverlay` handler — check whether a `stopDaemon(graceful bool)` helper is introduced there and reuse it; if not, call the stop/kill functions directly

## Success Metrics

- Exiting the status view when the daemon is running with auto-start disabled shows the one-line footer prompt
- Exiting when auto-start is enabled bypasses the prompt completely
- A user pressing `l` or `esc` in the overlay exits cleanly without touching the daemon

## Open Questions

*(none)*
