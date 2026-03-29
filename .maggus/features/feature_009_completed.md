<!-- maggus-id: 1b619147-69f7-4b37-be9d-d0569ce565a0 -->
# Feature 009: Move daemon.log to shared parent directory

## Introduction

`daemon.log` is the OS-level stdout/stderr redirect for the detached daemon subprocess — it captures panics, crashes, and any unstructured output the daemon process writes. Currently it lives inside the per-run directory (`.maggus/runs/<runID>/daemon.log`), which means each daemon start creates a new empty log file buried inside a run-specific folder.

Moving it to `.maggus/runs/daemon.log` makes it a single persistent log that appends across all daemon runs, easier to find and tail.

Additionally, the `OpenDaemonLog()` method on `Logger` in `runlog.go` is dead code — it opens daemon.log via the Logger struct but is never called anywhere in the codebase (the OS-level redirect handles it instead). It should be removed.

## Goals

- Move daemon.log to `.maggus/runs/daemon.log` (shared, persistent, not per-run)
- Remove dead `OpenDaemonLog()` method and its tests from the runlog package

## Tasks

### TASK-009-001: Move daemon.log path and remove dead OpenDaemonLog method
**Description:** As a developer, I want daemon.log to live at `.maggus/runs/daemon.log` so that all daemon output is in one predictable location, and I want the unused `OpenDaemonLog()` method removed so the codebase is clean.

**Token Estimate:** ~20k tokens
**Predecessors:** none
**Successors:** none
**Parallel:** N/A — single task

**Acceptance Criteria:**
- [x] `daemonLogPathFor(dir, runID string)` in `src/cmd/daemon.go` is replaced with `daemonLogPath(dir string)` returning `.maggus/runs/daemon.log`
- [x] Both call sites in `src/cmd/daemon_start.go` (lines ~101 and ~231) are updated to call `daemonLogPath(dir)`
- [x] The `Long` description string in `src/cmd/daemon_start.go` is updated from `.maggus/runs/<RUN_ID>/daemon.log` to `.maggus/runs/daemon.log`
- [x] The `OpenDaemonLog()` method (lines 60–73) is removed from `src/internal/runlog/runlog.go`
- [x] `TestOpenDaemonLog_CreatesFile` and `TestOpenDaemonLog_NilLogger` tests are removed from `src/internal/runlog/runlog_test.go`
- [x] `go build ./...` passes
- [x] `go test ./...` passes

## Task Dependency Graph

```
TASK-009-001
```

| Task | Estimate | Predecessors | Parallel | Model |
|------|----------|--------------|----------|-------|
| TASK-009-001 | ~20k | none | N/A | — |

**Total estimated tokens:** ~20k

## Functional Requirements

- FR-1: `maggus start` must write the daemon process stdout/stderr to `.maggus/runs/daemon.log` (not a per-run subdirectory)
- FR-2: The path printed after `maggus start` (`"Logs: <path>"`) must reflect the new location
- FR-3: The `OpenDaemonLog()` method must not exist in the runlog package

## Non-Goals

- No changes to `run.log` (structured JSONL per-run logging is unchanged)
- No changes to `state.json` or any other run-directory files
- No rotation or truncation of daemon.log — append-only is fine

## Technical Considerations

- The `.maggus/runs/` directory is guaranteed to exist at the point daemon.log is opened because `os.MkdirAll(runDir, 0755)` (which creates `.maggus/runs/<runID>/`) runs just before — no extra directory creation needed
- The file is opened with `O_APPEND` so multiple daemon start/stop cycles accumulate in the same log

## Open Questions

— none
