<!-- maggus-id: 85f2f7a9-520d-42f6-8a4b-d917355024a2 -->
# Feature 047: Skip Plan Branch Creation on Non-Protected Branches

## Introduction

When the daemon starts on a non-protected branch (e.g., a feature branch you're already working on), it currently creates an unnecessary plan branch (`feature/maggus-NNN-plan`). This is redundant — the daemon should use the current branch as the base and merge task branches back to it. Plan branches should only be created when starting from a protected branch (main/master/dev).

Also renames plan branch prefixes: `feature/feat-NNN` for features and `fix/bug-NNN` for bugs (replacing the current `feature/maggus-NNN` and `bugfix/maggus-bug-NNN` patterns).

### Architecture Context

- **Vision alignment:** Cleaner git history and simpler branching model
- **Components involved:** `internal/gitbranch` (plan branch naming and creation), `cmd/daemon_keepalive.go` (plan branch setup in work cycle), `cmd/run_parallel.go` (plan branch usage in parallel orchestrator)

## Goals

- No plan branch is created when the daemon starts on a non-protected branch
- Task branches are created off the current branch and merge back to it
- When starting from a protected branch, plan branches use new naming: `feature/feat-NNN` (features) and `fix/bug-NNN` (bugs)
- Parallel and sequential modes both respect the new behavior

## User Stories

### TASK-047-001: Update plan branch naming and skip creation on non-protected branches
**Description:** As a developer, I want the daemon to work directly on my current branch when I'm not on a protected branch, and use cleaner plan branch names when I am.

**Token Estimate:** ~50k tokens
**Predecessors:** none
**Successors:** TASK-047-002
**Parallel:** no
**Model:** opus

**Acceptance Criteria:**
- [x] `gitbranch.PlanBranchNameFromTaskID("TASK-038-003")` returns `feature/feat-038` (was `feature/maggus-038`)
- [x] `gitbranch.PlanBranchNameFromTaskID("BUG-001-003")` returns `fix/bug-001` (was `bugfix/maggus-bug-001`)
- [x] New function `gitbranch.ShouldCreatePlanBranch(currentBranch string, protectedList []string) bool` returns true only when `currentBranch` is in `protectedList`
- [x] `EnsurePlanBranch` returns the current branch name (unchanged) when `ShouldCreatePlanBranch` is false
- [x] `EnsurePlanBranch` creates and checks out the new plan branch when `ShouldCreatePlanBranch` is true
- [x] All existing `gitbranch` tests updated to expect new naming
- [x] New tests cover: non-protected branch passthrough, protected branch creation, bug naming
- [x] `go vet ./...` and `go test ./...` pass

### TASK-047-002: Wire up plan branch skip in daemon work loop
**Description:** As a user, I want the daemon to use my current branch as the base when I'm not on a protected branch so no unnecessary branches are created.

**Token Estimate:** ~35k tokens
**Predecessors:** TASK-047-001
**Successors:** TASK-047-003
**Parallel:** no

**Acceptance Criteria:**
- [x] `daemon_keepalive.go` passes the protected branch list from config to `EnsurePlanBranch`
- [x] When on a non-protected branch, `planBranch` is set to the current branch (no new branch created)
- [x] Task branches in `run_parallel.go` are created off `planBranch` (which is now the current branch when non-protected)
- [x] After task completion, task branches merge back to `planBranch` (which is the current branch)
- [x] Sequential mode in `daemon_keepalive.go` also respects the non-protected branch passthrough
- [x] Starting the daemon on `master` still creates a `feature/feat-NNN` plan branch
- [x] Starting the daemon on `my-feature-branch` does NOT create a plan branch
- [x] `go vet ./...` and `go test ./...` pass

### TASK-047-003: Update docs for new branching behavior
**Description:** As a user reading the docs, I want the branching documentation to reflect the new naming and non-protected branch behavior.

**Token Estimate:** ~15k tokens
**Predecessors:** TASK-047-002
**Successors:** none
**Parallel:** no

**Acceptance Criteria:**
- [x] `docs/guide/concepts.md` Git Branch Behavior section updated: mentions `feature/feat-NNN` and `fix/bug-NNN` naming; explains that no plan branch is created on non-protected branches
- [x] `CLAUDE.md` gitbranch package description updated
- [x] `ARCHITECTURE.md` branch naming references updated if present
- [x] `go vet ./...` and `go test ./...` pass

## Task Dependency Graph

```
TASK-047-001 ──→ TASK-047-002 ──→ TASK-047-003
```

| Task | Estimate | Predecessors | Parallel | Model |
|------|----------|--------------|----------|-------|
| TASK-047-001 | ~50k | none | no | opus |
| TASK-047-002 | ~35k | 001 | no | — |
| TASK-047-003 | ~15k | 002 | no | — |

**Total estimated tokens:** ~100k

## Functional Requirements

- FR-1: On a protected branch, the daemon creates a plan branch: `feature/feat-NNN` for features, `fix/bug-NNN` for bugs
- FR-2: On a non-protected branch, the daemon stays on the current branch — no plan branch created
- FR-3: Task branches always branch off the plan branch (which may be the current branch when non-protected)
- FR-4: Task branches merge back to the plan branch after completion
- FR-5: The protected branch list is configurable via `git.protected_branches` in config

## Non-Goals

- No changes to task branch naming (that's feature 043)
- No changes to worktree behavior
- No migration of existing plan branches to new naming

## Technical Considerations

- The parallel orchestrator stores `planBranch` and passes it through — the change is mostly in `EnsurePlanBranch` and `daemon_keepalive.go`
- Sequential mode uses `setupBranch` which calls `EnsureFeatureBranch` — this needs the same non-protected check
- Dispatched workers receive the base branch via `--dispatch-base-branch` flag — this already works regardless of whether it's a plan branch or the original branch

## Open Questions

None.
