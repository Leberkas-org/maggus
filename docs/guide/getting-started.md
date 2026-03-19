# Getting Started

Get Maggus installed and run your first automated task in minutes.

## Prerequisites

You need:

- **Git** — any recent version ([download](https://git-scm.com/downloads))
- **A terminal** — your OS default works (Terminal on macOS, PowerShell or Git Bash on Windows, any shell on Linux)
- **An AI coding agent** on your `PATH`:
  - [Claude Code](https://docs.anthropic.com/en/docs/claude-code) — Anthropic's coding agent **(default)**
  - [OpenCode](https://opencode.ai) — open-source agent supporting multiple providers

Verify your setup:

```bash
git --version         # any recent version
claude --version      # if using Claude Code
opencode --version    # if using OpenCode
```

## Installation

### Pre-built binary (recommended)

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

### `go install`

If you have Go 1.22+ installed:

```bash
go install github.com/leberkas-org/maggus@latest
```

The binary is placed in your `$GOPATH/bin` (or `$HOME/go/bin` by default).

### Build from source

```bash
git clone https://github.com/leberkas-org/maggus.git
cd maggus/src
go build -o maggus .
```

::: tip Windows
On Windows, build with `go build -o maggus.exe .` instead. For embedded icon and version info, install [go-winres](https://github.com/tc-hib/go-winres) and run `go-winres make` before building.
:::

Move the resulting binary to a directory on your `PATH`, or run it directly with `./maggus` (or `.\maggus.exe` on Windows).

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

Create a plan file at `.maggus/plan_1.md`:

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

Start the work loop:

```bash
maggus work
```

Maggus will:

1. **Parse** your plan and find the first incomplete task (`TASK-001`)
2. **Branch** — create `feature/maggustask-001` if you're on a protected branch (main/master/dev)
3. **Prompt** — build a detailed prompt with your task, acceptance criteria, and project context
4. **Invoke** the AI agent (Claude Code by default) to complete the task
5. **Commit** — the agent's changes are committed automatically
6. **Loop** — move on to `TASK-002` and repeat until all tasks are done

Sample startup output:

```
Maggus v1.0.0                            abc123
[████████░░░░░░░░░░░░] 0/2 Tasks

  TASK-001: Create a greeting file

  ◐ Working...
```

::: tip Choosing an agent
By default, Maggus uses Claude Code. To use OpenCode instead, set it in your config:

```yaml
# .maggus/config.yml
agent: opencode
model: openai/gpt-4.1
```

Or pass `--agent opencode` on the command line. See the [Configuration reference](/reference/configuration) for details.
:::

## Understanding the Output

While Maggus runs, the TUI (terminal UI) shows:

| Section | What it shows |
|---|---|
| **Header** | Version, host fingerprint, progress bar (`N/M Tasks`) |
| **Task info** | Current task ID and title |
| **Spinner & status** | What the agent is doing right now (reading files, writing code, running commands) |
| **Tool history** | Recent tools the agent has used |
| **Tokens** | Input/output token usage for the current task |
| **Elapsed** | Time spent on the current task |

**Tabs:** Press `1` for the Progress view or `2` for Recent Commits. Press `Alt+I` to toggle a detail panel showing full tool parameters.

**When a task completes**, Maggus commits the changes and immediately moves to the next task. When all tasks are done (or remaining tasks are blocked), you'll see a summary screen with:

- Total tasks completed
- Commit range
- Remaining/blocked tasks (if any)
- Token usage breakdown

Press any key to exit the summary.

**Keyboard shortcuts during work:**

| Key | Action |
|---|---|
| `Ctrl+C` | Stop gracefully after the current task |
| `Ctrl+C` (twice) | Force-quit immediately |
| `1` / `2` | Switch between Progress and Commits tabs |
| `Alt+I` | Toggle detail panel |

## Next Steps

- [Writing Plans](./writing-plans) — learn the full plan format, blocked tasks, and multi-plan workflows
- [Concepts](./concepts) — understand the work loop, git behavior, run logs, and project memory
- [CLI Commands](/reference/commands) — explore all available commands (`status`, `list`, `blocked`, `clean`, and more)
- [Configuration](/reference/configuration) — customize agent, model, includes, and notifications
