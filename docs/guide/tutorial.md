# Tutorial: Your First Project

This step-by-step tutorial walks you through a complete Maggus workflow — from an empty repository to automated tasks, status checks, and handling blocked tasks.

## What We'll Build

A tiny Go project with a greeting function and a test. Maggus will write the code for us by working through a plan with three tasks:

1. Initialize a Go module
2. Create a greeting function
3. Add a test (blocked until the function exists)

## Step 1: Create a Project

Start with a fresh Git repository:

```bash
mkdir greeting-demo && cd greeting-demo
git init
git commit --allow-empty -m "Initial commit"
```

Initialize Maggus:

```bash
maggus init
```

```
✓ Created .maggus/
✓ Created .maggus/config.yml
✓ Updated .gitignore
```

Commit the scaffolding so Maggus has a clean working tree:

```bash
git add -A && git commit -m "Initialize project with maggus"
```

## Step 2: Write a Plan

Create `.maggus/plan_1.md` with the following content:

````markdown
# Plan: Greeting Library

## Introduction

Build a small Go package with a greeting function and tests.

## Goals

- Create a working Go module with a tested public API

## User Stories

### TASK-001: Initialize Go module

**Description:** Set up the Go module so the project can be built.

**Acceptance Criteria:**
- [ ] `go.mod` exists with module path `greeting-demo`
- [ ] `go build ./...` succeeds

### TASK-002: Add greeting function

**Description:** Create a `Greet` function that returns a personalized greeting string.

**Acceptance Criteria:**
- [ ] File `greet.go` exists in the root package
- [ ] `Greet(name string) string` is exported
- [ ] `Greet("World")` returns `"Hello, World!"`

### TASK-003: Add greeting tests

**Description:** Write tests for the `Greet` function.

**Acceptance Criteria:**
- [ ] BLOCKED: Requires TASK-002 to be completed first
- [ ] File `greet_test.go` exists
- [ ] Tests cover at least: empty string, normal name, name with spaces
- [ ] `go test ./...` passes
````

Key things to notice:

- Each task uses the `### TASK-NNN: Title` heading format
- Acceptance criteria are checkboxes (`- [ ]`) that Maggus checks off when done
- **TASK-003** has a `BLOCKED:` criterion — Maggus will skip it until you resolve the blocker

Commit the plan:

```bash
git add .maggus/plan_1.md && git commit -m "Add greeting plan"
```

## Step 3: Check Status

Before running, preview what Maggus sees:

```bash
maggus status
```

```
Greeting Library
[░░░░░░░░░░░░░░░░░░░░] 0/3 tasks · 2 pending · 1 blocked

→ TASK-001  Initialize Go module
  TASK-002  Add greeting function
⊘ TASK-003  Add greeting tests
```

The arrow (`→`) marks the next task Maggus will pick up. The `⊘` icon shows TASK-003 is blocked.

You can also preview the task queue:

```bash
maggus list
```

```
All incomplete tasks (3)

→ TASK-001  Initialize Go module
  TASK-002  Add greeting function
⊘ TASK-003  Add greeting tests
```

## Step 4: Run Maggus

Start the work loop:

```bash
maggus work
```

Maggus opens a full-screen TUI and starts working:

```
Maggus v1.0.0                            a1b2c3
[██████░░░░░░░░░░░░░░] 1/3 Tasks

  TASK-001: Initialize Go module

  ◐ Running: go mod init greeting-demo
  ──────────────────────────────────
  │ Write    go.mod
  │ Bash     go mod init greeting-demo
  ▶ Bash     go build ./...
  ──────────────────────────────────
  Tokens: 2.1k in / 0.8k out
  Elapsed: 12s
```

Maggus works through tasks one by one:

1. **TASK-001** — creates `go.mod`, verifies `go build` passes, commits
2. **TASK-002** — creates `greet.go` with the `Greet` function, commits
3. **TASK-003** — skipped! The `BLOCKED:` criterion tells Maggus this task isn't ready

When it finishes the workable tasks, you'll see a summary:

```
Run Summary
──────────────────────────────────────
  Run ID:     20260319-143022
  Branch:     feature/maggustask-001
  Agent:      claude
  Elapsed:    1m 42s

  Tasks:      2/3 completed

  Commits:
    a1b2c3f  feat: initialize Go module (TASK-001)
    d4e5f6a  feat: add Greet function (TASK-002)

  Remaining:
    ⊘ TASK-003  Add greeting tests (blocked)
──────────────────────────────────────
```

Press any key to exit.

## Step 5: Inspect the Results

Check what Maggus created:

```bash
git log --oneline
```

```
d4e5f6a feat: add Greet function (TASK-002)
a1b2c3f feat: initialize Go module (TASK-001)
b0c1d2e Add greeting plan
f0a1b2c Initialize project with maggus
```

Verify the code works:

```bash
go test ./...
```

This will fail — we don't have tests yet (TASK-003 was blocked).

Check the updated plan status:

```bash
maggus status
```

```
Greeting Library
[████████████░░░░░░░░] 2/3 tasks · 0 pending · 1 blocked

  ✓ TASK-001  Initialize Go module
  ✓ TASK-002  Add greeting function
⊘ TASK-003  Add greeting tests
```

## Step 6: Resolve the Blocked Task

TASK-003 is blocked because its first criterion says `BLOCKED: Requires TASK-002 to be completed first`. Now that TASK-002 is done, you can resolve this.

**Option A: Edit the plan file directly.**

Open `.maggus/plan_1.md` and change the blocked criterion from:

```markdown
- [ ] BLOCKED: Requires TASK-002 to be completed first
```

to:

```markdown
- [x] Requires TASK-002 to be completed first
```

This marks the blocker as resolved (checked off) so Maggus treats the task as workable.

**Option B: Use the status TUI.**

Run `maggus status`, select the blocked task, and press `Enter` to open the detail view. Press `Tab` to enter criteria mode, navigate to the blocked criterion, and press `Enter` to choose **Resolve** (removes the criterion) or **Unblock** (removes the `BLOCKED:` prefix).

After resolving, run Maggus again:

```bash
maggus work
```

Maggus picks up TASK-003, writes the tests, and commits. Run `maggus status` one more time:

```
Greeting Library
[████████████████████] 3/3 tasks · 0 pending · 0 blocked

  ✓ TASK-001  Initialize Go module
  ✓ TASK-002  Add greeting function
  ✓ TASK-003  Add greeting tests
```

All done! The plan file is automatically renamed to `plan_1_completed.md`.

## What You've Learned

- **Plans** define tasks with acceptance criteria that Maggus works through sequentially
- **`maggus work`** runs the AI agent in a loop, committing after each task
- **`maggus status`** shows progress at a glance
- **Blocked tasks** are skipped automatically — resolve the blocker and re-run
- **Completed plans** are renamed to `_completed.md` so they don't clutter future runs

## Next Steps

- [Writing Plans](./writing-plans) — full format reference, multi-plan strategies, and tips
- [Concepts](./concepts) — how the work loop, git branches, and run logs work under the hood
- [CLI Commands](/reference/commands) — all commands and flags
- [Configuration](/reference/configuration) — customize agent, model, and project includes
