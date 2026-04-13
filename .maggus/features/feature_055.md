<!-- maggus-id: c8c34efa-61c8-4cd6-92d7-29b229e158b5 -->
# Feature 055: Cross-Feature Predecessor Support

## Introduction

Tasks in a feature plan can declare dependencies on entire other features, not just individual tasks within the same feature. For example:

```
**Predecessors:** TASK-014-001, Feature 006 (DTOs), Features 004-013 (all RMC features)
```

Currently the parser treats `Feature 006 (DTOs)` and `Features 004-013 (all RMC features)` as unknown task IDs, which are silently added to `skippedOrBlockedIDs` as a defensive fallback (BUG-047-002). This means the task runs even if the referenced feature is not complete — and shows as "(cross-feature)" in the plan tab with no completion-aware resolution.

This feature makes cross-feature predecessor references first-class: they are parsed explicitly, resolved against actual feature completion state at runtime, and surfaced clearly in the TUI.

### Architecture Context

- **Vision alignment:** Spec-driven development requires correct task sequencing. Cross-feature dependencies are a real coordination pattern in multi-feature projects. Getting them wrong means work runs out of order.
- **Components involved:** Parser (`internal/parser`), Orchestrator (`cmd/orchestrator.go` and related), Execution plan builder (`cmd/status_execution.go`), Plan tab renderer (`cmd/status_plantab.go`), Daemon snapshot (`internal/runlog`)
- **New patterns:** Cross-feature completion check function in `internal/parser`; `CrossFeatureRef` type on `Task` struct; `featureDir` threading into execution plan and TUI rendering

## Goals

- Parse `Feature NNN (label)` and `Features NNN-MMM (label)` tokens from `**Predecessors:**` fields as explicit cross-feature references — not as unknown task IDs
- Block a task at runtime if any referenced feature is not yet complete
- When all remaining tasks in a feature are blocked on cross-feature deps, show "Waiting for Feature NNN (Label)" in the daemon snapshot instead of "Preparing"
- In the execution plan tab, show which feature(s) a task is waiting on, and move the task to its normal phase once those features are complete
- Define "complete" as: `feature_NNN_completed.md` exists, OR all tasks in `feature_NNN.md` have all criteria checked, OR neither file exists (treat missing as complete)

## Tasks

### TASK-055-001: Parser — cross-feature predecessor parsing

**Description:** As a developer, I want the parser to recognise `Feature NNN (label)` and `Features NNN-MMM (label)` tokens in `**Predecessors:**` fields and store them as structured cross-feature references, separate from same-feature task IDs.

**Token Estimate:** ~55k tokens
**Predecessors:** none
**Successors:** TASK-055-003, TASK-055-004
**Parallel:** yes — can run alongside TASK-055-002

**Acceptance Criteria:**
- [x] New type `CrossFeatureRef` defined in `internal/parser/`:
  ```go
  type CrossFeatureRef struct {
      FeatureNums []string // e.g. ["006"] for single, ["004","005",...,"013"] for range
      DisplayText string   // original token text, e.g. "Feature 006 (DTOs)" or "Features 004-013 (all RMC features)"
  }
  ```
- [x] `Task` struct has new field `CrossFeaturePredecessors []CrossFeatureRef`
- [x] Parser recognises `Feature NNN` and `Feature NNN (label)` tokens (singular, case-insensitive "Feature") and adds one `CrossFeatureRef` with a single `FeatureNum`
- [x] Parser recognises `Features NNN-MMM` and `Features NNN-MMM (label)` tokens (plural "Features" + dash range) and adds one `CrossFeatureRef` with all feature numbers from NNN to MMM inclusive
- [x] Cross-feature tokens are NOT added to `Predecessors []string` — they are extracted cleanly to `CrossFeaturePredecessors`
- [x] Non-matching tokens (genuine task IDs like `TASK-NNN-MMM`, or truly unknown IDs) continue to be handled exactly as before
- [x] `none` prefix handling is unaffected (still treated as no predecessors)
- [x] Tests cover: single feature ref, range ref, mixed line with task ID and cross-feature ref, range with no label, feature ref at start/middle/end of comma list
- [x] `go vet ./...` and `go test ./...` pass

---

### TASK-055-002: Feature completion checker

**Description:** As a developer, I want a function that determines whether a referenced feature is complete, so the orchestrator and TUI can evaluate cross-feature predecessor state.

**Token Estimate:** ~35k tokens
**Predecessors:** none
**Successors:** TASK-055-003, TASK-055-004
**Parallel:** yes — can run alongside TASK-055-001

