# Feature 001: Dual Timer & Usage Log Cleanup

## Introduction

The work TUI currently shows a single elapsed timer that resets on each new task, making it impossible to see overall run duration during execution. Additionally, the `Elapsed` field in the usage log (`usage_work.jsonl`) is redundant since it can be computed from `StartTime` and `EndTime`.

This feature removes the redundant field from the usage log and adds a persistent run-level timer alongside the existing per-task timer in the TUI.

## Goals

- Remove the computed `Elapsed` field from usage log records to reduce redundancy
- Show both per-task and per-run elapsed time in the work TUI Progress tab
- Ensure the summary screen continues to display total run elapsed time

## Tasks

### TASK-001-001: Remove Elapsed field from usage log
**Description:** As a developer, I want the usage log to not store redundant computed fields so that the data is clean and canonical.

**Token Estimate:** ~15k tokens
**Predecessors:** none
**Successors:** none
**Parallel:** yes — can run alongside TASK-001-002

**Acceptance Criteria:**
- [x] `Elapsed` field is removed from the `Record` struct in `internal/usage/usage.go`
- [x] The elapsed computation on line 54 of `usage.go` is removed
- [x] The JSON tag `"elapsed"` no longer appears in the struct
- [x] All tests in `internal/usage/` pass (update any that reference the `Elapsed` field)
- [x] `go vet ./...` passes

### TASK-001-002: Add run elapsed timer to the work TUI
**Description:** As a user running `maggus work`, I want to see both the current task elapsed time and the total run elapsed time so I know how long the overall run has been going.

**Token Estimate:** ~30k tokens
**Predecessors:** none
**Successors:** none
**Parallel:** yes — can run alongside TASK-001-001

**Acceptance Criteria:**
- [ ] `TUIModel` has a new `runStartTime time.Time` field set once in `NewTUIModel()` and never reset
- [ ] `handleIterationStart()` does NOT reset `runStartTime` (only resets `startTime` as before)
- [ ] The Progress tab Elapsed line shows both timers, formatted as: `Task: 2m 15s  ·  Run: 12m 30s`
- [ ] The task timer (`startTime`) continues to reset per task as before
- [ ] The run timer (`runStartTime`) counts continuously from TUI creation until the run ends
- [ ] The summary screen elapsed display continues to work correctly (it already uses `SummaryData.StartTime` from the run tracker)
- [ ] All tests in `cmd/` pass
- [ ] `go vet ./...` passes

## Task Dependency Graph

```
TASK-001-001 (no deps)
TASK-001-002 (no deps)
```

| Task | Estimate | Predecessors | Parallel | Model |
|------|----------|--------------|----------|-------|
| TASK-001-001 | ~15k | none | yes (with 002) | — |
| TASK-001-002 | ~30k | none | yes (with 001) | — |

**Total estimated tokens:** ~45k

## Functional Requirements

- FR-1: The `usage_work.jsonl` records must no longer contain an `elapsed` JSON key
- FR-2: The Progress tab must display task elapsed and run elapsed on the same line, visually separated
- FR-3: The task timer resets to zero when a new task starts
- FR-4: The run timer starts when the TUI model is created and never resets
- FR-5: The summary screen must continue to show total run elapsed time

## Non-Goals

- No changes to the summary screen layout (it already shows run elapsed)
- No migration or rewriting of existing JSONL log files
- No new fields added to the usage log (run elapsed is TUI-only, not persisted)
- No changes to the 2x countdown ticker

## Technical Considerations

- The `runStartTime` field is separate from `SummaryData.StartTime` (which comes from `runtracker`). Both should be nearly identical in practice since the TUI model is created right before the work loop starts.
- Existing JSONL files will still have `elapsed` entries from prior runs — consumers parsing old logs should handle the missing field gracefully (standard JSON unmarshalling in Go ignores unknown fields by default, so this is safe).

## Success Metrics

- User can see how long the current task AND the overall run have been going at a glance
- Usage log files are smaller and contain no redundant data

## Open Questions

None — all resolved.
