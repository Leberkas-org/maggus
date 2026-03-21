# Feature 004: Live Task Queue Updates in Work View

## Introduction

Currently the work view's task list is fixed at startup and only re-parsed after each task completes. With the fsnotify file watcher (from feature_002), the work TUI can detect new feature/bug files in real-time and update the progress bar, task counts, and notify the user of incoming work — all without interrupting the current task.

Bugs are prioritized over features (existing behavior). When a new bug file is added mid-task, the user should see an inline notification that it will run next. The actual task reordering happens at the natural task boundary (after the current task finishes), which the work loop already does via `parseAllTasks()`.

### Architecture Context

- **Components involved:** `internal/filewatcher/` (reuse from feature_002), `internal/runner/` (TUI model), `cmd/work_loop.go` + `cmd/work_task.go` (work loop)
- **New patterns:** The work TUI gains a file watcher alongside the existing per-100ms tick, receiving `featureSummaryUpdateMsg` to refresh counters live

## Goals

- Show live-updating task counts and progress bar in the work view header when files change
- Notify the user with a brief fade-out message when new bugs or features are detected
- Count bug tasks as regular tasks in the progress bar, with a hint showing how many bugs are active
- Reuse the `internal/filewatcher/` component from feature_002

## Tasks

### TASK-004-001: Integrate file watcher into the work TUI
**Description:** As a developer, I want the work TUI to watch `.maggus/features/` and `.maggus/bugs/` for file changes so that the view can react to new or modified files in real-time.

**Token Estimate:** ~40k tokens
**Predecessors:** none
**Successors:** TASK-004-002, TASK-004-003
**Parallel:** no

**Acceptance Criteria:**
- [x] `TUIModel` gains a `watcher *filewatcher.Watcher` and `watcherCh chan struct{}` field (same pattern as the menu model)
- [x] The watcher is created in `NewTUIModel()` or via a `SetWatcher()` method, watching the work directory
- [x] `Init()` includes `listenForWatcherUpdate(m.watcherCh)` in its `tea.Batch`
- [x] `Update()` handles a file change message by triggering a live summary re-parse (see TASK-004-002)
- [x] The watcher is cleaned up when the TUI exits (no leaked goroutines or file handles)
- [x] `go vet ./...` passes

### TASK-004-002: Live progress bar and task count updates
**Description:** As a user running `maggus work`, I want the progress bar and task counts to update live when feature or bug files are added or modified, so I can see the current state of the queue without waiting for the next task to start.

**Token Estimate:** ~50k tokens
**Predecessors:** TASK-004-001
**Successors:** none
**Parallel:** yes — can run alongside TASK-004-003

**Acceptance Criteria:**
- [ ] When a file change message is received, the TUI re-parses all feature and bug files to compute updated counts (total tasks, done, blocked, workable)
- [ ] The progress bar (`renderHeaderInner`, line 96-103 of `tui_render.go`) recalculates `totalIters` based on the updated workable task count
- [ ] Bug tasks are counted as regular tasks in the progress bar total (e.g., `3/12 Tasks`)
- [ ] A hint line is shown after the progress bar when bugs are active, e.g., `2 bugs active` in a distinct style (warning/yellow)
- [ ] The hint line disappears when there are no active (workable) bug tasks
- [ ] `currentIter` is NOT changed by live updates — only `totalIters` is adjusted
- [ ] The stop picker's remaining task list is NOT updated live — it only refreshes at task boundaries (existing `sendIterationStart` behavior)
- [ ] Unit tests verify progress bar recalculation with various file change scenarios
- [ ] `go vet ./...` and `go test ./...` pass

### TASK-004-003: Inline notification for new files
**Description:** As a user, I want a brief notification when new bug or feature files are detected so I know work has been queued without having to watch the counters.

**Token Estimate:** ~35k tokens
**Predecessors:** TASK-004-001
**Successors:** none
**Parallel:** yes — can run alongside TASK-004-002

**Acceptance Criteria:**
- [ ] When a file change results in new workable tasks compared to the previous count, a notification message appears in the header area (below the progress bar)
- [ ] For new bugs: notification reads e.g., `+1 bug added (will run next)` styled in warning/yellow
- [ ] For new features: notification reads e.g., `+2 features added` styled in muted/info color
- [ ] If both new bugs and features are detected in the same debounce window, both are shown
- [ ] The notification fades (disappears) after 5 seconds using a delayed `tea.Cmd` message
- [ ] Multiple rapid notifications replace the previous one (don't stack)
- [ ] The notification does not interfere with the stop picker overlay or summary screen
- [ ] Unit tests verify notification appearance and timeout behavior
- [ ] `go vet ./...` and `go test ./...` pass

## Task Dependency Graph

```
TASK-004-001 ──→ TASK-004-002
             └──→ TASK-004-003
```

| Task | Estimate | Predecessors | Parallel | Model |
|------|----------|--------------|----------|-------|
| TASK-004-001 | ~40k | none | no | — |
| TASK-004-002 | ~50k | 001 | yes (with 003) | — |
| TASK-004-003 | ~35k | 001 | yes (with 002) | — |

**Total estimated tokens:** ~125k

## Functional Requirements

- FR-1: The work TUI must watch `.maggus/features/` and `.maggus/bugs/` for file changes using fsnotify (via `internal/filewatcher/`)
- FR-2: The progress bar must recalculate total task count on file changes, combining bugs and features into a single count
- FR-3: A hint line after the progress bar must show the number of active bug tasks when > 0
- FR-4: An inline notification must appear when new workable tasks are detected, distinguishing bugs from features
- FR-5: Bug notifications must say `(will run next)` since bugs are prioritized
- FR-6: Notifications must auto-dismiss after 5 seconds
- FR-7: The stop picker's task list must NOT update live — only at task boundaries
- FR-8: The `currentIter` counter must not change from live updates — only `totalIters` adjusts

## Non-Goals

- No live update of the stop picker's remaining task list (refreshes at task boundaries only)
- No interruption of the current task when new files appear
- No reordering mid-task — task reordering happens after the current task finishes (existing behavior via `parseAllTasks()` in `work_task.go:115`)
- No changes to the summary screen

## Technical Considerations

- The file watcher from feature_002 (`internal/filewatcher/`) uses a debounced channel pattern. The work TUI should use the same `listenForWatcherUpdate` → `featureSummaryUpdateMsg` → re-listen pattern as the menu
- The re-parse on file change should use `parser.ParseFeatures()` + `parser.ParseBugs()` to get workable task counts. This is lightweight (glob + parse markdown) and safe to run on the UI goroutine via a `tea.Cmd`
- To detect "new" tasks for notifications, the TUI needs to store the previous workable count and compare after re-parse
- The fade-out notification uses a delayed `tea.Cmd` with a monotonic ID (same pattern as `hideShortcutsMsg` in the menu) to avoid stale timers hiding newer notifications
- `currentIter` must be protected from live updates — it only increments when the work loop sends `IterationStartMsg`

## Success Metrics

- User sees the progress bar and counts update within ~500ms of adding a new file
- Notification clearly communicates that new bugs will run next
- No flicker, layout jumps, or stale data in the header area

## Open Questions

None — all resolved.