**Acceptance Criteria:**
- [x] New function `IsFeatureComplete(featureDir, featureNum string) bool` in `internal/parser/` (e.g. `feature_complete.go`):
  - Returns `true` if `feature_NNN_completed.md` exists in `featureDir`
  - Falls back: returns `true` if `feature_NNN.md` exists and all tasks in it have all acceptance criteria checked
  - Returns `true` if neither file exists (missing feature = treat as complete, so dependent tasks are not permanently blocked)
  - Returns `false` only when `feature_NNN.md` exists and has at least one incomplete task
- [x] New method on `Task`: `CrossFeaturePredecessorsSatisfied(featureDir string) bool` — returns `true` if every `CrossFeatureRef` in `CrossFeaturePredecessors` has all its `FeatureNums` satisfied by `IsFeatureComplete`; returns `true` if `CrossFeaturePredecessors` is empty
- [x] Tests cover:
  - `_completed.md` present → true
  - `feature_NNN.md` present, all tasks complete → true
  - `feature_NNN.md` present, one task incomplete → false
  - Neither file exists → true
  - Range ref: all in range complete → true
  - Range ref: one in range incomplete → false
- [x] `go vet ./...` and `go test ./...` pass

---

### TASK-055-003: Orchestrator — cross-feature blocking and "Waiting for Feature" snapshot status

**Description:** As a user, I want the daemon to correctly block tasks whose cross-feature predecessors are not yet complete, and to show a descriptive "Waiting for Feature NNN" status instead of "Preparing" when blocked.

**Token Estimate:** ~85k tokens
**Predecessors:** TASK-055-001, TASK-055-002
**Successors:** none
**Parallel:** yes — can run alongside TASK-055-004
**Model:** opus

**Acceptance Criteria:**
- [x] The orchestrator's `classifyWorkable` (or equivalent) calls `task.CrossFeaturePredecessorsSatisfied(featureDir)` as an additional gate; a task whose cross-feature deps are unsatisfied is classified as not runnable for this cycle
- [x] The `featureDir` (the `.maggus/features/` path) is available in the orchestrator context and passed through correctly
- [x] The existing workaround in `orchestrator.go` that adds unknown predecessor IDs to `skippedOrBlockedIDs` is **not** applied to cross-feature refs (they no longer appear in `Predecessors []string`, so no change is needed — just verify and confirm in a comment)
- [x] `countWorkable` and `firstWorkableTask` (used in `daemon_keepalive.go` pre-check) also respect cross-feature blocking — pass `featureDir` if needed, or update to call `CrossFeaturePredecessorsSatisfied`
- [x] When the orchestrator determines that the next task(s) to run are all blocked on cross-feature deps, it writes a snapshot status:
  - Single cross-feature ref: `"Waiting for Feature 006 (DTOs)"`
  - Multiple refs: `"Waiting for Feature 006 (DTOs), Features 004-013 (all RMC features)"`
  - This replaces the "Preparing" status that would otherwise persist for the entire waiting period (same mechanism as BUG-049)
- [x] When cross-feature deps become satisfied (referenced feature completes between daemon cycles), the task is picked up normally on the next cycle
- [x] Sequential-only features with no cross-feature deps are unaffected
- [x] `go vet ./...` and `go test ./...` pass

---

### TASK-055-004: Execution plan tab and status TUI — cross-feature dep display

**Description:** As a user, I want the execution plan tab to show which features a task is waiting on, and to show the task in its normal phase once those features are complete, so I understand exactly what is blocking progress.

**Token Estimate:** ~65k tokens
**Predecessors:** TASK-055-001, TASK-055-002
**Successors:** none
**Parallel:** yes — can run alongside TASK-055-003

**Acceptance Criteria:**
- [x] `buildExecutionPlan` in `status_execution.go` accepts `featureDir string` and calls `IsFeatureComplete` for each task's cross-feature deps:
  - If all cross-feature deps are satisfied: task appears in its normal phase (no longer in the cross-feature group)
  - If any cross-feature dep is unsatisfied: task stays in the cross-feature group
