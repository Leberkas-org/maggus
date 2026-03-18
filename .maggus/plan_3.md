# Plan: Ignore Plans and Tasks

## Introduction

Add an ignore mechanism to Maggus that allows users to exclude entire plan files or individual tasks from the work loop. Ignored items are skipped during `maggus work` (treated like blocked) but remain visible in `maggus status` and `maggus list` with a distinct marker. Ignoring and un-ignoring is done via a new `maggus ignore` / `maggus unignore` CLI command.

## Goals

- Allow entire plan files to be excluded from the work loop by renaming them with an `_ignored` suffix (e.g. `plan_3_ignored.md`)
- Allow individual tasks to be excluded by prefixing their title with `IGNORED` (e.g. `### IGNORED TASK-003: Title`)
- Show ignored plans and tasks in `maggus status` and `maggus list` with a clear visual marker (`[~]`) — never hidden
- Provide `maggus ignore` and `maggus unignore` CLI subcommands to toggle ignore state on plans and tasks
- All ignore state is file-based and requires no external database

## User Stories

### TASK-001: Parser recognizes ignored plan files
**Description:** As a developer, I want the parser to detect `_ignored` plan files so that they are loaded but flagged as ignored.

**Acceptance Criteria:**
- [x] Parser detects plan files matching `plan_*_ignored.md` (in addition to `plan_*.md` and `plan_*_completed.md`)
- [x] Plans loaded from `_ignored` files have an `Ignored: true` field on the plan struct
- [x] All tasks inside an ignored plan are also considered ignored (no per-task override)
- [x] Ignored plans are NOT skipped during load — they are included in the full plan list
- [x] Unit tests cover: ignored plan detected, tasks inside inherit ignored status
- [x] Typecheck/lint passes

### TASK-002: Parser recognizes ignored tasks within a plan
**Description:** As a developer, I want the parser to detect `### IGNORED TASK-NNN: Title` markers so that individual tasks can be excluded.

**Acceptance Criteria:**
- [x] Task title parsing strips the `IGNORED ` prefix and sets `Ignored: true` on the task struct
- [x] The stored task title does NOT include the `IGNORED` prefix (display is handled by the UI layer)
- [x] A task with `IGNORED` prefix inside a non-ignored plan is treated as ignored, not active
- [x] A task without the prefix inside a non-ignored plan is treated as active (no regression)
- [x] Unit tests cover: ignored task detected, non-ignored task unaffected, ignored plan + any task = ignored
- [x] Typecheck/lint passes

### TASK-003: Work loop skips ignored plans and tasks
**Description:** As a user running `maggus work`, I want ignored plans and tasks to be skipped so that the work loop only works on active tasks.

**Acceptance Criteria:**
- [x] `maggus work` skips tasks where `Ignored == true` when searching for the next workable task
- [x] Skipped ignored tasks are logged/printed similarly to blocked tasks ("Skipping TASK-003: ignored")
- [x] If all remaining tasks are ignored or blocked, `maggus work` exits with an appropriate message
- [x] No regression: non-ignored, non-blocked tasks are still picked up correctly
- [x] Typecheck/lint passes

### TASK-004: Status and list commands show ignored items with [~] marker
**Description:** As a user, I want `maggus status` and `maggus list` to display ignored plans and tasks with a `[~]` marker so I always know what is being skipped.

**Acceptance Criteria:**
- [x] `maggus status` renders ignored tasks with `[~]` instead of `[ ]`, `[x]`, or `[!]`
- [x] `maggus status` renders the header of an ignored plan with a `[~]` marker on the plan name
- [x] `maggus list` shows ignored tasks with a `[~]` marker (they are never omitted from the list)
- [x] The `[~]` marker is visually distinct — use a different color or style if the terminal supports it
- [x] Ignored items appear inline in their natural order (not moved to a separate section)
- [x] Typecheck/lint passes

### TASK-005: `maggus ignore` command for plans
**Description:** As a user, I want to run `maggus ignore plan <plan-id>` to exclude an entire plan from the work loop.

**Acceptance Criteria:**
- [x] `maggus ignore plan <plan-id>` renames `plan_<N>.md` → `plan_<N>_ignored.md`
- [x] If the plan is already completed (`_completed.md`) or does not exist, print an error and exit non-zero
- [x] If the plan is already ignored, print a message ("already ignored") and exit zero
- [x] Plan ID can be given as a number (e.g. `3`) and resolved to the correct filename
- [x] Typecheck/lint passes

