# CLI Commands

Maggus provides commands for working with implementation plans, managing projects, and maintaining your installation. When run without arguments in an interactive terminal, Maggus shows an interactive menu. All commands that load configuration will show the configured agent (defaults to `claude`).

## maggus work

The main command. Parses plan files, finds the next workable task, builds a prompt with project context, invokes the configured agent, and commits the result. Repeats until the count is reached or all tasks are done.

### Usage

```bash
maggus work [count] [flags]
```

### Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--count` | `-c` | `5` | Number of tasks to work on |
| `--task` | | | Run a specific task by ID (e.g. `TASK-001`). Sets count to 1. |
| `--agent` | | *(from config)* | Agent backend to use (`claude` or `opencode`) |
| `--model` | | *(from config)* | Model to use (e.g. `opus`, `sonnet`, `anthropic/claude-sonnet-4-6`, or a full model ID) |
| `--worktree` | | *(from config)* | Run in an isolated git worktree |
| `--no-worktree` | | `false` | Force disable worktree mode (overrides config) |
| `--no-bootstrap` | | `false` | Skip reading CLAUDE.md, AGENTS.md, PROJECT_CONTEXT.md, and TOOLING.md |

The positional `[count]` argument and `--count` flag are interchangeable. The positional argument takes precedence.

### Examples

```bash
# Work on the next 5 tasks (default)
maggus work

# Work on the next 10 tasks
maggus work 10

# Work on 3 tasks using the --count flag
maggus work -c 3

# Override the model for this run
maggus work --model opus

# Use OpenCode as the agent backend
maggus work --agent opencode

# Run a specific task
maggus work --task TASK-003

# Run in an isolated worktree
maggus work --worktree

# Skip bootstrap context files
maggus work --no-bootstrap
```

