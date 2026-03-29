<!-- maggus-id: 165db6b6-aba2-400e-955c-ee3f1ce14785 -->
# Feature 029: Remove Foreground Work Mode (Runner TUI Cleanup)

## Introduction

The daemon is now the only execution path. The old foreground `maggus work` mode — which ran an interactive Bubble Tea TUI in the terminal while Claude Code executed — is dead code. This feature removes it entirely: ~6,000 LOC across 17 runner package files and several cmd files.

### Architecture Context

- **No new components** — pure deletion
- **Message types must be extracted first** — `internal/runner/tui_messages.go` defines types that `cmd/work_loop.go`, `cmd/work_task.go`, `cmd/daemon_keepalive.go`, and `cmd/daemon_tui.go` all reference. These types must move to `cmd/` before the runner package can be deleted.
- **`internal/runner/` package** — 17 files, ~5,000 LOC, entirely foreground-only
- **`internal/gitsync/` package** — KEEP (daemon still uses `gitsync.Pull`, `gitsync.RemoteStatus`, etc.)
- **`cmd/gitsync.go`** — DELETE (interactive sync TUI screen, foreground-only)
- **`cmd/work.go`** — KEEP the `--daemon-run` routing (daemon entry point), DELETE the foreground TUI setup block (lines ~87–320)

## Goals

- Delete `internal/runner/` entirely (~5,000 LOC)
- Delete all foreground-only cmd files: `gitsync.go`, `gitsync_test.go`, `workpid.go`, `work_sync.go`, `work_sync_test.go`
- Trim `work.go` to just flag setup + daemon delegation
- Keep `go build ./...` and `go test ./...` passing throughout

## Tasks

### TASK-029-001: Extract message types and format helpers out of runner package

**Description:** As a developer, I want the shared message types and formatting helpers to live in `cmd/` so the runner package can be deleted without breaking the daemon and work loop.

**Token Estimate:** ~55k tokens
**Predecessors:** none
**Successors:** TASK-029-002, TASK-029-003
**Parallel:** no — prerequisite for both follow-on tasks

**What to create:**

- **`cmd/work_messages.go`** — move all types from `internal/runner/tui_messages.go` that are used outside the runner package:
  - Message types: `ProgressMsg`, `CommitMsg`, `InfoMsg`, `IterationStartMsg`, `SummaryMsg`, `PushStatusMsg`, `QuitMsg`, `SyncCheckMsg`, `SyncCheckResult`, `SyncAction` + `SyncProceed`/`SyncAbort`/`SyncRestart` constants
  - Data types: `TaskCriterion`, `RemainingTask`, `FailedTask`, `SummaryData`, `StopReason` + all `StopReasonXxx` constants, `BannerInfo`, `TaskUsage`
  - Keep the exact same type names — do not rename anything

- **`cmd/format_helpers.go`** — move from `internal/runner/tui_tokens.go`:
  - `FormatTokens(n int) string`
  - `FormatCost(cost float64) string`

**Files to update (imports only — no logic changes):**
- `cmd/work_loop.go` — replace `runner.ProgressMsg`, `runner.SummaryMsg`, etc.
- `cmd/work_task.go` — replace `runner.CommitMsg`, `runner.InfoMsg`, `runner.IterationStartMsg`, `runner.SyncCheckMsg`, `runner.SyncCheckResult`, `runner.SyncProceed`, `runner.StopReason*`
- `cmd/daemon_keepalive.go` — replace `runner.InitSyncFuncs` call (just remove it — `nullTUIModel` auto-proceeds on SyncCheckMsg and does not use sync functions), replace `runner.TaskUsage`
- `cmd/daemon_tui.go` — replace `runner.QuitMsg`, `runner.SyncCheckMsg`, `runner.SyncCheckResult`, `runner.SyncProceed`, `runner.IterationStartMsg`, `runner.CommitMsg`, `runner.TaskUsage`
- `cmd/daemon_tui_test.go` — update imports
- `cmd/status_rightpane.go` — replace `runner.FormatTokens`, `runner.FormatCost`
- `cmd/status_metrics.go` — replace `runner.FormatTokens`, `runner.FormatCost`

**Acceptance Criteria:**
- [x] `cmd/work_messages.go` exists and contains all message/data types listed above
- [x] `cmd/format_helpers.go` exists and contains `FormatTokens` and `FormatCost`
- [x] All files listed under "Files to update" no longer import `internal/runner`
- [x] `internal/runner` package is NOT deleted yet — this task only moves types and updates imports
- [x] `go build ./...` passes
- [x] `go test ./...` passes

---

### TASK-029-002: Delete internal/runner package

**Description:** As a developer, I want the dead runner package removed so the codebase no longer contains 5,000 lines of unreachable foreground TUI code.

**Token Estimate:** ~25k tokens
**Predecessors:** TASK-029-001
**Successors:** none
**Parallel:** yes — can run alongside TASK-029-003

**Files to delete (entire package):**