### TASK-006: `maggus unignore` command for plans
**Description:** As a user, I want to run `maggus unignore plan <plan-id>` to re-include an ignored plan in the work loop.

**Acceptance Criteria:**
- [x] `maggus unignore plan <plan-id>` renames `plan_<N>_ignored.md` → `plan_<N>.md`
- [x] If the plan is not currently ignored, print an error and exit non-zero
- [x] If the plan is completed, print an error ("cannot unignore a completed plan") and exit non-zero
- [x] Typecheck/lint passes

### TASK-007: `maggus ignore` command for tasks
**Description:** As a user, I want to run `maggus ignore task <task-id>` to exclude a single task from the work loop.

**Acceptance Criteria:**
- [x] `maggus ignore task <TASK-NNN>` finds the task heading `### TASK-NNN:` in its plan file and rewrites it to `### IGNORED TASK-NNN:`
- [x] If the task does not exist, print an error and exit non-zero
- [x] If the task is already ignored, print a message ("already ignored") and exit zero
- [x] If the task is inside an `_ignored` plan, print a warning ("plan is already ignored") but still apply the marker
- [x] The file is written back atomically (write to temp, rename)
- [x] Typecheck/lint passes

### TASK-008: `maggus unignore` command for tasks
**Description:** As a user, I want to run `maggus unignore task <task-id>` to re-include an ignored task in the work loop.

**Acceptance Criteria:**
- [ ] `maggus unignore task <TASK-NNN>` finds `### IGNORED TASK-NNN:` and rewrites it to `### TASK-NNN:`
- [ ] If the task does not exist or is not currently ignored, print an error and exit non-zero
- [ ] File is written back atomically
- [ ] Typecheck/lint passes

### TASK-009: Wire up ignore/unignore commands in CLI root
**Description:** As a developer, I want `ignore` and `unignore` registered as top-level cobra commands so that users can discover and run them.

**Acceptance Criteria:**
- [ ] `maggus ignore` and `maggus unignore` appear in `maggus --help`
- [ ] Both commands show usage for `plan <id>` and `task <TASK-NNN>` subcommands
- [ ] Running `maggus ignore` with no subcommand prints usage and exits non-zero
- [ ] Typecheck/lint passes

### TASK-010: TUI list view — toggle task ignore with `alt+i`
**Description:** As a user in the `maggus status` list view, I want to press `alt+i` on a selected task to toggle its ignored state so that I can ignore or unignore tasks without leaving the TUI.

**Acceptance Criteria:**
- [ ] Pressing `alt+i` in the list view calls `parser.ToggleTaskIgnore(task.SourceFile, task.ID)` (or equivalent) to flip the `### IGNORED TASK-NNN:` prefix in the plan file
- [ ] After the file is written, plans are reloaded from disk and the list refreshes in place — the cursor stays on the same logical task if it still exists
- [ ] Ignored tasks show the `~` icon and distinct style in the list (consistent with TASK-004)
- [ ] The footer bar is updated to include `alt+i: ignore/unignore`
- [ ] If the selected plan is an `_ignored` plan, `alt+i` on a task shows a note ("plan is already ignored") but still toggles the heading prefix
- [ ] Typecheck/lint passes

### TASK-011: TUI detail view — show ignored status and toggle with `alt+i`
**Description:** As a user viewing a task's detail screen, I want to see whether the task is ignored and be able to toggle it with `alt+i` so that I have full context and control without going back to the list.

**Acceptance Criteria:**
- [ ] The `Status:` metadata row in `renderDetailContent` shows `"Ignored"` (with a distinct muted/yellow style) when `task.Ignored == true`, in addition to the existing Pending / Blocked / Complete states
- [ ] Pressing `alt+i` in the detail view toggles the task's ignored state (same file rewrite as TASK-010)
- [ ] After toggling, the detail viewport refreshes to reflect the new status without navigating away
- [ ] The detail footer (`detailFooter`) includes `alt+i: ignore/unignore` in its hint line
- [ ] Pressing `alt+i` on a completed task is a no-op with a brief inline note ("cannot ignore a completed task")
- [ ] Typecheck/lint passes

### TASK-012: TUI list view — toggle plan ignore with `alt+p`
**Description:** As a user in the `maggus status` list view, I want to press `alt+p` to toggle the ignored state of the currently selected plan tab so that I can exclude or re-include an entire plan without using the CLI.

