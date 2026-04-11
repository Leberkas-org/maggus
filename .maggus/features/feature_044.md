<!-- maggus-id: e502839e-7332-41ab-9b4f-e72838caa022 -->
# Feature 044: Execution Plan Tab in Status View

## Introduction

Add a "Plan" tab to the status view that appears when a feature is selected in the left pane. This tab visualizes the execution order of tasks based on their Predecessors and Parallel fields — showing which tasks run first, which can run in parallel, and which are blocked waiting on dependencies. This gives users a clear picture of how the daemon will work through the feature.

### Architecture Context

- **Vision alignment:** "Live status view: feature tree, current task, acceptance criteria" — the execution plan adds understanding of task ordering and parallelism
- **Components involved:** `internal/parser` (Task struct already has `Predecessors` and `Parallel` fields), `cmd/status_rightpane.go` (tab rendering), `cmd/status_model.go` (tab definitions via `availableTabs()`)
- **Depends on:** Feature 040 (context-sensitive right pane) — the Plan tab is added to the feature-selected tab set

## Goals

- Users can see the full execution order of a feature's tasks at a glance
- Parallel batches are visually distinct from sequential steps
- Task status (done, running, pending, blocked, skipped) is reflected in the plan view
- The execution plan is computed from existing Predecessors/Parallel metadata — no new plan file format needed

## User Stories

### TASK-044-001: Compute execution order from task dependency metadata
**Description:** As a developer, I want a function that takes a list of tasks and produces an ordered execution plan (batches of parallel tasks, then sequential steps) so the Plan tab can render it.

**Token Estimate:** ~35k tokens
**Predecessors:** none
**Successors:** TASK-044-002
**Parallel:** yes — can run alongside nothing (single foundation task)

**Acceptance Criteria:**
- [ ] A new function `buildExecutionPlan(tasks []parser.Task) []executionStep` is created (in a new file `cmd/status_execution.go` or similar)
- [ ] Each `executionStep` contains: step number, list of task IDs in this step, whether the step is parallel
- [ ] Tasks with `Predecessors: none` are grouped into the first step(s) — parallel tasks together, sequential tasks as individual steps
- [ ] Tasks whose predecessors are all in earlier steps are placed in the earliest possible step
- [ ] Tasks with unresolvable predecessors (ID not found) are placed in a final "unresolved" group
- [ ] The function handles: no tasks, single task, all parallel, all sequential, diamond dependencies, completed tasks
- [ ] Unit tests cover each of the above cases
- [ ] `go vet ./...` and `go test ./...` pass

### TASK-044-002: Render the Plan tab with execution order visualization
**Description:** As a user viewing a feature in the status TUI, I want a Plan tab that shows the execution order so I can understand how tasks will be processed.

**Token Estimate:** ~45k tokens
**Predecessors:** TASK-044-001
**Successors:** TASK-044-003
**Parallel:** no

**Acceptance Criteria:**
- [ ] A new `renderPlanTab(width, height int) string` method is added to `statusModel`
- [ ] The Plan tab renders execution steps as a vertical list, e.g.:
  ```
  Step 1 (parallel)
    ✓ TASK-044-001  Compute execution order
    ⠹ TASK-044-002  Render Plan tab
  
  Step 2
    ○ TASK-044-003  Wire up Plan tab
  ```
- [ ] Each task row shows a status icon: `✓` done, spinner if running, `○` pending, `⚠` blocked, `>` skipped
- [ ] Parallel steps are labeled "(parallel)" and show all tasks in the batch
- [ ] Sequential steps show a single task
- [ ] The total estimated tokens for the feature is shown at the bottom (sum of token estimates from plan file)
- [ ] The view is scrollable if the plan is longer than the available height
- [ ] `go vet ./...` and `go test ./...` pass

### TASK-044-003: Wire up Plan tab in the context-sensitive tab system
**Description:** As a user, I want the Plan tab to appear when I select a feature in the left pane so I can access it alongside Summary, Details, and Metrics.

**Token Estimate:** ~20k tokens
**Predecessors:** TASK-044-002
**Successors:** none
**Parallel:** no

**Acceptance Criteria:**
- [ ] `availableTabs()` for `selFeature` returns `[Summary, Plan, Details, Metrics]` (Plan inserted after Summary)
- [ ] The Plan tab renders via `renderPlanTab()` in the tab dispatch switch
- [ ] Tab number keys map correctly with the new tab count
- [ ] The Plan tab does NOT appear for task selections or no-selection (only features)
- [ ] `go vet ./...` and `go test ./...` pass

## Task Dependency Graph

```
TASK-044-001 ──→ TASK-044-002 ──→ TASK-044-003
```

| Task | Estimate | Predecessors | Parallel | Model |
|------|----------|--------------|----------|-------|
| TASK-044-001 | ~35k | none | no | — |
| TASK-044-002 | ~45k | 001 | no | — |
| TASK-044-003 | ~20k | 002 | no | — |

**Total estimated tokens:** ~100k

## Functional Requirements

- FR-1: The Plan tab appears in the tab bar when a feature is selected in the left pane
- FR-2: The execution plan is computed from `Predecessors` and `Parallel` fields on each task
- FR-3: Tasks are grouped into steps — parallel tasks in the same step, sequential tasks in their own steps
- FR-4: Each task shows its current status (done/running/pending/blocked/skipped)
- FR-5: The view is scrollable for features with many tasks
- FR-6: No new fields or format changes required in plan files — uses existing metadata

## Non-Goals

- No interactive editing of the execution plan from this tab (read-only)
- No drag-and-drop reordering of tasks
- No Gantt chart or timeline visualization (text-based step list only)
- No changes to how the daemon actually executes tasks — this is a visualization only

## Technical Considerations

- The `parser.Task` struct already has `Predecessors []string` and `Parallel bool` — all the data needed is parsed
- Tasks without Predecessors/Parallel fields (older plan files) should default to sequential, no predecessors — the execution plan shows them in file order
- The `buildExecutionPlan` function should be a pure function (takes tasks, returns steps) for easy testing
- Feature 040 must be completed first for the dynamic tab system to be in place

## Success Metrics

- Users can immediately see which tasks will run in parallel and which are sequential
- The execution plan matches the actual order the daemon processes tasks
- No additional plan file markup needed — works with existing features

## Open Questions

None.
