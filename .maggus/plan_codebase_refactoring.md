# Plan: Codebase Refactoring — TUI, List/Status Dedup, Work Loop

## Introduction

The Maggus codebase has grown to ~25k lines with three areas of accumulated technical debt:
1. **`runner/tui.go`** is a 1,656-line god object with 70+ fields managing sync, summary, tokens, task detail, and work progress all in one struct.
2. **`cmd/list.go` and `cmd/status.go`** duplicate ~400 lines of identical task-list browsing logic (cursor, scroll, detail view, criteria mode).
3. **`cmd/work.go`** has a 656-line `RunE` function with 16 internal imports, mixing setup, sync checking, task execution, committing, and summary building inline.

This plan surgically extracts sub-models, deduplicates shared components, and breaks the work loop into named stages — all while keeping existing behavior intact and tests passing after every task.

## Goals

- Reduce `runner/tui.go` from ~1,656 lines to ~600 lines by extracting sync, summary, and token tracking into separate files in the same package
- Eliminate ~400 lines of duplication between `list.go` and `status.go` via a shared task-list component
- Break `work.go`'s RunE into named workflow stages, each independently testable
- Add unit tests for all newly extracted components
- Zero user-visible behavior changes — all existing tests must pass after every task

## User Stories

### TASK-001: Extract sync state and logic from TUIModel into tui_sync.go
**Description:** As a developer, I want the sync resolution screen state and methods extracted into a separate file so that the TUI model is easier to understand and modify.

**Acceptance Criteria:**
- [x] New file `src/internal/runner/tui_sync.go` contains all sync-related types, fields struct, and methods
- [x] Create a `syncState` struct holding: `active`, `behind`, `ahead`, `remoteBranch`, `resultCh`, `options`, `cursor`, `confirmForce`, `running`, `errorMsg`, `dir` fields
- [x] `TUIModel` embeds or holds a `syncState` field instead of 12 individual sync fields
- [x] `handleSyncKeys()`, `selectSyncOption()`, `buildSyncOptions()`, `renderSyncView()`, `runSyncPull()`, `runSyncPullRebase()`, `runSyncForcePull()` move to `tui_sync.go`
- [x] `syncMenuOption` type, `SyncCheckMsg`/`SyncCheckResult`/`SyncAction`/`syncActionDoneMsg` types move to `tui_sync.go`
- [x] `InitSyncFuncs()` and the package-level sync function vars move to `tui_sync.go`
- [x] All sync-related message handling in `Update()` delegates to a `handleSyncMsg()` method on syncState
- [x] `go test ./internal/runner` passes
- [x] `go build ./...` succeeds
- [x] `go vet ./...` reports no issues

### TASK-002: Extract summary state and rendering from TUIModel into tui_summary.go
**Description:** As a developer, I want the summary/post-run screen state and rendering extracted into a separate file so that adding summary features doesn't require touching the main TUI model.

**Acceptance Criteria:**
- [x] New file `src/internal/runner/tui_summary.go` contains all summary-related state and methods
- [x] Create a `summaryState` struct holding: `show`, `data`, `elapsed`, `pushStatus`, `pushDone`, `menuChoice`, `editingCount`, `countInput`, `runAgain` fields
- [x] `TUIModel` holds a `summaryState` field instead of 9 individual summary fields
- [x] `handleSummaryKeys()`, `renderSummaryView()`, `renderSummaryMenu()` move to `tui_summary.go`
- [x] `SummaryData`, `SummaryMsg`, `PushStatusMsg`, `QuitMsg`, `RunAgainResult`, `StopReason`, `RemainingTask`, `FailedTask` types move to `tui_summary.go`
- [x] Summary-related message handling in `Update()` delegates to methods on summaryState
- [x] `go test ./internal/runner` passes
- [x] `go build ./...` succeeds
- [x] `go vet ./...` reports no issues

### TASK-003: Extract token tracking from TUIModel into tui_tokens.go
**Description:** As a developer, I want token usage tracking isolated so that changes to usage reporting don't affect the main TUI model.

