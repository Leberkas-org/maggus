<!-- maggus-id: 861236a4-1a28-47d5-8e1b-afad319e720c -->
# Feature 040: Context-Sensitive Right Pane in Status View

## Introduction

Redesign the status view's right pane to be context-sensitive — the available tabs and their content change based on what is selected in the left pane tree. Currently, the right pane has a fixed set of 4 tabs (Output, Item Details, Task Details, Metrics) that are always visible regardless of selection. This leads to dead tabs (e.g., Output showing "No active run" when a feature is selected) and the parallel-mode Output tab cramming all workers into tiny cards that only show the last tool invocation.

The new design makes the left pane the primary navigator: what you select determines what tabs appear on the right. This gives each context level (no selection, feature, running task, completed task) the exact tabs that make sense.

### Architecture Context

- **Vision alignment:** "Live status view: feature tree, current task, acceptance criteria, tool invocation log, token usage" — this feature improves the status view to be more contextual and informative
- **Components involved:** `cmd/status_model.go` (model struct, tab state), `cmd/status_rightpane.go` (tab bar + tab content rendering), `cmd/status_update.go` (key handling, tab switching), `cmd/status_view.go` (footer hints), `cmd/status_workers.go` (parallel worker panes — will be replaced for task-level output), `cmd/status_metrics.go` (metrics rendering — needs scoping changes), `cmd/status_leftpane.go` (selection awareness)
- **New patterns:** Dynamic tab set based on selection context; per-task tool log loading from run log files

## Goals

- Right pane tabs adapt to the currently selected item in the left pane tree
- Each tab always shows meaningful content — no dead/empty states
- Output tab shows a full scrollable tool log for a single selected task (same quality as current single-mode daemon output)
- Completed tasks can show their tool history loaded from run logs
- Metrics tab always includes global metrics plus scoped metrics for the current selection
- Documentation (tui.md) reflects the new behavior with programmatically-rendered text screenshots

## User Stories

### TASK-040-001: Introduce selection context type and dynamic tab mapping
**Description:** As a developer, I want the status model to track what kind of item is selected (nothing, feature, running task, completed task) and compute which tabs are available, so the rest of the UI can render accordingly.

**Token Estimate:** ~50k tokens
**Predecessors:** none
**Successors:** TASK-040-002, TASK-040-003, TASK-040-004, TASK-040-005
**Parallel:** no — this is the foundation all other tasks depend on
**Model:** opus

**Acceptance Criteria:**
- [x] A new type `selectionContext` (or similar) is introduced in `status_model.go` with values: `selNone`, `selFeature`, `selRunningTask`, `selCompletedTask`
- [x] A method `selectionCtx()` on `statusModel` returns the current context by examining `buildTreeItems()[treeCursor]` and daemon state (is the task currently being worked on by the daemon?)
- [x] A method `availableTabs()` returns the ordered list of tab definitions (name + render function) for the current selection context, following this mapping:
  - `selNone` → `[Metrics]`
  - `selFeature` → `[Summary, Details, Metrics]`
  - `selRunningTask` → `[Output, Task Details, Metrics]`
  - `selCompletedTask` → `[Summary, Output, Task Details, Metrics]`
- [x] `activeTab` is clamped to `len(availableTabs())-1` whenever the selection changes (cursor movement in left pane)
- [x] When selection context changes (e.g., moving from a task to a feature), `activeTab` resets to 0 (first available tab)
- [x] The old fixed `rightPaneTabNames` slice is removed
- [x] `go vet ./...` and `go test ./...` pass
- [x] Unit tests verify the tab mapping for each selection context

### TASK-040-002: Render dynamic tab bar and route to correct tab content
**Description:** As a user, I want the tab bar to only show tabs relevant to my selection, and number keys to map positionally to visible tabs, so I can quickly navigate without dead tabs.

**Token Estimate:** ~45k tokens
**Predecessors:** TASK-040-001
**Successors:** TASK-040-006
**Parallel:** yes — can run alongside TASK-040-003, TASK-040-004, TASK-040-005

