<!-- maggus-id: c8cc7491-c4a3-4565-9827-fd54e98c40bb -->
# Feature 038: Multi-Daemon Parallel Task Execution

## Introduction

Enable Maggus to execute multiple tasks from the same plan in parallel, using git worktrees for filesystem isolation. Each parallel-eligible task runs in its own worktree on its own `feature/maggustask-NNN-XXX` branch, merges back to a shared `feature/maggus-NNN` feature branch on completion, and is displayed in a split-pane TUI. Wall-clock time for plans with independent tasks reduces from O(n) sequential to O(depth of dependency graph).

### Architecture Context

- **Vision alignment:** Directly advances the core "spec-driven, autonomous execution" goal — the work loop becomes as parallel as the dependency graph allows
- **Components involved:** `cmd/run.go` (work loop), `internal/gitbranch` (branch strategy), `internal/runtracker` (per-worker state), `internal/config` (new field), TUI status view
- **New components:** `internal/gitworktree` (worktree CRUD), `internal/gitmerge` (merge orchestration)
- **New pattern:** Concurrent workers via `golang.org/x/sync/errgroup`; per-worker run state in runtracker; two-tier branch strategy (feature branch + task branch)

## Goals

- Run all `Parallel: yes` tasks with no unmet predecessors concurrently, each in its own git worktree at `.maggus/worktrees/<task-id>/`
- Each task commits to its own `feature/maggustask-NNN-XXX` branch, which merges into `feature/maggus-NNN` on completion; merged branches are deleted
- Merge conflicts inject a `BLOCKED:` criterion on the task and preserve the worktree for human inspection
- Parallel mode is opt-in: `parallel: true` in `.maggus/config.yml` or `--parallel` CLI flag; CLI flag takes precedence
- TUI shows one live status pane per active worker in parallel mode, unchanged for sequential mode
- `Parallel: no` tasks still execute sequentially in the main worktree — existing behavior is fully preserved when parallel mode is off

## Tasks

### TASK-038-001: Add `parallel` config field and `--parallel` CLI flags
**Description:** As a developer, I want to enable parallel mode via config or CLI flag so that I can opt-in to parallel execution without changing existing behavior.

**Token Estimate:** ~20k tokens
**Predecessors:** none
**Successors:** TASK-038-005
**Parallel:** yes — can run alongside TASK-038-002

**Acceptance Criteria:**
- [x] `Parallel bool` field added to `Config` struct in `internal/config` with default `false`
- [x] `parallel: true/false` is parsed from `.maggus/config.yml` correctly
- [x] `--parallel` flag added to `maggus work` cobra command
- [x] `--parallel` flag added to `maggus start` cobra command
- [x] CLI `--parallel` flag overrides the config value when both are set
- [x] `maggus work --help` and `maggus start --help` show the new flag with a description
- [x] Unit tests cover config parsing for `parallel: true`, `parallel: false`, and missing field
- [x] Typecheck/lint passes

### TASK-038-002: Implement `internal/gitworktree` package
**Description:** As the work loop, I want a package that manages git worktrees so that each parallel task can operate in full filesystem isolation.

**Token Estimate:** ~35k tokens
**Predecessors:** none
**Successors:** TASK-038-003
**Parallel:** yes — can run alongside TASK-038-001

**Acceptance Criteria:**
- [ ] `CreateWorktree(repoRoot, path, branch string) error` creates a worktree at the given path; creates the branch off HEAD if it does not already exist
- [ ] `RemoveWorktree(repoRoot, path string) error` removes the worktree via `git worktree remove --force`
- [ ] `ListWorktrees(repoRoot string) ([]WorktreeInfo, error)` parses `git worktree list --porcelain` output
- [ ] `WorktreeInfo` struct carries `Path`, `Branch`, and `HEAD` fields
- [ ] Stale entries are pruned via `git worktree prune` before listing
- [ ] `internal/gitignore` is updated to ensure `.maggus/worktrees/` is present in `.gitignore`
- [ ] Unit tests cover create, remove, and list operations
- [ ] Typecheck/lint passes

### TASK-038-003: Update branch strategy for parallel mode
**Description:** As the work loop, I want a feature branch per plan and a task branch per parallel task so that changes are isolated and can be merged cleanly.

