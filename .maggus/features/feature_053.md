<!-- maggus-id: e978e24f-ba58-4c30-bf53-3997321939d8 -->
# Feature 053: Consistent Tab Navigation in Status TUI

## Introduction

The status TUI uses a single `activeTab int` field to track which right-pane tab is selected. Tab sets differ between selection contexts (feature row vs. task row), and `updateTabsForSelectionChange` resets `activeTab = 0` whenever the context changes. This means pressing [3] to select a tab, then navigating to a task of a different type (e.g., completed → running, or feature → task), always drops the user back to tab [1]. The experience is inconsistent: the user expects their selected tab to "stick" as they move through the tree.

The fix is two independent tab position trackers — one for feature rows, one for task rows (running and completed share one tracker). Each tracker persists independently so navigating between contexts no longer resets the other tracker's position.

A cosmetic companion fix: the "Task Details" tab is renamed to "Details" for consistency with the "Details" tab on feature rows.

### Architecture Context

- **Vision alignment:** Improves the developer experience of the TUI feedback loop — a core value in the vision's TUI & Feedback section
- **Components involved:** `cmd/status_model.go` (model fields + tab helpers), `cmd/status_update.go` (number key handler, cursor navigation), `cmd/status_rightpane.go` (tab bar render), `cmd/status_test.go`, `cmd/status_rightpane_test.go`
- **No new components or patterns** — this is a focused change to existing tab selection logic

## Goals

- Navigating between tasks preserves the task tab position
- Navigating between feature rows preserves the feature tab position
- Navigating from a feature to a task (or back) does not affect either tracker
- "Task Details" tab is renamed to "Details" for display consistency
- No behavioral changes beyond tab position persistence

## Tasks

### TASK-053-001: Split activeTab into two independent trackers
**Description:** As a user, I want my selected tab to stay selected when I navigate between tasks or between features, so I don't have to re-select my preferred view after each cursor move.

**Token Estimate:** ~60k tokens
**Predecessors:** none
**Successors:** TASK-053-002
**Parallel:** no

**Implementation details:**

1. **Remove `activeTab int` from `statusModel`**. Add two replacement fields:
   ```go
   activeTabFeature int // remembers selected tab when a feature row is selected
   activeTabTask    int // remembers selected tab when a task row is selected (running or completed)
   ```
   Initialize both to `0` in `newStatusModel`.

2. **Add `activeTabIndex() int`** — returns the tracker for the current selection context:
   - `selFeature` → `m.activeTabFeature`
   - `selRunningTask`, `selCompletedTask` → `m.activeTabTask`
   - `selNone` → `0` (only one tab; no tracker needed)

3. **Add `setActiveTabIndex(idx int)`** — sets the tracker for the current selection context (same context mapping as above; `selNone` is a no-op).

4. **Update `activeTabKey()`** — replace `m.activeTab` with `m.activeTabIndex()`.

5. **Update `clampActiveTab()`** — replace direct `m.activeTab` reads/writes with calls to `activeTabIndex()` and `setActiveTabIndex()`.

6. **Update `updateTabsForSelectionChange()`** — remove the `m.activeTab = 0` reset. Each tracker persists independently, so crossing a context boundary no longer resets anything. The function body becomes just:
   ```go
   m.clampActiveTab()
   if m.selectionCtx() == selCompletedTask {
       m.ensureCompletedTaskOutput()
   }
   ```
   Remove the `prevCtx selectionContext` parameter only if it is no longer used anywhere — if callers still pass it, keep the signature and ignore the parameter internally, or remove the parameter and update all callers. Prefer removing it to keep the API clean.

7. **Update number key handler** in `status_update.go` (around line 567):
   Replace `m.activeTab = idx` with `m.setActiveTabIndex(idx)`.

8. **Update `status_rightpane.go`** — replace the two direct `m.activeTab` reads (lines 30 and 57) with `m.activeTabIndex()`.

