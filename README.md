# Maggus

<img src="docs/avatar.png" alt="Maggus" width="300">

Your best and worst co-worker at the same time. A junior developer that just works and only asks questions when the work itself is unclear.

Give him an implementation plan and he'll grind through the tasks one by one, prompting an AI agent for each.

## How It Works

1. Create an implementation plan using the **maggus-plan** skill (or write one manually)
2. The plan lives in `.maggus/plan_*.md` and contains tasks in this format:

```markdown
### TASK-001: Do the thing
**Description:** As a user, I want the thing so that it works.

**Acceptance Criteria:**
- [ ] First criterion
- [ ] Second criterion
```

3. Run `maggus work` and Maggus will find the next incomplete task, build a focused prompt, and send it to Claude Code
4. A task is complete when all its acceptance criteria are checked (`[x]`)

## Installation

Build from source (requires Go 1.22+):

```bash
cd src
go build -o maggus .
```

Make sure `claude` (Claude Code CLI) is available on your PATH.

## Usage

```bash
# Work on the next 5 tasks (default)
maggus work

# Work on the next 10 tasks
maggus work 10

# Same thing with a flag
maggus work --count 10
maggus work -c 10
```

Maggus processes tasks sequentially, one at a time. After each task it re-reads the plan to pick up any changes the agent made, then moves to the next incomplete task.

## Roadmap

- **Single binary distribution** -- Cross-platform builds for Windows, Mac, and Linux
- **Agent choice** -- Support for AI agents beyond Claude Code
- **Task management service** -- A hosted backend replacing the markdown files, like a Jira board optimized for Maggus to read and for humans to edit, plan, and supervise
- **Status overview** -- A `maggus status` command to show task progress at a glance
- **Auto-update checkboxes** -- Maggus marks tasks as done in the plan file after the agent completes them
