# Plan: Model Selection and Custom Prompt Includes

## Introduction

Extend maggus to support configurable model selection and custom markdown file includes in the prompt. Currently the model is hardcoded to whatever Claude CLI defaults to, and the bootstrap section only reads a fixed set of files (CLAUDE.md, AGENTS.md, PROJECT_CONTEXT.md, TOOLING.md). This plan adds a `.maggus/config.yml` configuration file for persistent settings, with CLI flag overrides for model selection.

## Goals

- Allow users to select which Claude model to use via config file or CLI flag
- Support short model aliases (`sonnet`, `opus`, `haiku`) and full model IDs (`claude-sonnet-4-6`)
- Allow registering additional markdown files in the config that get included in the bootstrap prompt
- Keep backwards compatibility — everything works without a config file using current defaults

## User Stories

### TASK-301: Config File Parsing

**Description:** As a user, I want maggus to read settings from `.maggus/config.yml` so that I can configure model and includes persistently.

**Acceptance Criteria:**
- [ ] New package `internal/config` with a `Config` struct containing `Model string` and `Include []string` fields
- [ ] `config.Load(dir string)` reads `.maggus/config.yml` from the given directory
- [ ] If the file does not exist, return a zero-value Config (no error)
- [ ] If the file exists but is invalid YAML, return a descriptive error
- [ ] Config file uses this format:
  ```yaml
  model: sonnet
  include:
    - ARCHITECTURE.md
    - docs/PATTERNS.md
  ```
- [ ] Unit test: missing file returns empty config
- [ ] Unit test: valid YAML parses correctly
- [ ] Unit test: invalid YAML returns error

### TASK-302: Model Alias Resolution

**Description:** As a user, I want to use short names like `sonnet` or `opus` instead of full model IDs so that configuration is convenient.

**Acceptance Criteria:**
- [ ] New function `config.ResolveModel(input string) string` that maps short aliases to full model IDs
- [ ] Supported aliases: `sonnet` → `claude-sonnet-4-6`, `opus` → `claude-opus-4-6`, `haiku` → `claude-haiku-4-5-20251001`
- [ ] If the input is not a known alias, return it unchanged (allows full model IDs like `claude-sonnet-4-6`)
- [ ] Empty string input returns empty string (means "use CLI default")
- [ ] Unit test: known aliases resolve correctly
- [ ] Unit test: unknown strings pass through unchanged
- [ ] Unit test: empty string returns empty string

### TASK-303: Pass Model to Claude CLI

**Description:** As a user, I want maggus to pass the selected model to the Claude CLI so that my tasks run on the chosen model.

**Acceptance Criteria:**
- [ ] `runner.RunClaude` accepts a model parameter (empty string means no `--model` flag)
- [ ] When model is non-empty, `--model <resolved-model-id>` is added to the claude command arguments
- [ ] When model is empty, no `--model` flag is passed (Claude CLI picks its default)
- [ ] The startup banner displays the resolved model name (or "default" if empty)
- [ ] The run tracker receives the resolved model name instead of hardcoded "claude"

### TASK-304: CLI Flag for Model Override

**Description:** As a user, I want a `--model` flag on the `work` command so that I can override the config file model for a single run.

**Acceptance Criteria:**
- [ ] `maggus work --model opus` overrides the config file model
- [ ] `maggus work` without `--model` uses the config file value
- [ ] If neither CLI flag nor config file specifies a model, no `--model` flag is passed to Claude CLI
- [ ] The flag accepts both short aliases and full model IDs
- [ ] Flag is documented in `maggus work --help`

### TASK-305: Custom Markdown Includes in Prompt

**Description:** As a user, I want to register additional markdown files in the config so that Claude reads them as part of the bootstrap context.

**Acceptance Criteria:**
- [ ] The `include` list from config is passed to `prompt.Build` via `prompt.Options`
- [ ] Each included file is added to the bootstrap section as: "Read the file `<path>` if it exists in the working directory."
- [ ] Paths are relative to the project root (e.g. `ARCHITECTURE.md`, `docs/PATTERNS.md`)
- [ ] The existing hardcoded bootstrap files (CLAUDE.md, AGENTS.md, PROJECT_CONTEXT.md, TOOLING.md) remain unchanged
- [ ] Custom includes appear after the standard bootstrap files
- [ ] If the include list is empty, the bootstrap section is unchanged from current behavior
- [ ] Unit test: empty includes produces standard bootstrap only
- [ ] Unit test: includes adds "Read the file" instructions for each entry

### TASK-306: Wire Config into Work Command

**Description:** As a user, I want the work command to load the config and apply model + includes so that everything works end-to-end.

**Acceptance Criteria:**
- [ ] `work` command loads config via `config.Load(dir)` early in execution
- [ ] CLI `--model` flag overrides `config.Model` if provided
- [ ] Model is resolved via `config.ResolveModel` before passing to runner and run tracker
- [ ] `config.Include` is passed through to prompt options
- [ ] Works correctly with no config file (backwards compatible)
- [ ] Works correctly with config file but no `--model` flag
- [ ] Works correctly with `--model` flag overriding config

## Functional Requirements

- FR-1: maggus must read `.maggus/config.yml` on startup of the `work` command
- FR-2: If `.maggus/config.yml` does not exist, maggus must continue with default behavior
- FR-3: The `model` field in config must accept short aliases (`sonnet`, `opus`, `haiku`) and full model IDs
- FR-4: The `--model` CLI flag must override the config file value
- FR-5: When a model is specified, maggus must pass `--model <id>` to the `claude` CLI invocation
- FR-6: When no model is specified anywhere, maggus must not pass a `--model` flag to `claude`
- FR-7: The `include` field in config must list relative file paths to include in the bootstrap prompt
- FR-8: Included files must be added as read instructions (not inlined content) in the prompt bootstrap section

## Non-Goals

- No interactive config editor or `maggus config` subcommand
- No per-task model override (all tasks in a run use the same model)
- No validation that included files actually exist (Claude handles missing files gracefully)
- No config file for other settings beyond model and includes in this iteration
- No CLI flag for includes (config file only)

## Technical Considerations

- Use `gopkg.in/yaml.v3` for YAML parsing (already an indirect dependency via cobra)
- The config package should be independent — no imports from other maggus packages
- Model alias map should be a simple hardcoded map, easy to extend later
- Claude CLI `--model` flag expects the full model ID (e.g. `claude-sonnet-4-6`)

## Success Metrics

- Running `maggus work --model opus` uses the opus model for all tasks
- Running `maggus work` with a config file picks up model and includes from config
- Running `maggus work` without a config file works exactly as before
- The startup banner shows which model is being used

## Open Questions

- Should the config file support additional settings in the future (e.g. `dangerously-skip-permissions: false`, custom task count default)?
- Should model aliases be kept up to date automatically, or is a hardcoded map sufficient?
