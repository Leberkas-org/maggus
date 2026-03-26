<!-- maggus-id: 8bd44f88-d1d2-428b-a3a3-f323106f5b9b -->
# Feature 001: Split-Pane Status UI Redesign

## Introduction

Redesign the `maggus status` command from a flat feature-tab layout into a two-pane TUI: a persistent left pane listing all features and bugs, and a right pane with four numbered tabs (Output, Feature Details, Current Task, Metrics). This gives users a clear overview of all work items at a glance while keeping the rich detail views accessible via tab switching.

### Architecture Context

- **Vision alignment:** Improves the primary user-facing feedback surface — the status command is the main way users monitor ongoing work
- **Components touched:** `cmd/status_*.go` files (full rewrite of model/update/view/runlog), `internal/usage/` (read-only for metrics), `internal/globalconfig/` (read-only for global metrics)
- **Shared components reused unchanged:** `cmd/tasklist.go` (`taskListComponent`), `cmd/detail.go` (detail view), `internal/tui/styles/` (all styles and nav helpers)
- **New file introduced:** `cmd/status_metrics.go` for metrics loading and calculation
- **Entry point unchanged:** `cmd/status_cmd.go`

## Goals

- Replace the current horizontal feature-tab layout with a split-pane layout (left list + right tabbed content)
- Left pane: scrollable list of features and bugs with approval status, ~50-char title truncation, and inline actions
- Right pane: 4 tabs switchable by number keys — `1 Output`, `2 Feature Details`, `3 Current Task`, `4 Metrics`
- Tab 2 reuses the existing `taskListComponent` and detail view for the selected plan's tasks
- Tab 3 shows a read-only detail view of the next workable task
- Tab 4 shows per-feature, per-repo, and global metrics with calculated stats (cache hit rate, avg duration, etc.)
- Left pane supports approve/unapprove, delete (with confirmation), and UI-only reorder

## Tasks

### TASK-001-001: New statusModel struct and init
**Description:** As a developer, I want a new `statusModel` struct shaped around the split-pane layout so that all subsequent tasks have a stable foundation to build on.

**Token Estimate:** ~40k tokens
**Predecessors:** none
**Successors:** TASK-001-002, TASK-001-003, TASK-001-004
**Parallel:** no — everything else depends on this

**Acceptance Criteria:**
- [x] `status_model.go` defines `statusModel` with fields: `plans []parser.Plan`, `planCursor int`, `leftFocused bool`, `showAll bool`, `activeTab int` (0–3), embedded `taskListComponent`, `currentTaskViewport viewport.Model`, `currentTaskDetail detailState`, `width int`, `height int`, plus existing fields for daemon/log/snapshot/watcher/presence/agentName/is2x/nextTaskID/nextTaskFile
- [x] `newStatusModel()` loads all plans (features + bugs), sets `leftFocused = true`, `activeTab = 0`
- [x] Helper `visiblePlans()` returns plans filtered by `showAll` (hides completed when false)
- [x] Helper `selectedPlan()` returns the plan at `planCursor` from `visiblePlans()`
- [x] Helper `rebuildForSelectedPlan()` loads the selected plan's tasks into `taskListComponent.Tasks` and resets cursor/scroll
- [x] `WindowSizeMsg` handler sets `width`/`height` and propagates sizes to `taskListComponent` viewport and `currentTaskViewport`
- [x] Existing daemon status types and `daemonStatus` field are preserved
- [x] `go build ./...` passes with no errors

---

### TASK-001-002: Left pane rendering
**Description:** As a user, I want to see a clear left pane listing all features and bugs so that I can understand the overall work state at a glance.

**Token Estimate:** ~50k tokens
**Predecessors:** TASK-001-001
**Successors:** TASK-001-003
**Parallel:** yes — can run alongside TASK-001-004

**Acceptance Criteria:**
- [x] Left pane renders at exactly `min(50, terminalWidth/3)` chars wide with a right border `│`
- [x] Header row: `Features & Bugs` in muted uppercase label style
- [x] Each row shows: cursor indicator (`▶` selected, ` ` otherwise), title truncated to fit with `…`, approval badge right-aligned (`✓` green for approved, `○` orange for pending, `done` muted for completed)
- [x] Features listed first, bugs listed below a `───` separator line; bugs rendered in error/red color
- [x] Selected row has a highlighted background (`#1f2937` or styles equivalent) and left accent border in primary color
- [x] Footer shows key hint lines: `↑↓ navigate  enter inspect`, `alt+p approve  alt+d delete`, `alt+↑↓ reorder`
- [x] When `leftFocused = false`, the left pane border dims to muted color
- [x] `go build ./...` passes