**Acceptance Criteria:**
- [x] New file `src/internal/runner/tui_tokens.go` contains token tracking state and methods
- [x] Create a `tokenState` struct holding: `iterInput`, `iterOutput`, `totalInput`, `totalOutput`, `hasData`, `usages []TaskUsage`, `onUsage func(TaskUsage)` fields
- [x] `TUIModel` holds a `tokenState` field instead of 7 individual token fields
- [x] `saveIterationUsage()`, `FormatTokens()`, `TaskUsage` type, `SetOnTaskUsage()`, `TaskUsages()` move to `tui_tokens.go`
- [x] `agent.UsageMsg` handling in `Update()` delegates to `tokenState.addUsage()` method
- [x] `IterationStartMsg` handler calls `tokenState.saveAndReset()` to finalize previous iteration
- [x] `go test ./internal/runner` passes
- [x] `go build ./...` succeeds
- [x] `go vet ./...` reports no issues

### TASK-004: Write unit tests for extracted TUI sub-components
**Description:** As a developer, I want unit tests for the sync, summary, and token sub-components so that they can be modified with confidence.

**Acceptance Criteria:**
- [x] New file `src/internal/runner/tui_sync_test.go` with tests for: `buildSyncOptions` populates correct menu items, `handleSyncKeys` navigation (up/down changes cursor), sync abort sends correct result on channel
- [x] New file `src/internal/runner/tui_summary_test.go` with tests for: `handleSummaryKeys` menu navigation, editing count input accepts digits and rejects letters, Enter on Exit triggers quit
- [x] New file `src/internal/runner/tui_tokens_test.go` with tests for: `addUsage` accumulates totals, `saveAndReset` creates TaskUsage record and resets iteration counters, `FormatTokens` formatting (already tested if exists, add if not)
- [x] All new tests pass: `go test ./internal/runner -v`
- [x] Existing tests still pass: `go test ./...`

### TASK-005: Clean up TUIModel.Update() — delegate to sub-component handlers
**Description:** As a developer, I want the main `Update()` method to be a thin dispatcher that delegates to sub-components, reducing it from ~260 lines to ~80 lines.

**Acceptance Criteria:**
- [x] `Update()` method in `tui.go` is reduced to a message router: check sync → check summary/done → handle key events → delegate message types to sub-components
- [x] No business logic remains inline in `Update()` switch cases — each case is a one-liner delegation
- [x] `tui.go` is under 700 lines (down from 1,656)
- [x] `go test ./internal/runner` passes
- [x] `go build ./...` succeeds
- [x] `go vet ./...` reports no issues

### TASK-006: Extract shared TaskListComponent for list.go and status.go
**Description:** As a developer, I want a reusable task-list browsing component that both `list` and `status` commands can embed, eliminating duplicated cursor/scroll/detail/criteria logic.

**Acceptance Criteria:**
- [x] New file `src/cmd/tasklist.go` containing a `taskListComponent` struct
- [x] `taskListComponent` handles: cursor movement (up/down/home/end with wrapping), scroll offset management, `visibleTaskLines()` calculation, `ensureCursorVisible()`, detail viewport open/close, detail rendering via `detailState`, criteria mode delegation, action picker delegation, delete confirmation flow
- [x] The component accepts a configuration struct or parameters for: header line count (differs between list and status), task slice, width/height, agent name
- [x] `taskListComponent` has `Update(tea.KeyMsg)` and `View()` methods that the parent models call
- [x] `go build ./...` succeeds
- [x] `go vet ./...` reports no issues

### TASK-007: Refactor list.go to use TaskListComponent
**Description:** As a developer, I want `list.go` to use the shared `taskListComponent` so that list-specific code only contains list-specific behavior.

**Acceptance Criteria:**
- [x] `listModel` embeds `taskListComponent` instead of duplicating cursor, scroll, detail, criteria fields
- [x] `listModel.Update()` delegates key handling to `taskListComponent.Update()` for shared behavior, only handling list-specific keys (like `alt+r` run) itself
- [x] `listModel.View()` uses `taskListComponent` for task list rendering
- [x] Removed methods from `listModel`: `visibleTaskLines()`, `ensureCursorVisible()`, `updateDetail()`, `updateCriteriaMode()`, `updateActionPicker()`, `updateConfirmDelete()`, `refreshDetailViewport()`
- [x] `list.go` is under 250 lines (down from 609)
- [x] `go test ./cmd -run TestList` passes
- [x] `go test ./...` passes
- [x] Behavior is identical: cursor wrapping, scroll, detail view, criteria mode, delete confirmation all work as before

