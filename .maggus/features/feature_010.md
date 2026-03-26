<!-- maggus-id: 788e26c6-2531-4110-8501-198e5a219975 -->
# Feature 010: Per-Plan Flat Log Files with Rotation

## Introduction

Currently Maggus writes run logs into a per-run subdirectory:
`.maggus/runs/<runID>/run.log`. This creates an ever-growing tree of directories
that is hard to navigate and provides no built-in cleanup. The `runs/` folder
also contains an unrelated `daemon.log` at its root, making the structure
inconsistent.

This feature replaces the per-run-directory scheme with a flat, per-plan log
file named `<timestamp>_<maggus_id>.log` placed directly in `.maggus/runs/`.
A configurable retention limit automatically deletes the oldest log files when
the limit is exceeded, so the folder stays bounded without any manual cleanup.
The `daemon.log` file is not affected and remains at `.maggus/runs/daemon.log`.

## Goals

- One log file per feature/bug plan execution, named `<timestamp>_<maggus_id>.log`
- All log files live flat in `.maggus/runs/` — no subdirectories per run
- Oldest log files are pruned automatically when the count exceeds the configured limit
- Default retention limit is 50 log files (configurable via `.maggus/config.yml`)
- `daemon.log` is preserved as-is and excluded from the rotation count
- The run directory creation code in `daemon_start.go` is removed (no longer needed)

## Tasks

### TASK-010-001: Add `max_log_files` to config
**Description:** As a user, I want to configure the maximum number of log files
Maggus retains so that the `.maggus/runs/` folder stays bounded.

**Token Estimate:** ~15k tokens
**Predecessors:** none
**Successors:** TASK-010-002
**Parallel:** yes — can start immediately

**Acceptance Criteria:**
- [x] `Config` struct in `src/internal/config/config.go` has a new `MaxLogFiles int` field with YAML tag `max_log_files`
- [x] A method `LogMaxFiles() int` on `Config` returns the configured value, or `50` if the field is zero or negative (i.e. 50 is the default)
- [x] `go vet ./...` passes

---

### TASK-010-002: Refactor `runlog.Open` for flat per-plan log files with automatic pruning
**Description:** As a developer, I want `runlog.Open` to write log files directly
into `.maggus/runs/` with a `<timestamp>_<maggus_id>.log` name, and to prune
the oldest such files when the retention limit is exceeded.

**Token Estimate:** ~70k tokens
**Predecessors:** TASK-010-001
**Successors:** TASK-010-003
**Parallel:** no

**Acceptance Criteria:**
- [x] `runlog.Open` signature changes from `Open(runID, dir string)` to `Open(maggusID, dir string, maxFiles int)`
- [x] The log file is written to `.maggus/runs/<timestamp>_<maggusID>.log` where timestamp uses format `20060102-150405`
- [x] If `maggusID` is empty (plan file has no `<!-- maggus-id: ... -->`), a timestamp-only name is used: `<timestamp>.log`
- [x] No subdirectory is created under `runs/`; `os.MkdirAll` targets `.maggus/runs/` directly
- [x] After successfully opening the new log file, the pruning logic runs:
  - Scans `.maggus/runs/` for files matching the pattern `*_*.log` (underscore between timestamp and maggus_id)
  - Files named `daemon.log` are excluded from pruning entirely
  - Files are sorted by name ascending (oldest timestamp first)
  - If the count exceeds `maxFiles`, the oldest files are deleted until the count equals `maxFiles`
  - Pruning errors are non-fatal (log to stderr if deletion fails, continue)
- [x] The `Logger` struct no longer stores `runID` (it is now derived from the filename and not needed at runtime); `dir` still stored for potential future use
- [x] All existing tests in `src/internal/runlog/runlog_test.go` are updated to use the new `Open(maggusID, dir, maxFiles)` signature
- [x] New tests cover: pruning removes correct files, `daemon.log` is never pruned, empty `maggusID` fallback name, log file written to flat path (no subdirectory)
- [x] `go test ./internal/runlog` passes
- [x] `go vet ./...` passes

---

