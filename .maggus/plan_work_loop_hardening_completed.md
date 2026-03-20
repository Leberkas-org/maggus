# Plan: Work Loop Hardening

## Introduction

Two related weaknesses in the Maggus work loop: (1) when an agent omits `COMMIT.md`, staged changes are silently dropped instead of committed; (2) any task-level error (agent failure, commit failure, re-parse failure) aborts the entire run immediately. Together these cause "unclean endings" — selecting 5 tasks but only 2 getting committed. This plan fixes both by making the commit step smarter and the loop resilient to individual task failures.

## Goals

- Commit staged changes using the task title when `COMMIT.md` is absent
- Process all N selected tasks even when individual tasks fail
- Leave git in a clean state between tasks (no partial staged changes leaking into the next task)
- Show which tasks failed (and why) in the summary screen
- Never silently swallow errors — every failure is visible to the user

## User Stories

### TASK-001: Add fallback commit message parameter to CommitIteration
**Description:** As a developer, I want `CommitIteration` to accept a fallback commit message so that it can commit staged changes even when `COMMIT.md` is absent.

**Acceptance Criteria:**
- [x] `CommitIteration` signature changes to `CommitIteration(workDir, fallbackMsg string) (Result, error)`
- [x] When `COMMIT.md` is not found **and** `fallbackMsg` is non-empty: run the safety-gate unstaging, then run `git commit -m <fallbackMsg>`
- [x] If `git commit -m` returns "nothing to commit": return `Result{Committed: false, Message: "No changes to commit — continuing to next task"}`
- [x] If `git commit -m` succeeds: return `Result{Committed: true, Message: <git output>}`
- [x] When `COMMIT.md` is not found **and** `fallbackMsg` is empty: preserve the old warning (`"Warning: COMMIT.md not found, agent may not have made changes"`)
- [x] Existing `COMMIT.md` path is unchanged
- [x] All existing tests in `gitcommit_test.go` still pass
- [x] Unit tests for the new fallback path are written and pass

### TASK-002: Pass task title as fallback commit message in work loop
**Description:** As a Maggus user, I want the work loop to pass the current task's title as the fallback commit message so that missing `COMMIT.md` files don't silently drop staged changes.

**Acceptance Criteria:**
- [x] In `cmd/work.go`, the call `gitcommit.CommitIteration(workDir)` is updated to `gitcommit.CommitIteration(workDir, next.ID+": "+next.Title)`
- [x] When a fallback commit succeeds, a `runner.CommitMsg` is sent to the TUI (same path as a normal COMMIT.md commit)
- [x] When a fallback commit results in "nothing to commit", a `runner.InfoMsg` warning is sent
- [x] `go build ./...` and `go test ./...` pass in `src/`

### TASK-003: Make agent errors non-fatal in the work loop
**Description:** As a Maggus user, I want a failed agent run to be recorded and skipped rather than aborting the entire batch, so that the remaining tasks are still attempted.

**Acceptance Criteria:**
- [x] When `activeAgent.Run` returns a non-nil error and the context is not cancelled, the loop does NOT `return` — instead it records the failure and calls `continue`
- [x] The failed task is added to a local `failedTasks` slice (struct with `ID string`, `Title string`, `Reason string`)
- [x] A `runner.InfoMsg` is sent to the TUI: `"✗ TASK-NNN failed: <error> — skipping to next task"`
- [x] `workErr` is NOT set (the outer Cobra error path is not triggered)
- [x] If the context IS cancelled (Ctrl+C), the existing interrupt path is preserved unchanged
- [x] `go test ./...` passes in `src/`

### TASK-004: Clean git state between tasks
**Description:** As a Maggus user, I want any uncommitted staged changes cleaned up before the next task starts, so that a failed task's partial work does not contaminate the next task's commit.

**Acceptance Criteria:**
- [ ] A helper `resetStagedChanges(workDir string) error` is added (in `work.go` or `gitcommit` package) that runs `git reset HEAD`
- [ ] `resetStagedChanges` is called after the commit step whenever a task ends without a successful commit (failure path and "nothing to commit" path)
- [ ] Untracked files are NOT removed — only staged index entries are reset
- [ ] If `resetStagedChanges` fails, a warning `InfoMsg` is sent but the loop continues
- [ ] `go test ./...` passes in `src/`

### TASK-005: Make commit errors non-fatal
**Description:** As a Maggus user, I want a failed `git commit` to be recorded and skipped, so that a commit failure on task 2 doesn't prevent tasks 3–5 from running.

