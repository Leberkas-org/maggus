# Feature 004: Dynamic 2x Status Caching

## Introduction

Replace the per-TUI-open API calls to `isclaude2x.com` with a single lazy-cached fetch. The first call to `FetchStatus()` within a process hits the API and stores the raw result with a timestamp. All subsequent calls compute the current `Is2x` state and remaining duration from that cache — no network required. TUI models that show the 2x indicator get a per-second tick that recomputes from cache while `Is2x` is true, making the countdown live.

## Goals

- Call the `isclaude2x.com` API exactly once per process lifetime (lazy — on first use)
- Compute `Is2x`, `TwoXWindowExpiresInSeconds`, and `TwoXWindowExpiresIn` dynamically from the cached result + elapsed time
- All TUI models (menu, status, config, repos, update) show a live per-second countdown when in 2x mode, without additional API calls
- `work.go` transparently benefits from the cache (no code change needed beyond the package)
- Existing tests continue to pass; new tests cover cache behavior and format logic

## Tasks

### TASK-004-001: Add lazy caching and dynamic computation to claude2x package
**Description:** As a developer, I want `FetchStatus()` to hit the API at most once per process and derive current status from elapsed time, so that all callers automatically get cached results without any changes.

**Token Estimate:** ~30k tokens
**Predecessors:** none
**Successors:** TASK-004-002
**Parallel:** no

**Acceptance Criteria:**
- [x] A package-level `sync.Once` ensures the HTTP fetch runs at most once per process lifetime
- [x] The raw `Status` and the `time.Time` of the fetch are stored in unexported package-level vars
- [x] `FetchStatus()` public signature is unchanged — callers require no modification
- [x] `FetchStatus()` triggers the lazy fetch via `sync.Once`, then delegates to an internal `computeStatus()` that returns a freshly derived `Status`:
  - If original `Is2x` was `false` (or fetch failed), return `Status{}` as before
  - Otherwise compute `remaining = TwoXWindowExpiresInSeconds - int(time.Since(fetchedAt).Seconds())`
  - If `remaining <= 0`: return `Status{Is2x: false}`
  - Else: return `Status{Is2x: true, TwoXWindowExpiresInSeconds: remaining, TwoXWindowExpiresIn: formatRemaining(remaining)}`
- [x] `formatRemaining(seconds int) string` produces a human-readable string matching the API format, e.g.:
  - 64484s → `"17h 54m 44s"`
  - 3723s → `"1h 2m 3s"`
  - 125s → `"2m 5s"`
  - 45s → `"45s"`
  - Omit zero components (no `"0h"` prefix), except when only seconds remain
- [x] An unexported `resetCache()` function exists for use in tests (resets `sync.Once` and cached vars)
- [x] Existing tests (`TestFetchStatus_*`) are updated to use a test server + `resetCache()` so they still exercise the real HTTP path
- [x] New tests cover:
  - `TestComputeStatus_StillActive`: cache set with 3600s remaining, 30 min elapsed → `Is2x=true`, remaining ≈ 1800s, formatted correctly
  - `TestComputeStatus_Expired`: cache set with 10s remaining, 11+ seconds elapsed → `Is2x=false`
  - `TestComputeStatus_WasNot2x`: original `Is2x=false` → always returns `Status{}`
  - `TestFormatRemaining_*`: covers hours+min+sec, min+sec only, sec only
- [x] `go fmt ./...` and `go vet ./...` pass

### TASK-004-002: Add per-second ticker to TUI models when 2x is active
**Description:** As a user, I want the 2x countdown in the TUI border/header to count down in real time without the app making repeated API calls, so I always see an accurate remaining time.

**Token Estimate:** ~50k tokens
**Predecessors:** TASK-004-001
**Successors:** none
**Parallel:** no

