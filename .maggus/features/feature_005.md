# Feature 005: Configurable On-Complete Behaviour for Feature and Bug Files

## Introduction

When Maggus finishes all tasks in a feature or bug file, it currently always renames the file by appending `_completed` (e.g. `feature_001.md` → `feature_001_completed.md`). This feature adds a per-repository config option to choose between **rename** (current default) and **delete** (permanently remove the file). The setting is exposed in the interactive `maggus config` TUI and saved to `.maggus/config.yml`.

### Architecture Context

- **Components involved:** `internal/config` (struct + YAML), `internal/parser` (MarkCompleted functions), `cmd/work_task.go` (caller), `cmd/config.go` (TUI)
- **No new packages required** — this is a pure extension of existing config and parser patterns
- **Pattern:** follows the same approach as `NotificationsConfig` (nested struct with helper methods) and the existing `configRow` cycle pattern in the TUI

## Goals

- Allow users to configure whether completed feature/bug files are renamed or deleted
- Expose the setting in the `maggus config` TUI, grouped clearly under "On complete behaviour:"
- Persist the setting to `.maggus/config.yml` on save
- Default to `rename` so existing behaviour is unchanged

## Tasks

### TASK-005-001: Add `OnCompleteConfig` struct to the config package

**Description:** As a developer, I want a typed config struct for on-complete behaviour so that the rest of the codebase can read and default it correctly.

**Token Estimate:** ~15k tokens
**Predecessors:** none
**Successors:** TASK-005-002, TASK-005-003, TASK-005-004
**Parallel:** no — other tasks depend on this
**Model:** haiku

**Acceptance Criteria:**
- [x] A new `OnCompleteConfig` struct is added in `src/internal/config/config.go` with two `string` fields: `Feature` and `Bug` (yaml tags `feature` and `bug`)
- [x] `Config` has a new field `OnComplete OnCompleteConfig` with yaml tag `on_complete`
- [x] Two helper methods are added: `OnCompleteConfig.FeatureAction() string` and `OnCompleteConfig.BugAction() string`, each returning `"rename"` when the field is empty or not `"delete"`, and `"delete"` when it is `"delete"`
- [x] Unit tests in `config_test.go` (or a new `onComplete_test.go`) verify: zero value returns `"rename"`, `"rename"` returns `"rename"`, `"delete"` returns `"delete"`, unknown string returns `"rename"`
- [x] `go test ./internal/config` passes
- [x] `go vet ./...` passes

---

### TASK-005-002: Update parser `MarkCompleted*` functions to support delete action

**Description:** As the work loop, I want to pass the configured action to the parser so that completed files are either renamed or deleted depending on config.

**Token Estimate:** ~30k tokens
**Predecessors:** TASK-005-001
**Successors:** TASK-005-004
**Parallel:** yes — can run alongside TASK-005-003

**Acceptance Criteria:**
- [ ] `MarkCompletedFeatures(dir string)` signature changes to `MarkCompletedFeatures(dir, action string) error` in `src/internal/parser/parser.go`
- [ ] `MarkCompletedBugs(dir string)` signature changes to `MarkCompletedBugs(dir, action string) error`
- [ ] When `action == "delete"`, a fully-completed file is removed via `os.Remove` instead of renamed
- [ ] When `action` is anything else (including `"rename"` or empty string), behaviour is unchanged (rename to `_completed`)
- [ ] All existing callers in `src/cmd/work_task.go` are updated to pass `""` (empty = rename) as a temporary placeholder — the final wiring happens in TASK-005-004
- [ ] Existing parser tests still pass; new tests cover: delete removes the file, rename still works, unknown action defaults to rename
- [ ] `go test ./internal/parser` passes

---

### TASK-005-003: Add "On complete behaviour" rows to the config TUI

**Description:** As a user, I want to see and change the on-complete behaviour for features and bugs in the `maggus config` screen so that I don't have to edit the YAML file manually.

**Token Estimate:** ~35k tokens
**Predecessors:** TASK-005-001
**Successors:** TASK-005-004
**Parallel:** yes — can run alongside TASK-005-002

