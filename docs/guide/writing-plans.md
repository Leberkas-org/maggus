# Writing Plans

Plans are the core input to Maggus. A plan is a markdown file that describes a set of tasks for Claude Code to complete. This page explains how to write plans that Maggus can process.

## Plan File Location

Plan files live in the `.maggus/` directory at the root of your project. They must follow the naming pattern:

```
.maggus/plan_*.md
```

For example:
- `.maggus/plan_1.md`
- `.maggus/plan_2.md`
- `.maggus/plan_auth_refactor.md`

Maggus scans all files matching this pattern and processes them in order.

## Plan File Structure

A plan file is a standard markdown document with a specific structure. At the top level, it contains an introduction, goals, and a list of user stories (tasks).

### Task Headings

Each task is defined as a level-3 heading with a specific format:

```markdown
### TASK-NNN: Title
```

- `TASK-NNN` is a unique identifier (e.g., `TASK-001`, `TASK-042`)
- The title is a short description of what the task accomplishes

### Description

Below the task heading, add a description explaining the task:

```markdown
**Description:** As a user, I want X so that Y.
```

### Acceptance Criteria

Each task must have acceptance criteria defined as markdown checkboxes:

```markdown
**Acceptance Criteria:**
- [ ] First criterion
- [ ] Second criterion
- [ ] Third criterion
```

Acceptance criteria tell both Maggus and Claude Code exactly what needs to be done. Be specific — each checkbox should describe a concrete, verifiable outcome.

## Task Completion

A task is considered **complete** when all of its acceptance criteria checkboxes are checked:

```markdown
- [x] First criterion
- [x] Second criterion
- [x] Third criterion
```

Maggus checks each task's criteria before selecting it for work. If every checkbox is marked `[x]`, the task is skipped as already done.

## Blocked Tasks

Sometimes a task can't be completed due to external dependencies or missing requirements. You can mark individual criteria as blocked using the `BLOCKED:` prefix:

```markdown
- [ ] BLOCKED: Deploy to production — waiting for staging environment setup
```

When any unchecked criterion contains `BLOCKED:`, Maggus treats the entire task as blocked and skips it. This lets you keep tasks in the plan without Maggus attempting work it can't finish.

Blocked tasks show up in `maggus status` with a distinct indicator, and you can manage them interactively from the status or list TUI (see [CLI Commands](/reference/commands#managing-blocked-tasks)).

::: tip
You can also use the `⚠️ BLOCKED:` prefix variant — Maggus recognizes both forms.
:::

## Completed Plans

When all tasks in a plan file are complete, Maggus automatically renames the file:

```
.maggus/plan_1.md  →  .maggus/plan_1_completed.md
```

Completed plan files are preserved for reference but are no longer processed by Maggus. The `maggus status` command can show completed plans with the `--all` flag.

## Full Example Plan

Here's a complete plan file you can use as a template:

```markdown
# Plan: Add User Authentication

## Introduction

Our application currently has no authentication. This plan adds basic
username/password authentication with JWT tokens.

## Goals

- Add login and registration endpoints
- Protect existing API routes with JWT middleware
- Store user credentials securely with bcrypt

## User Stories

### TASK-001: Create user model and database migration

**Description:** As a developer, I want a user model with secure password
storage so that we have a foundation for authentication.

**Acceptance Criteria:**
- [ ] User model with fields: id, email, password_hash, created_at
- [ ] Database migration creates the users table
- [ ] Password hashing uses bcrypt with a cost factor of 12
- [ ] Unit tests for the user model

### TASK-002: Add registration endpoint

**Description:** As a new user, I want to register an account so that
I can access the application.

**Acceptance Criteria:**
- [ ] POST /api/register accepts email and password
- [ ] Returns 201 with user ID on success
- [ ] Returns 409 if email already exists
- [ ] Input validation rejects weak passwords (min 8 chars)
- [ ] Integration test covers success and duplicate cases

### TASK-003: Add login endpoint

**Description:** As a registered user, I want to log in so that I receive
a JWT token for authenticated requests.

**Acceptance Criteria:**
- [ ] POST /api/login accepts email and password
- [ ] Returns 200 with JWT token on success
- [ ] Returns 401 on invalid credentials
- [ ] JWT token expires after 24 hours
- [ ] Integration test covers success and failure cases

### TASK-004: Add JWT middleware

**Description:** As a developer, I want JWT middleware protecting API
routes so that only authenticated users can access them.

**Acceptance Criteria:**
- [ ] Middleware validates JWT from Authorization header
- [ ] Returns 401 if token is missing or invalid
- [ ] Attaches user ID to request context on success
- [ ] All existing /api/ routes are protected except /register and /login
- [ ] BLOCKED: Load testing with 1000 concurrent users — need load test infrastructure
```

## Generating Plans Automatically

Instead of writing plans by hand, you can use the **maggus-plan** skill in Claude Code to generate them. Simply describe the feature you want and the skill will produce a properly formatted plan file:

```
/maggus-plan Add OAuth2 support with Google and GitHub providers
```

The skill understands the plan format and generates tasks with detailed acceptance criteria, saving you time on the structure so you can focus on reviewing the content.

## Tips for Writing Good Plans

- **Keep tasks small and focused.** Each task should be completable in a single Claude Code session. If a task feels too big, split it into multiple tasks.
- **Be specific in acceptance criteria.** Instead of "add tests", write "unit tests cover the happy path and the error case for invalid input".
- **Order tasks by dependency.** Maggus processes tasks in document order, so put foundational tasks first.
- **Use descriptions to provide context.** The description helps Claude Code understand *why* the task exists, not just *what* to do.
- **Use blocking for external dependencies.** If a task depends on something outside the repo (infrastructure, API keys, human decisions), mark the relevant criterion as blocked.