**Acceptance Criteria:**
- [ ] A new `claude2xTickMsg` type is defined in `package cmd` (e.g., in `menu.go` or a new `cmd/claude2x.go`)
- [ ] A helper `next2xTick() tea.Cmd` returns `tea.Tick(time.Second, ...)` that emits `claude2xTickMsg`
- [ ] `menu.go` — `Update` handles `claude2xResultMsg`: when `Is2x=true`, schedule `next2xTick()`; update `m.twoXExpiresIn` from `msg.status.TwoXWindowExpiresIn`
- [ ] `menu.go` — `Update` handles `claude2xTickMsg`: call `claude2x.FetchStatus()` (returns cached+recomputed), update `m.is2x` and `m.twoXExpiresIn`, schedule `next2xTick()` only if `m.is2x` is still true
- [ ] `status.go` — same `claude2xTickMsg` handling pattern; updates `m.is2x` and `m.BorderColor`
- [ ] `config.go` — same pattern; updates `m.is2x` only (no expiry text in config view)
- [ ] `repos.go` — same pattern; updates `m.is2x` only
- [ ] `update.go` — same pattern; updates `m.is2x` (check whether `twoXExpiresIn` is used there)
- [ ] `work.go` line 118 (`claude2x.FetchStatus()`) requires no change — it transparently uses the cache from TASK-004-001
- [ ] When `Is2x` flips from true to false mid-session, the ticker stops (no more `next2xTick()` scheduled) and border/display resets to normal
- [ ] No additional API calls are made during ticker ticks — only `FetchStatus()` which returns the cached computed value
- [ ] `go fmt ./...` and `go vet ./...` pass
- [ ] Existing TUI tests (`menu_test.go`, `status_test.go`, etc.) pass without modification or with minimal updates to send `claude2xTickMsg` where needed
- [ ] New unit tests cover:
  - Sending `claude2xTickMsg` to a model in 2x mode → `next2xTick()` scheduled again
  - Sending `claude2xTickMsg` to a model where cache has expired → no next tick scheduled, `is2x=false`

## Task Dependency Graph

```
TASK-004-001 ──→ TASK-004-002
```

| Task | Estimate | Predecessors | Parallel | Model |
|------|----------|--------------|----------|-------|
| TASK-004-001 | ~30k | none | no | — |
| TASK-004-002 | ~50k | 001 | no | — |

**Total estimated tokens:** ~80k

## Functional Requirements

- FR-1: `claude2x.FetchStatus()` must perform at most one HTTP request per process lifetime
- FR-2: The first call to `FetchStatus()` triggers the HTTP fetch; all subsequent calls return a value derived from the cached result and elapsed time
- FR-3: `Is2x` must flip to `false` automatically when `TwoXWindowExpiresInSeconds` worth of time has elapsed since the fetch, without any further API call
- FR-4: `TwoXWindowExpiresIn` must reflect the actual remaining seconds at the time of the call, formatted as `"Xh Ym Zs"` (omitting zero leading components)
- FR-5: When a TUI model receives a `claude2xResultMsg` with `Is2x=true`, it must schedule a per-second tick
- FR-6: On each `claude2xTickMsg`, the model calls `FetchStatus()` (cache-only), updates its state, and reschedules the tick only if `Is2x` is still true
- FR-7: `work.go` must not be changed — the cache transparently applies to its `FetchStatus()` call

## Non-Goals

- No live countdown in the runner TUI (`internal/runner`) — the work banner shows the value at work-start and is static
- No cache persistence across process restarts (in-memory only)
- No configurable cache TTL or forced refresh
- No change to the API response format or the `Status` struct fields

## Technical Considerations

- `sync.Once` is safe for concurrent use; no additional mutex needed for the cache vars since they're written exactly once inside `once.Do`
- `resetCache()` for tests must replace `sync.Once` with a new `sync.Once{}` value (e.g., via a package-level `var once sync.Once`) — this is the standard Go testing pattern for `sync.Once`
- `formatRemaining` should use integer arithmetic only — no `time.Duration` parsing required since the input is already seconds
- The ticker in TUI models adds ~1 goroutine per open TUI but only while `Is2x` is true; this is negligible
- `update.go` should be checked for whether it displays `twoXExpiresIn` text before deciding whether to wire up the expiry string update

## Success Metrics

- `maggus menu` opened 10 times in one process session makes exactly 1 HTTP request
- With 1 hour remaining on a 2x window, the countdown in the menu header decrements every second
- When the countdown reaches zero, the yellow border reverts to the default color without a restart
- All existing tests pass

## Open Questions

*None — all questions resolved.*