### TASK-010-003: Wire `maggusID` through work loop and daemon; remove run directory creation
**Description:** As a developer, I want every call site of `runlog.Open` to pass
the plan's `MaggusID` and the configured `maxFiles`, and for the now-unnecessary
run-directory creation code to be removed.

**Token Estimate:** ~50k tokens
**Predecessors:** TASK-010-002
**Successors:** none
**Parallel:** no

**Acceptance Criteria:**
- [ ] `cmd/work.go`: the `runlog.Open(runID, dir)` call is replaced with `runlog.Open(plan.MaggusID, dir, cfg.LogMaxFiles())`, where `plan` is the currently selected plan and `cfg` is the loaded config
- [ ] `cmd/daemon_keepalive.go`: the `runlog.Open(runID, dir)` call is replaced similarly — passing the active plan's `MaggusID` and `cfg.LogMaxFiles()`; if the plan is not yet known at open-time, open the logger lazily once the plan is selected, or use an empty string for `maggusID`
- [ ] `cmd/daemon_start.go`: the `runDir` variable and its `os.MkdirAll(runDir, 0755)` call are removed from both `startCurrentDaemon` and `startDaemon`; the `runID` variable is also removed from `startCurrentDaemon` if it is no longer used
- [ ] `cmd/daemon.go`: `generateDaemonRunID()` is removed if it has no remaining callers; `daemonLogPath()` is unchanged (daemon.log stays at `.maggus/runs/daemon.log`)
- [ ] No remaining caller passes a bare `runID` string (e.g. `"20060102-150405"`) as the first argument to `runlog.Open`
- [ ] `go build ./...` passes
- [ ] `go test ./...` passes

## Task Dependency Graph

```
TASK-010-001 ──→ TASK-010-002 ──→ TASK-010-003
```

| Task | Estimate | Predecessors | Parallel | Model |
|------|----------|--------------|----------|-------|
| TASK-010-001 | ~15k | none | yes | haiku |
| TASK-010-002 | ~70k | 001 | no | — |
| TASK-010-003 | ~50k | 002 | no | — |

**Total estimated tokens:** ~135k

## Functional Requirements

- FR-1: Each time Maggus works on a plan (feature or bug), it opens exactly one log file at `.maggus/runs/<timestamp>_<maggus_id>.log`
- FR-2: If the plan has no `<!-- maggus-id: ... -->` frontmatter, the log file is named `<timestamp>.log`
- FR-3: After opening each new log file, Maggus counts all `*_*.log` files in `.maggus/runs/` (excluding `daemon.log`) and deletes the oldest ones until at most `max_log_files` remain
- FR-4: `max_log_files` is read from `.maggus/config.yml` under the key `max_log_files`; the default is 50
- FR-5: `daemon.log` at `.maggus/runs/daemon.log` is never touched by the rotation logic
- FR-6: No per-run subdirectories are created under `.maggus/runs/`

## Non-Goals

- Changing the format of log entries (JSONL structure stays the same)
- Rotating `daemon.log`
- Compressing or archiving old log files
- Exposing a `maggus logs` command to browse log files
- Migrating or converting existing run directories (old `runs/<runID>/` folders are left in place; the user can delete them manually)

## Technical Considerations

- The pruning sort uses filename lexicographic order, which is equivalent to chronological order because the timestamp prefix (`20060102-150405`) sorts correctly as a string
- `daemon_start.go` currently creates a `runDir` before launching the daemon process. This directory was only needed because `runlog.Open` previously created files inside it. After this change, only `.maggus/runs/` (the parent) needs to exist, which `runlog.Open` will ensure via `os.MkdirAll`
- `generateDaemonRunID()` in `daemon.go` is only called from `daemon_start.go`. If all callers are removed, delete the function to avoid dead code
- The `--daemon-run-id` flag passed to the `work` subprocess is still used for other purposes (check `cmd/work.go` to confirm before removing)

## Success Metrics

- After 60 `maggus work` runs, `.maggus/runs/` contains exactly 50 `*_*.log` files and no subdirectories (apart from any pre-existing ones)
- `daemon.log` is present and untouched alongside the log files
- Log file names are human-readable and sort chronologically in any file explorer

## Open Questions

_(none)_
