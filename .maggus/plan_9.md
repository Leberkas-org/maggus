# Plan: Git Worktree Support

## Introduction

Add built-in git worktree support to Maggus so that each `maggus work` session runs inside an isolated worktree. This enables two key benefits: **workspace isolation** (the main working directory stays untouched while Maggus works) and **parallel execution** (multiple `maggus work --worktree` sessions can run simultaneously in separate terminals, each in its own worktree working on different tasks).

Worktrees are session-based: one worktree is created per `maggus work` run, used for all iterations in that run, and cleaned up when the run finishes. Each worktree gets its own feature branch which is pushed to the remote; the user merges via PRs.

## Goals

- Allow `maggus work` to operate inside a git worktree so the main working directory is not modified
- Enable concurrent `maggus work --worktree` sessions that each grab different tasks without conflicts
- Provide a config option (`worktree: true`) and CLI flags (`--worktree` / `--no-worktree`) to control the behavior
- Store worktrees in `.maggus-work/` inside the repository root
- Clean up worktrees automatically when a run completes (success or interrupt)
- Prevent two concurrent sessions from picking the same task via a file-based lock

## User Stories

### TASK-001: Create `worktree` internal package
**Description:** As a developer, I want a `worktree` package that wraps `git worktree` commands so that Maggus can create, list, and remove worktrees programmatically.

**Acceptance Criteria:**
- [x] New package at `src/internal/worktree/worktree.go`
- [x] `Create(repoDir, worktreeDir, branch string) error` — runs `git worktree add <worktreeDir> -b <branch>` from `repoDir`
- [x] `Remove(repoDir, worktreeDir string) error` — runs `git worktree remove <worktreeDir> --force` from `repoDir`
- [x] `List(repoDir string) ([]string, error)` — runs `git worktree list --porcelain` and returns worktree paths
- [x] All functions use `repoDir` as the command working directory (not the worktree)
- [x] Errors from git are wrapped with descriptive context
- [x] Unit tests cover success and error paths (use `exec.Command` mocking or test in a real temp git repo)

### TASK-002: Add task locking for parallel execution
**Description:** As a user running multiple Maggus sessions in parallel, I want a locking mechanism so that two sessions never pick the same task.

**Acceptance Criteria:**
- [x] New package at `src/internal/tasklock/tasklock.go`
- [x] Lock files are stored in `.maggus/locks/` directory (e.g., `.maggus/locks/TASK-001.lock`)
- [x] `Acquire(dir, taskID string) (Lock, error)` — creates a lock file atomically using `os.OpenFile` with `O_CREATE|O_EXCL`. The lock file contains the run ID and timestamp for diagnostics
- [x] `Lock.Release() error` — removes the lock file
- [x] `IsLocked(dir, taskID string) bool` — checks if a lock file exists for the given task
- [x] Stale lock detection: lock files older than 2 hours are considered stale and can be overwritten (handles crashed sessions)
- [x] Add `.maggus/locks/` to the gitignore entries in `src/internal/gitignore/gitignore.go`
- [x] Unit tests cover: acquire, release, double-acquire fails, stale lock override

### TASK-003: Extend config and CLI flags for worktree option
**Description:** As a user, I want to enable worktree mode via `.maggus/config.yml` or CLI flags so I can control when worktrees are used.

**Acceptance Criteria:**
- [x] `Config` struct in `src/internal/config/config.go` gains a `Worktree bool` field (`yaml:"worktree"`)
- [x] `maggus work` command gains `--worktree` and `--no-worktree` bool flags
- [x] Resolution order: `--no-worktree` (force off) > `--worktree` (force on) > config file value > default (`false`)
- [x] Existing config files without the `worktree` key continue to work (defaults to `false`)
- [x] Unit tests for config loading with and without the worktree field
- [x] Typecheck/lint passes (`go vet ./...`)

### TASK-004: Add `.maggus-work/` to gitignore management
**Description:** As a developer, I want `.maggus-work/` automatically added to `.gitignore` so worktree directories are never committed.

**Acceptance Criteria:**
- [x] `src/internal/gitignore/gitignore.go` includes `.maggus-work/` in its required entries list
- [x] Existing `EnsureEntries` function handles the new entry without changes to its API
- [x] When `maggus work` runs (with or without worktree mode), `.maggus-work/` is present in `.gitignore`
- [x] Unit tests verify the new entry is added

### TASK-005: Integrate worktree lifecycle into the work loop
**Description:** As a user running `maggus work --worktree`, I want the work loop to create a worktree at the start of the session, run all iterations inside it, push the branch, and clean up the worktree when done.

