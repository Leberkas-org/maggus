# Feature 002: Live Feature & Bug Counts in Main Menu

## Introduction

The main menu currently shows a static feature summary that is loaded once at startup and only refreshed when the user returns from a command. It also ignores bug files entirely. Users must restart maggus or navigate to work/status to see updated counts.

This feature adds live-updating feature and bug statistics to the main menu using `fsnotify` to watch the `.maggus/features/` and `.maggus/bugs/` directories for file additions, removals, and content changes.

## Goals

- Show both feature and bug counts with per-type task breakdowns in the main menu
- Update counts automatically and instantly when feature/bug files are added, removed, or modified
- Use OS-native file system notifications (fsnotify) instead of polling

## Tasks

### TASK-002-001: Add fsnotify dependency
**Description:** As a developer, I want fsnotify available in the project so that file system watching can be used in the menu TUI.

**Token Estimate:** ~10k tokens
**Predecessors:** none
**Successors:** TASK-002-002
**Parallel:** yes — can run alongside TASK-002-003

**Acceptance Criteria:**
- [ ] `github.com/fsnotify/fsnotify` is added to `go.mod` and `go.sum` via `go get`
- [ ] `go build ./...` succeeds
- [ ] `go vet ./...` passes

### TASK-002-002: Implement file watcher for feature and bug directories
**Description:** As a developer, I want a reusable component that watches `.maggus/features/` and `.maggus/bugs/` directories and sends a bubbletea message when files change, so the menu can react to file system events.

**Token Estimate:** ~40k tokens
**Predecessors:** TASK-002-001
**Successors:** TASK-002-003
**Parallel:** no

**Acceptance Criteria:**
- [ ] A new bubbletea `tea.Cmd` or goroutine-based watcher watches both `.maggus/features/` and `.maggus/bugs/` directories
- [ ] The watcher sends a `featureSummaryUpdateMsg` (or similar) to the bubbletea program when any Create, Write, Remove, or Rename event occurs on `feature_*.md` or `bug_*.md` files
- [ ] Events are debounced (e.g., 200-500ms) so rapid successive writes don't trigger multiple re-parses
- [ ] The watcher handles missing directories gracefully (e.g., `.maggus/bugs/` may not exist yet)
- [ ] The watcher is cleaned up when the menu model exits (no leaked goroutines or file handles)
- [ ] Unit tests verify the watcher sends update messages on file changes
- [ ] `go vet ./...` passes

### TASK-002-003: Update menu summary to show live feature and bug counts
**Description:** As a user, I want the main menu to show live feature and bug counts with task breakdowns so I can see project progress at a glance without navigating away.

**Token Estimate:** ~35k tokens
**Predecessors:** TASK-002-002
**Successors:** none
**Parallel:** no

**Acceptance Criteria:**
- [ ] `featureSummary` struct is extended to include bug counts (bugs, bugTasks, bugDone, bugBlocked) alongside existing feature counts
- [ ] `loadFeatureSummary()` parses both features AND bugs (currently only parses features)
- [ ] The summary line format is: `3 features (5 tasks, 3 done) · 2 bugs (4 tasks, 2 done, 1 blocked)` — parts with zero counts for done/blocked are omitted for brevity
- [ ] When a `featureSummaryUpdateMsg` is received in `Update()`, the summary is reloaded by calling `loadFeatureSummary()` and the view re-renders
- [ ] The watcher is started in `Init()` and torn down on menu exit
- [ ] If both features and bugs are zero, the summary line shows a hint like `No features or bugs found`
- [ ] The summary updates within ~500ms of a file change (debounce window)
- [ ] All existing menu tests pass
- [ ] New tests verify the summary line renders correctly for various combinations (features only, bugs only, both, neither)
- [ ] `go vet ./...` passes

## Task Dependency Graph

```
TASK-002-001 ──→ TASK-002-002 ──→ TASK-002-003
```

| Task | Estimate | Predecessors | Parallel | Model |
|------|----------|--------------|----------|-------|
| TASK-002-001 | ~10k | none | yes | haiku |
| TASK-002-002 | ~40k | 001 | no | — |
| TASK-002-003 | ~35k | 002 | no | — |

**Total estimated tokens:** ~85k

## Functional Requirements

- FR-1: The main menu summary must show feature count with task/done/blocked breakdown
- FR-2: The main menu summary must show bug count with task/done/blocked breakdown
- FR-3: The summary must update automatically when feature or bug files are added, removed, or modified
- FR-4: File watching must use OS-native notifications via fsnotify, not polling
- FR-5: Rapid file changes (e.g., saving multiple files in quick succession) must be debounced to avoid excessive re-parsing
- FR-6: The watcher must not leak goroutines or file handles when the menu exits
- FR-7: Missing directories (`.maggus/features/` or `.maggus/bugs/` not yet created) must not cause errors

## Non-Goals

- No watching of subdirectories or non-markdown files
- No live updates in other views (status, work) — only the main menu
- No watching for new directory creation (if `.maggus/bugs/` doesn't exist at startup, it won't be watched until next menu entry)
- No websocket or network-based notifications

## Technical Considerations

- fsnotify watches directories, not glob patterns — the watcher should filter events to only `feature_*.md` and `bug_*.md` filenames in the event handler
- On Windows, `ReadDirectoryChangesW` is used under the hood — this works well but has a known limitation where rename events may arrive as separate remove+create pairs
- The debounce should collapse all events within the window into a single re-parse, not queue them
- `loadFeatureSummary()` currently calls `parseFeatures()` from `status_plans.go` which wraps `parser.GlobFeatureFiles()`. The bug equivalent is `parser.GlobBugFiles()` + `parser.ParseFile()` — follow the same pattern
- The bubbletea `tea.Cmd` pattern (returning commands from Update) is the correct way to schedule async work — avoid spawning unmanaged goroutines where possible

## Success Metrics

- User sees feature and bug counts update in real-time as files are added/modified/removed
- No visible lag or flicker when counts update
- Zero resource leaks (goroutines, file handles) after menu exit

## Open Questions

None — all resolved.
