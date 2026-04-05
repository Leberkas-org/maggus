# Architecture

## Overview

Maggus is a single Go binary with an embedded Bubble Tea TUI and an optional
background daemon. It orchestrates AI agents for spec-driven development by
parsing markdown plan files, invoking agents as subprocesses with focused
prompts, and committing the results. All state is file-based — no database,
no HTTP server.

## System Shape

```
┌─────────────────────────────────────────────────────┐
│  maggus binary                                      │
│                                                     │
│  ┌──────────────┐     file I/O      ┌────────────┐  │
│  │  TUI (Bubble │ ◄──────────────── │   Daemon   │  │
│  │  Tea)        │    .maggus/ dir   │  (bg PID)  │  │
│  └──────┬───────┘                   └─────┬──────┘  │
│         │ tea.ExecProcess                 │         │
│         ▼                          subprocess       │
│  ┌──────────────┐ ◄────────────────────────────     │
│  │  AI Agent    │  stdin prompt / stdout JSON       │
│  │  subprocess  │                                   │
│  └──────────────┘                                   │
└─────────────────────────────────────────────────────┘
```

## Key Components

| Component | Package(s) | Responsibility |
|---|---|---|
| **Screen Router** | `cmd/` (`appModel`) | Single Bubble Tea program; routes between sub-models via `navigateToMsg`/`navigateBackMsg`; eliminates alt-screen flicker |
| **Work Loop** | `cmd/run.go` | Orchestrates the full parse → prompt → invoke → commit → repeat cycle |
| **Parser** | `internal/parser` | Reads `.maggus/features/feature_*.md` and `.maggus/bugs/bug_*.md`; extracts tasks, checkboxes, blocked criteria, UUIDs |
| **Prompt Builder** | `internal/prompt` | Assembles per-task prompts: bootstrap context files + run metadata + task details + behavioral instructions |
| **Agent Interface** | `internal/agent` | Pluggable agent backend abstraction: `Run()` (streaming + TUI), `RunOnce()` (text), `Validate()` |
| **Claude Agent** | `internal/agent/claude` | Implements agent interface for Claude Code: invokes `claude --output-format stream-json`, parses streaming events, feeds TUI messages |
| **Config** | `internal/config` | Parses `.maggus/config.yml`; resolves model aliases; validates include paths |
| **Approval** | `internal/approval` | Manages `.maggus/feature_approvals.yml`; opt-in / opt-out modes; UUID-keyed state |
| **Run Tracker** | `internal/runtracker` | Creates `.maggus/runs/<RUN_ID>/`; writes `state.json` snapshots and iteration logs for live status rendering |
| **Git Layer** | `internal/git*` | Branch creation, commit from `COMMIT.md`, `.gitignore` enforcement, remote sync check |
| **Global Config** | `internal/globalconfig` | `~/.config/maggus/settings.json`; tracks registered repos, startup metrics, Discord toggle |
| **File Watcher** | `internal/filewatcher` | Monitors `.maggus/features/` and `.maggus/bugs/` for changes; notifies status view to refresh |
| **Session Lock** | `internal/sesslock` | Prevents multiple daemon instances per project; enforced on `maggus start` |
| **Daemon** | `cmd/start`, `cmd/stop` | Background work loop; PID in `.maggus/daemon.pid`; sentinel files for stop signals |
| **Discord** | `internal/discord` | Rich Presence via local IPC (named pipe on Windows, domain socket on Unix) |
| **Updater** | `internal/updater` | Checks GitHub Releases; downloads and installs new binary |

## File-Based State

All runtime state is stored in the `.maggus/` directory. No database.

| Path | Purpose |
|---|---|
| `.maggus/config.yml` | Per-repo configuration (agent, model, includes, approval mode, hooks, etc.) |
| `.maggus/features/feature_*.md` | Feature plan files (active) |
| `.maggus/features/feature_*_completed.md` | Completed feature plans (renamed on completion) |
| `.maggus/bugs/bug_*.md` | Bug plan files |
| `.maggus/feature_approvals.yml` | Approval state, keyed by stable UUID |
| `.maggus/runs/<RUN_ID>/state.json` | State snapshots per run iteration (for live TUI status) |
| `.maggus/runs/<RUN_ID>/<iter>.log` | Raw streaming JSON events per iteration |
| `.maggus/MEMORY.md` | Cross-task architectural learnings; included in every prompt bootstrap (gitignored) |
| `.maggus/daemon.pid` | PID of the running daemon process |
| `.maggus/daemon.stop` | Sentinel: immediate daemon stop |
| `.maggus/daemon.stop-after-task` | Sentinel: graceful daemon stop after current task |
| `COMMIT.md` | Written by agent; read by Maggus to commit; never staged itself |

## Session Locking

Only one daemon may run per project at a time:

- `maggus start` acquires a session lock via `internal/sesslock` before launching the work loop
- If a lock is already held (another daemon is running), startup is rejected with an error
- The lock is released when the daemon exits (cleanly or via signal)
- This prevents concurrent agent invocations from conflicting on plan files, branches, and commits

## Work Loop (Detailed Flow)

```
start
  │
  ▼
load config → validate includes → resolve model alias
  │
  ▼
ensure .gitignore entries
  │
  ▼
parse all active plans (bugs first, then features)
  │
  ▼
find next workable task
  (incomplete + not blocked + approved)
  │
  ├── none found → done / blocked → exit
  │
  ▼
create feature branch if on protected branch
  (feature/maggustask-NNN)
  │
  ▼
build prompt
  (bootstrap context + run metadata + task details + instructions)
  │
  ▼
invoke agent subprocess (streaming JSON)
  │
  ├── TUI: live spinner, tool list, token usage, elapsed time
  │
  ▼
agent writes COMMIT.md, checks off criteria, stages files
  │
  ▼
maggus reads COMMIT.md → git commit -F
  │
  ▼
rename completed plans (feature_N.md → feature_N_completed.md)
  │
  ▼
loop back to "parse all active plans"
```

