# Plan: Fix Token Usage Reporting — Include Cache Tokens, Cost, and Per-Model Tracking

## Introduction

Maggus currently reports token usage by parsing only `input_tokens` and `output_tokens` from Claude Code's stream-json result events. However, Claude Code also reports `cache_creation_input_tokens` and `cache_read_input_tokens`, which represent the vast majority of input token usage (system prompts, CLAUDE.md, context files are cached). Additionally, the `total_cost_usd` field and per-model `modelUsage` breakdown are available but not captured.

**Example:** A simple "say hi" prompt reports 3 input tokens, but the real usage is ~19,750 tokens (3 direct + 13,055 cache creation + 6,692 cache read). The reported usage is off by orders of magnitude.

This plan adds cache token tracking, cost tracking, and per-model usage breakdown across the entire pipeline: JSON parsing → message types → TUI state → display → CSV persistence.

## Goals

- Accurately report total input tokens including cache creation and cache read tokens
- Show cache breakdown (write/read) in all token display locations (live progress + summary + per-task)
- Track and display `total_cost_usd` from Claude Code result events
- Track per-model token usage for runs where subagents use different models
- Persist all new fields to a new `usage_v2.csv` format
- Maintain all existing tests and add coverage for new fields

## User Stories

### TASK-001: Add Cache Token Fields to Stream Parsing and Message Types
**Description:** As a developer, I want the stream-json parser to capture `cache_creation_input_tokens` and `cache_read_input_tokens` so that the full token usage is available downstream.

**Acceptance Criteria:**
- [x] `streamUsage` struct in `internal/agent/claude.go` includes `CacheCreationInputTokens int` with JSON tag `cache_creation_input_tokens` and `CacheReadInputTokens int` with JSON tag `cache_read_input_tokens`
- [x] `UsageMsg` struct in `internal/agent/messages.go` includes `CacheCreationInputTokens int` and `CacheReadInputTokens int`
- [x] The `case "result"` handler in `ClaudeAgent.Run()` passes the two new cache fields from `event.Usage` into the `UsageMsg` sent to the bubbletea program
- [x] Typecheck/lint passes
- [x] Unit tests are written and successful

### TASK-002: Add Cost Field to Stream Parsing and Message Types
**Description:** As a developer, I want to capture `total_cost_usd` from the result event so that cost information is available for display and persistence.

**Acceptance Criteria:**
- [x] `streamEvent` struct in `internal/agent/claude.go` includes `CostUSD float64` with JSON tag `total_cost_usd`
- [x] `UsageMsg` struct in `internal/agent/messages.go` includes `CostUSD float64`
- [x] The `case "result"` handler passes `event.CostUSD` into the `UsageMsg`
- [x] Typecheck/lint passes
- [x] Unit tests are written and successful

### TASK-003: Add Per-Model Usage Parsing from Result Events
**Description:** As a developer, I want to parse the `modelUsage` field from Claude Code result events so that token usage can be attributed to specific models.

**Acceptance Criteria:**
- [x] A new `modelUsageEntry` struct is defined in `internal/agent/claude.go` with fields: `InputTokens int` (`inputTokens`), `OutputTokens int` (`outputTokens`), `CacheReadInputTokens int` (`cacheReadInputTokens`), `CacheCreationInputTokens int` (`cacheCreationInputTokens`), `CostUSD float64` (`costUSD`)
- [x] `streamEvent` struct includes `ModelUsage map[string]modelUsageEntry` with JSON tag `modelUsage`
- [x] A new `ModelUsageMsg` struct is defined in `internal/agent/messages.go` with field `Models map[string]ModelTokens` where `ModelTokens` has fields `InputTokens`, `OutputTokens`, `CacheCreationInputTokens`, `CacheReadInputTokens`, `CostUSD float64`
- [x] The `case "result"` handler sends a `ModelUsageMsg` when `event.ModelUsage` is non-empty
- [x] Typecheck/lint passes
- [x] Unit tests are written and successful

### TASK-004: Update Token State to Track Cache Tokens and Cost
**Description:** As a developer, I want the TUI token state to accumulate cache tokens and cost so they can be displayed and persisted per-task.

**Acceptance Criteria:**
- [x] `TaskUsage` struct in `internal/runner/tui_tokens.go` includes `CacheCreationInputTokens int`, `CacheReadInputTokens int`, and `CostUSD float64`
- [x] `tokenState` struct includes per-iteration fields `iterCacheCreation`, `iterCacheRead`, `iterCost` and cumulative fields `totalCacheCreation`, `totalCacheRead`, `totalCost`
- [x] `addUsage()` method accumulates the three new fields for both iter and total counters
- [x] `addUsage()` sets `hasData = true` when any of the new fields are non-zero
- [x] `saveAndReset()` populates the new `TaskUsage` fields from iter counters and resets iter counters to zero
- [x] Typecheck/lint passes
- [x] Unit tests in `tui_tokens_test.go` are updated: test that cache and cost fields accumulate correctly, are included in `TaskUsage` on save, and reset properly

### TASK-005: Track Per-Model Usage in Token State
**Description:** As a developer, I want per-model token usage tracked in the TUI state so it can be displayed in the summary and persisted to CSV.

