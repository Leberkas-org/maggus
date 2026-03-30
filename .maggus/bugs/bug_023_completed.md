<!-- maggus-id: 42794726-3fb9-4990-b2b7-10824c2e10ee -->
# Bug: No quit prompt when exiting status view with daemon running but no active task

## Summary

Pressing `q` to exit the status view while the daemon is running (but idle — no active task) exits immediately without showing the "Daemon is running" prompt. The prompt is expected to appear so the user can choose to stop the daemon, kill it, or keep it running.

## Steps to Reproduce

1. Start the daemon from the status view with `s`
2. Wait for the daemon to finish its current task (so `CurrentTask` becomes empty — daemon is running but idle)
3. Press `q` to exit the status view
4. Observe: status view exits immediately with no prompt

## Expected Behavior

The exit daemon overlay should appear:
`Exiting... Daemon is running:  [s] stop after task  [k / ctrl+c] kill now  [esc] cancel`

## Root Cause

`shouldPromptOnExit()` in `src/cmd/status_update.go:444` reads the global config and checks whether auto-start is enabled for the repo, instead of using the already-live `m.daemon.Running` state from the daemon state cache. This introduces multiple failure modes: config load failure, repo not found in config, and auto-start enabled all silently return `false` — skipping the prompt even when the daemon is actively running.

`m.daemon.Running` is the authoritative source (updated via `daemonCacheUpdateMsg`). The entire config lookup is unnecessary. The function should be:

```go
func (m statusModel) shouldPromptOnExit() bool {
    return m.daemon.Running
}
```

## User Stories

### BUG-023-001: Simplify shouldPromptOnExit to use daemon.Running directly

**Description:** As a user, I want the status view to always show the exit-daemon prompt when the daemon is running and I press `q`, so I'm never surprised by an orphaned daemon process.

**Acceptance Criteria:**
- [x] `shouldPromptOnExit()` is simplified to `return m.daemon.Running`
- [x] All global config loading and auto-start checks are removed from `shouldPromptOnExit()`
- [x] Exit daemon overlay appears when pressing `q` with daemon running and no active task
- [x] Exit daemon overlay appears when pressing `q` with daemon running and an active task
- [x] No prompt appears when daemon is not running
- [x] No regression in the stop/kill/cancel overlay options (`s`, `k`, `esc`)
- [x] `go vet ./...` and `go test ./...` pass
