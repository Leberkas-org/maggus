# Feature 001: Feature-Centric Work Loop with Approval Flow & Daemon Mode

## Introduction

Rework the execution model from task-count-based runs to a feature-centric continuous loop. Maggus should work a feature to completion, then move to the next approved feature automatically (when configured). Users control which features are worked on via an approval mechanism, and a long-running daemon mode enables background operation. A live log view is integrated into the existing TUI status screen.

### Architecture Context

- **Components touched:** `cmd/work.go`, `cmd/work_loop.go`, `internal/config`, `cmd/status.go`, `internal/runner`, `internal/parser`
- **New components:** `internal/approval` (feature approval state), `internal/runlog` (structured log), new `start`/`stop` commands
- **New patterns:** daemon PID file management, feature-level loop (wrapping the existing task-level loop), structured append-only run log

## Goals

- Work a full feature to completion before moving to the next
- Gate feature execution behind a configurable approval state (opt-in by default)
- Run maggus as a background daemon (`maggus start` / `maggus stop`)
- Write a structured run log that can be tailed externally
- Show a live log view inside the existing `maggus status` TUI without needing a separate terminal

## Tasks

### TASK-001-001: Extend config for approval mode and auto-continue
**Description:** As a developer, I want config options that control approval and continuation behavior so that I can tune maggus to my workflow.

**Token Estimate:** ~20k tokens
**Predecessors:** none
**Successors:** TASK-001-002, TASK-001-003
**Parallel:** yes — can run alongside anything with no predecessors
**Model:** haiku

**Acceptance Criteria:**
- [x] `Config` struct gains `ApprovalMode string` field (`"opt-in"` | `"opt-out"`, default `"opt-in"`)
- [x] `Config` struct gains `AutoContinue *bool` field (default `false` — stop after each feature completes)
- [x] Accessor `IsApprovalRequired() bool` returns true when opt-in
- [x] Accessor `IsAutoContinueEnabled() bool` returns true when auto_continue is true
- [x] Existing config tests updated; new tests cover both fields and their defaults
- [x] Typecheck/lint passes
- [x] Unit tests are written and successful

### TASK-001-002: Feature approval state store + approve/unapprove commands
**Description:** As a user, I want to approve features for execution and revoke approval so that maggus only works on features I've explicitly cleared.

**Token Estimate:** ~60k tokens
**Predecessors:** TASK-001-001
**Successors:** TASK-001-003
**Parallel:** no

**Acceptance Criteria:**
- [x] New package `internal/approval` with `Load`, `Save`, `Approve`, `Unapprove`, `IsApproved` functions
- [x] Approval state stored in `.maggus/feature_approvals.yml` (feature ID → approved bool)
- [x] `maggus approve <feature-id>` marks a feature approved; when called with no argument it shows an interactive picker listing unapproved features
- [x] `maggus unapprove <feature-id>` revokes approval; when called with no argument it shows an interactive picker listing approved features
- [x] Interactive picker uses arrow keys to select, Enter to confirm, Esc/q to cancel
- [x] Both commands print confirmation of the change
- [x] When `approval_mode: opt-out`, `IsApproved` returns `true` for all features by default
- [x] When `approval_mode: opt-in`, `IsApproved` returns `false` unless explicitly approved
- [x] `feature_approvals.yml` is added to `.gitignore` entries managed by `internal/gitignore`
- [x] Typecheck/lint passes
- [x] Unit tests are written and successful

### TASK-001-003: Rework work loop to be feature-centric
**Description:** As a user, I want maggus to complete an entire feature before moving on so that work is coherent and reviewable at feature boundaries.

**Token Estimate:** ~90k tokens
**Predecessors:** TASK-001-001, TASK-001-002
**Successors:** TASK-001-004, TASK-001-005
**Parallel:** no
**Model:** opus

**Acceptance Criteria:**
- [ ] Work loop groups tasks by feature file and processes one feature at a time
- [ ] Only features that pass `approval.IsApproved()` are eligible for execution
- [ ] After all tasks in a feature complete, the feature is marked done (existing rename/delete logic preserved)
- [ ] If `auto_continue: false` (default), maggus stops after the first feature completes
- [ ] If `auto_continue: true`, maggus moves to the next approved feature without stopping
- [ ] `--count` flag is repurposed to mean "number of features to work on" (not tasks); `0` means all
- [ ] `--task` flag continues to work for targeting a specific task within the current feature
- [ ] No approved features available → prints clear message and exits cleanly
- [ ] Existing TUI progress display updated to show "Feature N/M, Task X/Y"
- [ ] Typecheck/lint passes
- [ ] Unit tests are written and successful

### TASK-001-004: Structured run log file
**Description:** As a user, I want a log file per run so that I can tail it from another terminal or inspect it after the fact.

**Token Estimate:** ~40k tokens
**Predecessors:** TASK-001-003
**Successors:** TASK-001-006
**Parallel:** yes — can run alongside TASK-001-005

