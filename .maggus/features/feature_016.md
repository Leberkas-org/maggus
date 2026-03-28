<!-- maggus-id: 642154f2-f23c-4b15-84f3-fb13b328cc18 -->
# Feature 016: Dedicated Plan Stores for Features and Bugs

## Introduction

Extract all file-based plan I/O from `internal/parser` into two dedicated store types — `FeatureStore` and `BugStore` — each backed by a Go interface, a concrete file-based implementation, and an in-memory fake for tests. Migrate all callers in `cmd/` and `internal/runner/` to use the stores.

This resolves two structural problems:
- `parser.go` is 729 lines — well over the 500-line file limit defined in CLAUDE.md.
- File I/O is scattered across standalone functions with no central abstraction, making it hard to test callers in isolation.

### Architecture Context

- **Components involved:** `internal/parser` (source of all current logic), `cmd/` (primary callers), `internal/runner/tui_messages.go` (secondary caller)
- **New package:** `internal/stores` — owns the store interfaces and implementations
- **Pattern introduced:** Interface + concrete file-based impl + in-memory fake (standard Go testability pattern)
- **Parser package:** Retains pure parsing logic (markdown → structs). Mutation functions (`DeleteTask`, `UnblockCriterion`, etc.) move into the stores. `LoadPlans`, `ParseFeatures`, `ParseBugs`, `MarkCompleted*`, `Glob*` also move.

## Goals

- Introduce `FeatureStore` and `BugStore` interfaces in `internal/stores`
- Provide `FileFeatureStore` and `FileBugStore` as the concrete file-backed implementations
- Provide `MemFeatureStore` and `MemBugStore` as in-memory fakes for use in tests
- Migrate all 9 call sites across `cmd/` and `internal/runner/` to use the stores
- Leave `internal/parser` with only pure parsing logic (no file I/O beyond `ParseFile` and `ParseMaggusID`)

## Tasks

### TASK-016-001: Define FeatureStore and BugStore interfaces + package skeleton
**Description:** As a developer, I want clearly defined Go interfaces for both stores so that callers depend on abstractions and fakes can be substituted in tests.

**Token Estimate:** ~25k tokens
**Predecessors:** none
**Successors:** TASK-016-002, TASK-016-003, TASK-016-004
**Parallel:** no — all other tasks depend on this

**Acceptance Criteria:**
- [x] `internal/stores/` package created with a `doc.go` or `stores.go` file
- [x] `FeatureStore` interface defined with methods:
  - `LoadAll(includeCompleted bool) ([]parser.Plan, error)`
  - `MarkCompleted(action string) ([]string, error)` — renames/deletes completed feature files
  - `GlobFiles(includeCompleted bool) ([]string, error)`
  - `DeleteTask(filePath, taskID string) error`
  - `UnblockCriterion(filePath string, c parser.Criterion) error`
  - `ResolveCriterion(filePath string, c parser.Criterion) error`
  - `DeleteCriterion(filePath string, c parser.Criterion) error`
- [x] `BugStore` interface defined with the same method set (identical shape, separate type)
- [x] Both interfaces are exported and documented with a one-line comment each
- [x] Package compiles (`go build ./internal/stores`)

---

### TASK-016-002: Implement FileFeatureStore + tests
**Description:** As a developer, I want a concrete file-backed implementation of `FeatureStore` so that the production code path works correctly against real `.maggus/features/` files.

**Token Estimate:** ~60k tokens
**Predecessors:** TASK-016-001
**Successors:** TASK-016-005
**Parallel:** yes — can run alongside TASK-016-003 and TASK-016-004

