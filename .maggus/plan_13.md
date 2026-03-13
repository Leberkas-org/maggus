# Plan: Multi-Agent Support (Claude Code + OpenCode)

## Introduction

Maggus currently only supports Claude Code as its AI backend. This plan abstracts the agent layer behind a common interface so users can configure either **Claude Code** or **OpenCode** (or future agents) via `.maggus/config.yml`. The config gains a new `agent` field (separate from `model`), model specification switches to the `provider/model` format as the default, and each agent adapter manages its own CLI flags and streaming JSON parsing. Documentation is updated to cover both agents with configuration examples and the `maggus-plan` skill is clearly documented.

## Goals

- Introduce an `Agent` interface that abstracts CLI invocation and streaming event parsing
- Implement adapters for Claude Code and OpenCode behind that interface
- Add an `agent` config field and `--agent` CLI flag to select the backend
- Switch model specification to `provider/model` format as the canonical form
- Normalize streaming JSON from both agents into Maggus's internal event types
- Update VitePress documentation with agent configuration guides and examples
- Document the `maggus-plan` skill in the docs site

## User Stories

### TASK-001: Define the Agent interface and internal event types

**Description:** As a developer, I want a clean `Agent` interface so that Maggus can invoke different AI backends through a unified contract.

**Acceptance Criteria:**
- [x] New package `internal/agent` with an `Agent` interface defining: `Run(ctx, prompt, model, program) error` (streaming mode) and `RunOnce(ctx, prompt, model) (string, error)` (text mode)
- [x] Internal event types (messages sent to bubbletea) remain in the existing `runner` package or a shared `internal/tui/messages` package — the Agent interface uses `*tea.Program` for event delivery, same as today
- [x] The interface includes a `Name() string` method returning the agent identifier (e.g. `"claude"`, `"opencode"`)
- [x] The interface includes a `Validate() error` method that checks if the agent CLI is available on PATH
- [x] Typecheck/lint passes (`go vet ./...`, `go fmt ./...`)

### TASK-002: Implement the Claude Code adapter

**Description:** As a developer, I want the existing Claude Code invocation logic extracted into an adapter that implements the `Agent` interface.

**Acceptance Criteria:**
- [ ] New file `internal/agent/claude.go` implementing the `Agent` interface for Claude Code
- [ ] All logic from `runner.RunClaude` and `runner.RunOnce` is moved into the adapter — the runner package becomes a thin wrapper or is replaced
- [ ] Executable lookup uses `exec.LookPath("claude")`
- [ ] CLI flags: `-p`, `--output-format stream-json`, `--verbose`, `--dangerously-skip-permissions`, `--model` (conditional)
- [ ] Streaming JSON parsing handles Claude Code's event schema (`type: "assistant"`, `type: "result"`, usage, tool_use blocks) exactly as today
- [ ] The `describeToolUse` helper and tool-name mappings (Bash, Read, Edit, Write, Glob, Grep, Skill, MCP) remain functional
- [ ] Process management (Windows `taskkill` / Unix `Kill`, `WaitDelay`, `setProcAttr`) is preserved
- [ ] Existing tests still pass — `go test ./...`
- [ ] Typecheck/lint passes

### TASK-003: Implement the OpenCode adapter

**Description:** As a developer, I want an OpenCode adapter so that Maggus can use OpenCode as an alternative AI backend.

**Acceptance Criteria:**
- [ ] New file `internal/agent/opencode.go` implementing the `Agent` interface for OpenCode
- [ ] Executable lookup uses `exec.LookPath("opencode")`
- [ ] For streaming mode (`Run`): invokes `opencode -p <prompt> --output-format stream-json` (or the correct OpenCode equivalent flags) with `--model` if provided
- [ ] OpenCode does NOT need `--dangerously-skip-permissions` — non-interactive mode auto-approves
- [ ] Streaming JSON parser handles OpenCode's event schema and normalizes events into the same bubbletea messages (StatusMsg, OutputMsg, ToolMsg, UsageMsg, etc.)
- [ ] For text mode (`RunOnce`): invokes `opencode -p <prompt> --output-format text` (or equivalent)
- [ ] Process management follows the same pattern as Claude adapter (setProcAttr, Cancel, WaitDelay)
- [ ] `Validate()` checks for `opencode` on PATH and returns a helpful error if not found
- [ ] Typecheck/lint passes

### TASK-004: Add agent registry and factory function

**Description:** As a developer, I want a registry that maps agent names to their constructors so the work loop can instantiate the right adapter from config.

**Acceptance Criteria:**
- [ ] New file `internal/agent/registry.go` with a `New(name string) (Agent, error)` factory function
- [ ] Supported names: `"claude"` (default), `"opencode"`
- [ ] Unknown agent names return a clear error listing available agents
- [ ] An empty or omitted name defaults to `"claude"` for backwards compatibility
- [ ] Unit tests cover: default selection, explicit selection, unknown agent error
- [ ] Typecheck/lint passes

