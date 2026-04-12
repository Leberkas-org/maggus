<!-- maggus-id: e18c84e6-9f98-4d40-9197-eb67ba602276 -->
# Feature 048: Daemon Auto-Recovery from Interrupted Runs

## Introduction

When a daemon run is interrupted (crash, kill, power loss), it can leave the repository in a dirty state that prevents subsequent runs from making progress: uncommitted changes with a COMMIT.md present, the repo stuck on a task branch that was never merged back, and orphaned git worktrees from interrupted parallel runs locking branches. The daemon silently fails in this state — it starts the agent, the agent explores briefly, then stops without completing. No error is surfaced and the user must manually diagnose and clean up.

This feature adds a `RecoverDirtyState` pre-flight function that runs at the top of every daemon cycle. It detects signs of an interrupted run and fixes them automatically before the normal work loop begins.

### Architecture Context

- **Vision alignment:** Maggus is designed to be "fire-and-forget" — the daemon should self-heal without human intervention. This feature closes a gap where interrupted runs require manual recovery.
- **Components involved:** Daemon keep-alive loop (`cmd/daemon_keepalive.go`), git layer (`internal/git*`), commit handler (`internal/gitcommit`), worktree manager (`internal/gitworktree`), parser (`internal/parser`), stores (`internal/stores`)
- **New package:** `internal/gitrecover` — encapsulates all recovery logic, called from the daemon cycle before `initIteration`
- **Existing code reused:** `gitcommit.CommitIteration`, `gitworktree.ListWorktrees`/`RemoveWorktree`, `gitbranch.IsProtected`/`DeleteBranch`, `gitmerge.taskIDFromBranch` (needs exporting)

---

## Goals

- Automatically recover from uncommitted agent work (COMMIT.md left behind) without user intervention
- Consolidate orphaned task branches back into the parent integration branch so the daemon resumes from a unified state
- Clean up orphaned worktrees from interrupted parallel runs
- Make recovery idempotent — each step is safe to retry if recovery itself gets interrupted
- Keep the common case (clean repo) fast — a few sub-second git commands

---

## Tasks

### TASK-048-001: Export taskIDFromBranch from gitmerge and add branch classification helpers
**Description:** The recovery logic needs to determine whether a branch is a task branch and extract task IDs from branch names. `taskIDFromBranch` in `internal/gitmerge` is currently unexported. Export it and add a helper function `IsTaskBranch(branch string) bool` to `internal/gitbranch` that matches the `feature/maggus-NNN/task-NNN` and `bugfix/maggus-bug-NNN/task-NNN` patterns. Also add `TaskPrefixFromBranch(branch string) string` that extracts the feature prefix (e.g. `feature/maggus-004` from `feature/maggus-004/task-007`) for finding sibling task branches.

**Token Estimate:** ~15k tokens
**Predecessors:** none
**Successors:** TASK-048-002, TASK-048-003, TASK-048-004
**Parallel:** yes — can run alongside nothing initially, but is a prerequisite for all other tasks

**Acceptance Criteria:**
- [x] `gitmerge.TaskIDFromBranch` is exported (capital T) and existing callers updated
- [x] `gitbranch.IsTaskBranch(branch string) bool` correctly identifies both feature task branches (`feature/maggus-NNN/task-NNN`) and bug task branches (`bugfix/maggus-bug-NNN/task-NNN`)
- [x] `gitbranch.TaskPrefixFromBranch(branch string) string` returns the prefix portion (e.g. `feature/maggus-004` from `feature/maggus-004/task-007`), empty string for non-task branches
- [x] `gitbranch.IsTaskBranch` returns false for plan branches (`feature/feat-NNN-plan`), protected branches, and arbitrary branch names
- [x] Unit tests cover: feature task branch, bug task branch, plan branch, protected branch, arbitrary branch, empty string

---

