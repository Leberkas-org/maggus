# Bug: Prompt picker invokes Claude non-interactively, and skill list lacks group separators

## Summary

Two issues with the `maggus prompt` picker. First and critical: when a skill is selected (e.g. `/maggus-bugreport`), Claude Code is launched in non-interactive mode instead of an interactive REPL. Second: the skill list has no visual separators between the `open console`, `/maggus-*`, and `/bryan-*` groups.

## Steps to Reproduce

**Bug 1 — Non-interactive invocation:**
1. Run `maggus prompt`
2. Select any skill (e.g. `/maggus-bugreport`)
3. Enter a description and press Enter
4. Observe: Claude Code runs headless/non-interactive instead of opening an interactive REPL session

**Bug 2 — Missing separators:**
1. Run `maggus prompt`
2. Look at the skill list
3. Observe: all items render as a flat list with no visual grouping between `open console`, the `/maggus-*` skills, and the `/bryan-*` skills

## Expected Behavior

**Bug 1:** After picker confirmation, Claude Code should launch in full interactive mode (REPL) — stdin/stdout/stderr connected to the terminal — with the selected skill command sent as the initial message.

**Bug 2:** The skill list should have visible separators (e.g. `--- maggus ---` and `--- bryan ---`) between groups, making the picker easier to navigate.

## Root Cause

### Bug 1 — Non-interactive invocation

`launchInteractive` in `src/cmd/plan.go:108–110` appends the skill command as a positional argument:

```go
if prompt != "" {
    args = append(args, prompt)
}
cmd := exec.Command(path, args...)
```

This produces a command like:

```
claude --dangerously-skip-permissions --model claude-sonnet-4-6 "/maggus-bugreport some description"
```

When Claude Code CLI receives a positional string argument it runs in non-interactive (print/headless) mode — it does not open a REPL. The fix is to deliver the initial message differently, e.g. by writing it to stdin before handing stdin over to the user (using `io.MultiReader` to prepend the skill line, then pipe remaining os.Stdin), or by whatever mechanism the Claude Code CLI supports for pre-filling the interactive session.

### Bug 1b — `skillMappings` key mismatch (related, will cause immediate crash)

The current uncommitted changes to `src/cmd/prompt_picker.go` renamed the skill labels from `"Plain"`, `"Plan"`, `"Bug report"`, etc. to `"open console"`, `"/maggus-plan"`, `"/maggus-bugreport"`, etc.

However, `skillMappings` in `src/cmd/prompt.go:33–41` still uses the old keys:

```go
var skillMappings = map[string]skillMapping{
    "Plain":             {...},
    "Plan":              {...},
    "Bug report":        {...},
    ...
}
```

`result.Skill` is now set to the new label (e.g. `"/maggus-bugreport"`) at `src/cmd/prompt_picker.go:213`, so `skillMappings[result.Skill]` always returns `ok = false`, and `runPrompt` immediately returns `"unknown skill: /maggus-bugreport"`. The `skillMappings` keys must be updated to match the new labels.

### Bug 2 — Missing separators

`defaultSkills` in `src/cmd/prompt_picker.go:35–43` is a flat `[]skillOption` slice with no concept of separators. The `View()` function at line 239 renders every entry the same way. To add separators, either a `separator bool` field needs to be added to `skillOption` and rendered as non-selectable rows, or separator entries need to be skipped during cursor navigation.

## User Stories

### BUG-001-001: Fix non-interactive invocation in launchInteractive

**Description:** As a user, I want `maggus prompt` to open a full interactive Claude Code REPL session so I can have a back-and-forth conversation using the selected skill.

**Acceptance Criteria:**
- [x] After selecting a skill and entering a description in the picker, Claude Code opens in interactive (REPL) mode with stdin/stdout/stderr connected to the terminal
- [x] The selected skill command (e.g. `/maggus-bugreport some description`) is delivered as the first message of the interactive session
- [x] Selecting `open console` (no skill) still opens plain interactive mode with no pre-filled message
- [x] No regression for the `--dangerously-skip-permissions` and `--model` flags
- [x] `go vet ./...` and `go test ./...` pass

### BUG-001-002: Update skillMappings keys to match new picker labels

**Description:** As a user, I want selecting any skill in the picker to actually launch without a crash so I can use the prompt command at all.

**Acceptance Criteria:**
- [ ] `skillMappings` in `src/cmd/prompt.go` uses keys that match the labels in `defaultSkills` (`"open console"`, `"/maggus-plan"`, `"/maggus-vision"`, `"/maggus-architecture"`, `"/maggus-bugreport"`, `"/bryan-plan"`, `"/bryan-bugreport"`)
- [ ] Selecting any skill in the picker no longer returns `"unknown skill: ..."` error
- [ ] `go vet ./...` and `go test ./...` pass

### BUG-001-003: Add group separators to the skill picker list

**Description:** As a user, I want the skill list to show visual separators between the `open console` entry, the `/maggus-*` skills, and the `/bryan-*` skills so I can quickly navigate and identify skill groups.

**Acceptance Criteria:**
- [ ] A visual separator (e.g. `─── maggus ────`) appears before the first `/maggus-*` skill and after the last one (before `/bryan-*` skills)
- [ ] Separator rows are not selectable — the cursor skips over them during up/down navigation
- [ ] Separator rows are rendered in a muted/dimmed style distinct from skill labels
- [ ] `go vet ./...` and `go test ./...` pass
