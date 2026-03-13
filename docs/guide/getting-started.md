# Getting Started

This guide walks you through installing Maggus and running your first automated task.

## Prerequisites

Before you begin, make sure you have:

- **Go 1.22+** — [Download Go](https://go.dev/dl/)
- **Claude Code CLI** — installed and available on your `PATH`. See [Claude Code docs](https://docs.anthropic.com/en/docs/claude-code) for setup instructions.

Verify both are installed:

```bash
go version    # should print go1.22 or later
claude --version
```

## Installation

### Pre-built binaries

Download the latest binary for your platform from [GitHub Releases](https://github.com/leberkas-org/maggus/releases).

Available platforms:
- Linux (amd64, arm64)
- macOS (amd64, arm64)
- Windows (amd64)

After downloading, extract the archive and move the `maggus` binary to a directory on your `PATH`.

### Build from source

```bash
git clone https://github.com/leberkas-org/maggus.git
cd maggus/src
go build -o maggus .
```

Move the resulting `maggus` binary to a directory on your `PATH`, or run it directly with `./maggus`.

## First Run

### 1. Create the `.maggus/` directory

In your project root, create the working directory Maggus needs:

```bash
mkdir .maggus
```

### 2. Write a minimal plan file

Create a plan file at `.maggus/plan_1.md`:

```markdown
# Plan: My First Plan

## Introduction

A simple plan to test Maggus.

## Goals

- Verify Maggus can process a task end-to-end

## User Stories

### TASK-001: Add a hello-world script

**Description:** Create a simple hello-world script to verify the setup.

**Acceptance Criteria:**
- [ ] File `hello.sh` exists with a hello-world message
- [ ] The script is executable
```

### 3. Run Maggus

```bash
maggus work
```

Maggus will:
1. Parse your plan file and find `TASK-001`
2. Create a feature branch (`feature/maggustask-001`)
3. Build a prompt with your task details and project context
4. Invoke Claude Code to complete the task
5. Commit the result automatically
6. Check for the next task (none left, so it stops)

You'll see a TUI with a progress bar, live status updates, and tool history as Claude works through the task.

::: tip
Maggus shows a 3-second countdown before starting. Press **Ctrl+C** during this window to abort. Once running, press **Ctrl+C** once to stop gracefully after the current task, or twice to force-quit immediately.
:::

## Minimal End-to-End Example

Here's a complete copy-paste example you can try in any Git repository:

```bash
# 1. Initialize a git repo (skip if you already have one)
mkdir my-project && cd my-project
git init

# 2. Create the Maggus working directory
mkdir .maggus

# 3. Write a plan file
cat > .maggus/plan_1.md << 'EOF'
# Plan: Hello World

## Introduction

Test plan for Maggus.

## Goals

- Verify Maggus works

## User Stories

### TASK-001: Create a greeting file

**Description:** Create a file that greets the user.

**Acceptance Criteria:**
- [ ] File `greeting.txt` exists containing "Hello from Maggus!"
EOF

# 4. Make an initial commit so Maggus can branch
git add -A && git commit -m "Initial commit"

# 5. Run Maggus
maggus work
```

After Maggus finishes, you'll have a new branch with `greeting.txt` committed.

## What's Next

- Learn how to write more complex plans in [Writing Plans](./writing-plans)
- Explore all CLI commands in the [CLI Reference](/reference/commands)
- Understand Maggus concepts like run logs, memory, and the TUI in [Concepts](./concepts)
