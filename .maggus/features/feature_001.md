<!-- maggus-id: 27748140-8b44-401d-9090-292fefb1f15f -->
# Feature 001: Global Usage Tracking with Item Context

## Introduction

Rework the usage tracking system to support posting records to a global API for cross-project tracking. Currently, usage records are stored per-project in `.maggus/` with minimal task context (`TaskID`, `FeatureFile`). This feature enriches records with item-level metadata (feature/bug ID, title, short name), repository identification, and a session kind field, while consolidating all usage into two global files under `~/.maggus/usage/`.

## Goals

- Enrich usage records with item-level context (ItemID, ItemShort, ItemTitle, TaskShort) for API consumption
- Add repository URL to every record for cross-project identification
- Consolidate usage storage from many per-skill files into two global files (work + sessions)
- Auto-generate MaggusID UUIDs for feature/bug files that lack one
- Migrate existing per-project usage data to the new global location
- Add a `kind` field to session records indicating the type of interaction (plan, bugreport, prompt, etc.)

## Tasks

### TASK-001-001: Add EnsureMaggusID to parser and RepoURL to gitutil
**Description:** As a developer, I want utility functions to auto-generate MaggusIDs and retrieve the git remote URL so that every usage record has a stable item ID and repository reference.

**Token Estimate:** ~30k tokens
**Predecessors:** none
**Successors:** TASK-001-003, TASK-001-004
**Parallel:** yes -- can run alongside TASK-001-002

**Acceptance Criteria:**
- [x] `parser.EnsureMaggusID(path string) (string, error)` added to `src/internal/parser/parser.go`
- [x] If file already has a `<!-- maggus-id: ... -->` line, returns existing UUID without modifying the file
- [x] If file lacks a maggus-id, generates a UUID v4 (using `crypto/rand`, same pattern as `internal/fingerprint/fingerprint.go`), prepends `<!-- maggus-id: <uuid> -->` as the first line, and returns the new UUID
- [x] `gitutil.RepoURL(dir string) string` added to `src/internal/gitutil/`
- [x] RepoURL runs `git config --get remote.origin.url` and returns trimmed output, or empty string on error
- [x] Unit tests for both functions pass
- [x] `go vet ./...` passes

### TASK-001-002: Update Record struct and global file routing in usage package
**Description:** As a developer, I want the usage Record to include Repository, Kind, ItemID, ItemShort, ItemTitle, and TaskShort fields, and write to `~/.maggus/usage/` instead of per-project `.maggus/`.

**Token Estimate:** ~35k tokens
**Predecessors:** none
**Successors:** TASK-001-003, TASK-001-004, TASK-001-005
**Parallel:** yes -- can run alongside TASK-001-001

**Acceptance Criteria:**
- [ ] `usage.Record` struct has fields: `Repository`, `ItemID`, `ItemShort`, `ItemTitle`, `TaskShort`, `Kind` (with `json:"kind,omitempty"`)
- [ ] Old fields `TaskID` and `FeatureFile` are removed from the struct
- [ ] `Append(records []Record) error` no longer takes a `dir` parameter -- writes to `~/.maggus/usage/`
- [ ] Work records (Kind empty) go to `~/.maggus/usage/work.jsonl`
- [ ] Session records (Kind set) go to `~/.maggus/usage/sessions.jsonl`
- [ ] `~/.maggus/usage/` directory is auto-created if missing
- [ ] `AppendTo` function remains unchanged for backward compatibility and test use
- [ ] Unit tests updated and passing
- [ ] `go vet ./...` passes

### TASK-001-003: Update runner pipeline (TaskUsage, IterationStartMsg, TUIModel)
**Description:** As a developer, I want the runner pipeline to carry item-level metadata through TaskUsage, IterationStartMsg, and TUIModel so that usage callbacks can populate the enriched Record fields.

**Token Estimate:** ~75k tokens
**Predecessors:** TASK-001-001, TASK-001-002
**Successors:** TASK-001-004
**Parallel:** no
**Model:** opus

