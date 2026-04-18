# Feature-level Output Tab

**Date:** 2026-04-13
**Status:** Approved

## Summary

Add an Output tab to the `selFeature` context in the status TUI. When a feature row is selected, the Output tab aggregates tool invocations from all tasks in that feature into a single scrollable view, grouped by task with a header line per task showing inline stats.

## Tab Bar Change

`selFeature` tab order changes from:

```
[Summary, Plan, Details, Metrics]
```

to:

```
[Output, Summary, Plan, Details, Metrics]
```

Output is always tab 1 (index 0) in the feature context, consistent with the `selRunningTask` context.

## Layout

The Output tab renders a vertically stacked, scrollable list of task sections. Each task gets a separator header line followed by its tool entries (indented 4px/1 space).

### Task header format

```
─── TASK-001: Implement parser ✓  12k tok  $0.04  45s ───────────
```

- Status icon: `✓` (done, green), `▶` (running, yellow), `○` (pending/blocked, dim)
- Token count and cost only shown when non-zero
- Duration only shown for completed tasks
- Running task header uses warning color (yellow), completed uses muted/dim, pending uses dim

### Tool entry format

Reuses the existing `buildToolLines` format — indented by 2 spaces:

```
  📖 [Read]  parser.go                               09:14:32
```

For the live running task, the most recent tool entry gets the animated spinner character prepended.

### Pending/blocked tasks

Header line only, no tool entries below it:

```
─── TASK-004: Add tests ○ pending ───────────────────────────────
```

## Data Sources

| Task state   | Data source                                          |
|--------------|------------------------------------------------------|
| Completed    | JSONL via `loadCompletedTaskOutput` (existing)       |
| Running      | `workerSnapshots[taskID]` or main `snapshot`         |
| Pending      | Parser task list only (no log data)                  |

## Scroll Behaviour

- Uses the same `logScroll` / `logAutoScroll` fields on `statusModel`
- When a task in the feature is running: `logAutoScroll = true`, scroll position follows the live section
- User can manually scroll up; auto-scroll resumes on next log update (existing behaviour)
- Scroll offset is reset when the selected feature changes (same `updateTabsForSelectionChange` logic)

## Cache

Add two fields to `statusModel`:

```go
cachedFeatureOutput   []*runlog.StateSnapshot // one entry per task, in task order
cachedFeatureOutputID string                  // MaggusID of the plan for which cache is valid
```

`ensureFeatureOutput()` mirrors `ensureCompletedTaskOutput()` — loads all completed/pending tasks' snapshots once per feature selection. The running task's snapshot is always read live from `workerSnapshots` / `snapshot` and is not cached.

Cache is invalidated when:
- The selected feature changes (treeCursor moves to a different plan)
- A file-watcher reload fires (reloadPlans invalidates by resetting the ID)

## Implementation Pieces

1. **`status_model.go`** — add `cachedFeatureOutput` + `cachedFeatureOutputID` fields; update `availableTabs()` to prepend `{name: "Output", key: "featureoutput"}` for `selFeature`
2. **`status_task_output.go`** — add `loadFeatureOutput(dir, maggusID string, tasks []parser.Task)` that calls `loadCompletedTaskOutput` for each task and returns the slice; add `ensureFeatureOutput()` method on `statusModel`
3. **`status_rightpane.go`** — add `renderFeatureOutputTab(width, contentH int) string`; add `case "featureoutput":` dispatch in `renderRightPane`; call `ensureFeatureOutput()` from `updateTabsForSelectionChange` when ctx is `selFeature`
4. **`status_update.go`** — ensure `ensureFeatureOutput()` is called when log update ticks fire and the feature output tab is active (same pattern as `ensureCompletedTaskOutput`)

## Out of Scope

- No aggregated totals footer (stats are per-task in headers only)
- No collapsing/expanding individual task sections
- No separate scroll state per task section