**Acceptance Criteria:**
- [x] When `gitcommit.CommitIteration` returns a non-nil `commitErr`, the loop records the failure and calls `continue` instead of `return`
- [x] The failed task is added to the `failedTasks` slice with the commit error as the reason
- [x] A `runner.InfoMsg` is sent: `"✗ TASK-NNN commit failed: <error> — skipping to next task"`
- [x] `resetStagedChanges` (from TASK-004) is called after a commit error to ensure a clean state for the next task
- [x] Re-parse errors (`parser.ParsePlans`) are treated the same way: add to `failedTasks`, call `resetStagedChanges`, `continue` with the last good `tasks` slice
- [x] `go test ./...` passes in `src/`

### TASK-006: Track and display failed tasks in the summary
**Description:** As a Maggus user, I want the summary screen to show which tasks failed and why, so I know exactly what to fix after a run completes.

**Acceptance Criteria:**
- [x] `SummaryData` gains two new fields: `FailedTasks []FailedTask` (where `FailedTask` has `ID`, `Title`, `Reason string`) and `TasksFailed int`
- [x] A new `StopReason` value `StopReasonPartialComplete` is added for runs where the loop finished all N iterations but some tasks failed
- [x] In `cmd/work.go`, `summaryData` is populated with `failedTasks` and `TasksFailed = len(failedTasks)` before `SummaryMsg` is sent; `stopReason` is set to `StopReasonPartialComplete` when `len(failedTasks) > 0` and the loop ran to completion
- [x] When `Reason == StopReasonComplete` and `TasksFailed == 0`: title renders as `"✓ Work Complete"` (unchanged)
- [x] When `Reason == StopReasonPartialComplete`: title renders as `"⚠ Work Complete (with failures)"` in warning color
- [x] The summary screen renders a `"Failed Tasks:"` section listing each failed task's ID, title, and reason when `len(FailedTasks) > 0`
- [x] `go test ./...` passes in `src/`

## Functional Requirements

- FR-1: `CommitIteration` must accept a second `fallbackMsg string` parameter; when `COMMIT.md` is absent and `fallbackMsg` is non-empty, use it as the commit message via `git commit -m`
- FR-2: The fallback commit must run the safety-gate (unstage internal files) before committing, same as the `COMMIT.md` path
- FR-3: The caller in `work.go` must format the fallback as `"<TASK-ID>: <TaskTitle>"` (e.g. `"TASK-003: Add login button"`)
- FR-4: The work loop MUST process all N tasks unless interrupted by the user (Ctrl+C / stop key) or context cancellation
- FR-5: After every task (success OR failure), staged index changes not included in a commit MUST be reset via `git reset HEAD` before the next task starts
- FR-6: Every task failure MUST produce a visible `InfoMsg` in the TUI during the run and appear in the post-run summary with its reason
- FR-7: `workErr` (the Cobra exit code) remains nil when all N tasks are attempted, even with partial failures — the command exits 0 and the summary shows the failures
- FR-8: `workErr` is still set (aborting the run) for infrastructure errors: config load failure, branch creation failure, run tracker creation failure

## Non-Goals

- No auto-staging of untracked files — only already-staged changes are committed via the fallback
- No retry logic — failed tasks are skipped, not retried in the same run
- No automatic re-queuing of failed tasks into the plan file
- No changes to the Ctrl+C / interrupt path — that still aborts immediately
- No changes to the stop-after-task (`s` key) path — that still stops cleanly after the current task
- No changes to worktree-mode task locking behavior

## Technical Considerations

- `resetStagedChanges(workDir string)` runs `git reset HEAD` — placing it as an unexported helper in `work.go` is sufficient since it is only called from the work loop
- The `failedTasks` slice is declared before the `for i := 0; i < count; i++` loop and closed over by the summary builder at the end of the goroutine
- Re-parse errors: if `parser.ParsePlans(workDir)` fails after a task, continue with the last good `tasks` slice and record the task as failed (the in-memory task list won't reflect the agent's checkbox changes, but the loop can still proceed)
- The `stopReason` logic: after the loop exits normally (not via interrupt/error return), if `len(failedTasks) > 0` set `stopReason = StopReasonPartialComplete`; if `len(failedTasks) == 0` keep `StopReasonComplete`
- TASK-001 and TASK-002 are independent of TASK-003–006 and can be implemented in any order; TASK-004 should be implemented before TASK-005 since TASK-005 calls `resetStagedChanges`

## Success Metrics

- When an agent stages changes but omits `COMMIT.md`, `maggus work` commits those changes with the task title and shows the commit in the TUI
- Selecting 5 tasks where task 2 fails results in 4 tasks attempted; the summary shows "1 failed, 4 completed" with the failure reason
- `git status` shows no staged files at the start of each task's agent run
- No regression in existing test coverage

## Open Questions

- None — scope is fully clear