### TASK-008: Refactor status.go to use TaskListComponent
**Description:** As a developer, I want `status.go` to use the shared `taskListComponent` so that status-specific code only contains plan browsing and status-specific behavior.

**Acceptance Criteria:**
- [x] `statusModel` embeds `taskListComponent` instead of duplicating cursor, scroll, detail, criteria fields
- [x] `statusModel.Update()` delegates shared key handling to `taskListComponent.Update()`, only handling status-specific keys (tab/shift+tab plan switching, alt+a toggle, alt+i ignore) itself
- [x] `statusModel.View()` uses `taskListComponent` for task list rendering within the plan context
- [x] Removed methods from `statusModel`: `visibleTaskLines()`, `ensureCursorVisible()`, duplicated detail/criteria/action methods
- [x] `status.go` is under 700 lines (down from 1,136)
- [x] `go test ./cmd -run TestStatus` passes
- [x] `go test ./...` passes
- [x] Behavior is identical: plan tab switching, task browsing, detail view, ignore toggle all work as before

### TASK-009: Write unit tests for TaskListComponent
**Description:** As a developer, I want the shared task-list component to have its own tests so that changes don't accidentally break both list and status commands.

**Acceptance Criteria:**
- [ ] New file `src/cmd/tasklist_test.go` with tests for:
  - Cursor movement wraps at boundaries (top→bottom, bottom→top)
  - `ensureCursorVisible()` adjusts scroll offset correctly when cursor moves out of view
  - `visibleTaskLines()` returns correct count based on height and header lines
  - Opening detail view sets showDetail and creates viewport
  - Closing detail view resets state
  - Criteria mode enter/exit lifecycle
- [ ] All new tests pass: `go test ./cmd -run TestTaskList -v`
- [ ] Existing tests still pass: `go test ./...`

### TASK-010: Extract work loop setup into a workSetup function
**Description:** As a developer, I want the work loop's initialization code (config loading, agent resolution, model resolution, gitignore, fingerprint, worktree mode) extracted into a named function so that RunE focuses on orchestration.

**Acceptance Criteria:**
- [ ] New function `workSetup(cmd, args) (*workConfig, error)` in `work.go` or a new `work_setup.go` file
- [ ] `workConfig` struct holds all resolved configuration: count, dir, cfg, validIncludes, includeWarnings, activeAgent, resolvedModel, modelDisplay, notifier, useWorktree
- [ ] RunE's first ~60 lines (lines 64-160 in current code) are replaced by a single `workSetup()` call
- [ ] `go test ./cmd -run TestWork` passes
- [ ] `go build ./...` succeeds
- [ ] `go vet ./...` reports no issues

### TASK-011: Extract git sync check into a checkSync function
**Description:** As a developer, I want the pre-work git sync check logic extracted so it's independently testable and doesn't clutter RunE.

**Acceptance Criteria:**
- [ ] New function `checkSync(dir string) (syncInfoMsg string, shouldAbort bool, err error)` in `work.go` or `work_sync.go`
- [ ] Function encapsulates: `gitsync.FetchRemote()`, `gitsync.RemoteStatus()`, `gitsync.WorkingTreeStatus()`, the interactive sync TUI call, and the result interpretation
- [ ] RunE's sync check block (lines 162-191 in current code) is replaced by a single `checkSync()` call
- [ ] `go test ./cmd -run TestWork` passes
- [ ] `go build ./...` succeeds
- [ ] `go vet ./...` reports no issues

### TASK-012: Extract task execution into a runTask function
**Description:** As a developer, I want the per-task execution logic (find next task, acquire lock, build prompt, invoke agent, parse results) extracted so the work loop body is concise.

**Acceptance Criteria:**
- [ ] New function or method that encapsulates the per-iteration body of the work loop (lines 410-590 in current code)
- [ ] Handles: finding next task, acquiring lock, sending IterationStartMsg, building prompt, running agent, re-parsing plans, marking completed plans, staging plan renames
- [ ] Returns a result struct indicating: success/failure, whether to continue/break, any warnings
- [ ] The main loop in RunE calls this function and handles the result
- [ ] `go test ./cmd -run TestWork` passes
- [ ] `go build ./...` succeeds
- [ ] `go vet ./...` reports no issues

### TASK-013: Extract commit and post-task logic into a completeTask function
**Description:** As a developer, I want the commit handling and between-task sync check extracted from the work loop body.

