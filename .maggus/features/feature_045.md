<!-- maggus-id: b0d528c9-21ad-4004-acfd-0794d60ce720 -->
# Feature 045: Manual Task Dispatch as Parallel Worker

## Introduction

Add the ability to manually dispatch a bug or task from the status TUI while the daemon is already working. The dispatched task runs as a parallel worker in its own worktree and branch — same infrastructure as parallel mode — without interrupting or waiting for the daemon's current work. This is user-initiated only, never automatic.

Currently, `Alt+R` in the status view runs a task by launching `maggus run --task <id>` as a foreground process, which takes over the TUI and requires the daemon to be stopped. The new dispatch mechanism spawns the task as an additional background worker alongside the running daemon.

### Architecture Context

- **Vision alignment:** "Human control over what the agent is allowed to work on" — manual dispatch gives users fine-grained control over what runs when
- **Components involved:**
  - `cmd/status_update.go` — key handling for dispatch action
  - `cmd/dispatch.go` — existing dispatch mechanism (runs `maggus run --task`)
  - `cmd/run_parallel.go` — parallel worker infrastructure (worktree creation, branch management, agent invocation)
  - `cmd/daemon_keepalive.go` — daemon work loop (needs to accept externally-spawned workers)
  - `internal/gitbranch` — branch creation for the dispatched task
  - `internal/gitworktree` — worktree creation for isolation
- **New patterns:** The daemon currently owns all worker lifecycle. This introduces externally-triggered workers that run alongside the daemon's own work.

## Goals

- Users can dispatch any task from the status TUI while the daemon is running, without stopping it
- The dispatched task runs in its own worktree/branch, isolated from the daemon's current work
- The dispatched worker appears in the status TUI alongside daemon workers (same worker pane)
- The daemon is unaware of the dispatch — it continues its own work; the dispatched task is a separate process

## User Stories

### TASK-045-001: Implement background task dispatch command
**Description:** As a developer, I want a mechanism to spawn a single task as an isolated background worker (in its own worktree and branch) so the TUI can trigger it without stopping the daemon.

**Token Estimate:** ~60k tokens
**Predecessors:** none
**Successors:** TASK-045-002, TASK-045-003
**Parallel:** no — foundation task
**Model:** opus

**Acceptance Criteria:**
- [x] A new function `dispatchTask(dir, taskID, model, agent string) error` is created (in `cmd/dispatch.go` or a new file)
- [x] The function: creates a worktree at `.maggus/worktrees/<taskID>/`, creates a task branch (using the new hierarchical naming from feature 043 if available, else current naming), launches `maggus run --task <taskID> --daemon-run` as a detached background process with stdout/stderr redirected to a log file
- [x] The dispatched process writes its own per-worker state file (`state-<taskID>.json`) so the status TUI can track it
- [x] The dispatched process registers itself in the workers index file so it appears alongside daemon workers
- [x] If the worktree already exists (e.g., from a previous interrupted dispatch), the function cleans it up first or reuses it
- [x] The function returns immediately after spawning — it does not wait for the task to complete
- [x] `go vet ./...` and `go test ./...` pass

### TASK-045-002: Wire up dispatch shortcut in status TUI
**Description:** As a user viewing the status TUI, I want to press a key to dispatch the selected task as a parallel worker so I can start urgent bugs without stopping the daemon.

**Token Estimate:** ~35k tokens
**Predecessors:** TASK-045-001
**Successors:** TASK-045-004
**Parallel:** yes — can run alongside TASK-045-003

**Acceptance Criteria:**
- [ ] Pressing `Alt+R` on a task while the daemon is running triggers `dispatchTask()` instead of the current foreground `maggus run --task` behavior
- [ ] Pressing `Alt+R` while the daemon is NOT running keeps the current foreground behavior (launch `maggus run --task` via `tea.ExecProcess`)
- [ ] A status note shows "Dispatched <TASK-ID>" briefly after dispatch
- [ ] If the task is already running (in the worker index), `Alt+R` shows "Task already running" and does nothing
- [ ] If the task is complete or blocked, `Alt+R` does nothing
- [ ] Footer hint updates to show `alt+r: dispatch` when daemon is running (was `alt+r: run`)
- [ ] `go vet ./...` and `go test ./...` pass

