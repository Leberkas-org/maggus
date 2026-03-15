# CLI Commands

Maggus provides four commands for working with implementation plans. All commands that load configuration will show the configured agent (defaults to `claude`).

## maggus work

The main command. Parses plan files, finds the next workable task, builds a prompt with project context, invokes Claude Code, and commits the result. Repeats until the count is reached or all tasks are done.

### Usage

```bash
maggus work [count] [flags]
```

### Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--count` | `-c` | `5` | Number of tasks to work on |
| `--task` | | | Run a specific task by ID (e.g. `TASK-001`). Sets count to 1. |
| `--agent` | | *(from config)* | Agent backend to use (`claude` or `opencode`) |
| `--model` | | *(from config)* | Model to use (e.g. `opus`, `sonnet`, `anthropic/claude-sonnet-4-6`, or a full model ID) |
| `--worktree` | | *(from config)* | Run in an isolated git worktree |
| `--no-worktree` | | `false` | Force disable worktree mode (overrides config) |
| `--no-bootstrap` | | `false` | Skip reading CLAUDE.md, AGENTS.md, PROJECT_CONTEXT.md, and TOOLING.md |

The positional `[count]` argument and `--count` flag are interchangeable. The positional argument takes precedence.

### Examples

```bash
# Work on the next 5 tasks (default)
maggus work

# Work on the next 10 tasks
maggus work 10

# Work on 3 tasks using the --count flag
maggus work -c 3

# Override the model for this run
maggus work --model opus

# Use OpenCode as the agent backend
maggus work --agent opencode

# Run a specific task
maggus work --task TASK-003

# Run in an isolated worktree
maggus work --worktree

# Skip bootstrap context files
maggus work --no-bootstrap
```

### TUI

The work view displays inside a bordered full-screen box. A **tab bar** lets you switch between four views:

| Tab | Key | Content |
|-----|-----|---------|
| **Progress** | `1` | Live status, recent tool list, model, elapsed time, token usage |
| **Detail** | `2` | Scrollable structured log of every tool invocation with timestamps and parameters |
| **Task** | `3` | Current task description and acceptance criteria with completion status |
| **Commits** | `4` | List of commits made during the run |

Switch tabs with `←/→` arrow keys or number keys `1`–`4`. The Detail tab supports `↑/↓/Home/End` scrolling.

### Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `←/→` or `1-4` | Switch tabs |
| `↑/↓` | Scroll detail log (on Detail tab) |
| `Home/End` | Jump to top/bottom of detail log |
| `Alt+S` | Stop after current task (with confirmation) |
| `Alt+S` (when stopping) | Cancel the stop and resume |
| `Ctrl+C` | Interrupt immediately |

### Stop After Task

Press **Alt+S** during execution to request a graceful stop after the current task completes. A confirmation prompt appears: `Stop after current task? (y/n)`. When active, the box border turns yellow as a visual indicator. Press **Alt+S** again to revert and continue.

### Summary Screen

After all tasks complete (or the run is stopped/interrupted), a summary screen shows run details, token usage per task, commits, and remaining tasks. The title reflects the stop reason:

| Title | When |
|-------|------|
| **✓ Work Complete** | All requested tasks finished successfully |
| **⊘ Stopped by User** | User pressed Alt+S to stop after a task |
| **⊘ Work Interrupted** | User pressed Ctrl+C during execution |
| **✗ Work Failed** | A task or commit failed (shows error detail) |
| **⊘ No Tasks Available** | No workable tasks found |

From the summary screen you can choose **Exit** or **Run again** (with a custom task count).

---

## maggus list

Preview upcoming workable tasks without running them. Only shows incomplete, non-blocked tasks.

### Usage

```bash
maggus list [N] [flags]
```

### Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--count` | `-c` | `5` | Number of tasks to show |
| `--all` | | `false` | Show all upcoming workable tasks (ignores `--count`) |
| `--plain` | | `false` | Strip colors and use ASCII characters for scripting/piping |

The positional `[N]` argument overrides `--count`. It is ignored when `--all` is set.

### Examples

```bash
# Show the next 5 workable tasks (default)
maggus list

# Show the next 3 tasks
maggus list 3

# Show all workable tasks
maggus list --all

# Plain output for scripting
maggus list --plain
```

### Example Output

```
Next 5 task(s):

 #1  TASK-005: Create "CLI Commands" reference page
 #2  TASK-006: Create "Configuration" reference page
 #3  TASK-007: Create "Concepts" page covering run logs, memory, and TUI
 #4  TASK-008: Configure sidebar navigation and header nav
 #5  TASK-009: Add GitHub Actions workflow for deployment on release
```

The first task is highlighted in cyan (unless `--plain` is used).

In TUI mode (without `--plain`), the list is displayed in a full-screen bordered view with keyboard navigation. If no pending tasks exist, a friendly "All done!" screen is shown instead of exiting immediately.

---

## maggus status

Show a compact summary of plan progress including task counts, progress bars, per-plan breakdowns, and the configured agent.

### Usage

```bash
maggus status [flags]
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--all` | `false` | Show completed (archived) plans in task sections and the plans table |
| `--plain` | `false` | Strip colors and use ASCII characters for scripting/piping |

### Examples

```bash
# Show status of active plans
maggus status

# Include completed plans
maggus status --all

# Plain output for CI or scripting
maggus status --plain
```

### Example Output

```
Maggus Status — 3 plans (2 active), 24 tasks total
 Agent: claude

 Summary: 18/24 tasks complete · 4 pending · 2 blocked

 Tasks — plan_8.md
 ──────────────────────────────────────────
   ✓  TASK-001: Scaffold VitePress project in docs/
   ✓  TASK-002: Apply Simpsons-inspired theme
   ✓  TASK-003: Create "Getting Started" guide
   ✓  TASK-004: Create "Writing Plans" documentation page
 → o  TASK-005: Create "CLI Commands" reference page
   o  TASK-006: Create "Configuration" reference page

 Plans
 ──────────────────────────────────────────
   plan_7_completed.md              [██████████]  7/7   done
   plan_8.md                        [████░░░░░░]  4/10  in progress
```

- `✓` = completed task, `⚠` = blocked task, `o` = pending task
- `→` marks the next task that `maggus work` will pick up
- With `--plain`, symbols are replaced: `[x]` for done, `[!]` for blocked, `->` for next

In TUI mode (without `--plain`), the status is displayed in a full-screen bordered view with tabbed plan sections, keyboard navigation, and a detail view for individual tasks. If no plans exist, a helpful empty state screen is shown with a hint to run `maggus plan`.

### Managing Blocked Tasks

Blocked tasks can be managed directly from the task detail view in both `maggus status` and `maggus list`. When viewing a task with blocked criteria:

1. Press **Enter** on a task to open its detail view
2. Press **Enter** again to enter **criteria mode** — blocked criteria are highlighted
3. Navigate between blocked criteria with **↑/↓**
4. Press **Enter** on a blocked criterion to open the **action picker**

The action picker offers four options:

| Action | Description |
|--------|-------------|
| **Unblock** | Removes the `BLOCKED:` prefix, turning it back into a normal unchecked criterion |
| **Resolve** | Marks the criterion as done (removes the block and checks it) |
| **Delete** | Removes the criterion entirely from the plan file |
| **Skip** | Leaves the criterion unchanged |

Changes are applied immediately to the plan file. Press **Esc** at any point to go back.

---
