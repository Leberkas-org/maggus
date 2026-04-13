<!-- maggus-id: ed4a6e04-6092-4494-9a7b-ac575aad84cc -->
# Feature 052: Unified Work Orchestrator

## Introduction

The daemon work loop currently has two separate code paths (`runWorkGoroutine` for sequential, `runParallelWorkGoroutine` + `parallelOrchestrator` for parallel) plus a dispatch mode that spawns a separate OS process. All three ultimately call the same `RunTaskWorker`, but the orchestration logic above it is duplicated with subtle differences: the sequential path has multi-feature iteration, stop flags, approval re-checks, between-task sync, and pre-commit callbacks that the parallel path lacks. The parallel path has worktree management and concurrent dispatch that the sequential path lacks. Dispatch mode adds subprocess spawning and special CLI flags.

This feature replaces all three with a single orchestrator that owns task scheduling and a worker that owns task execution. The orchestrator decides what to run, when, and where (worktree or main repo). The worker gets a directory and does the work. No mode flags, no `UseWorktree` booleans, no `ExistingWorktreePath`.

### Architecture Context

- **Vision alignment:** Simplifies the core work loop, making the daemon more reliable and the codebase more maintainable
- **Components involved:** `cmd/run_loop.go` (sequential loop), `cmd/run_parallel.go` (parallel orchestrator), `cmd/dispatch.go` (subprocess dispatch), `cmd/worker.go` (unified worker), `cmd/daemon_keepalive.go` (cycle entry point), `cmd/run_task.go` (sequential task runner), `cmd/daemon_tui.go` (dispatch finalization), `cmd/clean.go` (dispatch cleanup), `cmd/status_update.go` (TUI dispatch trigger)
- **New patterns:** Single orchestrator with task classification; file-based dispatch signaling (consistent with existing stop-signal pattern)
- **Architecture update needed:** ARCHITECTURE.md must be updated to reflect orchestrator+worker separation

## Goals

- One orchestrator that handles feature iteration, task classification, concurrency, worktree lifecycle, approval/stop checks, and between-task sync
- One worker that receives a work directory and executes a task without knowing about execution strategy
- Remove dispatch mode (separate OS process, `--dispatch-repo`, `--dispatch-base-branch` flags)
- TUI "dispatch task" uses file-based signaling to the orchestrator (consistent with stop-signal pattern)
- No behavioral changes from the user's perspective

## Tasks

### TASK-052-001: Create orchestrator with sequential feature loop
**Description:** As a developer, I want an orchestrator struct that handles the feature iteration loop so that multi-feature progression, approval, and stop checks exist in one place.

**Token Estimate:** ~50k tokens
**Predecessors:** none
**Successors:** TASK-052-002
**Parallel:** yes -- can run alongside TASK-052-004

Create `cmd/orchestrator.go` with an `Orchestrator` struct that handles the sequential (non-parallel) work loop:

