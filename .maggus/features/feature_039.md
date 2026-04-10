<!-- maggus-id: 7d8562dc-b176-4912-968c-0e432810c058 -->
# Feature 039: Documentation-Code Gap Fix

## Introduction

A comprehensive documentation cleanup to bring all docs in sync with the current codebase. The `work` command was renamed to `run` (hidden), the foreground TUI was removed in favor of a daemon-based architecture, several CLI commands were replaced by TUI-only screens, flags were removed, and the internal package count grew from 8 to 27. This feature fixes every gap identified in the full gap analysis.

### Architecture Context

- **Vision alignment:** Accurate documentation is essential for developers using Maggus and for AI agents reading CLAUDE.md to understand the codebase
- **Components involved:** Documentation files only — no code changes. Touches `docs/reference/commands.md`, `docs/reference/configuration.md`, `docs/guide/getting-started.md`, `docs/guide/concepts.md`, `docs/guide/maggus-plan-skill.md`, `CLAUDE.md`, `ARCHITECTURE.md`
- **Key constraint:** The `run` command is hidden and internal — user-facing docs should reference `maggus start`/`maggus stop` and the interactive menu

## Goals

- Every CLI command documented in `commands.md` matches a real cobra command with correct flags
- TUI-only screens are not documented as CLI commands
- Configuration reference matches actual config struct fields and CLI flags
- CLAUDE.md internal packages table reflects the current 27-package codebase
- ARCHITECTURE.md skills table correctly describes access via prompt picker, not CLI
- Getting-started guide describes the current daemon-based workflow

## User Stories

### TASK-039-001: Rewrite commands.md — remove non-existent commands
**Description:** As a user reading the CLI reference, I want every documented command to actually exist so I don't try to run commands that don't work.

**Token Estimate:** ~40k tokens
**Predecessors:** none
**Successors:** none
**Parallel:** yes — can run alongside all other tasks