## Agent Abstraction

The `Agent` interface decouples the work loop from any specific AI tool:

```go
type Agent interface {
    Run(ctx, prompt, opts) error          // streaming, drives TUI
    RunOnce(ctx, prompt) (string, error)  // single-shot text response
    Validate() error                      // checks binary availability
}
```

- **Current backend:** Claude Code (`claude --output-format stream-json --dangerously-skip-permissions`)
- **Planned backends:** OpenCode, self-hosted AI agents
- Model aliases (`sonnet` → `claude-sonnet-4-6`, `opus` → `claude-opus-4-6`,
  `haiku` → `claude-haiku-4-5-20251001`) resolved at config parse time

## TUI Architecture

Built with **Bubble Tea** (charmbracelet/bubbletea).

- **Single program, multiple sub-models** — `appModel` is the screen router; it
  lazy-initializes sub-models on first navigation
- **Navigation** — `navigateToMsg` / `navigateBackMsg` messages; no program
  restart, no alt-screen flicker
- **Subprocess execution** — `execProcessMsg` + `tea.ExecProcess`; TUI suspends
  cleanly while agent runs, resumes after
- **Terminal dimensions** — cached in `appModel`, seeded to new sub-models on
  navigation

### TUI Sub-Models

| Sub-Model | Purpose |
|---|---|
| **Menu** | Entry point; repo list, start/stop daemon, update check |
| **Status** | Live feature/bug tree, current task details, run logs, token usage |
| **Config** | YAML editor for `.maggus/config.yml` with hot-reload validation |
| **Repos** | Multi-repo browser; per-repo daemon state, switch active repo |
| **Prompt Picker** | Select skill or free-form context → launch Claude Code interactively |
| **Update** | Changelog display, one-click binary update |

## TUI ↔ Daemon Communication

The TUI and daemon share state via the file system only — no sockets, no pipes.

| Mechanism | Direction | Purpose |
|---|---|---|
| `.maggus/daemon.pid` | Daemon → TUI | TUI watches for start/stop events |
| `.maggus/runs/<RUN_ID>/state.json` | Daemon → TUI | Live task status, tool activity, token counts |
| `.maggus/daemon.stop` | TUI → Daemon | Immediate stop signal |
| `.maggus/daemon.stop-after-task` | TUI → Daemon | Graceful stop-after-task signal |
| **File watcher** | FS → TUI | Auto-refresh status when plan files change externally |

## Prompt Design

Each agent invocation receives a minimal, task-scoped prompt:

1. **Bootstrap context** — CLAUDE.md, AGENTS.md, PROJECT_CONTEXT.md, TOOLING.md,
   `.maggus/MEMORY.md`, + files listed in `config.yml include`
2. **Run metadata** — `RUN_ID`, `ITERATION` number
3. **Task details** — ID, title, description, acceptance criteria
4. **Behavioral instructions** — "Complete only this task. Update checkboxes.
   Stage files. Write COMMIT.md. Optionally update MEMORY.md and RELEASE_NOTES.md."

Token efficiency is achieved by keeping context minimal per invocation.
Cross-task learnings are accumulated in `.maggus/MEMORY.md` by the agent.

## Multi-Repo Workflow

- **Global config** at `~/.config/maggus/settings.json` tracks all registered repos
- Each repo has its own `.maggus/` directory — fully independent config,
  approval state, and run history
- The **Repos view** in the TUI allows switching between projects and managing
  per-repo daemon state without leaving the application
- The **resolver** package determines the active repo on startup from global config

## Process Management

| Scenario | Windows | Unix |
|---|---|---|
| Kill agent subprocess | `taskkill /T /F /PID` (tree kill) | `process.Kill()` |
| Graceful shutdown | 5-second wait before force kill | 5-second wait before force kill |
| Daemon PID tracking | `.maggus/daemon.pid` | `.maggus/daemon.pid` |
| Discord IPC | Named pipe | Unix domain socket |

## Skills System

Three Claude Code skills extend Maggus with interactive spec-authoring:

| Skill | CLI Entry Point | Output |
|---|---|---|
| `/maggus-plan` | `maggus plan` | `.maggus/features/feature_*.md` |
| `/maggus-bugreport` | `maggus bugreport` | `.maggus/bugs/bug_*.md` |
| `/maggus-vision` | `maggus vision` | `VISION.md` |
| `/maggus-architecture` | `maggus architecture` | `ARCHITECTURE.md` |

Skills are installed via the Claude Code plugin marketplace on `maggus init`.
They run inside Claude Code's context and write files directly to the repo.

## Release & Distribution

- **GoReleaser v2** builds binaries for `linux/{amd64,arm64}`,
  `darwin/{amd64,arm64}`, `windows/amd64`
- Triggered by publishing a GitHub Release; version injected from git tag via
  `-ldflags "-X cmd.Version=<tag>"`
- Windows builds include icon/manifest via `go-winres`
- In-app updater checks GitHub Releases API; downloads and replaces binary

## Code Organization Rules

- No `.go` file exceeds 500 lines (excluding tests); split by responsibility
- Large TUI commands split into `<cmd>_model.go`, `<cmd>_update.go`,
  `<cmd>_view.go`, `<cmd>_cmd.go`
- Shared helpers are **pure functions** (not struct methods) to work across
  different TUI model shapes
- Shared TUI utilities: `internal/tui/styles/nav.go` (`CursorUp`, `CursorDown`,
  `ClampCursor`)
- No duplicated logic — check for existing helpers before implementing patterns
  like cursor navigation or file loading
