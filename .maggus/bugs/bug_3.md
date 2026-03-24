# Bug: Approval system uses filename as ID — collisions and stale entries

## Summary

`feature_approvals.yml` keys are derived from filenames (`bug_1`, `feature_001`). When a completed file is replaced by a new one with the same name, the old approval entry incorrectly pre-approves the new file. Completed entries are never removed, causing unbounded accumulation of stale keys.

## Steps to Reproduce

**Collision / false pre-approval:**
1. Create `bug_1.md`, approve it (`maggus approve bug_1`)
2. Complete the bug — maggus renames it to `bug_1_completed.md`, entry `bug_1: true` stays in `feature_approvals.yml`
3. Create a new `bug_1.md` (a different bug)
4. Observe: `feature_approvals.yml` still has `bug_1: true` — the new file is already "approved" without the user ever approving it

**Stale accumulation:**
1. Run maggus through several features and bugs
2. Open `.maggus/feature_approvals.yml`
3. Observe: entries for all completed features remain indefinitely

## Expected Behavior

Each feature/bug file has a stable unique identity independent of its filename. Approval state follows the identity, not the name. When a file is completed and removed, its approval entry is also removed.

## Root Cause

`featureIDFromPath` in `src/cmd/approve.go:18` derives the approval key purely from the filename:

```go
func featureIDFromPath(path string) string {
    base := filepath.Base(path)
    base = strings.TrimSuffix(base, ".md")
    base = strings.TrimSuffix(base, "_completed")
    return base
}
```

This function is called in four places:
- `src/cmd/approve.go:33` — building approval IDs for all active features
- `src/cmd/status.go:364` — toggling approval in the status TUI
- `src/cmd/status_plans.go:81,109` — checking approval state for display

And the work loop at `src/cmd/work_loop.go:194,212` does the same inline:

```go
id := strings.TrimSuffix(filepath.Base(f), ".md")
if !approval.IsApproved(approvals, id, approvalRequired) {
```

Neither `MarkCompletedFeatures` nor `MarkCompletedBugs` (called in `src/cmd/work_task.go:137-138`) removes the corresponding approval entry after renaming/deleting the file.

The fix: embed a UUID in each feature/bug file's header. Use the UUID as the `feature_approvals.yml` key. Remove the entry on completion.

## User Stories

### BUG-003-001: Add GUID header support to parser

**Description:** As a developer, I want the parser to read and write a stable UUID from each feature/bug file so that approval identity is tied to content, not filename.

**Acceptance Criteria:**
- [ ] `src/internal/parser` exposes `GetOrCreateFileGUID(path string) (string, error)` that reads the first line of the file for a comment `<!-- maggus-id: <uuid> -->` and returns the UUID; if absent, generates a new UUID v4, prepends the comment line to the file, and returns it
- [ ] `src/internal/parser` exposes `GetFileGUID(path string) (string, error)` that reads and returns the UUID without creating one (returns `""` if absent)
- [ ] The comment line is added before the `#` title line with a blank line between comment and title, so the file renders cleanly in markdown
- [ ] A new dependency on a UUID library (e.g. `github.com/google/uuid`) is added to `go.mod`/`go.sum`
- [ ] Unit tests for `GetOrCreateFileGUID` cover: file with existing GUID (idempotent), file without GUID (creates and writes), missing file (returns error)
- [ ] `go vet ./...` and `go test ./...` pass

### BUG-003-002: Use file GUID as approval key in the work loop

**Description:** As a user, I want the work loop to identify features and bugs by their embedded GUID so that renaming or replacing a file does not corrupt approval state.

**Acceptance Criteria:**
- [ ] `src/cmd/work_loop.go` calls `parser.GetOrCreateFileGUID(f)` instead of `strings.TrimSuffix(filepath.Base(f), ".md")` for both bug and feature groups (`work_loop.go:194` and `work_loop.go:212`)
- [ ] The `featureGroup` struct's `id` field is set to the GUID
- [ ] Approval is checked against the GUID key
- [ ] `featureIDFromPath` in `src/cmd/approve.go` is updated or replaced: interactive approve/unapprove pickers display the filename as a label but pass the GUID to `approval.Approve`/`approval.Unapprove`
- [ ] `src/cmd/status.go:364` and `src/cmd/status_plans.go:81,109` look up the GUID from the file instead of deriving an ID from the path
- [ ] No regression in opt-in / opt-out approval modes
- [ ] `go vet ./...` and `go test ./...` pass

### BUG-003-003: Remove approval entry when a feature or bug is completed

**Description:** As a user, I want completed feature/bug approval entries to be removed from `feature_approvals.yml` so the file stays clean and IDs cannot leak into future files.

**Acceptance Criteria:**
- [ ] `src/internal/parser.MarkCompletedFeatures` and `MarkCompletedBugs` return the GUIDs of the files they rename or delete
- [ ] `src/cmd/work_task.go:137-138` (callers of `MarkCompletedFeatures`/`MarkCompletedBugs`) calls `approval.Remove(dir, guid)` for each returned GUID
- [ ] `src/internal/approval` exposes a `Remove(dir, featureID string) error` function that deletes the key from `feature_approvals.yml` and persists the result
- [ ] After a feature or bug is completed in a work run, its GUID no longer appears in `feature_approvals.yml`
- [ ] `go vet ./...` and `go test ./...` pass
