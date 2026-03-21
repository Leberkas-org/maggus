# Feature 003: Support Bug Report Files in Work Loop

## Introduction

Extend maggus to recognize and process bug report files (`.maggus/bugs/bug_N.md`) alongside feature files. Bug files use the same `### TASK-NNN:` format with acceptance criteria but live in a separate directory, use `BUG-NNN-XXX` task IDs, and create `bugfix/` branches. The work loop picks up bug tasks before feature tasks, and the status view shows bugs in a separate tab group.

## Goals

- Parse bug files from `.maggus/bugs/` using the same task/criteria structure as features
- Auto-migrate legacy `TASK-NNN` IDs in bug files to `BUG-NNN-XXX` format silently
- Merge bug tasks into the work loop, processing bugs before features
- Show bugs as separate tabs in the status view
- Rename completed bugs to `bug_N_completed.md`
- Create `bugfix/maggus-bug-NNN` branches for bug tasks
- Support `BUG-NNN-XXX` task ID format

## Tasks

### TASK-003-001: Extend parser to glob, parse, and auto-migrate bug files
**Description:** As a developer, I want the parser to discover and parse `bug_*.md` files from `.maggus/bugs/` so that bug tasks are available to the work loop.

**Token Estimate:** ~70k tokens
**Predecessors:** none
**Successors:** TASK-003-003, TASK-003-004, TASK-003-005
**Parallel:** yes — can run alongside TASK-003-002

**Acceptance Criteria:**
- [x] New `GlobBugFiles(dir, includeCompleted)` function finds `bug_*.md` files in `.maggus/bugs/`, excludes `_completed.md` unless requested
- [x] New `ParseBugs(dir)` function returns `[]Task` from all active bug files
- [x] New `ParseBugsGrouped(dir)` function returns `[]Feature` (reuse the `Feature` struct) grouped by bug file
- [x] `SortBugFiles()` sorts bug files numerically (same logic as features but for `bug_` prefix)
- [x] `MarkCompletedBugs(dir)` renames fully-completed bug files to `bug_N_completed.md`
- [x] `IsIgnoredFile()` also handles `bug_N_ignored.md` files
- [x] Bug task IDs use `BUG-NNN-XXX` format (e.g., `BUG-001-001`) and are parsed correctly by `ParseFile`
- [x] Auto-migration: when a bug file contains legacy `TASK-NNN` IDs (e.g., `### TASK-001:`), the parser rewrites them in-place to `BUG-NNN-XXX` format (where NNN is derived from the bug file number) silently before parsing
- [x] Existing feature parsing is not affected
- [x] `go fmt ./...` and `go vet ./...` pass
- [x] Unit tests cover bug file globbing, parsing, sorting, completion, ignored detection, and auto-migration of legacy IDs

### TASK-003-002: Update gitbranch to create bugfix branches for BUG- task IDs
**Description:** As a developer, I want bug tasks to create `bugfix/` branches instead of `feature/` branches so that branch naming reflects the work type.

**Token Estimate:** ~20k tokens
**Predecessors:** none
**Successors:** TASK-003-003
**Parallel:** yes — can run alongside TASK-003-001

**Acceptance Criteria:**
- [x] `FeatureBranchName()` (or a renamed/extended function) detects `BUG-NNN` prefix in the task ID
- [x] Bug task IDs produce `bugfix/maggus-bug-NNN` branches (e.g., `BUG-001-001` → `bugfix/maggus-bug-001`)
- [x] Feature task IDs still produce `feature/maggustask-NNN` branches (no regression)
- [x] Protected branch check still applies for bug branches
- [x] `go fmt ./...` and `go vet ./...` pass
- [x] Unit tests cover both feature and bug branch name generation

### TASK-003-003: Integrate bug tasks into the work loop
**Description:** As a user, I want `maggus work` to pick up bug tasks before feature tasks so that bugs are fixed as part of the normal workflow.

**Token Estimate:** ~50k tokens
**Predecessors:** TASK-003-001, TASK-003-002
**Successors:** TASK-003-005
**Parallel:** no

**Acceptance Criteria:**
- [x] `initIteration()` parses both features and bugs, merges them into a single task list with bugs first, then features
- [x] `FindNextIncomplete()` works on the merged list without modification
- [x] `--task BUG-NNN-XXX` flag works to target a specific bug task
- [x] `MarkCompletedBugs()` is called alongside `MarkCompletedFeatures()` after each iteration
- [x] Prompt builder correctly references the bug source file (not a feature file) for checkbox updates
- [x] `capCount()` accounts for workable tasks from both sources
- [x] `go fmt ./...` and `go vet ./...` pass
- [x] Unit tests cover merged task selection, bug-first ordering, and bug-specific task targeting

### TASK-003-004: Show bugs as separate tab group in status view
**Description:** As a user, I want to see bug files as separate tabs in the status view so I can track bug fix progress alongside features.

