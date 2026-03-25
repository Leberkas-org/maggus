<!-- maggus-id: ae7c2fb0-e8b2-4850-abc7-a11e55d686a2 -->
# Feature 005: Enhanced Discord Rich Presence

## Introduction

Enhance the Discord Rich Presence integration to show more contextual information about what Maggus is doing. Currently, Discord always shows "Running Maggus" as the state regardless of mode. This feature adds context-aware verbs (e.g. "Working", "Fixing", "Planning", "Consulting"), wires the `prompt` command into Discord presence, and adds a text-based progress bar showing feature/bug task completion during the work loop.

### Architecture Context

- **Components involved:** `internal/discord` (presence state model + protocol), `cmd/prompt.go` (prompt command), `cmd/work_task.go` (work loop presence updates), `cmd/root.go` (menu presence)
- **Existing patterns:** `PresenceState` struct drives all presence updates; `buildActivity()` in `protocol.go` maps state to Discord payload. The `prompt` command currently has no Discord integration at all.
- **New patterns:** Verb-based state field, progress tracking in presence state, skill-to-verb mapping

## Goals

- Show context-aware activity verbs in Discord instead of generic "Running Maggus"
- Distinguish between "Working" (features) and "Fixing" (bugs) in the work loop
- Show skill-specific verbs during prompt mode (e.g. "Planning", "Consulting", "Architecting")
- Display task progress as text in the state field during work (e.g. "Working — 3/7 tasks (43%)")
- Wire the `prompt` command into Discord Rich Presence so it shows activity

## Tasks

### TASK-005-001: Extend PresenceState with Verb and Progress fields
**Description:** As a developer, I want the `PresenceState` struct to support a context-aware verb and optional progress data so that downstream code can set richer presence information.

**Token Estimate:** ~25k tokens
**Predecessors:** none
**Successors:** TASK-005-002, TASK-005-003, TASK-005-004
**Parallel:** yes — can run alongside nothing (foundational task, but small)

**Acceptance Criteria:**
- [ ] `PresenceState` in `internal/discord/discord.go` has a new `Verb` field (string) — e.g. "Working", "Fixing", "Planning", "Consulting"
- [ ] `PresenceState` has new `ProgressCurrent` (int) and `ProgressTotal` (int) fields for task progress (0/0 means no progress bar)
- [ ] `buildActivity()` in `protocol.go` uses `Verb` for the `State` field instead of hardcoded "Running Maggus". Falls back to "Running Maggus" when Verb is empty (backward compat)
- [ ] When `ProgressTotal > 0`, the state field format is: `"Verb — Current/Total tasks (XX%)"` (e.g. "Working — 3/7 tasks (43%)")
- [ ] When `ProgressTotal == 0`, the state field is just the verb (e.g. "Consulting")
- [ ] `FormatDetails()` behavior is unchanged
- [ ] Existing tests in `discord_test.go` still pass
- [ ] New unit tests cover: verb-only state, verb with progress, empty verb fallback, progress percentage calculation (including 0/0, 1/1, 0/5 edge cases)
- [ ] `go vet ./...` and `go test ./...` pass

### TASK-005-002: Add skill-to-verb mapping and wire prompt command to Discord
**Description:** As a user, I want my Discord status to show what I'm doing in prompt mode (e.g. "Planning" when running `/maggus-plan`, "Consulting" when using open console) so my friends and colleagues see meaningful activity.

**Token Estimate:** ~40k tokens
**Predecessors:** TASK-005-001
**Successors:** TASK-005-005
**Parallel:** yes — can run alongside TASK-005-003, TASK-005-004

**Acceptance Criteria:**
- [ ] A verb mapping exists (can be in `cmd/prompt.go` or a shared location) that maps picker labels to Discord verbs:
  - `"open console"` → `"Consulting"`
  - `"/maggus-plan"` → `"Planning"`
  - `"/maggus-vision"` → `"Visioning"`
  - `"/maggus-architecture"` → `"Architecting"`
  - `"/maggus-bugreport"` → `"Reporting Bug"`
  - `"/bryan-plan"` → `"Planning"`
  - `"/bryan-bugreport"` → `"Reporting Bug"`
- [ ] `runPrompt()` in `cmd/prompt.go` connects to Discord presence (same pattern as `runMenu` — check `cfg.DiscordPresence`, connect, defer close)
- [ ] After the user picks a skill, Discord presence is updated with the appropriate verb, no progress (ProgressTotal=0), and `StartTime` set to now
- [ ] The details field shows the skill name or "Open Console" as the feature title
- [ ] Presence is cleared on command exit (deferred Close)
- [ ] When Discord is not running, the prompt command works exactly as before (no errors)
- [ ] `go vet ./...` and `go test ./...` pass

### TASK-005-003: Use "Working" vs "Fixing" verbs in the work loop
**Description:** As a user, I want Discord to show "Working" when Maggus processes a feature task and "Fixing" when it processes a bug task, so my status reflects what kind of work is happening.

**Token Estimate:** ~30k tokens
**Predecessors:** TASK-005-001
**Successors:** TASK-005-005
**Parallel:** yes — can run alongside TASK-005-002, TASK-005-004

