# Daemon Auto-Recovery Design

## Problem

When a daemon run is interrupted (crash, kill, power loss), it can leave the repository in a dirty state that prevents subsequent runs from making progress:

- **Uncommitted changes** with a COMMIT.md present (agent finished but commit step never ran)
- **Stuck on a task branch** that can't be consolidated back into the parent branch
- **Orphaned git worktrees** from interrupted parallel runs, locking branches

The daemon silently fails in this state: it starts the agent, the agent explores for a few minutes, and then stops without completing. No error is surfaced. The user has to manually diagnose and clean up.

## Solution

A `RecoverDirtyState` function runs at the top of every daemon cycle, before `initIteration`. It detects signs of an interrupted run and fixes them automatically. If the repo is clean, it's a fast no-op.

### Location

New package: `internal/gitrecover`

Entry point:
```go
func RecoverDirtyState(repoDir string, cfg config.Config, featureStore stores.FeatureStore, bugStore stores.BugStore) ([]string, error)
```

Returns a list of human-readable log messages describing what was fixed. The caller (`runOneDaemonCycle` in `daemon_keepalive.go`) sends these as `InfoMsg` to the TUI.

### Integration Point

Called in `daemon_keepalive.go` at the top of `runOneDaemonCycle`, before `initIteration`:

```go
if msgs, err := gitrecover.RecoverDirtyState(dir, wc.cfg, featureStore, bugStore); err != nil {
    // log warning but don't abort — let the normal flow attempt to proceed
} else {
    for _, msg := range msgs {
        cmd.Println(msg)
    }
}
```

## Recovery Steps

The three steps run in order. Each is idempotent — if recovery is interrupted, the next cycle retries from wherever it left off.

### Step 1: Commit Pending Changes

**Goal:** Get the working tree clean so subsequent steps can switch branches.

| Condition | Action |
|-----------|--------|
| COMMIT.md exists | Call `gitcommit.CommitIteration(repoDir, "")`. Log recovered commit message. |
| No COMMIT.md, dirty tree, on a task branch, branch task ID **differs** from next workable task | Safety-gate unstage internal files, then `git add -A && git commit -m "maggus: recover uncommitted changes from <branch>"`. |
| No COMMIT.md, dirty tree, on a task branch, branch task ID **matches** next workable task | Skip — this is a resume. The agent will pick up where it left off. |
| Clean tree or not on a task branch | Skip. |

**Determining "next workable task":** Parse features and bugs via the provided stores, then call `parser.FindNextIncomplete` on the merged task list (same logic as `initIteration`).

**Extracting task ID from branch name:** Use `gitmerge.taskIDFromBranch` (or extract the equivalent logic) to derive e.g. `TASK-004-007` from `feature/maggus-004/task-007`.

### Step 2: Consolidate Task Branches

**Goal:** Merge completed task work back into the parent branch so the daemon starts from a unified state.

**Precondition:** Working tree is clean (step 1 ensured this). Current branch is a task branch. If not on a task branch, skip this step entirely.

#### 2a. Find the merge target

1. List all local branches that are ancestors of HEAD (`git branch --merged HEAD`).
2. Filter out branches matching the task branch pattern (`feature/maggus-NNN/task-NNN`, `bugfix/maggus-bug-NNN/task-NNN`).
3. Filter out protected branches (from `cfg.Git.ProtectedBranchList()`).
4. If any remain: pick the one whose tip is closest to HEAD (fewest commits behind). This is the "original" branch the feature work started from.
5. If none remain: create a new integration branch off the first existing protected branch:
   - Feature task (ID starts with `TASK-`) → `feature/maggus`
   - Bug task (ID starts with `BUG-`) → `bug/maggus`

#### 2b. Merge and clean up

1. Checkout the merge target.
2. `git merge --no-ff <current-task-branch>` into the target.
3. Find all other task branches for the same feature prefix (e.g. `feature/maggus-004/*`).
4. For each: if it's an ancestor of HEAD (`git merge-base --is-ancestor`) → `git branch -d` (safe delete, only deletes if merged).
5. Log each deleted branch.

### Step 3: Clean Orphaned Worktrees

**Goal:** Remove leftover worktrees from interrupted parallel runs.

1. Call `gitworktree.ListWorktrees(repoDir)`.
2. For each worktree whose path is under `.maggus/worktrees/`:
   a. `gitworktree.RemoveWorktree(repoDir, path)` (force remove).
   b. If the worktree's branch is a task branch AND is an ancestor of HEAD → `git branch -d` (safe delete).
   c. If the branch is NOT an ancestor (has divergent work) → leave the branch, only remove the worktree directory. Log a warning.
3. Remove `.maggus/worktrees/` directory if empty.

## Task Branch Detection

A branch is a "task branch" if it matches one of these patterns:
- `feature/maggus-NNN/task-NNN` (feature tasks)
- `bugfix/maggus-bug-NNN/task-NNN` (bug tasks)

Use `gitbranch.BranchName` patterns or a dedicated regex. The existing `gitmerge.taskIDFromBranch` function already derives task IDs from branch names — reuse or expose that logic.

## Edge Cases

| Scenario | Behavior |
|----------|----------|
| Repo is clean, on protected branch | All three steps are no-ops. Fast path. |
| Repo is clean, on a non-task, non-protected branch | All steps skip. Daemon works from this branch normally. |
| COMMIT.md exists but `git commit` fails (e.g. empty diff) | `CommitIteration` already handles "nothing to commit" — deletes COMMIT.md and returns gracefully. |
| Dirty tree, no COMMIT.md, not on a task branch | Step 1 skips. Could be user's own work. Don't touch it. |
| Merge target checkout fails | Return error. Next cycle will retry. |
| Merge into target has conflicts | Return error with descriptive message. User must resolve manually. |
| Worktree branch has divergent (unmerged) work | Remove worktree directory but preserve the branch. Log warning. |
| Parallel workers still running (worktrees in active use) | Only remove worktrees that have no running agent process. Check the worker index (`state-workers.json`) for entries with status `working` — skip those worktrees. Stale entries (no process) are already pruned by `PruneStaleWorkerEntries` which runs earlier in the daemon cycle. |
| Multiple feature prefixes have orphaned branches | Step 2 only consolidates branches matching the current branch's feature prefix. Other features' branches are untouched. |

## Performance

On a clean repo (the common case), the function runs:
- One `git status --porcelain` to check for dirty tree
- One `git rev-parse --abbrev-ref HEAD` to get current branch
- One `git worktree list --porcelain` to check for orphaned worktrees

All are sub-second operations. No parsing of feature files unless the dirty-tree check triggers step 1's task-mismatch logic.

## Existing Code Reused

| Component | Package | Usage |
|-----------|---------|-------|
| `CommitIteration` | `internal/gitcommit` | Step 1: commit with COMMIT.md |
| `ListWorktrees`, `RemoveWorktree` | `internal/gitworktree` | Step 3: detect and remove worktrees |
| `IsProtected`, `BranchName` | `internal/gitbranch` | Step 2: classify branches |
| `DeleteBranch` | `internal/gitbranch` | Steps 2-3: clean up merged branches |
| `taskIDFromBranch` | `internal/gitmerge` (currently unexported) | Steps 1-2: extract task ID from branch |