### TASK-045-003: Ensure dispatched workers appear in status TUI worker view
**Description:** As a user, I want dispatched tasks to appear in the status view alongside daemon workers so I can monitor all running work in one place.

**Token Estimate:** ~30k tokens
**Predecessors:** TASK-045-001
**Successors:** TASK-045-004
**Parallel:** yes — can run alongside TASK-045-002

**Acceptance Criteria:**
- [ ] The dispatched process writes to the same `state-workers.json` index that the parallel orchestrator uses
- [ ] `refreshWorkerSnapshots()` in `status_workers.go` picks up dispatched workers alongside daemon workers
- [ ] The worker pane shows dispatched tasks with the same card format (spinner, task ID, tool, tokens, elapsed)
- [ ] When a dispatched task completes, its worker card shows `✓` (done) or `✗` (failed)
- [ ] After completion, the worktree is cleaned up automatically (or flagged for `maggus clean`)
- [ ] `go vet ./...` and `go test ./...` pass

### TASK-045-004: Handle merge-back and cleanup for dispatched workers
**Description:** As a developer, I want dispatched tasks to merge their changes back to the plan branch and clean up their worktree so the result integrates cleanly.

**Token Estimate:** ~40k tokens
**Predecessors:** TASK-045-002, TASK-045-003
**Successors:** none
**Parallel:** no
**Model:** opus

**Acceptance Criteria:**
- [ ] When the dispatched task completes successfully, its commits are merged back to the plan branch (same merge strategy as parallel orchestrator's `runSingleTask`)
- [ ] If the merge has conflicts, the task is marked as failed and the worktree is preserved for manual resolution
- [ ] On success, the worktree is removed and the task branch is deleted
- [ ] On failure, the worktree is preserved and a status note indicates manual intervention needed
- [ ] The daemon's work loop is not affected by merge-back — it continues independently
- [ ] `go vet ./...` and `go test ./...` pass

## Task Dependency Graph

```
TASK-045-001 ──→ TASK-045-002 ──┐
             ──→ TASK-045-003 ──├──→ TASK-045-004
                                │
```

| Task | Estimate | Predecessors | Parallel | Model |
|------|----------|--------------|----------|-------|
| TASK-045-001 | ~60k | none | no | opus |
| TASK-045-002 | ~35k | 001 | yes (with 003) | — |
| TASK-045-003 | ~30k | 001 | yes (with 002) | — |
| TASK-045-004 | ~40k | 002, 003 | no | opus |

**Total estimated tokens:** ~165k

## Functional Requirements

- FR-1: `Alt+R` on a task in the status TUI dispatches it as a parallel background worker when the daemon is running
- FR-2: The dispatched task runs in an isolated worktree with its own branch
- FR-3: The dispatched worker appears in the status TUI worker view with live status updates
- FR-4: On completion, changes are merged back to the plan branch and the worktree is cleaned up
- FR-5: The daemon continues its own work undisturbed — dispatch is fully independent
- FR-6: Only the user can trigger dispatch — it is never automatic
- FR-7: Dispatching an already-running, complete, or blocked task is a no-op with a status message

## Non-Goals

- No automatic dispatch based on priority or urgency
- No queueing multiple dispatches — each dispatch is immediate and independent
- No changes to the daemon's own task selection logic
- No dispatch from the CLI (this is TUI-only for now)

## Technical Considerations

- The dispatched process is a separate `maggus run --task <id> --daemon-run` process, not a goroutine inside the daemon. This ensures full isolation — if it crashes, the daemon is unaffected
- The worker index file (`state-workers.json`) needs concurrent-write safety since both the daemon and the dispatched process may write to it. Use file-level locking or append-only semantics
- The merge-back step is the trickiest part — the parallel orchestrator already has this logic in `runSingleTask`. Extract it into a shared function rather than duplicating
- If feature 043 (hierarchical branches) is not yet merged, use the current branch naming. The dispatch mechanism is branch-naming-agnostic

## Open Questions

None.