**Acceptance Criteria:**
- [ ] New package `internal/runlog` with `Open(runID, dir) (Logger, error)` and `Close()`
- [ ] Structured log written to `.maggus/runs/<RUN_ID>/run.log`
- [ ] Log entries are plain timestamped lines (RFC3339 prefix), e.g. `2026-03-23T14:05:01Z [INFO] Feature 003 started`
- [ ] Events logged: feature start, task start, task complete (with commit hash), task failed, feature complete, agent stdout summary lines (tool use events)
- [ ] Logger is injected into the work loop via `taskContext` or equivalent
- [ ] Full agent stdout/stderr additionally streamed to `.maggus/runs/<RUN_ID>/daemon.log` when running in daemon mode
- [ ] `run.log`, `daemon.log`, and the runs directory remain gitignored (already handled by existing gitignore logic)
- [ ] Typecheck/lint passes
- [ ] Unit tests are written and successful

### TASK-001-005: Daemon mode — maggus start / maggus stop
**Description:** As a user, I want to run maggus in the background so that it works unattended while I do other things.

**Token Estimate:** ~85k tokens
**Predecessors:** TASK-001-003, TASK-001-004
**Successors:** none
**Parallel:** yes — can run alongside TASK-001-006

**Acceptance Criteria:**
- [ ] `maggus start` launches the feature-centric work loop as a detached background process
- [ ] PID written to `.maggus/daemon.pid` on start; removed on clean exit
- [ ] `maggus start` when a daemon is already running prints an error and exits non-zero
- [ ] `maggus stop` reads `.maggus/daemon.pid`, sends graceful shutdown signal, waits up to 10s, then force-kills
- [ ] `maggus stop` when no daemon is running prints a clear message and exits cleanly
- [ ] Full agent stdout/stderr streamed to `.maggus/runs/<current-RUN_ID>/daemon.log` (written by the daemon process itself)
- [ ] `maggus start` supports `--model` and `--agent` flags (same as `work`)
- [ ] Cross-platform: Windows uses `taskkill` for force-kill, Unix uses SIGKILL as fallback after SIGTERM
- [ ] `daemon.pid` is added to `.gitignore` entries managed by `internal/gitignore`
- [ ] Typecheck/lint passes

### TASK-001-006: Enhanced status TUI with integrated live log view
**Description:** As a user, I want to see the live agent output in the status TUI so that I don't need a separate terminal to monitor what maggus is doing.

**Token Estimate:** ~75k tokens
**Predecessors:** TASK-001-004
**Successors:** none
**Parallel:** yes — can run alongside TASK-001-005

**Acceptance Criteria:**
- [ ] `maggus status` gains a "Live Log" panel showing the last N lines of the active run's `run.log`
- [ ] Log panel auto-scrolls as new lines appear (tail the file using fsnotify or 200ms poll)
- [ ] If no daemon is running, the log panel shows "No active run"
- [ ] Feature progress (feature N/M, task X/Y) shown in the status header
- [ ] Panel is scrollable with arrow keys or `j`/`k`
- [ ] Daemon status (running/stopped, PID, current feature) shown in the status header
- [ ] Typecheck/lint passes

## Task Dependency Graph

```
TASK-001-001 ──→ TASK-001-002 ──→ TASK-001-003 ──→ TASK-001-004 ──→ TASK-001-006
                                              └──→ TASK-001-005
```

| Task | Estimate | Predecessors | Parallel | Model |
|------|----------|--------------|----------|-------|
| TASK-001-001 | ~20k | none | yes | haiku |
| TASK-001-002 | ~60k | 001 | no | — |
| TASK-001-003 | ~90k | 001, 002 | no | opus |
| TASK-001-004 | ~40k | 003 | yes (with 005) | — |
| TASK-001-005 | ~85k | 003, 004 | yes (with 006) | — |
| TASK-001-006 | ~75k | 004 | yes (with 005) | — |

**Total estimated tokens:** ~370k

## Functional Requirements

- FR-1: When `approval_mode: opt-in` (default), a feature must be explicitly approved before maggus will work on it
- FR-2: When `approval_mode: opt-out`, all features are worked on unless explicitly unapproved
- FR-3: `maggus approve` accepts an optional feature ID argument; when omitted an interactive picker lists unapproved features; `maggus unapprove` works the same way
- FR-4: The work loop processes all tasks within one feature before evaluating whether to continue to the next
- FR-5: When `auto_continue: false`, maggus exits after completing one feature (or when interrupted)
- FR-6: When `auto_continue: true`, maggus proceeds to the next approved feature without user interaction
- FR-7: `maggus start` launches a background daemon; `maggus stop` terminates it gracefully
- FR-8: Every run writes structured events to `.maggus/runs/<RUN_ID>/run.log`; daemon mode additionally streams full agent output to `daemon.log`
- FR-9: `maggus status` shows daemon state, current feature/task progress, and a live-scrolling log panel

## Non-Goals

- No web UI or HTTP API — all interaction is CLI and file-based
- No multi-repo or multi-project daemon — one daemon per working directory
- No task-level approval — approval granularity is the feature, not individual tasks
- No persistent notification service — sound notifications remain as-is

## Technical Considerations

- Windows daemon: use `os.StartProcess` with `DETACHED_PROCESS` flag (SysProcAttr); PID file approach is the same across platforms
- Log tailing in TUI: prefer `fsnotify` (already a transitive dep via Bubbletea) over polling for responsiveness
- The `--count` flag semantic change (tasks → features) is a breaking change; document in release notes
- Feature ID for approval commands should match the base filename without extension (e.g. `feature_003`, not the full path)
- `.maggus/feature_approvals.yml` and `.maggus/daemon.pid` should be gitignored since they are local workflow state
