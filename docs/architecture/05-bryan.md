# Bryan Integration (Optional)

Bryan is a C# gRPC backend that adds distributed coordination to maggus. When configured, it provides remote task dispatch, live dashboards, cross-machine memory sync, and usage reporting. Maggus works fully without it.

---

## Enabling Bryan

In `~/.maggus/config.yml`:

```yaml
bryan:
  address: "bryan.example.com:443"
  machine_id: "uuid-of-this-machine"
```

If the `bryan` section is absent or null, maggus runs in standalone mode. The daemon checks `bryan != nil` before any Bryan-specific operations.

---

## What Bryan Adds

| Feature | Standalone | With Bryan |
|---------|-----------|------------|
| Task source | Local plan files | Local files + Bryan-pushed TaskAssignments |
| Approval | TUI only | TUI + Bryan web dashboard |
| Live logs | Local files | Local + streamed to Bryan frontend |
| Usage reporting | Local log files | Local + reported via ReportUsage RPC |
| Memory sync | Local MEMORY.md only | Synced across machines via MemorySync |
| Multi-machine | N/A | Bryan coordinates agents across machines |

---

## gRPC Service (BryanAgentService)

Three RPCs defined in `src/protos/service.proto`:

1. **Connect** `(stream AgentMessage) returns (stream BryanMessage)` — bidi stream
   - Registration (first time) or authentication (reconnect)
   - Task assignments from Bryan
   - Status updates to Bryan
   - Lifecycle notifications (pause, resume, terminate)

2. **Log** `(stream LoggingMessage) returns (google.protobuf.Empty)` — client-streaming
   - LiveLogBatch entries for real-time dashboard
   - AgentLog entries for observability

3. **ReportUsage** `(ReportUsageRequest) returns (ReportUsageResponse)` — unary
   - Token counts, cost, model breakdown
   - Kind: work, prompt, bugreport, plan, vision, architecture

---

## MemorySync

`MEMORY.md` is synced across machines via Bryan:

```protobuf
message MemorySync {
    string repository_url = 1;
    string content = 2;
}
```

Added to both `AgentMessage` and `BryanMessage` oneofs. Same message, two directions:
- **On connect:** Bryan sends stored memory to agent
- **On session end:** Agent pushes updated memory to Bryan

Last-write-wins. Bryan stores per-repo.

---

## Bryan Client Interface

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
    Messages() <-chan *proto.BryanMessage
    Close() error
}
```

---

## How Bryan Integrates with the Daemon

When Bryan is connected, the daemon:
1. **Receives TaskAssignments** from Bryan → enqueues them alongside local plan files
2. **Reports status** back to Bryan (IN_PROGRESS, REVIEW, DONE, FAILED)
3. **Streams logs** to Bryan's Log RPC in addition to local log files
4. **Reports usage** via unary ReportUsage RPC after each task
5. **Syncs memory** on connect (pull) and session end (push)
6. **Handles notifications** — pause, resume, terminate from Bryan operators

Bryan-sourced tasks and local plan file tasks coexist in the same queue. The TUI shows both.

---

## Authentication

RSA 2048-bit challenge-response:
1. Agent generates key pair, stores in `~/.maggus/keys/`
2. First connect: registration flow with operator approval (6-char code)
3. Subsequent connects: sign Bryan's nonce with private key (PKCS1v15, SHA-256)

---

## Reconnection

Exponential backoff with jitter. On reconnect:
1. Re-authenticate
2. Re-report in-progress tasks
3. Re-sync memory
4. Resume log streaming