**Acceptance Criteria:**
- [ ] In `cmd/work_task.go`, the `presence.Update()` call determines the verb based on the task's `SourceFile` path:
  - If path contains `/bugs/` or `\bugs\` → verb is `"Fixing"`
  - Otherwise → verb is `"Working"`
- [ ] The verb detection logic is a simple helper function (e.g. `verbForTask(sourceFile string) string`) with unit tests
- [ ] Existing Discord presence update in `runTask()` passes the correct verb
- [ ] Menu idle presence uses verb `"Idle"` or remains as-is (no verb needed for menu — keep current behavior)
- [ ] `go vet ./...` and `go test ./...` pass

### TASK-005-004: Add progress tracking to Discord presence during work
**Description:** As a user, I want to see task completion progress in my Discord status (e.g. "Working — 3/7 tasks (43%)") so I can glance at Discord and know how far along the current feature is.

**Token Estimate:** ~40k tokens
**Predecessors:** TASK-005-001
**Successors:** TASK-005-005
**Parallel:** yes — can run alongside TASK-005-002, TASK-005-003

**Acceptance Criteria:**
- [ ] When `presence.Update()` is called in `runTask()`, it includes `ProgressCurrent` and `ProgressTotal` computed from the task list scoped to the current feature/bug file
- [ ] Progress counts completed tasks vs total tasks in the same source file (using `task.IsComplete()` from the parser)
- [ ] The progress is calculated at the start of each task iteration (before the agent runs), showing how many tasks are already done
- [ ] After a task completes and the commit succeeds, presence is updated again with incremented progress (the just-completed task now counts as done)
- [ ] When `--task` flag targets a single task (no broader feature context), progress shows 0/1 or 1/1
- [ ] Edge case: if all tasks are complete after the final iteration, progress shows e.g. "7/7 tasks (100%)"
- [ ] `go vet ./...` and `go test ./...` pass

### TASK-005-005: Integration tests for enhanced Discord presence
**Description:** As a developer, I want integration tests that verify the enhanced presence updates are sent correctly across prompt and work modes so we catch regressions.

**Token Estimate:** ~35k tokens
**Predecessors:** TASK-005-002, TASK-005-003, TASK-005-004
**Successors:** none
**Parallel:** no

**Acceptance Criteria:**
- [ ] Test that `buildActivity()` produces correct state strings for all verb+progress combinations
- [ ] Test the skill-to-verb mapping covers all entries in `skillMappings`
- [ ] Test `verbForTask()` with feature paths, bug paths, and edge cases (Windows vs Unix separators)
- [ ] Test progress percentage formatting: 0%, 14.28% rounds correctly, 100%, 0/0 (no progress)
- [ ] Test that `FormatDetails()` still works correctly with the new fields (backward compatibility)
- [ ] All tests pass: `cd src && go test ./...`

## Task Dependency Graph

```
TASK-005-001 ──→ TASK-005-002 ──→ TASK-005-005
             ├─→ TASK-005-003 ──┘
             └─→ TASK-005-004 ──┘
```

| Task | Estimate | Predecessors | Parallel | Model |
|------|----------|--------------|----------|-------|
| TASK-005-001 | ~25k | none | — | — |
| TASK-005-002 | ~40k | 001 | yes (with 003, 004) | — |
| TASK-005-003 | ~30k | 001 | yes (with 002, 004) | — |
| TASK-005-004 | ~40k | 001 | yes (with 002, 003) | — |
| TASK-005-005 | ~35k | 002, 003, 004 | no | — |

**Total estimated tokens:** ~170k

## Functional Requirements

- FR-1: The Discord state field must show a context-aware verb instead of "Running Maggus"
- FR-2: During `work`, the verb must be "Working" for feature tasks and "Fixing" for bug tasks, determined by the task's source file path
- FR-3: During `prompt`, the verb must match the selected skill: "Consulting" (open console), "Planning" (/maggus-plan, /bryan-plan), "Visioning" (/maggus-vision), "Architecting" (/maggus-architecture), "Reporting Bug" (/maggus-bugreport, /bryan-bugreport)
- FR-4: When progress data is available (ProgressTotal > 0), the state field must show "Verb — X/Y tasks (Z%)"
- FR-5: When no progress data is available, the state field must show only the verb
- FR-6: The `prompt` command must connect to Discord presence when `discord_presence: true` in config
- FR-7: Progress is scoped to the current feature/bug file — not across all features
- FR-8: All existing Discord functionality (graceful degradation, disconnect handling, activity clearing on close) must continue working unchanged

## Non-Goals

- No graphical progress bar (Discord Rich Presence doesn't support actual bar widgets — text-based only)
- No per-acceptance-criterion progress (progress tracks tasks, not individual checkboxes within tasks)
- No Discord presence for `list`, `status`, or `config` commands
- No new Discord application assets or images
- No reconnection logic changes

## Technical Considerations

- The `SourceFile` path on `parser.Task` uses OS-native separators — the bug detection helper must handle both `/bugs/` and `\bugs\` for cross-platform correctness
- Discord's `state` field has a 128-character limit — the verb + progress string must stay well under this
- The `prompt` command launches Claude Code as an interactive subprocess — Discord presence should be set before launch and cleared after exit. There is no mid-session update opportunity (unlike the work loop which iterates)
- Progress calculation requires access to the full task list for the current source file. In `runTask()`, this is available via the `tasks` parameter

## Success Metrics

- Discord status shows the correct verb for every mode (work/prompt/menu)
- Feature vs bug tasks show "Working" vs "Fixing" respectively
- Progress text updates after each completed task during work
- No regressions in existing Discord functionality
- Zero impact on performance or startup time when Discord is not running