### TASK-005: Update config to support `agent` field and `provider/model` format

**Description:** As a user, I want to set `agent: opencode` in my config and use `provider/model` format for model specification.

**Acceptance Criteria:**
- [ ] `Config` struct gains an `Agent` field (`yaml:"agent"`)
- [ ] Model specification uses `provider/model` as the canonical format (e.g. `anthropic/claude-sonnet-4-6`, `openai/gpt-4.1`)
- [ ] Legacy short aliases (`sonnet`, `opus`, `haiku`) still work and resolve to full IDs for backwards compatibility — `ResolveModel` is updated accordingly
- [ ] Full model IDs without provider prefix (e.g. `claude-sonnet-4-6`) still work for backwards compatibility
- [ ] If `agent` is empty or omitted, it defaults to `"claude"`
- [ ] Unit tests cover: new config field parsing, model resolution with provider prefix, legacy alias resolution, default agent
- [ ] Typecheck/lint passes

### TASK-006: Wire agent selection into the work loop and CLI

**Description:** As a user, I want `maggus work --agent opencode` to use OpenCode, and the config file `agent:` field to be respected.

**Acceptance Criteria:**
- [ ] `cmd/work.go` uses `agent.New(agentName)` to instantiate the agent, where `agentName` comes from CLI flag > config > default
- [ ] New `--agent` flag on `maggus work` command (string, default empty)
- [ ] Precedence: `--agent` CLI flag > `config.yml` agent field > `"claude"` default
- [ ] The agent's `Validate()` is called before the work loop starts — exits with a clear error if the CLI tool isn't installed
- [ ] The agent's `Run()` replaces the direct `runner.RunClaude()` call
- [ ] The agent's `RunOnce()` replaces any `runner.RunOnce()` calls
- [ ] The TUI startup banner shows the active agent name (e.g. `Agent: claude` or `Agent: opencode`)
- [ ] The `--dangerously-skip-permissions` warning is only shown for agents that use it (Claude Code)
- [ ] Existing behavior is unchanged when no `agent` field is set (defaults to Claude Code)
- [ ] Typecheck/lint passes

### TASK-007: Update `list` and `status` commands to show configured agent

**Description:** As a user, I want `maggus list` and `maggus status` to show which agent is configured so I know what backend will be used.

**Acceptance Criteria:**
- [ ] `maggus status` output includes the configured agent name in its summary section (e.g. `Agent: opencode`)
- [ ] `maggus list` header or footer shows the configured agent when printing task previews
- [ ] `--plain` mode includes agent info in a machine-readable format
- [ ] If agent is the default (`claude`), it still displays explicitly for clarity
- [ ] Typecheck/lint passes

### TASK-008: Update VitePress docs — Configuration reference

**Description:** As a user, I want the configuration docs to explain the new `agent` and `provider/model` fields with examples for both Claude Code and OpenCode.

**Acceptance Criteria:**
- [ ] `docs/reference/configuration.md` documents the new `agent` field with allowed values and default behavior
- [ ] Model section is updated to show `provider/model` as the primary format with examples for multiple providers
- [ ] Legacy alias table is preserved but marked as a convenience shorthand
- [ ] Full config example shows both a Claude Code setup and an OpenCode setup side by side
- [ ] CLI flag `--agent` is documented with precedence rules
- [ ] Typecheck/lint passes (VitePress builds without errors: `npm run docs:build` in `docs/`)

### TASK-009: Update VitePress docs — Getting Started guide

**Description:** As a new user, I want the Getting Started guide to explain that Maggus supports multiple agents and show how to set up either Claude Code or OpenCode.

**Acceptance Criteria:**
- [ ] Prerequisites section lists both Claude Code and OpenCode as supported backends, with links to their installation docs
- [ ] A short section explains that Maggus defaults to Claude Code but can be switched to OpenCode via config or CLI flag
- [ ] The "First Run" example still uses Claude Code as the default path (minimal change for simplicity)
- [ ] A callout/tip box shows how to switch to OpenCode for users who prefer it
- [ ] Typecheck/lint passes (VitePress builds without errors)

### TASK-010: Update VitePress docs — CLI Commands reference

**Description:** As a user, I want the CLI Commands reference to document the new `--agent` flag and show agent-aware example output.

**Acceptance Criteria:**
- [ ] `maggus work` section documents the `--agent` flag in the flags table
- [ ] Example output section shows the `Agent:` line in the startup banner
- [ ] Examples include `maggus work --agent opencode` usage
- [ ] `maggus status` section mentions that it shows the configured agent
- [ ] Typecheck/lint passes (VitePress builds without errors)

### TASK-011: Update VitePress docs — Concepts page

**Description:** As a user, I want the Concepts page to explain the agent abstraction and how different backends integrate.

**Acceptance Criteria:**
- [ ] New "Agents" section (after "Work Loop Lifecycle" or as a sub-section) explaining: what an agent is in Maggus, which agents are supported, and how they differ (permissions handling, model format, etc.)
- [ ] Work Loop Lifecycle section is updated to say "Invoke the configured agent" instead of "Invoke Claude Code" where appropriate
- [ ] Mentions that agent selection affects CLI flags and streaming format but not the plan/task workflow
- [ ] Typecheck/lint passes (VitePress builds without errors)

