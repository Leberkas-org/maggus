# Plan: Maggus CLI — PoC

## Introduction

Maggus is a Go CLI tool that reads implementation plans (`.maggus/plan_*.md`) and works through the listed tasks one-by-one by prompting Claude Code. The user specifies how many tasks to process (default: 5), and Maggus sequentially parses each task, builds a focused prompt, shells out to `claude -p`, and moves on to the next incomplete task.

This PoC covers the core loop: parse plan → find next task → prompt agent → repeat.

## Goals

- Parse `.maggus/plan_*.md` files and extract tasks with their acceptance criteria
- Determine task completion by checking if all acceptance criteria checkboxes are marked `[x]`
- Build a focused prompt per task and shell out to `claude -p`
- Process tasks sequentially, one at a time, up to the user-specified count
- Provide a simple CLI via Cobra: `maggus work [count]`

## User Stories

### TASK-001: Initialize Go project with Cobra
**Description:** As a developer, I want a Go project scaffolded with Cobra so that I have a working CLI entry point.

**Acceptance Criteria:**
- [x] `go.mod` initialized with a sensible module name (e.g., `github.com/dirnei/maggus`)
- [x] Cobra dependency added
- [x] `main.go` exists and calls the root command
- [x] `cmd/root.go` defines the root command with a basic description
- [x] `maggus --help` prints usage info
- [x] Project builds successfully with `go build`

### TASK-002: Add the `work` subcommand
**Description:** As a user, I want a `maggus work` command so that I can tell Maggus to start working on tasks.

**Acceptance Criteria:**
- [x] `cmd/work.go` defines the `work` subcommand under root
- [x] Accepts an optional positional argument for task count (e.g., `maggus work 10`)
- [x] Accepts a `--count` / `-c` flag as alternative (e.g., `maggus work --count 10`)
- [x] Defaults to `5` if no count is provided
- [x] Positional argument takes precedence over flag if both are given
- [x] `maggus work --help` prints usage info explaining the count option
- [x] For now, the command just prints: `Starting work on <N> tasks...`

### TASK-003: Parse plan files and extract tasks
**Description:** As Maggus, I want to parse `.maggus/plan_*.md` files so that I can extract the list of tasks with their descriptions and acceptance criteria.

**Acceptance Criteria:**
- [x] A `parser` package exists (e.g., `internal/parser/`)
- [x] Parser reads all `.maggus/plan_*.md` files from the current working directory
- [x] Each `### TASK-xxx: Title` heading is extracted as a task
- [x] The `**Description:**` text is extracted per task
- [x] The `**Acceptance Criteria:**` checkboxes are extracted per task
- [x] Each criterion tracks its checked state (`[x]` = done, `[ ]` = open)
- [x] A task is considered "complete" when all its criteria are `[x]`
- [x] Unit tests cover: parsing a valid plan, extracting multiple tasks, detecting complete vs. incomplete tasks
- [x] Typecheck/lint passes

### TASK-004: Find the next incomplete task
**Description:** As Maggus, I want to find the next incomplete task so that I know what to work on.

**Acceptance Criteria:**
- [x] A function returns the next incomplete task (first task where any criterion is `[ ]`)
- [x] If all tasks are complete, it returns nil/empty with an appropriate indication
- [x] Tasks are ordered by their appearance in the file (TASK-001 before TASK-002, etc.)
- [x] If multiple plan files exist, tasks from earlier files (by filename sort) come first
- [x] Unit tests cover: finding next task, all-done scenario, ordering across files
- [x] Typecheck/lint passes

### TASK-005: Build the prompt for Claude Code
**Description:** As Maggus, I want to build a focused prompt for a single task so that Claude Code knows exactly what to implement.

**Acceptance Criteria:**
- [x] A `prompt` package exists (e.g., `internal/prompt/`)
- [x] The prompt includes the task title, description, and all acceptance criteria
- [x] The prompt instructs the agent to work on this specific task only
- [x] The prompt tells the agent to verify acceptance criteria before finishing
- [x] Unit test verifies prompt contains all task fields
- [x] Typecheck/lint passes

### TASK-006: Shell out to Claude Code
**Description:** As Maggus, I want to execute `claude -p "<prompt>"` so that the AI agent works on the task.

**Acceptance Criteria:**
- [x] Maggus invokes `claude` via `os/exec` with the `-p` flag and the built prompt
- [x] stdout and stderr from claude are streamed to the user's terminal in real-time
- [x] If `claude` exits with a non-zero code, Maggus logs the error and stops processing further tasks
- [x] If `claude` is not found on PATH, a clear error message is shown
- [x] Typecheck/lint passes

### TASK-007: Wire the work loop together
**Description:** As a user, I want `maggus work` to process N tasks sequentially so that Maggus works through my plan.

**Acceptance Criteria:**
- [x] The `work` command parses plans, finds the next incomplete task, builds the prompt, and invokes Claude Code
- [x] After each task completes, Maggus moves to the next incomplete task
- [x] Processing stops after N tasks (user-specified count) or when no incomplete tasks remain
- [x] Before each task, Maggus prints which task it is starting (e.g., `[1/5] Working on TASK-003: Parse plan files...`)
- [x] After all tasks are done (or count reached), Maggus prints a summary (e.g., `Completed 3/5 tasks. 2 tasks remaining.`)
- [x] Typecheck/lint passes

## Functional Requirements

- FR-1: The CLI must be built with Go and Cobra
- FR-2: `maggus work` defaults to processing 5 tasks
- FR-3: `maggus work 10` and `maggus work --count 10` both set the task count to 10
- FR-4: Tasks are parsed from `### TASK-xxx:` headings in `.maggus/plan_*.md` files
- FR-5: A task's `**Description:**` and `**Acceptance Criteria:**` sections are extracted
- FR-6: A task is "complete" when all `- [x]` checkboxes are checked
- FR-7: Maggus always works on the next incomplete task in file/document order
- FR-8: The prompt sent to `claude -p` contains the task title, description, and acceptance criteria
- FR-9: Claude Code is invoked via shell (`os/exec`), streaming output to the terminal
- FR-10: If Claude Code fails (non-zero exit), Maggus stops and reports the error
- FR-11: Maggus processes tasks one at a time, sequentially

## Non-Goals

- No web UI or service backend (that's a future feature)
- No parallel task execution
- No task selection or skipping — strictly sequential
- No updating checkboxes in the plan file after agent completion (future feature)
- No support for AI agents other than Claude Code in this PoC
- No configuration file — all options via CLI flags

## Technical Considerations

- Go 1.22+ recommended
- Cobra v1.8+ for CLI scaffolding
- The plan parser works with regex or a simple line-by-line state machine — no need for a full markdown AST parser
- `claude` must be available on PATH; Maggus does not install or manage it
- Plan files may contain sections other than tasks (Introduction, Goals, etc.) — the parser should skip those

## Success Metrics

- `maggus work` successfully parses a plan file and invokes Claude Code for the first incomplete task
- Tasks are processed one-by-one up to the specified count
- A user can create a plan with `/maggus-plan:plan`, then run `maggus work` to start implementation

## Open Questions

- Should Maggus update the checkboxes in the plan file after Claude Code completes a task, or leave that to the agent/user? (Deferred to post-PoC)
- Should there be a `maggus status` command to show task progress? (Deferred to post-PoC)
