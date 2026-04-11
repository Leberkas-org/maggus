<!-- maggus-id: fcf08d3a-ea16-433e-be9c-1905444c98db -->
# Feature 043: Hierarchical Branch Naming and Task Skip Status

## Introduction

Two improvements to maggus's workflow: a cleaner branch naming strategy that groups task branches under their parent feature, and a new "skipped" task status that lets users manually exclude tasks from the work loop via the status TUI.

Currently, branches are named `feature/maggustask-NNN` which is flat and doesn't show the relationship between a feature plan and its tasks. The new format `feature/maggus-NNN/task-XXX` makes the hierarchy visible in `git branch` output and groups related branches together.

For task skipping, users currently have no way to say "don't work on this task" without editing the plan file. The new skip status (`[>]` + `SKIPPED:` prefix) provides a clean, reversible way to exclude tasks from the daemon's work queue, controllable from the TUI.

### Architecture Context

- **Vision alignment:** "Human control over what the agent is allowed to work on" — skip extends this from plan-level (approval) to task-level
- **Components involved:**
  - `internal/gitbranch` — branch name generation and creation (naming change)
  - `internal/parser` — task parsing, `Criterion` struct, `IsBlocked`/`IsWorkable` (add skip detection)
  - `cmd/status_update.go` / `cmd/tasklist.go` — keyboard shortcut handling (add skip toggle)
  - `cmd/detail.go` — criteria action picker (add skip action)
  - `cmd/run_parallel.go` — per-task branching in parallel mode (uses new naming)
  - `cmd/status_leftpane.go` / `cmd/status_rightpane.go` — render skip marker
- **New patterns:** `SKIPPED:` prefix for criteria (mirrors `BLOCKED:` pattern); `[>]` display marker

## Goals

- Branch names follow `feature/maggus-NNN/task-XXX` format, grouping task branches under their feature
- Bug branches follow `bugfix/maggus-bug-NNN/task-XXX` format
- Users can toggle skip on a task from the status TUI left pane (`x` key) or from the task detail action picker
- Skipped tasks are excluded from the daemon work loop (treated like blocked)
- Skipped status is reversible — pressing `x` again or unskipping from the action picker restores the task
- Skipped tasks display with `[>]` marker in the TUI and `SKIPPED:` prefix in the plan file

## User Stories

### TASK-043-001: Update branch naming to hierarchical format
**Description:** As a developer, I want task branches nested under their feature branch name so `git branch` output is organized and I can easily see which tasks belong to which feature.

**Token Estimate:** ~45k tokens
**Predecessors:** none
**Successors:** TASK-043-003
**Parallel:** yes — can run alongside TASK-043-002

**Acceptance Criteria:**
- [x] `gitbranch.FeatureBranchName("TASK-038-003")` returns `feature/maggus-038/task-003` (was `feature/maggustask-038-003`)
- [x] `gitbranch.FeatureBranchName("TASK-003")` returns `feature/maggus-003/task-003` (single-segment task ID)
- [x] `gitbranch.BranchName("BUG-001-003")` returns `bugfix/maggus-bug-001/task-003` (was `bugfix/maggus-bug-001-003`)
- [x] `gitbranch.PlanBranchNameFromTaskID("TASK-038-003")` still returns `feature/maggus-038` (unchanged — plan branches keep their current format)
- [x] `gitbranch.EnsureFeatureBranch()` creates branches with the new naming
- [x] `gitbranch.EnsureTaskBranchFromBase()` creates branches with the new naming
- [x] All existing `gitbranch` tests updated to expect new format
- [x] New tests cover edge cases: single-segment IDs, lowercase normalization, bug IDs
- [x] `go vet ./...` and `go test ./...` pass

### TASK-043-002: Add SKIPPED: prefix detection to parser and IsSkipped method
**Description:** As a developer, I want the parser to recognize `SKIPPED:` criteria so the work loop can skip tasks the user has marked.

**Token Estimate:** ~35k tokens
**Predecessors:** none
**Successors:** TASK-043-004, TASK-043-005
**Parallel:** yes — can run alongside TASK-043-001

