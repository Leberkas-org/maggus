<!-- maggus-id: ede59f6b-1208-463f-8a2b-3dc510304627 -->
# Feature 054: Feature-level Output Tab

## Introduction

Add an Output tab to the `selFeature` context in the status TUI. When a feature row is selected in the left-pane tree, the Output tab aggregates tool invocations from all tasks of that feature into a single scrollable view — grouped by task, with a header per task showing inline stats. Running tasks show a live tool feed with auto-scroll.

### Architecture Context

- **Vision alignment:** Improves the TUI feedback loop — developers can see the full execution history of a feature at a glance without selecting individual tasks
- **Components involved:** `cmd/status_model.go`, `cmd/status_rightpane.go`, `cmd/status_task_output.go`, `cmd/status_update.go`
- **New patterns:** None — extends existing output-loading, snapshot-reading, and scroll patterns already used by the per-task Output tab

## Goals

- Show all tool invocations from all tasks of a feature in one scrollable view when a feature row is selected
- Group entries by task with a header line per task (status icon, ID, title, tokens, cost, duration)
- For a running task: live-feed its tool entries at the bottom with spinner + auto-scroll
- Output tab is the first tab in the `selFeature` context, consistent with `selRunningTask`

## Tasks

### TASK-054-001: Add feature output cache, data loader, and tab registration
**Description:** As a TUI developer, I want the model to expose a feature-level output cache and register the Output tab for `selFeature` so that the rendering layer has data to display.

**Token Estimate:** ~50k tokens
**Predecessors:** none
**Successors:** TASK-054-002, TASK-054-003
**Parallel:** no

**Acceptance Criteria:**
- [ ] `statusModel` has two new fields: `cachedFeatureOutput []*runlog.StateSnapshot` and `cachedFeatureOutputID string`
- [ ] `loadFeatureOutput(dir, maggusID string, tasks []parser.Task) []*runlog.StateSnapshot` is added to `status_task_output.go`; it calls `loadCompletedTaskOutput` for each task and returns a slice (one entry per task, nil entries for tasks with no log data)
- [ ] `ensureFeatureOutput()` method is added to `statusModel`; it checks the cache ID against the selected plan's MaggusID, reloads on mismatch, and resets `logScroll = 0` + `logAutoScroll = true` on cache invalidation
- [ ] `availableTabs()` for `selFeature` is updated to `[{Output, featureoutput}, {Summary, summary}, {Plan, plan}, {Details, details}, {Metrics, metrics}]`
- [ ] `updateTabsForSelectionChange` calls `ensureFeatureOutput()` when `selectionCtx() == selFeature`
- [ ] All existing tests pass (`cd src && go test ./...`)

### TASK-054-002: Render the feature output tab
**Description:** As a developer, I want the feature Output tab to render task-grouped tool history so that I can see what every task in the feature did in one view.

**Token Estimate:** ~75k tokens
**Predecessors:** TASK-054-001
**Successors:** none
**Parallel:** yes — can run alongside TASK-054-003

**Acceptance Criteria:**
- [ ] `renderFeatureOutputTab(width, contentH int) string` is added to `status_rightpane.go`
- [ ] `case "featureoutput":` is wired in `renderRightPane`'s tab dispatch
- [ ] Each task renders a separator header line: `─── TASK-NNN-MMM: Title <icon> [tokens] [cost] [duration] ───`
  - Status icon: `✓` (green) for done, `▶` (yellow) for running, `○` (dim) for pending/blocked
  - Token count and cost are only shown when non-zero; duration only shown for completed tasks
  - Running task header uses `statusCyanStyle` or warning color to distinguish it
