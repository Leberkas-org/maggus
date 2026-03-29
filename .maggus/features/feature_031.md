<!-- maggus-id: b36793b5-2f4d-4d9f-8b5b-e9eebb179e5d -->
# Feature 031: Stop daemon after current task from status view

## Introduction

When the user presses `[s]` ("stop after task") in the daemon stop overlay inside the
status view, the daemon currently calls `stopDaemonGracefully` which sends SIGTERM / creates
a stop-file that cancels `workCtx` — interrupting the active Claude invocation mid-task.

This feature changes that behaviour so that "stop after task" truly means what it says:
the daemon finishes the task it is currently running, does **not** start the next task, and
then exits. The status view stays open and shows a "stopping after task…" indicator while
it waits; it returns to idle state when the daemon process exits.

A secondary fix: both stop overlays in the status view show `[s] stop after task` in the
footer but the handler actually listens for `y`/`Y`. The handler is corrected to match.

### Architecture Context

- **Components involved:** `cmd/daemon.go` (helpers), `cmd/daemon_stop.go` (new signal
  function), `cmd/daemon_keepalive.go` (poll goroutine), `cmd/status_model.go` (new field),
  `cmd/status_update.go` (overlay handlers), `cmd/status_view.go` (footer, header indicator)
- **New pattern introduced:** A second sentinel file (`daemon.stop-after-task`) distinct from
  `daemon.stop`. The daemon polls for it between tasks, sets `stopFlagAtomic`, and removes the
  file — without touching `workCtx`. This leaves the current task running to completion.

## Goals

- "Stop after task" in the daemon stop overlay finishes the running task before stopping.
- The status view stays open with a visible "stopping after task…" indicator.
- The status view returns to its normal idle state once the daemon exits.
- Fix the `y`/`Y` vs `[s]` key-binding mismatch in both daemon-related overlays.

## Tasks

### TASK-031-001: Add stop-after-task signal file mechanism to the daemon layer
**Description:** As a developer, I want the daemon to recognise a new
`daemon.stop-after-task` sentinel file so that it can finish its current task and then
stop, without the running Claude invocation being cancelled mid-execution.

**Token Estimate:** ~40k tokens
**Predecessors:** none
**Successors:** TASK-031-002
**Parallel:** no — TASK-031-002 depends on the API introduced here

**Acceptance Criteria:**
- [ ] `daemonStopAfterTaskFilePath(dir string) string` helper added to `cmd/daemon.go`,
  returning `<dir>/.maggus/daemon.stop-after-task`
- [ ] `removeStopAfterTaskFile(dir string)` helper added to `cmd/daemon.go` (mirrors
  `removeDaemonStopFile`)
- [ ] `sendStopAfterTaskSignal(dir string) error` function added to `cmd/daemon_stop.go`;
  it writes the PID (from `daemon.pid`) into the new sentinel file — same format as the
  existing stop-file on Windows — so the daemon can verify the signal is for itself
- [ ] In `runOneDaemonCycle` (`cmd/daemon_keepalive.go`), a goroutine polls
  `daemonStopAfterTaskFilePath` every 500 ms (same interval as the existing stop-file
  watcher); when the file is found it removes the file, sets `stopFlagAtomic` to `true`,
  and returns — it does **not** call `workCancel()`
- [ ] The polling goroutine exits cleanly when `workCtx` is cancelled
- [ ] `removeStopAfterTaskFile` is called at daemon startup (before the work loop) to clean
  up any leftover file from a previous run — mirrors how `removeDaemonStopFile` is called
- [ ] `go build ./...` passes with no errors

---

### TASK-031-002: Update status view overlays — fix key bindings and show stopping indicator
**Description:** As a user, I want the status view's "stop after task" button to actually
send the new stop-after-task signal, and I want to see a visible indicator while the daemon
is finishing its current task, so I know it received my request.

**Token Estimate:** ~50k tokens
**Predecessors:** TASK-031-001
**Successors:** none
**Parallel:** no

**Acceptance Criteria:**
- [ ] `statusModel` gains a `daemonStoppingAfterTask bool` field (in `status_model.go`)
- [ ] In `updateStatusDaemonStopOverlay` (`status_update.go`):
  - `s`/`S` (was `y`/`Y`) → calls `sendStopAfterTaskSignal(dir)`, sets
    `m.daemonStoppingAfterTask = true`, closes the overlay, stays in the status view
  - `k`/`K`/`ctrl+c` key handling is unchanged (immediate kill)
  - `esc` handling is unchanged (cancel overlay)
  - Old `y`/`Y` case is removed (it was a dead binding — the footer always showed `[s]`)