**Acceptance Criteria:**
- [x] `Criterion` struct gains a `Skipped bool` field
- [x] Parser detects `- [ ] SKIPPED:` and `- [>] SKIPPED:` prefixes on unchecked criteria, setting `Skipped: true`
- [x] `Task` gains an `IsSkipped() bool` method that returns true if ANY criterion has `Skipped == true`
- [x] `Task.IsWorkable()` returns false when `IsSkipped()` is true (skipped tasks are not workable, same as blocked)
- [x] `parser.SkipCriterion(filePath, criterion)` adds `SKIPPED: ` prefix to a criterion and changes `[x]`/`[ ]` to `[>]`
- [x] `parser.UnskipCriterion(filePath, criterion)` removes `SKIPPED: ` prefix and changes `[>]` to `[ ]`
- [x] Existing `FindNextIncomplete` skips tasks where `IsSkipped()` is true
- [x] Unit tests cover: parsing `SKIPPED:` prefix, `IsSkipped()` method, skip/unskip file mutations, interaction with `IsBlocked()` (a task can be both blocked and skipped)
- [x] `go vet ./...` and `go test ./...` pass

### TASK-043-003: Update parallel orchestrator to use new branch naming
**Description:** As a developer, I want the parallel work loop to create branches with the new hierarchical naming so parallel tasks get properly nested branches.

**Token Estimate:** ~25k tokens
**Predecessors:** TASK-043-001
**Successors:** TASK-043-006
**Parallel:** yes — can run alongside TASK-043-004, TASK-043-005

**Acceptance Criteria:**
- [x] `run_parallel.go` `runSingleTask()` creates branches like `feature/maggus-038/task-003` instead of `feature/maggustask-038-003`
- [x] Worktree paths remain at `.maggus/worktrees/TASK-ID/` (only the branch name changes, not the directory)
- [x] Sequential mode (`daemon_keepalive.go`) also uses the new branch naming via `EnsureFeatureBranch()`
- [x] `go vet ./...` and `go test ./...` pass

### TASK-043-004: Add skip toggle keyboard shortcut to status TUI left pane
**Description:** As a user viewing the status TUI, I want to press `x` on a task to toggle its skip status so I can quickly exclude tasks without opening the detail view.

**Token Estimate:** ~40k tokens
**Predecessors:** TASK-043-002
**Successors:** TASK-043-006
**Parallel:** yes — can run alongside TASK-043-003, TASK-043-005

**Acceptance Criteria:**
- [x] Pressing `x` when a task row is selected in the left pane toggles the first unchecked criterion's skip status (adds or removes `SKIPPED:` prefix)
- [x] If the task has no unchecked criteria (already complete), `x` is a no-op
- [x] If the task is already skipped (has a `SKIPPED:` criterion), `x` unskips it (removes the prefix)
- [x] After toggling, the plan file is written immediately and the view refreshes
- [x] A status note briefly shows "task skipped" or "task unskipped"
- [x] Pressing `x` on a feature row (not a task) is a no-op
- [x] `go vet ./...` and `go test ./...` pass
- [x] Unit test verifies `x` key sends the correct skip/unskip action

### TASK-043-005: Add skip action to task detail criteria picker
**Description:** As a user in the task detail view, I want a "Skip" option in the criteria action picker so I can skip individual criteria from the detail view.

**Token Estimate:** ~30k tokens
**Predecessors:** TASK-043-002
**Successors:** TASK-043-006
**Parallel:** yes — can run alongside TASK-043-003, TASK-043-004

**Acceptance Criteria:**
- [x] The criteria action picker in `detail.go` gains a new `criteriaActionSkipTask` action (distinct from the existing `criteriaActionSkip` which means "do nothing")
- [x] When selected, it calls `parser.SkipCriterion()` on the selected criterion
- [x] An "Unskip" action appears instead when the criterion already has `SKIPPED:` prefix
- [x] After the action, the plan file is reloaded and the detail view refreshes
- [x] `go vet ./...` and `go test ./...` pass

### TASK-043-006: Render skipped tasks with [>] marker in TUI
**Description:** As a user, I want skipped tasks to display with a `[>]` marker so I can visually distinguish them from blocked, complete, and pending tasks.

**Token Estimate:** ~25k tokens
**Predecessors:** TASK-043-004, TASK-043-005
**Successors:** TASK-043-007
**Parallel:** no — needs skip functionality working first

**Acceptance Criteria:**
- [ ] Left pane tree: skipped tasks show `>` icon with a distinct style (e.g., dimmed or muted color, similar to how `~` shows for ignored)
- [ ] Right pane task list (Details tab): skipped tasks show `[>]` marker
- [ ] Status summary counts: skipped tasks are counted separately (e.g., "3 done, 2 pending, 1 blocked, 1 skipped")
- [ ] Plain mode output (`--plain`): skipped tasks show `[>]` marker
- [ ] Footer hints include `x: skip` when a task is selected in the left pane
- [ ] `go vet ./...` and `go test ./...` pass