- [x] In `status_plantab.go`, cross-feature group tasks render the `DisplayText` of each unsatisfied `CrossFeatureRef` as an annotation:
  - e.g. `TASK-014-002  ← depends on Feature 006 (DTOs)` (exact formatting at implementor's discretion, but must be visually distinct and reference the feature name/label)
- [x] If a task has multiple unsatisfied cross-feature refs, all are shown (one per line, or comma-separated — implementor's choice)
- [x] The `(cross-feature)` group heading remains as-is (bug_050 already renamed it from `(unresolved)`)
- [x] `status_execution.go` and related callers compile; `featureDir` is threaded correctly from the TUI's working directory context
- [x] `go vet ./...` and `go test ./...` pass

---

## Task Dependency Graph

```
TASK-055-001 ──┬──→ TASK-055-003
               │
TASK-055-002 ──┤
               │
               └──→ TASK-055-004
```

| Task | Estimate | Predecessors | Parallel | Model |
|---|---|---|---|---|
| TASK-055-001 | ~55k | none | yes (with 002) | — |
| TASK-055-002 | ~35k | none | yes (with 001) | — |
| TASK-055-003 | ~85k | 001, 002 | yes (with 004) | opus |
| TASK-055-004 | ~65k | 001, 002 | yes (with 003) | — |

**Total estimated tokens:** ~240k

## Functional Requirements

- FR-1: The parser must recognise `Feature NNN` (singular) with optional ` (label)` as a single cross-feature reference.
- FR-2: The parser must recognise `Features NNN-MMM` (plural + dash range) with optional ` (label)` as a range cross-feature reference, expanding to all feature numbers from NNN to MMM inclusive.
- FR-3: Cross-feature tokens must be stored in `Task.CrossFeaturePredecessors []CrossFeatureRef` and must NOT be stored in `Task.Predecessors []string`.
- FR-4: A feature is considered complete if: (a) `feature_NNN_completed.md` exists, or (b) `feature_NNN.md` exists and all tasks in it have all criteria checked, or (c) neither file exists.
- FR-5: A task with at least one unsatisfied cross-feature predecessor must not be scheduled by the orchestrator.
- FR-6: When all remaining tasks in a feature group are blocked on cross-feature deps, the daemon snapshot status must read `"Waiting for [DisplayText, ...]"` rather than `"Preparing"`.
- FR-7: In the execution plan tab, a task blocked on cross-feature deps must show the feature name/label of each unsatisfied dependency.
- FR-8: When all cross-feature deps for a task are satisfied, the task must appear in its normal phase in the execution plan tab (not in the cross-feature group).
- FR-9: Mixed predecessor lines (e.g. `TASK-014-001, Feature 006 (DTOs)`) must correctly split same-feature task IDs into `Predecessors` and cross-feature refs into `CrossFeaturePredecessors`.

## Non-Goals

- `Bug NNN` cross-bug-fix predecessor references — not in scope for this feature.
- Partial feature completion tracking (e.g. "wait until 80% of feature_006 tasks are done") — it's all-or-nothing.
- Bidirectional awareness (feature_006 does not know that feature_009 depends on it).
- Transitively resolving cross-feature deps (feature_006 may itself have cross-feature deps; those are not evaluated when checking if 006 is complete — only 006's own task completion is checked).

## Technical Considerations

- Feature numbers in file names are zero-padded three-digit strings (`"006"`, `"013"`). The parser should normalise the parsed number to the same format for consistent file lookups.
- The `featureDir` must be the same directory used by the parser to load feature files — typically `.maggus/features/` relative to the repo root.
- `IsFeatureComplete` reads from the file system on every call; for large ranges this may be called many times per cycle. Consider caching the result per cycle within the orchestrator (a simple `map[string]bool` populated once per orchestrator run is sufficient).
- `status_execution.go` currently receives plan data without a `featureDir` parameter. When adding `featureDir`, check all callers of `buildExecutionPlan` and update accordingly.
- The `CrossFeatureRef.DisplayText` should preserve the original token text exactly as written in the plan file (including "Features" vs "Feature" and the label), so the TUI shows what the author wrote.

## Success Metrics

- A task with `**Predecessors:** Feature 006 (DTOs)` is correctly blocked until `feature_006_completed.md` exists or all tasks in `feature_006.md` are checked.
- The daemon snapshot shows `"Waiting for Feature 006 (DTOs)"` while the task is blocked — not `"Preparing"`.
- The execution plan tab shows `depends on Feature 006 (DTOs)` next to the blocked task, and moves it to its normal phase once feature_006 is complete.
- `Features 004-013 (all RMC features)` blocks the task until all 10 features are complete.
- Mixed lines like `TASK-014-001, Features 004-013 (all RMC features)` parse correctly with the task ID in `Predecessors` and the range in `CrossFeaturePredecessors`.

## Open Questions

_(none)_
