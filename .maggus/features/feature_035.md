<!-- maggus-id: c3c5fc17-99da-48e2-9391-400d25f8bf42 -->
# Feature 035: Documentation Update

## Introduction

The Maggus documentation has drifted from the actual implementation in several areas. Key gaps include: outdated file naming conventions (`plan_*.md` vs the real `feature_*.md` / `bug_*.md` structure), undocumented config fields, stale TUI reference content, no documentation for the bug workflow, and screenshots that predate the screen router refactor.

This feature brings the docs back in sync with the codebase and adds coverage for features that were never documented.

### Architecture Context

- **Components involved:** VitePress docs site in `docs/`, all source `.md` files under `docs/guide/` and `docs/reference/`, and screenshots in `docs/public/screenshots/`
- **No code changes required** — this is a pure documentation update

## Goals

- Fix all references to the old `plan_*.md` naming convention throughout the docs
- Document the actual directory structure: `features/feature_*.md` and `bugs/bug_*.md`
- Document all undocumented `config.yml` fields (approval_mode, auto_continue, max_log_files, git, on_complete, hooks)
- Remove stale "Claude 2x mode" mention from the TUI reference
- Add PgUp/PgDn plan-hopping to the status view keyboard table
- Document the bug workflow briefly, pointing users to `/maggus-bugreport` and `/maggus-plan` skills
- Refresh all 8 TUI screenshots to match the current UI

## Tasks

### TASK-035-001: Fix file naming in getting-started.md and concepts.md
**Description:** As a new user, I want the getting-started guide and concepts page to show the real file paths and directory structure so that I can follow the docs without getting confused.

**Token Estimate:** ~30k tokens
**Predecessors:** none
**Successors:** none
**Parallel:** yes — can run alongside TASK-035-002, TASK-035-003, TASK-035-004, TASK-035-005

**Acceptance Criteria:**
- [x] `docs/guide/getting-started.md`: All references to `plan_*.md` are replaced with `feature_*.md` and the `.maggus/features/` directory
- [x] `docs/guide/getting-started.md`: The "Writing Your First Plan" section shows `feature_001.md` at `.maggus/features/feature_001.md`
- [x] `docs/guide/getting-started.md`: The task heading example uses `TASK-001-001` format (feature-prefixed IDs) if applicable, or is left as `TASK-001` — whatever the actual code accepts
- [x] `docs/guide/concepts.md`: All `plan_*.md` references are updated to `feature_*.md` in `.maggus/features/`
- [x] `docs/guide/concepts.md`: The "Work Loop Lifecycle" step 1 correctly describes loading from `.maggus/features/feature_*.md` and `.maggus/bugs/bug_*.md`
- [x] `docs/guide/concepts.md`: The "Completed Plans" rename example uses the correct path (`feature_N.md` → `feature_N_completed.md`)
- [x] `docs/reference/commands.md`: Any `plan_N.md` references in examples (e.g. in `maggus status` output or `maggus ignore`) are updated to `feature_N.md`

---

### TASK-035-002: Fix file naming in writing-plans.md and add bug workflow
**Description:** As a user learning the plan format, I want the writing-plans reference to show the correct file locations and also explain the bug workflow so that I know where to put my files and which skill to use.

**Token Estimate:** ~40k tokens
**Predecessors:** none
**Successors:** none
**Parallel:** yes — can run alongside TASK-035-001, TASK-035-003, TASK-035-004, TASK-035-005

**Acceptance Criteria:**
- [x] `docs/guide/writing-plans.md`: The "Plan File Location" section is updated — pattern changes from `.maggus/plan_*.md` to `.maggus/features/feature_*.md`, examples updated accordingly
- [x] `docs/guide/writing-plans.md`: A new "Bug Files" section (or equivalent) explains that bug files live in `.maggus/bugs/bug_*.md`, follow the same task format as feature files, and are worked on before features in the work loop
- [x] `docs/guide/writing-plans.md`: The bug section mentions using `/maggus-bugreport` to generate bug files and `/maggus-plan` to generate feature files, rather than writing by hand
- [x] `docs/guide/writing-plans.md`: The "Full Example Plan" uses the correct file path in its intro (`.maggus/features/feature_001.md` or similar)
- [x] `docs/guide/writing-plans.md`: The "Generating Plans Automatically" section references `/maggus-plan` for features and mentions `/maggus-bugreport` for bug files
- [x] `docs/guide/writing-plans.md`: The "Completed Plans" section is updated — rename example uses `feature_N.md` → `feature_N_completed.md` in `.maggus/features/`

