<!-- maggus-id: 4715a80f-c462-4f9d-bff8-98d322f559bb -->
# Feature 001: Per-Task Model Override

## Introduction

Currently, Maggus uses a single model for all tasks in a run — configured via `.maggus/config.yml` or the `--model` CLI flag. Feature plans already support an optional `**Model:**` field per task (e.g. `**Model:** opus`), but the parser ignores it. This feature adds support for parsing that field and using it to override the configured model on a per-task basis.

## Goals

- Allow feature plan authors to specify a model per task using the existing `**Model:**` field
- Support both aliases (`opus`, `sonnet`, `haiku`) and full model IDs (`claude-opus-4-6`)
- Show the active model in the TUI when a task-level override is in effect
- Keep the override optional — when not set, the globally configured model is used as before

## Tasks

### TASK-001-001: Parse Model field from task markdown
**Description:** As a feature plan author, I want the parser to extract the optional `**Model:**` field from task sections so that per-task model preferences are available to the work loop.

**Token Estimate:** ~30k tokens
**Predecessors:** none
**Successors:** TASK-001-002
**Parallel:** yes — can run alongside TASK-001-003

**Acceptance Criteria:**
- [x] `Task` struct in `internal/parser/parser.go` has a new `Model string` field
- [x] Parser extracts `**Model:** <value>` from anywhere in the task section before acceptance criteria
- [x] Leading/trailing whitespace on the value is trimmed
- [x] When no `**Model:**` line is present, `Model` is empty string
- [x] Works for both feature files (`feature_*.md`) and bug files (`bug_*.md`) since they share the same parser
- [x] Unit tests cover: model present, model absent, model with whitespace, model with full ID vs alias
- [x] `go vet ./...` and `go test ./...` pass

### TASK-001-002: Use per-task model in the work loop
**Description:** As maggus, I want to use a task's model field (when set) to override the configured model so that each task runs with the right model.

**Token Estimate:** ~35k tokens
**Predecessors:** TASK-001-001, TASK-001-003
**Successors:** none
**Parallel:** no

**Acceptance Criteria:**
- [ ] In `cmd/work_task.go`, before calling `agent.Run()`, check if the current task has a non-empty `Model` field
- [ ] If set, resolve it through `config.ResolveModel()` (supporting aliases and full IDs)
- [ ] Pass the resolved per-task model to `agent.Run()` instead of `tc.resolvedModel`
- [ ] If not set, behavior is unchanged — uses `tc.resolvedModel` as before
- [ ] Unit tests cover: task with model override, task without model override, alias resolution
- [ ] `go vet ./...` and `go test ./...` pass

### TASK-001-003: Show active model in TUI
**Description:** As a user watching maggus work, I want to see which model is being used for each task so that I can confirm per-task overrides are in effect.

**Token Estimate:** ~25k tokens
**Predecessors:** none
**Successors:** TASK-001-002
**Parallel:** yes — can run alongside TASK-001-001

**Acceptance Criteria:**
- [ ] The TUI status display shows the model being used for the current task
- [ ] When a per-task override is active, the display distinguishes it from the default (e.g. `model: opus (task override)` vs `model: sonnet`)
- [ ] When no override is set, the display shows the configured model as before
- [ ] `go vet ./...` and `go test ./...` pass

## Task Dependency Graph

```
TASK-001-001 ──→ TASK-001-002
TASK-001-003 ──┘
```

| Task | Estimate | Predecessors | Parallel | Model |
|------|----------|--------------|----------|-------|
| TASK-001-001 | ~30k | none | yes (with 003) | — |
| TASK-001-003 | ~25k | none | yes (with 001) | — |
| TASK-001-002 | ~35k | 001, 003 | no | — |

**Total estimated tokens:** ~90k

## Functional Requirements

- FR-1: The parser must extract an optional `**Model:**` field from task sections in markdown feature and bug files
- FR-2: The `**Model:**` value must support both aliases (`opus`, `sonnet`, `haiku`) and full model IDs (`claude-opus-4-6`)
- FR-3: When a task has a `Model` field set, `agent.Run()` must be called with the task-level model instead of the globally configured model
- FR-4: When a task has no `Model` field, the globally configured model is used unchanged
- FR-5: The TUI must display the active model per task and indicate when a task-level override is active

## Non-Goals

- No support for per-task agent override (only model)
- No validation that the specified model actually exists — that's the agent backend's responsibility
- No changes to the config file format or CLI flags
- No per-criterion or per-feature model override — only per-task

## Technical Considerations

- The parser already extracts `**Description:**` using text between known markers — the `**Model:**` field should use a similar regex-based approach
- `config.ResolveModel()` already handles alias expansion — reuse it for task-level models
- The `taskContext` struct in `work_task.go` carries `resolvedModel` — the override should happen at invocation time, not by mutating the context

## Success Metrics

- Running `maggus work` on a feature file with mixed `**Model:**` values invokes Claude Code with the correct `--model` flag per task
- TUI clearly shows when a task-level model override is active

## Open Questions

*None — all questions resolved.*
