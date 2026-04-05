# Maggus

Maggus is a spec-driven development tool for solo developers and small teams.
You write a structured plan; Maggus executes it — invoking an AI agent task by
task, committing after each, and running unattended until the work is done or
blocked. It pairs with interactive planning skills to close the loop from
spec authoring to autonomous execution.

## Core Concepts

### Spec-Driven Development

The central philosophy: write the spec first, then let Maggus grind through it.

- A **plan** is a markdown file describing a feature or bug fix, broken into
  discrete, independently-executable tasks
- Each **task** has a description, acceptance criteria (checkboxes), optional
  token budget, and predecessor/successor relationships
- Maggus works through tasks sequentially, one per agent invocation
- Tasks marked `BLOCKED:` are skipped until a human resolves the blocker
- When all tasks complete, the plan is marked done (renamed or deleted)

### Plans (Features & Bugs)

Plans live in `.maggus/features/feature_*.md` and `.maggus/bugs/bug_*.md`.

- **Feature plans** describe new functionality to build
- **Bug plans** describe defects to fix — bugs are worked first (higher priority)
- Plans are authored by the developer (manually or via planning skills)
- Each plan carries a stable UUID (`<!-- maggus-id: ... -->`) so it survives
  renames without breaking approval state
- Tasks within a plan can declare `BLOCKED:` criteria to signal human
  intervention is required before work can proceed

### The Work Loop

The core runtime behavior:

1. Parse all active plans → find the next workable task (incomplete, not blocked, approved)
2. Build a focused prompt: bootstrap context (CLAUDE.md, MEMORY.md, includes) + run metadata + task details + behavioral instructions
3. Invoke the AI agent as a subprocess, streaming output to the TUI in real time
4. Agent completes the task, checks off acceptance criteria, stages files, and writes `COMMIT.md`
5. Maggus commits using the agent-provided message
6. Loop back to step 1

The loop runs until all tasks are done, all remaining tasks are blocked, or the
user stops it. In daemon mode, this runs unattended in the background.

### Prompt Design (Token Efficiency)

Maggus deliberately keeps each agent invocation small and focused:

- One task per invocation — no full-feature context dumped into every prompt
- Bootstrap context is limited to files explicitly listed in config (`include`)
  plus a shared `.maggus/MEMORY.md` for cross-task learnings
- Token estimates per task are advisory; actual usage is tracked and displayed
- The planning skills are designed to produce well-scoped tasks that minimize
  back-and-forth within a single invocation

### Planning Skills

Three interactive Claude Code skills that close the spec-authoring loop:

| Skill | Purpose |
|---|---|
| `/maggus-plan` | Guides the developer through clarifying questions, produces a `.maggus/features/feature_*.md` plan |
| `/maggus-bugreport` | Produces a structured `.maggus/bugs/bug_*.md` bug plan |
| `/maggus-vision` | Generates `VISION.md` — project vision and intent |
| `/maggus-architecture` | Generates `ARCHITECTURE.md` — architecture decisions |

Skills reduce token usage by front-loading design decisions into small,
interactive planning sessions rather than letting the agent discover them
at execution time.

### Daemon Mode

Maggus can run as a background daemon for unattended, overnight execution:

- `maggus start` launches the work loop as a background process
- A separate TUI (`maggus status`) monitors progress via file watchers and
  log subscriptions — no polling, no tight coupling
- Stop signals: immediate stop (`.maggus/daemon.stop`) or graceful
  stop-after-current-task (`.maggus/daemon.stop-after-task`)
- Discord Rich Presence integration shows current task and progress in real time

### Approval Workflow

Human control over what the agent is allowed to work on:

- **Opt-in mode** (default): features must be explicitly approved before Maggus
  will work on them — prevents accidental AI work
- **Opt-out mode**: all features are approved unless explicitly blocked
- Approval state is persisted in `.maggus/feature_approvals.yml`, keyed by
  stable UUID — survives file renames

### Multi-Repo Workflow

Many developers work on several projects simultaneously. Maggus treats this as
a first-class use case:

- A global config (`~/.config/maggus/settings.json`) tracks all registered
  repositories
- The TUI repos view lets the developer switch between projects, start/stop
  daemons per repo, and see cross-project status at a glance
- Each repo has its own `.maggus/` directory, config, and approval state —
  fully independent

### Agent Abstraction

Maggus treats AI agents as pluggable backends:

- **Current focus:** Claude Code (`claude --output-format stream-json`)
- **Roadmap:** OpenCode and self-hosted AI agents
- The `Agent` interface defines `Run()`, `RunOnce()`, and `Validate()` —
  new backends can be added without changing the work loop
- Model aliases (`sonnet`, `opus`, `haiku`) are resolved at config parse time
  to full model IDs

### TUI & Feedback

Maggus provides a rich terminal UI built with Bubble Tea:

- Single screen-router model eliminates alt-screen flicker between views
- Live status view: feature tree, current task, acceptance criteria, tool
  invocation log, token usage, elapsed time
- Config editor, repo browser, prompt picker, and update checker all within
  the same TUI session
- Subprocess execution via `tea.ExecProcess` — the TUI never exits for agent runs
- Notifications: sound alerts on task/run completion and errors; Discord Rich
  Presence for ambient progress awareness

## Workflow (End to End)

```
Write spec          Execute                     Review
──────────────────  ──────────────────────────  ──────────────────
/maggus-plan     →  maggus start (daemon)    →  maggus status
/maggus-bugreport   maggus work (one shot)      git log
```

The developer authors plans (manually or via skills), approves them, and starts
Maggus. Maggus works through the queue autonomously. The developer reviews
commits when ready — hours or the next morning.

## Out of Scope (Current)

- Multi-user collaboration or shared plan queues
- Cloud-hosted execution or remote agents
- Non-git version control
- Real-time agent steering (Maggus is intentionally fire-and-forget per task)
- Extended notification channels (Slack, email, push) — sound and Discord are sufficient for now
