# Terminal UI

Maggus uses a full-screen terminal UI built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) and [Lipgloss](https://github.com/charmbracelet/lipgloss). Every interactive view runs inside a bordered box with a status bar at the bottom showing available keyboard shortcuts.

## Main Menu

When you run `maggus` without any arguments in a terminal, the interactive main menu launches.

![Main Menu](/screenshots/main-menu.png)

### Layout

The menu screen shows:

- **Logo and version** at the top, centered
- **Summary line** — `N features, N open tasks · N bugs, N open tasks` (or "No open tasks")
- **Daemon status** — shows whether the background worker is running (see [Daemon Status](#daemon-status) below)
- **Current directory** displayed below the summary
- **Menu items** grouped by category
- **Status bar** at the bottom with navigation hints

When an update is available, a green banner appears below the summary.

### Daemon Status

Work is driven by the background daemon (`maggus start` / `maggus stop`). The menu header always shows one of three states:

| Indicator | Meaning |
|---|---|
| `● daemon running (PID XXXX)` | Daemon is active and processing tasks |
| `⏳ daemon stopping after task (PID XXXX)` | A graceful stop was requested; daemon will stop after the current task completes |
| `○ daemon not running` | No daemon is active; use `maggus start` to begin work |

### Menu Items

Items are grouped by category:

::: tip Shortcut hints
Hold `Alt` to briefly reveal underlined shortcut characters on each menu item. The underlines auto-hide after 1.5 seconds.
:::

#### Core Workflow

| Item | Shortcut | Description |
|---|---|---|
| **status** | `alt+s` | Live log & feature management |
| **repos** | `alt+r` | Manage configured repositories |

#### AI-Assisted Creation

These items only appear when Claude Code is installed.

| Item | Shortcut | Description |
|---|---|---|
| **prompt** | `alt+o` | Launch interactive Claude session with usage tracking |

#### Project Management

| Item | Shortcut | Description |
|---|---|---|
| **release** | `alt+z` | Generate RELEASE.md with changelog |
| **clean** | `alt+n` | Remove completed features and finished runs |
| **update** | `alt+u` | Check for and install updates |
| **config** | `alt+c` | Edit project settings |
| **init** | `alt+i` | Initialize a .maggus project (only shown when not yet initialized) |
| **exit** | — | Exit Maggus |

### Navigation

| Key | Action |
|---|---|
| `Up` / `Down` | Move through menu items |
| `Enter` | Select the highlighted item |
| `Home` / `End` | Jump to first / last item |
| `Alt` + shortcut | Jump directly to an item (e.g. `Alt+s` for status) |
| `q` / `Esc` / `Ctrl+C` | Exit Maggus |

### Auto-Update

Maggus checks for updates on startup (non-blocking). The behavior depends on the `auto_update` setting in your global config (`~/.maggus/config.yml`):

| Mode | Behavior |
|---|---|
| `off` | No update check |
| `notify` | Shows a banner when an update is available |
| `auto` | Downloads and applies updates automatically |

See [Configuration](/reference/configuration) for details on setting the update mode.

---

## Status View

The status view shows plan progress with tabbed plan sections, progress bars, and task lists.

![Status View](/screenshots/plan-view.png)

### Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `Up` / `Down` | Navigate plans and tasks |
| `PgUp` / `PgDn` | Jump to previous/next plan |
| `Left` / `Right` | Collapse/expand a plan |
| `Home` / `End` | Jump to first/last item |
| `Enter` | Open task detail view |
| `Alt+A` | Toggle showing completed plans |
| `a` | Approve/unapprove the selected plan |
| `Alt+R` | Run the selected task |
| `Alt+D` | Delete the selected plan (with confirmation) |
| `Alt+Backspace` | Delete the selected task (with confirmation) |
| `q` | Exit |

### Task Detail

Press **Enter** on any task to open a detail view showing its plan file, status, criteria summary, description, and acceptance criteria.

![Task Detail](/screenshots/task-detail.png)

| Key | Action |
|-----|--------|
| `PgUp` / `PgDn` | Previous/next task |
| `Tab` | Enter criteria mode (for blocked tasks) |
| `Alt+P` | Approve/unapprove the plan |
| `Alt+R` | Run the task |
| `Alt+Backspace` | Delete the task |
| `q` / `Backspace` | Back to task list |

### Managing Blocked Tasks

When viewing a task with blocked criteria, press **Tab** to enter **criteria mode**. Navigate between blocked criteria with **Up/Down** and press **Enter** to open the action picker.

![Blocked Handling](/screenshots/blocked-handling.png)

The action picker offers four options:

| Action | Description |
|--------|-------------|
| **Unblock** | Removes the `BLOCKED:` prefix, turning it back into a normal unchecked criterion |
| **Resolve** | Marks the criterion as done (removes the block and checks it) |
| **Delete** | Removes the criterion entirely from the plan file |
| **Skip** | Leaves the criterion unchanged |

Changes are applied immediately to the plan file. Press **Esc** to go back.

---

## List Command

`maggus list` is a plain-text CLI command, not a TUI view. It prints upcoming workable tasks as tab-separated output to stdout — suitable for scripting or quick inspection in the terminal. Run `maggus list --help` for available flags.
