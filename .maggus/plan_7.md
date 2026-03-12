# Plan: Maggus Blocked Command — Interactive Blocked Task Wizard

## Introduction

When maggus encounters blocked tasks (criteria containing `BLOCKED:`), there is currently no way to resolve them without manually editing the plan markdown files. The `maggus blocked` command provides an interactive step-by-step wizard that walks the user through each blocked task, shows full context (plan name, task details, all criteria), and lets the user either **unblock** (remove the `BLOCKED:` prefix so maggus will attempt it) or **resolve** (delete the criterion entirely). The user can abort at any time.

## Goals

- Provide a user-friendly way to manage blocked tasks without hand-editing markdown
- Show enough context per blocked task that the user can make an informed decision
- Allow batch processing: handle multiple blocked items in one session
- Support aborting at any point without leaving the plan files in a broken state

## User Stories

### TASK-001: Add `maggus blocked` cobra command skeleton
**Description:** As a developer, I want a new `blocked` subcommand registered with cobra so that `maggus blocked` is a valid CLI invocation.

**Acceptance Criteria:**
- [x] New file `src/cmd/blocked.go` with a cobra command registered on `rootCmd`
- [x] `maggus blocked` prints a message and exits if no blocked tasks are found
- [x] `maggus blocked --help` shows a short description of the wizard
- [x] Typecheck/lint passes (`go vet ./...`)
- [x] Unit tests are written and successful

### TASK-002: Collect and group blocked tasks by plan
**Description:** As a developer, I want to parse all active plans and collect every blocked task with its plan filename so the wizard knows what to present.

**Acceptance Criteria:**
- [x] Reuses `parser.ParsePlans(dir)` to get all tasks
- [x] Filters to only tasks where `task.IsBlocked()` is true
- [x] Each blocked task retains its `SourceFile` so the wizard can display the plan name and later modify the correct file
- [x] The list is ordered by plan file name, then by document order within each plan (same order as `parser.ParsePlans` returns)
- [x] If no blocked tasks exist, prints "No blocked tasks found." and exits cleanly
- [x] Typecheck/lint passes
- [x] Unit tests are written and successful

### TASK-003: Build the blocked task detail view
**Description:** As a user, I want to see the full context of a blocked task — plan name, task ID, title, description, and all acceptance criteria (with blocked ones highlighted) — so I can make an informed decision.

**Acceptance Criteria:**
- [ ] The detail view shows: plan filename (e.g. `Plan: plan_6.md`), task ID and title, full description text, and all acceptance criteria
- [ ] Completed criteria `[x]` are shown in green with a checkmark
- [ ] Blocked criteria `[ ] BLOCKED: ...` are shown in red and clearly highlighted (e.g. with a `⚠` icon or `>>>` marker)
- [ ] Normal unchecked criteria `[ ]` are shown in default color
- [ ] The view fits within the terminal width (long lines are wrapped or truncated)
- [ ] Typecheck/lint passes

### TASK-004: Build the action picker for each blocked criterion
**Description:** As a user, I want to choose what to do with each blocked criterion — unblock it, resolve (remove) it, skip it, or abort the wizard entirely.

**Acceptance Criteria:**
- [ ] For each blocked criterion in the current task, the wizard presents the criterion text and an action menu
- [ ] Available actions: **Unblock** (remove `BLOCKED:` prefix from the criterion text, keep it as unchecked `- [ ]`), **Resolve** (delete the entire criterion line from the file), **Skip** (leave this criterion unchanged, move to next), **Abort** (quit the wizard immediately)
- [ ] Actions are selectable via arrow keys and Enter (using an interactive picker — bubbletea list or survey-style prompt)
- [ ] If the user selects Abort, the wizard exits immediately; any changes already written to disk in previous steps are preserved (each file write is atomic per criterion)
- [ ] Typecheck/lint passes

### TASK-005: Implement file modification for unblock and resolve actions
**Description:** As a developer, I want the plan markdown file to be updated on disk when the user unblocks or resolves a criterion, so that the change is immediately reflected.

