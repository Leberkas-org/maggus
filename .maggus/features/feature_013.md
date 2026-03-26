<!-- maggus-id: b4b5065f-00ba-456f-95d0-a989f48cbbf8 -->
# Feature 013: Structured Runlog Tool-Use Entries

## Introduction

The `run.log` JSONL file currently stores `tool_use` entries with a human-formatted `description` field (e.g. `"Read: /path/to/file"`) that redundantly prefixes the tool name already present in the `tool` field. This formatting was designed for the TUI spinner but the same string leaked into the log. The log should store raw structured data; human-readable formatting happens at render time when needed.

Additionally, the `item_id` field carries the maggus UUID but its name is opaque — it should be renamed to `maggus_id` for clarity. Finally, token usage and cost data flows through to the TUI but is never persisted to the log, losing valuable per-task metrics.

### Architecture Context

- **Components involved:** `internal/runlog` (Entry schema, Logger), `internal/runner` (TUI callback wiring), `internal/agent` (ToolMsg already has Params map — no changes needed), `cmd/work.go`, `cmd/work_loop.go`, `cmd/daemon_keepalive.go`, `cmd/daemon_tui.go`, `cmd/status_runlog.go`
- **Key existing helpers to reuse:** `agent.buildToolParams()` already produces `map[string]string` with the right keys (`file`, `command`, `pattern`, `skill`, `description`); `ToolMsg.Params` is already populated — the data exists, it's just not wired to the logger
- **No changes to:** `agent/claude.go`, `agent/messages.go`, `runlog/snapshot.go` (snapshot keeps human-readable description for the live status view)

## Goals

- Store raw structured input data in `tool_use` log entries instead of a human-formatted string
- Rename `item_id` → `maggus_id` in log entries so the field name matches its semantic meaning
- Rename `Logger.SetCurrentItem` → `Logger.SetCurrentMaggusID` for the same reason
- Persist token usage and cost per task as a new `task_usage` log event
- Keep all TUI display behavior unchanged (spinner, tool list, status view all still use human-readable strings)

## Tasks

### TASK-013-001: Refactor runlog.Entry and Logger
**Description:** As a developer, I want the runlog Entry struct to use structured input fields and a clearly named maggus_id field so that log data is raw and queryable without needing to parse human-formatted strings.

**Token Estimate:** ~40k tokens
**Predecessors:** none
**Successors:** TASK-013-002, TASK-013-003
**Parallel:** no — other tasks depend on the new schema

**Acceptance Criteria:**
- [x] `Entry.ItemID` (json: `item_id`) renamed to `Entry.MaggusID` (json: `maggus_id`)
- [x] `Entry.Description string` removed; `Entry.Input map[string]string` added (json: `input,omitempty`)
- [x] Token/cost fields added to `Entry`: `InputTokens int`, `OutputTokens int`, `CacheCreationInputTokens int`, `CacheReadInputTokens int`, `CostUSD float64`, `ModelUsage map[string]ModelTokensEntry` (all `omitempty`)
- [x] `ModelTokensEntry` struct added to the package (same fields as above per-model, plus `CostUSD float64`)
- [x] `TaskUsageData` struct added as the parameter type for the new `TaskUsage` method
- [x] `Logger.currentItemID` field renamed to `currentMaggusID`
- [x] `Logger.SetCurrentItem` renamed to `Logger.SetCurrentMaggusID`
- [x] `Logger.ToolUse` signature changed to `ToolUse(taskID, toolType string, params map[string]string)`; emits `Input: params`
- [x] `Logger.TaskUsage(data TaskUsageData)` method added; emits `event: "task_usage"` with token/cost fields
- [x] `go build ./...` passes
- [x] `go test ./internal/runlog/...` passes

### TASK-013-002: Update runlog tests
**Description:** As a developer, I want the runlog tests updated to the new struct fields and method signatures so that the test suite stays green.

**Token Estimate:** ~25k tokens
**Predecessors:** TASK-013-001
**Successors:** TASK-013-004
**Parallel:** no

**Acceptance Criteria:**
- [ ] All `ToolUse(...)` calls in `runlog_test.go` updated to new signature `(taskID, toolType string, params map[string]string)`
- [ ] All assertions on `entry.Description` replaced with assertions on `entry.Input` map fields
- [ ] All assertions on `entry.ItemID` / `"item_id"` JSON replaced with `entry.MaggusID` / `"maggus_id"`
- [ ] All `SetCurrentItem` calls replaced with `SetCurrentMaggusID`
- [ ] New file `runlog_usage_test.go` created with `TestTaskUsage`: calls `runLogger.TaskUsage(...)` and asserts the emitted JSONL contains `event: "task_usage"` with correct token and cost fields
- [ ] `runlog_test.go` stays under 500 lines (new test moved to separate file)
- [ ] `go test ./internal/runlog/...` passes

### TASK-013-003: Wire new callback signatures in runner and cmd layers
**Description:** As a developer, I want the `onToolUse` callback and all its call sites updated to pass raw `params map[string]string` instead of a human-formatted description string, and to wire the `task_usage` log event at run completion.

**Token Estimate:** ~50k tokens
**Predecessors:** TASK-013-001
**Successors:** TASK-013-004
**Parallel:** no

