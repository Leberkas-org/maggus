# Plan: Usage Tracking Overhaul & Interactive Prompt Command

## Introduction

Rethink usage data persistence from CSV to JSON Lines (`.jsonl`) for easier REST API integration, add a `maggus prompt` command that launches Claude Code interactively with full stdin/stdout passthrough, and extract usage data from Claude's session files after the session ends. This gives users a way to use Claude directly through maggus while still capturing usage metrics for future API reporting.

## Goals

- Replace `usage_v2.csv` with `usage_work.jsonl` and `usage_prompt.jsonl` (JSON Lines format)
- Provide a `maggus prompt` command that launches Claude Code interactively (full terminal passthrough)
- After an interactive session ends, parse Claude's session JSONL files to extract token usage and record it
- Keep all usage files gitignored
- Maintain backward compatibility — existing `work` command continues to function, just writes JSONL instead of CSV

## User Stories

### TASK-001: Replace CSV Usage Writer with JSONL

**Description:** As a developer, I want usage data written as JSON Lines so that each record can be trivially deserialized and POST-ed to a REST API without CSV parsing.

**Acceptance Criteria:**
- [x] New `internal/usage/jsonl.go` (or refactored `usage.go`) writes records as one JSON object per line to `.maggus/usage_work.jsonl`
- [x] The `usage.Record` struct gains JSON struct tags
- [x] `usage.Append` writes JSON Lines instead of CSV rows
- [x] Header row logic is removed (JSONL is self-describing)
- [x] Existing call sites in `work_loop.go` continue to work without changes beyond import/type adjustments
- [x] Old `usage_v2.csv` code is removed
- [x] Unit tests verify JSONL append creates valid JSON on each line
- [x] `go vet ./...` and `go test ./...` pass

### TASK-002: Add `usage_work.jsonl` and `usage_prompt.jsonl` to Gitignore

**Description:** As a developer, I want the new JSONL usage files automatically gitignored so they are never committed.

**Acceptance Criteria:**
- [x] `internal/gitignore/gitignore.go` `requiredEntries` includes `.maggus/usage_work.jsonl` and `.maggus/usage_prompt.jsonl`
- [x] Remove the old `.maggus/usage_v2.csv` entry from `requiredEntries` (keep `.maggus/usage.csv` for backward compat with older repos)
- [x] `init_test.go` updated to expect the new entries
- [x] `go test ./...` passes

### TASK-003: Add `maggus prompt` Command with Interactive Claude Passthrough

**Description:** As a user, I want to run `maggus prompt` to launch Claude Code interactively with full terminal control (stdin/stdout/stderr passthrough) so I can have a normal conversation while maggus tracks usage afterward.

**Acceptance Criteria:**
- [x] New `cmd/prompt.go` registers a `prompt` command on the root cobra command
- [x] The command accepts an optional `--model` flag (uses config default if not set)
- [x] The command resolves the model alias via `config.ResolveModel` (same as `work`)
- [x] Claude Code is launched interactively: `claude --model <model> --dangerously-skip-permissions` (no `-p`, no `--output-format`)
- [x] stdin, stdout, and stderr are connected directly to the terminal (full passthrough, not piped)
- [x] ⚠️ BLOCKED: The command captures the Claude session ID from the process (see TASK-004) — TASK-004 not yet implemented; TODO placeholder added
- [x] ⚠️ BLOCKED: On exit (normal or Ctrl+C), usage extraction is triggered (TASK-005) — TASK-005 not yet implemented; TODO placeholder added
- [x] `go vet ./...` passes

### TASK-004: Detect Claude Session ID for Post-Hoc Usage Extraction

**Description:** As a developer, I need to identify which Claude session file corresponds to the interactive session maggus just launched, so I can extract usage data from it.

**Acceptance Criteria:**
- [x] New `internal/session/detect.go` package with a function to find the session file
- [x] Detection strategy: snapshot `.claude/projects/<project-hash>/` directory listing before launch, then after Claude exits, find the new `.jsonl` file(s) that appeared (diff approach)
- [x] The project hash is derived from the current working directory path (Claude uses path with separators replaced by `-`, e.g. `C--c-maggus` for `C:\c\maggus` — verify the exact hashing/encoding logic by reading existing session directories)
- [x] Falls back gracefully if no new session file is found (log warning, skip usage extraction)
- [x] Unit tests cover the diff-based detection with a temp directory
- [x] `go test ./...` passes

### TASK-005: Extract Usage from Claude Session JSONL Files

**Description:** As a developer, I want to parse a Claude session JSONL file to sum up all token usage across the session and write a single usage record to `usage_prompt.jsonl`.

