# Feature 006: Global Lifetime Metrics

## Introduction

Maggus will persistently track lifetime usage counters across all repositories and sessions, stored globally in `~/.maggus/config.yml` under a `metrics:` key. Counters increment atomically on every relevant event — no UI is added; the metrics are written silently and are readable by inspecting the config file or querying programmatically in the future.

### Architecture Context

- **Components involved:** `internal/globalconfig` (struct + atomic load/save), `cmd/root.go` (startup counter), `cmd/work.go` + `cmd/work_loop.go` + `cmd/work_task.go` (all work-loop counters)
- **Storage:** Extends the existing `Settings` struct with a `Metrics` sub-struct; persists alongside `auto_update` in `~/.maggus/config.yml`
- **New pattern:** Atomic load-increment-save with a lockfile (`~/.maggus/config.lock`) to handle concurrent sessions (worktree mode)

## Goals

- Record 10 lifetime counters that grow monotonically and survive across machines (via sync)
- Writes are safe under concurrent Maggus sessions (worktree mode)
- Zero user-visible change to any existing command output or behaviour
- Metrics survive upgrades and unknown fields in the config file (use `omitempty` to avoid noise)

## Tasks

### TASK-006-001: Add `Metrics` struct to `globalconfig` and implement atomic increment helper

**Description:** As the Maggus runtime, I want a thread-safe, process-safe way to increment named counters in `~/.maggus/config.yml` so that lifetime metrics accumulate correctly even when multiple sessions run at the same time.

**Token Estimate:** ~50k tokens
**Predecessors:** none
**Successors:** TASK-006-002
**Parallel:** no — TASK-006-002 depends on this

**Acceptance Criteria:**
- [x] A `Metrics` struct is added in `src/internal/globalconfig/globalconfig.go` with these `int64` fields (all `yaml:"...,omitempty"` and `json:"-"`):
  - `StartupCount`, `WorkRuns`, `TasksCompleted`, `TasksFailed`, `TasksSkipped`
  - `FeaturesCompleted`, `BugsCompleted`, `TokensUsed`, `AgentErrors`, `GitCommits`
- [x] `Settings` gains a field `Metrics Metrics yaml:"metrics,omitempty"`
- [x] A function `IncrementMetrics(delta Metrics) error` is added to the `globalconfig` package. It:
  1. Acquires `~/.maggus/config.lock` using `O_CREATE|O_EXCL` (same pattern as `tasklock.go`); retries up to 10 times with 50 ms sleep between attempts; returns error after timeout
  2. Reads current settings via `LoadSettings()`
  3. Adds each field of `delta` to the corresponding field in `settings.Metrics`
  4. Writes to `~/.maggus/config.yml.tmp` then renames to `~/.maggus/config.yml` (atomic on all target platforms)
  5. Removes the lockfile
- [x] Lock acquisition and release are deferred-safe (lock is always released even if write fails)
- [x] Unit tests cover: zero delta is a no-op (no file write needed), single increment persists correctly, concurrent increments from two goroutines do not corrupt the count (run 50 goroutines each adding 1 to `WorkRuns`, assert final value is 50)
- [x] `go test ./internal/globalconfig` passes
- [x] `go vet ./...` passes

---

### TASK-006-002: Wire metric increments into all callsites

**Description:** As Maggus, I want every relevant event to increment the appropriate lifetime counter so that the metrics reflect real usage accurately.

**Token Estimate:** ~55k tokens
**Predecessors:** TASK-006-001
**Successors:** none
**Parallel:** no

**Acceptance Criteria:**

**`startup_count`:**
- [x] Incremented once per process in a `PersistentPreRun` hook on the root command (`src/cmd/root.go`). Fires for every subcommand invocation (`work`, `list`, `status`, `config`, etc.)
- [x] Failures to write metrics are logged to stderr at most once and never abort the command

**`work_runs`:**
- [x] Incremented at the start of `workCmd.RunE` in `src/cmd/work.go`, before the work loop begins

**`tasks_completed`:**
- [x] Incremented in `runWorkGoroutine` (`src/cmd/work_loop.go`) each time `result.committed` is `true` (same location as the existing `completed++` counter)

**`tasks_failed`:**
- [x] Incremented in `runWorkGoroutine` each time a task is added to `failedTasks`

**`tasks_skipped`:**
- [x] Incremented in `runWorkGoroutine` for each task logged as ignored at the start of the loop (the existing ignored-task log block)

