<!-- maggus-id: 391986bd-9748-4902-9c26-a0d19ec43dfb -->
# Feature 050: Restructure Run Logging to Per-Feature Directories

## Introduction

Restructure the run logging system from flat files in `.maggus/runs/` to a per-feature directory structure at `.maggus/logs/<maggus_id>/<pid>.log`. This solves recurring TUI bugs where completed task output cannot be found because:
- The log filename and `state.json` RunID were generated independently (race condition)
- Dispatched worker JSONL logs lived in worktrees that get deleted after merge
- `loadCompletedTaskOutput` brute-force scanned all log files with no index
- Log pruning could delete files containing completed task output

The new structure makes log discovery deterministic: to find logs for a feature, read all files in `.maggus/logs/<maggus_id>/`. To find a specific task's output, filter entries by `task_id`. No scanning, no guessing.

### Architecture Context

- **Vision alignment:** Reliable TUI output display is core to the daemon monitoring experience
- **Components involved:** `runlog` (Logger, snapshot), `cmd/status_*` (TUI log display), `cmd/daemon_*` (daemon lifecycle), `cmd/run_*` (work loop), `cmd/dispatch.go` (worktree workers), `internal/usage` (token tracking)
- **New patterns:** Directory-per-feature log organization; Logger manages multiple open file handles keyed by maggus_id
- **Architecture update needed:** ARCHITECTURE.md should document the new `.maggus/logs/` directory and the removal of `run_id`

## Goals

- Completed task output is always findable in the TUI, regardless of daemon restarts, worktree cleanup, or log pruning
- Log lookup is O(1) directory read instead of O(n) file scan
- The `run_id` concept is removed — PID is the log filename, `maggus_id` is the directory
- Worktree-dispatched workers write logs to the main repo so data survives worktree cleanup
- Dead code left behind by these changes is cleaned up

## Tasks

### TASK-050-001: Restructure Logger to write to .maggus/logs/<maggus_id>/<pid>.log
**Description:** As a developer, I want the Logger to write log files organized by feature/bug GUID so that log discovery is deterministic.

**Token Estimate:** ~75k tokens
**Predecessors:** none
**Successors:** TASK-050-002, TASK-050-003
**Parallel:** no
**Model:** opus

Refactor `internal/runlog/runlog.go`:

1. **Change `Open` signature** from `Open(maggusID, dir string, maxFiles int)` to `Open(dir string, maxFiles int)`. The logger no longer needs an ID at open time — it discovers the target directory from `SetCurrentMaggusID`.

2. **Add `logsDir` field** to `Logger` struct: the base path `.maggus/logs/`. Also add a `pid` field set to `os.Getpid()` at creation.

3. **Refactor `SetCurrentMaggusID`** to switch the active file handle:
   - Close the current file handle (if any)
   - If the new maggus_id is non-empty, open/create `.maggus/logs/<maggus_id>/<pid>.log` with `O_CREATE|O_APPEND|O_WRONLY`
   - Create the `<maggus_id>` subdirectory if it doesn't exist
   - If the new maggus_id is empty, set the file handle to nil (entries outside a feature are dropped — or write to a fallback `_daemon.log` at the base)

4. **Update `emit()`**: write to the currently active file handle. If nil, entries are silently dropped (same nil-safe pattern as today).

5. **Update `Close()`**: close the active file handle.

6. **Remove `OpenWorker`** — parallel workers use the same Logger mechanism now. Each worker opens its own Logger and calls `SetCurrentMaggusID` with the feature's GUID. The PID differentiates their files.

7. **Update pruning**: `pruneLogFiles` should work per-directory (prune oldest files within each `<maggus_id>/` dir) rather than globally. Consider pruning entire feature directories when the feature file is completed/deleted (can defer to TASK-050-005).

8. **Update tests**: all `runlog_test.go` and `runlog_pruning_test.go` tests to use the new directory structure. Test files should appear at `.maggus/logs/<maggus_id>/<pid>.log`. Update fixture filenames.

**Acceptance Criteria:**
- [x] `Logger` writes to `.maggus/logs/<maggus_id>/<pid>.log` where `pid` is `os.Getpid()`
- [x] `SetCurrentMaggusID` switches the target file, creating the directory if needed
- [x] `Open` no longer takes a `maggusID`/`runID` parameter
- [x] `OpenWorker` is removed
- [x] Entries emitted outside any feature (empty maggus_id) are silently dropped or written to a fallback
- [x] Pruning works per-directory
- [x] All existing `runlog` tests updated and passing
- [x] `go build ./...` succeeds
- [x] `go test ./internal/runlog/...` passes

