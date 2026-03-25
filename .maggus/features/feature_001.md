<!-- maggus-id: 5f40cbd3-549c-4b27-b6a3-09770656ad07 -->
# Feature 001: Streamline Prompt Picker — Remove Description & Fix Slow Launch

## Introduction

The `maggus prompt` command's interactive picker currently has a Description text input field that adds an unnecessary step before launching Claude. Additionally, when a skill is selected, Claude launches via a slow two-step process (non-interactive print + resume), causing a visible delay where the user sees a bare shell for several seconds before Claude appears.

This feature removes the Description field entirely and switches to a single-step interactive launch, making the picker feel instant.

## Goals

- Remove the Description input field and all related code from the prompt picker
- Launch Claude interactively in a single step (no two-step print+resume)
- Pressing Enter on a skill in the skill list immediately launches Claude
- Reduce perceived launch time to near-instant after selection

## Tasks

### TASK-001-001: Remove Description Field from Prompt Picker
**Description:** As a user, I want the prompt picker to skip the Description input so that I can launch Claude faster with fewer steps.

**Token Estimate:** ~25k tokens
**Predecessors:** none
**Successors:** TASK-001-003
**Parallel:** yes — can run alongside TASK-001-002

**Acceptance Criteria:**
- [x] `promptPickerResult.Description` field is removed
- [x] `textinput` model and all `focusDescription` handling is removed from `prompt_picker.go`
- [x] `focusDescription` constant is removed; focus cycle is now: `focusSkillList` → `focusToggle`
- [x] The Description rendering block in `View()` (lines 279-291) is removed
- [x] Tab/Shift+Tab navigation skips directly between skill list and toggle
- [x] Pressing Enter while focused on the skill list immediately builds the result and quits (calls `tea.Quit`)
- [x] The `charmbracelet/bubbles/textinput` import is removed if no longer used
- [x] `prompt.go` lines 92-98 no longer reference `result.Description` — skill prompt is just the skill name
- [x] `go build ./...` succeeds
- [x] `go vet ./...` passes

### TASK-001-002: Replace Two-Step Launch with Single Interactive Launch
**Description:** As a user, I want Claude to open interactively immediately after I select a skill, instead of waiting for a hidden non-interactive step to complete.

**Token Estimate:** ~35k tokens
**Predecessors:** none
**Successors:** TASK-001-003
**Parallel:** yes — can run alongside TASK-001-001

**Acceptance Criteria:**
- [x] `launchInteractive()` in `plan.go` no longer runs a non-interactive `-p` step followed by `--resume`
- [x] When a skill prompt is provided, Claude is launched interactively in a single command that passes the skill as the initial message
- [x] The approach uses `--initial-prompt` or pipes the skill command as initial input — whichever Claude CLI supports for interactive mode
- [x] `generateSessionUUID()` is removed if no longer needed
- [x] Plain mode ("open console") still works as before (no prompt passed)
- [x] `ensureMaggusPlugin()` is still called before launching skill sessions
- [x] Usage extraction (`extractSkillUsage`) still works after the session ends
- [x] Signal handling (Ctrl+C forwarding) still works correctly
- [x] `go build ./...` succeeds
- [x] `go vet ./...` passes

### TASK-001-003: Integration Testing & Cleanup
**Description:** As a developer, I want to verify the full flow works end-to-end and remove any dead code left over from the previous tasks.

**Token Estimate:** ~15k tokens
**Predecessors:** TASK-001-001, TASK-001-002
**Successors:** none
**Parallel:** no — requires both predecessor tasks to be complete

**Acceptance Criteria:**
- [ ] Selecting a skill and pressing Enter launches Claude interactively with the skill command
- [ ] Selecting "open console" and pressing Enter launches Claude with no prompt
- [ ] The skip-permissions toggle still works and is passed correctly
- [ ] The `--model` flag still works
- [ ] No dead code remains (unused imports, unreachable functions, orphaned constants)
- [ ] `go build ./...` succeeds
- [ ] `go test ./...` passes
- [ ] `go vet ./...` passes

## Task Dependency Graph

```
TASK-001-001 ──→ TASK-001-003
TASK-001-002 ──┘
```

| Task | Estimate | Predecessors | Parallel | Model |
|------|----------|--------------|----------|-------|
| TASK-001-001 | ~25k | none | yes (with 002) | — |
| TASK-001-002 | ~35k | none | yes (with 001) | — |
| TASK-001-003 | ~15k | 001, 002 | no | — |

**Total estimated tokens:** ~75k

## Functional Requirements

- FR-1: The prompt picker must not display a Description input field
- FR-2: Pressing Enter while the skill list is focused must immediately launch Claude (after building the result)
- FR-3: The focus cycle in the picker must be: skill list → skip-permissions toggle (two stops only)
- FR-4: When a skill is selected, Claude must launch interactively with the skill command as the initial prompt in a single process invocation
- FR-5: When "open console" is selected, Claude must launch interactively with no prompt (current plain behavior)
- FR-6: Usage extraction must still detect the session file and record token usage after the session ends

## Non-Goals

- No changes to the skill list itself (same skills, same separators)
- No changes to the `maggus work` command or its TUI
- No changes to the `maggus plan` command beyond what's in `launchInteractive()`
- No new UI elements or features added to the picker

## Technical Considerations

- The Claude CLI's `--initial-prompt` flag (or equivalent) needs to be verified — check `claude --help` to confirm the correct flag for passing an initial message to an interactive session
- If no such flag exists, an alternative is writing the skill command to stdin after launch, but this is fragile — prefer a CLI flag
- Session file detection for usage extraction currently relies on snapshotting the session directory before/after; this must still work with the single-step approach
- The `ensureMaggusPlugin()` call must happen before the interactive launch since it runs `claude plugin` commands that would conflict with an active session

## Success Metrics

- Time from pressing Enter in the picker to seeing the Claude CLI is under 2 seconds
- Zero extra keystrokes required compared to current flow (fewer, in fact — no Tab through Description)
- Usage tracking still records accurate token counts after sessions

## Open Questions

_None — all questions resolved._
