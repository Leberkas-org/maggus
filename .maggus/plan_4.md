# Plan: Comprehensive Test Coverage

## Introduction

Maggus has 207 tests across 21 test files, but significant gaps remain — 11 of 17 cmd files and 2 internal packages have zero tests. This plan adds tests for all untested code, prioritized by risk (critical paths first, UI/display last). No refactoring — tests are written for the existing code as-is. All tests must run without external dependencies (no Claude Code or OpenCode CLI required).

## Goals

- Achieve test coverage for every package and command in the codebase
- Prioritize critical business logic (ignore/unignore, work loop helpers, dispatch) over display-only code
- All tests runnable via `go test ./...` with no external tools needed
- Follow existing test patterns: `t.TempDir()`, table-driven tests, helper functions with `t.Helper()`

## User Stories

### TASK-001: Add tests for ignore command logic
**Description:** As a developer, I want tests for `cmd/ignore.go` so that plan and task ignore operations are verified.

**Acceptance Criteria:**
- [x] Test `findPlanFile` returns correct state (active, ignored, completed, not found)
- [x] Test `findPlanFile` does not match partial IDs (plan_3 vs plan_30)
- [x] Test `runIgnorePlan` on an active plan renames to `_ignored.md`
- [x] Test `runIgnorePlan` on an already-ignored plan returns nil (idempotent)
- [x] Test `runIgnorePlan` on a completed plan returns error
- [x] Test `runIgnorePlan` on a missing plan returns error
- [x] Test `runIgnoreTask` rewrites heading from `### TASK-NNN:` to `### IGNORED TASK-NNN:`
- [x] Test `runIgnoreTask` on an already-ignored task returns nil (idempotent)
- [x] Test `runIgnoreTask` with bare ID (e.g. "007") normalizes to "TASK-007"
- [x] Test `runIgnoreTask` on a missing task returns error
- [x] Test `rewriteTaskHeading` performs atomic write (temp file + rename)
- [x] Unit tests are written and successful

### TASK-002: Add tests for unignore command logic
**Description:** As a developer, I want tests for `cmd/unignore.go` so that plan and task unignore operations are verified.

**Acceptance Criteria:**
- [x] Test `runUnignorePlan` on an ignored plan renames back to `.md`
- [x] Test `runUnignorePlan` on an active plan returns error ("not currently ignored")
- [x] Test `runUnignorePlan` on a completed plan returns error
- [x] Test `runUnignorePlan` on a missing plan returns error
- [x] Test `runUnignoreTask` rewrites heading from `### IGNORED TASK-NNN:` to `### TASK-NNN:`
- [x] Test `runUnignoreTask` on a non-ignored task returns error
- [x] Test `runUnignoreTask` with bare ID normalizes to "TASK-007"
- [x] Test `runUnignoreTask` on a missing task returns error
- [x] Unit tests are written and successful

### TASK-003: Add tests for usage package
**Description:** As a developer, I want tests for `internal/usage` so that CSV recording of token usage is verified.

**Acceptance Criteria:**
- [x] Test `Append` creates file with header row when file does not exist
- [x] Test `Append` appends rows without duplicating header on subsequent calls
- [x] Test `Append` with empty records slice is a no-op (returns nil, no file created)
- [x] Test `Append` writes correct CSV columns matching `header()` order
- [x] Test elapsed time calculation is correct (EndTime - StartTime truncated to seconds)
- [x] Test `Append` returns error when directory does not exist
- [x] Unit tests are written and successful

### TASK-004: Add tests for capabilities package
**Description:** As a developer, I want tests for `internal/capabilities` so that tool detection and caching logic is verified.

**Acceptance Criteria:**
- [x] Test `Load` returns zero-value when cache file does not exist
- [x] Test `Load` returns zero-value when cache file contains invalid JSON
- [x] Test `Load` correctly deserializes a valid capabilities JSON file
- [x] Test `write` creates directory structure and writes valid JSON
- [x] Test `configFile` returns a non-empty path
- [x] Unit tests are written and successful

### TASK-005: Add tests for agent registry and message types
**Description:** As a developer, I want tests for `internal/agent/messages.go` and additional coverage for `agent.go` interface contracts.

**Acceptance Criteria:**
- [x] Test that `ErrInterrupted` error message is stable
- [x] Test that message types (StatusMsg, OutputMsg, ToolMsg, SkillMsg, MCPMsg, UsageMsg) can be instantiated with expected fields
- [x] Test `ToolMsg.Timestamp` preserves time precision
- [x] Unit tests are written and successful

### TASK-006: Add tests for work command helpers
**Description:** As a developer, I want tests for the helper functions in `cmd/work.go` so that task selection logic is verified.

**Acceptance Criteria:**
- [x] Test `findTaskByID` returns the correct task when it exists and is incomplete
- [x] Test `findTaskByID` returns nil when task is complete
- [x] Test `findTaskByID` returns nil when task ID does not exist
- [x] Test `findNextUnlocked` returns first workable unlocked task
- [x] Test `findNextUnlocked` skips locked tasks (requires setting up a lock file)
- [x] Test `findNextUnlocked` returns nil when all workable tasks are locked
- [x] Unit tests are written and successful