**Acceptance Criteria:**
- [ ] `runner.TaskUsage` struct replaces `TaskID`/`FeatureFile` with `ItemID`, `ItemShort`, `ItemTitle`, `TaskShort`
- [ ] `tokenState.saveAndReset` signature updated to accept new item-level fields
- [ ] `runner.IterationStartMsg` replaces `FeatureFile` with `ItemID`, `ItemShort`, `ItemTitle`
- [ ] `TUIModel` fields `taskFeatureFile` replaced with `itemID`, `itemShort`, `itemTitle`
- [ ] `handleIterationStart` maps new IterationStartMsg fields to TUIModel and passes them to saveAndReset
- [ ] TUI render code updated: `m.taskFeatureFile` references changed to `m.itemTitle` or `m.itemShort`
- [ ] `nullTUIModel` (daemon_tui.go) updated with matching field changes
- [ ] All runner tests updated and passing (`tui_tokens_test.go`, `tui_render_test.go`, `tui_active_elapsed_test.go`)
- [ ] `go test ./internal/runner/...` passes
- [ ] `go vet ./...` passes

### TASK-001-004: Update work loop and daemon callers
**Description:** As a developer, I want the work loop, daemon, and skill/prompt callers to populate the new usage fields so that every record written contains full item context, repository, and kind.

**Token Estimate:** ~80k tokens
**Predecessors:** TASK-001-003
**Successors:** TASK-001-005
**Parallel:** no
**Model:** opus

**Acceptance Criteria:**
- [ ] `taskContext` struct gains `currentPlan *parser.Plan` field (`src/cmd/work_task.go`)
- [ ] `runGroupTasks` calls `parser.EnsureMaggusID(group.File)` and sets `tc.currentPlan = &group` (`src/cmd/work_loop.go`)
- [ ] `sendIterationStart` accepts Plan info and populates `ItemID` (MaggusID), `ItemShort` (Plan.ID), `ItemTitle` (via `parser.ParseFileTitle`) on `IterationStartMsg` (`src/cmd/work_task.go`)
- [ ] `setupUsageCallback` gets `repoURL := gitutil.RepoURL(dir)` once, maps new TaskUsage fields to usage.Record, calls `usage.Append(records)` without dir (`src/cmd/work_loop.go`)
- [ ] Daemon callback in `daemon_keepalive.go` updated identically
- [ ] `skillMapping.usageFile` replaced with `skillMapping.kind` in `src/cmd/prompt.go`
- [ ] Kind values: `prompt`, `plan`, `vision`, `architecture`, `bugreport`, `bryan_plan`, `bryan_bugreport`
- [ ] `extractSkillUsage` in `src/cmd/plan.go` updated: takes `kind` instead of `usageFile`, sets `Repository` and `Kind`, calls `usage.Append()`
- [ ] All cmd tests updated and passing (`work_loop_test.go`, `daemon_tui_test.go`, `plan_test.go`, `prompt_test.go`)
- [ ] `go test ./cmd/...` passes
- [ ] `go vet ./...` passes

### TASK-001-005: Migrate existing usage data to global directory
**Description:** As a user, I want my existing per-project usage data migrated to the new global `~/.maggus/usage/` directory so that historical records are available alongside new ones.

**Token Estimate:** ~50k tokens
**Predecessors:** TASK-001-002, TASK-001-004
**Successors:** TASK-001-006
**Parallel:** no

**Acceptance Criteria:**
- [ ] New function `usage.MigrateProject(projectDir string) error` added
- [ ] Reads `.maggus/usage_work.jsonl` and appends records to `~/.maggus/usage/work.jsonl` (mapping old `TaskID` to `TaskShort`, dropping `FeatureFile`, setting `Repository` from git remote if available)
- [ ] Reads `.maggus/usage_plan.jsonl`, `usage_prompt.jsonl`, `usage_bugreport.jsonl`, `usage_vision.jsonl`, `usage_architecture.jsonl`, `usage_bryan_plan.jsonl`, `usage_bryan_bugreport.jsonl` and appends to `~/.maggus/usage/sessions.jsonl` with appropriate `Kind` derived from filename
- [ ] Skips files that don't exist (no error)
- [ ] After successful migration, renames old files with `.migrated` suffix (not deletes, for safety)
- [ ] Migration is idempotent -- skips already-migrated files (checks for `.migrated` suffix)
- [ ] Migration is called once at startup in the work command (before main loop) and prompt command
- [ ] Also handles legacy CSV files (`usage.csv`, `usage_v3.csv`) -- skip them (just rename to `.migrated`), they use an incompatible format
- [ ] Unit tests cover: migration of each file type, skipping missing files, idempotency
- [ ] `go test ./...` passes

