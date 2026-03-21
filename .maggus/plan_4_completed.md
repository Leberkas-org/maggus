# Plan: Token Usage Tracking for Skill Commands (vision, architecture, plan)

## Introduction

The `maggus plan`, `maggus vision`, and `maggus architecture` commands launch Claude Code interactively but do not track token usage. The `prompt` command already solves this exact problem — it snapshots the Claude session directory before launch, detects the new session file afterward, extracts usage from the JSONL, and appends a record. This plan applies the same pattern to the three skill commands, writing to separate per-command usage files.

## Goals

- Track token usage (input, output, cache creation, cache read) and per-model breakdown for `plan`, `vision`, and `architecture` commands
- Store usage records in dedicated JSONL files: `usage_plan.jsonl`, `usage_vision.jsonl`, `usage_architecture.jsonl`
- Silently log usage without displaying a summary to the user
- Reuse the existing `session` and `usage` packages — no new infrastructure

## User Stories

### TASK-001: Refactor launchInteractive to support usage tracking
**Description:** As a developer, I want `launchInteractive` to optionally capture session usage so that skill commands can track tokens without duplicating the prompt command's logic.

**Acceptance Criteria:**
- [x] `launchInteractive` in `src/cmd/plan.go` accepts the working directory, performs session directory snapshotting before launch, and returns session timing info
- [x] After the interactive session ends, the caller can extract usage from the detected session file
- [x] The existing `plan`, `vision`, and `architecture` commands call the updated function
- [x] The function handles errors gracefully (warnings to stderr, never causes non-zero exit)
- [x] Typecheck/lint passes
- [x] Unit tests verify the new function signature and that usage extraction is wired up

### TASK-002: Wire usage extraction into plan command
**Description:** As a user running `maggus plan`, I want token usage silently logged to `.maggus/usage_plan.jsonl` so that I can track costs over time.

**Acceptance Criteria:**
- [x] After a `maggus plan` session ends, usage is extracted from the Claude session file
- [x] A `usage.Record` is appended to `.maggus/usage_plan.jsonl` with: run ID (timestamp), model, agent, all token fields, model usage map, start/end time
- [x] If session detection or extraction fails, a warning is printed to stderr and the command exits successfully
- [x] Typecheck/lint passes
- [x] Unit tests are written and successful

### TASK-003: Wire usage extraction into vision command
**Description:** As a user running `maggus vision`, I want token usage silently logged to `.maggus/usage_vision.jsonl` so that I can track costs over time.

**Acceptance Criteria:**
- [x] After a `maggus vision` session ends, usage is extracted from the Claude session file
- [x] A `usage.Record` is appended to `.maggus/usage_vision.jsonl` with the same fields as TASK-002
- [x] If session detection or extraction fails, a warning is printed to stderr and the command exits successfully
- [x] Typecheck/lint passes
- [x] Unit tests are written and successful

### TASK-004: Wire usage extraction into architecture command
**Description:** As a user running `maggus architecture`, I want token usage silently logged to `.maggus/usage_architecture.jsonl` so that I can track costs over time.

**Acceptance Criteria:**
- [x] After a `maggus architecture` session ends, usage is extracted from the Claude session file
- [x] A `usage.Record` is appended to `.maggus/usage_architecture.jsonl` with the same fields as TASK-002
- [x] If session detection or extraction fails, a warning is printed to stderr and the command exits successfully
- [x] Typecheck/lint passes
- [x] Unit tests are written and successful

## Functional Requirements

- FR-1: Before launching the interactive Claude session, the system must snapshot the Claude session directory to detect new session files afterward
- FR-2: After the session ends, the system must diff the session directory against the snapshot and find the new `.jsonl` session file
- FR-3: The system must parse the session file using `session.ExtractUsage()` to sum token usage per model
- FR-4: The system must write a `usage.Record` to the command-specific JSONL file using `usage.AppendTo()`
- FR-5: Usage files must be named `usage_plan.jsonl`, `usage_vision.jsonl`, and `usage_architecture.jsonl` in the `.maggus/` directory
- FR-6: The run ID must be the session start timestamp formatted as `20060102-150405`
- FR-7: All errors during usage extraction must be handled as warnings (printed to stderr) and must not cause the command to fail

## Non-Goals

- No TUI summary display after skill sessions — usage is logged silently
- No changes to the `prompt` command's existing usage tracking
- No changes to the `work` command's usage tracking
- No aggregation or reporting across usage files (that's a separate feature)
- No cost calculation — `CostUSD` is set to 0 (same as prompt command, since session files don't include cost)

## Technical Considerations

- The `prompt` command (`src/cmd/prompt.go`) is the reference implementation — follow the same pattern exactly: snapshot → launch → detect → extract → append
- `launchInteractive` currently takes just `agentName` and `prompt` — it needs to be extended with `dir` and usage-file parameters, or replaced with a shared helper
- Signal handling (forwarding SIGINT to child) should be added to match the prompt command's behavior
- The `session` and `usage` packages are already battle-tested and need no changes
- Config loading is needed to resolve the model for the usage record

## Success Metrics

- All three commands (`plan`, `vision`, `architecture`) produce usage JSONL files after interactive sessions
- Usage records contain accurate per-model token breakdowns
- No user-visible behavior change (no new output, no new prompts)
- Existing tests continue to pass; new tests cover the usage extraction wiring
