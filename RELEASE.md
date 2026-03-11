# Release Notes

## What's New

### Compact `maggus list` Output

`maggus list` now shows one line per task — no more description snippets cluttering the output.

```
Next 5 task(s):

 #1  TASK-001: First upcoming task
 #2  TASK-002: Second upcoming task
 #3  TASK-003: Third upcoming task
```

The first task (`#1`) is still highlighted in cyan in color mode. A new `--all` flag shows every upcoming workable task with no count cap, changing the header to `All upcoming tasks:`.

```bash
maggus list --all
maggus list --all --plain
```

### Redesigned `maggus status` Layout

The status output is restructured for clarity: task details appear first, with the Plans table as a summary at the very bottom.

New output order: **Header → Summary → Task sections → Plans table**

### Hide Completed Plans by Default in `maggus status`

Completed plans are now hidden by default to keep the output focused on active work. Pass `--all` to reveal the full history.

```bash
# Default: only active plans shown
maggus status

# Show everything including completed plans
maggus status --all
```

The header count and summary totals always include completed plans in their numbers regardless of the flag, so the overall counts remain accurate.

### Completed Plan Renames Included in Iteration Commits

When Maggus renames a finished plan to `plan_N_completed.md`, that rename is now included in the same commit as the task's changes rather than appearing as a separate untracked file.
