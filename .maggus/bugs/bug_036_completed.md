<!-- maggus-id: ed2b5d33-01e5-4bde-bd3b-e65c4b2dcca1 -->
# Bug: Stop-after-task signal ignored in parallel/worktree mode

## Summary

Pressing `s` (stop after task) in the status TUI has no effect when the daemon is running in parallel mode. The daemon continues launching new task batches until all tasks are done or `Ctrl+C` is used. In sequential mode, stop-after-task works correctly.

## Steps to Reproduce

1. Enable `parallel: true` in config
2. Start the daemon with a feature that has multiple parallel tasks
3. While tasks are running, press `s` → select "stop after task"
4. Observe: the daemon continues working through all tasks, ignoring the stop signal

## Expected Behavior

After pressing "stop after task", the daemon should finish the currently running batch and then stop — no new batches should be launched.

## Root Cause

The parallel orchestrator in `src/cmd/run_parallel.go` never checks `stopFlagAtomic`. 

The stop-after-task mechanism works via a sentinel file (`.maggus/daemon.stop-after-task`). In `daemon_keepalive.go:345-361`, a goroutine polls for this file and sets `stopFlagAtomic.Store(true)` when found. After the work cycle returns, `daemon_keepalive.go:412` checks `stopFlagAtomic.Load()` and returns `errStopAfterTask`.

But `runParallelWorkGoroutine` (line 66) and `parallelOrchestrator.run()` (line 138) only check `o.ctx.Err()` for cancellation — they never read `stopFlagAtomic`. The outer loop at line 148 keeps iterating through task batches because the flag is invisible to it.

The `stopFlagAtomic` is created in `runOneDaemonCycle` (line 368) and passed to `runLoopParams`, but `runParallelWorkGoroutine` never accesses `params.stopFlag`.

## User Stories

### BUG-036-001: Check stopFlag in parallel orchestrator loop

**Description:** As a user, I want stop-after-task to work in parallel mode so I can gracefully stop the daemon between task batches.

**Acceptance Criteria:**
- [x] `parallelOrchestrator` gains access to the `stopFlag *atomic.Bool` (passed through `runLoopParams` or the orchestrator struct)
- [x] The main loop in `parallelOrchestrator.run()` (line 148) checks `stopFlag.Load()` alongside `o.ctx.Err()` — if true, sets `result.stopReason = StopReasonInterrupted` and breaks
- [x] The check happens BETWEEN batches (after a batch completes, before launching the next) — currently running tasks finish cleanly
- [x] After the fix, pressing "stop after task" in parallel mode stops the daemon after the current batch completes
- [x] Sequential mode stop-after-task behavior is unchanged (no regression)
- [x] `go vet ./...` and `go test ./...` pass
