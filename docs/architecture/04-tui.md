# TUI Architecture

The TUI is a separate process that reads daemon state via IPC (`~/.maggus/state.json`) and sends commands via sentinel files. It never runs in the daemon process.

---

## Layout

```
┌──────────────────────────────────────────┐
│ Left Pane (nav) │ Right Pane (tabs)      │
│                 │                        │
│  Repos          │  [Output] [Log] [Met]  │
│  v maggus       │                        │
│    v F-12       │  Agent output here...  │
│      TASK-001 ✓ │                        │
│      TASK-002 → │                        │
│      TASK-003 ○ │                        │
│                 │                        │
├──────────────────────────────────────────┤
│ Footer: status + keybind hints           │
└──────────────────────────────────────────┘
```

- **Left pane**: `min(width/3, 50)` — navigation tree
- **Right pane**: remainder — tabbed content
- **Footer**: 1 line — status bar + keybind hints
- **Divider**: `│` column between left and right

---

## Base Pane Abstraction

All panes implement a shared interface. `BasePane` provides common layout and focus logic.

```go
// internal/tui/pane/pane.go

type Pane interface {
    Init() tea.Cmd
    Update(msg tea.Msg) (Pane, tea.Cmd)
    View() string
    Resize(width, height int)
    Focus()
    Blur()
    IsFocused() bool
}

type BasePane struct {
    width, height int
    focused       bool
}
```

---

## App Model

```go
// internal/tui/app.go

type App struct {
    width, height int
    left          pane.Pane       // navigation tree
    right         pane.Pane       // tabbed content
    footer        pane.Pane       // status bar
    focus         FocusTarget     // Left or Right
    state         ipc.StateReader // reads daemon state
    modal         Modal           // nil when no modal active
}
```

- `WindowSizeMsg` → `Resize()` all panes
- `Tab` key switches focus between left and right
- Key messages only routed to focused pane
- Modal overlays (quit dialog) capture all input when active

---

## Left Pane — Navigation Tree

Shows repos, work items, and tasks in a collapsible tree. Data comes from `DaemonSnapshot`.

```
 Repos
 v leberkas-org/maggus
   v F-12: Auth system         [active]
     > TASK-001: Setup         [done]  ✓
     > TASK-002: Login         [running] →
     > TASK-003: Tokens        [ready] ○
   v BUG-001: Fix crash        [pending] ⏳
 v leberkas-org/bryan
   > F-05: Dashboard           [ready] ○
```

### Tree Component

```go
// internal/tui/component/tree.go

type TreeNode struct {
    ID       string
    Label    string
    Icon     string       // ✓, →, ○, ⚠, ⏳, etc.
    Children []*TreeNode
    Expanded bool
}

type Tree struct {
    nodes    []*TreeNode
    cursor   int
    offset   int          // scroll offset
    width, height int
}
```

**Navigation:** ↑/↓ move cursor, Left collapse, Right expand, Enter select.

### Status Icons

| Icon | Meaning |
|------|---------|
| ✓ | Done |
| → | Running |
| ○ | Ready |
| ⏳ | Pending (awaiting approval) |
| ⚠ | Failed |
| ⏭ | Skipped |

---

## Right Pane — Tabbed Content

Tabs change based on what's selected in the left pane.

```go
// internal/tui/tab/tab.go

type Tab interface {
    Name() string
    Update(msg tea.Msg) (Tab, tea.Cmd)
    View(width, height int) string
    SetData(data interface{})
}
```

```go
// internal/tui/pane/right.go

type RightPane struct {
    pane.BasePane
    tabs      []tab.Tab
    activeTab int
    context   SelectionContext
}
```

### Context-Sensitive Tabs

| Selection | Available Tabs |
|-----------|---------------|
| Work item (pending) | Plan |
| Work item (active) | Plan, Output, Metrics |
| Work item (done) | Plan, Summary, Output, Log, Metrics |
| Running task | Output, Log, Metrics |
| Completed task | Summary, Output, Log, Metrics |
| Failed task | Summary, Output, Log, Metrics |

### Tab Descriptions

- **Plan** — feature description, goals, task list with acceptance criteria
- **Output** — live agent output with spinner for running tasks, scrollable history for completed
- **Log** — activity log (tool invocations: Read, Edit, Bash, etc.)
- **Metrics** — token counts, cost breakdown by model, duration
- **Summary** — completed run summary (status, duration, commit hash, total cost)

**Tab switching:** Number keys 1-5, or left/right arrows when tab bar is focused.

---

## Footer

```go
// internal/tui/pane/footer.go

type FooterPane struct {
    pane.BasePane
    statusText string    // "Daemon running | 2 tasks active | Bryan: disconnected"
    keyHints   string    // "tab: switch pane  a: approve  x: skip  q: quit"
}
```

Footer never receives focus. Key hints update based on context (what's selected, what actions are available).

---

## Quit Modal

When user presses q/Ctrl+C:

```
┌─────────────────────────────┐
│  Stop daemon or detach?     │
│                             │
│  [S] Stop everything        │
│  [D] Detach (daemon stays)  │
│  [Esc] Cancel               │
└─────────────────────────────┘
```

---

## Reusable Components

| Component | File | Purpose |
|-----------|------|---------|
| Tree | `component/tree.go` | Generic collapsible tree with keyboard nav |
| Viewport | `component/viewport.go` | Scrollable content (Shift+↑/↓, g/G) |
| Spinner | `component/spinner.go` | Braille spinner (wraps `styles.SpinnerFrames`) |
| Progress | `component/progress.go` | Progress bar (wraps `styles.ProgressBar`) |

---

## TUI ↔ Daemon Communication

**Reading state:** TUI polls `~/.maggus/state.json` every 500ms (or uses fsnotify).

**Sending commands:** TUI writes sentinel files:
- Approve item: `~/.maggus/cmd.approve.<item_id>`
- Skip item: `~/.maggus/cmd.skip.<item_id>`
- Reorder: `~/.maggus/cmd.reorder` (JSON with priority map)
- Stop repo: `~/.maggus/cmd.stop.<repo>`
- Stop all: `~/.maggus/cmd.stop`

Daemon watches for these files, processes the command, deletes the sentinel.
