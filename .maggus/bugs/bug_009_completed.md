<!-- maggus-id: 27384524-fb43-4e45-965e-a212dcdcba0e -->
# Bug: Duplicate "Read: " prefix in progress panel tool lines

## Summary

The progress panel (left-side compact tool list) shows `📖 Read: Read: /path` instead of `📖 Read: /path`. The tool type label is added twice — once by `DescribeToolUse` (which returns `"Read: /path"`) and again by the progress panel render code which prepends `styledType + ": "` before the description.

## Steps to Reproduce

1. Run `maggus work` on any project
2. Observe the left-side progress panel while a `Read` tool call is in progress
3. Notice the tool line reads: `📖 Read: Read: C:\path\to\file   19:55:20`

## Expected Behavior

The tool line should read: `📖 Read: C:\path\to\file   19:55:20`

## Root Cause

Two places independently add the `"TypeName: "` prefix:

**1. `src/internal/agent/claude.go:357`** — `DescribeToolUse` returns `"Read: /path"`:
```go
case "Read":
    return fmt.Sprintf("Read: %s", input.FilePath)
```
This value is stored as `entry.Description` in the `ToolMsg`.

**2. `src/internal/runner/tui_render.go:702`** — The progress panel format string prepends `styledType + ": "` again:
```go
leftPart := fmt.Sprintf("  %s %s: %s", icon, styledType, blueStyle.Render(desc))
```
Result: `  📖 Read: Read: /path`

The detail panel (`tui_render.go:516–519`) does NOT have this bug — it uses `entry.Description` directly without adding the type prefix, so it renders correctly as `📖 Read: /path`.

The fix is to remove `styledType` from the progress panel format string and update the `fixedCols` calculation accordingly, so the progress panel is consistent with the detail panel.

## User Stories

### BUG-009-001: Remove redundant type prefix from progress panel tool lines

**Description:** As a user, I want the progress panel to show `📖 Read: /path` (not `📖 Read: Read: /path`) so that tool lines are readable and not duplicated.

**Acceptance Criteria:**
- [x] Change `tui_render.go:702` format from `"  %s %s: %s"` to `"  %s %s"` (drop `styledType` argument)
- [x] Update the `fixedCols` calculation at `tui_render.go:696` to remove `typeW + 2` from the overhead (since `styledType: ` is no longer rendered)
- [x] Progress panel shows `📖 Read: /path` with no duplicate prefix
- [x] Same fix applies consistently — verify `Edit:`, `Write:`, `Bash:`, `Glob:`, `Grep:` tool lines also have no duplicate prefix
- [x] Detail panel rendering is unchanged and still correct
- [x] No regression in progress panel scroll, layout, or timestamp alignment
- [x] `go vet ./...` and `go test ./...` pass
