<!-- maggus-id: 5da19b06-94d2-4d79-a451-4286902cac67 -->
# Bug: "Predecessors: none <comment>" creates garbage predecessor IDs, permanently blocking the task

## Summary

The parser treats `**Predecessors:** none` as "no predecessors" via an exact string match. Any inline comment after "none" — e.g. `**Predecessors:** none (Feature 005 provides controllers, but middleware is independent)` — breaks the match. The remainder is then split on commas, producing invalid predecessor IDs like `"none (Feature 005 provides controllers"` and `"but middleware is independent)"`. These IDs never appear in `completedIDs`, so `IsRunnable` returns false forever, the daemon loops writing "Preparing", and the task never starts.

## Related

- **Bug:** bug_045.md, bug_046.md (same predecessor-handling family)

## Steps to Reproduce

1. Create a task with `**Predecessors:** none (any inline comment here)`
2. Run `maggus start`
3. Observe: task shows "Preparing" forever, execution plan labels it "unresolved"

## Expected Behavior

`**Predecessors:** none` with any trailing comment is treated identically to `**Predecessors:** none` — the task has no predecessors and runs normally.

## Root Cause

`src/internal/parser/parser.go:273`:

```go
if strings.ToLower(value) != "none" && value != "" {
```

The guard is an exact equality check. `"none (Feature 005 provides controllers, but middleware is independent)"` does not equal `"none"`, so the branch is taken. The value is then split on commas:

- `"none (Feature 005 provides controllers"` → appended as predecessor ID
- `"but middleware is independent)"` → appended as predecessor ID

Neither ID exists in the task list. `predecessorsComplete` returns false on every cycle. The task is permanently skipped and the daemon loops at "Preparing" with no progress visible anywhere.

## User Stories

### BUG-047-001: Fix parser to treat "none" prefix as no predecessors

**Description:** As a user, I want inline comments after "none" in the `**Predecessors:**` field to be ignored so that annotated "none" values don't create phantom predecessor IDs.

**Acceptance Criteria:**
- [x] The `"none"` check in `parser.go:273` uses `strings.HasPrefix(strings.ToLower(value), "none")` instead of exact equality
- [x] `**Predecessors:** none` → `Predecessors: []`
- [x] `**Predecessors:** none (any comment)` → `Predecessors: []`
- [x] `**Predecessors:** None — explanation text` → `Predecessors: []`
- [x] `**Predecessors:** TASK-007-001` → still parsed correctly as `["TASK-007-001"]`
- [x] Existing parser tests pass unchanged
- [x] `go vet ./...` and `go test ./...` pass

### BUG-047-002: Skip unknown predecessor IDs at runtime as a defensive fallback

**Description:** As a user running maggus against externally-generated feature files, I want tasks with predecessor IDs that don't match any task in the file to run normally, so that a bad reference never permanently deadlocks a task.

**Acceptance Criteria:**
- [x] A `knownTaskIDs` set is built from all task IDs in the group at the start of `runGroupTasks`
- [x] `IsRunnable` / `predecessorsComplete` treats a predecessor ID absent from `knownTaskIDs` as satisfied
- [x] A task with `**Predecessors:** NONEXISTENT-ID` runs as if it had no predecessors
- [x] A task with `**Predecessors:** TASK-A, NONEXISTENT-ID` runs as soon as TASK-A is complete
- [x] Tasks with valid-but-incomplete predecessors are still correctly held back
- [x] `go vet ./...` and `go test ./...` pass
