# Plan: Git Sync Integration

## Introduction

Integrate git remote synchronization checks into the `maggus work` flow. Before starting work and between task iterations, Maggus should detect if the remote has new changes, warn about uncommitted local changes, and let the user decide how to resolve the situation — all within the existing Bubbletea TUI. This prevents working on stale code and avoids push conflicts after completing tasks.

## Goals

- Detect remote changes before work begins and between task iterations
- Warn users about uncommitted local changes that would conflict with a pull
- Provide interactive resolution options within the Bubbletea TUI
- Never silently discard user work — always require explicit user choice for destructive actions

## User Stories

### TASK-001: Create `gitsync` internal package with remote status detection
**Description:** As a developer, I want a `gitsync` package that can check if the remote branch has new commits, so that Maggus can inform me before I start working on stale code.

**Acceptance Criteria:**
- [x] New package `src/internal/gitsync/` created
- [x] `FetchRemote(dir string) error` function runs `git fetch` to update remote tracking refs
- [x] `RemoteStatus(dir string) (Status, error)` function compares local HEAD with remote tracking branch
- [x] `Status` struct contains: `Behind int`, `Ahead int`, `RemoteBranch string`, `HasRemote bool`
- [x] Uses `git rev-list --count --left-right HEAD...@{upstream}` to determine ahead/behind counts
- [x] Handles the case where no upstream is configured (returns `HasRemote: false`, no error)
- [x] Handles the case where `git fetch` fails (e.g. no network) gracefully with a warning, not a fatal error
- [x] Unit tests cover: ahead-only, behind-only, both, up-to-date, no-upstream, detached HEAD

### TASK-002: Add local working tree status detection to `gitsync`
**Description:** As a developer, I want Maggus to detect uncommitted local changes so that I'm warned before a pull would fail or overwrite my work.

**Acceptance Criteria:**
- [x] `WorkingTreeStatus(dir string) (WorkTree, error)` function added to `gitsync` package
- [x] `WorkTree` struct contains: `HasUncommittedChanges bool`, `HasUntrackedFiles bool`, `ModifiedFiles []string`
- [x] Uses `git status --porcelain` to detect changes
- [x] `ModifiedFiles` is capped to first 10 entries (for display purposes, full count reported separately)
- [x] `TotalModified int` field reports the total count
- [x] Unit tests cover: clean tree, staged changes, unstaged changes, untracked files, mixed state

### TASK-003: Implement pull resolution actions in `gitsync`
**Description:** As a developer, I want Maggus to perform various git pull strategies so that I can choose how to sync with the remote.

**Acceptance Criteria:**
- [ ] `Pull(dir string) error` — standard `git pull` (fast-forward merge)
- [ ] `PullRebase(dir string) error` — `git pull --rebase`
- [ ] `ForcePull(dir string) error` — `git fetch origin && git reset --hard @{upstream}` (discards local commits and changes)
- [ ] `StashAndPull(dir string) error` — `git stash push -m "maggus: auto-stash before pull" && git pull && git stash pop`
- [ ] `StashAndPull` returns a specific error type if stash pop has conflicts, so the TUI can inform the user
- [ ] `ForcePull` requires an explicit confirmation parameter (e.g. `confirm bool`) — returns error if `false`
- [ ] Unit tests cover success and failure paths for each action
- [ ] Unit tests verify `ForcePull` refuses to run without confirmation

### TASK-004: Add git sync TUI screen to the Bubbletea work flow
**Description:** As a user, I want to see a pre-work TUI screen that shows me the git sync status and lets me choose how to resolve it, so that I can stay in the TUI flow without switching to a separate terminal.

**Acceptance Criteria:**
- [ ] New TUI view/state added that displays before the work loop begins
- [ ] Shows: current branch, remote status (ahead/behind counts), uncommitted changes summary
- [ ] When up-to-date and clean: auto-proceeds to work after brief display (e.g. 1-2 seconds or immediately with a status line)
- [ ] When behind remote: shows resolution menu with options: Pull, Pull with rebase, Force pull (reset to remote), Stash & pull, Skip (continue without pulling), Abort
- [ ] When local uncommitted changes exist AND behind remote: warns about uncommitted changes before showing pull options; Pull and Pull with rebase are shown but marked with a warning; Stash & pull is highlighted as recommended
- [ ] When only local uncommitted changes exist but up-to-date with remote: shows a warning but allows proceeding
- [ ] "Force pull" option shows a confirmation prompt ("This will discard ALL local commits and changes. Are you sure?")
- [ ] "Abort" exits `maggus work` cleanly
- [ ] After successful pull action: shows result message and proceeds to work
- [ ] If pull action fails: shows error and returns to resolution menu
- [ ] Keyboard navigation: arrow keys to select, Enter to confirm, `q` or Esc for Abort

