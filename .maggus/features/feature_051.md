<!-- maggus-id: 7a5c4fbd-2df4-4cce-b39b-b9c0b063aeb0 -->
# Feature 051: Unify Task Worker Into Single Implementation

## Introduction

The codebase has three separate code paths that do the same job — execute a single task (branch, prompt, agent, commit, merge-back, cleanup). They differ only in how they're invoked (sequential loop, parallel goroutines, dispatched processes), but each re-implements the full task lifecycle with subtle behavioral differences. This duplication caused bug_041 (sequential path forgot merge-back) and will cause similar bugs in the future.

### Architecture Context

- **Vision alignment:** A single worker implementation makes the daemon more reliable and the codebase more maintainable
- **Components involved:** `cmd/run_task.go` (sequential worker), `cmd/run_loop.go` (sequential loop), `cmd/run_parallel.go` (parallel orchestrator + worker), `cmd/daemon_tui.go` (dispatched worker finalization), `cmd/daemon_keepalive.go` (cycle entry point)
- **New patterns:** Extract a `Worker` that owns the full task lifecycle; callers only decide invocation strategy
- **Architecture update needed:** ARCHITECTURE.md should document the unified worker pattern

## Goals

- One implementation of the task lifecycle: create branch → build prompt → run agent → commit → merge back → delete branch
- Sequential mode = invoke the worker once, wait, repeat
- Parallel mode = invoke N workers concurrently
- Dispatched mode = invoke the worker in a separate process
- Bug_041 (missing merge-back in sequential mode) is resolved as a side effect of unification
- No behavioral changes from the user's perspective — same branching, same output, same TUI

## Tasks

### TASK-051-001: Extract unified task worker
**Description:** As a developer, I want a single `Worker` that encapsulates the full task lifecycle so that sequential, parallel, and dispatched modes share the same behavior.

**Token Estimate:** ~120k tokens
**Predecessors:** none
**Successors:** TASK-051-002, TASK-051-003
**Parallel:** no
**Model:** opus

Extract a `Worker` (function or struct) that owns the complete lifecycle for a single task:

1. **Create task branch** from the plan branch (via `gitbranch.EnsureFeatureBranch` or `CreateBranchFrom`)
2. **Build prompt** with bootstrap context + task details
3. **Run agent** subprocess with streaming output
4. **Commit** changes (read COMMIT.md, stage, commit)
5. **Merge task branch** back into plan branch (via `gitmerge.MergeTaskBranch`)
6. **Delete task branch** (via `gitbranch.DeleteBranch`, best-effort)
7. **Handle worktree cleanup** if applicable (via `gitworktree.RemoveWorktree`)

The worker takes a config struct with:
- Task + plan metadata (task ID, plan file, maggus_id, etc.)
- Agent + model config
- Repo dir, work dir (may differ for worktrees)
- Plan branch name (to merge back into)
- Logger, TUI program, snapshot writer
- Callbacks: onToolUse, onOutput, onTaskUsage

The worker returns a result struct with: success/failure, commit hash, error, stop reason.

**Source material:**
- Sequential logic: `run_task.go:83-220` (`runTask`) + `run_loop.go:380-470` (feature loop with branch/merge gaps)
- Parallel logic: `run_parallel.go:265-414` (`runSingleTask`) — this is the most complete implementation
- Dispatched logic: `daemon_tui.go:218-265` (`finalizeDispatchWorker`) — merge-back on exit

The parallel `runSingleTask` is the best starting point since it already does the full lifecycle (branch, work, commit, merge, cleanup). Extract and generalize it.

**Acceptance Criteria:**
- [x] A single `Worker` function/struct handles the full task lifecycle
- [x] Branch creation, merge-back, and cleanup are part of the worker — not the caller's responsibility
- [x] Worker handles merge conflicts by marking the task as blocked (same as current parallel behavior)
- [x] Worker handles worktree mode (work dir != repo dir) when applicable
- [x] Worker is usable from sequential, parallel, and dispatch call sites
- [x] `go build ./...` succeeds
- [x] `go test ./...` passes

### TASK-051-002: Rewire sequential daemon to use unified worker
**Description:** As a developer, I want the sequential daemon loop to use the unified worker so that it gets merge-back and branch cleanup for free.

**Token Estimate:** ~60k tokens
**Predecessors:** TASK-051-001
**Successors:** TASK-051-004
**Parallel:** yes — can run alongside TASK-051-003

Replace the sequential work loop in `run_loop.go` / `run_task.go` with calls to the unified worker:

1. `run_loop.go` iterates over tasks and invokes the worker for each one sequentially
2. Branch setup moves from `daemon_keepalive.go:311` (once per cycle) into the worker (per task)
3. `run_task.go` `runTask()` is replaced by a call to the worker
4. The plan branch is passed to the worker so it knows where to merge back

This resolves bug_041 as a side effect — the worker always merges back and cleans up.

**Acceptance Criteria:**
- [x] Sequential daemon loop calls the unified worker per task
- [x] Task branches are created, merged, and deleted per task (not once per cycle)
- [x] `setupBranch()` in `daemon_keepalive.go` is no longer called for sequential mode — the worker handles it
- [x] Bug_041 behavior verified: after each task, the task branch is merged and deleted
- [x] Daemon returns to plan branch between tasks
- [x] No behavioral change from user's perspective
- [x] `go build ./...` succeeds
- [x] `go test ./cmd/...` passes

