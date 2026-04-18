# Package Structure

```
src/
├── main.go
├── cmd/
│   ├── root.go                        # (exists) cobra root — auto-starts daemon + TUI
│   ├── stop.go                        # `maggus stop <repo>` / `maggus stop -a`
│   └── version.go                     # `maggus version`
│
├── internal/
│   ├── config/
│   │   ├── config.go                  # Config struct, Load(), validation
│   │   └── repo.go                    # Multi-repo registry
│   │
│   ├── daemon/
│   │   ├── daemon.go                  # MainDaemon: lifecycle, file watcher, dispatch loop
│   │   ├── queue.go                   # TaskQueue: pending/ready/active item management
│   │   ├── dispatcher.go              # Queue → Worker assignment
│   │   ├── worker.go                  # Worker interface + WorkerPool (concurrency)
│   │   ├── task_worker.go             # Single task lifecycle: branch → agent → commit → merge
│   │   ├── parser.go                  # Plan file parser → WorkItem + TaskSpec
│   │   ├── state.go                   # DaemonState: observable state, atomic IPC snapshots
│   │   └── events.go                  # Internal event types
│   │
│   ├── bryan/
│   │   ├── client.go                  # BryanClient interface + gRPC impl
│   │   ├── connect.go                 # Connect stream handler (register, auth, dispatch)
│   │   ├── auth.go                    # RSA key mgmt, challenge-response
│   │   ├── logger.go                  # Log stream handler (LiveLogBatch, AgentLog)
│   │   ├── usage.go                   # ReportUsage unary call
│   │   └── memory.go                  # MemorySync handler
│   │
│   ├── agent/
│   │   ├── agent.go                   # Agent interface (Run, Name, Validate)
│   │   ├── output.go                  # OutputSink interface + event types
│   │   ├── claude.go                  # Claude Code subprocess
│   │   ├── opencode.go                # OpenCode subprocess
│   │   └── registry.go                # Agent registry (name → constructor)
│   │
│   ├── git/
│   │   ├── commander.go               # Commander interface — wraps exec.Command("git")
│   │   ├── branch.go                  # Create, checkout, delete, naming, IsProtected
│   │   ├── worktree.go                # Create, remove, list worktrees
│   │   ├── merge.go                   # Merge/rebase operations
│   │   ├── sync.go                    # Fetch, remote status
│   │   └── commit.go                  # Stage + commit
│   │
│   ├── ipc/
│   │   ├── ipc.go                     # StateWriter / StateReader / CommandWriter interfaces
│   │   ├── state_file.go              # Atomic JSON writes + reads (~/.maggus/state.json)
│   │   └── subscriber.go              # fsnotify watcher for TUI state updates
│   │
│   ├── tui/
│   │   ├── app.go                     # Top-level bubbletea model — owns panes, layout
│   │   ├── layout.go                  # Layout math: split ratios, divider rendering
│   │   ├── keys.go                    # Shared key bindings
│   │   ├── pane/                      # Pane interface + implementations (see tui.md)
│   │   ├── tab/                       # Tab interface + implementations (see tui.md)
│   │   ├── component/                 # Reusable widgets (see tui.md)
│   │   └── styles/
│   │       └── styles.go              # (exists) colors, reusable styles, layout helpers
│   │
│   ├── prompt/
│   │   └── prompt.go                  # Prompt builder: task context → agent prompt string
│   │
│   └── runlog/
│       └── runlog.go                  # Structured run logging (.maggus/logs/)
│
├── proto/                             # Generated Go code (protoc output, only if Bryan used)
│   ├── *.pb.go
│   └── *_grpc.pb.go
│
└── protos/                            # (exists) Proto source definitions
```

---

## Key Interfaces

### Config

```go
// internal/config/config.go

type Config struct {
    Agent         string            `yaml:"agent"`          // "claude" or "opencode"
    Model         string            `yaml:"model"`          // "sonnet", "opus", etc.
    MaxWorkers    int               `yaml:"max_workers"`    // parallel task limit
    AutoApprove   bool              `yaml:"auto_approve"`   // skip manual approval
    Git           GitConfig         `yaml:"git"`
    Bryan         *BryanConfig      `yaml:"bryan"`          // nil = standalone mode
    Notifications NotificationConfig `yaml:"notifications"`
}

type BryanConfig struct {
    Address   string `yaml:"address"`    // gRPC server address
    MachineID string `yaml:"machine_id"` // agent identity
}

func Load() (Config, error)                // loads ~/.maggus/config.yml
func LoadRepo(dir string) (Config, error)  // merges <repo>/.maggus/config.yml over global
```

### Git Operations

```go
// internal/git/commander.go

type Commander interface {
    Run(dir string, args ...string) error
    Output(dir string, args ...string) (string, error)
}

// internal/git/ (across branch.go, worktree.go, merge.go, sync.go, commit.go)

type Operations interface {
    CurrentBranch(dir string) (string, error)
    BranchExists(dir string, branch string) bool
    CreateBranch(dir, name, from string) error
    CheckoutBranch(dir, name string) error
    DeleteBranch(dir, name string) error
    IsProtected(branch string) bool

    CreateWorktree(repoRoot, path, branch string) error
    RemoveWorktree(repoRoot, path string) error
    ListWorktrees(repoRoot string) ([]WorktreeInfo, error)

    MergeTaskBranch(repoRoot, featureBranch, taskBranch string) error

    Commit(dir, message string) (hash string, err error)
    HasChanges(dir string) bool

    Fetch(dir string) error
    RepoURL(dir string) string
}
```

### Agent

```go
// internal/agent/agent.go

type Agent interface {
    Run(ctx context.Context, opts RunOptions) error
    Name() string
    Validate() error
}

type RunOptions struct {
    Prompt  string
    Model   string
    WorkDir string
    Output  OutputSink
}

// internal/agent/output.go

type OutputSink interface {
    OnStatus(status string)
    OnOutput(text string)
    OnTool(tool ToolEvent)
    OnUsage(usage UsageEvent)
    OnComplete(success bool)
}
```

### IPC

```go
// internal/ipc/ipc.go

type StateWriter interface {
    WriteState(state DaemonSnapshot) error
}

type StateReader interface {
    ReadState() (DaemonSnapshot, error)
    Watch(ctx context.Context) <-chan DaemonSnapshot
}

type CommandWriter interface {
    StopAll() error
    StopRepo(repoURL string) error
    Approve(itemID string) error
    Skip(itemID string) error
    Reorder(priorities map[string]int) error
}
```

### Bryan Client (Optional)

```go
// internal/bryan/client.go

type Client interface {
    Connect(ctx context.Context, machineID string, repos []string) error
    UpdateTaskStatus(taskID string, status proto.TaskStatus, msg string) error
    GetFeatureContext(featureID string) (*proto.FeatureContext, error)
    RequestNextTask() error
    LogStream(ctx context.Context) (LogSender, error)
    ReportUsage(ctx context.Context, req *proto.ReportUsageRequest) error
    SyncMemory(repoURL string, content string) error
    Close() error
}
```