### TASK-048-002: Implement Step 1 — commit pending changes
**Description:** Create the `internal/gitrecover` package with a function that detects and commits pending agent work. If COMMIT.md exists, use the existing `gitcommit.CommitIteration`. If COMMIT.md is absent but the working tree is dirty and the repo is on a task branch whose task ID differs from the next workable task, commit with a recovery message. If the task IDs match, skip (it's a resume). Apply the safety-gate unstage for internal files before recovery commits.

**Token Estimate:** ~30k tokens
**Predecessors:** TASK-048-001
**Successors:** TASK-048-005
**Parallel:** no

**Acceptance Criteria:**
- [x] New package `internal/gitrecover` with function `commitPending(repoDir string, featureStore stores.FeatureStore, bugStore stores.BugStore) ([]string, error)`
- [x] When COMMIT.md exists: calls `gitcommit.CommitIteration(repoDir, "")`, returns log message with first line of commit
- [x] When COMMIT.md absent, dirty tree, on task branch, task ID differs from next workable task: runs safety-gate unstage of internal files, then `git add -A && git commit -m "maggus: recover uncommitted changes from <branch>"`
- [x] When COMMIT.md absent, dirty tree, on task branch, task ID matches next workable task: returns nil (skip — resume scenario)
- [x] When clean tree or not on task branch: returns nil (no-op)
- [x] "Next workable task" determined by parsing features/bugs via stores and calling `parser.FindNextIncomplete`
- [x] Unit tests with a temporary git repo: COMMIT.md present scenario, dirty+mismatched scenario, dirty+matched (resume) scenario, clean repo scenario

---

### TASK-048-003: Implement Step 2 — consolidate task branches
**Description:** Add a function that detects when the repo is stuck on a task branch, finds the correct merge target (the original non-task, non-protected ancestor branch), merges the current branch into it, and deletes merged sibling task branches. If no ancestor branch exists, create a new integration branch (`feature/maggus` for features, `bug/maggus` for bugs) off the first existing protected branch.

**Token Estimate:** ~40k tokens
**Predecessors:** TASK-048-001
**Successors:** TASK-048-005
**Parallel:** yes — can run alongside TASK-048-002 (both depend only on TASK-048-001)

**Acceptance Criteria:**
- [x] Function `consolidateBranches(repoDir string, cfg config.Config) ([]string, error)` in `internal/gitrecover`
- [x] If current branch is not a task branch or is protected: returns nil (skip)
- [x] Finds merge target: lists branches that are ancestors of HEAD (`git branch --merged HEAD`), filters out task branches and protected branches, picks the one closest to HEAD
- [x] If no merge target found: creates `feature/maggus` (for TASK- IDs) or `bug/maggus` (for BUG- IDs) off the first existing protected branch from config
- [x] Checks out the merge target, runs `git merge --no-ff <current-task-branch>`
- [x] Finds sibling task branches (same prefix, e.g. `feature/maggus-004/*`) and deletes those that are ancestors of HEAD via `gitbranch.DeleteBranch`
- [x] Returns log messages for each action taken (merge, branch deletions)
- [x] If merge has conflicts: returns descriptive error, does not leave repo in mid-merge state (aborts merge)
- [x] Unit tests with temporary git repo: basic merge scenario, no-ancestor scenario (creates integration branch), sibling branch cleanup, merge conflict scenario

---

### TASK-048-004: Implement Step 3 — clean orphaned worktrees
**Description:** Add a function that removes leftover worktrees from `.maggus/worktrees/` and safely deletes their branches if merged. Skip worktrees that have active workers (check `state-workers.json` for `working` status entries).

**Token Estimate:** ~20k tokens
**Predecessors:** TASK-048-001
**Successors:** TASK-048-005
**Parallel:** yes — can run alongside TASK-048-002 and TASK-048-003

**Acceptance Criteria:**
- [x] Function `cleanOrphanedWorktrees(repoDir string) ([]string, error)` in `internal/gitrecover`
- [x] Calls `gitworktree.ListWorktrees(repoDir)` and filters to entries under `.maggus/worktrees/`
- [x] Skips worktrees whose task ID has status `working` in the worker index (via `runlog` package)
- [x] For each orphaned worktree: calls `gitworktree.RemoveWorktree`, then if the worktree's branch is a task branch AND is an ancestor of HEAD, deletes it via `gitbranch.DeleteBranch`
- [x] If a worktree branch has divergent (unmerged) work: removes only the worktree directory, leaves the branch, logs a warning
- [x] Removes `.maggus/worktrees/` directory if empty after cleanup
- [x] Returns log messages for each worktree removed and each branch deleted
- [x] Unit tests: orphaned worktree removal, branch cleanup for merged branches, divergent branch preserved with warning

---

### TASK-048-005: Wire RecoverDirtyState into daemon cycle
**Description:** Create the top-level `RecoverDirtyState` function that orchestrates all three steps and integrate it into `runOneDaemonCycle` in `daemon_keepalive.go`, called before `initIteration`. Send recovery log messages to the TUI as `InfoMsg` entries.

**Token Estimate:** ~20k tokens
**Predecessors:** TASK-048-002, TASK-048-003, TASK-048-004
**Successors:** none
**Parallel:** no

**Acceptance Criteria:**
- [x] Public function `gitrecover.RecoverDirtyState(repoDir string, cfg config.Config, featureStore stores.FeatureStore, bugStore stores.BugStore) ([]string, error)` that calls `commitPending`, `consolidateBranches`, and `cleanOrphanedWorktrees` in order
- [x] Each step is independent: if step 1 succeeds but step 2 fails, the error is returned but step 1's commit is preserved
- [x] Integrated into `daemon_keepalive.go` `runOneDaemonCycle`, before `initIteration`
- [x] Recovery messages sent via `cmd.Println` (same pattern as other daemon info messages)
- [x] Recovery errors are logged as warnings but do not abort the daemon cycle — the normal flow is attempted regardless
- [x] On a clean repo: function completes in under 100ms (just a few git status checks)
- [x] Unit test: orchestration calls all three steps, early failure in step 2 does not prevent step 3

---

## Task Dependency Graph

```
TASK-048-001 ──→ TASK-048-002 ──→ TASK-048-005
             ├─→ TASK-048-003 ──┘
             └─→ TASK-048-004 ──┘
```

| Task | Title | Estimate | Predecessors | Parallel | Model |
|------|-------|----------|--------------|----------|-------|
| TASK-048-001 | Export taskIDFromBranch + branch helpers | ~15k | none | yes | — |
| TASK-048-002 | Step 1: commit pending changes | ~30k | 001 | yes (with 003, 004) | — |
| TASK-048-003 | Step 2: consolidate task branches | ~40k | 001 | yes (with 002, 004) | — |
| TASK-048-004 | Step 3: clean orphaned worktrees | ~20k | 001 | yes (with 002, 003) | — |
| TASK-048-005 | Wire into daemon cycle | ~20k | 002, 003, 004 | no | — |

**Total estimated tokens:** ~125k

---

## Functional Requirements

- FR-1: If COMMIT.md exists at daemon startup, the pending changes must be committed using the existing `CommitIteration` logic before any new work begins
- FR-2: If the working tree is dirty without COMMIT.md and the current task branch does not match the next workable task, changes must be committed with a recovery message to prevent cross-task contamination
- FR-3: If the working tree is dirty and the current task branch matches the next workable task, the dirty state must be preserved (resume scenario)
- FR-4: If the repo is stuck on a task branch, all task branches for that feature must be merged into the original parent branch (the nearest non-task, non-protected ancestor)
- FR-5: If no suitable parent branch exists, a new integration branch (`feature/maggus` or `bug/maggus`) must be created off the first existing protected branch
- FR-6: Task branches that are fully merged (ancestors of HEAD) must be deleted after consolidation
- FR-7: Orphaned worktrees under `.maggus/worktrees/` must be removed, with their branches deleted if safely merged
- FR-8: Worktrees with active workers (status `working` in worker index) must not be removed
- FR-9: Recovery must be idempotent — interrupted recovery is retried on the next daemon cycle
- FR-10: Recovery must not abort the daemon cycle on failure — errors are logged as warnings

---

## Non-Goals

- No manual `maggus recover` command — recovery is automatic only
- No recovery of merge conflicts — if a merge conflicts during consolidation, the user must resolve manually
- No recovery of work done in worktrees that was never committed (divergent worktree branches are preserved, not merged)
- No changes to the `maggus clean` command — that remains user-triggered for completed file cleanup

---

## Technical Considerations

- **`taskIDFromBranch` export:** Currently unexported in `internal/gitmerge`. Exporting it (capital T) is a minor breaking change within the package but has no external consumers. All internal callers must be updated.
- **Merge strategy:** Use `git merge --no-ff` to preserve branch history. On conflict, abort the merge (`git merge --abort`) and return a descriptive error.
- **Protected branch detection:** Reuse `cfg.Git.ProtectedBranchList()` for the configured list. Check existence with `git rev-parse --verify` to find which actually exist in the repo.
- **Worker index check:** Use `runlog` package to read `state-workers.json` and check for `working` status entries before removing worktrees. The `PruneStaleWorkerEntries` call earlier in the daemon cycle already removes terminal entries older than 5 minutes.
- **Performance:** On a clean repo, the function should run three cheap git commands (status, rev-parse for current branch, worktree list) and return immediately. Feature/bug parsing only happens if the dirty-tree check triggers the task-mismatch logic.

---

## Success Metrics

- Daemon self-recovers from the exact turbomail scenario (uncommitted COMMIT.md + stuck task branch + orphaned worktrees) without user intervention
- Recovery adds less than 200ms to daemon startup on a clean repo
- No data loss: uncommitted agent work is always committed before branch operations

---

## Open Questions

_(none — all resolved during design)_
