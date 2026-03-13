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
| `--agent` | | *(from config)* | Agent backend to use (`claude` or `opencode`) |
| `--model` | | *(from config)* | Model to use (e.g. `opus`, `sonnet`, `anthropic/claude-sonnet-4-6`, or a full model ID) |
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

# Skip bootstrap context files
maggus work --no-bootstrap
```

### Example Output

```
══════════════════════════════════════════
  Maggus Work Session (v1.0.0)
══════════════════════════════════════════
  Agent:        claude
  Model:        claude-opus-4-6
  Iterations:   5
  Branch:       feature/maggustask-042
  Run ID:       20260312-143000
  Run Dir:      .maggus/runs/20260312-143000
  Permissions:  --dangerously-skip-permissions
══════════════════════════════════════════

WARNING: Running with --dangerously-skip-permissions

Press Ctrl+C within 3 seconds to abort...
```

After the 3-second safety window, Maggus enters the TUI and begins working through tasks.

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

---

## maggus blocked

Interactive wizard for managing blocked tasks. Walks through each blocked criterion and lets you choose an action.

### Usage

```bash
maggus blocked
```

This command takes no flags. It scans all active plan files for blocked tasks and presents an interactive picker for each blocked criterion.

### Actions

For each blocked criterion, you can choose:

| Action | Description |
|--------|-------------|
| **Unblock** | Removes the `BLOCKED:` prefix, turning it back into a normal unchecked criterion |
| **Resolve** | Removes the entire criterion line from the plan file |
| **Skip** | Leaves the criterion unchanged and moves to the next one |
| **Abort** | Stops the wizard immediately (changes already made are preserved) |

### Examples

```bash
# Launch the blocked task wizard
maggus blocked
```

### Example Output

```
Found 2 blocked task(s).

Blocked task 1 of 2

──────────────────────────────────────────
 Plan: plan_8.md
 TASK-002: Apply Simpsons-inspired theme

 Acceptance Criteria:
   ✓ VitePress theme config uses Simpsons yellow (#FDD835) as the primary brand color
   ✓ Custom CSS overrides are in docs/.vitepress/theme/custom.css
   >>> ⚠ BLOCKED: Verify in browser using dev-browser skill

   Choose action:

   > Unblock
     Resolve
     Skip
     Abort

   ↑/↓ navigate • enter select • q abort
```

After processing all blocked criteria:

```
──────────────────────────────────────────
 Done. Summary: 1 unblocked, 0 resolved, 1 skipped
```
