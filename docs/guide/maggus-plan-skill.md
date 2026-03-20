# Maggus Skills

Maggus ships with three Claude Code skills that use AI to generate project documents interactively. Instead of writing files by hand, you describe what you want and the skill guides you through clarifying questions before producing the output.

| Skill | Command | Produces |
|---|---|---|
| `/maggus-plan` | `maggus plan <description>` | `.maggus/plan_*.md` — implementation plan |
| `/maggus-vision` | `maggus vision <description>` | `VISION.md` — project vision document |
| `/maggus-architecture` | `maggus architecture <description>` | `ARCHITECTURE.md` — architecture document |

::: info Claude Code Only
Maggus skills are Claude Code plugins. They require Claude Code as your agent. If you're using OpenCode, you'll need to write these files manually.
:::

## Installation

Skills are installed automatically when you run `maggus init` in a project with Claude Code available. Under the hood, Maggus registers the [maggus-skills marketplace](https://github.com/Leberkas-org/maggus-skills) and installs the `maggus@maggus` plugin.

If you need to install manually:

```bash
claude plugin marketplace add https://github.com/Leberkas-org/maggus-skills.git
claude plugin install maggus@maggus
```

## How Skills Work

All three skills follow the same flow:

1. **You** provide a description (via `maggus <command>` or the `/maggus-<skill>` slash command in Claude Code)
2. **The skill** asks 3–5 clarifying questions tailored to your description
3. **You** answer the questions
4. **The skill** generates the file and writes it to your project

You can invoke skills either through the Maggus CLI (`maggus plan`, `maggus vision`, `maggus architecture`) or directly in a Claude Code session using the slash command (`/maggus-plan`, `/maggus-vision`, `/maggus-architecture`).

---

## /maggus-plan

Generates Maggus plan files in the `.maggus/plan_*.md` format with tasks, descriptions, and acceptance criteria — ready to run with `maggus work`.

### Invocation

```bash
# Via Maggus CLI
maggus plan Add OAuth2 login with Google and GitHub providers

# Via Claude Code slash command
/maggus-plan Add OAuth2 login with Google and GitHub providers
```

### Example Interaction

#### Step 1: Invoke the skill

```
You: /maggus-plan Add a REST API for managing blog posts
```

#### Step 2: Answer clarifying questions

```
Skill: I have a few questions before generating the plan:

1. What database are you using? (PostgreSQL, SQLite, etc.)
2. Should the API include authentication/authorization?
3. What fields should a blog post have beyond title and body?
4. Do you need pagination for the list endpoint?
5. Should there be any draft/published status workflow?

You: 1. PostgreSQL
     2. Yes, JWT auth — we already have it from a previous plan
     3. Add tags (array of strings) and a published_at timestamp
     4. Yes, cursor-based pagination
     5. Yes, posts start as draft and can be published
```

#### Step 3: Skill generates the plan

The skill writes a file like `.maggus/plan_3.md`:

```markdown
# Plan: Blog Post REST API

## Introduction

Add a full CRUD REST API for managing blog posts with
draft/publish workflow, tag support, and cursor-based pagination.
Builds on existing JWT authentication.

## Goals

- Create blog post model with tags and draft/published states
- Implement CRUD endpoints with proper authorization
- Add cursor-based pagination to the list endpoint

## User Stories

### TASK-001: Create blog post model and migration

**Description:** As a developer, I want a blog post database model
so that posts can be stored and queried.

**Acceptance Criteria:**
- [ ] Blog post model with fields: id, title, body, tags,
      status (draft/published), published_at, created_at, updated_at
- [ ] PostgreSQL migration creates the blog_posts table
- [ ] Tags stored as a text array column
- [ ] Unit tests for model validation

### TASK-002: Add create and read endpoints
...
```

#### Step 4: Run Maggus

The generated plan is immediately usable:

```bash
maggus work
```

### Output Format

The skill produces standard Maggus plan files. Key formatting conventions:

| Element | Format |
|---|---|
| File name | `.maggus/plan_*.md` (auto-numbered) |
| Task heading | `### TASK-NNN: Title` |
| Description | `**Description:** As a <role>, I want <goal> so that <reason>.` |
| Acceptance criteria | Checkbox list under `**Acceptance Criteria:**` |
| Blocked criteria | `- [ ] BLOCKED: <text> — <reason>` |

For full details on plan structure, see [Writing Plans](./writing-plans).

---

## /maggus-vision

Creates or improves a `VISION.md` file for your project. The vision document captures the project's purpose, target audience, core values, and long-term direction.

### Invocation

```bash
# Via Maggus CLI
maggus vision A CLI tool for orchestrating AI agents

# Via Claude Code slash command
/maggus-vision A CLI tool for orchestrating AI agents
```

The skill reads your existing codebase for context. If a `VISION.md` already exists, the skill improves it rather than starting from scratch.

---

## /maggus-architecture

Creates or improves an `ARCHITECTURE.md` file for your project. The architecture document describes the system's structure, key components, data flow, and design decisions.

### Invocation

```bash
# Via Maggus CLI (full or alias)
maggus architecture A Go CLI with plugin system and streaming output
maggus arch "Review and improve our current architecture"

# Via Claude Code slash command
/maggus-architecture A Go CLI with plugin system and streaming output
```

The skill reads your existing codebase for context. If an `ARCHITECTURE.md` already exists, the skill improves it rather than starting from scratch.

---

## Integration with Maggus

Generated plan files are standard `.maggus/plan_*.md` files — Maggus treats them identically to hand-written plans:

- `maggus work` picks up generated plans automatically
- `maggus status` shows progress across all plans including generated ones
- `maggus list` previews upcoming tasks from the plan

You can mix hand-written and generated plans freely, and edit generated plans before running them.

## Tips

- **Be specific in your description.** The more context you give, the fewer questions the skill needs to ask and the better the output.
- **Review before running.** Skills generate good starting points, but you know your project best. A quick review can catch assumptions that don't match your codebase.
- **Iterate if needed.** If the output isn't quite right, invoke the skill again with a more refined description or edit the file directly.
