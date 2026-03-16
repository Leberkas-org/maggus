# Concepts

This page explains Maggus's runtime behavior — what happens when you run `maggus work`, what gets logged, and how to interact with the TUI.

## Work Loop Lifecycle

When you run `maggus work`, Maggus enters a loop that processes tasks one at a time:

1. **Parse** — Load all active plan files (`.maggus/plan_*.md`), skipping completed (`_completed.md`) files
2. **Find task** — Identify the next workable task (incomplete and not blocked) across all plans
3. **Branch** — If on a protected branch (`main`, `master`, `dev`), create a feature branch
4. **Prompt** — Assemble the prompt with bootstrap context files, run metadata, and task details
5. **Run** — Invoke the configured agent as a subprocess with the assembled prompt
6. **Commit** — Read the `COMMIT.md` file written by the agent, stage all changes, and commit
7. **Repeat** — Loop back to step 2 for the next task

When all tasks are complete or blocked, the loop exits. If a plan has all tasks completed, it is automatically renamed from `plan_N.md` to `plan_N_completed.md`.

You can limit the number of iterations with the `--count` flag:

```bash
maggus work --count 3   # stop after 3 tasks
```

## Agents

In Maggus, an **agent** is an AI coding assistant that executes tasks. Maggus doesn't talk to AI APIs directly — instead, it invokes the agent's CLI tool as a subprocess, passes it a prompt, and parses the streaming output.

The agent abstraction means the plan/task workflow stays the same regardless of which backend you use. Switching agents only affects the CLI flags Maggus passes and how it parses the streaming response — your plan files, acceptance criteria, and work loop behavior are unchanged.

### Supported Agents

| | Claude Code | OpenCode |
|---|---|---|
| CLI tool | `claude` | `opencode` |
| Streaming | Real-time JSON events | Single JSON response on completion |
| Model flag | `--model` (passed by Maggus) | Configured via OpenCode's own config file |
| Permissions | `--dangerously-skip-permissions` flag | Auto-approves in non-interactive mode |
| Model format | Bare ID (e.g. `claude-sonnet-4-6`) | `provider/model` (e.g. `anthropic/claude-sonnet-4-6`) |

### Selecting an Agent

Set the agent in `.maggus/config.yml`:

```yaml
agent: opencode
```

Or override per-run with the CLI flag:

```bash
maggus work --agent opencode
```

If no agent is configured, Maggus defaults to `claude` (Claude Code) for backwards compatibility. See the [Configuration reference](/reference/configuration) for full details.

## Git Branch Behavior

Maggus automatically manages branches to keep your main branch clean:

- If you're on a **protected branch** (`main`, `master`, or `dev`), Maggus creates a new branch named `feature/maggustask-NNN` (where NNN is the task number) before starting work.
- If you're already on a **non-protected branch**, Maggus works directly on it without creating a new one.

This means you can either let Maggus manage branches automatically, or check out a specific branch beforehand to control where changes land.

## Stopping a Run

Maggus provides two ways to stop a running work session:

### Stop After Task (Alt+S)

Press **Alt+S** during execution to request a graceful stop after the current task finishes. A confirmation prompt appears (`y/n`). While active, the border turns yellow. Press **Alt+S** again to cancel the stop and continue working.

This is the recommended way to stop — no work is lost and the current task completes cleanly.

### Ctrl+C (Immediate Stop)

- **First Ctrl+C** — Signals an immediate stop. The in-progress agent subprocess is cancelled and the run transitions to the summary screen.
- **Second Ctrl+C** — Force-kills the process immediately.

## The TUI

When Maggus is running, it displays a full-screen terminal UI (built with [Bubbletea](https://github.com/charmbracelet/bubbletea)) inside a bordered box that keeps you informed about progress.

### Header

The top section shows:
- **Version** (left) and **host fingerprint** (right)
- **Progress bar** showing overall task completion: `[████████░░░░] N/M Tasks`
- **Current task** ID and title in cyan

### Tab Bar

Below the header, a tab bar lets you switch between four views:

| Tab | Key | Content |
|-----|-----|---------|
| **Progress** | `1` | Live spinner, status, recent tool list, extras, model, elapsed time, token usage |
| **Detail** | `2` | Scrollable structured log of every tool invocation — each entry shows an icon, description, timestamp, and parameters |
| **Task** | `3` | Current task's plan file, description, and acceptance criteria with status icons (✓ done, ⚠ blocked, ○ pending) |
| **Commits** | `4` | List of commits made during the current run |

Switch tabs with `←/→` arrow keys or number keys `1`–`4`. The Detail tab supports `↑/↓/Home/End` scrolling with auto-scroll that pauses when you scroll up.

### Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `←/→` or `1-4` | Switch tabs |
| `↑/↓` | Scroll (on Detail tab) |
| `Home/End` | Jump to top/bottom |
| `Alt+S` | Toggle stop-after-task |
| `Ctrl+C` | Interrupt immediately |

### Summary Screen

After the run ends, a summary screen shows the outcome with a title reflecting the stop reason:

- **✓ Work Complete** — All requested tasks finished
- **⊘ Stopped by User** — Graceful stop via Alt+S
- **⊘ Work Interrupted** — Cancelled via Ctrl+C
- **✗ Work Failed** — A task or commit error (with detail)
- **⊘ No Tasks Available** — Nothing workable found

The summary includes run ID, branch, model, elapsed time, per-task token breakdown, commit list, and remaining tasks. You can choose to **Exit** or **Run again** with a custom task count.

## Run Logs

Every `maggus work` invocation creates a **run directory** under `.maggus/runs/`:

```
.maggus/runs/<RUN_ID>/
├── run.md              # Run-level metadata (start time, config, plan files)
└── iteration-NN.md     # Per-iteration log (one per task processed)
```

The `RUN_ID` is a timestamp like `20260312-215039`.

Each **iteration log** (`iteration-NN.md`) records:
- Which task was selected (ID and title)
- Commands and tools that were invoked
- Any deviations or skips from the acceptance criteria

Run logs are **gitignored** — they're local-only records of what Maggus did. They're useful for debugging if something goes wrong or for reviewing what happened in a long unattended run.

## Project Memory

Maggus maintains a **project memory file** at `.maggus/MEMORY.md`. This file:

- Stores project-specific knowledge gained during task execution (architecture decisions, completed tasks, tooling details, conventions)
- Is updated at the end of each run with any new information
- Is **gitignored** — it's not committed to the repository
- Is designed to be synced across machines via an external service, so Maggus has consistent context regardless of where it runs

The memory file is fed into prompts as bootstrap context, giving Maggus continuity across runs. Think of it as Maggus's long-term memory for your project — it remembers what it learned so it doesn't have to rediscover the same things on every run.