See the [Work View](/reference/tui#work-view) in the TUI reference for details on the interactive interface, tabs, keyboard shortcuts, and the summary screen.

---

## maggus list

Preview upcoming tasks without running them. In TUI mode, shows all incomplete tasks (workable and blocked) with keyboard navigation and a scrollable view.

### Usage

```bash
maggus list [N] [flags]
```

### Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--count` | `-c` | `5` | Number of tasks to show |
| `--all` | | `false` | Show all upcoming workable tasks (ignores `--count`) |
| `--plain` | | `false` | Strip colors and use ASCII characters for scripting/piping |

The positional `[N]` argument overrides `--count`. It is ignored when `--all` is set.

### Examples

```bash
# Show the next 5 workable tasks (default)
maggus list

# Show the next 3 tasks
maggus list 3

# Show all workable tasks
maggus list --all

# Plain output for scripting
maggus list --plain
```

### Example Output

```
Next 5 task(s):

 #1  TASK-005: Create "CLI Commands" reference page
 #2  TASK-006: Create "Configuration" reference page
 #3  TASK-007: Create "Concepts" page covering run logs, memory, and TUI
 #4  TASK-008: Configure sidebar navigation and header nav
 #5  TASK-009: Add GitHub Actions workflow for deployment on release
```

The first task is highlighted in cyan (unless `--plain` is used).

In TUI mode (without `--plain`), the list is displayed in a full-screen bordered view. See the [List View](/reference/tui#list-view) in the TUI reference for keyboard shortcuts and navigation.

---

## maggus status

Show a compact summary of plan progress with tabbed plan sections, task counts, progress bars, and the configured agent.

### Usage

```bash
maggus status [flags]
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--all` | `false` | Show completed (archived) plans in task sections and the plans table |
| `--plain` | `false` | Strip colors and use ASCII characters for scripting/piping |

### Examples

```bash
# Show status of active plans
maggus status

# Include completed plans
maggus status --all

# Plain output for CI or scripting
maggus status --plain
```

### Example Output

```
Maggus Status — 3 plans (2 active), 24 tasks total
 Agent: claude

 Summary: 18/24 tasks complete · 4 pending · 2 blocked

 Tasks — plan_8.md
 ──────────────────────────────────────────
   ✓  TASK-001: Scaffold VitePress project in docs/
   ✓  TASK-002: Apply Simpsons-inspired theme
   ✓  TASK-003: Create "Getting Started" guide
   ✓  TASK-004: Create "Writing Plans" documentation page
 → o  TASK-005: Create "CLI Commands" reference page
   o  TASK-006: Create "Configuration" reference page
```

- `✓` = completed task, `⚠` = blocked task, `o` = pending task, `~` = ignored task
- `→` marks the next task that `maggus work` will pick up
- With `--plain`, symbols are replaced: `[x]` for done, `[!]` for blocked, `[~]` for ignored, `->` for next

In TUI mode (without `--plain`), the status is displayed in a full-screen bordered view with tabbed plan sections and keyboard navigation. See the [Status View](/reference/tui#status-view) in the TUI reference for the interactive features, task detail view, blocked task management, and ignoring tasks/plans.

---

## maggus update

Check for and install updates from GitHub Releases.

### Usage

```bash
maggus update
```

### Behavior

- Compares the current version against the latest GitHub Release
- If no update is available, prints "Already up to date"
- If an update is available, shows the changelog and asks for confirmation
- On confirmation, downloads the release asset and replaces the running binary
- When running a dev build (version = `"dev"`), any available release is treated as newer, allowing manual updates to the latest stable version

### Example

```bash
$ maggus update
Checking for updates...
Update available: v1.2.0 → v1.3.0

Changelog:
- Added repository switcher
- Improved TUI performance

Install update? [y/N] y
Downloading and installing...
Successfully updated to v1.3.0! Please restart maggus.
```

---

## maggus init

Initialize a `.maggus` project in the current directory.

### Usage

```bash
maggus init
```

### Behavior

- Creates the `.maggus/` directory
- Creates `.maggus/config.yml` with commented default settings (skips if it already exists)
- Updates `.gitignore` with required entries (run directories, memory, worktree directories)
- Installs the maggus plugin in Claude Code if the CLI is available

This is the recommended first step when setting up Maggus in a new project.

---

## maggus plan

Open an interactive AI session to create an implementation plan.

### Usage

```bash
maggus plan <description...>
```

### Examples

```bash
maggus plan Add OAuth2 authentication with Google provider
maggus plan "Refactor the parser to support nested tasks"
```

Launches the configured agent (Claude Code by default) interactively with the `/maggus-plan` skill. The AI walks you through clarifying questions before generating a plan file in `.maggus/`.

---

## maggus vision

Open an interactive AI session to create or improve `VISION.md`.

### Usage

```bash
maggus vision <description...>
```

### Examples

```bash
maggus vision A CLI tool for orchestrating AI agents
maggus vision "Improve the vision for our e-commerce platform"
```

---

## maggus architecture

Open an interactive AI session to create or improve `ARCHITECTURE.md`.

### Usage

```bash
maggus architecture <description...>
```

**Alias:** `maggus arch`

### Examples

```bash
maggus architecture A Go CLI with plugin system and streaming output
maggus arch "Review and improve our current architecture"
```

---

## maggus config

Edit project settings (`.maggus/config.yml`) interactively.

### Usage

```bash
maggus config
```

Opens a TUI editor where you can change agent, model, worktree, and notification settings. Navigate with arrow keys, cycle values with Enter or Left/Right, then select **Save**, **Edit file in editor**, or **Cancel**.

---

## maggus clean

Remove completed plan files and finished run directories.

### Usage

```bash
maggus clean [flags]
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--dry-run` | `false` | Show what would be removed without actually deleting anything |

### Examples

```bash
# Preview what would be removed
maggus clean --dry-run

# Remove completed plans and finished runs
maggus clean
```

Removes `_completed.md` plan files from `.maggus/` and run directories in `.maggus/runs/` that contain a `## End` section (indicating the run finished).

---

## maggus release

Generate a `RELEASE.md` with a conventional changelog and an AI-generated summary.

### Usage

```bash
maggus release [flags]
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--model` | *(from config)* | Model to use for AI summary generation |

### Examples

```bash
# Generate release notes using default model
maggus release

# Use a specific model
maggus release --model opus
```

Finds all commits since the last version tag, groups them by conventional commit type, and uses the configured agent to produce a human-readable summary. If `.maggus/RELEASE_NOTES.md` exists (accumulated during work iterations), it is included as context and then deleted after generation.

---

## maggus ignore

Exclude plans or tasks from the work loop.

### Usage

```bash
maggus ignore plan <plan-id>
maggus ignore task <TASK-NNN>
```

### Examples

```bash
# Ignore plan 3 (renames plan_3.md → plan_3_ignored.md)
maggus ignore plan 3

# Ignore a specific task (rewrites heading to ### IGNORED TASK-007: ...)
maggus ignore task TASK-007
```

Ignored plans and tasks are skipped by `maggus work` but remain visible in status/list views with a `~` marker.

---

## maggus unignore

Re-include ignored plans or tasks in the work loop.

### Usage

```bash
maggus unignore plan <plan-id>
maggus unignore task <TASK-NNN>
```

### Examples

```bash
# Unignore plan 3 (renames plan_3_ignored.md → plan_3.md)
maggus unignore plan 3

# Unignore a specific task
maggus unignore task TASK-007
```

---

## maggus worktree

Manage git worktrees created by `maggus work --worktree`.

### Subcommands

### `maggus worktree list`

Show active worktrees with their run IDs and branches.

```bash
$ maggus worktree list
Active worktrees:
  20260315-143022  branch: feature/maggustask-001
  20260315-150112  branch: feature/maggustask-002
```

### `maggus worktree clean`

Remove all worktrees in `.maggus-work/` and their associated branches. Also cleans up stale task lock files and prunes git worktree references.

```bash
maggus worktree clean
```

---

## maggus repos

Manage the list of known repositories. Opens an interactive TUI for adding, removing, and switching between projects.

### Usage

```bash
maggus repos
```

### Behavior

- Shows a list of repositories registered in `~/.maggus/repositories.yml`
- Add new repositories or remove existing ones
- Switch the active repository — Maggus changes its working directory accordingly

This command powers the **Repos** option in the interactive main menu. See the [Repository Registry](/reference/configuration#repository-registry-maggus-repositories-yml) section in the configuration reference for details on the underlying file format.
