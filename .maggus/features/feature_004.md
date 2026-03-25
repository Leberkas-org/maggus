<!-- maggus-id: bb306ba7-1d51-4651-ac47-df7f9123d692 -->
# Feature 004: Lifecycle Hooks

## Introduction

Maggus currently has no way to notify external systems or run custom scripts when features, bugs, or tasks complete. Users want to integrate maggus with tools like Slack, ticket trackers, or custom workflows — but the only completion behavior today is renaming/deleting the file.

This feature adds a hooks system to `.maggus/config.yml` that lets users define shell commands triggered by lifecycle events. Each hook receives a JSON payload on stdin containing event metadata. Hooks are fire-and-forget: failures are logged as warnings but never block the work loop.

### Architecture Context

- **Components involved:** `internal/config` (new `HooksConfig` struct), new `internal/hooks` package (executor), `cmd/work_task.go` (wiring)
- **One new package:** `internal/hooks` — keeps hook execution logic isolated from the work loop
- **No changes to existing behavior:** Hooks are purely additive; when unconfigured, nothing changes

## Goals

- Allow users to run arbitrary shell commands when a feature, bug, or task completes
- Provide rich JSON context on stdin so scripts can act on event metadata without parsing files
- Keep the work loop reliable — hook failures must never block or crash the loop
- Support multiple hooks per event type for composability

## Tasks

### TASK-004-001: Add HooksConfig to the config package
**Description:** As a developer, I want the config parser to recognize a `hooks` section in `.maggus/config.yml`, so that users can declare lifecycle hooks.

**Token Estimate:** ~20k tokens
**Predecessors:** none
**Successors:** TASK-004-003, TASK-004-004
**Parallel:** yes — can run alongside TASK-004-002

**Acceptance Criteria:**
- [x] `HooksConfig` struct is added to `src/internal/config/config.go` with fields `OnFeatureComplete`, `OnBugComplete`, and `OnTaskComplete`, each a `[]HookEntry`
- [x] `HookEntry` struct has a `Run string` field (`yaml:"run"`)
- [x] `Config` struct gains a `Hooks HooksConfig` field (`yaml:"hooks"`)
- [x] Parsing the following YAML populates the config correctly:
  ```yaml
  hooks:
    on_feature_complete:
      - run: "./scripts/notify.sh"
      - run: "powershell -File ./scripts/track.ps1"
    on_bug_complete:
      - run: "./scripts/close-ticket.sh"
    on_task_complete:
      - run: "./scripts/log-task.sh"
  ```
- [x] Empty or missing `hooks` section results in empty slices (no errors)
- [x] Unit tests cover: full config, partial config (only one event type), empty hooks, missing hooks section
- [x] `go vet ./...` and `go test ./...` pass

### TASK-004-002: Create internal/hooks package with executor
**Description:** As a developer, I want a hooks executor that runs shell commands with a JSON payload on stdin, so that lifecycle events can trigger user-defined scripts.

**Token Estimate:** ~40k tokens
**Predecessors:** none
**Successors:** TASK-004-003, TASK-004-004
**Parallel:** yes — can run alongside TASK-004-001

**Acceptance Criteria:**
- [x] New package `src/internal/hooks/` is created
- [x] `hooks.Event` struct defines the JSON payload shape:
  ```go
  type Event struct {
      Type           string     `json:"event"`           // "feature_complete", "bug_complete", "task_complete"
      File           string     `json:"file"`            // basename of the source file
      MaggusID       string     `json:"maggus_id"`       // UUID from <!-- maggus-id: ... --> or empty
      Title          string     `json:"title"`            // feature/bug title or task title
      Action         string     `json:"action"`           // "rename" or "delete" (empty for task_complete)
      Tasks          []TaskInfo `json:"tasks"`            // list of tasks in the feature/bug
      Timestamp      string     `json:"timestamp"`        // RFC3339
  }
  type TaskInfo struct {
      ID    string `json:"id"`
      Title string `json:"title"`
  }
  ```
