# Bug: `maggus start` and `maggus stop` only operate on current working directory

## Summary

`maggus stop` only stops the daemon for the current working directory instead of stopping all running daemons across all registered repos. Similarly, `maggus start` only starts a daemon for the current repo instead of starting daemons for all repos that have auto-start enabled. Both commands need an `--all` flag to operate across all registered repositories.

## Root Cause

Both `startCmd` and `stopCmd` in `src/cmd/daemon_start.go` and `src/cmd/daemon_stop.go` are hardcoded to use `os.Getwd()` as their only target directory.

**`maggus stop`** (`src/cmd/daemon_stop.go:17-18`):
```go
dir, err := os.Getwd()
```
It reads the PID file from only this single directory and stops only that daemon. There is no mechanism to iterate over all registered repos from `~/.maggus/repositories.yml`.

**`maggus start`** (`src/cmd/daemon_start.go:25-26`):
```go
dir, err := os.Getwd()
```
Same issue — it only starts a daemon in the current directory. There is no `--all` flag to iterate registered repos and start daemons for those with auto-start enabled.

The global config (`~/.maggus/repositories.yml`) already tracks all registered repos and their `AutoStartDisabled` preference via `globalconfig.Load()`, and the programmatic helpers `startDaemon(dir)` and `stopDaemonGracefully(dir)` already accept arbitrary directory paths. The wiring to iterate all repos is simply missing from the CLI commands.

## Steps to Reproduce

### For `maggus stop`:
1. Register two repos in maggus (via `maggus repos` → add)
2. Start daemons in both repos (cd to each, run `maggus start`)
3. From repo A, run `maggus stop`
4. Observe: only repo A's daemon is stopped; repo B's daemon keeps running
5. Expected: `maggus stop --all` should stop both daemons

### For `maggus start`:
1. Register two repos with auto-start enabled
2. Run `maggus start --all` from any directory
3. Observe: flag does not exist; only the current repo's daemon starts
4. Expected: daemons should start for all repos with auto-start enabled

## Expected Behavior

- `maggus stop --all` stops daemons in every registered repo that has a running daemon.
- `maggus start --all` starts daemons in every registered repo where auto-start is enabled and no daemon is already running.
- Without `--all`, both commands keep current behavior (operate on current working directory only).

## User Stories

### BUG-002-001: Add `--all` flag to `maggus stop` to stop all running daemons

**Description:** As a user, I want `maggus stop --all` to stop daemons across all registered repos so I don't have to cd into each project individually.

**Acceptance Criteria:**
- [x] `maggus stop --all` loads `globalconfig.Load()` and iterates all `Repositories`
- [x] For each repo with a running daemon, calls `stopDaemonGracefully(repo.Path)`
- [x] Prints status per repo (e.g. "Stopped daemon in /path/to/repo (PID 1234)" or "No daemon running in /path/to/repo")
- [x] Collects errors per repo and reports them without aborting the loop (best-effort: stop as many as possible)
- [x] `maggus stop` (without `--all`) retains current behavior — stops only the current directory's daemon
- [x] No regression in `maggus stop` single-repo behavior
- [x] `go vet ./...` and `go test ./...` pass

### BUG-002-002: Add `--all` flag to `maggus start` to start all auto-start-enabled daemons

**Description:** As a user, I want `maggus start --all` to start daemons for all registered repos that have auto-start enabled so I can spin up my entire workflow with one command.

**Acceptance Criteria:**
- [ ] `maggus start --all` loads `globalconfig.Load()` and iterates all `Repositories`
- [ ] For each repo where `repo.IsAutoStartEnabled()` returns true, calls `startDaemon(repo.Path)`
- [ ] Skips repos where auto-start is disabled (silently or with a note)
- [ ] Skips repos where a daemon is already running (no error, just a note)
- [ ] Prints status per repo (e.g. "Started daemon in /path/to/repo (PID 5678)" or "Skipped /path/to/repo (auto-start disabled)")
- [ ] `--model` and `--agent` flags are ignored when `--all` is used (daemons use per-repo config defaults), or returns an error if combined with `--all`
- [ ] `maggus start` (without `--all`) retains current behavior — starts only the current directory's daemon
- [ ] No regression in `maggus start` single-repo behavior
- [ ] `go vet ./...` and `go test ./...` pass