**Acceptance Criteria:**
- [ ] Pressing `alt+p` in the list view renames the currently selected plan file: `plan_<N>.md` ↔ `plan_<N>_ignored.md`
- [ ] After the rename, plans are reloaded and the tab bar refreshes; the tab selection stays on the same plan (by filename base, not index)
- [ ] The tab bar label for an ignored plan shows a `~` prefix (e.g. ` ~plan_3 0/5 `) to distinguish it from active plans
- [ ] Pressing `alt+p` on a completed plan is a no-op with no visible error (completed plans cannot be ignored)
- [ ] The footer bar is updated to include `alt+p: ignore/unignore plan`
- [ ] `planInfo` struct gains an `ignored bool` field, set by `parsePlans` when the filename contains `_ignored`
- [ ] Typecheck/lint passes

## Functional Requirements

- FR-1: A plan file named `plan_<N>_ignored.md` is treated as ignored; all tasks inside it are also ignored
- FR-2: A task heading `### IGNORED TASK-NNN: Title` marks that task as ignored regardless of plan state
- FR-3: `maggus work` skips any task where `Ignored == true`, logging the skip reason
- FR-4: `maggus status` and `maggus list` always display ignored plans and tasks with a `[~]` / `~` marker; ignored items are never hidden
- FR-5: `maggus ignore plan <N>` renames the file; `maggus unignore plan <N>` renames it back
- FR-6: `maggus ignore task <TASK-NNN>` rewrites the heading prefix; `maggus unignore task <TASK-NNN>` removes it
- FR-7: Plan lookup by ID resolves the numeric portion of the filename regardless of suffix (`_ignored`, `_completed`, or none)
- FR-8: All file mutations are atomic (write temp file + rename)
- FR-9: In the `maggus status` TUI list view, `alt+i` toggles the ignore state of the currently selected task
- FR-10: In the `maggus status` TUI detail view, `alt+i` toggles the ignore state of the viewed task and shows the ignored status in the `Status:` row
- FR-11: In the `maggus status` TUI list view, `alt+p` toggles the ignore state of the currently selected plan tab (renames the file)

## Non-Goals

- No support for ignoring individual acceptance criteria (checkbox level)
- No `maggus ignore` integration with `maggus work` auto-ignore on repeated failure
- No persistent ignore list outside of the plan files themselves
- No batch ignore (e.g. `maggus ignore plan --all`)
- No separate "Ignored" section in the TUI — ignored items stay inline in their natural position

## Technical Considerations

- The parser already distinguishes `_completed` files by suffix — extend the same pattern to detect `_ignored`
- Task heading regex must be updated to match both `### TASK-NNN:` and `### IGNORED TASK-NNN:`
- The `ignore`/`unignore` commands should live in `cmd/ignore.go` and `cmd/unignore.go`
- File rewrite for task ignore/unignore: read file, replace the matching heading line, write atomically
- Use the existing `gitbranch` / plan file resolution patterns from `cmd/work.go` for plan ID lookup
- `planInfo` struct (in `cmd/status.go`) needs a new `ignored bool` field set by `parsePlans` when the filename contains `_ignored`; this mirrors the existing `completed bool` field
- TUI key bindings follow the existing pattern: `alt+r` (run), `alt+backspace` (delete) — `alt+i` and `alt+p` fit naturally alongside them
- The `renderDetailContent` function in `cmd/detail.go` needs to handle the new `Ignored` status in its `Status:` row; add a new `ignoredStyle` (muted yellow / `styles.Warning` faint) for the label
- After any TUI-triggered file mutation, reload plans via the existing `parsePlans(m.dir)` call and update `m.nextTaskID`/`m.nextTaskFile` — same pattern as the existing delete flow in `updateConfirmDelete`

## Success Metrics

- A plan file suffixed with `_ignored` is never picked as the next workable plan
- An `IGNORED TASK-NNN` heading is never picked as the next workable task
- Both still appear in `maggus status` output with `[~]` / `~` marker — never hidden
- `maggus ignore task TASK-003` + `maggus unignore task TASK-003` round-trips without data loss
- Pressing `alt+i` on a task in the TUI list view toggles its heading prefix and refreshes the view without navigating away
- Pressing `alt+p` on a plan tab in the TUI renames the file and the tab bar immediately reflects the new state

## Open Questions

- Should `maggus status` show a summary count for ignored items (e.g. "2 ignored") in the plan header, similar to how blocked tasks might be counted?
- Should ignoring a task that is already completed be an error or a no-op?
