<!-- maggus-id: 66d1aedb-db05-465a-8d31-09685af5ae2a -->
# Feature 046: Clean Up Task Branches and Worktrees After Successful Merge

## Introduction

When a parallel task completes and its branch is successfully merged back into the plan branch, the task branch and worktree are left behind. Over time this accumulates stale branches and worktree directories that clutter `git branch` output and waste disk space. They should be deleted automatically after a successful merge.

### Architecture Context

- **Vision alignment:** Clean git state is part of a good developer experience
- **Components involved:** `cmd/run_parallel.go` (merge + cleanup), `internal/gitworktree` (worktree removal), `internal/gitbranch` (branch deletion — needs new function)
- **New patterns:** None — just adding cleanup calls after existing merge logic

## Goals

- Task branches are deleted after successful merge into the plan branch
- Worktrees are removed after successful merge (parallel mode)
- Failed/blocked tasks keep their branches and worktrees for manual inspection

## User Stories

### TASK-046-001: Delete task branch and worktree after successful merge
**Description:** As a developer, I want task branches and worktrees to be cleaned up automatically after merging so I don't accumulate stale git state.

**Token Estimate:** ~30k tokens
**Predecessors:** none
**Successors:** none
**Parallel:** no

**Acceptance Criteria:**
- [x] After `gitmerge.MergeTaskBranch` succeeds in `run_parallel.go` (line ~372 for worktree tasks, line ~379 for sequential tasks), the task branch is deleted via `git branch -d <taskBranch>`
- [x] For worktree tasks: the worktree at `.maggus/worktrees/<taskID>/` is removed via `gitworktree.RemoveWorktree` after the merge succeeds (currently only done on failure paths)
- [x] A new `gitbranch.DeleteBranch(repoDir, branchName string) error` function is added (runs `git branch -d`) — uses `-d` (not `-D`) so it only deletes fully-merged branches
- [x] If branch deletion fails (e.g., branch doesn't exist), the error is logged but does not fail the task — cleanup is best-effort
- [x] If worktree removal fails, the error is logged but does not fail the task
- [x] Failed and blocked tasks still preserve their branches and worktrees (existing behavior unchanged)
- [x] After a successful parallel run, `git branch` shows no leftover task branches — only the plan branch remains
- [x] After a successful parallel run, `.maggus/worktrees/` contains no leftover task directories
- [x] Unit test for `gitbranch.DeleteBranch` covers: normal deletion, already-deleted branch, unmerged branch (should error with `-d`)
- [x] `go vet ./...` and `go test ./...` pass

## Task Dependency Graph

```
TASK-046-001 (single task)
```

| Task | Estimate | Predecessors | Parallel | Model |
|------|----------|--------------|----------|-------|
| TASK-046-001 | ~30k | none | no | — |

**Total estimated tokens:** ~30k

## Functional Requirements

- FR-1: After a successful merge, the task branch is deleted with `git branch -d`
- FR-2: After a successful merge of a worktree task, the worktree directory is removed
- FR-3: Cleanup failures are logged but never fail the task or the work loop
- FR-4: Failed/blocked tasks retain their branches and worktrees

## Non-Goals

- No cleanup of plan-level branches (those are managed by the user or `maggus clean`)
- No cleanup of branches from previous interrupted runs (that's bug_028 / manual `maggus worktree clean`)
- No remote branch deletion (`git push --delete`)

## Technical Considerations

- Use `git branch -d` (lowercase) not `-D` — this ensures the branch is only deleted if fully merged, providing a safety net
- The worktree must be removed BEFORE the branch can be deleted — `git branch -d` refuses to delete a branch that has a worktree checked out
- Order: remove worktree → delete branch → mark complete

## Open Questions

None.
