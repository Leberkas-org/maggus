# Feature 002: Approve/Unapprove Features in Status TUI

## Introduction

Consolidate feature gating into a single approval system by surfacing approve/unapprove controls directly in the status TUI. Currently there are two overlapping mechanisms for excluding features from the work loop: file-based `_ignored.md` renaming (`maggus ignore`) and the logical approval store (`feature_approvals.yml`). This feature collapses them into one: approval. The ignore system — CLI commands, file naming convention, and TUI controls — is removed entirely.

### Architecture Context

- **Vision alignment:** Simplifies the mental model for controlling which features maggus works on. One mechanism instead of two.
- **Components involved:**
  - `cmd/status.go` — status TUI model; `alt+p` handler replaced, approval state added
  - `cmd/status_plans.go` — `featureInfo` struct gains `approved bool`; `parseFeatures`/`parseBugs` gain approval loading + `_ignored.md` migration
  - `cmd/approve.go` — `featureIDFromPath` and `listActiveFeatureIDs` lose `_ignored` suffix handling
  - `cmd/ignore.go` / `cmd/unignore.go` — deleted entirely
  - `internal/parser` — `IsIgnoredFile`, `_ignored.md` globbing, `Task.Ignored`, and `IGNORED TASK-NNN:` heading parsing removed
- **New patterns:** none — reuses the existing `internal/approval` package throughout.

## Goals

- Show approval state (✓ approved / ✗ unapproved) visually on each feature tab in the status TUI.
- Allow toggling feature approval directly in the status TUI via `alt+p`.
- Automatically migrate any existing `_ignored.md` files to `approved=false` in `feature_approvals.yml` on load — no manual step required.
- Remove `maggus ignore` / `maggus unignore` CLI commands and all supporting infrastructure.

## Tasks

### TASK-002-001: Auto-migrate `_ignored.md` files to unapproved on load

**Description:** As a maggus user upgrading from the ignore system, I want existing `_ignored.md` files to be automatically converted to unapproved features so that I don't need to manually migrate state.

**Token Estimate:** ~30k tokens
**Predecessors:** none
**Successors:** TASK-002-005
**Parallel:** yes — can run alongside TASK-002-002

**Acceptance Criteria:**
- [x] `parseFeatures(dir)` detects any `_ignored.md` file, renames it to `<name>.md` (stripping `_ignored` suffix), and calls `approval.Unapprove(dir, featureID)` before building the `featureInfo` slice
- [x] Same migration logic applied in `parseBugs(dir)` for `_ignored.md` bug files
- [x] Migration is idempotent: if a renamed file already exists as `.md`, the rename is skipped gracefully
- [x] After migration the file is parsed and included in the returned slice with the renamed filename
- [x] Unit tests cover: no ignored files (no-op), one ignored file (renamed + unapproved), already-migrated file (idempotent)
- [x] `go test ./...` passes

---

### TASK-002-002: Add approval state to `featureInfo` and `statusModel`

**Description:** As a developer, I want `featureInfo` to carry approval state so that the status TUI can render and act on it without loading the approvals file separately each time.

**Token Estimate:** ~35k tokens
**Predecessors:** none
**Successors:** TASK-002-003, TASK-002-004
**Parallel:** yes — can run alongside TASK-002-001

**Acceptance Criteria:**
- [x] `featureInfo` gains an `approved bool` field
- [x] `parseFeatures(dir string)` accepts `dir` and loads `approval.Load(dir)` once per call; sets `fi.approved = approval.IsApproved(a, featureIDFromPath(f), false)` for each feature (always uses `approvalRequired=false` — visual display only, not gate logic)
- [x] `parseBugs(dir string)` does the same
- [x] All call sites of `parseFeatures` and `parseBugs` updated to pass `dir`
- [x] `statusModel` has a field `approvals approval.Approvals` (kept for reload; populated in `reloadFeatures`)
- [x] `reloadFeatures()` reloads approvals alongside features so the view stays fresh after a toggle
- [x] `go test ./...` passes

---

