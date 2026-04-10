# CLI Commands

Maggus provides commands for working with implementation plans, managing projects, and maintaining your installation. When run without arguments in an interactive terminal, Maggus shows an interactive menu. The available commands are: `start`, `stop`, `approve`, `unapprove`, `list`, `clean`, `release`, `init`, and `update`. All commands that load configuration will show the configured agent (defaults to `claude`).

## maggus start

Launch the work loop as a background daemon. Parses plan files, finds approved workable tasks, builds a prompt with project context, invokes the configured agent, and commits the result. Repeats until all approved tasks are done.

### Usage

```bash
maggus start [flags]
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--model` | *(from config)* | Model to use (e.g. `opus`, `sonnet`, `anthropic/claude-sonnet-4-6`, or a full model ID) |
| `--agent` | *(from config)* | Agent backend to use (`claude` or `opencode`) |
| `--all` | `false` | Start daemons for all registered repositories with auto-start enabled |

### Examples

```bash
# Start the daemon
maggus start

# Start with a specific model
maggus start --model opus

# Start daemons for all registered repos
maggus start --all
```

Use `maggus stop` to terminate the daemon.

---

## maggus stop

Stop the running maggus daemon.

### Usage

```bash
maggus stop [flags]
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--all` | `false` | Stop daemons in all registered repositories |

### Examples

```bash
# Stop the daemon in the current repository
maggus stop

# Stop daemons in all registered repositories
maggus stop --all
```

---

## maggus approve

Mark a feature or bug plan as approved so the daemon will pick it up.

### Usage

```bash
maggus approve <plan-id>
```

### Examples

```bash
# Approve plan 3
maggus approve 3
```

---

## maggus unapprove

Revoke approval from a feature or bug plan so the daemon will skip it.

### Usage

```bash
maggus unapprove <plan-id>
```

### Examples

```bash
# Unapprove plan 3
maggus unapprove 3
```

---

## maggus list

Preview upcoming tasks without running them.

### Usage

```bash
maggus list
```

### Behavior

Lists all active features and bugs as tab-separated lines. Output columns: `filename`, `id`, `title`, `approved`.

### Example Output

```
Next 5 task(s):

 #1  TASK-005: Create "CLI Commands" reference page
 #2  TASK-006: Create "Configuration" reference page
 #3  TASK-007: Create "Concepts" page covering run logs, memory, and TUI
 #4  TASK-008: Configure sidebar navigation and header nav
 #5  TASK-009: Add GitHub Actions workflow for deployment on release
```

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

## maggus clean

Remove completed feature and bug files.

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

# Remove completed plans
maggus clean
```

Removes all `_completed.md` files from `.maggus/features/` and `.maggus/bugs/`.

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