### TASK-050-002: Update daemon and work loop to use new Logger
**Description:** As a developer, I want the daemon, work loop, and parallel orchestrator to use the restructured Logger so that all log data lands in the correct per-feature directories.

**Token Estimate:** ~60k tokens
**Predecessors:** TASK-050-001
**Successors:** TASK-050-004
**Parallel:** no

Update all callers of the old `runlog.Open` / `runlog.OpenWorker`:

1. **`daemon_keepalive.go`**: Change `runlog.Open(runID, dir, maxFiles)` to `runlog.Open(dir, maxFiles)`. Remove the `runID` local variable entirely. The `runLogger` no longer needs a run ID.

2. **`run_loop.go`**: `SetCurrentMaggusID(group.MaggusID)` already fires at the right time — the Logger now switches files here. No functional change needed, just verify it works.

3. **`run_parallel.go`**: Replace `runlog.OpenWorker(iter, task.ID, dir)` with `runlog.Open(dir, maxFiles)` + `SetCurrentMaggusID(group.MaggusID)`. Each parallel worker gets its own Logger; the PID differentiates files.

4. **`dispatch.go`**: Dispatched workers already run as separate processes with their own PID. Ensure they write to `<dispatch-repo>/.maggus/logs/<maggus_id>/<pid>.log` instead of the worktree. The `--dispatch-repo` flag already provides the main repo path — pass it as the Logger's `dir` instead of using the worktree dir.

5. **Remove `--daemon-run-id` flag**: Delete from `run.go` flag registration. Remove `daemonRunIDFlag` variable. Remove from `daemon_start.go` and `dispatch.go` where it's passed to subprocess args.

6. **Remove `generateDaemonRunID()`** from `daemon.go`.

7. **Update `nullTUIModel`**: Remove `snapshotRunID` field. The snapshot no longer carries a RunID (handled in TASK-050-003).

8. **Update `daemon_keepalive.go` `runOneDaemonCycle`**: Remove `runID` parameter. Remove all places where `runID` is threaded through.

**Acceptance Criteria:**
- [x] `daemon_keepalive.go` calls `runlog.Open(dir, maxFiles)` without run ID
- [x] `run_parallel.go` uses `runlog.Open` + `SetCurrentMaggusID` instead of `OpenWorker`
- [x] Dispatched workers write JSONL to `<dispatch-repo>/.maggus/logs/<maggus_id>/`
- [x] `--daemon-run-id` flag removed from `run.go`
- [x] `generateDaemonRunID()` deleted from `daemon.go`
- [x] `daemonRunIDFlag` variable removed
- [x] `daemon_start.go` and `dispatch.go` no longer pass `--daemon-run-id` to subprocesses
- [x] `nullTUIModel.snapshotRunID` removed
- [x] `go build ./...` succeeds
- [x] `go test ./cmd/...` passes

### TASK-050-003: Update TUI log display to read from per-feature directories
**Description:** As a user, I want the TUI status view to reliably show task output by reading from the correct per-feature log directory.

**Token Estimate:** ~75k tokens
**Predecessors:** TASK-050-001
**Successors:** TASK-050-004
**Parallel:** yes — can run alongside TASK-050-002

Update all TUI code that reads log files:

