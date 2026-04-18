# Observability

Maggus is a zero-setup binary. Observability features activate only when Bryan is connected — no local collector, no config, no extra infrastructure.

---

## Two Systems, Different Purposes

| System | Purpose | When Active | Transport |
|--------|---------|-------------|-----------|
| **Logs** | Real-time display in TUI + Bryan frontend | Always (local files), + Bryan when connected | Local files + Log RPC (client-streaming) |
| **OTel (traces + metrics)** | Dashboards, performance analysis, cost tracking in Bryan | Only when Bryan is connected | OTLP over gRPC (reuses Bryan connection) |

---

## Logging

Logs are always written locally — the TUI depends on them for real-time display. When Bryan is connected, they are additionally streamed via the Log RPC.

### Local Logs

Written to `<repo>/.maggus/logs/<item_id>/<pid>.log` as JSON-per-line matching the `LiveLogEntry` proto format:

```json
{"ts":"...","level":"info","event":"task_start","item_id":"...","task_id":"TASK-001","title":"Create the projects"}
{"ts":"...","level":"info","event":"tool_use","item_id":"...","task_id":"TASK-001","tool":"Bash","input":{...}}
{"ts":"...","level":"output","event":"output","item_id":"...","task_id":"TASK-001","text":"Running tests..."}
{"ts":"...","level":"info","event":"task_complete","item_id":"...","task_id":"TASK-001","commit":"af95451"}
{"ts":"...","level":"info","event":"task_usage","item_id":"...","task_id":"TASK-001","input_tokens":22,...}
```

### TUI Consumption

TUI watches log files via fsnotify. New lines are parsed and displayed in the Output and Log tabs in real-time.

### Bryan Streaming

When connected, the daemon batches log entries and sends them via `rpc Log(stream LoggingMessage) returns (google.protobuf.Empty)`. Bryan persists them and relays to the web frontend for live display.

---

## OpenTelemetry

OTel is initialized only when Bryan is connected. The Bryan gRPC endpoint serves as the OTLP exporter target. When Bryan is not configured, the OTel SDK is not initialized — zero overhead.

### Initialization

```go
// internal/daemon/daemon.go (during startup)

if cfg.Bryan != nil {
    // Initialize OTel with OTLP gRPC exporter pointed at Bryan
    // Bryan exposes an OTLP-compatible collector endpoint
    shutdown := initOTel(cfg.Bryan.Address)
    defer shutdown()
}
```

No environment variables, no config files, no local collector. Bryan's address is the only thing needed.

### Traces

Spans instrument the daemon and worker lifecycle:

```
work_item (root span)
├── import
│   ├── parse_plan
│   └── split_tasks
├── task_001
│   ├── create_branch
│   ├── create_worktree
│   ├── build_prompt
│   ├── run_agent          (long span — duration of agent subprocess)
│   │   ├── tool: Read
│   │   ├── tool: Edit
│   │   ├── tool: Bash
│   │   └── ...
│   ├── commit
│   ├── rebase
│   ├── merge
│   └── cleanup_worktree
├── task_002
│   └── ...
└── task_003
    └── ...
```

Key span attributes:
- `item.id`, `item.title`, `repo.url`
- `task.id`, `task.title`, `task.status`
- `agent.name`, `agent.model`
- `git.branch`, `git.worktree_path`, `git.commit_hash`

### Metrics

Counters and gauges exported via OTLP:

| Metric | Type | Description |
|--------|------|-------------|
| `maggus.tasks.total` | Counter | Total tasks processed (by status: done, failed, skipped) |
| `maggus.tasks.active` | Gauge | Currently running tasks |
| `maggus.queue.depth` | Gauge | Items in ready queue |
| `maggus.queue.pending` | Gauge | Items awaiting approval |
| `maggus.tokens.input` | Counter | Total input tokens consumed (by model) |
| `maggus.tokens.output` | Counter | Total output tokens consumed (by model) |
| `maggus.tokens.cache_read` | Counter | Cache read tokens (by model) |
| `maggus.tokens.cache_creation` | Counter | Cache creation tokens (by model) |
| `maggus.cost.usd` | Counter | Total cost in USD (by model, kind) |
| `maggus.agent.duration_seconds` | Histogram | Agent subprocess duration |
| `maggus.task.duration_seconds` | Histogram | Full task duration (branch to cleanup) |
| `maggus.git.merge.duration_seconds` | Histogram | Merge/rebase duration |
| `maggus.git.merge.conflicts` | Counter | Merge conflicts encountered |

### What Bryan Does With It

Bryan receives traces and metrics via its OTLP collector endpoint and can:
- Show task timelines and waterfall views in the web dashboard
- Track cost per feature, per repo, per agent over time
- Alert on anomalies (long-running tasks, high failure rates, cost spikes)
- Aggregate metrics across multiple maggus agents on different machines

---

## Package Structure

```
src/internal/
  otel/
    otel.go              # initOTel(bryanAddr) → shutdown func
    traces.go            # span helpers: StartWorkItemSpan, StartTaskSpan, etc.
    metrics.go           # meter + instrument definitions
```

The `otel` package is only imported by `daemon.go`. Agent, git, and TUI packages have zero OTel dependency. The daemon passes span contexts down to workers; workers create child spans.

---

## Standalone Mode

When Bryan is not configured:
- No OTel SDK initialized
- No traces, no metrics exported
- Logs still written to local files
- TUI still reads logs via fsnotify
- Zero overhead, zero dependencies beyond the log writer
