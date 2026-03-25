<!-- maggus-id: 380c3bb4-1e64-4cf5-a62a-fc94d4dcd5b7 -->
# Feature 003: Split Large Files

## Introduction

After features 001 and 002 reduce duplication, split the remaining large files by responsibility. `status.go` (~1100 lines after feature 002) and `menu.go` (~950 lines) each contain model definition, update logic, view rendering, and command setup in a single file. Splitting by the standard Bubble Tea pattern (model/update/view) makes each file focused and navigable.

## Goals

- No single file exceeds ~500 lines
- Each file has a clear, single responsibility following the Bubble Tea model/update/view pattern
- Maintain all existing behavior — pure file reorganization

## Tasks

### TASK-003-001: Split status.go into focused files
**Description:** As a developer, I want `status.go` split by responsibility so that each file is focused and navigable.

**Token Estimate:** ~40k tokens
**Predecessors:** none
**Successors:** TASK-003-003
**Parallel:** yes — can run alongside TASK-003-002

**Acceptance Criteria:**
- [ ] `cmd/status_model.go`: `statusModel` struct, `newStatusModel()`, `visibleFeatures()`, `rebuildForSelectedFeature()`, `reloadFeatures()`, `syncDetailSuffix()` (~100 lines)
- [ ] `cmd/status_update.go`: `Init()`, `Update()`, `updateList()`, `updateStatusDetail()`, confirm handlers, `applyLogLines()`, scroll helpers (~250 lines)
- [ ] `cmd/status_view.go`: `View()`, `viewStatus()`, `viewLog()`, `viewEmpty()`, render helpers, tab bar, daemon status line (~500 lines)
- [ ] `cmd/status_cmd.go`: cobra command definition, `init()`, `renderStatusPlain()` (~100 lines)
- [ ] `status_plans.go` content merged into `status_model.go` (will be tiny after feature 002)
- [ ] Original `status.go` is deleted
- [ ] `go build ./cmd/...` succeeds
- [ ] `go test ./cmd/ -run TestStatus` passes
- [ ] `go vet ./...` passes

### TASK-003-002: Split menu.go into focused files
**Description:** As a developer, I want `menu.go` split by responsibility so that each file is focused and navigable.

**Token Estimate:** ~40k tokens
**Predecessors:** none
**Successors:** TASK-003-003
**Parallel:** yes — can run alongside TASK-003-001

**Acceptance Criteria:**
- [ ] `cmd/menu_model.go`: `menuModel` struct, `menuItem`, sub-menu types, `allMenuItems`, `activeMenuItems()`, `loadFeatureSummary()` (~200 lines)
- [ ] `cmd/menu_update.go`: `Init()`, `Update()`, `updateMainMenu()`, `updateSubMenu()`, `updateConfirmStopDaemon()`, message types (~300 lines)
- [ ] `cmd/menu_view.go`: `View()`, `viewMainMenu()`, `viewSubMenu()`, render helpers (~350 lines)
- [ ] `cmd/menu_cmd.go`: cobra command, `init()`, `isInitialized()` (~50 lines)
- [ ] Original `menu.go` is deleted
- [ ] `go build ./cmd/...` succeeds
- [ ] `go test ./cmd/ -run TestMenu` passes
- [ ] `go vet ./...` passes

### TASK-003-003: Final verification and cleanup
**Description:** As a developer, I want to verify the full test suite passes and no regressions were introduced across all three features.

**Token Estimate:** ~15k tokens
**Predecessors:** TASK-003-001, TASK-003-002
**Successors:** none
**Parallel:** no

**Acceptance Criteria:**
- [ ] `go test ./...` passes with zero failures
- [ ] `go vet ./...` passes
- [ ] `go build -o maggus .` produces a working binary
- [ ] No file in `cmd/` exceeds 600 lines (excluding test files)
- [ ] Run `maggus status` — TUI renders correctly with feature navigation
- [ ] Run `maggus menu` — TUI renders correctly with item selection

## Task Dependency Graph

```
TASK-003-001 ──> TASK-003-003
TASK-003-002 ──/
```

| Task | Estimate | Predecessors | Parallel | Model |
|------|----------|--------------|----------|-------|
| TASK-003-001 | ~40k | none | yes (with 002) | — |
| TASK-003-002 | ~40k | none | yes (with 001) | — |
| TASK-003-003 | ~15k | 001, 002 | no | — |

**Total estimated tokens:** ~95k

## Functional Requirements

- FR-1: All files remain in `package cmd` — no import changes needed
- FR-2: No behavioral changes — this is purely file organization
- FR-3: All existing tests must pass without modification (they test exported behavior, not file layout)
- FR-4: `status_plans.go` must be removed (merged into `status_model.go`)

## Non-Goals

- Not splitting `tui_render.go` (938 lines) — it's already part of the runner package split pattern
- Not renaming any functions or types
- Not changing any logic

## Technical Considerations

- Git blame will lose per-line history for moved code — document the split in commit messages
- Do this as a fast, isolated effort to minimize merge conflict risk with concurrent work
- All files stay in `package cmd` so the Go compiler treats them as one unit — the split is purely for human navigability

## Success Metrics

- `status.go` eliminated, replaced by 4 focused files each under 500 lines
- `menu.go` eliminated, replaced by 4 focused files each under 350 lines
- Full test suite green

## Open Questions

*None — all resolved during brainstorming.*
