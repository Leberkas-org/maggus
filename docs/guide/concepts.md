# Concepts

This page explains Maggus's runtime behavior — what happens when you run `maggus start`, what gets logged, and how to interact with the TUI.

## Work Loop Lifecycle

When you start Maggus (via `maggus start` or the interactive menu), it enters a loop that processes tasks one at a time:

1. **Parse** — Load all active feature files (`.maggus/features/feature_*.md`) and bug files (`.maggus/bugs/bug_*.md`), skipping completed (`_completed.md`) files
2. **Find task** — Identify the next workable task (incomplete and not blocked) across all plans
3. **Branch** — If on a protected branch (`main`, `master`, `dev`), create a feature branch
4. **Prompt** — Assemble the prompt with bootstrap context files, run metadata, and task details
5. **Run** — Invoke the configured agent as a subprocess with the assembled prompt
6. **Commit** — Read the `COMMIT.md` file written by the agent, stage all changes, and commit
7. **Repeat** — Loop back to step 2 for the next task

When all tasks are complete or blocked, the loop exits. If a feature file has all tasks completed, it is automatically renamed from `feature_N.md` to `feature_N_completed.md`.

When all tasks are complete or blocked, the loop exits.

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
maggus start --agent opencode
```

If no agent is configured, Maggus defaults to `claude` (Claude Code) for backwards compatibility. See the [Configuration reference](/reference/configuration) for full details.

## Git Branch Behavior

Maggus automatically manages branches to keep your main branch clean:

- If you're on a **protected branch** (`main`, `master`, or `dev`), Maggus creates a new branch named `feature/maggustask-NNN` (where NNN is the task number) before starting work.
- If you're already on a **non-protected branch**, Maggus works directly on it without creating a new one.

This means you can either let Maggus manage branches automatically, or check out a specific branch beforehand to control where changes land.

## Stopping the Daemon

Use `maggus stop` to gracefully terminate the running daemon. The in-progress task will be cancelled and the daemon shuts down.

## Monitoring Progress

Maggus runs as a background daemon — there is no interactive TUI attached to the work loop. To monitor progress, use the interactive menu (`maggus`) or `maggus status`, which shows live task progress, run logs, and daemon state.

See the [TUI reference](/reference/tui) for details on the status view and main menu.

## Run Logs

Every Maggus run creates a **run directory** under `.maggus/runs/`:

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
