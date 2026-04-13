# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What is Maggus

Maggus is a Go CLI tool that orchestrates Claude Code to work through feature files. It parses markdown feature files (`.maggus/features/feature_*.md`), finds the next workable task, builds a prompt with project context, invokes Claude Code as a subprocess, and commits the result. It runs in a loop until all tasks are done or blocked.

## Build & Test Commands

All Go source lives in `src/`. Run commands from that directory.

```bash
# Build
cd src && go build -o maggus .

# Build with version injection
cd src && go build -ldflags "-s -w -X github.com/leberkas-org/maggus/cmd.Version=1.0.0" -o maggus .

# Build with Windows icon/manifest (requires go-winres)
cd src && go-winres make && go build -o maggus.exe .

# Run all tests
cd src && go test ./...

# Run tests for a specific package
cd src && go test ./internal/parser
cd src && go test -v -run TestSpecificName ./internal/parser

# Format and vet
cd src && go fmt ./... && go vet ./...
```

CI runs `go build ./...` and `go test ./...` in the `src/` directory on PRs to master.

## Architecture

### CLI Commands (src/cmd/)

- **start** — Launch the work loop as a background daemon
- **stop** — Stop the running daemon
- **run** — (Hidden) Internal command used by the daemon; parses features → finds next task → builds prompt → runs Claude Code → commits → repeats
- **approve / unapprove** — Mark features as approved or revoke approval
- **list** — List all active features and bugs
- **status** — Interactive TUI showing feature progress, task details, and run logs
- **clean** — Remove completed feature and bug files

### Internal Packages (src/internal/)

| Package | Purpose |
|---|---|
| **parser** | Parses `.maggus/features/feature_*.md` and bug files. Extracts tasks (`### TASK-NNN: Title`), acceptance criteria (checkboxes), and blocked status (`BLOCKED:` prefix). Skips `_completed.md` files. |
| **prompt** | Assembles the prompt sent to the agent: bootstrap context files (CLAUDE.md, AGENTS.md, etc.), run metadata, task details, and behavioral instructions. Includes files specified in config. |
| **agent** | Defines the `Agent` interface for AI backend adapters (Claude Code, OpenCode, etc.). Orchestrator and worker use this to invoke whichever backend is configured without knowing its CLI specifics. |
| **config** | Parses `.maggus/config.yml`. Resolves model aliases (sonnet→claude-sonnet-4-6, opus→claude-opus-4-6, haiku→claude-haiku-4-5-20251001), validates include file paths, and holds notification settings. |
| **globalconfig** | Manages global Maggus settings in `~/.maggus/`: repository list, user preferences, lifetime metrics, and binary update state. |
| **approval** | Manages feature approval state in `.maggus/feature_approvals.yml`. Supports opt-in (explicit approval required) and opt-out (approved by default) modes. |
| **runlog** | Writes structured JSONL events to `.maggus/logs/<maggus_id>/<pid>.log`, organized by feature/bug GUID. Tracks per-task token usage and cost; prunes old log files per feature directory. Manages daemon state snapshots in `.maggus/runs/` (`state.json`, `state-*.json`). |
| **stores** | File-backed and in-memory repository implementations for features and bugs. Wraps parser operations for consistent use by the orchestrator and TUI. |
| **gitbranch** | Creates hierarchical branches when starting work on a protected branch (main/master/dev). On protected branches, creates a plan branch (`feature/feat-NNN` for features, `fix/bug-NNN` for bugs); on non-protected branches, stays on the current branch. Task branches: `feature/maggus-NNN/task-MMM` (features), `bugfix/maggus-bug-NNN/task-MMM` (bugs). |
| **gitcommit** | Reads COMMIT.md written by the agent, strips Co-Authored-By lines, and runs `git commit -F`. |
| **gitignore** | Ensures required entries exist in `.gitignore`. |
| **gitsync** | Handles remote git sync: fetch, ahead/behind status, stash, pull, and force-pull operations. |
| **hooks** | Executes lifecycle hook commands from config by writing JSON event payloads to their stdin (e.g., on task start/complete). |
| **discord** | Implements Discord Rich Presence integration via Discord's local IPC protocol (named pipe on Windows, domain socket on Unix). |
| **filewatcher** | Watches `.maggus/features/` and `.maggus/bugs/` for file changes, debounces rapid events, and sends Bubble Tea update messages to the TUI. |
| **tui** | Interactive terminal UI components using Bubble Tea: status display, feature browser, and related views. |
| **updater** | Downloads and applies binary updates from GitHub releases by replacing the currently running executable. |

