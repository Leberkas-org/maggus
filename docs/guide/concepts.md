# Concepts

This page explains Maggus's runtime behavior — what happens when you run `maggus work`, what gets logged, and how to interact with the TUI.

## Work Loop Lifecycle

When you run `maggus work`, Maggus enters a loop that processes tasks one at a time:

1. **Parse** — Load all active plan files (`.maggus/plan_*.md`), skipping completed (`_completed.md`) files
2. **Find task** — Identify the next workable task (incomplete and not blocked) across all plans
3. **Branch** — If on a protected branch (`main`, `master`, `dev`), create a feature branch
4. **Prompt** — Assemble the prompt with bootstrap context files, run metadata, and task details
5. **Run** — Invoke Claude Code as a subprocess with the assembled prompt
6. **Commit** — Read the `COMMIT.md` file written by the agent, stage all changes, and commit
7. **Repeat** — Loop back to step 2 for the next task

When all tasks are complete or blocked, the loop exits. If a plan has all tasks completed, it is automatically renamed from `plan_N.md` to `plan_N_completed.md`.

You can limit the number of iterations with the `--count` flag:

```bash
maggus work --count 3   # stop after 3 tasks
```

## Git Branch Behavior

Maggus automatically manages branches to keep your main branch clean:

- If you're on a **protected branch** (`main`, `master`, or `dev`), Maggus creates a new branch named `feature/maggustask-NNN` (where NNN is the task number) before starting work.
- If you're already on a **non-protected branch**, Maggus works directly on it without creating a new one.

This means you can either let Maggus manage branches automatically, or check out a specific branch beforehand to control where changes land.

## Startup Safety Pause

When `maggus work` starts, it shows a **3-second countdown** before beginning the first task. This gives you a window to review what's about to happen and abort if needed.

Press **Ctrl+C** during this countdown to cancel the run before any work begins.

## Ctrl+C Behavior

Maggus handles interrupts gracefully:

- **First Ctrl+C** — Signals a graceful stop. Maggus finishes processing the current Claude Code response, commits any pending work, and then exits cleanly.
- **Second Ctrl+C** — Force-kills the process immediately, including any running Claude Code subprocess.

This two-stage approach ensures you don't lose work from a partially completed task. If you need to stop urgently, the double Ctrl+C always works.

## The TUI

When Maggus is running, it displays a full-screen terminal UI (built with [Bubbletea](https://github.com/charmbracelet/bubbletea)) that keeps you informed about progress.

### Header

The top section shows:
- **Version** (left) and **host fingerprint** (right)
- **Progress bar** showing overall task completion: `[████████░░░░] N/M Tasks`
- A gray separator line

### Task Info

Below the header, the current task ID and title are displayed in cyan, so you always know which task is being worked on.

### Status Section

The main area of the TUI shows:
- **Spinner and status** — Current activity (e.g., "Running", "Writing file", "Done")
- **Output** — Recent text output from Claude Code
- **Tool history** — The last 10 tools Claude Code invoked, shown with `│` and `▶` prefixes
- **Model** — Which Claude model is being used
- **Elapsed time** — How long the current iteration has been running

The status updates in real-time as Claude Code streams its response.

### Recent Commits

At the bottom of the TUI, a "Recent Commits" section shows the commit messages from completed iterations. These persist across iterations so you can track what's been done in the current run.

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