### TASK-043-007: Update docs for new branch naming and skip status
**Description:** As a user reading the docs, I want the documentation to reflect the new branch naming format and the skip feature.

**Token Estimate:** ~20k tokens
**Predecessors:** TASK-043-006
**Successors:** none
**Parallel:** no

**Acceptance Criteria:**
- [ ] `CLAUDE.md` — gitbranch package description updated to mention new naming format
- [ ] `docs/guide/concepts.md` — Git Branch Behavior section updated with new format (`feature/maggus-NNN/task-XXX`)
- [ ] `docs/reference/tui.md` — Status View shortcuts table includes `x` for skip; task markers list includes `[>]` for skipped
- [ ] `docs/guide/writing-plans.md` — documents `SKIPPED:` prefix alongside `BLOCKED:`
- [ ] `ARCHITECTURE.md` — gitbranch row updated if branch naming is mentioned
- [ ] `go vet ./...` and `go test ./...` pass

## Task Dependency Graph

```
TASK-043-001 ──→ TASK-043-003 ──────────────────┐
                                                 ├──→ TASK-043-006 ──→ TASK-043-007
TASK-043-002 ──→ TASK-043-004 ──────────────────┤
             ──→ TASK-043-005 ──────────────────┘
```

| Task | Estimate | Predecessors | Parallel | Model |
|------|----------|--------------|----------|-------|
| TASK-043-001 | ~45k | none | yes (with 002) | — |
| TASK-043-002 | ~35k | none | yes (with 001) | — |
| TASK-043-003 | ~25k | 001 | yes (with 004, 005) | — |
| TASK-043-004 | ~40k | 002 | yes (with 003, 005) | — |
| TASK-043-005 | ~30k | 002 | yes (with 003, 004) | — |
| TASK-043-006 | ~25k | 004, 005 | no | — |
| TASK-043-007 | ~20k | 006 | no | — |

**Total estimated tokens:** ~220k

## Functional Requirements

- FR-1: Feature task branches are named `feature/maggus-NNN/task-XXX` where NNN is the feature number and XXX is the task suffix
- FR-2: Bug task branches are named `bugfix/maggus-bug-NNN/task-XXX`
- FR-3: Plan-level branches remain unchanged (`feature/maggus-NNN`, `bugfix/maggus-bug-NNN`)
- FR-4: The parser recognizes `SKIPPED:` prefix on unchecked criteria (`- [ ] SKIPPED:` and `- [>] SKIPPED:`)
- FR-5: Tasks with any skipped criterion are excluded from the daemon work queue (same as blocked)
- FR-6: Pressing `x` on a task in the status TUI left pane toggles skip status
- FR-7: The task detail criteria action picker includes a "Skip"/"Unskip" action
- FR-8: Skipped tasks display with `>` icon in the left pane and `[>]` in plain mode
- FR-9: Skip is reversible — pressing `x` again or selecting "Unskip" restores the task
- FR-10: The status summary line counts skipped tasks separately from blocked and pending

## Non-Goals

- No auto-skip based on conditions (skip is always manual)
- No skip at the plan/feature level (that's what approval/ignore already covers)
- No migration of existing branches to the new naming format (old branches keep their names)
- No changes to the `maggus clean` command for skipped tasks

## Technical Considerations

- The `/` in branch names like `feature/maggus-038/task-003` creates a directory hierarchy in `.git/refs/heads/` — this is standard git behavior and works fine
- Existing branches on disk won't be renamed — only new branches get the new format. This avoids breaking in-progress work
- The `SKIPPED:` prefix follows the same pattern as `BLOCKED:` for consistency. The `[>]` checkbox marker is non-standard markdown but consistent with how maggus uses `[x]`, `[~]` etc.
- `IsWorkable()` already checks `!IsBlocked()` — adding `!IsSkipped()` to the same condition is the minimal change

## Success Metrics

- `git branch` output for a parallel run shows neatly grouped branches under the feature prefix
- Users can skip/unskip tasks without leaving the TUI or editing plan files
- The daemon correctly ignores skipped tasks and works only on pending/unblocked/unskipped tasks

## Open Questions

None — all questions resolved.