### TASK-005: Integrate sync check at the start of `maggus work`
**Description:** As a developer, I want the git sync check to run automatically when I start `maggus work`, so that I always begin with an up-to-date codebase.

**Acceptance Criteria:**
- [ ] `cmd/work.go` calls `gitsync.FetchRemote` and `gitsync.RemoteStatus` before entering the work loop
- [ ] `gitsync.WorkingTreeStatus` is also checked
- [ ] If behind or dirty: the sync TUI screen (TASK-004) is shown before the main work TUI starts
- [ ] If up-to-date and clean: a brief info message is shown in the work TUI banner (e.g. "Branch up to date with origin/master")
- [ ] If fetch fails (no network): shows a warning "Could not reach remote — working offline" and proceeds
- [ ] If no remote is configured: silently skips the remote check (no warning needed)
- [ ] Worktree mode: sync check runs against the main repo before creating the worktree

### TASK-006: Integrate sync check between task iterations
**Description:** As a developer, I want Maggus to re-check the remote between tasks so that if someone else pushed while I was working, I find out before the next task rather than at push time.

**Acceptance Criteria:**
- [ ] Between task iterations in the work loop, `gitsync.FetchRemote` and `gitsync.RemoteStatus` are called
- [ ] Only the remote check is performed between tasks (not working tree status — Maggus itself manages the working tree at this point)
- [ ] If remote is ahead: the sync TUI screen is shown with resolution options before continuing to the next task
- [ ] If up-to-date: work continues without interruption (no message needed)
- [ ] If fetch fails: a warning is sent to the TUI info area and work continues
- [ ] The check does NOT run after the final task (unnecessary since push happens next)
- [ ] Context cancellation (Ctrl+C) during the sync check is handled gracefully

## Functional Requirements

- FR-1: `git fetch` must be called before any remote comparison to ensure refs are current
- FR-2: Remote status detection must use `git rev-list --count --left-right` for accurate ahead/behind counts
- FR-3: Working tree status must use `git status --porcelain` for reliable change detection
- FR-4: Force pull must require explicit user confirmation and must never run automatically
- FR-5: All git operations must respect the `workDir` parameter (support both normal and worktree modes)
- FR-6: Network failures during fetch must be non-fatal — Maggus should warn and continue
- FR-7: The sync TUI must integrate with the existing Bubbletea program lifecycle (not spawn a separate program)
- FR-8: Stash & pull must use a named stash (`maggus: auto-stash before pull`) for easy identification

## Non-Goals

- No standalone `maggus sync` command — this is integrated into `maggus work` only
- No automatic conflict resolution — if a pull results in merge conflicts, show the error and let the user handle it outside Maggus
- No remote push management — pushing is already handled at the end of the work loop
- No multi-remote support — only the default `origin` remote is checked
- No configuration options for sync behavior in `config.yml` (can be added later if needed)

## Technical Considerations

- The existing TUI in `runner/` uses Bubbletea with alt-screen. The sync screen needs to either be a state within the same program or a separate short-lived program that runs before the main TUI starts. Separate program before alt-screen is simpler and avoids complexity in the existing TUI model.
- `git fetch` can be slow on large repos or poor connections — consider a timeout (e.g. 15 seconds)
- `git stash pop` after pull can itself create merge conflicts — this needs clear error reporting
- The `gitsync` package should follow the same pattern as `gitbranch` and `gitcommit`: thin wrappers around git commands with proper error handling
- All git commands must set `cmd.Dir` to the working directory, consistent with existing packages

## Success Metrics

- User is never surprised by remote changes when starting or during `maggus work`
- No accidental data loss — force pull always requires explicit confirmation
- Network failures don't block the workflow — user can always skip and work offline
- The sync screen feels natural and fast — auto-proceeds when everything is clean

## Open Questions

- Should there be a `--no-sync` flag to skip the check entirely for offline/airplane mode use?
- Should the between-task check also detect if someone force-pushed to the remote (i.e. local ahead count decreased)?
