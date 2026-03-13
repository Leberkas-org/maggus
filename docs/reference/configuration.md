# Configuration

Maggus is configured through a YAML file and CLI flags. This page documents all available options.

## Config File

Maggus reads its configuration from `.maggus/config.yml` in your project root. If the file does not exist, Maggus uses default settings with no errors.

```yaml
# .maggus/config.yml
agent: claude
model: anthropic/claude-sonnet-4-6
include:
  - ARCHITECTURE.md
  - docs/PATTERNS.md
```

### `agent`

Selects the AI backend to use. Maggus supports multiple agents through a common interface.

```yaml
agent: opencode
```

| Value | Description |
|-------|-------------|
| `claude` | [Claude Code](https://docs.anthropic.com/en/docs/claude-code) — Anthropic's coding agent (default) |
| `opencode` | [OpenCode](https://opencode.ai) — Open-source coding agent supporting multiple providers |

If omitted, defaults to `claude`.

### `model`

Sets the model to use for all tasks. The canonical format is `provider/model`:

```yaml
model: anthropic/claude-opus-4-6
```

**Provider/model examples:**

| Provider | Example |
|----------|---------|
| Anthropic | `anthropic/claude-sonnet-4-6` |
| Anthropic | `anthropic/claude-opus-4-6` |
| OpenAI | `openai/gpt-4.1` |
| Google | `google/gemini-2.5-pro` |

The provider prefix tells Maggus (and the agent) which API to use. When using Claude Code as the agent, only the model ID portion is passed to the CLI (e.g. `claude-sonnet-4-6`). When using OpenCode, the full `provider/model` string is used as-is since OpenCode natively supports that format.

::: tip Convenience Aliases
For common Anthropic models, you can use short aliases instead of the full `provider/model` string. These are resolved automatically.

| Alias    | Resolves To                          |
|----------|--------------------------------------|
| `sonnet` | `anthropic/claude-sonnet-4-6`        |
| `opus`   | `anthropic/claude-opus-4-6`          |
| `haiku`  | `anthropic/claude-haiku-4-5-20251001`|
:::

Bare model IDs without a provider prefix (e.g. `claude-sonnet-4-6`) still work for backwards compatibility and are passed through unchanged.

If omitted, Maggus does not pass a model flag to the agent, which uses the agent's default model.

### `include`

A list of additional files to include in the prompt context. Paths are relative to the project root.

```yaml
include:
  - ARCHITECTURE.md
  - docs/API_SPEC.md
  - .cursor/rules.md
```

These files are read at the start of each task and appended to the bootstrap context section of the prompt. This is useful for feeding project-specific documentation, architecture decisions, or coding guidelines into every task.

If an included file does not exist, Maggus prints a warning to stderr and skips it — the task still runs with the remaining valid includes.

## CLI Flags

### `--model`

The `--model` flag on `maggus work` overrides the `model` field from the config file.

```bash
# Use opus for this run, regardless of config.yml
maggus work --model opus

# Use a full provider/model ID
maggus work --model anthropic/claude-sonnet-4-6
```

The same alias resolution applies: short aliases are expanded to full `provider/model` IDs.

**Precedence:** `--model` flag > `config.yml` model > agent default

### `--agent`

The `--agent` flag on `maggus work` overrides the `agent` field from the config file.

```bash
# Use OpenCode for this run
maggus work --agent opencode

# Combine with model override
maggus work --agent opencode --model openai/gpt-4.1
```

**Precedence:** `--agent` flag > `config.yml` agent > `claude` default

The agent's CLI tool must be installed and available on your PATH. Maggus validates this before starting the work loop and exits with a clear error if the tool is not found.

## Bootstrap Context Files

Before each task, Maggus automatically reads the following files from the project root (if they exist) and includes them in the prompt:

| File                  | Purpose                                                    |
|-----------------------|------------------------------------------------------------|
| `CLAUDE.md`           | Project conventions and instructions for the agent         |
| `AGENTS.md`           | Agent-specific instructions and behavioral guidelines      |
| `PROJECT_CONTEXT.md`  | High-level project context, goals, and architecture        |
| `TOOLING.md`          | Tooling setup, build commands, and environment details     |
| `.maggus/MEMORY.md`   | Portable project memory (gitignored, synced separately)    |

These bootstrap files are read first, before any `include` files from the config. Together they give the agent the full context it needs to work on tasks effectively.

### `--no-bootstrap` Flag

Use the `--no-bootstrap` flag on `maggus work` to skip reading all bootstrap context files:

```bash
maggus work --no-bootstrap
```

When this flag is set:
- None of the bootstrap files above are read
- Custom `include` files from config are also skipped
- Only the task details and run metadata are included in the prompt

This is useful for debugging or when you want minimal prompt context.

## Full Example

### Claude Code Setup

A typical configuration using Claude Code (the default agent):

```yaml
# .maggus/config.yml — Claude Code
agent: claude
model: sonnet
include:
  - ARCHITECTURE.md
  - docs/coding-guidelines.md
```

```bash
# Override model for this run
maggus work --model opus --count 3

# Skip bootstrap context
maggus work --no-bootstrap
```

### OpenCode Setup

A configuration using OpenCode with a non-Anthropic model:

```yaml
# .maggus/config.yml — OpenCode
agent: opencode
model: openai/gpt-4.1
include:
  - ARCHITECTURE.md
  - docs/coding-guidelines.md
```

```bash
# Run with OpenCode and a different model
maggus work --model google/gemini-2.5-pro

# Override agent on the CLI (e.g. try OpenCode without changing config)
maggus work --agent opencode --model anthropic/claude-sonnet-4-6
```

::: info Agent Differences
- **Claude Code** uses `--dangerously-skip-permissions` for non-interactive mode. Maggus shows a warning when this flag is active.
- **OpenCode** auto-approves in non-interactive mode — no permissions flag is needed.
- **Model format**: Claude Code receives just the model ID (e.g. `claude-sonnet-4-6`). OpenCode receives the full `provider/model` string.
:::