### Run Loop Flow (cmd/run.go)

The daemon invokes the orchestrator once per cycle:

1. Load config → validate includes → resolve model alias
2. Ensure .gitignore entries
3. Create `OrchestratorConfig` with approved feature groups, agent, stores, etc.
4. Call `Orchestrator.Run()`:
   - Parse all active feature files → build approved feature groups
   - For each feature group (bugs first, then features):
     - Re-check approval state between features
     - For each task in feature:
       - Check stop flags and dispatch requests
       - Classify task as parallel-eligible or sequential
       - If parallel: create worktree, launch `RunTaskWorker` concurrently
       - If sequential: call `RunTaskWorker` in main repo
       - Check remote divergence between tasks
     - Rename completed features (`feature_N.md` → `feature_N_completed.md`)
5. Return results to daemon (completed count, failed tasks, stop reason)

`RunTaskWorker` is a unified function used by both sequential and parallel
execution paths. The orchestrator decides which directory the worker runs in
(main repo or worktree) and whether to parallelize. The worker has no knowledge
of execution modes — it simply takes a `WorkDir`, executes the task, and commits
the result.

### Platform-Specific Code

`agent/procattr_windows.go` and `procattr_other.go` handle OS-specific process group attributes for subprocess management.

## Release

GoReleaser (v2) builds binaries for linux/{amd64,arm64}, darwin/{amd64,arm64}, windows/amd64. Triggered by publishing a GitHub Release. Version is injected from the git tag.

## Code Organization Rules

- **File size limit:** No single `.go` file should exceed 500 lines (excluding tests). If a file grows beyond this, split it by responsibility (e.g., model/update/view for Bubble Tea TUI files).
- **No duplicated logic:** Before writing cursor navigation, file loading, confirmation dialogs, or similar patterns — check if a shared helper already exists:
  - Cursor navigation: `internal/tui/styles/nav.go` (`CursorUp`, `CursorDown`, `ClampCursor`)
  - Feature/bug file loading: `internal/parser/plan.go` (`LoadPlans`, `Plan` type)
- **Bubble Tea file split pattern:** Large TUI commands should be split into `<cmd>_model.go` (struct + init), `<cmd>_update.go` (Update + key handling), `<cmd>_view.go` (View + render helpers), `<cmd>_cmd.go` (cobra command + init).
- **Pure functions over structs** for shared helpers — different TUI models have different field names, so helpers should take and return values rather than operating on a specific struct.

## Key Conventions

- Feature files use `### TASK-NNN: Title` format with checkbox acceptance criteria
- Tasks containing `BLOCKED:` in any unchecked criterion are skipped
- The `.maggus/` directory is the working data directory; `runs/` and `MEMORY.md` inside it are gitignored
- Config supports `include` paths for additional context files fed into prompts

## Auto-Memory

Do not use auto-memory. Do not read from or write to the memory directory.

## Session Persistence

At the end of each session, update `.maggus/MEMORY.md` with all project-relevant info needed for consistency across PCs. This file is gitignored and synced via a dedicated service. It should contain: project structure, build instructions, CI/CD setup, tools, and any setup-specific knowledge.

## Workflow Rules

- **Feature plans:** Always use the `/maggus-plan` skill (via `Skill` tool) when creating feature implementation plans. Do not use `superpowers:writing-plans`. The maggus project has its own plan format that saves to `.maggus/features/feature_NNN.md`.
- **Plan output:** When `/plan` + `maggus-plan` are used together, the `.maggus/features/feature_NNN.md` file IS the plan. Do not write a separate Claude plan file to the plans directory. Still call ExitPlanMode at the end to close plan mode.