### TASK-002-003: Render approval state in tab bar and plain output

**Description:** As a status TUI user, I want to see at a glance which features are approved and which are not, directly on the tab label.

**Token Estimate:** ~25k tokens
**Predecessors:** TASK-002-002
**Successors:** TASK-002-005
**Parallel:** yes — can run alongside TASK-002-004 once TASK-002-002 is merged

**Acceptance Criteria:**
- [x] `renderTabBar()` shows `✓` prefix for approved features and `✗` prefix for unapproved features in each tab label (e.g. ` ✓ feature_001 2/4 ` vs ` ✗ feature_002 0/3 `)
- [x] Approved tabs use the existing primary/selected color (cyan for selected, muted for unselected)
- [x] Unapproved tabs use a dim warning color (e.g. `styles.Warning` faint) for both selected and unselected states — distinct from approved but not alarming
- [x] `renderStatusPlain()` replaces the old `[~] ignored` prefix with `[✗]` and `"unapproved"` suffix for unapproved features; approved features show no special prefix/suffix
- [x] Layout does not break at narrow terminal widths
- [x] `go test ./...` passes

---

### TASK-002-004: Replace `alt+p` ignore toggle with approve/unapprove toggle

**Description:** As a status TUI user, I want to approve or unapprove the selected feature with a single keypress so that I can control what maggus works on without leaving the TUI.

**Token Estimate:** ~30k tokens
**Predecessors:** TASK-002-002
**Successors:** TASK-002-005
**Parallel:** yes — can run alongside TASK-002-003 once TASK-002-002 is merged

**Acceptance Criteria:**
- [x] `handleIgnoreFeature()` is replaced by `handleApproveToggle()` in `status.go`
- [x] `handleApproveToggle()` calls `approval.Unapprove(m.dir, featureID)` when feature is currently approved, and `approval.Approve(m.dir, featureID)` when currently unapproved
- [x] After the toggle, `reloadFeatures()` is called so tab bar and task list reflect the new state immediately
- [x] `statusNote` is set to `"feature approved"` or `"feature unapproved"` after a successful toggle
- [x] `statusNote` is set to `"cannot approve a completed feature"` if triggered on a completed feature (no-op)
- [x] Footer hint updated: replace `alt+p: ignore/unignore feature` with `alt+p: approve/unapprove feature`
- [x] The `alt+p` key also works in the detail view (if currently handled there — mirror the existing `alt+i` in detail pattern)
- [x] `go test ./...` passes

---

### TASK-002-005: Remove the ignore system entirely

**Description:** As a maggus maintainer, I want all ignore infrastructure removed so that the codebase has one clean mechanism for feature gating and no dead code.

**Token Estimate:** ~60k tokens
**Predecessors:** TASK-002-001, TASK-002-003, TASK-002-004
**Successors:** none
**Parallel:** no — requires all prior tasks

**Acceptance Criteria:**
- [ ] `cmd/ignore.go` and `cmd/ignore_test.go` are deleted
- [ ] `cmd/unignore.go` and `cmd/unignore_test.go` are deleted
- [ ] `featureInfo.ignored` field removed from `status_plans.go`; all references to `f.ignored` / `p.ignored` replaced with `!f.approved` / `!p.approved` or removed
- [ ] `parser.Task.Ignored` field removed; `IGNORED TASK-NNN:` heading variant removed from `parser.TaskHeadingRe` and `parser.ParseFile`
- [ ] `parser.IsIgnoredFile()` removed; all call sites removed
- [ ] `parser.GlobFeatureFiles` and `parser.GlobBugFiles` no longer glob `*_ignored.md` files
- [ ] `_ignored` suffix stripping removed from `featureIDFromPath` in `cmd/approve.go`
- [ ] `featureStateIgnored` and `featureStateActive`/`featureStateNotFound`/`featureStateCompleted` enum removed from `cmd/approve.go` along with `findFeatureFile` (only used by deleted ignore/unignore commands); `listActiveFeatureIDs` / `featureExists` kept if still used by approve/unapprove commands
- [ ] `alt+i` key handler and `handleIgnoreTask()` removed from `status.go`
- [ ] `rewriteTaskHeading()` removed (was in `ignore.go`, now deleted)
- [ ] Plain output in `renderStatusPlain()` no longer references ignored state
- [ ] All tests updated or removed; no test references `Ignored`, `_ignored`, or ignore/unignore behavior
- [ ] `go build ./...` and `go test ./...` pass with zero errors

