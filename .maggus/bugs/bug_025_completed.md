<!-- maggus-id: cc9f0354-9600-4cdd-960a-4203878af45e -->
# Bug: Daemon auto-starts from main menu even when not configured for auto-start

## Summary

Opening the main menu (`maggus` with no args) always triggers `autoStartDaemon()`, which starts the daemon even when the repository is not registered in global config or has no auto-start preference. The daemon should only auto-start for repos that are registered and have auto-start enabled.

## Steps to Reproduce

1. Open a directory that is NOT registered in `~/.maggus/repositories.yml` (or one where auto-start is not explicitly enabled)
2. Run `maggus` (no args) to open the main menu
3. Observe: the daemon starts in the background

## Expected Behavior

The daemon should only auto-start when the repository is registered in global config and has `auto_start_disabled: false` (or the field is absent, since auto-start is enabled by default for registered repos). If the repo is not registered, the daemon must not start.

## Root Cause

The guard logic in `autoStartDaemon()` at `src/cmd/daemon_start.go:247-262` has a fall-through bug:

```go
func autoStartDaemon(dir string) error {
    if cfg, err := globalconfig.Load(); err == nil {
        absDir, _ := filepath.Abs(dir)
        for _, repo := range cfg.Repositories {
            if repo.Path == absDir {
                if !repo.IsAutoStartEnabled() {
                    return nil // ← only exits here if repo found AND auto-start disabled
                }
                break
            }
        }
    }
    return startDaemon(dir) // ← always reached if repo is NOT in config
}
```

The `for` loop only guards the case where the repo IS found and has auto-start explicitly disabled. In two other cases, the function falls through to `startDaemon()`:

1. **Repo not registered in global config** — the loop finds no match, never hits the `break`, and falls through.
2. **`globalconfig.Load()` fails** — the entire `if` block is skipped, and `startDaemon()` runs unconditionally.

The fix is to invert the logic: only call `startDaemon()` when the repo is found AND `IsAutoStartEnabled()` returns true.

## User Stories

### BUG-025-001: Fix autoStartDaemon to only start when repo has auto-start enabled

**Description:** As a user, I want the daemon to only auto-start when my repository is registered and configured for auto-start, so that opening the main menu doesn't spawn an unwanted background process.

**Acceptance Criteria:**
- [x] `autoStartDaemon()` returns `nil` (no-op) when the repo is not found in global config
- [x] `autoStartDaemon()` returns `nil` when `globalconfig.Load()` fails
- [x] `autoStartDaemon()` returns `nil` when the repo is found but `IsAutoStartEnabled()` returns false
- [x] `autoStartDaemon()` calls `startDaemon()` only when the repo is found and `IsAutoStartEnabled()` returns true
- [x] Existing test coverage for `autoStartDaemon` is updated or added to cover all three no-op cases
- [x] No regression in `maggus start` (explicit start command still works regardless of auto-start setting)
- [x] No regression in `maggus start --all` (still respects per-repo auto-start preference)
- [x] `go vet ./...` and `go test ./...` pass
