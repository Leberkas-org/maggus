# Plan: Clean Command & Release Notes

## Introduction

Add two new CLI commands to Maggus: `maggus clean` for housekeeping (removing completed plan files and old run data) and `maggus release` for generating release notes. Additionally, integrate "running release notes" into the work loop — similar to how `.maggus/MEMORY.md` is continuously updated, the agent will append rough notes to `.maggus/RELEASE_NOTES.md` (gitignored) during each task iteration. The `maggus release` command then combines those rough notes with git history to produce a polished `RELEASE.md` with both a structured conventional changelog and an AI-generated summary.

## Goals

- Provide a `maggus clean` command that removes completed plan files and run directories associated with completed plans
- Provide a `maggus release` command that generates a `RELEASE.md` covering all changes since the last version tag
- Have the work loop instruct the agent to append release-relevant notes to `.maggus/RELEASE_NOTES.md` after each task
- The generated `RELEASE.md` should contain both a conventional changelog (grouped by type) and a human-friendly AI summary
- Keep `.maggus/RELEASE_NOTES.md` gitignored (ephemeral working data, like MEMORY.md)

## User Stories

### TASK-001: Implement `maggus clean` command
**Description:** As a user, I want to run `maggus clean` to remove completed plan files and their associated run directories so that `.maggus/` doesn't accumulate stale data.

**Acceptance Criteria:**
- [x] New Cobra command `clean` registered in `src/cmd/clean.go`
- [x] Deletes all `_completed.md` plan files from `.maggus/`
- [x] Identifies run directories in `.maggus/runs/` that are associated with completed plans: a run is "completed" if all plans that existed when it started have since been completed (i.e., renamed to `_completed.md`) or if the run's `run.md` contains an `## End` section (meaning it finished)
- [x] Removes only completed run directories; keeps runs that are still in progress (no `## End` section in `run.md`)
- [x] Prints a summary of what was removed (e.g., "Removed 3 completed plans, 5 run directories")
- [x] If nothing to clean, prints "Nothing to clean."
- [x] Has a `--dry-run` flag that shows what would be removed without actually deleting anything
- [x] Unit tests cover: completed plan removal, run directory cleanup, dry-run mode, nothing-to-clean case
- [x] Typecheck/lint passes (`go vet ./...`)

### TASK-002: Add `.maggus/RELEASE_NOTES.md` to gitignore and prompt instructions
**Description:** As a developer, I want the work loop to instruct Claude Code to append release-relevant notes to `.maggus/RELEASE_NOTES.md` after each task so that raw material for release notes accumulates during development.

**Acceptance Criteria:**
- [x] `.maggus/RELEASE_NOTES.md` is added to the gitignore entries in `src/internal/gitignore/gitignore.go`
- [x] The prompt instructions in `src/internal/prompt/prompt.go` gain a new step (step 6) after the MEMORY.md instruction
- [x] The new instruction tells the agent: "Append a short release note entry to `.maggus/RELEASE_NOTES.md` describing user-visible changes made in this task. Use the format: `## TASK-NNN: Title` followed by 1-3 bullet points. Focus on what changed from the user's perspective, not implementation details. If the task has no user-visible changes, skip this step. Do NOT commit this file."
- [x] Existing prompt steps are unchanged (only a new step is added)
- [x] Unit tests verify the new instruction appears in the built prompt
- [x] Typecheck/lint passes (`go vet ./...`)

### TASK-003: Create `release` internal package for changelog generation
**Description:** As a developer, I want a `release` package that can gather git history since the last tag and parse conventional commit messages so that `maggus release` has the data it needs.

**Acceptance Criteria:**
- [x] New package at `src/internal/release/release.go`
- [x] `FindLastTag(dir string) (tag string, err error)` — runs `git describe --tags --abbrev=0` to find the most recent version tag; returns empty string if no tags exist
- [x] `CommitsSinceTag(dir, tag string) ([]Commit, error)` — runs `git log <tag>..HEAD --pretty=format:<format>` (or `git log --pretty=format:<format>` if no tag) and returns parsed commits
- [x] `Commit` struct contains: `Hash`, `Subject`, `Body`, `Type` (feat/fix/chore/etc., parsed from conventional commit prefix), `Scope` (optional, from parenthetical), `IsBreaking` (from `!` suffix or `BREAKING CHANGE:` in body)
- [x] `GroupByType(commits []Commit) map[string][]Commit` — groups commits by their conventional commit type
- [x] `FormatChangelog(groups map[string][]Commit, tag string) string` — formats the grouped commits into a markdown changelog section with headers like `### Features`, `### Bug Fixes`, etc.
- [x] Commits that don't follow conventional commit format are grouped under `### Other Changes`
- [x] Unit tests cover: tag finding, commit parsing, conventional commit type extraction, grouping, formatting
- [x] Typecheck/lint passes (`go vet ./...`)

### TASK-004: Implement `maggus release` command
**Description:** As a user, I want to run `maggus release` to generate a `RELEASE.md` that combines a structured changelog with an AI-generated summary of all changes since the last version tag.

