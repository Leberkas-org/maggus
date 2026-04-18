# Git Management

Maggus owns all git operations for the repositories it manages. This includes branching, worktree lifecycle, committing, rebasing, merging, and cleanup — including recovery from failed or interrupted runs.

---

## Branching Model

```
main (or master)
 └── feature/F-12                      # feature branch (1 per work item)
      ├── feature/F-12/TASK-001        # task branch (1 per task, lives in worktree)
      ├── feature/F-12/TASK-002
      └── feature/F-12/TASK-003

 └── fix/BUG-001                       # bug fix branch
      └── fix/BUG-001/TASK-001
```

- **Protected branches** (`main`, `master`, `dev`) — never modified directly. Configured via `git.protected_branches` in config.
- **Feature/fix branch** — created per work item from the default branch. All task branches merge back here.
- **Task branch** — created per task from the feature branch. Each task branch lives in its own worktree.

### Branch Naming

Branch names are derived from the plan file:
- Feature: `feature/<item_id>` (e.g., `feature/F-12`)
- Bug fix: `fix/<item_id>` (e.g., `fix/BUG-001`)
- Task: `<parent_branch>/<task_id>` (e.g., `feature/F-12/TASK-001`)

If Bryan is connected, `TaskAssignment.branch_name` is used directly.

---

## Worktree Lifecycle

Each task runs in an isolated git worktree so multiple tasks can execute in parallel without conflicts.

```
<repo>/                                # main checkout (feature branch)
<repo>/.maggus/worktrees/
  TASK-001/                            # worktree for task 001
  TASK-002/                            # worktree for task 002
```

### Create

```
1. Ensure feature branch exists (create from default branch if not)
2. Create task branch from feature branch:
   git branch feature/F-12/TASK-001 feature/F-12
3. Create worktree:
   git worktree add <repo>/.maggus/worktrees/TASK-001 feature/F-12/TASK-001
```

### Work

Agent subprocess runs with `WorkDir` set to the worktree path. All file operations, tool calls, and commits happen inside the worktree. The main checkout is untouched.

### Merge Back

After the agent completes and commits:

```
1. Fetch latest feature branch state
2. Rebase task branch onto feature branch:
   git rebase feature/F-12 feature/F-12/TASK-001
3. Fast-forward merge into feature branch:
   git checkout feature/F-12
   git merge --ff-only feature/F-12/TASK-001
```

Merge operations are **serialized per feature branch** via a per-branch mutex. Two tasks from the same feature cannot merge simultaneously.

### Cleanup

After successful merge:

```
1. Remove worktree:
   git worktree remove <repo>/.maggus/worktrees/TASK-001
2. Delete task branch:
   git branch -d feature/F-12/TASK-001
```

---

## Commit Strategy

### During Task Execution

The agent (Claude Code, OpenCode) handles commits during its work. Maggus does not interfere with the agent's commit behavior while it's running.

### Post-Completion Commit

If the agent finishes with uncommitted changes:

```
1. Check for uncommitted changes:
   git status --porcelain
2. If changes exist:
   a. Stage all changes: git add -A
   b. Commit with auto-generated message:
      "chore(<task_id>): auto-commit uncommitted changes after agent completion"
```

### COMMIT.md Convention

If the agent wrote a `COMMIT.md` file in the worktree root, use its content as the commit message and delete the file after committing.

---

## Rebase Strategy

Maggus always rebases task branches before merging to keep a linear history on the feature branch.

```go
func (o *ops) MergeTaskBranch(repoRoot, featureBranch, taskBranch string) error {
    // 1. Checkout feature branch
    // 2. Pull latest (if remote exists)
    // 3. Rebase task branch onto feature branch
    // 4. If rebase conflicts:
    //    a. Abort rebase
    //    b. Fall back to merge commit (non-ff)
    //    c. Log warning
    // 5. Fast-forward merge
    // 6. If ff-only fails (shouldn't after rebase):
    //    a. Create merge commit as last resort
}
```

### Conflict Handling

If rebase produces conflicts:
1. **Abort** the rebase (`git rebase --abort`)
2. **Fall back** to a regular merge commit
3. **Log a warning** — the task completed but the merge was not clean
4. **Mark the item** for manual review in the TUI (⚠ icon)