---

### TASK-001-003: Left pane keyboard actions (approve, delete, reorder)
**Description:** As a user, I want to approve/unapprove, delete, and reorder plans from the left pane so that I can manage my work items without leaving the status view.

**Token Estimate:** ~50k tokens
**Predecessors:** TASK-001-001, TASK-001-002
**Successors:** TASK-001-008
**Parallel:** no

**Acceptance Criteria:**
- [x] `alt+p` toggles approval of the selected plan (calls existing approval toggle logic); left pane badge updates immediately
- [x] `alt+d` opens an inline confirmation dialog ("Delete feature_001? [y/n]"); `y` deletes the file and removes the plan from the list; `n` or `esc` cancels
- [x] `alt+↑` moves selected plan one position up in the `plans` slice (memory only, no file writes); cursor follows the moved item
- [x] `alt+↓` moves selected plan one position down; cursor follows
- [x] Reorder does not cross the features/bugs separator — features stay with features, bugs with bugs
- [x] `alt+a` toggles `showAll` (show/hide completed plans); cursor is clamped after toggle
- [x] `go build ./...` passes

---

### TASK-001-004: Right pane tab bar and Tab 1 (Output)
**Description:** As a user, I want a numbered tab bar on the right pane and a live output view on Tab 1 so that I can monitor the currently running agent task.

**Token Estimate:** ~70k tokens
**Predecessors:** TASK-001-001
**Successors:** TASK-001-005, TASK-001-006, TASK-001-007
**Parallel:** yes — can run alongside TASK-001-002

**Acceptance Criteria:**
- [ ] Tab bar renders at top of right pane: `1 Output  2 Feature Details  3 Current Task  4 Metrics`; active tab has bottom underline in primary color and bold text; inactive tabs are muted; number prefix is dimmed
- [ ] Pressing `1`–`4` switches `activeTab` regardless of which pane has focus
- [ ] Tab 1 renders the live output view: migrated and adapted from the current `status_runlog.go` rich/plain snapshot rendering
- [ ] Rich view (when snapshot available and daemon running): spinner + status line, task ID + title, tool invocations list, token/cost, elapsed time
- [ ] Plain view (fallback): raw log lines with scroll indicator
- [ ] `↑↓` scrolls the log when right pane is focused and Tab 1 is active
- [ ] Log polling (200ms tick) and spinner (80ms tick) still function correctly
- [ ] `go build ./...` passes

---

### TASK-001-005: Tab 2 — Feature Details
**Description:** As a user, I want Tab 2 to show the task list for the selected feature so that I can inspect and interact with its tasks without leaving the status view.

**Token Estimate:** ~60k tokens
**Predecessors:** TASK-001-001, TASK-001-004
**Successors:** TASK-001-008
**Parallel:** yes — can run alongside TASK-001-006 and TASK-001-007

**Acceptance Criteria:**
- [ ] Tab 2 renders the `taskListComponent` loaded with the selected plan's tasks
- [ ] Feature title, progress bar (`█░` style), and done/total count shown above the task list
- [ ] `↑↓` navigates tasks when right pane is focused and Tab 2 is active
- [ ] `enter` opens the task detail view (using existing `detail.go` rendering) inline in the right pane
- [ ] `esc` from detail view returns to the task list
- [ ] Criteria mode and action picker (unblock, resolve, delete, skip) work as in the existing implementation
- [ ] When the selected plan changes (left pane navigation), the task list reloads and cursor resets to 0
- [ ] `go build ./...` passes

---

### TASK-001-006: Tab 3 — Current Task
**Description:** As a user, I want Tab 3 to show the detail of the next workable task so that I can see exactly what the agent will work on next.

**Token Estimate:** ~40k tokens
**Predecessors:** TASK-001-001, TASK-001-004
**Successors:** TASK-001-008
**Parallel:** yes — can run alongside TASK-001-005 and TASK-001-007