**Token Estimate:** ~40k tokens
**Predecessors:** TASK-038-002
**Successors:** TASK-038-004
**Parallel:** no

**Acceptance Criteria:**
- [ ] When parallel mode is active: a `feature/maggus-NNN` branch is created off the current branch before any tasks start (if not already on one)
- [ ] Each parallel task gets a `feature/maggustask-NNN-XXX` branch created off `feature/maggus-NNN` inside its own worktree
- [ ] `Parallel: no` tasks in a parallel-mode run use the main worktree and a `feature/maggustask-NNN-XXX` branch off `feature/maggus-NNN`
- [ ] Non-parallel mode (default): existing `feature/maggustask-NNN` single-branch behavior is entirely unchanged
- [ ] `internal/gitbranch` is updated or extended to support the two-tier strategy without breaking the single-tier path
- [ ] Unit tests cover parallel and non-parallel branch creation
- [ ] Typecheck/lint passes

### TASK-038-004: Implement merge orchestration (`internal/gitmerge`)
**Description:** As the work loop, I want to merge completed task branches into the feature branch so that parallel work integrates cleanly after each task finishes.

**Token Estimate:** ~50k tokens
**Predecessors:** TASK-038-003
**Successors:** TASK-038-005
**Parallel:** no
**Model:** opus

**Acceptance Criteria:**
- [ ] `MergeTaskBranch(repoRoot, featureBranch, taskBranch string) error` merges `taskBranch` into `featureBranch` using a standard merge commit (no rebase, no fast-forward squash)
- [ ] Fast-forward and clean three-way merges succeed and return nil
- [ ] On conflict: the merge is aborted (`git merge --abort`), the function injects `BLOCKED: Merge conflict merging <taskBranch> into <featureBranch> — resolve manually, then uncheck this criterion` as an unchecked criterion into the task's plan file, and returns a typed `MergeConflictError`
- [ ] On conflict: the task's worktree is preserved (not removed) so the developer can inspect the changes
- [ ] On success: `taskBranch` is deleted (`git branch -d`) and the worktree is removed via `internal/gitworktree.RemoveWorktree`
- [ ] Unit tests cover fast-forward merge, clean three-way merge, and conflict paths
- [ ] Typecheck/lint passes

### TASK-038-005: Implement parallel work loop
**Description:** As a developer running Maggus in parallel mode, I want the work loop to execute all currently-workable parallel tasks concurrently so that my plans finish faster.

**Token Estimate:** ~80k tokens
**Predecessors:** TASK-038-001, TASK-038-004
**Successors:** TASK-038-006
**Parallel:** no
**Model:** opus

**Acceptance Criteria:**
- [ ] When `parallel` is enabled, the work loop finds all workable tasks (`Parallel: yes` + incomplete + not blocked + approved + all predecessors completed) and launches them concurrently using `errgroup`
- [ ] Each concurrent task runs in its own worktree (via `internal/gitworktree`) on its own branch (via `internal/gitbranch`)
- [ ] When a task completes: merge to feature branch (via `internal/gitmerge`), check off acceptance criteria in the plan file, then re-evaluate the dependency graph for newly-workable tasks
- [ ] Plan file mutations (checkbox updates, BLOCKED injection) are serialized — no concurrent writes to the same file
- [ ] `Parallel: no` tasks execute sequentially in the main worktree after all their predecessors complete
- [ ] `runtracker` supports per-worker iteration logs, namespaced by task ID (e.g. `<iter>-<task-id>.log`)
- [ ] `Ctrl+C` cancels all active workers gracefully: 5-second wait, then force kill
- [ ] `cmd/run.go` stays under 500 lines — parallel orchestration logic is extracted into `cmd/run_parallel.go`
- [ ] Non-parallel mode path is entirely unchanged
- [ ] Typecheck/lint passes

### TASK-038-006: TUI split view for parallel workers
**Description:** As a developer monitoring a parallel run, I want to see a live status pane per active worker so that I can track all tasks at a glance without switching views.

**Token Estimate:** ~100k tokens
**Predecessors:** TASK-038-005
**Successors:** none
**Parallel:** no
**Model:** opus

