# Graceful Shutdown (Ctrl+C) in Go gRPC Agents

## The Problem

Ctrl+C doesn't work — the process hangs and must be killed via Task Manager.

### Root Causes

1. **Signal handler set up too late** — if `signal.Notify` is called after a blocking operation like `grpc.Connect()` or `stream.Recv()`, Ctrl+C during that phase has no custom handler.

2. **`stream.CloseSend()` doesn't unblock `Recv()`** — `CloseSend()` only closes the client's send side. The receive side keeps blocking, waiting for server messages.

3. **Manual signal channel + goroutine races with process exit** — a goroutine that prints feedback and calls `cancel()` may not execute before the process terminates.

4. **No force-quit on second Ctrl+C** — if graceful shutdown stalls, the user is stuck.

## The Fix

### Use `signal.NotifyContext` before any blocking calls

```go
// BEFORE (broken)
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

stream, err := client.Run(ctx, ...)  // blocks — Ctrl+C not handled here!

sigCh := make(chan os.Signal, 1)
signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
go func() {
    <-sigCh
    _ = stream.CloseSend()
    cancel()
}()
```

```go
// AFTER (works)
ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
defer stop()

// Force-quit on second Ctrl+C: stop() resets signal handling to default,
// so the next SIGINT terminates the process immediately.
go func() {
    <-ctx.Done()
    stop()
}()

stream, err := client.Run(ctx, ...)  // Ctrl+C cancels ctx, Recv() unblocks
```

### Why this works

- `signal.NotifyContext` creates a context that cancels **immediately** when SIGINT/SIGTERM arrives.
- The gRPC stream is created with this context (`client.Connect(ctx)`), so all blocking `stream.Recv()` calls unblock when the context is cancelled.
- Signal handling is active **before** any blocking call, so Ctrl+C works at every stage.

### Print feedback in the main goroutine, not a background one

```go
// BEFORE (race — goroutine may not print before exit)
go func() {
    <-ctx.Done()
    log.Printf("Shutting down...")  // might never print
    stop()
}()

// AFTER (guaranteed to print)
go func() {
    <-ctx.Done()
    stop()  // only reset signal handling here
}()

// In the main flow, after each blocking call:
result, err := client.SomeBlockingCall(ctx, ...)
if err != nil {
    if ctx.Err() != nil {
        log.Printf("Disconnecting...")  // runs synchronously, always prints
        return nil
    }
    return err
}
```

### Checklist

- [ ] `signal.NotifyContext` is called **before** any blocking I/O
- [ ] The gRPC stream/connection is created with the cancellable `ctx`
- [ ] A background goroutine calls `stop()` after `ctx.Done()` to enable force-quit
- [ ] Every `ctx.Err()` check prints a shutdown message before returning
- [ ] No `os.Exit(0)` — use `return nil` so defers run cleanly
- [ ] No `stream.CloseSend()` needed — context cancellation handles everything

### Imports

```go
import (
    "context"
    "os/signal"
    "syscall"
)
```