1. **Struct definition** with fields for: context, tea.Program, agent config, logger, feature/bug stores, stop flags, config directory.
2. **`Run()` method** -- outer loop over approved feature groups (from `runWorkGoroutine`'s multi-feature loop at `run_loop.go:308-352`). Respects `--count` / `auto_continue` for feature limiting.
3. **Inner task loop** per feature -- find next workable task, invoke `RunTaskWorker` sequentially (from `runGroupTasks` at `run_loop.go:411-475`).
4. **Between-feature checks** -- approval re-check before each feature (from `run_loop.go:324-330`).
5. **Between-task checks** -- stop flag, stop-at-task, context cancellation (from `run_loop.go:419-429`).
6. **Between-task sync** -- remote divergence detection (from `run_task.go:364-401` `betweenTaskSync`).
7. **Pre-commit callback** -- pass `buildPreCommitFn` to worker (from `run_task.go:255-287`).
8. **Results collection** -- completed count, failed tasks, warnings, stop reason.
9. **TUI messages** -- `IterationStartMsg`, `ProgressMsg`, `InfoMsg`, `SummaryMsg`, `SyncCheckMsg`.

Uses the current `RunTaskWorker` and `WorkerConfig` as-is (worker simplification happens in TASK-052-004). Coexists with old code -- not wired into the daemon yet.

**Acceptance Criteria:**
- [x] `Orchestrator` struct in `cmd/orchestrator.go`
- [x] `Run()` method handles multi-feature iteration with approval re-checks
- [x] Sequential task dispatch via `RunTaskWorker`
- [x] Stop flag, stop-at-task, and context cancellation between tasks
- [x] Between-task remote sync check
- [x] Pre-commit callback passed to worker
- [x] Results collection (completed, failed, warnings, stop reason)
- [x] TUI messages sent (IterationStartMsg, ProgressMsg, SummaryMsg)
- [x] `go build ./...` succeeds
- [x] `go test ./...` passes

### TASK-052-002: Add parallel task dispatch to orchestrator
**Description:** As a developer, I want the orchestrator to handle parallel tasks so that it replaces the parallel orchestrator's concurrency logic.

**Token Estimate:** ~50k tokens
**Predecessors:** TASK-052-001
**Successors:** TASK-052-003
**Parallel:** no

Extend the orchestrator from TASK-052-001 with parallel task support:

1. **Task classification** -- per feature iteration, re-parse tasks and split into parallel-eligible and sequential using `Parallel` metadata and predecessor completion (from `run_parallel.go:189-218` `classifyWorkable`).
2. **Worktree lifecycle** -- for parallel tasks: create task branch + git worktree before calling worker, remove worktree + delete branch after worker returns (from `run_parallel.go:260-295`). Worktree path: `.maggus/worktrees/{taskID}`.
3. **Concurrent dispatch** -- launch parallel workers via `errgroup`, each in its own worktree (from `run_parallel.go:221-254` `runParallelBatch`).
4. **Merge serialization** -- shared `sync.Mutex` passed to parallel workers' `WorkerConfig.MergeMu` to prevent concurrent merges.
5. **Mixed batches** -- when `classifyWorkable` returns parallel tasks, run them concurrently; when it returns only sequential tasks, run one at a time. Alternate between batches as predecessors complete (same pattern as current `parallelOrchestrator.run`).

The orchestrator's inner task loop (from TASK-052-001) now checks if tasks are parallel-eligible before dispatching.

**Source material:**
- Task classification: `run_parallel.go:189-218` (`classifyWorkable`, `predecessorsComplete`)
- Worktree creation: `worker.go:155-163` (move to orchestrator)
- Parallel batch: `run_parallel.go:221-254` (`runParallelBatch`)
- Orchestrator loop: `run_parallel.go:131-186` (`parallelOrchestrator.run`)

**Acceptance Criteria:**
- [ ] Task classification splits tasks into parallel and sequential based on metadata
- [ ] Predecessor tracking determines task eligibility
- [ ] Parallel tasks run concurrently in isolated git worktrees
- [ ] Worktrees created before worker call, cleaned up after
- [ ] Merge serialization via shared mutex
- [ ] Sequential tasks still run in main repo (no worktree)
- [ ] Mixed parallel/sequential batches handled correctly
- [ ] `go build ./...` succeeds
- [ ] `go test ./...` passes

### TASK-052-003: Add worker snapshot tracking to orchestrator
**Description:** As a developer, I want the orchestrator to write per-worker snapshots so that the status TUI can display parallel task progress.

**Token Estimate:** ~35k tokens
**Predecessors:** TASK-052-002
**Successors:** TASK-052-005
**Parallel:** no

Move per-worker snapshot tracking from `run_parallel.go` / `run_parallel_tui.go` into the orchestrator:

1. **Worker snapshot writing** -- for parallel tasks, create a `workerSnapshotWriter` per task that writes to `.maggus/runs/state-{taskID}.json` (from `run_parallel_tui.go`). Pass as `AgentSender` to the worker so agent output goes to the per-worker snapshot.
2. **Workers index** -- maintain a shared workers index at `.maggus/runs/state-workers.json` with status per task (working/done/failed/blocked). Write index updates as tasks start and complete (from `run_parallel.go:226-232`, `run_parallel.go:373-406`).
3. **Status tracking maps** -- track worker order, statuses, titles, start times for TUI display (from `parallelOrchestrator` fields at `run_parallel.go:45-48`).
4. **Cleanup** -- `cleanupWorkerSnapshots` after all workers complete (from `run_parallel_tui.go`).

**Acceptance Criteria:**
- [ ] Per-worker snapshots written for parallel tasks
- [ ] Workers index maintained with task statuses
- [ ] Status tracking consistent with current TUI expectations
- [ ] Snapshot cleanup after orchestrator completes
- [ ] Status TUI can display parallel task progress (same as current behavior)
- [ ] `go build ./...` succeeds
- [ ] `go test ./...` passes

### TASK-052-004: Simplify worker -- remove mode awareness
**Description:** As a developer, I want the worker to have no knowledge of execution modes so that it only cares about executing a single task in a given directory.

**Token Estimate:** ~40k tokens
**Predecessors:** none
**Successors:** TASK-052-005
**Parallel:** yes -- can run alongside TASK-052-001, TASK-052-002, TASK-052-003

Simplify `WorkerConfig` in `cmd/worker.go`:

1. **Remove `UseWorktree`** -- the worker never creates worktrees. The orchestrator creates them and passes the path.
2. **Remove `ExistingWorktreePath`** -- replaced by the orchestrator setting `WorkDir`.
3. **Add explicit `WorkDir`** -- the directory the worker runs the agent in and commits from. May be the main repo or a worktree. The worker doesn't care.
4. **Keep `RepoDir`** -- for merge-back operations (always the main repo).
5. **Keep `PlanBranch`** -- worker creates task branch from this, merges back after commit.
6. **Keep `MergeMu`** -- orchestrator provides this for parallel merge serialization.
7. **Keep `PreCommit`** -- worker calls this before committing.
8. **Simplify `RunTaskWorker`** -- remove the three-way branch at the top (lines 148-169). Worker always: creates task branch from `PlanBranch` (when set), runs agent in `WorkDir`, commits in `WorkDir`, merges task branch into `PlanBranch` in `RepoDir`, deletes task branch. No worktree creation/cleanup.
9. **Update all existing callers** -- both old paths (`runTask`, `parallelOrchestrator.runSingleTask`) and tests must use the new `WorkDir` field instead of the removed fields. This keeps both old and new code compiling until the old paths are removed in TASK-052-006.

The worker's lifecycle becomes:
1. Check approval (existing check)
2. Create task branch from `PlanBranch` in `RepoDir` (when `PlanBranch` is set)
3. Build prompt, run agent in `WorkDir`
4. Pre-commit callback
5. Commit in `WorkDir`
6. Merge task branch into `PlanBranch` in `RepoDir` (using `MergeMu`)
7. Delete task branch (best-effort)

**Acceptance Criteria:**
- [x] `UseWorktree` and `ExistingWorktreePath` removed from `WorkerConfig`
- [x] `WorkDir` field added -- worker runs agent and commits here
- [x] Worker never creates or removes git worktrees
- [x] Worker still creates task branch, commits, merges back, deletes branch
- [x] All existing callers updated to use `WorkDir`
- [x] `go build ./...` succeeds
- [x] `go test ./cmd/...` passes

### TASK-052-005: Wire orchestrator into daemon
**Description:** As a developer, I want the daemon to use the unified orchestrator so that the mode branching in `runOneDaemonCycle` is eliminated.

**Token Estimate:** ~50k tokens
**Predecessors:** TASK-052-003, TASK-052-004
**Successors:** TASK-052-006
**Parallel:** no

Replace the mode branching in `runOneDaemonCycle` with a single orchestrator call:

1. **Replace `daemon_keepalive.go:487-491`** -- remove `if wc.parallel { runParallelWorkGoroutine(...) } else { runWorkGoroutine(...) }` and replace with `orchestrator.Run(...)`.
2. **Adapt orchestrator to simplified WorkerConfig** -- the orchestrator (from TASK-052-001/002/003) was written against the old `WorkerConfig`. Update it to use `WorkDir` instead of `UseWorktree`/`ExistingWorktreePath` (from TASK-052-004). For sequential tasks: `WorkDir = RepoDir`. For parallel tasks: `WorkDir = worktreePath`.
3. **Simplify `runOneDaemonCycle`** -- remove mode-dependent branch setup (sequential vs parallel vs dispatch at lines 297-337). The orchestrator handles plan branch setup internally. Remove the `parallelFlag` hot-reload (orchestrator reads config directly).
4. **Remove dispatch-specific setup** -- remove `dispatchRepoFlag`/`dispatchBaseBranchFlag` handling from `runOneDaemonCycle` (lines 297-306), PID skip (lines 37-39), worker registration (lines 46-53).

After this task, the old loop functions are dead code (unreachable) but still exist in the source files. Deletion happens in TASK-052-006.

**Acceptance Criteria:**
- [ ] `runOneDaemonCycle` calls the orchestrator with no mode branching
- [ ] Orchestrator uses simplified `WorkerConfig` with `WorkDir`
- [ ] Mode-dependent branch setup removed from `runOneDaemonCycle`
- [ ] Dispatch-specific setup removed from `runOneDaemonCycle`
- [ ] Sequential execution works end-to-end
- [ ] Parallel execution works end-to-end
- [ ] `go build ./...` succeeds
- [ ] `go test ./...` passes

### TASK-052-006: Remove old code paths and dispatch mode
**Description:** As a developer, I want all dead code from the old three-path implementation removed so that the codebase only contains the orchestrator+worker pattern.

**Token Estimate:** ~50k tokens
**Predecessors:** TASK-052-005
**Successors:** TASK-052-007
**Parallel:** no

Delete all old code that is now unreachable after TASK-052-005:

1. **Sequential path:**
   - `run_loop.go`: delete `runWorkGoroutine`, `runGroupTasks`, `groupTasksResult`
   - `run_task.go`: delete `runTask`, `buildPreCommitFn`, `betweenTaskSync`, `sendIterationStart`, `findNextWorkableTask`, `syncBreak`, `completionSnapshot`, `snapshotForHooks`, `fireCompletionHooks` (move any of these to the orchestrator if still needed there)
   - Keep shared helpers that the orchestrator still uses: `countWorkable`, `firstWorkableTask`, `buildApprovedPlans`, `buildSummaryData`, `pushToRemote`, `captureStartHash`, `isTaskAtOrPastTarget`, `runLoopParams` (if still used)

2. **Parallel path:**
   - `run_parallel.go`: delete `parallelOrchestrator`, `runParallelWorkGoroutine`, `runParallelBatch`, `runSingleTask`, `classifyWorkable`, `predecessorsComplete`, all marker methods (`markWorkerDone/Failed/Blocked`)
   - Keep `checkCriterionInFile` if still used by worker

3. **Dispatch mode:**
   - `dispatch.go`: delete entirely
   - `run.go`: remove `dispatchRepoFlag`, `dispatchBaseBranchFlag` flag definitions and reset
   - `run_setup.go`: remove dispatch-specific `planDir` logic
   - `daemon_tui.go`: remove `dispatchRepoDir`, `dispatchTaskID` fields, `finalizeDispatchWorker`, dispatch-specific snapshot writing in `writeSnapshot`
   - `clean.go`: remove `findFinishedDispatchWorkers`, `cleanFinishedDispatchWorkers`

4. **Run `go vet ./...`** and check for unused exports, variables, types.

**Acceptance Criteria:**
- [ ] `runWorkGoroutine`, `runGroupTasks`, `runTask` deleted
- [ ] `parallelOrchestrator`, `runParallelWorkGoroutine` deleted
- [ ] `dispatch.go` deleted entirely
- [ ] `--dispatch-repo` and `--dispatch-base-branch` flags removed
- [ ] Dispatch-specific code removed from daemon_tui.go, clean.go, run_setup.go
- [ ] No unused functions, types, or variables remain
- [ ] `go vet ./...` reports no new warnings
- [ ] `go build ./...` succeeds
- [ ] `go test ./...` passes

### TASK-052-007: Implement file-based TUI dispatch
**Description:** As a user, I want to dispatch a specific task from the status TUI so that I can run a task immediately without waiting for the orchestrator to pick it up naturally.

**Token Estimate:** ~40k tokens
**Predecessors:** TASK-052-006
**Successors:** TASK-052-008
**Parallel:** no

Replace the old subprocess dispatch (Alt+R in status TUI) with file-based signaling:

1. **TUI side** (`status_update.go`):
   - When user presses Alt+R on a task: write an empty sentinel file `.maggus/dispatch-{taskID}` (existence is the signal, consistent with `.maggus/daemon.stop` pattern)
   - Show status note: "Dispatched {taskID}"
   - If daemon is not running: fall back to foreground execution via `tea.ExecProcess` (preserve current fallback behavior)

2. **Orchestrator side** (extend orchestrator from TASK-052-001):
   - At the start of each task iteration (inner loop), check for `.maggus/dispatch-*` files
   - When found: create a worktree, run the requested task immediately (ahead of normal queue), clean up worktree after
   - Remove the sentinel file after picking up the request
   - If the task is already complete or currently running, remove the sentinel and skip

3. **Worker index** -- dispatched tasks get per-worker snapshots (same as parallel tasks) so the TUI shows their status.

4. **Validation** -- handle edge cases: task not found, task blocked, task already running, multiple dispatches for same task, stale sentinel files from interrupted runs.

**Acceptance Criteria:**
- [ ] Alt+R in status TUI writes `.maggus/dispatch-{taskID}` sentinel file
- [ ] Orchestrator detects and processes dispatch requests
- [ ] Dispatched task runs in a worktree with full worker lifecycle
- [ ] TUI shows dispatched task status via per-worker snapshots
- [ ] Fallback to foreground execution when daemon is not running
- [ ] Sentinel file removed after task is picked up
- [ ] Duplicate/stale dispatch requests handled gracefully
- [ ] `go build ./...` succeeds
- [ ] `go test ./...` passes

### TASK-052-008: Update documentation
**Description:** As a developer, I want ARCHITECTURE.md and CLAUDE.md to reflect the new orchestrator+worker design so that the documentation matches the code.

**Token Estimate:** ~25k tokens
**Predecessors:** TASK-052-007
**Successors:** none
**Parallel:** no
**Model:** haiku

1. **ARCHITECTURE.md:**
   - Replace "Unified Task Worker" section with "Orchestrator + Worker" section
   - Remove the three-mode table (`UseWorktree` / `ExistingWorktreePath` / etc.)
   - Document orchestrator responsibilities: feature iteration, task classification, worktree lifecycle, concurrency, approval/stop checks, dispatch handling
   - Document worker responsibilities: receives WorkDir, creates task branch, runs agent, commits, merges back
   - Update the Work Loop diagram to show orchestrator -> worker flow
   - Remove dispatch mode references throughout
   - Update file-based state table (add dispatch sentinel files, remove dispatch log files)

2. **CLAUDE.md:**
   - Update architecture table: replace "Unified Task Worker" description with orchestrator+worker
   - Update "Run Loop Flow" section
   - Remove references to dispatch mode, parallel vs sequential mode flags

**Acceptance Criteria:**
- [ ] ARCHITECTURE.md reflects orchestrator+worker design
- [ ] Three-mode table and dispatch mode references removed
- [ ] Work Loop diagram updated
- [ ] CLAUDE.md architecture table updated
- [ ] No stale references to `UseWorktree`, `ExistingWorktreePath`, dispatch flags

## Task Dependency Graph

```
TASK-052-001 ──→ TASK-052-002 ──→ TASK-052-003 ──→ TASK-052-005 ──→ TASK-052-006 ──→ TASK-052-007 ──→ TASK-052-008
TASK-052-004 ──────────────────────────────────────┘
```

| Task | Estimate | Predecessors | Parallel | Model |
|------|----------|--------------|----------|-------|
| TASK-052-001 | ~50k | none | yes (with 004) | -- |
| TASK-052-002 | ~50k | 001 | no | -- |
| TASK-052-003 | ~35k | 002 | no | -- |
| TASK-052-004 | ~40k | none | yes (with 001, 002, 003) | -- |
| TASK-052-005 | ~50k | 003, 004 | no | -- |
| TASK-052-006 | ~50k | 005 | no | -- |
| TASK-052-007 | ~40k | 006 | no | -- |
| TASK-052-008 | ~25k | 007 | no | haiku |

**Total estimated tokens:** ~340k

## Functional Requirements

- FR-1: The orchestrator must iterate approved features in priority order (bugs first, then features)
- FR-2: The orchestrator must classify tasks as parallel or sequential based on task metadata and predecessor completion
- FR-3: Parallel tasks must run concurrently in isolated git worktrees with serialized merges
- FR-4: Sequential tasks must run in the main repo directory without worktrees
- FR-5: The orchestrator must re-check approval state between tasks so mid-run revocations take effect
- FR-6: The orchestrator must check stop flags and stop-at-task signals between tasks
- FR-7: The orchestrator must check for remote divergence between tasks and present sync options
- FR-8: The worker must execute a task in whatever directory it's given without knowing about worktrees or execution modes
- FR-9: TUI dispatch (Alt+R) must work via file-based signaling when daemon is running
- FR-10: TUI dispatch must fall back to foreground execution when daemon is not running
- FR-11: The orchestrator must handle dispatch requests ahead of the normal task queue

## Non-Goals

- No new execution strategies (e.g., distributed workers, remote execution)
- No changes to the agent interface, prompt building, or commit logic
- No changes to the TUI status view layout (only the dispatch mechanism changes)
- No changes to the config file format
- No changes to the plan file format or task metadata

## Technical Considerations

- **File size limit:** The orchestrator will likely need to be split across multiple files to stay under 500 lines. Suggested split: `orchestrator.go` (struct + feature loop), `orchestrator_parallel.go` (task classification, worktree creation, concurrent dispatch), `orchestrator_tui.go` (snapshot writing, worker index).
- **Worker snapshot reuse:** `run_parallel_tui.go` contains `workerSnapshotWriter` and related helpers. These can be moved to the orchestrator or kept as shared utilities.
- **Plan branch setup:** Currently done in `runOneDaemonCycle` with mode-dependent logic. The orchestrator should handle plan branch creation/detection as part of its setup phase.
- **Worker branch creation for worktree tasks:** The orchestrator creates the task branch and worktree. The worker detects it's already on the task branch (branch exists) and skips creation. After commit, the worker merges back into PlanBranch. The orchestrator then cleans up the worktree and branch.
- **Dispatch sentinel race condition:** Multiple rapid Alt+R presses for the same task should not create duplicate runs. The orchestrator should atomically consume (read + delete) the sentinel file.
- **TASK-052-004 parallel merge note:** TASK-052-004 changes `WorkerConfig` while TASK-052-001/002/003 build the orchestrator against the old interface. TASK-052-005 reconciles both by adapting the orchestrator to use the new `WorkDir` field. Both tracks must remain compilable independently.

## Success Metrics

- `cmd/run_loop.go`, `cmd/run_parallel.go`, `cmd/run_task.go`, and `cmd/dispatch.go` are deleted or significantly reduced
- Zero mode-related fields (`UseWorktree`, `ExistingWorktreePath`, `--dispatch-repo`, `--dispatch-base-branch`)
- All features previously split across modes (approval re-check, stop flags, between-task sync, pre-commit) work uniformly
- No behavioral change from the user's perspective

## Open Questions

None -- scope is fully defined.