### TASK-051-003: Rewire parallel orchestrator to use unified worker
**Description:** As a developer, I want the parallel orchestrator to use the unified worker so that task execution logic is not duplicated.

**Token Estimate:** ~50k tokens
**Predecessors:** TASK-051-001
**Successors:** TASK-051-004
**Parallel:** yes — can run alongside TASK-051-002

Replace `run_parallel.go` `runSingleTask` with calls to the unified worker:

1. The parallel orchestrator still manages concurrency, worktree creation, and the workers index
2. Each goroutine invokes the unified worker with the appropriate config (worktree path, plan branch, etc.)
3. `runSingleTask` is deleted or reduced to a thin wrapper around the worker
4. Merge serialization (the `o.mu.Lock()` around merge) stays in the orchestrator — the worker's merge step uses a callback or mutex provided by the caller

**Acceptance Criteria:**
- [x] `runSingleTask` replaced with calls to the unified worker
- [x] Parallel concurrency and worktree management remain in the orchestrator
- [x] Merge serialization preserved (no concurrent merges to the plan branch)
- [x] Worker snapshot writing works correctly per worker
- [x] No behavioral change from user's perspective
- [x] `go build ./...` succeeds
- [x] `go test ./cmd/...` passes

### TASK-051-004: Rewire dispatched worker and remove dead code
**Description:** As a developer, I want dispatched workers to use the unified worker and all dead code from the old three-path implementation removed.

**Token Estimate:** ~40k tokens
**Predecessors:** TASK-051-002, TASK-051-003
**Successors:** none
**Parallel:** no

1. **Dispatched worker**: `daemon_tui.go` `finalizeDispatchWorker` currently does merge-back on process exit. With the unified worker, merge-back happens inside the worker. Remove or simplify `finalizeDispatchWorker`.

2. **Remove dead code:**
   - Old `runTask()` in `run_task.go` if fully replaced
   - Old `runSingleTask()` in `run_parallel.go` if fully replaced
   - `setupBranch()` in `run_setup.go` if no longer called
   - `mergeDispatchedTaskBackImpl` and related functions in `daemon_tui.go`
   - Any helper functions, types, or variables that became unreachable
   - Run `go vet ./...` and check for unused exports

3. **Update ARCHITECTURE.md**: Document the unified worker pattern under the Work Loop section.

4. **Update CLAUDE.md**: Update the architecture table to reflect the unified worker.

**Acceptance Criteria:**
- [ ] Dispatched workers use the unified worker for merge-back
- [ ] `finalizeDispatchWorker` simplified or removed
- [ ] No dead functions, types, or variables from the old three-path implementation remain
- [ ] `go vet ./...` reports no new warnings
- [ ] ARCHITECTURE.md updated with unified worker documentation
- [ ] CLAUDE.md architecture table updated
- [ ] `go build ./...` succeeds
- [ ] `go test ./...` passes

## Task Dependency Graph

```
TASK-051-001 ──→ TASK-051-002 ──→ TASK-051-004
             └─→ TASK-051-003 ──┘
```

| Task | Estimate | Predecessors | Parallel | Model |
|------|----------|--------------|----------|-------|
| TASK-051-001 | ~120k | none | no | opus |
| TASK-051-002 | ~60k | 001 | yes (with 003) | — |
| TASK-051-003 | ~50k | 001 | yes (with 002) | — |
| TASK-051-004 | ~40k | 002, 003 | no | — |

**Total estimated tokens:** ~270k

## Functional Requirements

- FR-1: A single worker implementation must handle the full task lifecycle (branch → prompt → agent → commit → merge → cleanup)
- FR-2: Sequential mode must invoke the worker once per task, waiting for completion before the next
- FR-3: Parallel mode must invoke N workers concurrently, with merge serialization
- FR-4: Dispatched mode must invoke the worker in a separate process with merge-back to the main repo
- FR-5: The worker must handle merge conflicts by marking the task as blocked
- FR-6: The worker must handle worktree mode (work dir != repo dir) transparently
- FR-7: Branch cleanup (delete task branch) must be best-effort — failures logged, not fatal

## Non-Goals

- No new invocation modes (e.g. distributed workers, remote execution)
- No changes to prompt building, agent invocation, or commit logic — only the lifecycle orchestration is unified
- No changes to the TUI or status display
- No changes to the parallel orchestrator's concurrency model (goroutines, worktree allocation)

## Technical Considerations

- **Merge serialization in parallel mode**: The unified worker cannot blindly merge — in parallel mode, merges must be serialized. The worker should accept an optional mutex or use a callback for the merge step so the parallel orchestrator can control serialization.
- **Worktree handling**: The worker needs to know repo dir (for merge) vs. work dir (for agent execution). In worktree mode these differ; in sequential mode they're the same.
- **`run_parallel.go` `runSingleTask`** is the most complete reference implementation — it already does branch, work, commit, merge, cleanup. Start there.
- **File size**: The unified worker should stay under 500 lines (per CLAUDE.md code organization rules). Split into worker config/types, worker execution, and worker branch management if needed.

## Success Metrics

- `run_task.go`, `run_loop.go`, and `run_parallel.go` combined LOC decreases significantly
- Bug_041 (missing sequential merge-back) is resolved without a targeted fix
- Future task lifecycle changes only need to be made in one place

## Open Questions

None — scope is fully defined.
