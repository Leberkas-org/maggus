# Getting Started

Get Maggus installed and run your first automated task in minutes.

## Prerequisites

You need:

- **Git**
- **A terminal**
- **An AI coding agent** on your `PATH`:
  - [Claude Code](https://docs.anthropic.com/en/docs/claude-code) — Anthropic's coding agent **(default)**
  - [OpenCode](https://opencode.ai) — open-source agent supporting multiple providers

## Installation

Download the latest release for your platform from [GitHub Releases](https://github.com/leberkas-org/maggus/releases).

| Platform | Archive |
|---|---|
| Linux (amd64, arm64) | `maggus_*_linux_*.tar.gz` |
| macOS (amd64, arm64) | `maggus_*_darwin_*.tar.gz` |
| Windows (amd64) | `maggus_*_windows_amd64.zip` |

Extract and move the binary to a directory on your `PATH`:

::: code-group

```bash [macOS / Linux]
tar xzf maggus_*_linux_amd64.tar.gz
sudo mv maggus /usr/local/bin/
```

```powershell [Windows]
# Extract the zip, then move maggus.exe to a directory on your PATH
# For example, to C:\tools (make sure C:\tools is in your PATH):
Expand-Archive maggus_*_windows_amd64.zip -DestinationPath .
Move-Item maggus.exe C:\tools\
```

:::


## First Project Setup

Navigate to any Git repository and run `maggus init`:

```bash
cd your-project
maggus init
```

This creates:

| Created | Purpose |
|---|---|
| `.maggus/` | Working directory for plans, run logs, and locks |
| `.maggus/config.yml` | Project configuration (agent, model, includes) |
| `.gitignore` entries | Ensures run logs and internal files aren't committed |

If Claude Code is installed, `init` also registers the Maggus plan skill so you can generate plans interactively.

::: tip No existing repo?
Create one first:
```bash
mkdir my-project && cd my-project
git init && git commit --allow-empty -m "Initial commit"
maggus init
```
:::

## Writing Your First Plan

Create a plan file at `.maggus/features/feature_001.md`:

```markdown
# Plan: Hello World

## Introduction

A simple plan to verify Maggus works.

## Goals

- Test that Maggus can pick up a task and complete it

## User Stories

### TASK-001: Create a greeting file

**Description:** Create a simple greeting file to verify the setup works.

**Acceptance Criteria:**
- [ ] File `greeting.txt` exists containing "Hello from Maggus!"

### TASK-002: Add a goodbye file

**Description:** Add a second file to confirm multi-task flow.

**Acceptance Criteria:**
- [ ] File `goodbye.txt` exists containing "See you next time!"
```

Key format rules:
- Tasks use `### TASK-NNN: Title` headings
- Acceptance criteria are markdown checkboxes (`- [ ]`)
- Maggus marks criteria as `[x]` when completed

See [Writing Plans](./writing-plans) for the full format reference.

## Running Maggus

Run `maggus` in your project directory to open the interactive menu:

```bash
maggus
```

The menu shows your project summary (features and open tasks), the current daemon status, and available actions. From here, follow this workflow:

1. Select **status** (`alt+s`) to open the feature browser
2. Press `a` on a plan to **approve** it — the daemon only processes approved plans
3. Press `q` to return to the menu, then start the daemon:

```bash
maggus start
```

Once the daemon is running, it works through your tasks in order:

1. **Parse** — Load your plan and find the first incomplete, approved task (`TASK-001`)
2. **Branch** — Create `feature/maggustask-001` if you're on a protected branch (main/master/dev)
3. **Prompt** — Build a detailed prompt with your task, acceptance criteria, and project context
4. **Invoke** — Run the AI agent (Claude Code by default) to complete the task
5. **Commit** — Commit the agent's changes automatically
6. **Loop** — Move on to `TASK-002` and repeat until all tasks are done

::: tip Choosing an agent
By default, Maggus uses Claude Code. To use OpenCode instead, set it in your config:

```yaml
# .maggus/config.yml
agent: opencode
model: openai/gpt-4.1
```

Or pass `--agent opencode` on the command line. See the [Configuration reference](/reference/configuration) for details.
:::

## Monitoring Progress

Maggus runs as a background daemon. To check on progress, use the interactive menu (`maggus`) or run:

```bash
maggus status
```

The status view shows live task progress, completed tasks, and daemon state. When all tasks are done (or remaining tasks are blocked), the daemon stops automatically.

To stop the daemon manually:

```bash
maggus stop
```

## Next Steps

- [Writing Plans](./writing-plans) — learn the full plan format, blocked tasks, and multi-plan workflows
- [Maggus Skills](./maggus-plan-skill) — generate plans, vision, and architecture docs with AI
- [Terminal UI](/reference/tui) — explore the main menu, status view, and more
- [Concepts](./concepts) — understand the work loop, git behavior, run logs, and project memory
- [CLI Commands](/reference/commands) — explore all available commands (`status`, `list`, `clean`, and more)
- [Configuration](/reference/configuration) — customize agent, model, includes, and notifications