**Acceptance Criteria:**
- [x] `renderRightPaneTabBar()` renders only the tabs returned by `availableTabs()` for the current selection
- [x] Number key labels in the tab bar start at `[2]` and increment sequentially (matching the current pattern where `[1]` focuses the left pane)
- [x] `renderRightPane()` dispatches to the correct render function based on `availableTabs()[activeTab]` instead of a hardcoded switch
- [x] Key handling in `updateList()` maps keys `2`, `3`, `4`, `5` to positional indices within `availableTabs()` (not fixed tab indices)
- [x] Keys beyond the available tab count are ignored (e.g., pressing `4` when only 3 tabs exist does nothing)
- [x] Footer hints in `statusSplitFooter()` update the tab range dynamically (e.g., `1-3: tabs` when 2 right-pane tabs exist, `1-4: tabs` when 3 exist)
- [x] `go vet ./...` and `go test ./...` pass

### TASK-040-003: Implement Summary tab for features and completed tasks
**Description:** As a user viewing a feature, I want a Summary tab showing progress, task counts, and aggregate stats so I get a quick overview without switching to Metrics.

**Token Estimate:** ~40k tokens
**Predecessors:** TASK-040-001
**Successors:** TASK-040-006
**Parallel:** yes — can run alongside TASK-040-002, TASK-040-004, TASK-040-005

**Acceptance Criteria:**
- [x] A new `renderSummaryTab(width, height int) string` method is added to `statusModel`
- [x] When a **feature** is selected, Summary shows:
  - Feature title and filename
  - Progress bar with `done/total` count
  - Task breakdown: N done, N pending, N blocked
  - If daemon is running on this feature: current task ID, elapsed time, spinner
  - If in parallel mode: list of active workers for this feature with their status
  - Aggregate tokens and cost (from `cachedFeatureMetrics`)
- [x] When a **completed task** is selected, Summary shows:
  - Task ID and title
  - Outcome: success/fail
  - Duration (from run log)
  - Tokens used (input/output) and cost
  - Commit hash and message (if available from run log)
- [x] Summary is never shown for `selNone` or `selRunningTask` (enforced by tab mapping)
- [x] `go vet ./...` and `go test ./...` pass

### TASK-040-004: Implement per-task Output tab with full tool log
**Description:** As a user viewing a running or completed task, I want the Output tab to show the full scrollable tool invocation log for that specific task, so I can see exactly what the agent did.

**Token Estimate:** ~70k tokens
**Predecessors:** TASK-040-001
**Successors:** TASK-040-006
**Parallel:** yes — can run alongside TASK-040-002, TASK-040-003, TASK-040-005
**Model:** opus

**Acceptance Criteria:**
- [ ] When a **running task** is selected (daemon is actively working on it), the Output tab shows the same rich snapshot view as the current `renderSnapshotInPane()`: spinner, status, task ID/title, scrollable tool list, tokens, cost, elapsed time
- [ ] The tool list for a running task auto-scrolls to follow the latest entry; manual scroll-up pauses auto-scroll (existing behavior preserved)
- [ ] When a **completed task** is selected, the Output tab loads tool history from the run log files in `.maggus/runs/`
- [ ] Completed task tool history is loaded by scanning the run log JSONL for entries matching the selected task ID, extracting tool-use events
- [ ] The completed task view shows: task ID/title, outcome (done/failed), full scrollable tool list, final token counts and cost
- [ ] Scrolling (up/down/g/G) works identically for both running and completed task output
- [ ] The old parallel-mode worker card grid (`renderWorkerPanes`) is no longer used for the Output tab (it may still exist but is not rendered)
- [ ] `go vet ./...` and `go test ./...` pass

### TASK-040-005: Update Metrics tab to always show global + scoped metrics
**Description:** As a user, I want the Metrics tab to always show global metrics and additionally show scoped metrics based on my selection, so I always have the full picture.

