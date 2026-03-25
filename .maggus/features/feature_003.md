<!-- maggus-id: d2062a56-007c-47cd-8e7b-ba3d2e361689 -->
# Feature 003: UUID-Based Approval Keys with Stale Entry Pruning

## Introduction

The current approval system uses the feature/bug **filename** (e.g. `feature_003`) as the key in `feature_approvals.yml`. This breaks when a completed file is deleted and a new file is created with the same number — the new file inherits the old approval state. It also causes `feature_approvals.yml` to grow indefinitely because entries for deleted files are never removed.

Feature and bug files now include a stable UUID on their first line as an HTML comment:
```
<!-- maggus-id: d2062a56-007c-47cd-8e7b-ba3d2e361689 -->
```

This feature migrates the approval system to use that UUID as the key, and adds automatic pruning of stale entries (UUIDs no longer present in any file on disk) on every save.

### Architecture Context

- **Components involved:** `internal/parser` (new `ParseMaggusID`), `internal/approval` (new `Prune`), `cmd/status_plans.go` (`featureInfo`, `parseFeatures`, `parseBugs`), `cmd/work_loop.go` (`featureGroup`, `buildApprovedFeatureGroups`), `cmd/approve.go` (CLI approve/unapprove), `cmd/status.go` (`handleApproveToggle`)
- **No new packages** — all changes are within existing packages
- **Backwards compatibility:** Files without a `maggus-id` comment fall back to the filename-based key, so old files continue to work

## Goals

- Approval state survives feature file deletion and renaming (completed → archived)
- `feature_approvals.yml` does not grow indefinitely — stale entries are pruned automatically on every write
- No user-visible behaviour change: approve/unapprove commands and the TUI toggle continue to work exactly as before
- Backwards-compatible with files that lack a `maggus-id` comment (fallback to filename key)

## Tasks

### TASK-003-001: Add `ParseMaggusID` to the parser package

**Description:** As a developer, I want a parser function that extracts the `maggus-id` UUID from the first line of a feature or bug file, so that the rest of the system can use it as a stable approval key.

**Token Estimate:** ~15k tokens
**Predecessors:** none
**Successors:** TASK-003-003, TASK-003-004, TASK-003-005
**Parallel:** yes — can run alongside TASK-003-002
**Model:** haiku

**Acceptance Criteria:**
- [x] `parser.ParseMaggusID(path string) string` is added to `src/internal/parser/parser.go`
- [x] It opens the file, reads only the first line, and matches the pattern `<!-- maggus-id: <uuid> -->`
- [x] Returns the UUID string (e.g. `d2062a56-007c-47cd-8e7b-ba3d2e361689`) on match
- [x] Returns empty string `""` if the first line does not match (file not found, no comment, wrong format)
- [x] Does not read beyond the first line (efficient for large files)
- [x] Unit tests in `src/internal/parser/` cover: valid UUID line, missing comment, wrong format, empty file, file not found
- [x] `go vet ./...` and `go test ./...` pass

### TASK-003-002: Add `Prune` function to the approval package

**Description:** As a developer, I want an approval package function that removes stale entries (UUIDs not present in any current file) from `feature_approvals.yml`, so that the file does not grow indefinitely.

**Token Estimate:** ~20k tokens
**Predecessors:** none
**Successors:** TASK-003-003, TASK-003-004, TASK-003-005
**Parallel:** yes — can run alongside TASK-003-001
**Model:** haiku

**Acceptance Criteria:**
- [x] `approval.Prune(dir string, knownIDs []string) error` is added to `src/internal/approval/approval.go`
- [x] It loads the current approvals, removes all entries whose key is not in `knownIDs`, and saves the result
- [x] If `knownIDs` is empty, the function is a no-op (does not wipe the file)
- [x] If no entries are removed (file is already clean), the file is not rewritten unnecessarily
- [x] Unit tests cover: pruning stale entries, no-op when all entries are known, empty knownIDs guard
- [x] `go vet ./...` and `go test ./...` pass

### TASK-003-003: Update `featureInfo`, `parseFeatures`, `parseBugs`, and `handleApproveToggle`

**Description:** As a developer, I want `featureInfo` to carry the `maggus-id` UUID and the parse functions to use it as the approval key and prune stale entries, so that the status view works correctly with UUID-based approvals.

**Token Estimate:** ~40k tokens
**Predecessors:** TASK-003-001, TASK-003-002
**Successors:** none
**Parallel:** yes — can run alongside TASK-003-004 and TASK-003-005

**Acceptance Criteria:**
- [x] `featureInfo` in `src/cmd/status_plans.go` gains a `maggusID string` field
- [x] `parseFeatures` calls `parser.ParseMaggusID(f)` for each file and stores the result in `featureInfo.maggusID`
- [x] `parseBugs` does the same for bug files
- [x] Both functions use `maggusID` as the approval key when calling `approval.IsApproved`; if `maggusID` is empty, they fall back to `featureIDFromPath(f)` (backwards compatibility)
- [x] After building both feature and bug lists, a combined list of all known IDs (UUIDs or filename fallbacks) is passed to `approval.Prune` to remove stale entries
- [x] `handleApproveToggle` in `src/cmd/status.go` uses `f.maggusID` (with filename fallback) instead of `featureIDFromPath(f.filename)` when calling `approval.Approve` / `approval.Unapprove`
- [x] `featureIDFromPath` is kept (still used in `approve.go` as a fallback label) but no longer used as a primary approval key in `status_plans.go`
- [x] Existing status TUI tests pass; add or update tests for the new pruning behaviour
- [x] `go vet ./...` and `go test ./...` pass

