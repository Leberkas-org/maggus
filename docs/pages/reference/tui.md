# Terminal UI

Maggus uses a full-screen terminal UI built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) and [Lipgloss](https://github.com/charmbracelet/lipgloss). Every interactive view runs inside a bordered box with a status bar at the bottom showing available keyboard shortcuts.

## Main Menu

When you run `maggus` without any arguments in a terminal, the interactive main menu launches.

```
┌─────────────────────────────────────────────────────────┐
│                                                         │
│                  maggus  v1.2.0                         │
│                                                         │
│   3 features, 5 open tasks · 1 bug, 2 open tasks        │
│   ● daemon running (PID 12345)                          │
│   /home/user/my-project                                 │
│                                                         │
│   ── Core Workflow ──────────────────────────────────── │
│   > status            Live log & feature management     │
│     repos             Manage configured repositories    │
│                                                         │
│   ── Project Management ────────────────────────────── │
│     clean             Remove completed features         │
│     update            Check for and install updates     │
│     config            Edit project settings             │
│     exit                                                │
│                                                         │
└───────── ↑/↓: navigate  enter: select  q: exit ────────┘
```

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

The status view is a split-pane interface: the left pane shows a tree of all feature and bug plans; the right pane shows context-sensitive tabs that adapt to whatever is selected in the left pane.

### Layout

```
┌──────────────────────────────────────────────────────────────────────┐
│  Items                        [1] Summary  [2] Details  [3] Metrics  │
│ ──────────────────────────  ─────────────────────────────────────── │
│  ✓ feature_001               feature_002                             │
│  ▸ feature_002               /features/feature_002.md                │
│    → TASK-002-001            ████████████░░░░░░░░░░░░░░░░ 2/5        │
│    ○ TASK-002-002                                                     │
│    ○ TASK-002-003            Done     2                               │
│  ○ feature_003               Pending  3                               │
│  ┃                           Blocked  0                               │
│  ⚠ bug_001                                                            │
│                              ────────────────────────────────────    │
│                              Tokens   142k                            │
│                              Cost     $0.38                           │
│                                                                       │
│──────────────────────────────────────────────────────────────────────│
│  1-3: tabs  ↑/↓: navigate  pgup/pgdn: prev/next feature  q: exit     │
└──────────────────────────────────────────────────────────────────────┘
```

The left pane displays a collapsible tree of all active plans. Each plan can be expanded to show its individual tasks with status icons:

| Icon | Meaning |
|---|---|
| `✓` | Complete |
| `→` | Next task the daemon will work on |
| `○` | Pending (not yet started) |
| `⚠` | Blocked (has a `BLOCKED:` criterion) |
| `[>]` | Skipped (has a `SKIPPED:` criterion) |
| `┃` | Separator between features and bugs |

### Context-Sensitive Tabs

The right pane tabs change based on what you have selected in the left pane. There are no dead tabs — every tab shown always has meaningful content.

| Selection | Available Tabs |
|---|---|
| Nothing selected | Metrics |
| Feature or bug plan | Summary · Details · Metrics |
| Running task (daemon active) | Output · Task Details · Metrics |
| Completed, pending, or blocked task | Summary · Output · Task Details · Metrics |

Tab numbers start at `[1]`. The footer shows the current tab range, e.g., `1-3: tabs` when there are three right-pane tabs.

### Feature Selected — Summary Tab

When a feature or bug plan is selected, the **Summary** tab gives a quick overview of that plan:

```
┌──────────────────────────────────────────────────────────────────────┐
│  Items                        [1] Summary  [2] Details  [3] Metrics  │
│ ──────────────────────────  ─────────────────────────────────────── │
│  ✓ feature_001               My Authentication Feature               │
│  ▸ feature_002               /features/feature_002.md                │
│    → TASK-002-001                                                     │
│    ○ TASK-002-002            ████████████░░░░░░░░░░░░░░░░ 2/5        │
│    ○ TASK-002-003                                                     │
│  ○ feature_003               Done     2                               │
│  ┃                           Pending  3                               │
│  ⚠ bug_001                   Blocked  0                               │
│                                                                       │
│                              ────────────────────────────────────    │
│                              ⠸  TASK-002-001    2m 14s               │
│                                                                       │
│                              Tokens   142k                            │
│                              Cost     $0.38                           │
│──────────────────────────────────────────────────────────────────────│
│  1-3: tabs  ↑/↓: navigate  pgup/pgdn: prev/next feature  q: exit     │
└──────────────────────────────────────────────────────────────────────┘
```

**Feature Summary** shows:
- Feature title and filename
- Progress bar with done/total count
- Task breakdown: done, pending, blocked counts
- If the daemon is actively working on a task in this feature: the task ID, a spinner, and elapsed time
- If running in parallel mode: a list of active workers with their status
- Aggregate token usage and cost across all tasks in the feature

### Running Task Selected — Output Tab

When you select a task that the daemon is currently working on, the **Output** tab becomes the first tab and shows the live tool invocation log:

```
┌──────────────────────────────────────────────────────────────────────┐
│  Items                        [1] Output  [2] Task Details  [3] Metrics│
│ ──────────────────────────  ─────────────────────────────────────── │
│  ✓ feature_001               Status:  ⠸  Running                     │
│  ▸ feature_002               Task:    TASK-002-001 - Add login page   │
│    ▸ TASK-002-001 (running)  ────────────────────────────────────    │
│    ○ TASK-002-002                                                     │
│    ○ TASK-002-003            Read       src/auth/login.go             │
│  ○ feature_003               Edit       src/auth/login.go             │
│  ┃                           Bash       go test ./internal/auth/...   │
│  ⚠ bug_001                   Read       src/auth/session.go           │
│                              Edit       src/auth/session.go           │
│                              Write      src/auth/middleware.go        │
│                              Bash       go build ./...                │
│                                                                       │
│                              ────────────────────────────────────    │
│                              Tokens   28k in / 4k out                 │
│                              Cost     $0.09                           │
│                              Run:     6m 42s                          │
│                              Task:    2m 14s                          │
│──────────────────────────────────────────────────────────────────────│
│  1-3: tabs  ↑/↓: navigate  shift+↑/↓: scroll  g: top  G: bottom  q: exit│
└──────────────────────────────────────────────────────────────────────┘
```

**Running Task Output** shows:
- Status with spinner animation
- Task ID and title
- Scrollable list of tool invocations (Read, Edit, Write, Bash, etc.) in chronological order
- Live token counts and cost
- Elapsed time for the current run and for this task specifically

The tool list auto-scrolls to follow the latest entry. Scroll up manually to pause auto-scroll and review earlier entries; scroll back to the bottom to resume.

### Completed Task Selected — Summary Tab

When you select a task that has already completed, the **Summary** tab shows the outcome and metrics for that specific task:

```
┌──────────────────────────────────────────────────────────────────────┐
│  Items              [1] Summary  [2] Output  [3] Task Details  [4] Metrics│
│ ──────────────────────────  ─────────────────────────────────────── │
│  ✓ feature_001               TASK-001-003                             │
│  ▸ feature_002               Add password reset endpoint              │
│  ▸ feature_001                                                        │
│    ✓ TASK-001-001            Status    ✓ Complete                     │
│    ✓ TASK-001-002            Duration  4m 28s                         │
│    ✓ TASK-001-003            Tokens    51k in / 7k out                │
│    ○ TASK-001-004            Cost      $0.17                          │
│  ○ feature_003               Model     claude-sonnet-4-6              │
│  ┃                           Commit    a3f9b12                        │
│  ⚠ bug_001                                                            │
│                                                                       │
│                                                                       │
│──────────────────────────────────────────────────────────────────────│
│  1-4: tabs  ↑/↓: navigate  pgup/pgdn: prev/next feature  q: exit     │
└──────────────────────────────────────────────────────────────────────┘
```

**Completed Task Summary** shows:
- Task ID and title
- Status (Complete, Pending, or Blocked)
- Duration of the run that completed the task
- Token usage (input/output) and cost
- Model used
- Commit hash (if a commit was recorded in the run log)

You can also switch to the **Output** tab for a completed task to browse its full tool invocation log, loaded from the run log files.

### Tab Reference

| Tab | Shown when | Content |
|---|---|---|
| **Summary** | Feature selected | Feature title, progress bar, task breakdown (done/pending/blocked), active daemon state, aggregate tokens/cost |
| **Summary** | Task selected (not running) | Task ID/title, status, duration, tokens, cost, model, commit hash |
| **Output** | Running task selected | Live tool invocation log with spinner, auto-scroll, and token/cost/elapsed display |
| **Output** | Completed task selected | Full tool invocation history loaded from run log files |
| **Details** | Feature selected | Flat task list for the selected plan with progress bar and task status icons |
| **Task Details** | Running or completed task | Read-only detail view of the next pending task (same as task detail view) |
| **Metrics** | Always | Token usage and cost broken down by selection, repository, and all-time global totals |

### Keyboard Shortcuts

#### Navigation (always active)

| Key | Action |
|---|---|
| `Up` / `Down` | Navigate plans and tasks in the tree |
| `PgUp` / `PgDn` | Jump to previous / next plan |
| `Left` / `Right` | Collapse / expand a plan |
| `Home` / `End` | Jump to first / last item |
| `a` | Approve / unapprove the selected plan |
| `x` | Skip / unskip the selected task |
| `Alt+R` | Run the selected task immediately |
| `Alt+D` | Delete the selected plan (with confirmation) |
| `Alt+Backspace` | Delete the selected task (with confirmation) |
| `Alt+A` | Toggle showing completed plans |
| `s` | Start / stop the daemon |
| `q` | Exit |

#### Switching Tabs

| Key | Action |
|---|---|
| `1` | Switch to first right-pane tab |
| `2` | Switch to second right-pane tab (if available) |
| `3` | Switch to third right-pane tab (if available) |
| `4` | Switch to fourth right-pane tab (if available) |

The tab numbers in the footer always reflect what is currently available. Keys beyond the available tab count are ignored.

#### Content Scrolling (Output and Task Details tabs)

| Key | Action |
|---|---|
| `Shift+Up` / `Shift+Down` | Scroll content one line |
| `g` | Jump to top |
| `G` | Jump to bottom (resumes auto-scroll for running tasks) |

The footer shows `shift+↑/↓: scroll` when the active tab has scrollable content.

#### Task Detail View (opened with Enter)

When you press `Enter` on a task row, an inline detail view opens in the right pane.

| Key | Action |
|---|---|
| `Up` / `Down` | Scroll the detail view |
| `PgUp` / `PgDn` | Previous / next task |
| `Tab` | Enter criteria mode (for blocked tasks) |
| `Alt+R` | Run the selected task |
| `Alt+Backspace` | Delete the selected task |
| `Backspace` / `q` | Back to tree |

### Managing Blocked Tasks

When viewing the **Details** tab and a task has blocked criteria, select the task and press **Enter** to open its detail view, then press **Tab** to enter **criteria mode**. Navigate between blocked criteria with **Up/Down** and press **Enter** to open the action picker.

The action picker offers options that vary depending on the criterion state:

| Action | Description |
|--------|-------------|
| **Unblock** | Removes the `BLOCKED:` prefix, turning it back into a normal unchecked criterion |
| **Resolve** | Marks the criterion as done (removes the block and checks it) |
| **Delete** | Removes the criterion entirely from the plan file |
| **Skip Task** | Adds a `SKIPPED:` prefix and marks the criterion with `[>]`; Maggus will not work on the task |
| **Unskip** | Restores a `SKIPPED:` criterion to a normal unchecked criterion (shown in place of Skip Task) |
| **Skip** | Leaves the criterion unchanged |

Changes are applied immediately to the plan file. Press **Esc** to go back.

---

## List Command

`maggus list` is a plain-text CLI command, not a TUI view. It prints upcoming workable tasks as tab-separated output to stdout — suitable for scripting or quick inspection in the terminal. Run `maggus list --help` for available flags.