**Acceptance Criteria:**
- [ ] New Cobra command `release` registered in `src/cmd/release.go`
- [ ] Uses the `release` package from TASK-003 to gather commits and generate the conventional changelog section
- [ ] Reads `.maggus/RELEASE_NOTES.md` if it exists (the rough notes accumulated during work iterations)
- [ ] Builds a prompt for Claude Code that includes: the conventional changelog, the rough release notes (if any), and the full diff summary (`git diff <tag>..HEAD --stat`)
- [ ] Invokes Claude Code (via `runner.RunClaude` or a simpler one-shot invocation) with the prompt, asking it to produce: (1) a short highlights/summary section written for end users, and (2) any notable breaking changes or migration notes
- [ ] Writes the final `RELEASE.md` to the repository root with this structure:
  ```
  # Release Notes

  ## Summary
  <AI-generated summary>

  ## Changelog
  <conventional changelog from release package>
  ```
- [ ] Accepts a `--model` flag (same alias resolution as `work`)
- [ ] If no commits since the last tag, prints "No changes since last tag." and exits
- [ ] After writing RELEASE.md, prints the path and a preview of the summary section
- [ ] Unit tests cover: command registration, output format
- [ ] Typecheck/lint passes (`go vet ./...`)

### TASK-005: Clear release notes after `maggus release`
**Description:** As a user, I want `.maggus/RELEASE_NOTES.md` to be cleared after running `maggus release` so that notes don't carry over to the next release cycle.

**Acceptance Criteria:**
- [ ] After `maggus release` successfully writes `RELEASE.md`, the command deletes `.maggus/RELEASE_NOTES.md` (or truncates it to empty)
- [ ] If `.maggus/RELEASE_NOTES.md` doesn't exist, no error is raised
- [ ] A message is printed: "Cleared .maggus/RELEASE_NOTES.md for next release cycle."
- [ ] Typecheck/lint passes (`go vet ./...`)

## Functional Requirements

- FR-1: `maggus clean` must remove all `*_completed.md` files from `.maggus/`
- FR-2: `maggus clean` must only remove run directories that have finished (contain `## End` in `run.md`); in-progress runs must be preserved
- FR-3: `maggus clean --dry-run` must show what would be removed without deleting anything
- FR-4: The work loop prompt must instruct the agent to append user-facing change notes to `.maggus/RELEASE_NOTES.md`
- FR-5: `.maggus/RELEASE_NOTES.md` must be gitignored
- FR-6: `maggus release` must find the last version tag and gather all commits since that tag
- FR-7: `maggus release` must produce a `RELEASE.md` containing both a conventional changelog (grouped by commit type) and an AI-generated summary
- FR-8: `maggus release` must use `.maggus/RELEASE_NOTES.md` as additional context for the AI summary if the file exists
- FR-9: `maggus release` must clear `.maggus/RELEASE_NOTES.md` after successfully writing `RELEASE.md`
- FR-10: Commits not following conventional commit format must appear under "Other Changes" in the changelog
- FR-11: `maggus release` must not create tags or GitHub releases — it only writes the file

## Non-Goals

- No automatic git tagging or GitHub release creation from `maggus release`
- No interactive confirmation prompts for `maggus clean` (use `--dry-run` to preview)
- No scheduled/automatic cleanup — `maggus clean` is always manual
- No cleanup of `.maggus/MEMORY.md` — that file is managed separately
- No support for non-conventional commit messages beyond grouping them as "Other Changes"

## Technical Considerations

- The `release` package should use `git log --pretty=format:` with a parseable separator (e.g., `%x00`) to reliably split fields, since commit subjects can contain special characters
- Conventional commit parsing should handle: `type:`, `type(scope):`, `type!:`, and `type(scope)!:` formats
- The Claude Code invocation for release summary generation should be simpler than the full work loop — no TUI, no streaming needed. Consider using `claude -p <prompt> --output-format text` for a one-shot call, or writing a minimal `RunOnce` helper in the runner package
- Run directory cleanup logic: parse `run.md` looking for `## End` as indicator of a completed run. This is more reliable than checking timestamps since a long-running session could still be active
- The `.maggus/RELEASE_NOTES.md` format should be simple and append-friendly. Each task appends a `## TASK-NNN: Title` section. The file may have duplicate or overlapping entries if a task is retried — the AI summary step can deduplicate
- `RELEASE.md` is written to the repo root (not `.maggus/`) because it's intended to be committed and potentially used as GitHub release body

## Success Metrics

- `maggus clean` removes all completed plans and finished run directories in a single command
- `maggus clean --dry-run` accurately previews what would be removed
- After several `maggus work` sessions, `.maggus/RELEASE_NOTES.md` contains useful per-task change summaries
- `maggus release` produces a readable `RELEASE.md` with both structured changelog and natural-language summary
- The AI summary section adds value beyond just listing commits — it highlights key changes and their impact

## Open Questions

- Should `maggus clean` have a `--all` flag that also removes in-progress runs (for when you know they're stale)?
- Should the conventional changelog include commit hashes as links (requires knowing the GitHub repo URL)?
- Should `maggus release` support a `--since <tag>` flag to override the auto-detected last tag?