**Acceptance Criteria:**
- [ ] Tab 3 renders the detail view of the task identified by `nextTaskID`/`nextTaskFile` using existing `detail.go` rendering logic
- [ ] View is read-only — no criteria editing, no action picker
- [ ] `↑↓` scrolls the viewport when right pane is focused and Tab 3 is active
- [ ] If no workable task exists, shows a centered message: `No pending tasks` in muted color
- [ ] Detail content updates when `featureSummaryUpdateMsg` fires (file watcher reload)
- [ ] `go build ./...` passes

---

### TASK-001-007: status_metrics.go and Tab 4 (Metrics)
**Description:** As a user, I want Tab 4 to show usage metrics for the selected feature, this repository, and all-time global stats so that I can understand the cost and efficiency of my work.

**Token Estimate:** ~80k tokens
**Predecessors:** TASK-001-001, TASK-001-004
**Successors:** TASK-001-008
**Parallel:** yes — can run alongside TASK-001-005 and TASK-001-006
**Model:** opus

**Acceptance Criteria:**
- [ ] New file `cmd/status_metrics.go` with function `loadFeatureMetrics(itemID string) featureMetrics` that reads `~/.maggus/usage/work.jsonl`, filters records by `item_id == itemID`, and returns aggregated stats
- [ ] New function `loadRepoMetrics(repoURL string) repoMetrics` that filters the same file by `repository == repoURL`
- [ ] `featureMetrics` contains: `tasksCompleted int`, `totalCostUSD float64`, `totalTokens int64`, `cacheHitRate float64`, `cacheSavingsUSD float64`, `avgDurationSecs float64`, `avgCostUSD float64`, `modelBreakdown map[string]modelStat`
- [ ] `repoMetrics` contains: `featuresCompleted int`, `bugsCompleted int`, `tasksCompleted int`, `totalCostUSD float64`, `totalTokens int64`, `gitCommits int`
- [ ] Cache hit rate calculated as `cacheReadTokens / (inputTokens + cacheReadTokens)`
- [ ] Cache savings calculated as `cacheReadTokens * (fullInputPrice - cacheReadPrice)` using opus pricing as default approximation
- [ ] Tab 4 renders 4 sections: **Selected Feature**, **This Repository**, **All Time (Global)** (from `~/.maggus/metrics.yml`), **Model Breakdown**
- [ ] Each section uses a two-column grid: label (muted) + value (bright)
- [ ] Metrics reload when selected plan changes
- [ ] `go build ./...` passes

---

### TASK-001-008: Final wiring and integration
**Description:** As a user, I want the fully integrated split-pane status UI so that all panes, tabs, and interactions work together correctly end-to-end.

**Token Estimate:** ~60k tokens
**Predecessors:** TASK-001-003, TASK-001-004, TASK-001-005, TASK-001-006, TASK-001-007
**Successors:** none
**Parallel:** no

**Acceptance Criteria:**
- [ ] `tab` key toggles focus between left pane and right pane; `leftFocused` bool drives border dim/highlight
- [ ] When left pane is focused, `↑↓` navigates plans; when right pane is focused, `↑↓` scrolls/navigates within the active tab
- [ ] `enter` from left pane moves focus to right pane (switches to Tab 2 — Feature Details)
- [ ] Selecting a different plan in the left pane refreshes Tab 2 task list, Tab 3 current task, and Tab 4 metrics in the right pane
- [ ] File watcher reload (`featureSummaryUpdateMsg`) preserves `planCursor` by matching plan filename; clamps cursor if list shrinks
- [ ] `q` or `ctrl+c` exits the TUI
- [ ] `alt+r` runs the next task (existing behavior preserved)
- [ ] Discord Rich Presence integration preserved
- [ ] Terminal resize (`WindowSizeMsg`) correctly updates all pane widths, viewport heights, and left pane character budget
- [ ] `go test ./...` passes
- [ ] `go vet ./...` passes with no warnings
- [ ] Manual smoke test: launch `maggus status` with at least one feature file present; navigate left pane, switch tabs, open task detail, verify metrics load

## Task Dependency Graph

```
TASK-001-001 ──→ TASK-001-002 ──→ TASK-001-003 ──┐
             ├──→ TASK-001-004 ──→ TASK-001-005 ──┤
             │                 ├──→ TASK-001-006 ──┤──→ TASK-001-008
             │                 └──→ TASK-001-007 ──┘
             └──────────────────────────────────────┘
```

