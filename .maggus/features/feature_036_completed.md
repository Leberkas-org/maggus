<!-- maggus-id: 7df3f5e6-c931-40e8-884e-53bb0a1c6f51 -->
# Feature 036: Rename `work` command to `run`

## Introduction

The hidden `work` cobra command is a legacy name from when users invoked it directly. It is now an internal execution entry point only — invoked by `maggus start` (as `work --daemon-run`) and by the status screen (as `work --task <id>`). If called directly without flags it just prints "Use 'maggus start' to start the daemon." Renaming it to `run` better reflects its actual role: running the agent work loop.

### Architecture Context

- **Vision alignment:** Internal quality improvement; no user-facing behavior changes.
- **Components involved:** `cmd/work.go` (Work Loop entry point), `cmd/daemon_start.go`, `cmd/status_update.go`, `cmd/dispatch.go`, and all `work_*.go` source files.
- **Architecture note:** `ARCHITECTURE.md` currently lists `cmd/work.go` as the Work Loop component — must be updated to `cmd/run.go`.

## Goals

- Rename the cobra command `Use` from `"work"` to `"run"` so internal invocations use `maggus run ...`
- Rename all Go source files from `work*.go` to `run*.go`
- Rename the exported symbols (`workCmd` → `runCmd`, `resetWorkFlags` → `resetRunFlags`, `workConfig` → `runConfig`, `workSetup` → `runSetup`, `workLoopParams` → `runLoopParams`)
- Update `ARCHITECTURE.md` to reference `cmd/run.go`
- All tests pass; build succeeds

## Tasks

### TASK-036-001: Rename cobra command and core symbols in work.go → run.go
**Description:** As a developer, I want the cobra command renamed from `work` to `run` and all core symbols updated so that the codebase is internally consistent.

**Token Estimate:** ~30k tokens
**Predecessors:** none
**Successors:** TASK-036-002
**Parallel:** no

**Acceptance Criteria:**
- [x] `src/cmd/work.go` is renamed to `src/cmd/run.go` (via `git mv`)
- [x] `var workCmd` → `var runCmd` (and all `workCmd.Flags()` → `runCmd.Flags()`)
- [x] `rootCmd.AddCommand(workCmd)` → `rootCmd.AddCommand(runCmd)`
- [x] `Use: "work [count]"` → `Use: "run [count]"`
- [x] Long help examples updated (`maggus work` → `maggus run`)
- [x] `func resetWorkFlags()` → `func resetRunFlags()`
- [x] `go build ./...` passes in `src/`

### TASK-036-002: Update all call sites that reference the "work" command string
**Description:** As a developer, I want all internal invocations updated to `"run"` so that the daemon and status screen correctly invoke `maggus run ...`.

**Token Estimate:** ~20k tokens
**Predecessors:** TASK-036-001
**Successors:** TASK-036-003
**Parallel:** no

**Acceptance Criteria:**
- [x] `src/cmd/daemon_start.go`: both `daemonArgs` string slices changed from `"work"` to `"run"` (~lines 103 and 222)
- [x] `src/cmd/status_update.go`: `exec.Command(execPath, "work", "--task", ...)` → `"run"`; nearby comment updated
- [x] `src/cmd/dispatch.go`: `rootCmd.Find([]string{"work", "--task", ...})` → `"run"`; `resetWorkFlags()` call → `resetRunFlags()`; comment updated
- [x] `src/cmd/init.go`: the user-facing line `"  maggus work ..."` in the init success message is removed (the command is hidden)
- [x] `go build ./...` passes in `src/`

### TASK-036-003: Rename remaining work_*.go source files and update tests
**Description:** As a developer, I want all `work_*.go` files renamed to `run_*.go` and all test references updated so that the file layout matches the command name.

**Token Estimate:** ~25k tokens
**Predecessors:** TASK-036-002
**Successors:** TASK-036-004
**Parallel:** no

**Acceptance Criteria:**
- [x] `git mv` renames applied for all nine files:
  - `work_loop.go` → `run_loop.go`
  - `work_setup.go` → `run_setup.go`
  - `work_task.go` → `run_task.go`
  - `work_messages.go` → `run_messages.go`
  - `work_test.go` → `run_test.go`
  - `work_loop_test.go` → `run_loop_test.go`
  - `work_setup_test.go` → `run_setup_test.go`
  - `work_task_test.go` → `run_task_test.go`
- [x] `src/cmd/dispatch_test.go`: all 6 occurrences of `workCmd` → `runCmd`
- [x] `src/cmd/root_test.go`: test case string `[]string{"maggus", "work"}` → `[]string{"maggus", "run"}`
- [x] Internal symbol renames across renamed files:
  - `workConfig` → `runLoopConfig` (renamed to `runLoopConfig` to avoid collision with existing `runConfig()` func in config.go)
  - `workSetup` → `runSetup`
  - `workLoopParams` → `runLoopParams`
- [x] `go build ./...` passes in `src/`
- [x] `go test ./...` passes in `src/`

### TASK-036-004: Update ARCHITECTURE.md
**Description:** As a developer, I want ARCHITECTURE.md updated to reflect the renamed component so that the architecture documentation stays accurate.

**Token Estimate:** ~10k tokens
**Predecessors:** TASK-036-003
**Successors:** none
**Parallel:** no
**Model:** haiku

**Acceptance Criteria:**
- [x] `ARCHITECTURE.md` Work Loop row in the components table updated from `cmd/work.go` to `cmd/run.go`
- [x] Any other references to `work.go` in ARCHITECTURE.md updated

## Task Dependency Graph

```
TASK-036-001 → TASK-036-002 → TASK-036-003 → TASK-036-004
```

| Task | Estimate | Predecessors | Parallel | Model |
|------|----------|--------------|----------|-------|
| TASK-036-001 | ~30k | none | no | — |
| TASK-036-002 | ~20k | 001 | no | — |
| TASK-036-003 | ~25k | 002 | no | — |
| TASK-036-004 | ~10k | 003 | no | haiku |

**Total estimated tokens:** ~85k

## Functional Requirements

- FR-1: After the rename, `maggus start` must still successfully launch the daemon (it constructs `["run", "--daemon-run", ...]` internally)
- FR-2: The status screen "Run Task" action must still work (it invokes `maggus run --task <id>`)
- FR-3: The `run` command must remain `Hidden: true` — it must not appear in `maggus --help`
- FR-4: No user-visible behavior changes

## Non-Goals

- Renaming the `--daemon-run` or `--daemon-run-id` flags (they are already hidden)
- Any changes to the work loop logic or daemon behavior
- Renaming `runWorkGoroutine` (already well-named, unrelated to the command name)

## Technical Considerations

- Use `git mv` for all file renames to preserve git history
- Apply renames in order (TASK-036-001 through 004) to avoid broken intermediate build states
- The `dispatch_test.go` references `workCmd` directly as a package-level variable — must be updated in TASK-036-003

## Success Metrics

- `go build ./...` passes
- `go test ./...` passes
- `maggus start` launches the daemon successfully
- `maggus run --help` shows the correct hidden help text

## Open Questions

(none)