**Acceptance Criteria:**
- [ ] New function that encapsulates: `gitcommit.CommitIteration()`, commit result handling, lock release, progress update, between-task sync check
- [ ] The main loop calls `completeTask()` after successful agent execution
- [ ] Error paths (commit failure, sync abort) return appropriate signals to the caller
- [ ] `go test ./cmd -run TestWork` passes
- [ ] `go build ./...` succeeds
- [ ] `go vet ./...` reports no issues

### TASK-014: Write unit tests for extracted work loop stages
**Description:** As a developer, I want unit tests for `workSetup`, `checkSync`, and the task execution helpers so that orchestration changes are safe.

**Acceptance Criteria:**
- [ ] Tests for `workSetup`: verifies config resolution (model flag overrides config, agent flag overrides config, worktree flag precedence)
- [ ] Tests for `checkSync`: verifies correct syncInfoMsg for various scenarios (no remote, behind remote, fetch failure, up to date)
- [ ] Tests use test helpers or interfaces to avoid real git/filesystem operations where possible
- [ ] All new tests pass: `go test ./cmd -v`
- [ ] Existing tests still pass: `go test ./...`

### TASK-015: Final cleanup — verify line counts and run full test suite
**Description:** As a developer, I want to verify that the refactoring achieved its goals and that the entire test suite passes.

**Acceptance Criteria:**
- [ ] `tui.go` is under 700 lines
- [ ] `list.go` is under 250 lines
- [ ] `status.go` is under 700 lines
- [ ] `work.go` RunE function is under 200 lines
- [ ] `go test ./...` passes with zero failures
- [ ] `go build ./...` succeeds
- [ ] `go vet ./...` reports no issues
- [ ] `go fmt ./...` reports no changes needed
- [ ] No new lint warnings introduced

## Functional Requirements

- FR-1: All extracted files must stay in their current packages (Option A — no new import paths)
- FR-2: The `TUIModel` public API must not change — `NewTUIModel()`, `SetSyncDir()`, `SetOnTaskUsage()`, `StopFlag()`, `Result()`, `TaskUsages()` signatures remain identical
- FR-3: All existing bubbletea message types (`SyncCheckMsg`, `SummaryMsg`, `IterationStartMsg`, etc.) must remain exported from the `runner` package
- FR-4: The work command's CLI flags and behavior must be completely unchanged
- FR-5: The list and status commands must produce identical TUI behavior (key bindings, visual output, navigation)
- FR-6: Each task must leave the codebase in a compilable, test-passing state

## Non-Goals

- No new packages or package reorganization — files stay in their current packages
- No changes to the CLI interface, flags, or user-facing behavior
- No refactoring of well-structured packages (parser, config, prompt, gitsync, etc.)
- No performance optimization — this is purely structural
- No changes to the build system, CI, or release process
- No UI/UX changes to the TUI rendering

## Technical Considerations

- Bubbletea models are value types — sub-components that are held as struct fields work naturally with bubbletea's `Update()` return pattern as long as the parent updates its copy
- The sync component communicates with the work goroutine via a channel (`syncResultCh`) — this channel must remain accessible from the parent model's message handling
- `renderSyncView()` and `renderSummaryView()` call `renderHeaderInner()` which is a method on `TUIModel` — the sub-components need access to the parent for header rendering, so they should accept a header string parameter or the parent should compose the full view
- The `detailState` in `cmd/detail.go` is already a good pattern to follow for the shared task-list component
- Some methods like `handleSummaryKeys()` currently operate on `TUIModel` value receiver — when moving to sub-components, ensure the pointer/value receiver semantics are preserved

## Success Metrics

- `tui.go` reduced from 1,656 to under 700 lines
- `list.go` reduced from 609 to under 250 lines
- `status.go` reduced from 1,136 to under 700 lines
- `work.go` RunE reduced from 656 to under 200 lines
- Zero test regressions
- All new extracted components have unit tests
- Total line count stays roughly the same (code is reorganized, not deleted)

## Open Questions

- Should `renderHeaderInner()` be extracted to its own file (`tui_header.go`) since both the sync view and summary view call it? Could be done as a follow-up.
- The `renderView()` (main work view) and `renderBannerView()` could also be candidates for extraction — defer to a future plan if the main tui.go is still too large after this refactoring.
