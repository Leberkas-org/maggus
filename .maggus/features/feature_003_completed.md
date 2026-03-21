# Feature 003: Format Elapsed Timers as HH:MM:SS

## Introduction

The elapsed timers on the work view Progress tab currently display in Go's default duration format (`2m15s`). This should be changed to a fixed-width `00:00:00` (hh:mm:ss) format for better readability, with more visual spacing between the Task and Run timers. The summary screen must not be modified.

## Goals

- Display elapsed timers in `HH:MM:SS` format on the work view Progress tab
- Improve visual spacing between Task and Run timer labels
- Leave the summary screen rendering untouched

## Tasks

### TASK-003-001: Format elapsed timers as HH:MM:SS with improved spacing
**Description:** As a user watching the work view, I want elapsed times displayed in `00:00:00` format so they are easier to read at a glance and have a consistent fixed width.

**Token Estimate:** ~20k tokens
**Predecessors:** none
**Successors:** none
**Parallel:** n/a — single task

**Acceptance Criteria:**
- [x] A helper function `formatHHMMSS(d time.Duration) string` is added to the runner package (e.g., in `tui_render.go` or a small util file) that converts a `time.Duration` to `HH:MM:SS` format (e.g., `00:02:15`, `01:30:00`)
- [x] The Progress tab Elapsed line in `tui_render.go` (line 599) uses the new format for both task and run timers
- [x] The spacing between Task and Run is increased (e.g., use `    ` or a wider separator like `   ·   ` instead of `  ·  `)
- [x] The summary screen (`tui_summary.go`) is NOT modified — it keeps Go's default duration format
- [x] Example rendered output: `  Elapsed:  Task: 00:02:15   ·   Run: 00:12:30`
- [x] Hours roll over correctly (e.g., 90 minutes renders as `01:30:00`, not `90:00`)
- [x] Unit tests for `formatHHMMSS` cover: zero duration, seconds only, minutes+seconds, hours+minutes+seconds, large durations (24h+)
- [x] `go vet ./...` passes
- [x] `go test ./...` passes

## Task Dependency Graph

```
TASK-003-001 (no deps)
```

| Task | Estimate | Predecessors | Parallel | Model |
|------|----------|--------------|----------|-------|
| TASK-003-001 | ~20k | none | n/a | — |

**Total estimated tokens:** ~20k

## Functional Requirements

- FR-1: Task elapsed timer on the Progress tab must display in `HH:MM:SS` format
- FR-2: Run elapsed timer on the Progress tab must display in `HH:MM:SS` format
- FR-3: There must be wider spacing between the Task and Run timer values
- FR-4: The summary screen elapsed display must remain unchanged

## Non-Goals

- No changes to the summary screen elapsed rendering
- No changes to the 2x countdown timer format
- No changes to usage log timestamps

## Technical Considerations

- Go's `time.Duration.String()` produces `2m15s` — the new helper must manually extract hours, minutes, seconds via integer division
- Use `fmt.Sprintf("%02d:%02d:%02d", h, m, s)` for zero-padded formatting
- The helper should truncate sub-second precision (already done by `.Truncate(time.Second)` at line 529-530)

## Success Metrics

- Elapsed timers are visually consistent and easy to parse at a glance
- Fixed-width format prevents layout shifts as time progresses

## Open Questions

None — all resolved.