### TASK-003-004: Update `featureGroup` and `buildApprovedFeatureGroups` in the work loop

**Description:** As a developer, I want the work loop's feature group builder to use the `maggus-id` UUID for approval checks and pruning, so that daemon runs respect the UUID-based approval state.

**Token Estimate:** ~30k tokens
**Predecessors:** TASK-003-001, TASK-003-002
**Successors:** none
**Parallel:** yes — can run alongside TASK-003-003 and TASK-003-005

**Acceptance Criteria:**
- [ ] `featureGroup` in `src/cmd/work_loop.go` gains a `maggusID string` field
- [ ] `buildApprovedFeatureGroups` calls `parser.ParseMaggusID(f)` for each bug and feature file
- [ ] Approval check uses `maggusID` as the key; falls back to the filename-based ID if empty
- [ ] After building all groups, calls `approval.Prune` with the set of all known IDs (UUIDs + fallbacks) to remove stale entries
- [ ] The `featureGroup.id` field is unchanged — it remains the filename-based label used for display and logging
- [ ] Existing work loop tests pass; no regressions in task dispatch or feature filtering
- [ ] `go vet ./...` and `go test ./...` pass

### TASK-003-005: Update `approve.go` CLI commands to use UUID

**Description:** As a developer, I want the `maggus approve` and `maggus unapprove` CLI commands to use the `maggus-id` UUID as the approval key, so that they are consistent with the rest of the system.

**Token Estimate:** ~35k tokens
**Predecessors:** TASK-003-001, TASK-003-002
**Successors:** none
**Parallel:** yes — can run alongside TASK-003-003 and TASK-003-004

**Acceptance Criteria:**
- [ ] A new helper `maggusIDFromFile(path string) string` (or inline usage of `parser.ParseMaggusID`) is used to resolve the approval key for each feature/bug file in `src/cmd/approve.go`
- [ ] `listActiveFeatureIDs` is updated (or replaced by a new helper `listActiveFeatures`) to return a struct or pair of `(displayName string, approvalKey string)` — `displayName` is the filename (for the interactive picker), `approvalKey` is the UUID (or filename fallback)
- [ ] `runApprove` and `runUnapprove` use the UUID-based approval key when calling `approval.Approve` / `approval.Unapprove`
- [ ] The interactive pickers (`runApproveInteractive`, `runUnapproveInteractive`) still display human-readable filenames but store/pass UUIDs internally
- [ ] `maggus approve feature_003` by filename still works: the command resolves the filename to its UUID before approving (find the file, read its UUID, use that as the key)
- [ ] `maggus unapprove` similarly resolves filename → UUID
- [ ] Existing approve/unapprove tests are updated and pass
- [ ] `go vet ./...` and `go test ./...` pass

## Task Dependency Graph

```
TASK-003-001 ──→ TASK-003-003
                 TASK-003-004
                 TASK-003-005
TASK-003-002 ──┘
```

| Task | Estimate | Predecessors | Parallel | Model |
|------|----------|--------------|----------|-------|
| TASK-003-001 | ~15k | none | yes (with 002) | haiku |
| TASK-003-002 | ~20k | none | yes (with 001) | haiku |
| TASK-003-003 | ~40k | 001, 002 | yes (with 004, 005) | — |
| TASK-003-004 | ~30k | 001, 002 | yes (with 003, 005) | — |
| TASK-003-005 | ~35k | 001, 002 | yes (with 003, 004) | — |

**Total estimated tokens:** ~140k

## Functional Requirements

- FR-1: `parser.ParseMaggusID(path)` reads only the first line of a file and extracts the UUID from `<!-- maggus-id: <uuid> -->`; returns `""` if absent
- FR-2: All approval lookups (`IsApproved`) use the UUID from `ParseMaggusID` as the key; fall back to filename-based key if UUID is empty
- FR-3: After every parse cycle (features + bugs together), `approval.Prune` is called with all currently known IDs to remove stale entries
- FR-4: `approval.Prune` is a no-op when `knownIDs` is empty (safety guard against wiping the file)
- FR-5: `maggus approve <filename>` resolves the filename to its UUID before writing to `feature_approvals.yml`
- FR-6: The interactive approve/unapprove pickers display filenames but key on UUIDs internally
- FR-7: `feature_approvals.yml` is rewritten only when entries are actually removed (avoid unnecessary disk writes)

## Non-Goals

- No migration of existing `feature_approvals.yml` entries from filename-keys to UUID-keys — old entries simply become stale and are pruned on the next run
- No changes to the `feature_approvals.yml` file format (stays YAML `map[string]bool`)
- No UI changes to how approval state is displayed
- No changes to how `maggus-id` comments are written (that is handled by the plan/bug generators)

## Technical Considerations

- The fallback to filename-based key (when no `maggus-id` is present) ensures backwards compatibility with any feature files that predate the UUID convention
- Pruning is intentionally done after parsing both features AND bugs together — pruning after only one set could incorrectly remove entries for the other set
- `approval.Prune` must NOT be called with an empty slice — this would wipe all approvals if called before any files are parsed (guard with `len(knownIDs) == 0` early return)
- The `featureGroup.id` field in `work_loop.go` retains the filename-based value for logging/display; the new `maggusID` field is only used for approval key lookup

## Success Metrics

- Approve a feature, complete it (file renamed to `_completed.md`), create a new feature with the same number → new feature has no inherited approval state
- After several complete cycles, `feature_approvals.yml` contains only entries for currently active files
- `go test ./...` passes with no regressions

## Open Questions

_(none)_
