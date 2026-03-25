<!-- maggus-id: 76f403d3-b95d-4931-9be5-19097228a947 -->
# Feature 001: Shared TUI Navigation Helpers

## Introduction

Extract duplicated cursor navigation logic (up/down with wraparound, home/end, clamp) into reusable pure functions in `internal/tui/styles/`. Currently this logic is copy-pasted 5+ times across `tasklist.go`, `menu.go`, `approve.go`, `gitsync.go`, `repos.go`, and `status.go` with inconsistent behavior (some wrap, some don't).

## Goals

- Eliminate duplicated cursor navigation code across all TUI files
- Make wrap vs. clamp behavior an explicit per-caller choice instead of accidental
- Keep helpers as simple pure functions (no struct/interface overhead)

## Tasks

### TASK-001-001: Create navigation helper functions
**Description:** As a developer, I want shared cursor navigation functions so that cursor movement logic is defined once and reused everywhere.

**Token Estimate:** ~25k tokens
**Predecessors:** none
**Successors:** TASK-001-002
**Parallel:** no

**Acceptance Criteria:**
- [x] New file `src/internal/tui/styles/nav.go` with `CursorUp(cursor, count int) int`, `CursorDown(cursor, count int) int`, `ClampCursor(cursor, count int) int`
- [x] All functions are pure (no side effects, no struct fields)
- [x] `CursorUp`/`CursorDown` implement wraparound behavior
- [x] `ClampCursor` constrains to `[0, count-1]`, returns 0 for empty lists
- [x] New file `src/internal/tui/styles/nav_test.go` with table-driven tests covering: normal movement, wraparound, count=0, count=1, clamp after deletion
- [x] `go test ./internal/tui/styles/` passes
- [x] `go vet ./internal/tui/styles/` passes

### TASK-001-002: Apply navigation helpers to all TUI files
**Description:** As a developer, I want all TUI files to use the shared navigation helpers so that cursor logic is consistent and DRY.

**Token Estimate:** ~50k tokens
**Predecessors:** TASK-001-001
**Successors:** none
**Parallel:** no

**Acceptance Criteria:**
- [ ] `cmd/tasklist.go`: `Cursor`, `criteriaCursor`, `actionCursor` navigation uses `styles.CursorUp`/`CursorDown`/`ClampCursor`
- [ ] `cmd/menu.go`: `cursor` and `subCursor` navigation uses shared helpers
- [ ] `cmd/approve.go`: cursor navigation uses `ClampCursor` (no wraparound)
- [ ] `cmd/gitsync.go`: cursor navigation uses shared helpers
- [ ] `cmd/repos.go`: cursor navigation uses shared helpers
- [ ] `cmd/status.go`: `selectedFeature` left/right navigation uses shared helpers
- [ ] `cmd/config.go`: cursor navigation uses shared helpers where applicable (keep inline logic for tab-bar focus switch at cursor=0)
- [ ] `cmd/prompt_picker.go` is LEFT UNCHANGED (has custom skip-separator logic)
- [ ] All existing tests pass: `go test ./...`
- [ ] Navigation behavior is unchanged in all TUI views (wrap where it wrapped before, clamp where it clamped)

## Task Dependency Graph

```
TASK-001-001 ──> TASK-001-002
```

| Task | Estimate | Predecessors | Parallel | Model |
|------|----------|--------------|----------|-------|
| TASK-001-001 | ~25k | none | no | — |
| TASK-001-002 | ~50k | 001 | no | — |

**Total estimated tokens:** ~75k

## Functional Requirements

- FR-1: `CursorUp(cursor, count)` must return `count-1` when cursor is 0 (wraparound)
- FR-2: `CursorDown(cursor, count)` must return `0` when cursor is `count-1` (wraparound)
- FR-3: `ClampCursor(cursor, count)` must return 0 when count is 0 or cursor is negative
- FR-4: All existing key bindings and navigation behavior must remain identical from the user's perspective

## Non-Goals

- Not unifying `msg.String()` vs `msg.Type` key matching approach (separate concern)
- Not extracting delete confirmation dialog (covered in feature 002/003)
- Not changing any key bindings or adding new shortcuts
- Not touching `prompt_picker.go` skip-separator logic

## Technical Considerations

- The `styles` package already contains layout helpers beyond just styles, so adding nav helpers fits
- Pure functions avoid the complexity of generics or interfaces for different cursor field names
- `config.go` has special up-at-0 behavior (moves focus to tab bar) — that logic stays inline

## Success Metrics

- Zero duplicated cursor up/down/home/end logic in `cmd/` files
- All 8 target files use shared helpers
- Full test suite passes with no behavioral changes

## Open Questions

*None — all resolved during brainstorming.*
