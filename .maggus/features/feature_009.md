# Feature 009: Remove Run File Tracking

## Introduction

The `.maggus/runs/` directory was introduced early on to log metadata and
iteration notes for each maggus work invocation. Over time this turned out to
be unused in practice. This feature removes all code that creates and manages
these files: the `runtracker` package, the iteration-log instruction in prompts,
the `RunDir` field in the TUI banner, and the run-directory cleanup logic in
`maggus clean`. The RunID concept (a timestamp string used by usage tracking and
task locking) is kept — it is just generated inline instead of going through
the runtracker package.

## Goals

- Delete `src/internal/runtracker/` entirely
- Remove the prompt instruction that told agents to write `iteration-NN.md` files
- Remove `RunDir`/`IterLog` fields from `prompt.Options` and from `runner.BannerInfo`
- Remove `.maggus/runs` from `.gitignore`
- Strip run-directory cleanup from `maggus clean` (command keeps cleaning
  completed feature/bug files)
- All existing tests pass; run-related tests are removed

## Tasks

### TASK-009-001: Remove RunDir/IterLog from prompt package

**Description:** As a developer, I want the prompt package to stop referencing
run directories and iteration log files so that agents are no longer told to
write files that nobody reads.

**Token Estimate:** ~20k tokens
**Predecessors:** none
**Successors:** TASK-009-002
**Parallel:** yes — can run alongside TASK-009-003

**Acceptance Criteria:**
- [x] `RunDir string` and `IterLog string` are removed from `prompt.Options` in
  `src/internal/prompt/prompt.go`
- [x] `writeRunMetadata()` no longer emits `RUN_DIR` or `ITER_LOG` lines
- [x] Step 4 ("Write an iteration log to `%s`…") is removed from
  `writeInstructions()` and the remaining steps are renumbered so that the
  sequence is continuous (no gaps)
- [x] `src/internal/prompt/prompt_test.go` is updated to remove the `RunDir`
  and `IterLog` fields from any `prompt.Options` literals; tests still pass
- [x] `go test ./internal/prompt` passes
- [x] `go vet ./...` passes

---

### TASK-009-002: Remove runtracker + wire RunID directly in work*.go + update TUI + gitignore

**Description:** As a developer, I want to delete the runtracker package and
replace its only real output (a timestamp-based RunID and a StartTime) with
inline logic so that no `.maggus/runs/` directory is ever created.

**Token Estimate:** ~80k tokens
**Predecessors:** TASK-009-001
**Successors:** none
**Parallel:** no

**Acceptance Criteria:**
- [x] `src/internal/runtracker/runtracker.go` is deleted
- [x] `src/internal/runtracker/runtracker_test.go` is deleted
- [x] `.maggus/runs` entry is removed from `.gitignore`
- [x] `runner.BannerInfo` in `src/internal/runner/tui.go` no longer has a
  `RunDir` field; any render line that displayed it in `tui_render.go` is
  removed
- [x] `iterationSetup` in `work_loop.go` replaces `run *runtracker.Run` with
  `runID string` and `startTime time.Time`
- [x] `initIteration()` in `work_loop.go` generates the RunID as
  `time.Now().Format("20060102-150405")` and captures `time.Now()` as
  `startTime`; the `runtracker.New()` call is gone
- [x] `workLoopParams` in `work_loop.go` replaces `run *runtracker.Run` with
  `runID string` and `startTime time.Time`
- [x] `runWorkGoroutine()` no longer calls `run.Finalize()` in its defer
- [x] `buildSummaryData()` uses `params.runID` and `params.startTime` directly
  instead of `params.run.*`
- [x] `setupUsageCallback()` in `work_loop.go` takes a plain `runID string`
  instead of `*runtracker.Run`
- [x] `setupBranch()` in `work_loop.go` takes a plain `runID string` instead of
  `*runtracker.Run`; the worktree path is constructed with that string
- [x] `taskContext` in `work_task.go` no longer has a `run *runtracker.Run`
  field; `runTask()` no longer passes `RunDir` or `IterLog` to `prompt.Options`
- [x] `work.go` is updated: the current git branch is obtained with a small
  inline git command (matching the existing pattern in `buildSummaryData`);
  `banner.RunDir` is removed; all call sites of updated helpers are adjusted
- [x] No import of `runtracker` package remains anywhere in `src/`
- [x] `go build ./...` passes
- [x] `go test ./...` passes

---

### TASK-009-003: Remove run-directory logic from clean.go and its tests

**Description:** As a user, I want `maggus clean` to only deal with completed
feature/bug files so that the command description and output no longer mention
run directories that no longer exist.

**Token Estimate:** ~25k tokens
**Predecessors:** none
**Successors:** none
**Parallel:** yes — can run alongside TASK-009-001 and TASK-009-002

**Acceptance Criteria:**
- [ ] `findCompletedRuns()` and `hasEndSection()` are deleted from `clean.go`
- [ ] `runClean()` no longer calls `findCompletedRuns()` or iterates over run
  directories
- [ ] The `cleanCmd` description (both `Short` and `Long`) no longer mentions
  run directories
- [ ] Output messages in `runClean()` only report completed feature/bug file
  counts; no "run directory(ies)" text remains
- [ ] `clean_test.go` removes `writeRunDir()` helper and all test functions that
  reference run directories (`TestCleanRemovesCompletedRuns`,
  `TestCleanKeepsInProgressRuns`, `TestCleanCombinedFeaturesAndRuns`,
  `TestCleanDryRun` if it references runs)
- [ ] `setupCleanDir()` no longer creates the `runs/` subdirectory
- [ ] Remaining clean tests (`TestCleanRemovesCompletedFeatures`,
  `TestCleanNothingToClean`, `TestCleanEmptyMaggusDir`,
  `TestCleanDryRunShowsPaths`) pass unchanged
- [ ] `go test ./cmd` passes
- [ ] `go vet ./...` passes

---

## Task Dependency Graph

```
TASK-009-001 ──→ TASK-009-002
TASK-009-003  (independent)
```

| Task | Estimate | Predecessors | Parallel | Model |
|------|----------|--------------|----------|-------|
| TASK-009-001 | ~20k | none | yes (with 003) | haiku |
| TASK-009-002 | ~80k | 001 | no | — |
| TASK-009-003 | ~25k | none | yes (with 001) | haiku |

**Total estimated tokens:** ~125k

## Functional Requirements

- FR-1: No file under `.maggus/runs/` is created during a `maggus work` run
- FR-2: The prompt sent to agents contains no `RUN_DIR`, `ITER_LOG`, or
  iteration-log writing instruction
- FR-3: The RunID (timestamp string) is still generated and passed to usage
  records, the task lock, and the TUI summary
- FR-4: `maggus clean` removes completed feature/bug files only; its output
  messages mention only feature/bug counts
- FR-5: `go build ./...` and `go test ./...` both pass after all tasks complete
- FR-6: `.gitignore` no longer lists `.maggus/runs`

## Non-Goals

- Do not delete any existing `.maggus/runs/` directories on disk — this is a
  code removal, not a data migration
- Do not change the RunID format
- Do not remove RunID from usage records, task locks, or the TUI summary screen
- Do not touch anything else in the prompt or runner beyond the specific fields
  listed above