**Acceptance Criteria:**
- [ ] In `src/cmd/config.go`, after the existing "Sound" / notification rows and before the save buttons, a section label row `"On complete behaviour:"` is rendered as a non-navigable header (use the same section mechanism as "Project" / "Global", or a simple display row — whichever fits more cleanly)
- [ ] Two new option rows appear indented under it: `"  Feature"` with values `["rename", "delete"]` and `"  Bug"` with values `["rename", "delete"]`
- [ ] Initial values are read from `cfg.OnComplete.FeatureAction()` and `cfg.OnComplete.BugAction()`
- [ ] `buildConfig()` maps the TUI selections back to `cfg.OnComplete.Feature` and `cfg.OnComplete.Bug` (storing `"rename"` or `"delete"` as plain strings; when `"rename"` is selected the field is left empty to keep the YAML clean — same pattern as other optional fields)
- [ ] Saving project config via the TUI writes the correct `on_complete:` block to `.maggus/config.yml`
- [ ] `go build ./...` passes
- [ ] `go test ./cmd` passes (existing config tests must not break)

---

### TASK-005-004: Wire config into `work_task.go` calls

**Description:** As the work loop, I want the correct action passed to `MarkCompleted*` so that the user's configured behaviour is actually applied at runtime.

**Token Estimate:** ~15k tokens
**Predecessors:** TASK-005-001, TASK-005-002, TASK-005-003
**Successors:** none
**Parallel:** no
**Model:** haiku

**Acceptance Criteria:**
- [ ] `taskContext` struct in `src/cmd/work_task.go` (or its setup in `work.go`) has the loaded `config.Config` available (it may already be reachable — check before adding a field)
- [ ] The two calls in `work_task.go` are updated: `parser.MarkCompletedFeatures(tc.workDir, cfg.OnComplete.FeatureAction())` and `parser.MarkCompletedBugs(tc.workDir, cfg.OnComplete.BugAction())`
- [ ] A manual smoke test (or integration-style test if one exists) confirms: with `on_complete: {feature: delete}` in config, a fully-completed feature file is removed after the work loop; with default config, it is renamed
- [ ] `go build ./...` passes
- [ ] `go test ./...` passes

---

## Task Dependency Graph

```
TASK-005-001 ──→ TASK-005-002 ──→ TASK-005-004
             └─→ TASK-005-003 ──┘
```

| Task | Estimate | Predecessors | Parallel | Model |
|------|----------|--------------|----------|-------|
| TASK-005-001 | ~15k | none | no | haiku |
| TASK-005-002 | ~30k | 001 | yes (with 003) | — |
| TASK-005-003 | ~35k | 001 | yes (with 002) | — |
| TASK-005-004 | ~15k | 002, 003 | no | haiku |

**Total estimated tokens:** ~95k

## Functional Requirements

- FR-1: The config struct must expose `on_complete.feature` and `on_complete.bug` YAML fields with string values `"rename"` or `"delete"`
- FR-2: When the action is `"delete"`, the completed file must be permanently removed from disk
- FR-3: When the action is `"rename"` or is unset, the current rename-to-`_completed` behaviour is preserved exactly
- FR-4: The config TUI must display an "On complete behaviour:" group with two indented toggle rows: "  Feature" and "  Bug", each cycling between `rename` and `delete`
- FR-5: Saving via the TUI must persist the selected values to `.maggus/config.yml` under the `on_complete:` key
- FR-6: Defaults must preserve backwards compatibility — a config without `on_complete` behaves identically to today

## Non-Goals

- No "archive" or "move to folder" option — delete means `os.Remove`, nothing more
- No global (per-user) config for this setting — project config only
- No UI to configure this outside of `maggus config` or direct YAML editing
- No undo / recycle bin behaviour

## Technical Considerations

- The `MarkCompletedFeatures` / `MarkCompletedBugs` signature change is breaking for any external callers — check `src/` for all call sites before changing (currently only `work_task.go`)
- The TUI section label "On complete behaviour:" should use the existing `section` field on `configRow` if it fits, otherwise a `display` row with empty `label` can serve as a visual separator — keep it consistent with the rest of the screen
- When saving, storing an empty string for `"rename"` (the default) keeps the YAML minimal and avoids noise in diffs. Only write `"delete"` explicitly. Mirror this with the helper method returning `"rename"` for empty input.
- `go vet` and `go test ./...` must pass after every task

## Success Metrics

- User can open `maggus config`, toggle "  Feature" to `delete`, save, and on the next completed run the feature file is gone rather than renamed
- Default behaviour (no config change) is identical to the current release
- All existing tests pass without modification
