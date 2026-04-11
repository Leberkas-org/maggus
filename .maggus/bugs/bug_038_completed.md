<!-- maggus-id: b68f5625-13db-4103-bf45-be5766981992 -->
# Bug: Dispatched workers don't kill agent subprocess on cleanup, blocking worktree removal

## Summary

When a dispatched worker fails or its worktree needs to be reused, the cleanup step (`cleanupExistingWorktree`) can't remove the worktree directory because the agent subprocess (claude.exe and its node child processes) is still running and holding files open. Maggus kills its own process but doesn't kill the agent subprocess tree it spawned.

## Steps to Reproduce

1. Dispatch a task via `Alt+R` in the status TUI
2. While the task is running, dispatch it again (or dispatch fails and needs re-dispatch)
3. Observe: "Dispatch failed: cleanup existing worktree: ... The process cannot access the file because it is being used by another process"
4. After killing all maggus processes, the agent subprocess (claude.exe + node.exe children) remain running and lock the worktree files

## Expected Behavior

When a dispatched worker exits (success, failure, or re-dispatch), it should kill its agent subprocess tree before exiting. The worktree should be cleanable immediately after the worker process ends.

## Root Cause

Two issues:

### 1. Dispatched worker doesn't kill agent subprocess on exit

The dispatched process runs `maggus run --task ... --daemon-run` which invokes the agent via `activeAgent.Run()`. When the dispatched process exits (or is killed), the agent subprocess continues running as an orphan because:

- On Windows, `launchDaemon` uses `CREATE_NEW_PROCESS_GROUP` for detachment, but child processes of the dispatched worker inherit no process group relationship
- The agent subprocess (claude.exe) spawns its own node.exe children which are fully independent processes

### 2. `cleanupExistingWorktree` doesn't check for running workers

`dispatch.go:132-143` tries `gitworktree.RemoveWorktree` then falls back to `os.RemoveAll`, but doesn't check if a worker process is still running in that worktree. It should find and kill the worker (and its agent subprocess) before attempting cleanup.

## User Stories

### BUG-038-001: Kill agent subprocess tree when dispatched worker exits

**Description:** As a user, I want dispatched workers to clean up their agent subprocesses so worktrees can be removed without manual intervention.

**Acceptance Criteria:**
- [x] When a dispatched worker completes (success or failure), it kills the agent subprocess tree before exiting
- [x] On Windows: use `taskkill /T /F /PID` (tree kill) to kill the agent and all its children
- [x] On Unix: use process group kill to terminate the agent tree
- [x] `cleanupExistingWorktree` in `dispatch.go` checks the workers index for a running worker at the target path and attempts to kill it before removing the worktree
- [x] After a failed dispatch + re-dispatch, the worktree is cleaned up and the new dispatch succeeds
- [x] No regression in normal agent subprocess termination (Ctrl+C, stop-after-task)
- [x] `go vet ./...` and `go test ./...` pass
