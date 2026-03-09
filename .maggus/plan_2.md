# Plan: Ralph-style Work Loop

## Introduction

Adopt the battle-tested prompt structure and workflow from `ralph.sh` into the `maggus work` command. This means each iteration gets a rich bootstrap prompt (read CLAUDE.md/AGENTS.md, find next task, implement, verify, stage + prepare commit), run tracking with iteration logs, protected branch detection, and Maggus handling the `git commit` after each iteration — not the agent.

## Goals

- Match ralph.sh's core behavior: bootstrap → one task → verify → stage → commit
- Add run tracking with a `.maggus/runs/` directory and per-iteration logs
- Detect protected branches and auto-create a feature branch
- Handle `git commit` in Maggus after each iteration (agent only stages + writes COMMIT.md)
- Make bootstrap file reading configurable (`--no-bootstrap` to disable, enabled by default)

## User Stories

### TASK-001: Restructure prompt to match ralph.sh bootstrap pattern
**Description:** As Maggus, I want to send a ralph-style prompt so that the agent reads bootstrap files, works on exactly one task, verifies criteria, and stages files with a commit message.

**Acceptance Criteria:**
- [x] The prompt instructs the agent to read CLAUDE.md and/or AGENTS.md first (if they exist in the working directory)
- [x] The prompt instructs the agent to read PROJECT_CONTEXT.md and TOOLING.md if present
- [x] The prompt includes run metadata: RUN_ID, RUN_DIR, ITERATION number, ITER_LOG path
- [x] The prompt includes the full task (ID, title, description, acceptance criteria)
- [x] The prompt instructs: "Work on this ONE task only. Do NOT continue to additional tasks."
- [x] The prompt instructs the agent to verify all acceptance criteria before finishing
- [x] The prompt instructs the agent to stage all changed files but NOT commit
- [x] The prompt instructs the agent to write a commit message to COMMIT.md
- [x] The prompt instructs the agent to update the plan file checkboxes for completed criteria
- [x] The prompt instructs the agent to write an iteration log to ITER_LOG before finishing
- [x] The iteration log should include: task selected, commands run + outcomes, deviations/skips
- [x] `--no-bootstrap` flag disables reading CLAUDE.md/AGENTS.md/PROJECT_CONTEXT.md/TOOLING.md
- [x] Unit tests verify the prompt contains all expected sections
- [x] Unit tests verify `--no-bootstrap` omits the bootstrap section
- [x] Typecheck/lint passes

### TASK-002: Add run tracking (run ID, run directory, iteration logs)
**Description:** As a user, I want each `maggus work` invocation to create a run directory with logs so that I can review what happened.

**Acceptance Criteria:**
- [x] Each `maggus work` invocation generates a RUN_ID in format `YYYYMMDD-HHMMSS`
- [x] A run directory is created at `.maggus/runs/<RUN_ID>/`
- [x] A `run.md` file is written at the start with: RUN_ID, branch, model, iteration count, start commit, start time
- [x] After the loop finishes, `run.md` is appended with: end time, end commit, commit range
- [x] Each iteration's log path is `.maggus/runs/<RUN_ID>/iteration-<NN>.md` (zero-padded)
- [x] The run directory path and iteration log path are passed to the agent via the prompt
- [x] At the end, Maggus prints a summary banner with RUN_ID, branch, logs path, and commit range
- [x] Typecheck/lint passes

### TASK-003: Protected branch detection and feature branch creation
**Description:** As a user, I want Maggus to automatically create a feature branch if I'm on a protected branch so that I don't accidentally commit to main/master/dev.

**Acceptance Criteria:**
- [x] Before starting the loop, Maggus checks the current git branch
- [x] If the branch is `main`, `master`, or `dev`, Maggus creates a branch like `feature/-maggustask-<NNN>`, NNN is the zero-padded task number .
- [x] Maggus prints which branch it switched to
- [x] If already on a non-protected branch, Maggus stays on it and prints the branch name
- [x] If git is not available or the directory is not a git repo, Maggus prints a warning but continues (no branch switching)
- [x] Unit tests cover: protected branch detection, branch name generation
- [x] Typecheck/lint passes

### TASK-004: Git commit after each iteration
**Description:** As Maggus, I want to run `git commit` after each iteration using the COMMIT.md file so that the agent doesn't handle commits.

