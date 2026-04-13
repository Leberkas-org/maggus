<!-- maggus-id: c1606306-1444-4fd8-9886-bf756d793a2b -->
# Bug: Daemon pre-check drops all groups with predecessor tasks — nothing runs

## Summary

After BUG-046, the daemon pre-check calls `countWorkable(g.Tasks, nil, nil)`. `PredecessorsSatisfied(nil, nil)` returns `false` for any task whose `Predecessors` slice is non-empty — nil map lookups return `false`, so every predecessor fails the check. Groups whose only runnable tasks have predecessors are removed from `workableGroups` and the daemon returns `false, nil`. Nothing runs.

## Related

- **Bug:** bug_046_completed.md (introduced by this fix)

## Steps to Reproduce

1. Have a feature with two tasks: TASK-001 (complete, no predecessors) and TASK-002 (incomplete, `**Predecessors:** TASK-001`)
2. Run `maggus start`
3. Observe: status shows nothing — daemon idles immediately, no "Preparing", no work

## Root Cause

`daemon_keepalive.go:264`:
```go
if countWorkable(g.Tasks, nil, nil) > 0 {
```

`countWorkable` calls `IsRunnable(nil, nil)` → `PredecessorsSatisfied(nil, nil)`. For a task with `Predecessors: ["TASK-001"]`:

```go
for _, predID := range t.Predecessors {          // predID = "TASK-001"
    if !completedIDs[predID] && !skippedOrBlockedIDs[predID] {
    //  !nil["TASK-001"] && !nil["TASK-001"]
    //  !false           && !false
    //  true             && true  → return false
    }
}
```

The nil map lookup returns `false`, so the condition fires and `PredecessorsSatisfied` returns `false`. Every task with predecessors is treated as unrunnable.

The orchestrator works correctly because it builds a proper `skippedOrBlockedIDs` map (including unknown IDs) before calling `countRunnable`. The daemon pre-check has no such context and gets pessimistic nil-map behavior instead.

The docstring on `PredecessorsSatisfied` says nil maps "degrade gracefully to `IsWorkable()` behaviour", but the implementation does the opposite for tasks with predecessors.

## Fix

`PredecessorsSatisfied` should treat nil maps as "no predecessor context — assume all satisfied", matching the documented intent. When both maps are nil, return `true` immediately (equivalent to the old `IsWorkable()`-only check).

## User Stories

### BUG-048-001: Fix PredecessorsSatisfied to treat nil maps as "all satisfied"

**Description:** As a user, I want the daemon to correctly identify runnable tasks even when predecessor context isn't available yet, so that groups with predecessor-chained tasks are not silently dropped.

**Acceptance Criteria:**
- [x] `PredecessorsSatisfied` in `parser.go` returns `true` immediately when both `completedIDs` and `skippedOrBlockedIDs` are nil (degrade to `IsWorkable()` as documented)
- [x] `IsRunnable(nil, nil)` behaves identically to `IsWorkable()` for all task states
- [x] A feature with TASK-001 (complete) → TASK-002 (has `**Predecessors:** TASK-001`): daemon includes the group and the orchestrator runs TASK-002
- [x] Existing `TestIsRunnable_DegradesToIsWorkableWhenNilMaps` test covers tasks with predecessors (add cases if missing)
- [x] No regression in orchestrator predecessor enforcement (non-nil maps still enforce order correctly)
- [x] `go vet ./...` and `go test ./...` pass