**Acceptance Criteria:**
- [x] `maggus ignore` and `maggus unignore` sections are removed entirely (these commands don't exist in code)
- [x] `maggus status` section is removed (TUI-only screen, not a cobra command — covered in `tui.md`)
- [x] `maggus config` section is removed (TUI-only screen — covered in `tui.md`)
- [x] `maggus repos` section is removed (TUI-only screen — covered in `tui.md`)
- [x] `maggus plan`, `maggus vision`, `maggus architecture` sections are removed (not cobra commands — they're prompt picker skills launched from the interactive menu; covered in `maggus-plan-skill.md`)
- [x] `maggus stop` section includes the `--all` flag with description "stop daemons in all registered repositories"
- [x] No references to commands that don't exist as cobra commands remain in the file
- [x] The intro paragraph is updated to list the actual command set: `start`, `stop`, `approve`, `unapprove`, `list`, `clean`, `release`, `init`, `update`

### TASK-039-002: Rewrite commands.md — fix list command documentation
**Description:** As a user, I want the `list` command docs to match the actual implementation so the documented flags and output format are correct.

**Token Estimate:** ~15k tokens
**Predecessors:** none
**Successors:** none
**Parallel:** yes — can run alongside all other tasks

**Acceptance Criteria:**
- [x] The `maggus list` section documents: `Use: "list"`, no flags, `cobra.NoArgs`
- [x] The description matches code: "List all active features and bugs as tab-separated lines"
- [x] Output format documented as tab-separated: `filename\tid\ttitle\tapproved`
- [x] Old flags (`--count`, `-c`, `--all`, `--plain`, positional `[N]`) are all removed
- [x] Old TUI mode description and keyboard shortcuts are removed
- [x] Example output shows actual tab-separated format

### TASK-039-003: Rewrite configuration.md — remove stale flags and fix fields
**Description:** As a user reading the configuration reference, I want all documented CLI flags and config fields to match what actually exists in code.

**Token Estimate:** ~35k tokens
**Predecessors:** none
**Successors:** none
**Parallel:** yes — can run alongside all other tasks

**Acceptance Criteria:**
- [ ] The `--worktree` / `--no-worktree` section is removed entirely (flag no longer exists on `run` command)
- [ ] The `--no-bootstrap` section is removed entirely (flag no longer exists on `run` command)
- [ ] The CLI Flags section only documents flags that exist: `--model` (on `start`, `release`), `--agent` (on `start`), `--all` (on `start`, `stop`)
- [ ] Model alias table is fixed: aliases resolve to bare model IDs (`claude-sonnet-4-6`), not `provider/model` format (`anthropic/claude-sonnet-4-6`)
- [ ] `discord_presence` field is added to the Global Config section with type `bool`, default `false`, description about Discord Rich Presence
- [ ] `auto_start_disabled` field is documented in the Repository Registry section (per-repo field, default `false`, prevents auto-start when true)
- [ ] The example config blocks in the "Full Example" section use `maggus start` not `maggus work`
- [ ] No references to `maggus work` remain in the file

### TASK-039-004: Rewrite CLAUDE.md internal packages table
**Description:** As a developer or AI agent reading CLAUDE.md, I want the internal packages table to reflect the actual codebase so I know where to find functionality.

**Token Estimate:** ~25k tokens
**Predecessors:** none
**Successors:** none
**Parallel:** yes — can run alongside all other tasks

**Acceptance Criteria:**
- [ ] `runner` package is removed from the table (no longer exists)
- [ ] `runtracker` package is removed from the table (replaced by `runlog`)
- [ ] The following ~15 important packages are listed with accurate descriptions: `parser`, `prompt`, `agent`, `config`, `globalconfig`, `approval`, `runlog`, `gitbranch`, `gitcommit`, `gitignore`, `gitsync`, `stores`, `hooks`, `discord`, `filewatcher`, `tui`, `updater`
- [ ] Minor helper packages (`fingerprint`, `notify`, `claude2x`, `gitutil`, `sesslock`, `session`, `resolver`, `capabilities`, `usage`, `release`) are NOT listed individually — they are too small/internal
- [ ] Each listed package has an accurate one-line purpose description matching current code
- [ ] The table is sorted logically (e.g. by domain: parsing, prompt, agent, config, git, runtime, tui)

### TASK-039-005: Fix ARCHITECTURE.md skills table
**Description:** As a developer reading ARCHITECTURE.md, I want the skills table to correctly describe how skills are accessed so I don't look for non-existent CLI entry points.

**Token Estimate:** ~10k tokens
**Predecessors:** none
**Successors:** none
**Parallel:** yes — can run alongside all other tasks

**Acceptance Criteria:**
- [ ] The Skills System table column "CLI Entry Point" is renamed to "Access" or "Invocation"
- [ ] Each skill row says "Prompt picker → `/maggus-plan`" (or similar) instead of `maggus plan`
- [ ] The description paragraph below the table mentions that skills are launched from the interactive menu's prompt picker, not as standalone CLI commands
- [ ] No references to `maggus plan`, `maggus bugreport`, `maggus vision`, or `maggus architecture` as CLI commands remain

### TASK-039-006: Update getting-started.md
**Description:** As a new user, I want the getting-started guide to describe the actual current workflow so I can follow along successfully.

**Token Estimate:** ~20k tokens
**Predecessors:** none
**Successors:** none
**Parallel:** yes — can run alongside all other tasks

**Acceptance Criteria:**
- [ ] The sample startup output section is replaced with a description of the interactive menu (what the user sees when running `maggus`)
- [ ] The flow description mentions: launch `maggus` → interactive menu → approve features → start daemon (or use `maggus start`)
- [ ] No old TUI spinner/progress bar output remains
- [ ] The "Running Maggus" section accurately describes the daemon-based workflow
- [ ] The "Next Steps" links are accurate (no references to removed features)

### TASK-039-007: Fix clean command description and maggus-plan-skill.md
**Description:** As a user, I want minor doc inaccuracies fixed so everything is consistent.

**Token Estimate:** ~15k tokens
**Predecessors:** none
**Successors:** none
**Parallel:** yes — can run alongside all other tasks

**Acceptance Criteria:**
- [ ] `commands.md` clean section: description matches code — "Remove completed feature and bug files" (removes `_completed.md` files only, no run directory cleanup)
- [ ] `commands.md` clean section: long description says "Removes all _completed.md files from .maggus/features/ and .maggus/bugs/" — no mention of run directories with `## End` section
- [ ] `maggus-plan-skill.md`: the "Integration with Maggus" section at the bottom uses `maggus start` not `maggus work`, and describes skills as prompt picker items not CLI commands
- [ ] `concepts.md`: verify no remaining references to old TUI (tabs, keyboard shortcuts, summary screen) — these were already removed in the earlier session but verify completeness

## Task Dependency Graph

```
TASK-039-001 (commands.md: remove non-existent)
TASK-039-002 (commands.md: fix list)
TASK-039-003 (configuration.md: flags & fields)
TASK-039-004 (CLAUDE.md: packages table)
TASK-039-005 (ARCHITECTURE.md: skills table)
TASK-039-006 (getting-started.md)
TASK-039-007 (clean desc & skill docs)

All tasks are independent — no dependencies.
```

| Task | Estimate | Predecessors | Parallel | Model |
|------|----------|--------------|----------|-------|
| TASK-039-001 | ~40k | none | yes | — |
| TASK-039-002 | ~15k | none | yes | — |
| TASK-039-003 | ~35k | none | yes | — |
| TASK-039-004 | ~25k | none | yes | — |
| TASK-039-005 | ~10k | none | yes | — |
| TASK-039-006 | ~20k | none | yes | — |
| TASK-039-007 | ~15k | none | yes | — |

**Total estimated tokens:** ~160k

## Functional Requirements

- FR-1: Every command in `commands.md` must correspond to a registered cobra command in `src/cmd/`
- FR-2: Every flag documented for a command must exist in its cobra flag set
- FR-3: Every config field in `configuration.md` must exist in the `Config` or `Settings` struct
- FR-4: The CLAUDE.md packages table must only list packages that exist in `src/internal/`
- FR-5: No documentation file may reference `maggus work` as a user-facing command
- FR-6: No documentation file may reference `maggus plan`, `maggus vision`, `maggus architecture`, or `maggus bugreport` as standalone CLI commands
- FR-7: The `run` command should only be mentioned as a hidden/internal command used by the daemon

## Non-Goals

- No code changes — this is documentation only
- No changes to completed feature files (`_completed.md`) or release notes (historical records)
- No changes to `tui.md` (already updated in feature 035)
- No new documentation pages
- Documenting the metrics system (`~/.maggus/metrics.yml`) — internal-only

## Technical Considerations

- The `run` command is hidden (`Hidden: true`) but still registered — it's the internal entry point used by `maggus start`. Docs should mention it exists but not encourage direct use.
- The prompt picker (accessed via "prompt" menu item) is how skills are launched. It offers: "open console", `/maggus-plan`, `/maggus-vision`, `/maggus-architecture`, `/maggus-bugreport`, `/bryan-plan`, `/bryan-bugreport`.
- The `list` command outputs raw tab-separated data — it's designed for scripting, not human reading. The TUI status view is the human-facing alternative.

## Success Metrics

- A new user reading the docs can successfully use every documented command without errors
- Running `maggus <command> --help` for every documented command produces matching flag descriptions
- No broken references to removed commands or flags
- CLAUDE.md gives an AI agent an accurate mental model of the codebase

## Open Questions

None — all questions resolved.
