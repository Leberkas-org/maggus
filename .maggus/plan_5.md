# Plan: Compact List Output & Redesigned Status Layout

## Introduction

Two UX improvements to `maggus list` and `maggus status`.

The `list` command currently shows a noisy description snippet below each task title. The fix: one line per task.

The `status` command currently shows the Plans progress table at the top, followed by all task details. The new layout flips this: task details come first (so you see actual work immediately), and the Plans table sits at the very bottom as a summary. Completed plans are hidden by default to keep the output focused — they are only shown with `--all`.

## Goals

- Make `maggus list` compact: one line per task, no description snippet
- Restructure `maggus status` so task details appear before the Plans table
- Move the Plans table to the very bottom of the output
- Hide completed plans by default; show them only with `--all`
- Keep completed and pending tasks visible within active plans

## User Stories

### TASK-501: Remove description line from `maggus list` output; add `--all` flag
**Description:** As a developer, I want `maggus list` to show only the task ID and title per line, and optionally show all upcoming workable tasks without a count limit using `--all`.

**Acceptance Criteria:**
- [x] Each task is rendered on a single line: `#1  TASK-001: Title`
- [x] No description/snippet line is printed below the task title
- [x] No blank lines between task entries
- [x] Without `--all`: only the next N workable tasks are shown (default N=5)
- [x] With `--all`: all workable tasks are shown with no count cap; completed tasks are not shown
- [x] With `--all`: the header reads `All upcoming tasks:` instead of `Next N task(s):`
- [x] With `--all`: the `--count` / positional `N` argument is ignored
- [x] The first task (#1) is still highlighted in cyan (color mode)
- [x] The `--plain` and `--all` flags can be combined
- [x] Typecheck/lint passes
- [x] Unit tests are written and successful

### TASK-502: Restructure `maggus status` layout — Tasks first, Plans at bottom
**Description:** As a developer, I want `maggus status` to show task details before the Plans table so that I see actual work immediately, with the Plans table as a summary at the very bottom.

**Acceptance Criteria:**
- [x] Output order is: Header → Summary → Task sections → Plans table
- [x] The current/active plan's task section appears directly above the Plans table
- [x] Within each active plan, both completed (`✓`) and pending (`o`) tasks are shown
- [x] The Plans table is the last thing printed
- [x] The `--plain` flag still works correctly
- [x] Typecheck/lint passes

### TASK-503: Hide completed plans by default; add `--all` flag
**Description:** As a developer, I want completed plans to be hidden by default so that the output stays focused on active work, with `--all` to reveal the full history.

**Acceptance Criteria:**
- [ ] Without `--all`: completed plan task sections are not printed
- [ ] Without `--all`: completed plan rows in the Plans table are not printed
- [ ] Without `--all`: the header count still reflects all plans (e.g. "4 plans (1 active)")
- [ ] Without `--all`: summary totals still include completed plan tasks in the numbers
- [ ] With `--all`: completed plan task sections are printed before the active plan sections
- [ ] With `--all`: completed plan rows appear in the Plans table (below the active plan rows)
- [ ] The `--plain` flag and `--all` flag can be combined
- [ ] Typecheck/lint passes

## Design Mockups

### TASK-501 — `maggus list` (before → after)

**Before** (current): two lines per task — title + dimmed description snippet

```
Next 5 task(s):

 #1  TASK-501: Remove description line from list output
     As a developer, I want `maggus list` to show only the task ID...

 #2  TASK-502: Restructure status layout
     As a developer, I want `maggus status` to show task details...

 #3  TASK-503: Hide completed plans by default
     As a developer, I want completed plans to be hidden by default...

```

**After — default**: one line per task, only workable tasks, no blank lines between entries

```
Next 5 task(s):

 #1  TASK-501: Remove description line from list output
 #2  TASK-502: Restructure status layout
 #3  TASK-503: Hide completed plans by default
```

`#1` is still highlighted in cyan in color mode.

**After — with `--all`**: all workable tasks shown with no count cap, same format

```
All upcoming tasks:

 #1  TASK-501: Remove description line from list output
 #2  TASK-502: Restructure status layout
 #3  TASK-503: Hide completed plans by default
 #4  TASK-504: Some further task
 #5  TASK-505: And another one
 #6  TASK-506: ...
```

Note: The header changes from `Next N task(s):` to `All upcoming tasks:`. Completed tasks are not shown. `--count` / positional `N` is ignored.

---

### TASK-502 + TASK-503 — `maggus status` (before → after)

**Before** (current): Plans table at top, task details below, completed plans always visible

```
Maggus Status — 4 plans (1 active), 40 tasks total

 Plans
 ──────────────────────────────────────────
 ✓ plan_1_completed.md        [██████████]  6/6    done
 ✓ plan_2_completed.md        [██████████]  9/9    done
 ✓ plan_3_completed.md        [██████████]  8/8    done
    plan_4.md                 [████░░░░░░]  8/14   in progress

 Summary: 31/40 tasks complete · 9 pending · 0 blocked

 Tasks — plan_1_completed.md (archived)
 ──────────────────────────────────────────
   ✓  TASK-101: First task
   ✓  TASK-102: Second task

 Tasks — plan_2_completed.md (archived)
 ──────────────────────────────────────────
   ✓  TASK-201: ...

 Tasks — plan_3_completed.md (archived)
 ──────────────────────────────────────────
   ✓  TASK-301: ...

 Tasks — plan_4.md
 ──────────────────────────────────────────
 → o  TASK-401: First workable task
   o  TASK-402: Second task
   ✓  TASK-403: Completed task
```

---

**After — default (no flags)**: only active plans shown, task details first, Plans table at bottom

```
Maggus Status — 4 plans (1 active), 40 tasks total

 Summary: 31/40 tasks complete · 9 pending · 0 blocked

 Tasks — plan_4.md
 ──────────────────────────────────────────
 → o  TASK-401: First workable task
   o  TASK-402: Second task
   ✓  TASK-403: Completed task

 Plans
 ──────────────────────────────────────────
    plan_4.md                 [████░░░░░░]  8/14   in progress
```

Completed plans are hidden. The Plans table shows only active plans.

---

**After — with `--all`**: completed plan sections appear above the active plan, Plans table at bottom shows everything

```
Maggus Status — 4 plans (1 active), 40 tasks total

 Summary: 31/40 tasks complete · 9 pending · 0 blocked

 Tasks — plan_1_completed.md (archived)
 ──────────────────────────────────────────
   ✓  TASK-101: First task
   ✓  TASK-102: Second task

 Tasks — plan_2_completed.md (archived)
 ──────────────────────────────────────────
   ✓  TASK-201: ...

 Tasks — plan_3_completed.md (archived)
 ──────────────────────────────────────────
   ✓  TASK-301: ...

 Tasks — plan_4.md
 ──────────────────────────────────────────
 → o  TASK-401: First workable task
   o  TASK-402: Second task
   ✓  TASK-403: Completed task

 Plans
 ──────────────────────────────────────────
 ✓ plan_1_completed.md        [██████████]  6/6    done
 ✓ plan_2_completed.md        [██████████]  9/9    done
 ✓ plan_3_completed.md        [██████████]  8/8    done
    plan_4.md                 [████░░░░░░]  8/14   in progress
```

The Plans table follows the same chronological order as the task sections: oldest plans at the top, active plan at the bottom.

---

## Functional Requirements

- FR-1: `maggus list` output lines must match the format `#N  TASK-XXX: Title` with no additional lines per task entry and no blank lines between tasks
- FR-2: The `#1` task in `maggus list` must retain its cyan highlight in color mode
- FR-3: `maggus list --all` must show all workable tasks across all active plans with no count cap; completed tasks are not shown
- FR-4: When `--all` is active, `--count` and the positional `N` argument are ignored and the header reads `All upcoming tasks:` instead of `Next N task(s):`
- FR-5: `maggus status` output order must be: Header line → Summary line → Task sections → Plans table
- FR-6: The active/current plan's task section is always the last task section printed, directly above the Plans table
- FR-7: Both completed (`✓`) and pending (`o`) tasks are shown within any displayed plan's task section
- FR-8: Without `--all`, completed plan task sections are not printed and completed plan rows are not shown in the Plans table
- FR-9: Without `--all`, the header line and summary totals still count all plans and tasks (including completed ones) so the numbers are accurate
- FR-10: With `--all`, both the task sections and the Plans table rows follow the same chronological order: completed plans oldest-first at the top, active plan at the bottom
- FR-11: The `--all` and `--plain` flags can be combined freely on both commands
- FR-12: Within each plan, task order is unchanged (not reversed)

## Non-Goals

- No changes to the `maggus work` command
- No changes to how tasks are parsed or how completion is detected
- No changes to the task row format within a task section (icons, indentation, blocked display remain the same)
- No per-task filtering — if a plan is shown, all its tasks are shown

## Technical Considerations

- In `src/cmd/list.go`:
  - Remove the description snippet block (lines 103–111 in current code) and the trailing `fmt.Println()` between tasks
  - Add `--all` boolean flag (default `false`)
  - When `--all`: skip the `workable = workable[:count]` cap; change header to `All upcoming tasks:`; ignore `--count` and positional `N`
- In `src/cmd/status.go`:
  - Add `--all` boolean flag (default `false`)
  - Change rendering order: print Summary, then task sections, then Plans table
  - When iterating for task sections and Plans table rows: if `!all && p.completed`, skip
  - Plans table and task sections both use the same chronological order (natural sort); no reversal needed
  - Summary totals loop must iterate ALL plans regardless of `--all` flag

## Success Metrics

- `maggus list` output is one line per task — confirmed visually
- `maggus status` (default) shows only the active plan task section, followed by the Plans table — confirmed visually
- `maggus status --all` reveals all archived plan sections above the active plan — confirmed visually

## Open Questions

- If there are multiple active plans (no `_completed` suffix), should they all appear in the default view or only the "current" one (the one containing the next workable task)?
