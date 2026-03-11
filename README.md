# Maggus

<img src="docs/avatar.png" alt="Maggus" width="300">

Your best and worst co-worker at the same time. A junior developer that just works and only asks questions when the work itself is unclear.

Give him an implementation plan and he'll grind through the tasks one by one, prompting an AI agent for each.

## How It Works

1. Create an implementation plan using the **maggus-plan** skill (or write one manually)
2. The plan lives in `.maggus/plan_*.md` and contains tasks in this format:

```markdown
### TASK-001: Do the thing
**Description:** As a user, I want the thing so that it works.

**Acceptance Criteria:**
- [ ] First criterion
- [ ] Second criterion
```

3. Run `maggus work` and Maggus will find the next incomplete task, build a focused prompt, and send it to Claude Code
4. A task is complete when all its acceptance criteria are checked (`[x]`)
5. After each task, Maggus commits the changes and moves to the next one

## Installation

Build from source (requires Go 1.22+):

```bash
cd src
go build -o maggus .
```

Make sure `claude` (Claude Code CLI) is available on your PATH.

### Pre-built binaries

Pre-built binaries for Windows, macOS, and Linux are attached to each [GitHub Release](https://github.com/leberkas-org/maggus/releases).

## Usage

### Work

```bash
# Work on the next 5 tasks (default)
maggus work

# Work on the next 10 tasks
maggus work 10

# Same thing with a flag
maggus work --count 10
maggus work -c 10

# Override the model for this run
maggus work --model opus
maggus work --model sonnet

# Skip reading bootstrap context files
maggus work --no-bootstrap
```

Maggus processes tasks sequentially, one at a time. After each task it commits the changes, re-reads the plan to pick up any updates, then moves to the next incomplete task.

### List

Preview the next tasks without starting any work:

```bash
# Show the next 5 upcoming tasks (default)
maggus list

# Show the next N upcoming tasks
maggus list 10
maggus list --count 10

# Show all upcoming tasks (no count cap)
maggus list --all

# Plain output (no colors)
maggus list --plain
maggus list --all --plain
```

Each task is shown on a single line: `#1  TASK-001: Title`. The first task is highlighted in cyan (color mode). Completed tasks are never shown.

### Status

Get a full overview of task and plan progress:

```bash
# Show status (completed plans hidden by default)
maggus status

# Show all plans, including completed ones
maggus status --all

# Plain output (no colors)
maggus status --plain
maggus status --all --plain
```

Output order: header → summary → task sections → plans table. Completed plans are hidden by default to keep the output focused on active work.

## Configuration

Maggus reads `.maggus/config.yml` from the project root. All fields are optional.

```yaml
# .maggus/config.yml

# Default model to use (short alias or full model ID)
model: sonnet

# Extra markdown files to include in the prompt bootstrap context
include:
  - ARCHITECTURE.md
  - docs/PATTERNS.md
```

### Model aliases

| Alias    | Full model ID                   |
|----------|---------------------------------|
| `sonnet` | `claude-sonnet-4-6`             |
| `opus`   | `claude-opus-4-6`               |
| `haiku`  | `claude-haiku-4-5-20251001`     |

You can also pass a full model ID directly. The `--model` CLI flag overrides the config file.

## Run Logs

Every `maggus work` invocation creates a timestamped run directory under `.maggus/runs/<RUN_ID>/`:

- `run.md` — start/end metadata, branch, model, commit range
- `iteration-01.md`, `iteration-02.md`, … — per-task logs written by the agent

These files are gitignored automatically.

## Blocked Tasks

If a task criterion cannot be completed (missing dependency, needs human input, external blocker), the agent marks it as:

```
- [x] ⚠️ BLOCKED: <original criterion text> — <reason>
```

Maggus treats blocked tasks as complete and skips them in future runs.

## Completed Plans

When all tasks in a plan file are done, Maggus renames it from `plan_N.md` to `plan_N_completed.md` so it is skipped in future runs.

## Project Memory

Each task instructs the agent to maintain `.maggus/MEMORY.md` — a portable project memory file with architecture decisions, build instructions, and conventions. This file is gitignored and intended to be synced separately across machines.

## Startup Safety Pause

Before starting work, Maggus prints a banner showing the model, iteration count, branch, and run ID, then waits 3 seconds. Press `Ctrl+C` during this window to abort before any agent is invoked.

## Graceful Interruption

Press `Ctrl+C` while a task is running to stop after the current task completes. The agent finishes its work, commits, and then Maggus exits cleanly.

## Roadmap

- **Agent choice** — Support for AI agents beyond Claude Code
- **Task management service** — A hosted backend replacing the markdown files, like a Jira board optimized for Maggus to read and for humans to edit, plan, and supervise