**Acceptance Criteria:**
- [x] New `internal/session/extract.go` with a function that reads a session JSONL, finds all `"type":"assistant"` entries, and sums their `usage` fields
- [x] Extracts: `input_tokens`, `output_tokens`, `cache_creation_input_tokens`, `cache_read_input_tokens` per model
- [x] Groups usage by model (from the `model` field on each assistant message) to produce per-model breakdown
- [x] Cost is NOT calculated (no pricing table needed) — the `CostUSD` field is set to 0 or omitted; the future REST API will handle pricing
- [x] Returns a `usage.Record` (or similar struct) that can be appended to `usage_prompt.jsonl`
- [x] Handles malformed lines gracefully (skip and continue)
- [x] Unit tests with sample JSONL input verify correct summation
- [x] `go test ./...` passes

### TASK-006: Wire Usage Extraction into the Prompt Command

**Description:** As a user, I want usage data from my interactive session automatically saved to `usage_prompt.jsonl` when the session ends.

**Acceptance Criteria:**
- [x] After Claude exits in `cmd/prompt.go`, the session detection (TASK-004) and extraction (TASK-005) run
- [x] The extracted usage record is appended to `.maggus/usage_prompt.jsonl` via the JSONL writer from TASK-001
- [x] The record includes: a generated run ID (timestamp-based, same format as `work`), model, agent name ("claude"), start time, end time, and token totals
- [x] If extraction fails (no session found, parse error), a warning is printed but the command exits cleanly (no error exit code)
- [ ] Manual test: run `maggus prompt`, have a short conversation, exit, verify `usage_prompt.jsonl` contains a valid JSON line with token counts
- [x] `go vet ./...` and `go test ./...` pass

### TASK-007: Add Prompt Command to TUI Menu

**Description:** As a user, I want to see "Prompt" as an option in the maggus interactive menu so I can launch it without remembering the CLI syntax.

**Acceptance Criteria:**
- [ ] The TUI menu (`cmd/menu.go`) includes a "Prompt" option that runs the prompt command
- [ ] Menu item has a clear description (e.g. "Launch interactive Claude session with usage tracking")
- [ ] `go test ./...` passes

## Functional Requirements

- FR-1: Usage records must be written as JSON Lines — one complete JSON object per line, newline-terminated
- FR-2: Each JSON line must be independently parseable (no array wrapper, no trailing commas)
- FR-3: The `prompt` command must give Claude full terminal control — no TUI overlay, no spinner, raw passthrough
- FR-4: Session file detection must handle the case where Claude creates the session file slightly after process start (allow small time window)
- FR-5: Usage extraction must not crash on empty or partially-written session files
- FR-6: Both `usage_work.jsonl` and `usage_prompt.jsonl` must be in `.gitignore` via the gitignore package
- FR-7: The JSONL writer must be shared between `work` and `prompt` — same `usage.Append` function, just different file paths

## Non-Goals

- No REST API client implementation — that's a future feature; we're just preparing the data format
- No cost/pricing calculation — the API will handle this; we only store raw token counts
- No conversation history display in maggus — Claude handles its own UI during interactive sessions
- No migration tool for existing `usage_v2.csv` data to JSONL
- No changes to the `work` command's TUI or workflow beyond switching the output format

## Technical Considerations

- Claude session files live at `~/.claude/projects/<PROJECT_HASH>/<SESSION_UUID>.jsonl`
- The project hash appears to be the absolute path with path separators replaced by dashes (e.g. `C:\c\maggus` → `C--c-maggus`) — this needs verification during implementation
- Session JSONL contains `"type":"assistant"` lines with a `usage` object but NO `cost` field — cost must come from elsewhere
- On Windows, Claude subprocess needs proper signal handling for clean shutdown (existing `signals_windows.go` patterns)
- The `usage.Append` function should accept a file path parameter (or a "kind" enum for work/prompt) rather than hardcoding the filename

## Success Metrics

- `maggus prompt` launches Claude interactively with zero noticeable overhead
- After exiting an interactive session, `usage_prompt.jsonl` contains accurate token counts matching what Claude reported
- `usage_work.jsonl` is produced by the `work` command with the same data as before, just in JSONL format
- All new usage files are gitignored automatically

## Open Questions

- Should `maggus prompt` pass through additional Claude flags (e.g. `--allowedTools`, `--mcp-config`)? Deferred for now — keep it simple.
- Should there be a `maggus usage` command to view/summarize usage data from the JSONL files? Could be a follow-up feature.