**Acceptance Criteria:**
- [ ] In parallel mode, the status view splits horizontally into one pane per active worker (up to terminal width)
- [ ] Each pane shows: task ID, task title, current tool being invoked, token usage, elapsed time — mirroring the existing single-pane layout
- [ ] Panes are added dynamically as new tasks start and frozen with a ✓ or ✗ indicator when tasks complete or are blocked
- [ ] When worker count exceeds what fits horizontally, panes stack vertically (no silent clipping)
- [ ] Terminal resize redistributes pane widths proportionally
- [ ] Non-parallel mode (single worker) shows the existing single-pane status view with zero changes
- [ ] Typecheck/lint passes

## Task Dependency Graph

```
TASK-038-001 ──────────────────────────────────────────────────────────────→ TASK-038-005 → TASK-038-006
TASK-038-002 → TASK-038-003 → TASK-038-004 ──────────────────────────────→ ┘
```

| Task | Estimate | Predecessors | Parallel | Model |
|------|----------|--------------|----------|-------|
| TASK-038-001 | ~20k | none | yes (with 002) | — |
| TASK-038-002 | ~35k | none | yes (with 001) | — |
| TASK-038-003 | ~40k | 002 | no | — |
| TASK-038-004 | ~50k | 003 | no | opus |
| TASK-038-005 | ~80k | 001, 004 | no | opus |
| TASK-038-006 | ~100k | 005 | no | opus |

**Total estimated tokens:** ~325k

## Functional Requirements

- FR-1: When `parallel` is false (default) or not set, all existing sequential behavior is entirely unchanged
- FR-2: Parallel mode is activated by `parallel: true` in `.maggus/config.yml` or `--parallel` CLI flag; the CLI flag takes precedence over config
- FR-3: In parallel mode, all workable `Parallel: yes` tasks with no unmet predecessors are launched concurrently at the start of each work loop iteration
- FR-4: Each concurrent task runs in its own git worktree at `.maggus/worktrees/<task-id>/`
- FR-5: Each concurrent task operates on a `feature/maggustask-NNN-XXX` branch created off `feature/maggus-NNN`
- FR-6: On task completion, the task branch is merged into `feature/maggus-NNN` using a standard merge commit; the task branch is then deleted and the worktree removed
- FR-7: A merge conflict injects `BLOCKED: Merge conflict merging <taskBranch> into <featureBranch> — resolve manually, then uncheck this criterion` into the plan file and preserves the worktree
- FR-8: `Parallel: no` tasks in parallel mode run sequentially in the main worktree after their predecessors complete
- FR-9: The TUI status view shows one live pane per active parallel worker; non-parallel mode is unchanged
- FR-10: `.maggus/worktrees/` is present in `.gitignore`
- FR-11: Plan file mutations are serialized — no concurrent writes to the same plan file

## Non-Goals

- Running tasks from different plans in parallel (only tasks within the same plan are parallelized in this feature)
- Automatic conflict resolution (humans always resolve merge conflicts)
- Rebase-based task integration (merge commits only)
- Distributed or remote worker execution
- Any change to sequential work loop behavior when parallel mode is off

## Technical Considerations

- **Goroutine orchestration:** Use `golang.org/x/sync/errgroup` with a context for clean cancellation; each worker is a goroutine in the group
- **Dependency graph tracking:** A mutex-protected set of completed task IDs; after each completion, re-scan the plan to find newly-workable tasks and launch them
- **Plan file serialization:** A single `sync.Mutex` (or dedicated goroutine acting as a file writer) must gate all plan file mutations across workers
- **runtracker extension:** The run ID stays shared; per-worker iteration logs use the format `<iter>-<task-id>.log` to avoid collisions
- **File size limit:** Extract parallel orchestration into `cmd/run_parallel.go` to keep `cmd/run.go` under 500 lines
- **ARCHITECTURE.md update needed:** Document `internal/gitworktree`, `internal/gitmerge`, and the two-tier branch strategy after this feature ships

## Success Metrics

- A plan with 3 independent parallel tasks completes in roughly 1/3 the wall-clock time of sequential execution
- Merge conflicts are detected and blocked correctly — no silent data loss or corrupted plan files
- Sequential mode shows zero regression in behavior or performance
- The split TUI panes are readable on an 80-column terminal with up to 3 active workers

## Open Questions

None.