**Acceptance Criteria:**
- [x] After Claude Code exits successfully, Maggus checks if COMMIT.md exists in the working directory
- [x] If COMMIT.md exists, Maggus runs `git commit -F COMMIT.md`
- [x] Any `Co-Authored-By` lines are stripped from COMMIT.md before committing
- [x] If the commit fails (nothing staged, merge conflict, etc.), Maggus logs the error and stops the loop
- [x] If COMMIT.md does not exist, Maggus logs a warning and continues to the next iteration (agent may not have made changes)
- [x] After a successful commit, COMMIT.md is deleted
- [x] Typecheck/lint passes

### TASK-005: Wire everything together in the work loop
**Description:** As a user, I want the full ralph-style loop wired together so that `maggus work` runs end-to-end with tracking, branching, prompting, and committing.

**Acceptance Criteria:**
- [x] The work command flow is: detect branch → create run dir → loop (find task → build prompt → invoke claude → git commit) → print summary
- [x] The iteration counter in the prompt matches the actual iteration (1-based)
- [x] The loop stops when: count reached, no incomplete tasks remain, or claude exits non-zero
- [x] Before each iteration, Maggus prints a banner: `========== Iteration <i> of <N> ==========`
- [x] After the loop, Maggus prints remaining incomplete tasks (title only, max 5)
- [x] The `--no-bootstrap` flag is wired through from the work command to the prompt builder
- [x] Existing unit tests still pass
- [x] Typecheck/lint passes

### TASK-006: Add startup banner and safety pause
**Description:** As a user, I want to see a clear banner at startup showing what Maggus is about to do, with a brief pause to abort.

**Acceptance Criteria:**
- [ ] Before starting the loop, Maggus prints a banner with: iteration count, branch, run ID, run directory, permissions mode
- [ ] Maggus prints "WARNING: Running with --dangerously-skip-permissions"
- [ ] Maggus prints "Press Ctrl+C within 3 seconds to abort..." and waits 3 seconds
- [ ] Ctrl+C during the pause cleanly exits without errors
- [ ] Typecheck/lint passes

## Functional Requirements

- FR-1: The prompt must instruct the agent to read CLAUDE.md/AGENTS.md before working (unless `--no-bootstrap`)
- FR-2: The prompt must include run metadata (RUN_ID, RUN_DIR, ITERATION, ITER_LOG)
- FR-3: The prompt must enforce "ONE task only" discipline
- FR-4: The agent must stage files and write COMMIT.md but NOT commit
- FR-5: Maggus handles `git commit -F COMMIT.md` after each iteration
- FR-6: Co-Authored-By lines are stripped from commit messages
- FR-7: Protected branches (main/master/dev) trigger automatic feature branch creation
- FR-8: Each run creates a `.maggus/runs/<RUN_ID>/` directory with `run.md` and per-iteration logs
- FR-9: `--no-bootstrap` flag disables bootstrap file instructions in the prompt
- FR-10: The startup banner shows configuration and gives 3 seconds to abort

## Non-Goals

- No adversarial review / mid-loop review (ralph-specific, too complex for now)
- No L3 verification gate (ralph-specific)
- No postmortem after the loop
- No `--model` flag (future feature)
- No pushing to remote (user does that manually)
- No TOOLING.md auto-update

## Technical Considerations

- Run ID uses `time.Now().Format("20060102-150405")` (Go's reference time format)
- Git operations use `os/exec` — same pattern as the claude runner
- COMMIT.md is read, stripped, used for commit, then deleted — all in Maggus, not the agent
- The prompt builder needs to accept options (bootstrap on/off, run metadata) — use a struct or functional options
- Keep the existing `internal/prompt` and `internal/runner` packages, extend them

## Success Metrics

- `maggus work` produces the same style of prompt and workflow as ralph.sh
- Each iteration creates a git commit with a clean message
- Run logs in `.maggus/runs/` provide a clear audit trail
- Protected branch detection prevents accidental commits to main/master/dev

## Open Questions

- Should Maggus support a `--model` flag to override the Claude model? (Deferred)
- Should Maggus push to remote after the loop like ralph.sh does? (Deferred — user can push manually)
- Should there be a `maggus postmortem` command? (Deferred)
