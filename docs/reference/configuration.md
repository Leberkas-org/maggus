# Configuration

Maggus is configured through a YAML file and CLI flags. This page documents all available options.

## Config File

Maggus reads its configuration from `.maggus/config.yml` in your project root. If the file does not exist, Maggus uses default settings with no errors.

```yaml
# .maggus/config.yml
model: sonnet
include:
  - ARCHITECTURE.md
  - docs/PATTERNS.md
```

### `model`

Sets the Claude model to use for all tasks. You can use a short alias or a full model ID.

```yaml
model: opus
```

**Model Alias Table:**

| Alias    | Full Model ID                   |
|----------|---------------------------------|
| `sonnet` | `claude-sonnet-4-6`             |
| `opus`   | `claude-opus-4-6`               |
| `haiku`  | `claude-haiku-4-5-20251001`     |

If the value is not a known alias, it is passed through unchanged. This lets you use any valid model ID directly:

```yaml
model: claude-sonnet-4-6
```

If omitted, Maggus does not pass a model flag to Claude Code, which uses Claude's default model.

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

## CLI Flag: `--model`

The `--model` flag on `maggus work` overrides the `model` field from the config file.

```bash
# Use opus for this run, regardless of config.yml
maggus work --model opus

# Use a full model ID
maggus work --model claude-sonnet-4-6
```

The same alias resolution applies: short aliases are expanded to full model IDs.

**Precedence:** `--model` flag > `config.yml` model > Claude Code default

## Bootstrap Context Files

Before each task, Maggus automatically reads the following files from the project root (if they exist) and includes them in the prompt:

| File                  | Purpose                                                    |
|-----------------------|------------------------------------------------------------|
| `CLAUDE.md`           | Project conventions and instructions for Claude Code       |
| `AGENTS.md`           | Agent-specific instructions and behavioral guidelines      |
| `PROJECT_CONTEXT.md`  | High-level project context, goals, and architecture        |
| `TOOLING.md`          | Tooling setup, build commands, and environment details     |
| `.maggus/MEMORY.md`   | Portable project memory (gitignored, synced separately)    |

These bootstrap files are read first, before any `include` files from the config. Together they give Claude Code the full context it needs to work on tasks effectively.

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

Here is a complete `.maggus/config.yml` with all available options:

```yaml
# Model to use — alias or full ID
model: sonnet

# Additional files to include in every task prompt
include:
  - ARCHITECTURE.md
  - docs/coding-guidelines.md
```

Combined with CLI flags:

```bash
# Override model and limit to 3 tasks
maggus work --model opus --count 3

# Skip bootstrap context
maggus work --no-bootstrap

# Preview upcoming tasks (not affected by config)
maggus list
maggus status
```
