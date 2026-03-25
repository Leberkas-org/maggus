<!-- maggus-id: 9be8f567-0041-433c-9940-246ea0f6c33f -->
# Feature 006: Improve Status View — Elapsed Timers & Model Layout

## Introduction

The status view's "Updated" line currently shows time since the last snapshot write, which is meaningless to the user. Replace it with two elapsed timers: run elapsed (since the work loop started) and task elapsed (since the current task started). Additionally, the per-model token breakdown currently joins all models on one line with `·` separators, which overflows on typical terminals. Each model should get its own line.

## Goals

- Show meaningful elapsed time: total run duration + current task duration
- Make per-model token/cost lines readable by putting each model on its own line
- Keep the bottom zone of the snapshot panel clean and scannable

## Tasks

### TASK-006-001: Add RunStartedAt and TaskStartedAt to StateSnapshot
**Description:** As a developer, I want the state snapshot to carry run and task start timestamps so the status view can compute elapsed times.

**Token Estimate:** ~25k tokens
**Predecessors:** none
**Successors:** TASK-006-003
**Parallel:** yes — can run alongside TASK-006-002

**Acceptance Criteria:**
- [ ] `StateSnapshot` struct has new fields `RunStartedAt string` and `TaskStartedAt string` (RFC3339)
- [ ] `nullTUIModel.writeSnapshot()` populates both fields from existing `startTime` (task) and a new `runStartedAt` field
- [ ] `nullTUIModel` gets a `runStartedAt time.Time` field, set when the model is created (passed from `workLoopParams.startTime`)
- [ ] `taskStartedAt` is set from `m.startTime` which is already reset on each `IterationStartMsg`
- [ ] Existing snapshot tests updated; new test verifies both timestamps are present in serialized JSON
- [ ] `go vet ./...` passes

### TASK-006-002: Reformat per-model token breakdown to one model per line
**Description:** As a user, I want each model's token usage on its own line so I can read them without horizontal overflow.

**Token Estimate:** ~20k tokens
**Predecessors:** none
**Successors:** TASK-006-003
**Parallel:** yes — can run alongside TASK-006-001

**Acceptance Criteria:**
- [ ] `formatSnapshotModelTokens()` in `status.go` returns multi-line output, one line per model
- [ ] Each line format: `  <model>: <in> in / <out> out ($<cost>)` — indented to align under the "Tokens:" label
- [ ] The old single-line `Model:` summary line is removed (model names now appear in the per-model breakdown)
- [ ] The aggregate `Tokens:` line still shows total in/out
- [ ] Cost per model shown inline; total `Cost:` line remains
- [ ] Visually verified: no line exceeds 80 characters for typical model names
- [ ] `go vet ./...` passes

### TASK-006-003: Replace "Updated" with run elapsed and task elapsed in snapshot panel
**Description:** As a user, I want to see how long the run has been going and how long the current task has been running instead of "Updated: 3s ago".

**Token Estimate:** ~20k tokens
**Predecessors:** TASK-006-001, TASK-006-002
**Successors:** none
**Parallel:** no

**Acceptance Criteria:**
- [ ] The "Updated:" line is removed from `renderSnapshotPanel()`
- [ ] Two new lines are rendered: `Run:` showing elapsed since `RunStartedAt`, and `Task:` showing elapsed since `TaskStartedAt`
- [ ] Format: human-friendly duration (e.g. `5m 32s`, `1h 12m 5s`) — not Go's default `5m32s`
- [ ] If timestamps are missing/unparseable, show `—` instead of crashing
- [ ] `go vet ./...` passes

## Task Dependency Graph

```
TASK-006-001 ──→ TASK-006-003
TASK-006-002 ──┘
```

| Task | Estimate | Predecessors | Parallel | Model |
|------|----------|--------------|----------|-------|
| TASK-006-001 | ~25k | none | yes (with 002) | — |
| TASK-006-002 | ~20k | none | yes (with 001) | — |
| TASK-006-003 | ~20k | 001, 002 | no | — |

**Total estimated tokens:** ~65k

## Functional Requirements

- FR-1: The snapshot panel must show "Run:" with elapsed time since the work loop started
- FR-2: The snapshot panel must show "Task:" with elapsed time since the current task started
- FR-3: The old "Updated:" line must be removed
- FR-4: Per-model token breakdown must show one model per line with cost
- FR-5: The old single-line "Model:" summary must be removed (redundant with per-model lines)
- FR-6: Elapsed durations must use human-friendly format with spaces (e.g. `5m 32s`)

## Non-Goals

- No changes to the plain-text (`--plain`) status output
- No changes to the runner TUI spinner (it already has its own elapsed timer)
- No changes to how `UpdatedAt` is set in `WriteSnapshot` — it stays for other potential consumers

## Technical Considerations

- `nullTUIModel` already has `startTime` (task start) — just need to add `runStartedAt` and wire it from `workLoopParams.startTime`
- The `RunStartedAt` field in the snapshot is set once per run; `TaskStartedAt` resets on each `IterationStartMsg`
- `UpdatedAt` continues to be set by `WriteSnapshot` but is no longer displayed