**Acceptance Criteria:**
- [x] When worktree mode is enabled, the work loop creates a worktree at `.maggus-work/<run-id>/` before the iteration loop starts
- [x] The worktree is created on a new branch: `feature/maggustask-<first-task-number>` (based on the first task that will be worked on)
- [x] All Claude Code invocations, plan parsing, staging, and committing operate on the worktree directory (not the original repo dir)
- [x] The `.maggus/` directory content (plans, config, runs) is shared via the worktree (git worktree shares the git state), so plan file changes in the worktree are visible to other sessions re-parsing plans
- [x] Task locking (`tasklock.Acquire`) is called before each iteration; locked tasks are skipped (find next unlocked workable task)
- [x] Task locks are released after each iteration's commit succeeds
- [x] The startup banner displays the worktree path when in worktree mode
- [x] On completion (all tasks done or count reached): the branch is pushed, then the worktree is removed
- [x] On interrupt (Ctrl+C): the worktree is removed in a deferred cleanup (best-effort; warn but don't fail if removal fails)
- [x] When worktree mode is disabled, behavior is identical to the current implementation (no regressions)
- [x] Typecheck/lint passes (`go vet ./...`)

### TASK-006: Handle worktree cleanup edge cases
**Description:** As a user, I want robust worktree cleanup so that `.maggus-work/` doesn't accumulate stale worktrees from crashed or interrupted sessions.

**Acceptance Criteria:**
- [ ] New subcommand `maggus worktree clean` that removes all worktrees in `.maggus-work/` and their associated branches
- [ ] `maggus worktree clean` calls `git worktree remove` for each directory in `.maggus-work/`, then `git worktree prune`
- [ ] `maggus worktree list` subcommand that shows active worktrees with their run IDs and branches
- [ ] On `maggus work --worktree` startup, if a stale worktree exists from a previous crashed run (detectable by checking if the run's lock files are all stale), it is cleaned up automatically before creating a new one
- [ ] Cleanup removes associated task lock files in `.maggus/locks/`
- [ ] Typecheck/lint passes (`go vet ./...`)

### TASK-007: Update prompt context for worktree awareness
**Description:** As the Claude Code agent working inside a worktree, I need to know that I'm operating in a worktree so I don't make incorrect assumptions about the git state.

**Acceptance Criteria:**
- [ ] The prompt built by `src/internal/prompt/prompt.go` includes a `WORKTREE: true` metadata field when running in worktree mode
- [ ] The prompt includes `WORKTREE_DIR: .maggus-work/<run-id>` so the agent knows the working directory context
- [ ] The prompt instructions remind the agent that other sessions may be running concurrently and it should not make assumptions about branch state outside its own branch
- [ ] `prompt.Options` struct gains `Worktree bool` and `WorktreeDir string` fields
- [ ] Existing prompts (non-worktree mode) are unchanged
- [ ] Unit tests verify prompt output with and without worktree fields

## Functional Requirements

- FR-1: When `worktree: true` is set in config or `--worktree` is passed, `maggus work` must create a git worktree before starting iterations
- FR-2: The worktree must be located at `.maggus-work/<run-id>/` relative to the repository root
- FR-3: Each worktree must have its own feature branch, named `feature/maggustask-<NNN>` based on the first task
- FR-4: Task locking must prevent two concurrent sessions from working on the same task
- FR-5: Lock files must be stored in `.maggus/locks/` and use atomic file creation (`O_CREATE|O_EXCL`)
- FR-6: Stale locks (older than 2 hours) must be automatically overridden
- FR-7: The worktree must be removed after the run completes (both success and interrupt paths)
- FR-8: The feature branch must be pushed to the remote before worktree removal
- FR-9: `--no-worktree` must override `worktree: true` in config
- FR-10: `.maggus-work/` and `.maggus/locks/` must be gitignored automatically
- FR-11: `maggus worktree clean` must remove all worktrees and their lock files
- FR-12: `maggus worktree list` must display active worktrees with metadata

## Non-Goals

- No automatic merging of worktree branches — users create PRs manually
- No inter-process communication between parallel sessions (they coordinate only via lock files and shared git state)
- No support for running worktrees on different base branches simultaneously (all worktrees branch from the same current HEAD)
- No remote worktree support (all worktrees are local)
- No automatic conflict resolution if two worktrees modify the same files

## Technical Considerations

- Git worktrees share the `.git` directory, so `.maggus/` plan files modified in one worktree are visible in others after they're committed. However, uncommitted changes are worktree-local. The task lock mechanism handles coordination for uncommitted work-in-progress
- `git worktree add` requires the target directory to not exist, so the `.maggus-work/<run-id>/` path uses the unique run ID to avoid collisions
- `git worktree remove --force` is needed because the worktree may have uncommitted changes if a session is interrupted mid-task
- On Windows, `git worktree remove` may fail if a process still has the directory open. The cleanup should retry once after a short delay
- The `.maggus-work/` directory should be at the repo root level (not inside `.maggus/`) to keep worktree paths shorter and avoid confusion with the shared `.maggus/` data directory
- Plan files live in `.maggus/` which is tracked by git. In worktree mode, each worktree has its own copy of the working tree. When Claude Code modifies and commits a plan file in one worktree, other worktrees will see the change after they re-parse (since they parse from their own working tree, which reflects the committed state after a fetch/pull). For same-repo worktrees this is automatic since they share the git object store
- **Important**: uncommitted plan changes in one worktree are NOT visible to other worktrees. The task lock mechanism is the primary coordination tool, not plan file state

## Success Metrics

- A user can run `maggus work --worktree` and continue editing files in their main working directory without conflicts
- Two parallel `maggus work --worktree` sessions work on different tasks without stepping on each other
- After both sessions complete, two feature branches exist on the remote ready for PR creation
- `.maggus-work/` is empty after all sessions complete (no stale worktrees)
- Existing `maggus work` (without worktree flag) works identically to before

## Open Questions

- Should `maggus worktree clean` also delete the remote branches, or only local worktrees and branches?
- Should there be a maximum number of concurrent worktrees to prevent resource exhaustion?
- Should the lock timeout (2 hours) be configurable?
