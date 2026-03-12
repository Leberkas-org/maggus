# Maggus

<img src="docs/avatar.png" alt="Maggus" width="300">

Your best and worst co-worker at the same time. Give Maggus an implementation plan and it'll grind through the tasks one by one — prompting an AI agent for each, committing the results, and moving on to the next. It's a Go CLI tool that orchestrates [Claude Code](https://claude.ai/code) to turn markdown plans into working code, autonomously.

> **📖 Read the full documentation at [leberkas-org.github.io/maggus](https://leberkas-org.github.io/maggus/)**

> **🛒 Skills Marketplace:** Looking for ready-made skills? Check out the [Maggus Skills Marketplace](https://github.com/Leberkas-org/maggus-skills.git).

## Quick Install

### Pre-built binaries

Download the latest binary for your platform from the [GitHub Releases](https://github.com/leberkas-org/maggus/releases) page.

### Build from source

Requires Go 1.22+:

```bash
cd src
go build -o maggus .
```

Make sure `claude` (Claude Code CLI) is available on your PATH.

## Quick Start

```bash
# Create a plan
mkdir -p .maggus
cat > .maggus/plan_1.md << 'EOF'
# My Plan

### TASK-001: Say hello
**Description:** Create a hello-world script.

**Acceptance Criteria:**
- [ ] A file `hello.sh` exists with a greeting
EOF

# Let Maggus work through it
maggus work
```

For detailed usage, configuration, and guides, visit the [documentation site](https://leberkas-org.github.io/maggus/).

## Roadmap

- **Agent choice** — Support for AI agents beyond Claude Code
- **Task management service** — A hosted backend replacing the markdown files, like a Jira board optimized for Maggus to read and for humans to edit, plan, and supervise