**Acceptance Criteria:**
- [ ] `tokenState` struct includes a per-iteration map `iterModelUsage map[string]ModelTokens` and a cumulative map `totalModelUsage map[string]ModelTokens` (reuse or mirror the `ModelTokens` struct from messages.go)
- [ ] A new `addModelUsage(msg agent.ModelUsageMsg)` method accumulates per-model tokens into both iter and total maps
- [ ] `saveAndReset()` stores the per-iteration model map in `TaskUsage` (add a `ModelUsage map[string]ModelTokens` field to `TaskUsage`) and resets the iter map
- [ ] The TUI `Update()` method in `tui.go` routes `agent.ModelUsageMsg` to `m.tokens.addModelUsage(msg)`
- [ ] Typecheck/lint passes
- [ ] Unit tests verify per-model accumulation, save, and reset

### TASK-006: Update Live Progress Token Display
**Description:** As a user, I want the live progress view to show total input tokens (including cache) with a cache breakdown so I can see accurate usage while tasks run.

**Acceptance Criteria:**
- [ ] The token display in `tui_render.go` computes total input as `totalInput + totalCacheCreation + totalCacheRead`
- [ ] When cache tokens are present, display format is: `"<total_in> in / <out> out (cache: <write> write, <read> read)"`
- [ ] When cache tokens are zero, display format remains: `"<in> in / <out> out"`
- [ ] Cost is displayed on a separate line: `"Cost: $X.XX"` (or "N/A" if zero)
- [ ] Typecheck/lint passes

### TASK-007: Update Summary View Token Display
**Description:** As a user, I want the summary view to show accurate token totals with cache breakdown, cost, and per-model breakdown so I can understand resource usage after a run.

**Acceptance Criteria:**
- [ ] The overall token line in `tui_summary.go` shows total input (including cache) with cache breakdown when cache > 0
- [ ] A cost line is displayed: `"Cost: $X.XX"` (formatted to 2 decimal places, or more if < $0.01)
- [ ] Per-task breakdown shows total input (including cache) per task
- [ ] If per-model data is available, a "Per-Model Usage" section lists each model with its input (including cache), output, and cost
- [ ] Typecheck/lint passes

### TASK-008: Migrate Usage CSV to v2 Format
**Description:** As a developer, I want usage data persisted to `usage_v2.csv` with the new cache, cost, and per-model fields so that historical data accurately reflects real usage.

**Acceptance Criteria:**
- [ ] `usage.Record` struct includes `CacheCreationInputTokens int`, `CacheReadInputTokens int`, `CostUSD float64`, and `ModelUsage map[string]ModelTokens`
- [ ] The CSV filename constant is changed to `.maggus/usage_v2.csv`
- [ ] CSV header includes new columns: `cache_creation_input_tokens`, `cache_read_input_tokens`, `cost_usd` placed after `output_tokens`; per-model data is serialized as a JSON string in a `model_usage` column at the end
- [ ] `Append()` writes the new columns in each row
- [ ] The `setupUsageCallback` in `work_loop.go` passes cache, cost, and model usage fields from `TaskUsage` to `usage.Record`
- [ ] Typecheck/lint passes
- [ ] All existing usage tests are updated for the new column count, positions, and filename
- [ ] New test verifies cache, cost, and model_usage columns are written correctly

## Functional Requirements

- FR-1: The system must parse `cache_creation_input_tokens` and `cache_read_input_tokens` from Claude Code stream-json result events
- FR-2: The system must parse `total_cost_usd` from Claude Code stream-json result events
- FR-3: The system must parse `modelUsage` map from Claude Code stream-json result events
- FR-4: All token displays (live progress and summary) must show total input as `input_tokens + cache_creation_input_tokens + cache_read_input_tokens`
- FR-5: Cache breakdown (write/read) must be shown in all token display locations when cache tokens are non-zero
- FR-6: Cost must be displayed in both the live progress view and summary view
- FR-7: Per-model usage breakdown must be displayed in the summary view when multiple models are used
- FR-8: The usage CSV must be written to `.maggus/usage_v2.csv` with all new fields
- FR-9: Per-model usage in CSV must be serialized as a JSON string column for flexibility

## Non-Goals

- No migration tool for converting existing `usage.csv` to `usage_v2.csv`
- No reading/display of historical `usage.csv` data — the old file is simply left as-is
- No cost estimation or budgeting features — just report what Claude Code tells us
- No aggregation dashboard or reporting commands — raw CSV is sufficient
- No changes to the `status` or `list` commands

## Technical Considerations

- Claude Code's stream-json `result` event structure (verified via `claude -p "say hi" --output-format json`):
  ```json
  {
    "total_cost_usd": 0.0855,
    "usage": {
      "input_tokens": 3,
      "cache_creation_input_tokens": 13055,
      "cache_read_input_tokens": 6692,
      "output_tokens": 24
    },
    "modelUsage": {
      "claude-opus-4-6[1m]": {
        "inputTokens": 3,
        "outputTokens": 24,
        "cacheReadInputTokens": 6692,
        "cacheCreationInputTokens": 13055,
        "costUSD": 0.0855
      }
    }
  }
  ```
- Note the `modelUsage` map uses camelCase JSON keys, while `usage` uses snake_case
- The `modelUsage` map key includes the context window suffix (e.g., `[1m]`) — store as-is
- Per-model data in CSV as JSON avoids schema explosion if many models are used
- Cost formatting: use `$0.0000` (4 decimal places) for display since individual task costs can be small fractions of a dollar; for values >= $1.00 use 2 decimal places

## Success Metrics

- Token usage reported by maggus matches the total shown by Claude Code's own billing/usage
- A typical task that previously showed ~100 input tokens now correctly shows ~50k-200k total input
- Cost is visible and matches `total_cost_usd` from Claude Code
- Per-model breakdown is available when subagents use different models

## Open Questions

- None — all decisions captured from user answers