**Acceptance Criteria:**
- [ ] **Unblock**: reads the plan file, finds the exact line `- [ ] ...BLOCKED: ...`, removes the `BLOCKED: ` substring (keeping the rest of the criterion text), writes the file back
- [ ] **Resolve**: reads the plan file, finds the exact criterion line, deletes the entire line, writes the file back
- [ ] File modifications are atomic: read entire file → modify in memory → write entire file. No partial writes
- [ ] After modifying, the file is still valid markdown parseable by `parser.ParseFile`
- [ ] If the criterion line cannot be found in the file (e.g. concurrent edit), an error is shown and the wizard continues to the next item
- [ ] Typecheck/lint passes
- [ ] Unit tests are written and successful (test both unblock and resolve on sample plan markdown)

### TASK-006: Wire the wizard loop — step through all blocked tasks
**Description:** As a user, I want the wizard to walk me through all blocked tasks one by one, showing the detail view and action picker for each, until all are handled or I abort.

**Acceptance Criteria:**
- [ ] After handling all blocked criteria in one task, the wizard moves to the next blocked task
- [ ] A progress indicator shows which blocked task the user is on (e.g. "Blocked task 2 of 5")
- [ ] After each action (unblock/resolve/skip), the screen is refreshed to show the updated state of the current task's criteria
- [ ] When all blocked tasks have been processed, the wizard prints a summary: N unblocked, N resolved, N skipped
- [ ] Abort at any point prints the same summary for actions taken so far
- [ ] Typecheck/lint passes
- [ ] Unit tests are written and successful

### TASK-007: End-to-end test and polish
**Description:** As a developer, I want to verify the full `maggus blocked` flow works correctly and handles edge cases.

**Acceptance Criteria:**
- [ ] `maggus blocked` with no blocked tasks prints "No blocked tasks found." and exits 0
- [ ] `maggus blocked` with one blocked task shows the detail view and action picker correctly
- [ ] `maggus blocked` with multiple blocked tasks across multiple plans walks through all of them
- [ ] Aborting mid-wizard preserves changes already made and prints a partial summary
- [ ] After unblocking a task, `maggus status` no longer shows it as blocked
- [ ] After resolving a criterion, the line is gone from the plan file
- [ ] `go build ./...` succeeds
- [ ] `go vet ./...` passes
- [ ] `go test ./...` passes

## Functional Requirements

- FR-1: `maggus blocked` must parse all active (non-completed) plan files to find blocked tasks
- FR-2: Each blocked task must be presented with its full context: plan filename, task ID, title, description, all criteria
- FR-3: For each blocked criterion, the user must choose one of: Unblock, Resolve, Skip, Abort
- FR-4: **Unblock** removes the `BLOCKED:` prefix from the criterion text, leaving it as an unchecked `- [ ] <remaining text>` line
- FR-5: **Resolve** deletes the entire criterion line from the plan file
- FR-6: File writes are atomic (full file read-modify-write), so a crash mid-wizard never produces a corrupt file
- FR-7: **Abort** exits immediately, preserving all changes already written to disk
- FR-8: A summary of actions taken is printed on exit (whether normal completion or abort)

## Non-Goals

- No undo/rollback of actions taken during the wizard (the user can use `git checkout` if needed)
- No editing of criterion text (only unblock or delete) — manual editing can be done in an editor
- No batch "unblock all" or "resolve all" shortcut — each criterion is handled individually
- No changes to how `maggus work` handles blocked tasks — it still skips them as before

## Technical Considerations

- Plan 6 introduces bubbletea as a dependency. The action picker can use bubbletea's list component or a simple key-press handler for the 4 options
- File modification must match the exact line from the parser. Use `Criterion.Text` plus the checkbox prefix to reconstruct the full line for matching
- The `parser.Task.SourceFile` field already stores the absolute path to the plan file, which is needed for file modification
- Consider using `strings.Replace` on the full file content with count=1 to ensure only the target line is modified
- Terminal colors should match the existing palette used in `cmd/status.go` (green for complete, red for blocked, cyan for active)

## Success Metrics

- A user can resolve all blocked tasks in under 30 seconds per task (no file editing needed)
- The wizard never corrupts a plan file
- After running `maggus blocked`, `maggus work` picks up the newly unblocked tasks immediately

## Open Questions

- Should `maggus blocked` also show tasks that are complete but were previously blocked (i.e. have checked `[x] BLOCKED:` criteria)? Probably not — only active blockers matter.
- Should there be a `--dry-run` flag that shows what would change without writing to disk?