**Token Estimate:** ~45k tokens
**Predecessors:** TASK-003-001
**Successors:** none
**Parallel:** yes — can run alongside TASK-003-003, TASK-003-005

**Acceptance Criteria:**
- [ ] Status view parses both features and bugs via `ParseFeaturesGrouped` and `ParseBugsGrouped`
- [ ] Bug tabs appear after feature tabs, visually separated (e.g., different label style or a separator)
- [ ] Bug tabs show the bug filename (e.g., `bug_1.md`)
- [ ] Task display, progress bars, completion status, and criteria all work identically for bug tasks
- [ ] Interactive actions (delete task, resolve/unblock criteria) work on bug files
- [ ] `findNextTask()` considers both bugs and features, with bugs taking priority
- [ ] `go fmt ./...` and `go vet ./...` pass
- [ ] Unit tests cover bug tab rendering and mixed feature/bug navigation

### TASK-003-005: Update list command to include bug tasks
**Description:** As a user, I want `maggus list` to show upcoming bug tasks before feature tasks so I can preview the full queue.

**Token Estimate:** ~25k tokens
**Predecessors:** TASK-003-001, TASK-003-003
**Successors:** none
**Parallel:** yes — can run alongside TASK-003-004

**Acceptance Criteria:**
- [ ] List command parses both features and bugs
- [ ] Bug tasks appear before feature tasks, matching work loop ordering
- [ ] Bug tasks are displayed with their `BUG-NNN-XXX` IDs
- [ ] Source file column shows the bug filename
- [ ] `go fmt ./...` and `go vet ./...` pass
- [ ] Unit tests cover listing with mixed feature and bug tasks

## Task Dependency Graph

```
TASK-003-001 ──→ TASK-003-003 ──→ TASK-003-005
     │                │
     └──→ TASK-003-004 (parallel with 003, 005)
TASK-003-002 ──→ TASK-003-003
```

| Task | Estimate | Predecessors | Parallel | Model |
|------|----------|--------------|----------|-------|
| TASK-003-001 | ~70k | none | yes (with 002) | — |
| TASK-003-002 | ~20k | none | yes (with 001) | — |
| TASK-003-003 | ~50k | 001, 002 | no | — |
| TASK-003-004 | ~45k | 001 | yes (with 003, 005) | — |
| TASK-003-005 | ~25k | 001, 003 | yes (with 004) | — |

**Total estimated tokens:** ~210k

## Functional Requirements

- FR-1: The parser must discover `bug_*.md` files in `.maggus/bugs/` and parse `### TASK-NNN:` sections with acceptance criteria identically to feature files
- FR-2: Bug task IDs must use `BUG-NNN-XXX` format where NNN is the bug number and XXX is the task number within the bug
- FR-3: Legacy `TASK-NNN` IDs in bug files must be auto-migrated to `BUG-NNN-XXX` format by silently rewriting the file in-place on first parse
- FR-4: `maggus work` must process bug tasks before feature tasks — bugs are prioritized over new features
- FR-5: Bug tasks must create `bugfix/maggus-bug-NNN` branches when on a protected branch
- FR-6: Completed bug files must be renamed from `bug_N.md` to `bug_N_completed.md`
- FR-7: `maggus status` must show bug files as separate tabs after feature tabs
- FR-8: `maggus work --task BUG-NNN-XXX` must target a specific bug task
- FR-9: The prompt builder must reference the correct bug source file for checkbox updates

## Non-Goals

- No priority system between bugs and features beyond the fixed "bugs first" ordering
- No new CLI commands specific to bugs (e.g., no `maggus bugs` command)
- No changes to the bug report creation skill (`/maggus-bugreport`) — updating it to generate `BUG-NNN-XXX` IDs will be handled separately
- No severity/priority fields on bug files
- No cross-referencing between bug files and feature files at the parser level

## Technical Considerations

- The `Feature` struct can be reused for bug grouping — it contains `File`, `Ignored`, and `Tasks[]` which are all applicable
- `ParseFile()` is already generic — it finds `### TASK-NNN:` headings regardless of file location. The main work is in globbing, file management, and auto-migration functions
- The auto-migration must derive the bug number from the filename (e.g., `bug_1.md` → NNN=001) and rewrite all `### TASK-XXX:` headings to `### BUG-001-XXX:` before parsing. This is a one-time rewrite per file
- The prompt's instruction to "update the feature file" needs to be generic ("update the source file") so it works for both features and bugs
- The `BUG-NNN-XXX` ID format means `FindNextIncomplete` and other generic task functions need no changes — they work on `Task.ID` strings regardless of prefix

## Success Metrics

- `maggus work` processes a bug file with BUG-prefixed tasks end-to-end without manual intervention
- Legacy bug files with `TASK-NNN` IDs are silently migrated on first run
- `maggus status` shows both feature and bug tabs with correct progress tracking
- Bug tasks are always processed before feature tasks
- Completed bug files are automatically renamed
- No regression in feature-only workflows

## Open Questions

*None — all questions resolved.*
