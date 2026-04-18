# Daemon Architecture

The daemon is a long-lived background process that watches for plan files, manages a task queue, and dispatches work to agent subprocesses. It runs repo-independently from `~/.maggus/`.

---

## Task Queue & Approval

### Ingestion

The daemon watches `<repo>/.maggus/tasks/` directories across all registered repos using fsnotify. When a new or modified plan file is detected:

1. **Parse** the plan file → extract feature context (title, description, goals) + ordered task list
2. **Validate** the plan (required fields present, tasks parseable)
3. **Enqueue** as a work item with status:
   - `pending` if `auto_approve: false` (default) — requires user approval in TUI
   - `ready` if `auto_approve: true` — goes straight to the dispatch queue

### Approval (TUI)

When `auto_approve` is off, pending items appear in the TUI. The user can:
- **Approve** an item → moves to `ready`
- **Reorder** items → changes priority in the ready queue
- **Skip** an item → moves to `skipped` (ignored by dispatcher)
- **Delete** an item → removes from queue (plan file stays on disk)

### Dispatch

The dispatcher picks the highest-priority `ready` item and creates workers for its tasks. Tasks within an item are executed respecting their dependency order (predecessors). Independent tasks can run in parallel up to `max_workers`.

---

## Daemon Lifecycle

```
maggus / maggus -d
  │
  ├─ Load ~/.maggus/config.yml
  ├─ Scan registered repos for plan files
  ├─ Start file watchers on all <repo>/.maggus/tasks/ dirs
  ├─ Optionally connect to Bryan (if configured)
  ├─ Enter main loop:
  │    ├─ Watch for new/modified plan files → enqueue
  │    ├─ Watch for IPC commands (stop, approve, reorder)
  │    ├─ Dispatch ready items to worker pool
  │    └─ Write state.json snapshot on every state change
  └─ On stop: graceful shutdown (finish or cancel active workers)
```

---

## MainDaemon

```go
type MainDaemon struct {
    cfg        config.Config
    queue      *TaskQueue          // pending + ready items
    state      *State              // observable state, writes IPC snapshots
    dispatcher *Dispatcher         // queue → worker assignment
    pool       *WorkerPool         // manages concurrent workers
    gitOps     git.Operations      // git interface
    agents     agent.Registry      // agent factory
    bryan      bryan.Client        // nil if Bryan not configured
}

func New(cfg config.Config, gitOps git.Operations, agents agent.Registry) *MainDaemon
func (d *MainDaemon) Run(ctx context.Context) error
func (d *MainDaemon) Stop() error
```

Note: `bryan` field is nil when running standalone. The daemon checks `bryan != nil` before any Bryan-specific operations.

---

## TaskQueue

```go
type ItemStatus string

const (
    ItemPending  ItemStatus = "pending"
    ItemReady    ItemStatus = "ready"
    ItemActive   ItemStatus = "active"
    ItemDone     ItemStatus = "done"
    ItemFailed   ItemStatus = "failed"
    ItemSkipped  ItemStatus = "skipped"
)

type WorkItem struct {
    ID           string          // derived from plan file name
    PlanFile     string          // absolute path to the plan file
    RepoURL      string          // which repo this belongs to
    RepoPath     string          // local path
    Title        string          // parsed from plan
    Description  string          // feature context
    Tasks        []TaskSpec      // ordered task list from plan
    Status       ItemStatus
    Priority     int             // user-assigned, lower = higher priority
}

type TaskQueue struct {
    items  []*WorkItem
    mu     sync.RWMutex
}

func (q *TaskQueue) Enqueue(item *WorkItem)
func (q *TaskQueue) Approve(id string) error
func (q *TaskQueue) Skip(id string) error
func (q *TaskQueue) Reorder(id string, newPriority int) error
func (q *TaskQueue) NextReady() *WorkItem
func (q *TaskQueue) Pending() []*WorkItem
func (q *TaskQueue) All() []*WorkItem
```

