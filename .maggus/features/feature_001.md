# Feature 001: Combined `prompt` Command with TUI Picker

## Introduction

Replace the four separate `prompt`, `plan`, `vision`, and `architecture` commands with a single `maggus prompt` command. When invoked, it opens an interactive TUI where the user picks a skill (or plain prompt), optionally types a description, and toggles `--dangerously-skip-permissions` (on by default) — then launches Claude.

### Architecture Context

- **Components touched:** `cmd/prompt.go`, `cmd/plan.go`, `cmd/menu.go`
- **Pattern:** Follows existing bubbletea TUI patterns from `cmd/config.go` and `cmd/menu.go`
- **Removed:** `planCmd`, `visionCmd`, `architectureCmd` cobra commands and their menu entries
- **Reused:** `launchInteractive`, `ensureMaggusPlugin`, `extractSkillUsage` utilities from `plan.go`

## Goals

- Single entry point for all Claude prompt modes
- User picks skill + types description in the TUI before Claude launches
- `--dangerously-skip-permissions` is toggleable, on by default
- Old `plan`/`vision`/`architecture` commands removed; menu updated accordingly

## Tasks

### TASK-001-001: Build the prompt picker TUI model

**Description:** As a developer, I want a bubbletea model that lets me pick a prompt mode, type a description, and toggle skip-permissions, so that I have one place to configure and launch any Claude session.

**Token Estimate:** ~55k tokens
**Predecessors:** none
**Successors:** TASK-001-002
**Parallel:** no

**Acceptance Criteria:**
- [x] New bubbletea model `promptPickerModel` in `cmd/prompt_picker.go`
- [x] Skill list contains these 7 options (in order): Plain, Plan, Vision, Architecture, Bug report, Bryan: plan, Bryan: bug report
- [x] Arrow keys (up/down) navigate the skill list; selected item is highlighted
- [x] A text input field is shown below the list for description; it is hidden (or greyed out) when "Plain" is selected
- [x] A toggle row `Skip permissions` shows `on / off`, default `on`; left/right or enter cycles the value
- [x] Tab (or down past the list) moves focus between the skill list, the description input, and the toggle
- [x] Enter on the description input or a dedicated "Launch" action confirms the selection
- [x] `q` / `esc` / `ctrl+c` exits without launching
- [x] Model returns a `promptPickerResult` struct: `{ Skill string, Description string, SkipPermissions bool, Cancelled bool }`
- [x] `go build ./...` passes

### TASK-001-002: Wire picker into `prompt` command and launch Claude

**Description:** As a developer, I want `runPrompt` to display the picker TUI, then launch Claude with the correct flags and skill, so that all prompt modes go through one consistent code path.

**Token Estimate:** ~45k tokens
**Predecessors:** TASK-001-001
**Successors:** TASK-001-003
**Parallel:** no

**Acceptance Criteria:**
- [x] `runPrompt` opens the `promptPickerModel` TUI before launching Claude
- [x] If result is cancelled, exits cleanly with no error
- [x] For "Plain": launches `claude [--dangerously-skip-permissions] [--model ...]` (no skill arg)
- [x] For skill options: launches `claude [--dangerously-skip-permissions] [--model ...] "/maggus:skill-name description"` — reuses `launchInteractive` with a new `skipPermissions bool` parameter added to its signature
- [x] `--dangerously-skip-permissions` is passed when `SkipPermissions == true`
- [x] `ensureMaggusPlugin()` is called before launching any non-plain skill
- [x] Usage tracking works for all modes: plain uses `usage_prompt.jsonl`, skill modes use their respective `usage_<skill>.jsonl` filenames
- [x] The `--model` CLI flag on `promptCmd` still works
- [x] `go build ./...` and `go test ./...` pass

### TASK-001-003: Remove old standalone commands and update the menu

**Description:** As a developer, I want the old `plan`, `vision`, and `architecture` commands removed and the menu cleaned up, so there are no duplicate or broken entry points.

**Token Estimate:** ~20k tokens
**Predecessors:** TASK-001-002
**Successors:** none
**Parallel:** no
**Model:** haiku

**Acceptance Criteria:**
- [ ] `planCmd`, `visionCmd`, `architectureCmd` and their `init()` registrations removed from `cmd/plan.go`
- [ ] `runSkillCommand` helper removed from `cmd/plan.go` (functionality now lives in `runPrompt`)
- [ ] Shared utilities (`launchInteractive`, `ensureMaggusPlugin`, `extractSkillUsage`, `SessionInfo`, plugin helpers) kept in `cmd/plan.go` or moved to a suitable file
- [ ] Menu items for `vision`, `architecture`, `plan` removed from `cmd/menu.go`; their keyboard shortcuts freed
- [ ] `maggus prompt` menu item kept; its shortcut remains `o` (or updated if a better letter is freed)
- [ ] `go build ./...` and `go test ./...` pass
- [ ] Running `maggus --help` no longer shows `plan`, `vision`, or `architecture` as subcommands

## Task Dependency Graph

```
TASK-001-001 ──→ TASK-001-002 ──→ TASK-001-003
```

| Task | Estimate | Predecessors | Parallel | Model |
|------|----------|--------------|----------|-------|
| TASK-001-001 | ~55k | none | no | — |
| TASK-001-002 | ~45k | 001 | no | — |
| TASK-001-003 | ~20k | 002 | no | haiku |

**Total estimated tokens:** ~120k

## Functional Requirements

- FR-1: `maggus prompt` must open a TUI picker before launching any Claude session
- FR-2: The picker must offer exactly 7 options: Plain, Plan, Vision, Architecture, Bug report, Bryan: plan, Bryan: bug report
- FR-3: The description text input must be accessible from within the TUI; it must be hidden/disabled for the Plain option
- FR-4: `--dangerously-skip-permissions` must be passed to Claude when the skip-permissions toggle is `on`
- FR-5: The skip-permissions toggle must default to `on`
- FR-6: All non-plain skill options must call `ensureMaggusPlugin()` before launching
- FR-7: Usage tracking must be written per-mode to the appropriate `.maggus/usage_<mode>.jsonl` file
- FR-8: `maggus plan`, `maggus vision`, `maggus architecture` must no longer exist as CLI subcommands
- FR-9: The menu must not reference removed commands

## Non-Goals

- No changes to `bryan-plan` or `bryan-bugreport` skill definitions themselves
- No new per-repo skill configuration (which skills appear is hardcoded in the picker)
- No persistent memory of last-used skill or description
- No changes to the `work` loop or any other command

## Technical Considerations

- `launchInteractive` needs a `skipPermissions bool` parameter added — update all callers
- Skill invocation strings follow the `/maggus:skill-name` format (confirm against what Claude Code actually accepts; currently `plan.go` uses `/maggus-plan` without namespace — verify and align)
- The description input in the TUI should use `charmbracelet/bubbles/textinput` (already a transitive dep via bubbletea)
- Tab-order focus management: list → description input → toggle → back to list (cycle)

## Open Questions

None.