- [x] `hooks.Run(commands []HookEntry, event Event, workDir string, logger *log.Logger)` executes each command sequentially
- [x] For each command: the JSON-encoded event is written to the command's stdin
- [x] Each command is executed with a 30-second timeout using `context.WithTimeout`
- [x] The command's working directory is set to `workDir`
- [x] If a command fails (non-zero exit, timeout, spawn error), the error is logged as a warning and execution continues to the next hook
- [x] If a command produces stderr output, it is included in the warning log
- [x] `hooks.Run` with an empty `commands` slice is a no-op (returns immediately)
- [x] Unit tests cover: successful execution, command failure (logged, not fatal), timeout, empty commands, JSON payload correctness
- [x] `go vet ./...` and `go test ./...` pass

### TASK-004-003: Wire feature and bug completion hooks into work_task.go
**Description:** As a developer, I want the work loop to fire `on_feature_complete` and `on_bug_complete` hooks after marking completed files, so that users' scripts run at the right time.

**Token Estimate:** ~35k tokens
**Predecessors:** TASK-004-001, TASK-004-002
**Successors:** TASK-004-005
**Parallel:** yes — can run alongside TASK-004-004

**Acceptance Criteria:**
- [x] `taskContext` gains a `hooks config.HooksConfig` field, populated from the loaded config in the work loop setup
- [x] After `parser.MarkCompletedFeatures` returns count > 0 in `runTask`, each completed feature file triggers `hooks.Run` with `config.Hooks.OnFeatureComplete` and an `Event` of type `"feature_complete"`
- [x] The event payload includes: file basename, maggus-id (via `parser.ParseMaggusID`), feature title (via `parser.ParseFileTitle`), the action taken ("rename" or "delete"), and the list of task IDs/titles from the feature
- [x] Same logic for `parser.MarkCompletedBugs` with `config.Hooks.OnBugComplete` and event type `"bug_complete"`
- [x] `MarkCompletedFeatures` and `MarkCompletedBugs` are updated (or wrapped) to return the list of completed file paths (not just a count), so that hook payloads can be built per file
- [x] Hooks run after the file action (rename/delete) but before `git add` staging
- [x] When no hooks are configured, there is zero overhead (no allocations, no function calls beyond the empty-slice check)
- [x] `go vet ./...` and `go test ./...` pass

### TASK-004-004: Wire task completion hooks into work_task.go
**Description:** As a developer, I want the work loop to fire `on_task_complete` hooks after a task is successfully committed, so that users can track individual task completions.

**Token Estimate:** ~25k tokens
**Predecessors:** TASK-004-001, TASK-004-002
**Successors:** TASK-004-005
**Parallel:** yes — can run alongside TASK-004-003

**Acceptance Criteria:**
- [x] After a successful commit in `completeTask`, `hooks.Run` is called with `config.Hooks.OnTaskComplete` and an `Event` of type `"task_complete"`
- [x] The event payload includes: source file basename, maggus-id of the source file, task ID, task title, and timestamp
- [x] The `tasks` field in the event contains a single entry (the completed task)
- [x] The `action` field is empty for task completion events
- [x] Hooks run after the commit succeeds but before the between-task sync check
- [x] When no `on_task_complete` hooks are configured, there is zero overhead
- [x] `go vet ./...` and `go test ./...` pass

### TASK-004-005: Integration test for hooks end-to-end
**Description:** As a developer, I want an integration test that verifies hooks fire with correct JSON payloads during the completion flow, so that we have confidence the wiring is correct.

**Token Estimate:** ~30k tokens
**Predecessors:** TASK-004-003, TASK-004-004
**Successors:** none
**Parallel:** no