---

## Dispatcher

```go
type Dispatcher struct {
    queue   *TaskQueue
    pool    *WorkerPool
    gitOps  git.Operations
    agents  agent.Registry
    cfg     config.Config
}
```

The dispatcher runs in a loop:
1. Check if pool has capacity (`active < max_workers`)
2. Pick `NextReady()` from queue
3. Resolve the local repo
4. Create `TaskWorker` for the next task in the item
5. Submit to pool
6. Repeat

---

## WorkerPool

```go
type WorkerPool struct {
    maxWorkers int
    active     map[string]*TaskWorker   // taskID → worker
    mergeMu    map[string]*sync.Mutex   // per feature branch, serializes merges
    mu         sync.Mutex
}

func (p *WorkerPool) Submit(w *TaskWorker) error
func (p *WorkerPool) Cancel(taskID string) error
func (p *WorkerPool) CancelRepo(repoURL string) error
func (p *WorkerPool) CancelAll() error
func (p *WorkerPool) Wait()
func (p *WorkerPool) ActiveCount() int
```

---

## TaskWorker

Each task runs as a goroutine. The agent subprocess (Claude Code, OpenCode) is the actual OS process.

```go
type TaskWorker struct {
    item      *WorkItem
    task      TaskSpec
    cfg       config.Config
    state     *WorkerState      // updated atomically, read by TUI via IPC
    gitOps    git.Operations
    agent     agent.Agent
    bryan     bryan.Client      // nil if standalone
}
```

### Worker Lifecycle

1. Update item status to `active`
2. Create task branch (feature branch → task branch)
3. Create git worktree for isolation
4. Build prompt (task description + feature context + MEMORY.md + bootstrap files)
5. Invoke agent subprocess in worktree directory
6. Stream output → state updates (for TUI) + local log file
7. If Bryan connected: also stream to Bryan Log RPC
8. On completion: commit changes
9. Rebase + fast-forward merge into feature branch (serialized via `mergeMu`)
10. Cleanup: remove worktree, delete task branch
11. If all tasks in item done: mark item as `done`

TaskWorker implements `OutputSink` — fans out to state + log (+ optionally Bryan).

---

## State & IPC

The daemon writes a `DaemonSnapshot` to `~/.maggus/state.json` on every meaningful state change. The TUI reads this file.

```go
type DaemonSnapshot struct {
    BryanConnected bool              `json:"bryan_connected"`
    Repos          []RepoSnapshot    `json:"repos"`
    Queue          []QueueItem       `json:"queue"`
    Workers        []WorkerSnapshot  `json:"workers"`
    ActiveTasks    int               `json:"active_tasks"`
    UpdatedAt      time.Time         `json:"updated_at"`
}

type QueueItem struct {
    ID       string     `json:"id"`
    Title    string     `json:"title"`
    RepoURL  string     `json:"repo_url"`
    Status   string     `json:"status"`
    Priority int        `json:"priority"`
    Tasks    int        `json:"tasks"`       // total task count
    Done     int        `json:"done"`        // completed tasks
}

type WorkerSnapshot struct {
    TaskID      string     `json:"task_id"`
    ItemID      string     `json:"item_id"`
    TaskTitle   string     `json:"task_title"`
    RepoURL     string     `json:"repo_url"`
    Status      string     `json:"status"`       // "branching", "running agent", "committing", "merging"
    AgentOutput string     `json:"agent_output"`  // last N lines
    TokenUsage  TokenUsage `json:"token_usage"`
    StartedAt   time.Time  `json:"started_at"`
}
```

### IPC Commands (TUI → Daemon)

Sentinel files in `~/.maggus/`:
- `cmd.stop` — stop all
- `cmd.stop.<repo>` — stop tasks for repo
- `cmd.approve.<item_id>` — approve an item
- `cmd.skip.<item_id>` — skip an item
- `cmd.reorder` — JSON file with new priority list