### TASK-007: Add tests for dispatch command
**Description:** As a developer, I want tests for `cmd/dispatch.go` so that the `dispatchWork` function is verified.

**Acceptance Criteria:**
- [x] Test `dispatchWork` correctly finds and configures the work subcommand
- [x] Test that the `--task` flag is correctly set on the subcommand
- [x] Unit tests are written and successful

### TASK-008: Add tests for init command
**Description:** As a developer, I want tests for `cmd/init.go` so that project initialization is verified.

**Acceptance Criteria:**
- [x] Test that `maggus init` creates `.maggus/` directory
- [x] Test that `maggus init` creates default `config.yml`
- [x] Test that `maggus init` is idempotent (running twice does not error or overwrite)
- [x] Test that `.gitignore` entries are added
- [x] Unit tests are written and successful

### TASK-009: Add tests for gitsync command (TUI-free parts)
**Description:** As a developer, I want tests for the non-TUI logic in `cmd/gitsync.go` so that sync result handling is verified.

**Acceptance Criteria:**
- [ ] Test sync result types and constants are consistent
- [ ] Test any helper functions that don't require a running TUI
- [ ] Unit tests are written and successful

BLOCKED: Need to read gitsync.go to determine which parts are testable without bubbletea

### TASK-010: Add tests for config command (TUI-free parts)
**Description:** As a developer, I want tests for any testable non-TUI logic in `cmd/config.go`.

**Acceptance Criteria:**
- [ ] Test config loading and display logic if extractable
- [ ] Unit tests are written and successful

BLOCKED: Need to read config.go to determine testable surface

### TASK-011: Add tests for detail command helpers
**Description:** As a developer, I want tests for any helper functions in `cmd/detail.go` that don't require a TUI.

**Acceptance Criteria:**
- [ ] Test any data transformation or formatting helpers
- [ ] Unit tests are written and successful

BLOCKED: Need to read detail.go to determine testable surface

### TASK-012: Add tests for status command helpers
**Description:** As a developer, I want tests for any non-TUI helper functions in `cmd/status.go`.

**Acceptance Criteria:**
- [ ] Test plan/task aggregation logic if extractable
- [ ] Test plain-text output rendering if it has a separate function
- [ ] Unit tests are written and successful

BLOCKED: Need to read status.go to determine testable surface

### TASK-013: Add tests for menu command helpers
**Description:** As a developer, I want tests for any non-TUI helpers in `cmd/menu.go`.

**Acceptance Criteria:**
- [ ] Test command availability filtering logic if extractable
- [ ] Unit tests are written and successful

BLOCKED: Need to read menu.go to determine testable surface

### TASK-014: Add tests for plan command
**Description:** As a developer, I want tests for `cmd/plan.go` non-interactive logic.

**Acceptance Criteria:**
- [ ] Test argument parsing and prompt assembly if extractable
- [ ] Unit tests are written and successful

BLOCKED: Need to read plan.go to determine testable surface — likely limited since it launches Claude Code

## Functional Requirements

- FR-1: All new tests must be in `*_test.go` files in the same package as the code under test
- FR-2: Tests must use `t.TempDir()` for any filesystem operations
- FR-3: Tests must not require `claude`, `opencode`, or any external CLI tool on PATH
- FR-4: Tests must not require network access or a git remote
- FR-5: Tests for git operations may initialize a local git repo in a temp dir using `git init`
- FR-6: Tests must follow existing patterns: standard library `testing`, table-driven where appropriate, `t.Helper()` on helpers
- FR-7: All tests must pass on `go test ./...` from the `src/` directory

## Non-Goals

- No refactoring of production code to improve testability
- No mocking frameworks or third-party test libraries
- No TUI/bubbletea integration tests (these require terminal simulation)
- No tests that invoke Claude Code or OpenCode as subprocesses
- No benchmark tests or fuzzing in this iteration
- No coverage threshold enforcement (CI change)

## Technical Considerations

- The `cmd` package tests can use `cobra.Command` copies with `SetOut`/`SetErr` buffers (pattern from `clean_test.go`)
- Git-dependent tests should use the `initGitRepo` helper pattern from `runtracker_test.go`
- The `work.go` file is 737 lines with heavy TUI/subprocess coupling — only the pure helper functions (`findTaskByID`, `findNextUnlocked`) are testable without refactoring
- BLOCKED tasks (TASK-008 through TASK-014) need investigation of the actual source files to determine testable surface area; they may yield zero testable functions if the code is purely TUI

## Success Metrics

- `go test ./...` passes with no failures
- Every `internal/` package has at least one test file
- Every `cmd/*.go` file with extractable logic has corresponding tests
- Zero flaky tests (no timing dependencies, no network calls)

## Open Questions

- Should TASK-008 through TASK-014 be consolidated if investigation shows minimal testable surface?
- Should we add a CI step for coverage reporting after this plan completes?