**Acceptance Criteria:**
- [ ] `runner/tui.go`: `onToolUse` field type changed to `func(taskID, toolType string, params map[string]string)`; `SetOnToolUse` setter updated to match
- [ ] `runner/tui_messages.go`: `handleToolMsg` passes `msg.Params` (not `msg.Description`) to `onToolUse` callback; TUI display of `msg.Description` is unchanged
- [ ] `cmd/daemon_tui.go`: `onToolUse` field type and `SetOnToolUse` updated; `nullTUIModel.Update` passes `msg.Params` to callback; `SnapshotToolEntry.Description` still uses `msg.Description`
- [ ] `cmd/work.go`: `SetOnToolUse` lambda updated to `func(taskID, toolType string, params map[string]string)` calling `runLogger.ToolUse(taskID, toolType, params)`; `setupUsageCallback` call updated to pass `runLogger`
- [ ] `cmd/work_loop.go`: `setupUsageCallback` gains `runLogger *runlog.Logger` parameter; inside `SetOnTaskUsage` callback, converts `runner.TaskUsage` model-usage map to `map[string]runlog.ModelTokensEntry` and calls `runLogger.TaskUsage(...)`
- [ ] `cmd/daemon_keepalive.go`: `SetOnToolUse` lambda updated same as work.go; existing `SetOnTaskUsage` callback augmented to also call `runLogger.TaskUsage(...)`
- [ ] All `SetCurrentItem` call sites updated to `SetCurrentMaggusID`
- [ ] `go build ./...` passes

### TASK-013-004: Update status_runlog.go renderer and run final verification
**Description:** As a developer, I want the log renderer updated to display `input` map data for `tool_use` entries and to render the new `task_usage` event, so that `maggus status` still works correctly.

**Token Estimate:** ~25k tokens
**Predecessors:** TASK-013-002, TASK-013-003
**Successors:** none
**Parallel:** no

**Acceptance Criteria:**
- [ ] `cmd/status_runlog.go` `tool_use` case updated: uses a new private `formatToolInput(tool string, input map[string]string) string` helper that returns the most meaningful value (priority: `file` → `command` → `pattern` → `skill` → `description` → first value → tool name)
- [ ] `cmd/status_runlog.go` `default` case updated: uses `entry.Input` instead of `entry.Description`
- [ ] New `case "task_usage":` renders token counts and cost (e.g. `usage: 12000 in / 800 out  $0.042`)
- [ ] All references to `entry.Description` and `entry.ItemID` removed from `status_runlog.go`
- [ ] `go build ./...` passes
- [ ] `go test ./...` passes (all packages)

## Task Dependency Graph

```
TASK-013-001 ──→ TASK-013-002 ──→ TASK-013-004
             └→ TASK-013-003 ──┘
```

| Task | Estimate | Predecessors | Parallel | Model |
|------|----------|--------------|----------|-------|
| TASK-013-001 | ~40k | none | no | — |
| TASK-013-002 | ~25k | 001 | no | — |
| TASK-013-003 | ~50k | 001 | no | — |
| TASK-013-004 | ~25k | 002, 003 | no | — |

**Total estimated tokens:** ~140k

## Functional Requirements

- FR-1: `tool_use` log entries must store raw input data in an `input` map (e.g. `{"file": "/path/to/file"}`) instead of a human-formatted description string
- FR-2: The `tool` field in `tool_use` entries must remain unchanged (tool name only, no prefix)
- FR-3: Every log entry must carry `maggus_id` (the feature UUID) instead of `item_id`
- FR-4: `Logger.SetCurrentMaggusID` replaces `Logger.SetCurrentItem`; all call sites updated
- FR-5: A new `task_usage` event must be emitted at the end of each task run containing: `input_tokens`, `output_tokens`, `cache_creation_input_tokens`, `cache_read_input_tokens`, `cost_usd`, and `model_usage` (per-model breakdown)
- FR-6: The TUI display (spinner, tool list, snapshot Description) must remain unchanged — it continues to use the human-formatted `msg.Description` from `ToolMsg`
- FR-7: `maggus status` log view must render `tool_use` entries using the most meaningful field from the `input` map, and must render `task_usage` entries showing token counts and cost

## Non-Goals

- No changes to `agent/claude.go`, `agent/messages.go`, or `agent/buildToolParams()` — the data is already correct there
- No changes to `runlog/snapshot.go` — `SnapshotToolEntry.Description` keeps using human text for the live status view
- No changes to log file naming or pruning logic
- No migration of existing log files — old entries with `description`/`item_id` are simply not rendered by the new renderer (or fall through to default)
- No new tool input fields beyond what `buildToolParams()` already extracts

## Technical Considerations

- `ToolMsg.Params` (type `map[string]string`) is already populated by `agent.buildToolParams()` and contains the right keys. No agent-layer changes needed — just pass `msg.Params` instead of `msg.Description` to the callback.
- The `runlog` package must not import `agent` or `runner`. `ModelTokensEntry` and `TaskUsageData` are defined in `runlog` as standalone types.
- `float64` fields with `omitempty` in JSON marshal as `0` when zero — use pointer `*float64` for `CostUSD` if zero-cost entries should omit the field, or accept that `0` appears. Given cost is almost always non-zero for real runs, plain `float64` is acceptable.
- The `task_usage` event fires via the existing `SetOnTaskUsage` → `runner.TaskUsage` path; no new TUI message type needed.

## Success Metrics

- `tool_use` entries in `run.log` contain `"input":{"file":"..."}` instead of `"description":"Read: ..."`
- Every entry has `"maggus_id":"<uuid>"` instead of `"item_id":"<uuid>"`
- End-of-task entries contain `"event":"task_usage"` with token and cost fields
- `go test ./...` passes with no regressions
- `maggus status` renders the log without errors

## Open Questions

*(none — all resolved in design session)*
