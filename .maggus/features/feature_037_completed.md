<!-- maggus-id: 1ad99cc5-8e50-4ade-9f36-392c0507f4ab -->
# Feature 037: Configurable Session Persistence Flag

## Introduction

Add a `session_persistence` config field to `.maggus/config.yml` that controls whether the `--no-session-persistence` flag is passed to the Claude Code subprocess. When `false` (the default), the flag is added. When `true`, the flag is omitted, allowing Claude to persist sessions between invocations.

### Architecture Context

- **Components involved:** `internal/config` (Config struct, defaults), `internal/agent/claude.go` (subprocess arg building)
- **Vision alignment:** Supports the "pluggable agent backend" and "per-repo configuration" concepts — users can tune agent behavior per project
- **No new components** — extends existing config and agent invocation

## Goals

- Allow users to control `--no-session-persistence` via config without editing code
- Default to `false` (flag added) so sessions don't persist by default
- Apply the setting to both `Run` (streaming) and `RunOnce` (text) invocations

## Tasks

### TASK-037-001: Add `session_persistence` to Config struct and defaults
**Description:** As a developer, I want the Config struct to include `session_persistence` so that the value is parsed from `config.yml` and has a sensible default.

**Token Estimate:** ~25k tokens
**Predecessors:** none
**Successors:** TASK-037-002
**Parallel:** no

**Acceptance Criteria:**
- [x] `src/internal/config/config.go`: `Config` struct has a new field `SessionPersistence *bool `yaml:"session_persistence"``
- [x] Default value is `false` (i.e. when the field is omitted from config.yml, it resolves to `false`)
- [x] The default is applied in the same way other `*bool` defaults are applied (e.g. `AutoBranch`, `CheckSync`)
- [x] `go vet ./...` and `go build ./...` pass
- [x] Unit tests cover: field absent → `false`, field explicitly `true` → `true`, field explicitly `false` → `false`

### TASK-037-002: Pass `--no-session-persistence` flag based on config
**Description:** As a user, I want the Claude subprocess to receive `--no-session-persistence` when `session_persistence` is `false` so that I can control session behavior from config.

**Token Estimate:** ~35k tokens
**Predecessors:** TASK-037-001
**Successors:** TASK-037-003
**Parallel:** no

**Acceptance Criteria:**
- [x] `src/internal/agent/claude.go`: The `Run` method appends `--no-session-persistence` to args when session persistence is `false`
- [x] `src/internal/agent/claude.go`: The `RunOnce` method appends `--no-session-persistence` to args when session persistence is `false`
- [x] The config value is threaded from `runSetup()` through to the agent — either via the `ClaudeAgent` struct or via the method signature (follow the existing pattern for how `model` flows)
- [x] When `session_persistence: true` is set in config, the flag is NOT added to the subprocess args
- [x] `go vet ./...` and `go build ./...` pass
- [x] Unit tests verify args contain `--no-session-persistence` when config is `false`, and do not contain it when `true`

### TASK-037-003: Update ARCHITECTURE.md and docs
**Description:** As a user reading the docs, I want to know about the `session_persistence` config field so I can use it.

**Token Estimate:** ~15k tokens
**Predecessors:** TASK-037-002
**Successors:** none
**Parallel:** no
**Model:** haiku

**Acceptance Criteria:**
- [x] `ARCHITECTURE.md`: The agent invocation section mentions `--no-session-persistence` is conditionally passed based on config
- [x] `docs/reference/configuration.md` (if it exists): `session_persistence` is documented with type (`bool`), default (`false`), and behavior description
- [x] `go vet ./...` and `go build ./...` still pass (no code changes, but verify nothing broke)

## Task Dependency Graph

```
TASK-037-001 ──→ TASK-037-002 ──→ TASK-037-003
```

| Task | Estimate | Predecessors | Parallel | Model |
|------|----------|--------------|----------|-------|
| TASK-037-001 | ~25k | none | no | — |
| TASK-037-002 | ~35k | 001 | no | — |
| TASK-037-003 | ~15k | 002 | no | haiku |

**Total estimated tokens:** ~75k

## Functional Requirements

- FR-1: When `session_persistence` is absent or `false` in `.maggus/config.yml`, the Claude subprocess must receive `--no-session-persistence`
- FR-2: When `session_persistence` is `true`, the flag must not be passed
- FR-3: The flag must be applied to both `Run` (streaming) and `RunOnce` (text) invocations
- FR-4: No CLI flag override — config only

## Non-Goals

- No per-task override of session persistence
- No CLI flag (`--no-session-persistence` on `maggus run`) — config only
- No changes to other agent backends or the Agent interface

## Technical Considerations

- The `session_persistence` field uses `*bool` to distinguish "not set" from "explicitly false", following the existing pattern for `AutoBranch`, `CheckSync`, etc.
- The config value needs to reach the `ClaudeAgent` — check how `model` currently flows through `runSetup` → `taskContext` → `agent.Run()` and follow the same pattern
- The `--no-session-persistence` flag is a Claude Code CLI flag that prevents session data from being saved between invocations

## Success Metrics

- `maggus run` with default config passes `--no-session-persistence` to claude
- Adding `session_persistence: true` to config omits the flag
- No regressions in existing tests

## Open Questions

*None — all questions resolved.*
