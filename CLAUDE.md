# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What is Maggus

Maggus is a Go CLI tool that orchestrates Claude Code to work through implementation plans. It parses markdown plan files (`.maggus/plan_*.md`), finds the next workable task, builds a prompt with project context, invokes Claude Code as a subprocess, and commits the result. It runs in a loop until all tasks are done or blocked.

## Build & Test Commands

All Go source lives in `src/`. Run commands from that directory.

```bash
# Build
cd src && go build -o maggus .

# Build with version injection
cd src && go build -ldflags "-s -w -X github.com/dirnei/maggus/cmd.Version=1.0.0" -o maggus .

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

- **work** — Main loop: parse plans → find next task → create feature branch → build prompt → run Claude Code → commit → repeat
- **list** — Preview next N upcoming workable tasks
- **status** — Show plan progress with progress bars and blocked task reasons

### Internal Packages (src/internal/)

| Package | Purpose |
|---|---|
| **parser** | Parses `.maggus/plan_*.md` files. Extracts tasks (`### TASK-NNN: Title`), acceptance criteria (checkboxes), and blocked status (`BLOCKED:` prefix). Skips `_completed.md` files. |
| **prompt** | Assembles the prompt sent to Claude Code: bootstrap context files (CLAUDE.md, AGENTS.md, etc.), run metadata, task details, and behavioral instructions. Includes files specified in config. |
| **runner** | Invokes `claude -p <prompt> --output-format stream-json` as a subprocess. Parses streaming JSON events in real-time, drives a live terminal spinner showing status/tools/elapsed time. Handles Ctrl+C gracefully. |
| **config** | Parses `.maggus/config.yml`. Resolves model aliases (sonnet→claude-sonnet-4-6, opus→claude-opus-4-6, haiku→claude-haiku-4-5-20251001). Validates include file paths. |
| **gitbranch** | Creates `feature/maggustask-NNN` branches when on a protected branch (main/master/dev). |
| **gitcommit** | Reads COMMIT.md written by the agent, strips Co-Authored-By lines, runs `git commit -F`. |
| **gitignore** | Ensures required entries exist in .gitignore. |
| **runtracker** | Creates `.maggus/runs/<RUN_ID>/` with metadata and iteration logs. |

### Work Loop Flow (cmd/work.go)

1. Load config → validate includes → resolve model alias
2. Ensure .gitignore entries
3. Parse all active plan files → find next workable (incomplete + not blocked) task
4. Create feature branch if on protected branch
5. Build prompt with bootstrap context + task details
6. Run Claude Code subprocess with streaming output
7. Agent writes COMMIT.md → Maggus commits all changes
8. Rename completed plans (`plan_N.md` → `plan_N_completed.md`)
9. Loop back to step 3

### Platform-Specific Code

`runner/procattr_windows.go` and `procattr_other.go` handle OS-specific process group attributes for subprocess management.

## Release

GoReleaser (v2) builds binaries for linux/{amd64,arm64}, darwin/{amd64,arm64}, windows/amd64. Triggered by publishing a GitHub Release. Version is injected from the git tag.

## Key Conventions

- Plan files use `### TASK-NNN: Title` format with checkbox acceptance criteria
- Tasks containing `BLOCKED:` in any unchecked criterion are skipped
- The `.maggus/` directory is the working data directory; `runs/` and `MEMORY.md` inside it are gitignored
- Config supports `include` paths for additional context files fed into prompts
