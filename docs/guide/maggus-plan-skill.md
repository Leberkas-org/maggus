# The maggus-plan Skill

The **maggus-plan** skill is a Claude Code skill that generates Maggus plan files interactively. Instead of writing `.maggus/plan_*.md` files by hand, you describe the feature you want and the skill asks clarifying questions before producing a well-structured plan file — ready to run with `maggus work`.

## What It Does

When you invoke the skill, it:

1. Reads your feature description
2. Asks clarifying questions to understand scope, constraints, and preferences
3. Generates a complete plan file in the `.maggus/plan_*.md` format with tasks, descriptions, and acceptance criteria

The generated plan follows the same conventions described in [Writing Plans](./writing-plans) — `TASK-NNN` headings, checkbox acceptance criteria, and proper markdown structure.

## How to Invoke It

In a Claude Code session, use the slash command:

```
/maggus-plan <feature description>
```

For example:

```
/maggus-plan Add OAuth2 login with Google and GitHub providers
```

You can also trigger it with natural language phrases like "create a plan for..." or "write a plan for..." — the skill activates when it detects planning intent.

## The Question-and-Answer Flow

The skill doesn't generate a plan immediately. It first asks targeted questions to fill in gaps and avoid assumptions. A typical flow looks like:

1. **You** provide a feature description
2. **The skill** asks 3–5 clarifying questions (scope, tech choices, constraints, etc.)
3. **You** answer the questions
4. **The skill** generates the plan file and writes it to `.maggus/`

Questions are tailored to your description. For a backend feature the skill might ask about database schema and error handling; for a frontend feature it might ask about component structure and accessibility.

## Example Interaction

Here's a full example showing a feature request through to running the generated plan.

### Step 1: Invoke the skill

```
You: /maggus-plan Add a REST API for managing blog posts
```

### Step 2: Answer clarifying questions

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

### Step 3: Skill generates the plan

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

**Description:** As an API consumer, I want to create and retrieve
blog posts.

**Acceptance Criteria:**
- [ ] POST /api/posts creates a new draft post (requires auth)
- [ ] GET /api/posts/:id returns a single post
- [ ] GET /api/posts returns paginated list with cursor-based pagination
- [ ] Published posts are visible to all; drafts only to the author
- [ ] Integration tests cover auth and pagination

### TASK-003: Add update, publish, and delete endpoints

**Description:** As an author, I want to edit, publish, and delete
my blog posts.

**Acceptance Criteria:**
- [ ] PUT /api/posts/:id updates post fields (requires auth, author only)
- [ ] POST /api/posts/:id/publish sets status to published and
      published_at to now
- [ ] DELETE /api/posts/:id soft-deletes the post (requires auth,
      author only)
- [ ] Returns 403 if the authenticated user is not the author
- [ ] Integration tests cover authorization checks
```

### Step 4: Run Maggus

The generated plan is immediately usable:

```bash
maggus work
```

Maggus picks up `TASK-001` from the new plan and starts working through the tasks in order. No manual editing of the plan file is needed — though you're free to review and tweak it before running.

## Output Format

The skill produces standard Maggus plan files. Key formatting conventions:

| Element | Format |
|---|---|
| File name | `.maggus/plan_*.md` (auto-numbered) |
| Task heading | `### TASK-NNN: Title` |
| Description | `**Description:** As a <role>, I want <goal> so that <reason>.` |
| Acceptance criteria | Checkbox list under `**Acceptance Criteria:**` |
| Blocked criteria | `- [ ] BLOCKED: <text> — <reason>` |

For full details on plan structure, see [Writing Plans](./writing-plans).

## Integration with maggus work

The generated plan file is a standard `.maggus/plan_*.md` file — Maggus treats it identically to a hand-written plan. There's nothing special about skill-generated plans:

- `maggus work` picks up the plan automatically
- `maggus status` shows progress across all plans including generated ones
- `maggus list` previews upcoming tasks from the plan
- Tasks complete, block, and rename the same way as any other plan

This means you can mix hand-written and generated plans freely. You can also edit a generated plan before running it — add tasks, remove tasks, reorder, or refine acceptance criteria.

## Tips

- **Be specific in your description.** The more context you give, the fewer questions the skill needs to ask and the better the generated plan will be.
- **Review before running.** The skill generates good starting plans, but you know your project best. A quick review can catch assumptions that don't match your codebase.
- **Iterate if needed.** If the generated plan isn't quite right, invoke the skill again with a more refined description or edit the plan file directly.
- **Combine with existing plans.** Generated plans coexist with hand-written ones. Maggus processes all active plans in file order.

::: info Claude Code Only
The `maggus-plan` skill is a Claude Code skill. It requires Claude Code as your coding agent. If you're using OpenCode, you'll need to write plan files manually following the format described in [Writing Plans](./writing-plans).
:::