**Acceptance Criteria:**
- [x] `FileFeatureStore` struct created in `internal/stores/feature_store.go`
- [x] Struct holds `dir string` (the project root, passed at construction via `NewFileFeatureStore(dir string) *FileFeatureStore`)
- [x] All `FeatureStore` interface methods implemented by delegating to existing `parser` functions
- [x] `LoadAll` delegates to `parser.GlobFeatureFiles` + `parser.ParseFile` (mirroring `parser.LoadPlans` but features only)
- [x] `MarkCompleted` delegates to `parser.MarkCompletedFeatures`
- [x] Mutation methods (`DeleteTask`, `UnblockCriterion`, `ResolveCriterion`, `DeleteCriterion`) delegate to the corresponding `parser.*` functions
- [x] Unit tests in `feature_store_test.go` use a temp directory with fixture `.md` files
- [x] Tests cover: `LoadAll`, `MarkCompleted` (rename), `DeleteTask`, `UnblockCriterion`, `ResolveCriterion`, `DeleteCriterion`
- [x] `go test ./internal/stores` passes

---

### TASK-016-003: Implement FileBugStore + tests
**Description:** As a developer, I want a concrete file-backed implementation of `BugStore` so that bug file operations are handled through the same abstraction as features.

**Token Estimate:** ~55k tokens
**Predecessors:** TASK-016-001
**Successors:** TASK-016-005
**Parallel:** yes — can run alongside TASK-016-002 and TASK-016-004

**Acceptance Criteria:**
- [ ] `FileBugStore` struct created in `internal/stores/bug_store.go`
- [ ] Struct holds `dir string`, constructed via `NewFileBugStore(dir string) *FileBugStore`
- [ ] All `BugStore` interface methods implemented by delegating to existing `parser` functions
- [ ] `LoadAll` delegates to `parser.GlobBugFiles` + `parser.MigrateLegacyBugIDs` + `parser.ParseFile`
- [ ] `MarkCompleted` delegates to `parser.MarkCompletedBugs`
- [ ] Mutation methods delegate to corresponding `parser.*` functions
- [ ] Unit tests in `bug_store_test.go` use a temp directory with fixture `.md` files
- [ ] Tests cover the same scenarios as TASK-016-002 plus legacy ID migration
- [ ] `go test ./internal/stores` passes

---

### TASK-016-004: Implement in-memory fakes (MemFeatureStore, MemBugStore) + tests
**Description:** As a developer writing tests for callers, I want in-memory store fakes so that I can test `cmd/` logic without touching the filesystem.

**Token Estimate:** ~45k tokens
**Predecessors:** TASK-016-001
**Successors:** TASK-016-005
**Parallel:** yes — can run alongside TASK-016-002 and TASK-016-003

**Acceptance Criteria:**
- [ ] `MemFeatureStore` and `MemBugStore` structs created in `internal/stores/mem_store.go`
- [ ] Both satisfy their respective interfaces (verified by compile-time assertion: `var _ FeatureStore = &MemFeatureStore{}`)
- [ ] `MemFeatureStore` is seeded with `[]parser.Plan` at construction: `NewMemFeatureStore(plans []parser.Plan) *MemFeatureStore`
- [ ] `LoadAll` returns the seeded plans filtered by `includeCompleted`
- [ ] `MarkCompleted` marks matching plans as completed in memory (sets `Completed: true`), returns their IDs
- [ ] Mutation methods (`DeleteTask`, `UnblockCriterion`, etc.) mutate the in-memory plan data
- [ ] Same constructor + interface pattern for `MemBugStore`
- [ ] Tests in `mem_store_test.go` verify each method produces the correct in-memory state
- [ ] `go test ./internal/stores` passes

---

### TASK-016-005: Migrate all callers to use the stores
**Description:** As a developer, I want all `cmd/` and `internal/runner/` code to use the new store types so that no call site directly calls `parser.*` file I/O functions.

**Token Estimate:** ~75k tokens
**Predecessors:** TASK-016-002, TASK-016-003
**Successors:** none
**Parallel:** no — depends on both store implementations

