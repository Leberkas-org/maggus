<!-- maggus-id: 5f01d8eb-13bf-44d7-b4ad-7db67bcf6570 -->
# Bug: UnblockCriterion fails for `[~]`-checkbox criteria

## Summary

Pressing `b` to unblock a task shows "criterion line not found" when the blocked criterion was written by the agent with a `[~]` checkbox (`- [~] ⚠️ BLOCKED: ...`). The `UnblockCriterion` function always searches for `- [ ] ` + c.Text, but agent-blocked criteria use `[~]` — so the search never finds the line.

## Steps to Reproduce

1. Run a task that has an unverifiable acceptance criterion (e.g., one requiring a browser test that depends on a prior task).
2. The agent follows the prompt instruction and marks the criterion as:
   `- [~] ⚠️ BLOCKED: <original criterion text> — <reason>`
3. In the `maggus status` TUI, navigate to that task.
4. Press `b` to unblock.
5. Observe: `error: criterion line not found in feature_NNN.md: ⚠️ BLOCKED: ...`

## Expected Behavior

Pressing `b` removes the `⚠️ BLOCKED:` prefix (and `[~]` → `[ ]` checkbox) so the task becomes runnable again.

## Root Cause

The agent prompt (`src/internal/prompt/prompt.go:90`) instructs the agent to write truly-blocked criteria as:

```
- [~] ⚠️ BLOCKED: <original criterion text> — <reason>
```

The parser (`src/internal/parser/parser.go:348–361`) correctly reads `[~]` criteria and sets `Blocked: true, Checked: true`, with `c.Text` containing the full `⚠️ BLOCKED: ...` string.

`handleUnblockAll` (`src/cmd/status_update.go:1068`) collects all criteria where `c.Blocked == true`, which includes `[~]` criteria.

`UnblockCriterion` (`src/internal/parser/parser.go:609`) then builds:

```go
oldLine := "- [ ] " + c.Text
```

This constructs `- [ ] ⚠️ BLOCKED: ...`, but the file contains `- [~] ⚠️ BLOCKED: ...`. The `strings.Contains` check at line 626 fails, returning the "criterion line not found" error.

`ResolveCriterion` (`src/internal/parser/parser.go:636`) has the same flaw.

## User Stories

### BUG-054-001: Fix UnblockCriterion and ResolveCriterion to handle `[~]` checkboxes

**Description:** As a user, I want to press `b` to unblock a `[~]`-type blocked criterion so that the task becomes runnable after its dependency is resolved.

**Acceptance Criteria:**
- [x] `UnblockCriterion` searches for both `- [ ] ` + c.Text and `- [~] ` + c.Text in the file
- [x] When `[~]` form is matched, the replacement changes it to `- [ ] ` + newText (i.e. checkbox changes from `[~]` to `[ ]` and `⚠️ BLOCKED:` / `BLOCKED:` prefix is stripped)
- [x] `ResolveCriterion` applies the same fix (searches for both `[ ]` and `[~]` forms)
- [x] Existing behaviour for plain `[ ]` blocked criteria is unchanged
- [x] A test in `internal/parser/parser_test.go` or the stores tests covers unblocking a `[~] ⚠️ BLOCKED:` criterion
- [x] No regression in related functionality
- [x] `go vet ./...` and `go test ./...` pass
