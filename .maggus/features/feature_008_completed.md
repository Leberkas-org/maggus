<!-- maggus-id: 2614a23a-09db-4b4d-94c8-462e1f220e06 -->
# Feature 008: Add item_id to run.log entries

## Introduction

Every entry in `run.log` is a JSONL object describing what happened during a work session (task start, tool use, agent output, task completion, etc.). Currently these entries are not tagged with which feature or bug was being worked on, making it impossible to filter the log by feature/bug after the fact.

This feature adds an `item_id` field — the `MaggusID` UUID already used in `usage.jsonl` — to every run.log entry emitted while a feature or bug is active. Entries outside any active plan (e.g. daemon idle messages) will have no `item_id` field (omitempty).

## Goals

- Every run.log entry emitted during active feature/bug execution carries `item_id` (the plan's MaggusID UUID)
- `item_id` is absent on entries that occur outside any active plan (backward compatible)
- The `item_id` field name matches the existing naming in `~/.maggus/usage/work.jsonl`

## Tasks

### TASK-008-001: Add item_id tracking to runlog.Logger and wire it up in the work loop

**Description:** As a developer analyzing logs, I want every run.log entry during feature/bug execution to carry an `item_id` so I can filter and correlate all log events to the plan that was being worked on.

**Token Estimate:** ~20k tokens
**Predecessors:** none
**Successors:** none
**Parallel:** no

**Acceptance Criteria:**
- [x] `Entry` struct in `src/internal/runlog/runlog.go` has a new `ItemID string` field with JSON tag `"item_id,omitempty"`
- [x] `Logger` struct has a `currentItemID string` field
- [x] A `SetCurrentItem(itemID string)` method exists on `*Logger`; it is nil-safe (no-op on nil)
- [x] `emit()` auto-injects `l.currentItemID` into the entry's `ItemID` field before marshaling (when `entry.ItemID` is empty)
- [x] In `src/cmd/work_loop.go`, `tc.logger.SetCurrentItem(group.MaggusID)` is called after `tc.currentPlan = &group` (line ~416) and before `tc.logger.FeatureStart(group.ID)`
- [x] In `src/cmd/work_loop.go`, `tc.logger.SetCurrentItem("")` is called after `tc.logger.FeatureComplete(group.ID)` (line ~488)
- [x] All existing entries that already carry a task-specific `TaskID` also gain `item_id` automatically via the emit injection
- [x] `item_id` is absent (not `""`) on entries emitted outside any active plan (omitempty ensures this)
- [x] `cd src && go build ./...` passes
- [x] `cd src && go test ./...` passes

## Task Dependency Graph

```
TASK-008-001
```

| Task | Estimate | Predecessors | Parallel | Model |
|------|----------|--------------|----------|-------|
| TASK-008-001 | ~20k | none | no | — |

**Total estimated tokens:** ~20k

## Functional Requirements

- FR-1: The `item_id` field in run.log entries must be the `MaggusID` UUID from the plan file's `<!-- maggus-id: ... -->` comment
- FR-2: All event types (`feature_start`, `task_start`, `tool_use`, `output`, `task_complete`, `task_failed`, `feature_complete`, `info`) must carry `item_id` when emitted during active plan execution
- FR-3: The `item_id` field must be absent (not an empty string) when no plan is active
- FR-4: The field name in JSON must be `item_id` to match the naming in `usage.jsonl`

## Non-Goals

- No changes to `daemon.log`
- No changes to `usage.jsonl` or any other log file
- No UI or status command changes
- No migration of existing run.log files

## Technical Considerations

- `emit()` is the single write point for all log entries — injecting `item_id` there keeps all call sites unchanged
- `SetCurrentItem("")` on FeatureComplete ensures daemon idle messages (no active plan) have no `item_id`
- The `Logger` is accessed from both the work goroutine (task lifecycle) and the TUI goroutine (tool use / output callbacks); `currentItemID` is only ever written by the work goroutine and changes only at feature boundaries (not mid-task), so no mutex is required in practice

## Open Questions

_(none)_