**Token Estimate:** ~30k tokens
**Predecessors:** TASK-040-001
**Successors:** TASK-040-006
**Parallel:** yes — can run alongside TASK-040-002, TASK-040-003, TASK-040-004

**Acceptance Criteria:**
- [x] When **nothing** is selected (`selNone`): Metrics shows "This Repository" and "All Time (Global)" sections only (no "Selected Feature" section)
- [x] When a **feature** is selected: Metrics shows "Selected Feature" (with feature-scoped data), "This Repository", and "All Time (Global)" — same as current behavior
- [x] When a **task** is selected (running or completed): Metrics shows "Selected Task" (task-level tokens, cost, duration, model), "Selected Feature" (parent feature aggregate), "This Repository", and "All Time (Global)"
- [x] The "Selected Task" section is new and shows: task ID, tokens (in/out), cost, cache hit rate, duration, model used
- [x] Task-level metrics are loaded from `~/.maggus/usage/work.jsonl` by matching task ID
- [x] `loadMetrics()` is updated to also load task-level metrics when a task is selected
- [x] `go vet ./...` and `go test ./...` pass

### TASK-040-006: Wire up selection-change behavior and clean up old code
**Description:** As a developer, I want cursor movement in the left pane to properly trigger tab resets and content updates, and old dead code to be removed, so the implementation is clean and correct.

**Token Estimate:** ~40k tokens
**Predecessors:** TASK-040-002, TASK-040-003, TASK-040-004, TASK-040-005
**Successors:** TASK-040-007
**Parallel:** no — integration task after all tab implementations are done

**Acceptance Criteria:**
- [ ] Moving the cursor up/down in the left pane calls `selectionCtx()` before and after; if the context type changed, `activeTab` resets to 0
- [ ] Collapsing a feature (Left key) while a child task is selected moves selection to the feature and resets tabs
- [ ] `rebuildRightPane()` and `rebuildForSelectedPlan()` trigger metric reloads scoped to the new selection
- [ ] The old `renderWorkerPanes()` card grid in `status_workers.go` is removed or kept only if still needed for a different purpose (verify and decide)
- [ ] The old fixed `case 0: / case 1: / case 2: / case 3:` switch in `renderRightPane()` is replaced by the dynamic dispatch
- [ ] The `1-5: tabs` footer hint dynamically reflects the actual tab count
- [ ] All keyboard shortcuts listed in the footer are accurate for each tab
- [ ] `go vet ./...` and `go test ./...` pass
- [ ] Manual verification: navigate between features and tasks, observe tabs changing correctly

### TASK-040-007: Update tui.md documentation with text-rendered screenshots
**Description:** As a user reading the TUI reference, I want the documentation to describe the new context-sensitive tab behavior and include text-rendered screenshots that can be updated without manual screenshotting.

**Token Estimate:** ~35k tokens
**Predecessors:** TASK-040-006
**Successors:** none
**Parallel:** no — needs final implementation to document accurately

**Acceptance Criteria:**
- [ ] `docs/reference/tui.md` Status View section is rewritten to describe the context-sensitive tab system
- [ ] The tab mapping table is documented:
  - Nothing selected → Metrics
  - Feature selected → Summary | Details | Metrics
  - Running task → Output | Task Details | Metrics
  - Completed task → Summary | Output | Task Details | Metrics
- [ ] Each tab's content is briefly described (what it shows for each context)
- [ ] Keyboard shortcuts section is updated (dynamic tab numbers, selection-dependent behavior)
- [ ] Screenshot image references (`![Status View](/screenshots/plan-view.png)` etc.) are replaced with text-rendered representations using fenced code blocks that show the TUI layout
- [ ] Text screenshots show at least: feature-selected view (Summary tab), running-task view (Output tab), and completed-task view (Summary tab)
- [ ] `docs/guide/concepts.md` is checked for consistency — no references to old fixed-tab behavior remain
- [ ] No broken image references remain in any docs file

## Task Dependency Graph

