# Bug: `maggus start` and `maggus stop` operate on wrong repo when run from unregistered directory

## Summary

When `maggus start` or `maggus stop` is called from a directory that is not a registered repository (and not a git repo), the commands silently start/stop the daemon in the last-opened repository instead of doing nothing.

## Steps to Reproduce

1. Register at least one repository via `maggus repos` (e.g. `/home/user/my-project`)
2. Open that repo so it becomes `last_opened` in `~/.maggus/repositories.yml`
3. `cd /tmp` (or any directory that is not a git repo and not registered)
4. Run `maggus start`
5. Observe: daemon starts in `/home/user/my-project` instead of erroring or doing nothing

Same behavior with `maggus stop`.

## Expected Behavior

When the current directory is not a registered repository, `maggus start` and `maggus stop` should print a message like "Not in a registered repository" and exit without starting/stopping any daemon.

## Root Cause

The startup resolver in `src/internal/resolver/resolver.go:85-88` silently changes the working directory to `LastOpened` when the current directory is not a git repo:

```go
// Case 3: cwd is neither configured nor a git repo — try last_opened.
if cfg.LastOpened != "" && deps.DirExists(cfg.LastOpened) {
    return selectDir(cfg.LastOpened, absCwd, cfg, deps)
}
```

This `selectDir` call invokes `os.Chdir` to the last-opened repo directory. Then in `src/cmd/daemon_start.go:44-45` and `src/cmd/daemon_stop.go:35-36`, both `startCurrentDaemon` and `stopCurrentDaemon` call `os.Getwd()` — which now returns the last-opened repo path, not the user's original directory.

The resolver's fallback behavior is useful for interactive commands (menu, work, status), but `start` and `stop` should guard against operating on a repo the user didn't explicitly choose. Neither function validates that the resolved directory is a registered repository before proceeding.

## User Stories

### BUG-001-001: Guard `startCurrentDaemon` against unregistered directories

**Description:** As a user, I want `maggus start` to refuse to start a daemon when I'm not in a registered repository, so that I don't accidentally start daemons in the wrong repo.

**Acceptance Criteria:**
- [x] `startCurrentDaemon` checks whether the current working directory is a registered repository (present in `GlobalConfig.Repositories`)
- [x] If not registered, prints a user-friendly message (e.g. "Not in a registered repository. Use 'maggus repos' to add one.") and returns without starting a daemon
- [x] When run from a registered repo directory, behavior is unchanged
- [x] No regression in `--all` flag behavior
- [x] `go vet ./...` and `go test ./...` pass

### BUG-001-002: Guard `stopCurrentDaemon` against unregistered directories

**Description:** As a user, I want `maggus stop` to refuse to stop a daemon when I'm not in a registered repository, so that I don't accidentally stop daemons in the wrong repo.

**Acceptance Criteria:**
- [ ] `stopCurrentDaemon` checks whether the current working directory is a registered repository (present in `GlobalConfig.Repositories`)
- [ ] If not registered, prints a user-friendly message (e.g. "Not in a registered repository. Use 'maggus repos' to add one.") and returns without stopping a daemon
- [ ] When run from a registered repo directory, behavior is unchanged
- [ ] No regression in `--all` flag behavior
- [ ] `go vet ./...` and `go test ./...` pass