**Acceptance Criteria:**
- [ ] All 9 call sites migrated (listed below)
- [ ] `cmd/approve.go` — replace `parser.LoadPlans` with store calls
- [ ] `cmd/detail.go` — replace `parser.UnblockCriterion`, `ResolveCriterion`, `DeleteCriterion`
- [ ] `cmd/menu_model.go` — replace `parser.LoadPlans`
- [ ] `cmd/status_model.go` — replace `parser.LoadPlans`
- [ ] `cmd/status_update.go` — replace `parser.DeleteTask`
- [ ] `cmd/tasklist.go` — replace `parser.DeleteTask`
- [ ] `cmd/work_loop.go` — replace `parser.ParseBugs`, `ParseFeatures`, `MarkCompletedFeatures`, `MarkCompletedBugs`, `LoadPlans`
- [ ] `cmd/work_task.go` — replace `parser.MarkCompletedFeatures`, `MarkCompletedBugs`, `GlobFeatureFiles`, `GlobBugFiles`
- [ ] `internal/runner/tui_messages.go` — replace `parser.ParseBugs`, `ParseFeatures`
- [ ] Store instances are constructed once and passed down (not re-created per call)
- [ ] No direct `parser.*` file I/O calls remain outside `internal/parser` and `internal/stores`
- [ ] `go build ./...` passes
- [ ] `go test ./...` passes

## Task Dependency Graph

```
TASK-016-001 ──→ TASK-016-002 ──→ TASK-016-005
             ──→ TASK-016-003 ──┘
             ──→ TASK-016-004
```

| Task | Estimate | Predecessors | Parallel | Model |
|------|----------|--------------|----------|-------|
| TASK-016-001 | ~25k | none | no | — |
| TASK-016-002 | ~60k | 001 | yes (with 003, 004) | — |
| TASK-016-003 | ~55k | 001 | yes (with 002, 004) | — |
| TASK-016-004 | ~45k | 001 | yes (with 002, 003) | — |
| TASK-016-005 | ~75k | 002, 003 | no | sonnet |

**Total estimated tokens:** ~260k

## Functional Requirements

- FR-1: `FeatureStore` and `BugStore` must be defined as Go interfaces in `internal/stores`
- FR-2: Both stores must have concrete file-backed implementations that delegate to existing `parser` functions (no logic duplication)
- FR-3: Both stores must have in-memory fakes that satisfy their interfaces via compile-time assertion
- FR-4: In-memory fakes must be seeded at construction — no global state
- FR-5: All callers in `cmd/` and `internal/runner/` must use the stores exclusively for file I/O
- FR-6: Store instances must be constructed once and injected (not re-created per function call)
- FR-7: The `internal/parser` package retains only pure parsing logic after this refactor
- FR-8: All existing tests must continue to pass after the migration

## Non-Goals

- Caching or watch-for-changes behavior (no inotify / fsnotify)
- Combining `FeatureStore` and `BugStore` into a single `PlanStore` — keep them separate
- Removing the existing `parser.*` functions (they stay; stores delegate to them)
- Changing the markdown file format
- Adding new store methods not needed by existing callers

## Technical Considerations

- File-backed stores delegate to existing `parser` functions — no logic is duplicated or rewritten
- `filePath` arguments in mutation methods should remain as-is for now (tasks carry `SourceFile`); a future refactor can introduce plan-ID–based lookup
- Store instances should be constructed in the command's `Run` or `runE` function and passed to helpers — avoid threading `dir string` through every call
- `parser.go` will still exist after this change — the store package is an additional layer, not a replacement of the parser package itself
- After TASK-016-005, `parser.go` can optionally be split further (separate concern from this feature)

## Success Metrics

- `parser.go` has no more direct callers from outside `internal/parser` and `internal/stores` for file I/O functions
- New `internal/stores` tests cover all interface methods with no real filesystem dependency (in-memory fakes)
- `go build ./...` and `go test ./...` pass on CI without changes to the test suite beyond the new store tests

## Open Questions

*(none)*
