<!-- maggus-id: dcab552e-c1c8-4543-a77e-4a6b4fdaaa5b -->
# Feature 002: Unified Feature/Bug Loading

## Introduction

Replace three parallel implementations of feature/bug file loading (`featureInfo` in `status_plans.go`, `featureGroup` in `work_loop.go`, `featureEntry` in `approve.go`) with a single `parser.Plan` type. Currently the same "glob files, parse them, extract metadata" logic is duplicated with inconsistent behavior — different struct fields, different `_completed` suffix handling, and different approval filtering strategies.

## Goals

- Single source of truth for loading and representing feature/bug files
- Consistent file ID extraction (handling `_completed` suffix)
- Eliminate 3 redundant struct types and their associated load functions
- Centralize the glob+parse+metadata pattern in `internal/parser/`

## Tasks

### TASK-002-001: Create parser.Plan type and LoadPlans function
**Description:** As a developer, I want a unified `Plan` type in the parser package so that all consumers use the same struct and loading logic.

**Token Estimate:** ~40k tokens
**Predecessors:** none
**Successors:** TASK-002-002
**Parallel:** no

**Acceptance Criteria:**
- [x] New file `src/internal/parser/plan.go` with `Plan` struct containing: `ID`, `MaggusID`, `File`, `Tasks`, `IsBug`, `Completed`
- [x] `Plan.ApprovalKey()` returns `MaggusID` if set, otherwise `ID`
- [x] `Plan.DoneCount()` returns count of completed tasks
- [x] `Plan.BlockedCount()` returns count of incomplete+blocked tasks
- [x] `LoadPlans(dir string, includeCompleted bool) ([]Plan, error)` loads all feature and bug files, bugs first
- [x] `LoadPlans` correctly handles `_completed` suffix detection
- [x] `LoadPlans` calls `MigrateLegacyBugIDs` for bug files (matching current `work_loop.go` behavior)
- [x] New file `src/internal/parser/plan_test.go` with tests for all methods and edge cases
- [x] `go test ./internal/parser/` passes
- [x] Approval status is NOT stored on `Plan` (callers look it up separately)

### TASK-002-002: Migrate all consumers to parser.Plan
**Description:** As a developer, I want all feature/bug file consumers to use `parser.Plan` so that the duplicated structs and load functions can be deleted.

**Token Estimate:** ~75k tokens
**Predecessors:** TASK-002-001
**Successors:** none
**Parallel:** no
**Model:** opus

**Acceptance Criteria:**
- [ ] `cmd/status_plans.go`: `featureInfo` struct deleted, `parseFeatures()` deleted, `parseBugs()` deleted — replaced with `parser.LoadPlans()` + inline approval lookup
- [ ] `cmd/work_loop.go`: `featureGroup` struct deleted, `buildApprovedFeatureGroups()` deleted — replaced with `parser.LoadPlans(dir, false)` + approval filtering
- [ ] `cmd/approve.go`: `featureEntry` struct deleted, `featureIDFromPath()` deleted, `listActiveFeatures()` deleted — replaced with `parser.LoadPlans()`
- [ ] `cmd/status.go`: all `featureInfo` references updated to `parser.Plan`
- [ ] `cmd/menu.go`: `loadFeatureSummary()` updated to use `parser.LoadPlans()`
- [ ] `cmd/status_test.go` updated for `parser.Plan` types
- [ ] `cmd/work_loop_test.go` updated for `parser.Plan` types
- [ ] `cmd/menu_test.go` updated if affected
- [ ] `cmd/approve_test.go` updated if affected
- [ ] All tests pass: `go test ./...`
- [ ] `go vet ./...` passes

## Task Dependency Graph

```
TASK-002-001 ──> TASK-002-002
```

| Task | Estimate | Predecessors | Parallel | Model |
|------|----------|--------------|----------|-------|
| TASK-002-001 | ~40k | none | no | — |
| TASK-002-002 | ~75k | 001 | no | opus |

**Total estimated tokens:** ~115k

## Functional Requirements

- FR-1: `LoadPlans` must return bugs before features (matching current priority order)
- FR-2: `LoadPlans(dir, true)` must include `_completed.md` files with `Completed=true`
- FR-3: `LoadPlans(dir, false)` must exclude `_completed.md` files
- FR-4: `Plan.ID` must strip both `.md` and `_completed` suffixes consistently
- FR-5: `Plan.ApprovalKey()` must match the current approval key behavior in both `status_plans.go` and `work_loop.go`
- FR-6: `LoadPlans` must call `MigrateLegacyBugIDs` for bug files before parsing

## Non-Goals

- Not changing the approval system itself (just how plans are loaded)
- Not modifying `parser.ParseFile()` or `parser.Task` — those stay as-is
- Not changing the work loop's task selection logic
- Not refactoring the delete confirmation dialog

## Technical Considerations

- `featureInfo.approved` field removal means rendering must look up approval from `approval.Load()` — this is arguably better (always fresh state)
- Test churn will be significant in `status_test.go` (1000 lines) and `work_loop_test.go` (1021 lines) since they construct these structs directly
- The `featureIDFromPath` in `approve.go` strips `_completed` but the one in `work_loop.go` doesn't — the unified version must strip it

## Success Metrics

- Zero duplicated file loading structs/functions
- Single `parser.Plan` type used by all consumers
- All existing behavior preserved (especially approval filtering and bug priority)

## Open Questions

*None — all resolved during brainstorming.*
