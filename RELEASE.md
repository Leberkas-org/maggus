# Release Notes

## What's New

### Config File Support
Maggus now reads `.maggus/config.yml` for persistent project-level settings. No more repeating flags on every run.

```yaml
model: sonnet
include:
  - ARCHITECTURE.md
```

### Model Selection
Choose which Claude model Maggus uses via config file or CLI flag:

```bash
maggus work --model opus
maggus work --model haiku
```

Short aliases (`sonnet`, `opus`, `haiku`) resolve to their full model IDs automatically. The `--model` flag overrides the config file.

### Custom Prompt Includes
Add project-specific context files to every agent prompt via `include` in `config.yml`. Useful for architecture docs, coding patterns, or any reference material the agent should read before starting.

### Run Logs
Every session now creates a timestamped run directory at `.maggus/runs/<RUN_ID>/` containing:
- `run.md` — session metadata (model, branch, start/end commit, timing)
- `iteration-01.md`, `iteration-02.md`, … — per-task logs written by the agent

### Completed Plan Renaming
When all tasks in a plan file are finished, Maggus renames it to `plan_N_completed.md` automatically. Future runs skip completed plans without needing manual cleanup.

### Improved Startup Banner
The startup screen now shows model, iteration count, current branch, and run ID before the 3-second safety pause.

### Summary Banner
After each session, Maggus prints a summary with the run ID, branch, log directory, and commit range.

### Auto Git Push
After completing tasks, Maggus automatically pushes the feature branch to remote.

### No-Bootstrap Flag
Skip reading `CLAUDE.md`/`AGENTS.md`/`PROJECT_CONTEXT.md`/`TOOLING.md` with `--no-bootstrap` for faster runs on projects that don't use these files.

---

## Installation

Download the binary for your platform from the Assets below, or build from source:

```bash
cd src
go build -o maggus .
```

Requires `claude` (Claude Code CLI) on your PATH.