- [ ] In `updateExitDaemonOverlay` (`status_update.go`):
  - `s`/`S` (was `y`/`Y`) → calls `sendStopAfterTaskSignal(dir)`, then quits the view
    (daemon will stop in background after the current task)
  - `k`/`K`/`ctrl+c` is unchanged
  - `d`/`D` is unchanged (disconnect without stopping daemon)
  - `esc`/`q`/`Q` is unchanged
  - Old `y`/`Y` case is removed
- [ ] When `daemonCacheUpdateMsg` arrives and `daemonStoppingAfterTask` is `true` and
  `msg.State.Running` is `false`, `daemonStoppingAfterTask` is reset to `false`
- [ ] `statusSplitFooter` (`status_view.go`) renders an amber/warning "Stopping after
  task…" line when `daemonStoppingAfterTask` is `true` and no overlay is active — placed
  above the normal footer hint so it is clearly visible
- [ ] The left-pane daemon status line (`status_leftpane.go` or equivalent) shows a
  `(stopping after task)` annotation next to the daemon running indicator while
  `daemonStoppingAfterTask` is `true`
- [ ] `go build ./...` passes with no errors

## Task Dependency Graph

```
TASK-031-001 ──→ TASK-031-002
```

| Task | Estimate | Predecessors | Parallel | Model |
|------|----------|--------------|----------|-------|
| TASK-031-001 | ~40k | none | no | — |
| TASK-031-002 | ~50k | 001 | no | — |

**Total estimated tokens:** ~90k

## Functional Requirements

- FR-1: Pressing `[s]` in the daemon stop overlay MUST send the stop-after-task signal, NOT
  the existing SIGTERM/stop-file signal that cancels `workCtx`.
- FR-2: After `[s]` is pressed the status view MUST remain open.
- FR-3: The status view MUST display a clear "stopping after task…" indicator for as long as
  `daemonStoppingAfterTask` is true.
- FR-4: When the daemon process exits (daemon cache reports `Running: false`) and
  `daemonStoppingAfterTask` is true, the indicator MUST be cleared automatically.
- FR-5: Pressing `[s]` in the exit overlay MUST send the stop-after-task signal and then
  close the status view (daemon finishes in background).
- FR-6: The `daemon.stop-after-task` sentinel file MUST be removed at daemon startup to
  prevent stale signals from a previous run affecting the new cycle.
- FR-7: `[k] / ctrl+c` (kill now) behaviour in both overlays MUST remain unchanged.

## Non-Goals

- No changes to the `maggus stop` CLI command — it continues to send the existing graceful
  signal (SIGTERM / stop-file) which cancels `workCtx`.
- No changes to how the work loop's `stopFlag` is interpreted — only the signal source changes.
- No visual redesign of the overlay or footer beyond the indicator addition.
- No timeout: if the current task never finishes, the daemon never stops. That is intentional —
  the user can always press `[k]` to kill immediately.

## Technical Considerations

- `sendStopAfterTaskSignal` writes the sentinel file; it does **not** send SIGTERM. On Windows
  the existing `sendGracefulSignal` already uses a file; this new function adds a second, distinct
  file path so the daemon can distinguish "stop now" from "stop after task".
- The new polling goroutine in `runOneDaemonCycle` lives alongside the existing stop-file
  watcher goroutine. They are independent: one calls `workCancel()`, the other sets `stopFlagAtomic`.
- `stopFlagAtomic` is already checked in `runGroupTasks` at `innerI > 0` — between tasks, not
  mid-task. Setting it from the new goroutine is therefore safe and will naturally produce
  "stop after current task" semantics.
- The `daemonStoppingAfterTask` flag on the model should survive overlay dismissal (i.e. it is
  set when the signal is sent and only cleared when the daemon actually exits, not on `esc`).

## Success Metrics

- Pressing `[s]` in the daemon stop overlay allows the current Claude invocation to run to
  completion before the daemon exits.
- The status view shows the "stopping after task…" indicator immediately after `[s]` is pressed
  and clears it automatically once `daemon.Running` becomes false.
- `go build ./...` and `go test ./...` pass without modification.

## Open Questions

_(none — all resolved before saving)_
