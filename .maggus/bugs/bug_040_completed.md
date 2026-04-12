<!-- maggus-id: 9341932f-2bc0-43a5-a38b-f7f62b328fb7 -->
# Bug: runlog.Open parameter named maggusID but receives runID

## Summary

`runlog.Open` has its first parameter named `maggusID`, but all callers pass a `runID` (daemon session timestamp). These are fundamentally different concepts — `maggusID` is a feature/bug GUID from markdown frontmatter, while `runID` is a timestamp identifying a daemon session. The misleading name makes the code confusing and error-prone.

## Steps to Reproduce

1. Read `src/internal/runlog/runlog.go:67` — parameter is `maggusID`
2. Read `src/cmd/daemon_keepalive.go:92` — passes `runID` (a timestamp string)
3. Read `src/internal/runlog/runlog.go:17` — `Logger.currentMaggusID` stores actual feature GUIDs
4. Note the contradiction: the `Open` parameter and the `Logger` field share the `maggusID` name but hold completely different values

## Expected Behavior

The `Open` function parameter should be named `runID` to accurately reflect what it receives — a daemon session timestamp. The doc comment and log filename description should also reflect this.

## Root Cause

When `runlog.Open` was originally written, the first parameter was named `maggusID` generically. Over time, `maggusID` gained a specific meaning elsewhere in the codebase: the stable GUID from `<!-- maggus-id: ... -->` in feature/bug files (see `parser.ParseMaggusID`, `Plan.MaggusID`, `Logger.currentMaggusID`, `Entry.MaggusID`). The `Open` parameter was never updated to match.

Specifically:
- `src/internal/runlog/runlog.go:67` — `Open(maggusID, dir string, maxFiles int)` — parameter name is wrong
- `src/internal/runlog/runlog.go:63-66` — doc comment says `<timestamp>_<maggusID>.log` but the value is a `runID`
- All test call sites use arbitrary strings like `"abc-uuid"`, `"someid"`, `"run1"`, `"new-uuid"` which reinforce the confusion

The following are correct and should NOT be changed:
- `Logger.currentMaggusID` — holds actual feature GUIDs, set via `SetCurrentMaggusID()`
- `Entry.MaggusID` — JSON field in log entries, populated from `currentMaggusID`
- `SetCurrentMaggusID()` — receives feature GUIDs from `Plan.MaggusID`

## User Stories

### BUG-040-001: Rename runlog.Open parameter from maggusID to runID

**Description:** As a developer, I want the `runlog.Open` parameter named correctly so that the distinction between feature GUIDs (maggusID) and daemon session timestamps (runID) is clear.

**Acceptance Criteria:**
- [x] `runlog.Open` signature changed from `Open(maggusID, dir string, maxFiles int)` to `Open(runID, dir string, maxFiles int)`
- [x] Doc comment on `Open` updated: references `runID` instead of `maggusID`, describes filename as `<runID>_<ts>.log`
- [x] Internal variable in `Open` body updated (`maggusID` -> `runID` on line 75/78)
- [x] Test call sites updated to use realistic runID-style values (timestamp strings like `"20260101-100000"`) instead of GUID-like strings (`"abc-uuid"`, `"someid"`, `"new-uuid"`, `"run1"`, etc.)
- [x] `Logger.currentMaggusID`, `SetCurrentMaggusID`, and `Entry.MaggusID` are NOT changed (they correctly use feature GUIDs)
- [x] No regression in related functionality
- [x] `go vet ./...` and `go test ./...` pass