### TASK-001-006: Final verification and cleanup
**Description:** As a developer, I want to verify the entire usage tracking pipeline works end-to-end and clean up any remaining references to the old system.

**Token Estimate:** ~25k tokens
**Predecessors:** TASK-001-005
**Successors:** none
**Parallel:** no

**Acceptance Criteria:**
- [ ] `go build ./...` succeeds
- [ ] `go test ./...` passes (all packages)
- [ ] `go vet ./...` passes
- [ ] No remaining references to old `TaskID` or `FeatureFile` fields in non-test code (grep verification)
- [ ] No remaining references to old per-skill usage filenames like `usage_plan.jsonl` in non-migration code
- [ ] runlog `StateSnapshot` updated if it references `FeatureFile` (use `ItemTitle` instead)
- [ ] Manual integration test: run `maggus work` on a test project and verify `~/.maggus/usage/work.jsonl` contains correct fields

## Task Dependency Graph

```
TASK-001-001 ──┐
               ├──→ TASK-001-003 ──→ TASK-001-004 ──→ TASK-001-005 ──→ TASK-001-006
TASK-001-002 ──┘
```

| Task | Estimate | Predecessors | Parallel | Model |
|------|----------|--------------|----------|-------|
| TASK-001-001 | ~30k | none | yes (with 002) | -- |
| TASK-001-002 | ~35k | none | yes (with 001) | -- |
| TASK-001-003 | ~75k | 001, 002 | no | opus |
| TASK-001-004 | ~80k | 003 | no | opus |
| TASK-001-005 | ~50k | 002, 004 | no | -- |
| TASK-001-006 | ~25k | 005 | no | -- |

**Total estimated tokens:** ~295k

## Functional Requirements

- FR-1: Every usage record must include a `repository` field containing the git remote origin URL
- FR-2: Every work record must include `item_id` (UUID), `item_short` (e.g. "feature_001"), `item_title` (parsed H1 title), and `task_short` (e.g. "TASK-001-003")
- FR-3: Feature/bug files without a `<!-- maggus-id: ... -->` comment must have one auto-generated and written back to the file
- FR-4: Work records (from `maggus work` / daemon) must be written to `~/.maggus/usage/work.jsonl`
- FR-5: Session records (from `maggus prompt` / skills) must be written to `~/.maggus/usage/sessions.jsonl` with a `kind` field
- FR-6: Kind values must match skill names: `prompt`, `plan`, `vision`, `architecture`, `bugreport`, `bryan_plan`, `bryan_bugreport`
- FR-7: Existing per-project usage files must be migrated to the global directory on first run
- FR-8: The `Append` function must auto-create the `~/.maggus/usage/` directory if it doesn't exist

## Non-Goals

- No API endpoint implementation -- this feature prepares records for API posting; the actual API integration is a separate feature
- No migration of legacy CSV files (`usage.csv`, `usage_v3.csv`) -- these use an incompatible format and will just be archived
- No UI changes to display item-level info in the usage/cost summary (could be a follow-up)
- No backward-compatible reading of old-format records

## Technical Considerations

- `crypto/rand` UUID generation pattern already exists in `internal/fingerprint/fingerprint.go` -- reuse the same approach
- `parser.ParseFileTitle(path)` already exists for extracting H1 titles from feature/bug files
- `parser.ParseMaggusID(path)` already exists for reading existing UUIDs
- `globalconfig.Dir()` returns `~/.maggus/` -- reuse for the global usage directory
- `usage.AppendTo` remains as-is for tests and low-level use; only the public `Append` changes signature
- The `taskContext` struct in `work_task.go` is the natural place to thread Plan context

## Success Metrics

- All usage records in `~/.maggus/usage/work.jsonl` contain non-empty `item_id`, `repository`, `task_short`
- All session records in `~/.maggus/usage/sessions.jsonl` contain a valid `kind`
- Zero per-project usage files created after migration
- Feature/bug files that lacked MaggusIDs now have them persisted

## Open Questions

(None -- all resolved via clarifying questions)