| File | LOC |
|------|-----|
| `internal/runner/tui.go` | 485 |
| `internal/runner/tui_render.go` | 944 |
| `internal/runner/tui_keys.go` | 307 |
| `internal/runner/tui_sync.go` | 339 |
| `internal/runner/tui_summary.go` | 349 |
| `internal/runner/tui_messages.go` | 228 |
| `internal/runner/tui_tokens.go` | 145 |
| `internal/runner/tui_2x.go` | 31 |
| `internal/runner/runner.go` | 21 |
| `internal/runner/runonce.go` | 14 |
| `internal/runner/tui_render_test.go` | 156 |
| `internal/runner/tui_filechange_test.go` | 700 |
| `internal/runner/tui_tokens_test.go` | 365 |
| `internal/runner/tui_summary_test.go` | 70 |
| `internal/runner/tui_sync_test.go` | 222 |
| `internal/runner/tui_active_elapsed_test.go` | 211 |
| `internal/runner/runner_test.go` | 445 |

Also delete `internal/runner/procattr_windows.go` and `internal/runner/procattr_other.go` if they exist (OS-specific subprocess attrs used only by foreground runner).

**Acceptance Criteria:**
- [ ] The `internal/runner/` directory no longer exists
- [ ] `go build ./...` passes (no dangling imports)
- [ ] `go test ./...` passes

---

### TASK-029-003: Remove foreground cmd files and trim work.go

**Description:** As a developer, I want the foreground-only cmd code deleted so `maggus work` no longer attempts to run an interactive session.

**Token Estimate:** ~45k tokens
**Predecessors:** TASK-029-001
**Successors:** none
**Parallel:** yes — can run alongside TASK-029-002

**Files to delete entirely:**
- `cmd/gitsync.go` — interactive git sync Bubble Tea screen (foreground-only)
- `cmd/gitsync_test.go` — tests for above
- `cmd/workpid.go` — foreground mutual exclusion via work.pid file
- `cmd/workpid_test.go` — tests for above (if exists)
- `cmd/work_sync.go` — `checkSync()` pre-flight function called only by foreground RunE
- `cmd/work_sync_test.go` — tests for above

**`cmd/work.go` — trim RunE:**
- Keep: flag declarations, `resetWorkFlags()`, the `if daemonRunFlag` branch (daemon entry point), `findTaskByID()` helper, `init()` registration
- Delete: everything in RunE after the `if daemonRunFlag { return runDaemonLoop(...) }` block (lines ~87–320) — this is the foreground TUI setup: PID file writes, signal handling, branch setup, `runner.NewTUIModel(...)`, `tea.NewProgram(...)`, `p.Run()`
- After deletion, RunE should be ≤ 10 lines: setup → check daemon flag → delegate to daemon, or print a message and return if called without `--daemon-run`

**What to print when `maggus work` is called without `--daemon-run`:**
Print a short message: `"Use 'maggus start' to start the daemon."` and return nil. Do not error — graceful redirect.

**Acceptance Criteria:**
- [ ] `cmd/gitsync.go`, `cmd/gitsync_test.go`, `cmd/workpid.go`, `cmd/work_sync.go`, `cmd/work_sync_test.go` are deleted
- [ ] `cmd/work.go` RunE no longer references `runner.NewTUIModel`, `tea.NewProgram`, `p.Run()`, or any interactive TUI setup
- [ ] `maggus work` (without `--daemon-run`) prints a redirect message and exits cleanly
- [ ] `maggus work --daemon-run` (the internal daemon entry point) still works correctly
- [ ] `go build ./...` passes
- [ ] `go test ./...` passes

---

## Task Dependency Graph

```
TASK-029-001 ──→ TASK-029-002
             └─→ TASK-029-003
```

| Task | Estimate | Predecessors | Parallel | Model |
|------|----------|--------------|----------|-------|
| TASK-029-001 | ~55k | none | no | — |
| TASK-029-002 | ~25k | 001 | yes (with 003) | haiku |
| TASK-029-003 | ~45k | 001 | yes (with 002) | — |

**Total estimated tokens:** ~125k

## Functional Requirements

- FR-1: `go build ./...` must pass at every step (after 001 completes, after 002 completes, after 003 completes)
- FR-2: `go test ./...` must pass after all tasks complete
- FR-3: The daemon entry point (`maggus work --daemon-run`, hidden flag) must continue to function exactly as before
- FR-4: `maggus work` without `--daemon-run` must exit cleanly with a redirect message — not panic, not error
- FR-5: `internal/gitsync/` package must NOT be touched — daemon and `work_task.go` still use it directly

## Non-Goals

- Removing `maggus work` from the CLI help entirely (the hidden `--daemon-run` flag makes this the daemon entry point — keep the command registered)
- Refactoring `work_loop.go` or `work_task.go` beyond updating imports
- Removing `internal/filewatcher/` package (still used by daemon)
- Any changes to daemon behavior

## Technical Considerations

- `runner.InitSyncFuncs` in `daemon_keepalive.go` — this call can simply be deleted. `nullTUIModel` auto-returns `SyncProceed` on any `SyncCheckMsg` and never calls the injected sync functions.
- Deletion order within a task matters: update imports first, then delete files, then verify build.
- `procattr_windows.go` / `procattr_other.go` in runner — these may use OS build tags (`//go:build windows`). Check before deleting to avoid build tag issues; they can simply be deleted since the whole package goes.

## Verification

```bash
cd src && go build ./...
cd src && go test ./...
# Confirm the runner package directory is gone:
ls internal/runner/   # should fail
# Confirm daemon entry point works:
maggus work --daemon-run --help   # should not panic
```

## Open Questions

_(none)_