Maggus never resolves conflicts automatically. A conflict means the work needs human review.

---

## Failure Recovery

The daemon must handle crashes, interrupted runs, and leftover state gracefully. Recovery runs at daemon startup and is idempotent (safe to run repeatedly).

### RecoverDirtyState()

Called at the top of every daemon start:

```
1. Recover uncommitted changes
2. Consolidate orphaned task branches
3. Clean orphaned worktrees
```

### Step 1: Recover Uncommitted Changes

For each registered repo, check all worktrees for uncommitted changes:

```
For each worktree in <repo>/.maggus/worktrees/:
  1. git status --porcelain in the worktree
  2. If changes exist:
     a. Check for COMMIT.md → use as commit message
     b. Otherwise auto-commit: "chore: recover uncommitted changes from interrupted run"
  3. Log what was recovered
```

### Step 2: Consolidate Orphaned Task Branches

Task branches that were committed but never merged back:

```
For each task branch matching <feature_branch>/<task_id>:
  1. Check if the task branch has commits ahead of the feature branch
  2. If yes:
     a. Attempt rebase + fast-forward merge
     b. If merge succeeds: delete task branch
     c. If merge fails (conflicts): leave branch, log warning, mark for review
  3. If no commits ahead: just delete the branch (nothing to save)
```

### Step 3: Clean Orphaned Worktrees

Worktrees left behind from crashed workers:

```
1. git worktree list --porcelain
2. For each worktree in <repo>/.maggus/worktrees/:
   a. Check if a corresponding worker is active (via state.json)
   b. If no active worker:
      - Check for uncommitted changes (Step 1 handles this)
      - Remove worktree: git worktree remove --force <path>
      - Prune: git worktree prune
3. Also clean filesystem directories that git doesn't know about:
   For each dir in <repo>/.maggus/worktrees/:
     If not in git worktree list: rm -rf the directory
```

### Step 4: Prune Stale References

```
git worktree prune
git remote prune origin (if remote exists)
```

---

## Git Tidiness Rules

The daemon enforces these rules to keep repos clean:

1. **No orphaned worktrees** — cleaned on startup and after every task completion
2. **No orphaned task branches** — merged or deleted after task completion, recovered on startup
3. **No uncommitted changes in worktrees** — committed on recovery
4. **Feature branches stay until item is fully done** — only deleted when all tasks complete (or user explicitly deletes)
5. **Protected branches are never touched** — IsProtected() check before any branch operation
6. **Worktrees live under `.maggus/worktrees/`** — not scattered across the filesystem
7. **Merges are serialized per feature** — prevents race conditions when parallel tasks finish

---

## Operations Interface

```go
type Operations interface {
    // Branch
    CurrentBranch(dir string) (string, error)
    DefaultBranch(dir string) (string, error)
    BranchExists(dir string, branch string) bool
    CreateBranch(dir, name, from string) error
    CheckoutBranch(dir, name string) error
    DeleteBranch(dir, name string) error
    IsProtected(branch string) bool

    // Worktree
    CreateWorktree(repoRoot, path, branch string) error
    RemoveWorktree(repoRoot, path string) error
    ListWorktrees(repoRoot string) ([]WorktreeInfo, error)
    PruneWorktrees(repoRoot string) error

    // Merge
    MergeTaskBranch(repoRoot, featureBranch, taskBranch string) error
    RebaseOnto(dir, upstream, branch string) error
    AbortRebase(dir string) error

    // Commit
    StageAll(dir string) error
    Commit(dir, message string) (hash string, err error)
    HasChanges(dir string) bool
    ReadCommitFile(dir string) (string, error)  // reads COMMIT.md if present

    // Sync
    Fetch(dir string) error
    Pull(dir string) error
    RemoteExists(dir string) bool
    RepoURL(dir string) string

    // Recovery
    RecoverDirtyState(repoRoot string) error
}

type WorktreeInfo struct {
    Path   string
    Branch string
    Bare   bool
}
```

All operations go through the `Commander` interface for testability:

```go
type Commander interface {
    Run(dir string, args ...string) error
    Output(dir string, args ...string) (string, error)
    CombinedOutput(dir string, args ...string) (string, error)
}
```