9. **Update tests** in `status_test.go` and `status_rightpane_test.go`:
   - Replace all `m.activeTab = X` struct-literal initializations and direct assignments with `m.activeTabFeature = X` or `m.activeTabTask = X` depending on which context the test is exercising.
   - Replace all `got.activeTab`, `m.activeTab`, `result.activeTab` reads with `got.activeTabFeature`/`got.activeTabTask` (whichever applies to the test's context).
   - Update the test at `status_test.go` around line 2316 that asserts `activeTab == 0` on context change — this test verified the old reset behavior, which is now removed. Replace it with a test that verifies each tracker is independent: navigating from a task (activeTabTask=2) to a feature (activeTabFeature=1) leaves activeTabTask unchanged at 2 and uses activeTabFeature=1.
   - Update the test `"activeTab is 0 by default"` (line 1017) to verify `activeTabFeature == 0 && activeTabTask == 0`.

**Acceptance Criteria:**
- [ ] `activeTab int` field removed from `statusModel`
- [ ] `activeTabFeature int` and `activeTabTask int` fields added, both initialized to 0
- [ ] `activeTabIndex()` and `setActiveTabIndex()` methods added
- [ ] `activeTabKey()`, `clampActiveTab()`, `updateTabsForSelectionChange()`, number key handler, and rightpane render all updated to use the new methods
- [ ] Navigating between two tasks preserves the task tab position
- [ ] Navigating between two feature rows preserves the feature tab position
- [ ] Navigating from a task to a feature (or vice versa) does not affect the other tracker's position
- [ ] `go build ./...` succeeds
- [ ] `go test ./cmd/...` passes

### TASK-053-002: Rename "Task Details" tab display name to "Details"
**Description:** As a user, I want the task tab labeled "Details" (matching the feature row's "Details" tab) so the naming is consistent across contexts.

**Token Estimate:** ~15k tokens
**Predecessors:** TASK-053-001
**Successors:** none
**Parallel:** no
**Model:** haiku

**Implementation details:**

1. In `availableTabs()` in `status_model.go`: change both `name: "Task Details"` entries to `name: "Details"`. The key `"taskdetails"` stays unchanged — only the display name changes.

2. Update the comment block above `availableTabs()` (lines 318–319) and the `renderCurrentTaskTab` comment in `status_rightpane.go` (line 275) that reference "Task Details".

3. Update `status_rightpane.go` line 17 comment: `Task Details` → `Details`.

4. Update test strings in `status_test.go` and `status_rightpane_test.go` that assert the tab name `"Task Details"` → `"Details"`.

**Acceptance Criteria:**
- [ ] `availableTabs()` returns `name: "Details"` for the taskdetails tab in both selRunningTask and selCompletedTask contexts
- [ ] Tab key `"taskdetails"` is unchanged
- [ ] All comments updated to say "Details" instead of "Task Details"
- [ ] All test assertions updated to match new display name
- [ ] `go build ./...` succeeds
- [ ] `go test ./cmd/...` passes

## Task Dependency Graph

```
TASK-053-001 ──→ TASK-053-002
```

| Task | Estimate | Predecessors | Parallel | Model |
|------|----------|--------------|----------|-------|
| TASK-053-001 | ~60k | none | no | — |
| TASK-053-002 | ~15k | 001 | no | haiku |

**Total estimated tokens:** ~75k

## Functional Requirements

- FR-1: Selecting tab [N] on a feature row stores position N in `activeTabFeature`; this value is not affected by navigating to task rows
- FR-2: Selecting tab [N] on a task row stores position N in `activeTabTask`; this value is not affected by navigating to feature rows
- FR-3: Each tracker is independently clamped to the valid range of tabs for the current context
- FR-4: The "Details" display name appears on the taskdetails tab in both selRunningTask and selCompletedTask contexts
- FR-5: The tab key `"taskdetails"` is unchanged (all existing dispatch logic remains valid)

## Non-Goals

- No changes to which tabs are available per context
- No changes to scroll position persistence
- No changes to tab key bindings or the order of tabs
- No changes to any other TUI views

## Success Metrics

- Navigating through the feature/task tree no longer surprises the user by jumping to a different tab
- "Task Details" never appears in the rendered TUI; "Details" appears consistently for both feature and task detail tabs