---

### TASK-035-003: Fix file naming and add /maggus-bugreport in maggus-plan-skill.md
**Description:** As a user of the Maggus skills, I want the skills reference page to show the correct output file format and also explain the /maggus-bugreport skill so that I can use both workflows.

**Token Estimate:** ~35k tokens
**Predecessors:** none
**Successors:** none
**Parallel:** yes — can run alongside TASK-035-001, TASK-035-002, TASK-035-004, TASK-035-005

**Acceptance Criteria:**
- [x] `docs/guide/maggus-plan-skill.md`: The skills overview table is updated — `/maggus-plan` output column shows `feature_*.md` in `.maggus/features/`
- [x] `docs/guide/maggus-plan-skill.md`: A new row is added to the table for `/maggus-bugreport` → produces `bug_*.md` in `.maggus/bugs/`
- [x] `docs/guide/maggus-plan-skill.md`: The `/maggus-plan` section output format table shows `.maggus/features/feature_*.md` (auto-numbered)
- [x] `docs/guide/maggus-plan-skill.md`: A new `/maggus-bugreport` section is added, explaining: what it produces, how to invoke it (`/maggus-bugreport <description>` in Claude Code), that it generates a structured bug ticket in `.maggus/bugs/bug_*.md`
- [x] `docs/guide/maggus-plan-skill.md`: Any example plan file paths in the page show the correct `features/` subdirectory

---

### TASK-035-004: Update configuration reference with all undocumented fields
**Description:** As a user configuring Maggus, I want the configuration reference to document every supported config field so that I can tune Maggus's behavior without guessing.

**Token Estimate:** ~55k tokens
**Predecessors:** none
**Successors:** none
**Parallel:** yes — can run alongside TASK-035-001, TASK-035-002, TASK-035-003, TASK-035-005
**Model:** sonnet

**Acceptance Criteria:**
- [ ] `docs/reference/configuration.md`: The example `.maggus/config.yml` block at the top is updated to reflect all supported fields (remove `worktree` if it is not in the Config struct; add approval_mode, auto_continue, max_log_files, git section, on_complete section, hooks section)
- [ ] `docs/reference/configuration.md`: `approval_mode` is documented — values `opt-in` (default, features must be approved before work starts) and `opt-out` (all features are worked unless explicitly excluded)
- [ ] `docs/reference/configuration.md`: `auto_continue` is documented — boolean, default false, when true Maggus continues to the next feature automatically instead of stopping after each feature completes
- [ ] `docs/reference/configuration.md`: `max_log_files` is documented — integer, default 50, controls how many run log directories are retained in `.maggus/runs/`
- [ ] `docs/reference/configuration.md`: `git` section is documented with all three sub-fields:
  - `auto_branch` (bool, default true — create feature branches automatically)
  - `protected_branches` (list of branch names, default `[main, master, dev]`)
  - `check_sync` (bool, default true — verify branch is in sync before starting work)
- [ ] `docs/reference/configuration.md`: `on_complete` section is documented — `feature` and `bug` fields accept `"rename"` (default, renames to `_completed.md`) or `"delete"` (deletes the file)
- [ ] `docs/reference/configuration.md`: `hooks` section is documented — `on_feature_complete`, `on_bug_complete`, `on_task_complete` each accept a list of `{ run: "shell command" }` entries that execute at the corresponding lifecycle event
- [ ] `docs/reference/configuration.md`: `worktree` is removed from the YAML config block if it is not a real config field (it is CLI-flag-only per the source code); the worktree behavior is mentioned under `--worktree` CLI flag instead

---

### TASK-035-005: Update tui.md — remove stale 2x mode line, add PgUp/PgDn
**Description:** As a user reading the TUI reference, I want accurate keyboard shortcut tables and no stale feature references so that the docs match what I see on screen.

**Token Estimate:** ~25k tokens
**Predecessors:** none
**Successors:** none
**Parallel:** yes — can run alongside TASK-035-001, TASK-035-002, TASK-035-003, TASK-035-004

**Acceptance Criteria:**
- [ ] `docs/reference/tui.md`: The sentence "When Claude 2x mode is active, the logo and borders turn yellow and a countdown timer is displayed." is removed entirely
- [ ] `docs/reference/tui.md`: The Status View keyboard shortcuts table includes `PgUp` / `PgDn` for switching between plans (added in TASK-032)
- [ ] `docs/reference/tui.md`: No other references to "Claude 2x mode" or "2x" remain on the page
- [ ] `docs/reference/tui.md`: All other keyboard shortcut tables are reviewed for accuracy against the current key bindings