1. **Replace `findLatestRunLog()`** in `status_runlog.go`: Instead of scanning `.maggus/runs/` for the latest `.log` file by name, the TUI reads the active `maggus_id` from `state.json` (or the selected task's plan) and lists files in `.maggus/logs/<maggus_id>/`.

2. **Replace `loadCompletedTaskOutput()`** in `status_task_output.go`: Instead of scanning all log files in `.maggus/runs/`, read only files in `.maggus/logs/<maggus_id>/` for the selected task's feature. Filter JSONL entries by `task_id` as before, but the search scope is now one directory instead of all logs.

3. **Update `StateSnapshot`** in `snapshot.go`: Remove the `RunID` field. Add `MaggusID` field if not already present (the TUI needs this to locate the log directory). The `TaskID` field already exists.

4. **Update `writeSnapshot()`** in `daemon_tui.go`: Write the current `maggus_id` into the snapshot instead of `runID`.

5. **Update `LogFileWatcher`** in `log_watcher.go`: Watch `.maggus/logs/` recursively (or watch specific feature subdirectories based on the active feature). When a `.log` file is created or written in any subdirectory, fire `logFileUpdateMsg`.

6. **Update `status_update.go`**: The `logFileUpdateMsg` handler reads the snapshot for `MaggusID` and `TaskID`, then reads from `.maggus/logs/<maggus_id>/` to build the task output.

7. **Update `parseLogForCurrentState()`**: Reads from per-feature directory. The "current feature" is now known from the snapshot's `MaggusID` field rather than parsed from log lines.

**Acceptance Criteria:**
- [x] `findLatestRunLog` replaced with directory-based lookup using `maggus_id`
- [x] `loadCompletedTaskOutput` reads only from `.maggus/logs/<maggus_id>/` for the relevant feature
- [x] `StateSnapshot.RunID` field removed
- [x] `StateSnapshot.MaggusID` field added (or confirmed present)
- [x] `LogFileWatcher` watches `.maggus/logs/` for changes
- [x] Live task output displays correctly in the TUI during a running task
- [x] Completed task output displays correctly in the TUI for finished tasks
- [x] `go build ./...` succeeds
- [x] `go test ./cmd/...` passes

### TASK-050-004: Deprecate RunID in usage tracking
**Description:** As a developer, I want RunID removed from active code paths while preserving backwards compatibility in persisted usage data.

**Token Estimate:** ~25k tokens
**Predecessors:** TASK-050-002, TASK-050-003
**Successors:** TASK-050-005
**Parallel:** no

1. **`usage.Record.RunID`**: Keep the field in the struct (for JSON deserialization of old records) but stop writing meaningful values. Set to empty string in all new records.

2. **Remove `runID` from `taskContext`** in `run_task.go` — it's no longer used.

3. **Remove `runID` from `parallelOrchestrator`** if present.

4. **Remove `runID` from daemon `runOneDaemonCycle` signature** and all downstream threading.

5. **Update usage tests**: Stop asserting specific RunID values in new records. Keep migration tests that read old records with RunID intact.

**Acceptance Criteria:**
- [ ] `usage.Record.RunID` field retained in struct for backwards compat
- [ ] New usage records write empty string for `RunID`
- [ ] `runID` removed from `taskContext` struct
- [ ] No code path generates or threads a `runID` value
- [ ] Migration tests for old usage records still pass
- [ ] `go build ./...` succeeds
- [ ] `go test ./internal/usage/...` passes

### TASK-050-005: Add .maggus/logs/ to gitignore and update pruning
**Description:** As a developer, I want the new logs directory properly gitignored and old feature log directories cleaned up when features are completed or deleted.

**Token Estimate:** ~20k tokens
**Predecessors:** TASK-050-001
**Successors:** TASK-050-006
**Parallel:** yes — can run alongside TASK-050-002, TASK-050-003

1. **Update `gitignore` package**: Add `.maggus/logs/` to the required gitignore entries (alongside existing `.maggus/runs/` entry).

2. **Update `clean` command**: When cleaning completed feature files, also remove `.maggus/logs/<maggus_id>/` for the cleaned features. The `maggus_id` is available from the feature file's frontmatter.

3. **Consider**: Should `.maggus/runs/` still be used for anything? After this feature, `state.json` and `state-*.json` snapshots still live there. `daemon.log` still lives there. But JSONL run logs move to `.maggus/logs/`. Document this split.

**Acceptance Criteria:**
- [x] `.maggus/logs/` added to gitignore entries
- [x] `maggus clean` removes log directories for cleaned features
- [x] `.maggus/runs/` retains only snapshot files and `daemon.log`
- [x] `go build ./...` succeeds
- [x] `go test ./...` passes

### TASK-050-006: Remove dead code and update documentation
**Description:** As a developer, I want all dead code from the old logging system removed and documentation updated to reflect the new structure.

**Token Estimate:** ~30k tokens
**Predecessors:** TASK-050-004, TASK-050-005
**Successors:** none
**Parallel:** no

1. **Remove dead functions/types**: Scan for any functions, types, or variables that became unreachable after the refactor. Likely candidates:
   - `findLatestRunLog()` (replaced)
   - `readLastNLogLines()` (if no longer used)
   - `parseLogForCurrentState()` (if replaced by snapshot-based lookup)
   - `generateDaemonRunID()` (removed in TASK-050-002)
   - `OpenWorker()` (removed in TASK-050-001)
   - Old test helpers tied to removed functions
   - Any `runID`-related fields, parameters, or variables missed in earlier tasks

2. **Run static analysis**: Use `go vet ./...` and check for unused exports. The existing linter diagnostics (visible in earlier edits) flag several unused functions — verify whether any of those are related to this refactor.

3. **Update CLAUDE.md**: Update the architecture table entry for `runlog` to describe the new `.maggus/logs/<maggus_id>/` structure. Update the "Run Loop Flow" section to remove references to run ID.

4. **Update ARCHITECTURE.md**: Document the new log directory structure and the removal of `run_id`.

**Acceptance Criteria:**
- [ ] No dead functions, types, or variables related to the old logging system remain
- [ ] `go vet ./...` reports no new warnings from this refactor
- [ ] CLAUDE.md architecture table updated for `runlog` package
- [ ] ARCHITECTURE.md updated with new log structure
- [ ] `go build ./...` succeeds
- [ ] `go test ./...` passes

## Task Dependency Graph

```
TASK-050-001 ──→ TASK-050-002 ──→ TASK-050-004 ──→ TASK-050-006
             ├─→ TASK-050-003 ──┘                  ↑
             └─→ TASK-050-005 ─────────────────────┘
```

| Task | Estimate | Predecessors | Parallel | Model |
|------|----------|--------------|----------|-------|
| TASK-050-001 | ~75k | none | no | opus |
| TASK-050-002 | ~60k | 001 | no | — |
| TASK-050-003 | ~75k | 001 | yes (with 002, 005) | — |
| TASK-050-004 | ~25k | 002, 003 | no | — |
| TASK-050-005 | ~20k | 001 | yes (with 002, 003) | — |
| TASK-050-006 | ~30k | 004, 005 | no | — |

**Total estimated tokens:** ~285k

## Functional Requirements

- FR-1: All JSONL log entries for a feature must be written to `.maggus/logs/<maggus_id>/` where `<maggus_id>` is the feature's GUID from `<!-- maggus-id: ... -->`
- FR-2: Log filenames within a feature directory must be `<pid>.log` where `<pid>` is the OS process ID of the writing process
- FR-3: The Logger must switch target files when `SetCurrentMaggusID` is called with a different GUID
- FR-4: Dispatched worktree workers must write their JSONL logs to the main repo's `.maggus/logs/`, not the worktree's
- FR-5: The TUI must find completed task output by reading `.maggus/logs/<maggus_id>/` and filtering by `task_id`
- FR-6: The TUI must find live task output via `state.json` which contains the active `maggus_id` and `task_id`
- FR-7: No code path may generate, thread, or depend on a `run_id` value
- FR-8: Old usage records with `RunID` must still deserialize correctly

## Non-Goals

- No migration of existing `.maggus/runs/*.log` files to the new structure (old logs are expendable)
- No changes to `daemon.log` — it stays in `.maggus/runs/`
- No changes to snapshot files (`state.json`, `state-*.json`) location — they stay in `.maggus/runs/`
- No real-time log streaming (the file watcher + poll approach stays the same)
- No database or index file — directory structure IS the index

## Technical Considerations

- **PID reuse is safe**: Log files are opened with `O_APPEND`. If a PID is recycled, new entries append to the existing file. Entries carry timestamps and `task_id` so they remain distinguishable.
- **Directory creation**: `SetCurrentMaggusID` must create `.maggus/logs/<maggus_id>/` on first write. Use `os.MkdirAll`.
- **File handle management**: The Logger holds one open file handle at a time. Switching features closes the old handle and opens the new one. This is clean and avoids file descriptor leaks.
- **Parallel workers**: Each parallel worker is a goroutine (or separate process) with its own Logger instance and its own PID. Multiple PIDs writing to the same feature directory is fine — append-only, separate files.
- **Architecture update**: ARCHITECTURE.md's "File-Based State" section needs updating to document `.maggus/logs/` alongside `.maggus/runs/`.

## Success Metrics

- The TUI "Output" tab shows correct tool history for every completed task, every time
- No more "No output history available" for tasks that were completed in previous daemon sessions
- Dispatched worker task output survives worktree cleanup
- `run_id` concept fully removed from codebase

## Open Questions

None — all design decisions resolved in conversation.