**`features_completed` and `bugs_completed`:**
- [x] `MarkCompletedFeatures` and `MarkCompletedBugs` in `src/internal/parser/parser.go` are updated to return `(int, error)` — the integer is the count of files actually renamed or deleted
- [x] In `src/cmd/work_task.go`, the returned counts are used to populate `FeaturesCompleted` and `BugsCompleted` in a single `IncrementMetrics` call after both functions run
- [x] **Note:** If TASK-005-002 has already changed these signatures, integrate cleanly rather than re-changing them. Both changes (return count + accept action string) must coexist.

**`tokens_used`:**
- [x] Incremented inside `setupUsageCallback` in `src/cmd/work_loop.go`, summing `tu.InputTokens + tu.OutputTokens + tu.CacheCreationInputTokens + tu.CacheReadInputTokens` for each completed task usage event

**`agent_errors`:**
- [x] Incremented in `runWorkGoroutine` when the agent subprocess returns a non-nil error or the task result action is `taskBreak` due to an agent error (not a user stop/ctrl+c)

**`git_commits`:**
- [x] Incremented in `src/cmd/work_task.go` after a successful `gitcommit.Commit` call

**General:**
- [x] All `IncrementMetrics` calls use a `Metrics` struct with only the relevant fields set (all others remain zero so they add nothing)
- [x] All increment errors are silently swallowed (log to stderr at debug level only) — metric failures must never abort a work run
- [x] `go build ./...` passes
- [x] `go test ./...` passes

---

## Task Dependency Graph

```
TASK-006-001 ──→ TASK-006-002
```

| Task | Estimate | Predecessors | Parallel | Model |
|------|----------|--------------|----------|-------|
| TASK-006-001 | ~50k | none | no | — |
| TASK-006-002 | ~55k | 001 | no | — |

**Total estimated tokens:** ~105k

## Functional Requirements

- FR-1: The `metrics:` block must appear in `~/.maggus/config.yml` after the first metric-incrementing event; it must not appear in a freshly-installed config with no activity
- FR-2: All counter fields use `omitempty` so zero-valued counters are omitted from the YAML file, keeping the file clean
- FR-3: `IncrementMetrics` must be safe to call concurrently from multiple OS processes (worktree mode spawns parallel Maggus instances)
- FR-4: The lockfile approach must not leave a stale lock indefinitely — the lock must be removed in a deferred cleanup, and any lock older than 30 seconds is considered stale and overwritten (much shorter than the 2-hour tasklock threshold since metric writes are fast)
- FR-5: Metric increments that fail (e.g. no write permission to `~/.maggus/`) must not surface as errors to the user or abort any command
- FR-6: `tokens_used` accumulates the sum of all four token categories (input, output, cache creation, cache read) per task
- FR-7: The `MarkCompletedFeatures` and `MarkCompletedBugs` return signature change (`(int, error)`) must be backwards-compatible with the TASK-005 changes if already applied

## Non-Goals

- No `maggus stats` display command — metrics are read by inspecting the file directly
- No per-repository breakdown — these are global totals only
- No metric resets or history — counters only go up
- No export to external systems (Prometheus, InfluxDB, etc.)
- No `last_run_at`, `first_run_at`, or `worktree_runs` counters

## Technical Considerations

- **Atomic rename on Windows:** `os.Rename` on Windows fails if the destination file exists and is locked by another process. Use `os.Remove(dest)` before `os.Rename(tmp, dest)` as a fallback only on Windows, guarded by `runtime.GOOS == "windows"`. The lockfile prevents concurrent writers, so the remove+rename window is safe.
- **TASK-005 interaction:** TASK-005-002 also modifies `MarkCompletedFeatures` / `MarkCompletedBugs` signatures. If TASK-005 is completed first, TASK-006-002 must integrate with the already-changed signatures (adding a return count to a function that already accepts an `action string`). If TASK-006 runs first, TASK-005-002 must account for the return count. Read the current signatures before making changes.
- **`omitempty` on int64:** Go's `yaml.v3` respects `omitempty` on integer fields (zero is omitted). Verify this with a quick test to avoid surprising YAML output.
- **Lock file path:** `~/.maggus/config.lock` — in the same directory as the config file. Delete it in a `defer` so it is always cleaned up.

## Success Metrics

- After 10 `maggus work` runs completing 3 tasks each, `~/.maggus/config.yml` shows `work_runs: 10` and `tasks_completed: 30`
- Running `maggus work` in parallel worktree mode with 2 sessions, each completing 5 tasks, results in `tasks_completed: 10` (not 8 or 9 from a race)
- `startup_count` increments on `maggus list`, `maggus status`, and `maggus config` as well as `maggus work`