**Acceptance Criteria:**
- [ ] Test creates a temporary directory with a `.maggus/config.yml` that configures hooks pointing to a test script
- [ ] The test script writes the received stdin JSON to a known output file
- [ ] Test triggers feature/bug/task completion through `MarkCompletedFeatures`/`MarkCompletedBugs` (or equivalent test helpers)
- [ ] Asserts the output file contains valid JSON with the expected `event`, `file`, `title`, `tasks`, and `timestamp` fields
- [ ] Asserts that a failing hook (script that exits 1) does not prevent subsequent hooks from running
- [ ] Test is skipped on platforms where shell execution is unavailable
- [ ] `go vet ./...` and `go test ./...` pass

## Task Dependency Graph

```
TASK-004-001 ──→ TASK-004-003 ──→ TASK-004-005
                 TASK-004-004 ──┘
TASK-004-002 ──┘
```

| Task | Estimate | Predecessors | Parallel | Model |
|------|----------|--------------|----------|-------|
| TASK-004-001 | ~20k | none | yes (with 002) | haiku |
| TASK-004-002 | ~40k | none | yes (with 001) | — |
| TASK-004-003 | ~35k | 001, 002 | yes (with 004) | — |
| TASK-004-004 | ~25k | 001, 002 | yes (with 003) | — |
| TASK-004-005 | ~30k | 003, 004 | no | — |

**Total estimated tokens:** ~150k

## Functional Requirements

- FR-1: Users can define hooks in `.maggus/config.yml` under `hooks.on_feature_complete`, `hooks.on_bug_complete`, and `hooks.on_task_complete`
- FR-2: Each hook entry has a `run` field specifying a shell command to execute
- FR-3: Multiple hooks per event type are supported and executed sequentially in declaration order
- FR-4: Each hook receives a JSON object on stdin with fields: `event`, `file`, `maggus_id`, `title`, `action`, `tasks` (array of `{id, title}`), and `timestamp` (RFC3339)
- FR-5: For `feature_complete` and `bug_complete` events, `tasks` contains all tasks from the completed file; for `task_complete`, it contains only the single completed task
- FR-6: For `feature_complete` and `bug_complete` events, `action` is `"rename"` or `"delete"`; for `task_complete`, `action` is empty
- FR-7: Hook commands run with the project root as their working directory
- FR-8: Each hook command has a 30-second execution timeout
- FR-9: Hook failures (non-zero exit, timeout, spawn error) are logged as warnings and do not block or crash the work loop
- FR-10: When no hooks are configured, the completion flow has zero additional overhead

## Non-Goals

- No built-in integrations (Slack, Discord webhook, etc.) — users write their own scripts
- No hook for task/feature start events — only completion events in this iteration
- No configurable timeout per hook — fixed 30 seconds for now
- No hook execution ordering guarantees across event types (only within a single event's hook list)
- No retry logic for failed hooks

## Technical Considerations

- `MarkCompletedFeatures` and `MarkCompletedBugs` currently return only a count. They need to be updated (or wrapped) to also return the list of completed file paths so that per-file hook payloads can be constructed. This is a small signature change — callers that don't need the paths can ignore the new return value.
- The `parser.ParseMaggusID` function (from feature 003) provides the UUID for the payload. If feature 003 is not yet implemented, the `maggus_id` field will be empty — the hooks system should handle this gracefully.
- Shell execution differs across platforms: on Unix, commands run via `sh -c`; on Windows, via `cmd /C` or the user's shell. Use `exec.Command` with appropriate shell wrapping, consistent with how `runner` handles subprocesses.
- The 30-second timeout prevents runaway hooks from blocking the work loop indefinitely. `context.WithTimeout` + `cmd.Cancel` handles this cleanly.

## Success Metrics

- A user can configure a hook that writes to a file on feature completion, and the file contains valid JSON with the expected metadata
- A hook that exits with an error code does not prevent the next task from running
- A hook that hangs is killed after 30 seconds with a warning logged
- `go test ./...` passes with no regressions

## Open Questions

_(none)_
