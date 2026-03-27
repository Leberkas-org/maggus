<!-- maggus-id: de75bef5-8ae5-4585-a0df-caa89e7300ca -->
# Feature 014: Move state.json to Fixed Path

## Introduction

Currently, the daemon writes its live snapshot to `.maggus/runs/<runID>/state.json`, creating a per-run subdirectory that contains only this one file. Since only one daemon can ever run at a time, this subdirectory is unnecessary indirection. This feature moves `state.json` to a fixed path `.maggus/runs/state.json`, simplifying the directory layout and removing the need to scan subdirectories to discover the active run.

The `RunID` (currently inferred from the directory name) is moved into the `StateSnapshot` struct so the status command can still reference the correct `.log` file.

### Architecture Context

- **Components involved:** `internal/runlog` (snapshot read/write), `cmd/daemon_tui.go` (writer), `cmd/daemon_keepalive.go` (cleanup), `cmd/status_runlog.go` (discovery), `cmd/status_update.go` (reader)
- **Vision alignment:** Internal cleanup — no user-facing behavior change, just a simpler on-disk layout
- **New patterns:** None; removes unnecessary indirection

## Goals

- `state.json` lives at `.maggus/runs/state.json` (no runID subdirectory)
- The status command reads from the fixed path without scanning for subdirectories
- `RunID` is preserved inside the snapshot struct for log-file lookup
- All tests pass with the updated path

## Tasks

### TASK-014-001: Update snapshot path and struct
**Description:** As a developer, I want the snapshot functions to use a fixed path so that the directory layout is simpler.

**Token Estimate:** ~25k tokens
**Predecessors:** none
**Successors:** TASK-014-002
**Parallel:** no

**Acceptance Criteria:**
- [x] `snapshotPath()` in `src/internal/runlog/snapshot.go` changed from `filepath.Join(dir, ".maggus", "runs", runID, "state.json")` to `filepath.Join(dir, ".maggus", "runs", "state.json")`
- [x] `runID` parameter removed from `WriteSnapshot`, `ReadSnapshot`, and `RemoveSnapshot` function signatures
- [x] `RunID string \`json:"run_id"\`` field added to `StateSnapshot` struct
- [x] All snapshot functions compile and behave correctly with the new signatures
- [x] `go build ./...` passes

### TASK-014-002: Update callers and status discovery
**Description:** As a developer, I want all callers of the snapshot functions updated so that the daemon writes to the new path and the status command reads from it correctly.

**Token Estimate:** ~30k tokens
**Predecessors:** TASK-014-001
**Successors:** TASK-014-003
**Parallel:** no

**Acceptance Criteria:**
- [x] `daemon_tui.go`: `writeSnapshot()` sets `snap.RunID = m.snapshotRunID` and calls `WriteSnapshot(m.snapshotDir, snap)` (no runID arg)
- [x] `daemon_keepalive.go`: `RemoveSnapshot(dir, runID)` call updated to `RemoveSnapshot(dir)`
- [x] `status_runlog.go`: subdirectory-scan logic replaced with a direct `os.Stat` check on `.maggus/runs/state.json`; `RunID` extracted from the snapshot struct for log-file lookup
- [x] `status_update.go`: `ReadSnapshot(m.dir, m.daemon.RunID)` updated to `ReadSnapshot(m.dir)`
- [x] `go build ./...` passes

### TASK-014-003: Update tests
**Description:** As a developer, I want all tests updated to reflect the new fixed path so that the test suite stays green.

**Token Estimate:** ~20k tokens
**Predecessors:** TASK-014-002
**Successors:** none
**Parallel:** no

**Acceptance Criteria:**
- [ ] `src/internal/runlog/snapshot_test.go`: 5 hardcoded path assertions updated to use `.maggus/runs/state.json` (no runID subdirectory)
- [ ] `src/cmd/daemon_tui_test.go`: 1 hardcoded path assertion (~line 189) updated to the new fixed path
- [ ] `go test ./...` passes with no failures

## Task Dependency Graph

```
TASK-014-001 ──→ TASK-014-002 ──→ TASK-014-003
```

| Task | Estimate | Predecessors | Parallel | Model |
|------|----------|--------------|----------|-------|
| TASK-014-001 | ~25k | none | no | — |
| TASK-014-002 | ~30k | 001 | no | — |
| TASK-014-003 | ~20k | 002 | no | — |

**Total estimated tokens:** ~75k

## Functional Requirements

- FR-1: `state.json` must be written to `.maggus/runs/state.json` while the daemon is active
- FR-2: `state.json` must be removed from `.maggus/runs/state.json` on clean daemon exit
- FR-3: The `StateSnapshot` struct must include a `run_id` field containing the active run's ID
- FR-4: The status command must discover the active snapshot by checking `.maggus/runs/state.json` directly, not by scanning subdirectories
- FR-5: The status command must use `snap.RunID` to locate the corresponding `.log` file

## Non-Goals

- No change to the `.log` file format or location
- No change to `daemon.log`
- No change to what data is stored in the snapshot
- No user-facing behavior change — the status TUI renders identically

## Technical Considerations

- The `runID` parameter can be fully removed from all three public snapshot functions since the path no longer depends on it
- `status_runlog.go` currently builds `snapDirs` by scanning for `<runID>/state.json` subdirectories — this logic is replaced by a single `os.Stat` on the fixed path
- Existing stale `<runID>/state.json` files in old run subdirectories will be ignored (they are not read by any code after this change)
- No migration of old files is needed; the directory is gitignored

## Success Metrics

- `.maggus/runs/` no longer contains subdirectories during or after a daemon run
- `go test ./...` passes
- `maggus status` correctly shows live daemon progress during a run