---

## Task Dependency Graph

```
TASK-002-001 ──────────────────────────────────────→ TASK-002-005
TASK-002-002 ──→ TASK-002-003 ──────────────────────┘
             ──→ TASK-002-004 ──┘
```

| Task | Estimate | Predecessors | Parallel | Model |
|------|----------|--------------|----------|-------|
| TASK-002-001 | ~30k | none | yes (with 002) | — |
| TASK-002-002 | ~35k | none | yes (with 001) | — |
| TASK-002-003 | ~25k | 002 | yes (with 004) | — |
| TASK-002-004 | ~30k | 002 | yes (with 003) | — |
| TASK-002-005 | ~60k | 001, 003, 004 | no | — |

**Total estimated tokens:** ~180k

## Functional Requirements

- FR-1: Any `_ignored.md` file found on load must be renamed to `.md` and recorded as `approved=false` in `feature_approvals.yml` automatically, without any user action.
- FR-2: `featureInfo.approved` must reflect `approval.IsApproved` using `approvalRequired=false` (opt-out semantics for display purposes regardless of config).
- FR-3: Each tab in the status TUI tab bar must show `✓` for approved and `✗` for unapproved features.
- FR-4: Unapproved feature tabs must render in a visually distinct dim/warning color.
- FR-5: Pressing `alt+p` on the selected feature must toggle its approval state and immediately refresh the view.
- FR-6: `alt+p` on a completed feature must be a no-op with an explanatory status note.
- FR-7: `maggus ignore` and `maggus unignore` commands must no longer exist in the binary.
- FR-8: The parser must not recognize `_ignored.md` filenames or `### IGNORED TASK-NNN:` headings after this feature.
- FR-9: `go build ./...` and `go test ./...` must pass with zero errors or test failures.

## Non-Goals

- No changes to the approval_mode config behavior or the work loop's approval gate logic.
- No task-level approval (approval remains feature-level only; task-level ignore is removed without a replacement in this iteration).
- No UI for the `maggus approve` / `maggus unapprove` CLI commands — those stay as-is.
- No migration of `### IGNORED TASK-NNN:` headings in existing feature files — after the parser stops recognizing them, they are treated as regular tasks.

## Technical Considerations

- `rewriteTaskHeading()` currently lives in `ignore.go` and is referenced by `status.go` (`handleIgnoreTask`). Both the function and the call site are removed together in TASK-002-005; make sure TASK-002-004 does not introduce a new call to it.
- `findFeatureFile()` and the `featureState` enum in `ignore.go` are used only by ignore/unignore commands. Once those files are deleted, these helpers go with them. Verify `approve.go` doesn't depend on them (it currently doesn't).
- `parseFeatures` and `parseBugs` currently don't take a `dir` parameter — TASK-002-002 adds it. All call sites are in `status.go`, `status_plans.go`, and `work_setup.go`; update all of them.
- The `_ignored.md` migration in TASK-002-001 must run before TASK-002-005 removes `_ignored.md` globbing from the parser — ordering is enforced by the dependency graph.
- After TASK-002-005, any user with leftover `_ignored.md` files that were not auto-migrated (e.g. they bypassed the status TUI and work loop) will have those files silently ignored by the parser. This is acceptable: the migration is best-effort.

## Success Metrics

- A user can approve or unapprove any feature without leaving the status TUI.
- The tab bar makes approval state immediately obvious without any extra keypress.
- `maggus ignore` and `maggus unignore` are gone from `maggus --help`.
- No regressions in existing approve/unapprove CLI commands or work loop behavior.

## Open Questions

_(none — all questions resolved)_