### TASK-012: Create VitePress docs page for the `maggus-plan` skill

**Description:** As a user, I want a dedicated documentation page explaining the `maggus-plan` skill — what it is, how to use it, and how it integrates with the Maggus workflow.

**Acceptance Criteria:**
- [ ] New file `docs/guide/maggus-plan-skill.md` documenting the skill
- [ ] Explains what the skill is: a Claude Code skill that generates `.maggus/plan_*.md` files interactively through clarifying questions
- [ ] Documents the trigger: how to invoke it (e.g. `/maggus-plan <description>` in Claude Code)
- [ ] Shows the question-and-answer flow with an example interaction
- [ ] Documents the output format: plan file structure, TASK-NNN format, acceptance criteria conventions
- [ ] Explains how the generated plan integrates with `maggus work` (the generated file is immediately usable)
- [ ] Includes at least one complete example showing a feature description → questions → generated plan → running `maggus work`
- [ ] VitePress sidebar in `.vitepress/config.ts` is updated to include the new page under Guide
- [ ] Typecheck/lint passes (VitePress builds without errors)

### TASK-013: Update VitePress site description and homepage

**Description:** As a visitor, I want the homepage and site metadata to reflect that Maggus supports multiple AI agents, not just Claude Code.

**Acceptance Criteria:**
- [ ] `docs/.vitepress/config.ts` description is updated to mention multi-agent support (e.g., "AI-powered task automation CLI that orchestrates AI coding agents to work through implementation plans")
- [ ] `docs/index.md` homepage is updated: hero description and feature bullets mention support for Claude Code and OpenCode
- [ ] No breaking changes to existing navigation or links
- [ ] Typecheck/lint passes (VitePress builds without errors)

## Functional Requirements

- FR-1: The `Agent` interface must define `Run`, `RunOnce`, `Name`, and `Validate` methods
- FR-2: The Claude Code adapter must reproduce the exact current behavior (flags, streaming parsing, process management)
- FR-3: The OpenCode adapter must handle OpenCode's CLI flags (`-p`, `--output-format`, `--model`) and auto-approve behavior (no permissions flag needed)
- FR-4: The OpenCode adapter must parse OpenCode's streaming JSON events and normalize them into the same bubbletea message types
- FR-5: The agent registry must default to `"claude"` when no agent is configured
- FR-6: Model specification must support `provider/model` format as the primary form, with legacy aliases as shortcuts
- FR-7: The `--agent` CLI flag must override the config file's `agent` field
- FR-8: The TUI must display which agent is active in the startup banner
- FR-9: Agent-specific warnings (e.g., `--dangerously-skip-permissions`) must only appear for agents that use them
- FR-10: All documentation must be updated to reflect multi-agent support and include concrete configuration examples

## Non-Goals

- No support for agents beyond Claude Code and OpenCode in this plan (architecture supports it, but only two are implemented)
- No custom agent plugin system — agents are compiled into the binary
- No agent-specific prompt templating (both agents receive the same prompt)
- No per-agent configuration sections in config.yml (e.g., no `claude:` or `opencode:` nested blocks)
- No migration tool for existing config files — legacy format continues to work

## Technical Considerations

- **OpenCode's streaming JSON schema** may differ from Claude Code's. The OpenCode adapter must investigate and handle the actual event format at implementation time. If the format is undocumented or unstable, start with text mode and add streaming later.
- **Model format**: OpenCode uses `provider/model` (e.g., `anthropic/claude-sonnet-4-6`). Claude Code uses bare model IDs (e.g., `claude-sonnet-4-6`). The adapter layer should handle any format translation needed.
- **Process management**: Both adapters share the same OS-level concerns (process groups, signal handling). Consider extracting shared process management into a helper if duplication is significant.
- **Backwards compatibility**: Existing configs with just `model: sonnet` and no `agent` field must continue to work identically to today's behavior.
- **OpenCode CLI stability**: OpenCode's CLI flags and output format may change between versions. Document the tested version in the adapter code.

## Success Metrics

- Running `maggus work` with no config changes behaves identically to the current version (zero regression)
- Running `maggus work --agent opencode` successfully invokes OpenCode and processes at least one task end-to-end
- All existing tests pass, plus new tests for agent registry and config parsing
- Documentation site builds cleanly and covers both agents with working examples
- A user can switch between Claude Code and OpenCode by changing one line in `config.yml`

## Open Questions

- What is the exact streaming JSON event schema for OpenCode's `--output-format stream-json`? The adapter implementation will need to investigate this at development time.
- Should the prompt include agent-specific instructions (e.g., tool names may differ between Claude Code and OpenCode)?
- Should `maggus-plan` skill documentation reference OpenCode's skill system if it has one, or is it Claude Code-only?