| Task | Estimate | Predecessors | Parallel | Model |
|------|----------|--------------|----------|-------|
| TASK-001-001 | ~40k | none | no | — |
| TASK-001-002 | ~50k | 001 | yes (with 004) | — |
| TASK-001-003 | ~50k | 001, 002 | no | — |
| TASK-001-004 | ~70k | 001 | yes (with 002) | — |
| TASK-001-005 | ~60k | 001, 004 | yes (with 006, 007) | — |
| TASK-001-006 | ~40k | 001, 004 | yes (with 005, 007) | — |
| TASK-001-007 | ~80k | 001, 004 | yes (with 005, 006) | opus |
| TASK-001-008 | ~60k | 003, 004, 005, 006, 007 | no | — |

**Total estimated tokens:** ~450k

## Functional Requirements

- FR-1: The status TUI must display a left pane and right pane separated by a vertical border
- FR-2: The left pane must list all features first, then bugs, separated by a horizontal divider line
- FR-3: Each plan entry in the left pane must show its title truncated at ~45 visible characters with `…` suffix if longer
- FR-4: Each plan entry must show an approval badge: `✓` (green, approved), `○` (orange, pending), `done` (muted, completed)
- FR-5: The right pane must show a tab bar with tabs `1 Output`, `2 Feature Details`, `3 Current Task`, `4 Metrics`; pressing the number key switches to that tab
- FR-6: Tab 1 must show the live agent output (spinner, tool calls, tokens, cost, elapsed) when a run is active, and the last run log otherwise
- FR-7: Tab 2 must show the task list for the plan currently selected in the left pane, with the existing task detail and criteria editing capabilities
- FR-8: Tab 3 must show a read-only detail view of the next workable task across all plans
- FR-9: Tab 4 must show metrics in four sections: Selected Feature, This Repository, All Time, Model Breakdown
- FR-10: Tab 4 must display calculated metrics: cache hit rate, estimated cache savings in USD, average task duration, average task cost, and success rate
- FR-11: `alt+p` must toggle approval of the selected plan in the left pane
- FR-12: `alt+d` must prompt for confirmation before deleting the selected plan's file
- FR-13: `alt+↑` and `alt+↓` must reorder the selected plan within its group (features or bugs) in memory only — no file writes, resets on next file watcher reload
- FR-14: `tab` key must toggle keyboard focus between left pane and right pane; the active pane border must be visually distinct
- FR-15: Selecting a plan in the left pane must update Tab 2, Tab 3, and Tab 4 to reflect that plan's data

## Non-Goals

- Persistent reorder (writing sort order to a file) — UI-only for now
- Creating or editing feature/bug files from the status view
- Running tasks directly from the status view beyond the existing `alt+r` shortcut
- Any changes to `status_cmd.go`, `tasklist.go`, or `detail.go`

## Design Considerations

- Left pane width: `min(50, terminalWidth/3)` characters; right pane takes the remainder
- Mockups reviewed and approved during brainstorming session (see `.superpowers/brainstorm/`)
- Reuse `styles.FullScreenColor()` with `styles.ThemeColor(is2x)` for the outer border
- Use `styles.CursorUp` / `styles.CursorDown` from `internal/tui/styles/nav.go` for all cursor movement
- Use `styles.Truncate()` for plan title truncation in the left pane
- Left pane footer key hints use `styles.StatusBar` style

## Technical Considerations

- The existing `taskListComponent` embed in `statusModel` continues to serve Tab 2 — no changes needed to `tasklist.go`
- `detail.go`'s `renderDetail()` function can be called directly for Tab 3's read-only view by constructing a read-only `detailState` with no criteria editing
- Metrics loading scans `~/.maggus/usage/work.jsonl` on plan selection change; for large files this may be slow — keep it synchronous for now (file is typically small)
- The `featureSummaryUpdateMsg` handler must update `plans`, re-clamp `planCursor`, and call `rebuildForSelectedPlan()` to keep Tab 2 in sync
- Cache savings approximation: use claude-opus-4-6 pricing ($15/$75 input/output per 1M tokens, cache read at $1.50/1M) as a safe default; the exact model used is available per record for accuracy

## Success Metrics

- The left pane shows all features and bugs at a glance without switching views
- Switching between plans in the left pane immediately updates all right-pane tabs
- Tab 4 Metrics loads within 500ms for a typical `work.jsonl` (< 10k records)
- All existing status command behaviors (approval, delete, task detail, criteria editing, live log) are preserved

## Open Questions

_(none — all resolved during design session)_
