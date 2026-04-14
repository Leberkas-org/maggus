<!-- maggus-id: 50658716-bb9a-4a76-959a-897661a3d1ab -->
# Bug: No way to unblock a task or open its file from the status TUI

## Summary

In the current split-pane status view, there is no accessible shortcut to unblock a blocked task so it can be retried. The only path requires pressing Enter to open a detail overlay, then Tab to enter criteria mode — a two-step flow that is not surfaced in the hint bar when a task is selected in the tree. There is also no shortcut to open the feature/bug file in an editor for manual edits.

## Steps to Reproduce

1. Run `maggus status` with a feature that has a blocked task (criterion prefixed `BLOCKED:`)
2. Navigate to the blocked task row in the left-pane tree
3. Observe: `alt+r: run` is shown in the hint bar, but pressing it silently no-ops (`handleAltRunDispatch` returns early when the task is blocked)
4. Observe: the hint bar does not mention any unblock action
5. Press Enter → the detail overlay opens with "tab: manage blocked" visible — this is the only path, but it is hidden behind two keystrokes and undocumented from the tree view
6. There is no key to open the plan file in an editor for ad-hoc manual fixes

## Expected Behavior

- A shortcut (e.g. `b`) should be available directly from the tree when a blocked task is selected, opening criteria mode in one step.
- A shortcut (e.g. `e`) should open the selected feature/bug file in `$EDITOR` / `$VISUAL` (fallback: `notepad.exe` on Windows) for manual editing.
- The hint bar should advertise both keys when applicable.

## Root Cause

**Unblock inaccessibility** — three contributing locations:

1. `src/cmd/status_update.go:893-894` — `alt+r` dispatches `handleAltRunDispatch` for the selected task. `handleAltRunDispatch` (`status_update.go:544`) explicitly no-ops when the task is blocked:
   ```go
   // No-ops when: task is nil, complete, blocked, or already running in a worker.
   ```
   There is no fallback that opens the unblock flow instead.

2. `src/cmd/status_view.go:225-228` — the hint bar emits `x: skip/unskip` and `alt+r: run` when a task is selected, but never mentions how to unblock. The "tab: manage blocked" hint (`status_view.go:201`) is only rendered when the `ShowDetail` overlay is already open — the user must discover the Enter → Tab path on their own.

3. `src/cmd/tasklist.go:257` — `b` already works as an alias for Tab in the detail overlay, but this key is not bound in the tree-level `updateList` handler, so it does nothing when the detail is closed.

**Open-in-editor shortcut** — there is no key binding for launching an external editor. The plan file path is readily available via `m.featureFilePath(m.selectedPlan())` and `m.selectedTask().SourceFile`.

## User Stories

### BUG-052-001: Add `b` shortcut to open criteria/unblock mode directly from the tree

**Description:** As a user, I want to press `b` on a blocked task row in the tree so that I can unblock it and retry without needing to first open the detail overlay.

**Acceptance Criteria:**
- [x] In `updateList` (`status_update.go`), when the key is `b` and `m.selectedTask() != nil` and the task has at least one blocked criterion, open the detail overlay and immediately enter criteria mode (same effect as Enter followed by Tab)
- [x] If the task has no blocked criteria, `b` is a no-op (or shows a brief "no blocked criteria" note)
- [x] The hint bar (`status_view.go`) shows `b: unblock` when a blocked task is selected in the tree and the detail overlay is not open
- [x] The existing Enter → Tab path continues to work unchanged
- [x] `go vet ./...` and `go test ./...` pass

### BUG-052-002: Add `e` shortcut to open the plan file in `$EDITOR`

**Description:** As a user, I want to press `e` on any plan or task row in the tree so that the feature/bug file opens in my configured editor for manual changes.

**Acceptance Criteria:**
- [x] In `updateList`, pressing `e` opens the plan file for the selected item in `$EDITOR` (falling back to `$VISUAL`, then `notepad.exe` on Windows, then `vi` on Unix) using `tea.ExecProcess` (or `exec.Command` outside the TUI alt-screen, similar to how `alt+r` spawns the agent)
- [x] When a task row is selected, the file opened is the task's `SourceFile` (the plan file containing that task)
- [x] When a plan row is selected, the file opened is the plan's `File` field
- [x] After the editor exits, the TUI resumes and triggers a plan reload (filewatcher debounce or explicit `reloadPlans`)
- [x] If no editor is found/configured, a brief status note is shown (e.g. "set $EDITOR to enable")
- [x] The hint bar shows `e: edit file` when any plan or task is selected
- [x] `go vet ./...` and `go test ./...` pass
