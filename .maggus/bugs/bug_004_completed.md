<!-- maggus-id: d5db8a00-f302-4892-8c89-100d2cb64a08 -->
# Bug: maggus start fails on fresh projects after log refactoring removed runs dir creation

## Summary

After TASK-010-003 removed `os.MkdirAll(runDir)` from `daemon_start.go`, the `.maggus/runs/` directory is no longer guaranteed to exist before `daemon_start.go` tries to open `daemon.log` at `.maggus/runs/daemon.log`. On any project that has never had a successful maggus run, `maggus start` fails immediately with a file-not-found error. Work is never started.

## Related

- **Commit:** 6f66419 (feat(TASK-010-003): Wire maggusID through work loop and daemon; remove run directory creation)

## Steps to Reproduce

1. Create a fresh project directory with `.maggus/config.yml` and a feature file
2. Register it and approve a feature
3. Run `maggus start`
4. Observe error: `create daemon log: open .maggus/runs/daemon.log: no such file or directory`

## Expected Behavior

`maggus start` should always succeed regardless of whether `.maggus/runs/` exists yet. The runs directory should be created on demand before the daemon log is opened.

## Root Cause

In `daemon_start.go`, `startCurrentDaemon` (and `startDaemon`) open the daemon log at line 98:

```go
daemonLogPath := daemonLogPath(dir)  // = .maggus/runs/daemon.log
logFile, err := os.OpenFile(daemonLogPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
```

`os.OpenFile` with `O_CREATE` creates the *file* but not its *parent directory*. Before TASK-010-003, these functions called `os.MkdirAll(runDir)` where `runDir = ".maggus/runs/<runID>"`, which also created the parent `.maggus/runs/` as a side effect. TASK-010-003 removed that call, but did not add a replacement `MkdirAll` for `.maggus/runs/` itself.

The daemon subprocess (`runDaemonLoop`) does call `runlog.Open("", dir, ...)` which creates `.maggus/runs/` via `MkdirAll`. But this only runs *inside* the daemon subprocess — after the parent process has already failed to open `daemon.log`.

**Observed evidence:** Investigation of this project (which has prior runs, so `.maggus/runs/` already exists) shows 5 separate daemon sessions all attempted TASK-013-001 but were stopped mid-run via context cancellation before any commit was made. The feature is being detected and approved correctly. The runs directory issue would only surface on a truly fresh project.

## User Stories

### BUG-004-001: Ensure .maggus/runs/ exists before opening daemon.log in daemon_start.go

**Description:** As a user running `maggus start` for the first time on a new project, I want the daemon to start successfully so that approved features are worked on without requiring a prior `maggus work` run to bootstrap the directory.

**Acceptance Criteria:**
- [x] `startCurrentDaemon` calls `os.MkdirAll(filepath.Join(dir, ".maggus", "runs"), 0755)` (or equivalent) before opening `daemon.log`
- [x] `startDaemon` has the same fix applied
- [x] `maggus start` succeeds on a project where `.maggus/runs/` has never been created
- [x] Existing projects with `.maggus/runs/` already present are unaffected
- [x] `go vet ./...` and `go test ./...` pass