```
TASK-040-001 ──→ TASK-040-002 ──┐
             ──→ TASK-040-003 ──┤
             ──→ TASK-040-004 ──├──→ TASK-040-006 ──→ TASK-040-007
             ──→ TASK-040-005 ──┘
```

| Task | Estimate | Predecessors | Parallel | Model |
|------|----------|--------------|----------|-------|
| TASK-040-001 | ~50k | none | no | opus |
| TASK-040-002 | ~45k | 001 | yes (with 003, 004, 005) | — |
| TASK-040-003 | ~40k | 001 | yes (with 002, 004, 005) | — |
| TASK-040-004 | ~70k | 001 | yes (with 002, 003, 005) | opus |
| TASK-040-005 | ~30k | 001 | yes (with 002, 003, 004) | — |
| TASK-040-006 | ~40k | 002, 003, 004, 005 | no | — |
| TASK-040-007 | ~35k | 006 | no | — |

**Total estimated tokens:** ~310k

## Functional Requirements

- FR-1: The available right-pane tabs must change based on left-pane selection: nothing → Metrics; feature → Summary, Details, Metrics; running task → Output, Task Details, Metrics; completed task → Summary, Output, Task Details, Metrics
- FR-2: Number keys map positionally to visible tabs (key `1` = left pane focus, key `2` = first right tab, etc.)
- FR-3: Changing selection in the left pane resets `activeTab` to 0 when the context type changes
- FR-4: Output tab for a running task shows the full scrollable tool invocation log with auto-scroll
- FR-5: Output tab for a completed task loads and displays tool history from run log JSONL files
- FR-6: Summary tab for a feature shows progress bar, task breakdown, aggregate tokens/cost, and active workers
- FR-7: Summary tab for a completed task shows outcome, duration, tokens, cost, and commit info
- FR-8: Metrics tab always shows global and repository metrics; additionally shows feature-scoped or task-scoped metrics based on selection
- FR-9: Collapsing a feature in the left pane while a child task is selected moves selection to the feature and resets tabs
- FR-10: Footer key hints dynamically reflect the current tab count and available actions

## Non-Goals

- No changes to the left pane tree rendering or feature tab bar at the top
- No new keyboard shortcuts beyond the existing set
- No changes to the daemon communication or run log format
- No changes to the approval workflow or feature management actions (approve, delete, run)
- No real image screenshots — text-rendered only for docs

## Design Considerations

- The `treeItem` struct already carries `kind` (plan/task/separator) and the parent `plan` — this provides the data needed to determine selection context
- A task is "running" when `daemon.Running && daemon.CurrentTask == task.ID` (or it appears in `workerIndex` for parallel mode)
- For completed task output, run log JSONL files in `.maggus/runs/` contain structured entries with `task_id` fields — filter by selected task ID
- The current `renderSnapshotInPane()` is excellent for running task output — reuse it directly
- The worker card grid (`renderWorkerPanes`) becomes unnecessary since each task now gets its own Output tab when selected

## Technical Considerations

- **File size limit:** `status_rightpane.go` is already substantial. The new Summary tab renderer and per-task output loader may need to go in a new file (e.g., `status_summary.go`, `status_task_output.go`) to respect the 500-line limit
- **Run log loading:** Loading completed task tool history requires scanning JSONL files. Cache the result per task ID to avoid re-reading on every render frame. Invalidate when a new log entry is detected.
- **Parallel mode:** With the new per-task Output tab, the worker card grid is no longer needed for the Output tab. However, the feature Summary tab could show a compact list of active workers when in parallel mode — reuse the worker index data.
- **ARCHITECTURE.md** may need a minor update to reflect the context-sensitive tab system in the TUI Sub-Models table

## Success Metrics

- Every tab shows meaningful content — no "No active run" or empty states when a valid item is selected
- Users can view the full tool log of any completed task without leaving the TUI
- Navigating between features and tasks feels responsive — tab switches are instant
- The docs accurately describe the new behavior and can be updated without screenshots

## Open Questions

None — all questions resolved during brainstorming.