---

### TASK-035-006: [YOU] Take fresh screenshots for all 8 TUI views
**Description:** As a user reading the docs, I want screenshots that match the current UI so that the visual references are useful rather than misleading.

**Token Estimate:** n/a — human task
**Predecessors:** none
**Successors:** none
**Parallel:** yes — can be done alongside or after the agent tasks

**Acceptance Criteria:**
- [ ] Run `maggus` without arguments, take a screenshot of the main menu → save as `docs/public/screenshots/main-menu.png`
- [ ] From the main menu, select **work** to open the sub-menu, take a screenshot → save as `docs/public/screenshots/sub-menu-work.png`
- [ ] Run `maggus work` on a repo with a task, take a screenshot of the **Progress tab** → save as `docs/public/screenshots/work-progress-view.png`
- [ ] Switch to the **Detail tab** during a work run, take a screenshot → save as `docs/public/screenshots/work-detail-view.png`
- [ ] Run `maggus status`, take a screenshot of the status view → save as `docs/public/screenshots/plan-view.png`
- [ ] Press **Enter** on a task in the status view, take a screenshot of the task detail view → save as `docs/public/screenshots/task-detail.png`
- [ ] Navigate to a task with a blocked criterion, press **Tab** to enter criteria mode, take a screenshot of the action picker → save as `docs/public/screenshots/blocked-handling.png`
- [ ] Run `maggus list`, take a screenshot of the list view → save as `docs/public/screenshots/list-view.png`
- [ ] All screenshots are saved at an appropriate terminal size (100+ columns wide) so the UI renders cleanly

## Task Dependency Graph

```
TASK-035-001 ─┐
TASK-035-002 ─┤
TASK-035-003 ─┼─ (all parallel, no dependencies)
TASK-035-004 ─┤
TASK-035-005 ─┤
TASK-035-006 ─┘
```

| Task | Estimate | Predecessors | Parallel | Model |
|------|----------|--------------|----------|-------|
| TASK-035-001 | ~30k | none | yes (with all) | — |
| TASK-035-002 | ~40k | none | yes (with all) | — |
| TASK-035-003 | ~35k | none | yes (with all) | — |
| TASK-035-004 | ~55k | none | yes (with all) | sonnet |
| TASK-035-005 | ~25k | none | yes (with all) | — |
| TASK-035-006 | n/a (human) | none | yes | — |

**Total estimated agent tokens:** ~185k

## Functional Requirements

- FR-1: All doc pages must use `feature_*.md` in `.maggus/features/` and `bug_*.md` in `.maggus/bugs/` — no remaining `plan_*.md` references
- FR-2: The configuration reference must document every field present in the `Config` struct in `src/internal/config/config.go`
- FR-3: `worktree` must not appear as a config.yml field if it is absent from the Config struct; it remains documented only as a CLI flag
- FR-4: The `/maggus-bugreport` skill must be introduced on the skills page with enough detail that a user knows when and how to use it
- FR-5: The TUI reference must not reference "Claude 2x mode" anywhere
- FR-6: The status view keyboard table must include PgUp/PgDn for plan navigation
- FR-7: All 8 screenshot files in `docs/public/screenshots/` must be replaced with current captures

## Non-Goals

- No changes to the VitePress config, theme, or navigation structure
- No new doc pages — only updating existing pages
- No documentation of internal/private API or Go package internals
- No changes to the `index.md` homepage
- No documentation of the `worktree` subcommands beyond what already exists

## Technical Considerations

- All file edits are in `docs/guide/` and `docs/reference/` — plain markdown, no build step required to edit
- The agent should read the current file content before editing to understand surrounding context
- `src/internal/config/config.go` is the authoritative source for all config fields and their defaults
- `src/internal/parser/plan.go` is the authoritative source for file naming conventions and directory layout
- Screenshots are PNG files placed in `docs/public/screenshots/` — filenames must exactly match the `/screenshots/X.png` references already in `tui.md`

## Success Metrics

- A new user following the getting-started guide can create a file at the correct path without 404-style confusion
- Every field in the real `config.yml` can be looked up in the configuration reference
- No "Claude 2x mode" text remains in the docs
- Screenshots visually match the running application