- [ ] Tool entries from `cachedFeatureOutput` are rendered below each header, indented 2 spaces, using `buildToolLines` + `renderScrollableToolList` (reuses existing shared helpers)
- [ ] For the currently running task: live snapshot from `workerSnapshots[taskID]` or main `snapshot` (same logic as `snapshotForSelectedTask` but for the feature) is used instead of the cache entry; latest tool entry gets the spinner character
- [ ] Pending/blocked tasks with no log data show the header only (no tool lines)
- [ ] The single `logScroll` offset applies to the combined flat list of all rendered lines across all tasks
- [ ] When no tasks have any output and none is running, shows `"  No output history"` placeholder
- [ ] `status_rightpane.go` stays under 500 lines; split into a helper file if needed
- [ ] All existing tests pass (`cd src && go test ./...`)

### TASK-054-003: Wire live updates for feature output tab
**Description:** As a developer, I want the feature output tab to refresh on every log tick when active so that the running task's tool feed stays live.

**Token Estimate:** ~25k tokens
**Predecessors:** TASK-054-001
**Successors:** none
**Parallel:** yes — can run alongside TASK-054-002

**Acceptance Criteria:**
- [ ] In `status_update.go`, the `logFileUpdateMsg` handler calls `ensureFeatureOutput()` when `selectionCtx() == selFeature` and `activeTabKey() == "featureoutput"`
- [ ] Auto-scroll advances when a task in the feature is running and `logAutoScroll` is true (the existing scroll advance logic is reused; no new mechanism needed)
- [ ] Navigating from one feature to a different feature resets `logScroll` and `logAutoScroll` (verified via `ensureFeatureOutput`'s cache ID check)
- [ ] All existing tests pass (`cd src && go test ./...`)

## Task Dependency Graph

```
TASK-054-001 ──→ TASK-054-002
             └─→ TASK-054-003
```

| Task | Estimate | Predecessors | Parallel | Model |
|------|----------|--------------|----------|-------|
| TASK-054-001 | ~50k | none | no | — |
| TASK-054-002 | ~75k | 001 | yes (with 003) | — |
| TASK-054-003 | ~25k | 001 | yes (with 002) | — |

**Total estimated tokens:** ~150k

## Functional Requirements

- FR-1: When a feature row is selected, the right pane tab bar must show `[1] Output  [2] Summary  [3] Plan  [4] Details  [5] Metrics`
- FR-2: The Output tab must display one section per task in the feature, in task order
- FR-3: Each task section must begin with a separator header showing: task ID, task title, status icon (✓/▶/○), and — for completed tasks with data — token count, cost in USD, and duration
- FR-4: Tool entries must be rendered using the existing `buildToolLines` format (icon, type, description, right-aligned timestamp)
- FR-5: Tool entries for completed tasks must be loaded from JSONL log files via `loadCompletedTaskOutput`
- FR-6: The currently running task's tool entries must come from the live snapshot (not JSONL cache), and must update on every log tick
- FR-7: The running task's latest tool entry must display the animated spinner character
- FR-8: Auto-scroll must follow the running task section when `logAutoScroll` is true
- FR-9: Navigating to a different feature must reset scroll position and cache

## Non-Goals

- No collapsing/expanding individual task sections
- No per-task scroll position — one shared scroll offset for the whole view
- No aggregated totals footer — stats are per-task in headers only
- No changes to the per-task Output tab (selRunningTask / selCompletedTask contexts)

## Technical Considerations

- `renderFeatureOutputTab` builds a flat list of all lines from all tasks, then applies the single `logScroll` offset across the combined list — this is simpler than per-section scroll and consistent with the existing tool list rendering
- The running task's snapshot must be read fresh each render (not cached in `cachedFeatureOutput`) to avoid stale live data
- `status_rightpane.go` is currently ~440 lines; adding `renderFeatureOutputTab` may push it past 500 — split overflow into `status_feature_output.go` if needed
- `loadFeatureOutput` should handle the case where `maggusID` is empty (feature has no UUID yet) gracefully — return nil entries for all tasks

## Success Metrics

- Developer can select a feature row and immediately see the full tool history of all completed tasks in one view
- Running task's live tool feed is visible in the feature Output tab without needing to expand and select the task row

## Open Questions

(none)
