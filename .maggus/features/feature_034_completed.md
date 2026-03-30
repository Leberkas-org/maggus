<!-- maggus-id: afa92ae8-122e-4ec0-9379-38e7fb1b1f67 -->
# Feature 034: Highlighted Tool Name in Output Log

## Introduction

Right now the tool output log shows each entry as a single blue string like `📖 Read: C:\path\to\file.go   11:37:37`. The tool name and its argument are concatenated inside `DescribeToolUse` before being stored in `SnapshotToolEntry.Description`, making them impossible to style independently.

This feature separates the two concerns: `Description` will hold only the argument (path, command, pattern), and the renderer will display the tool name as `[Read]` in blue and the argument in slightly muted white — giving the log better visual structure.

## Goals

- Render tool name as `[Read]` in blue, clearly distinct from the argument
- Render the argument in slightly muted white so it recedes without disappearing
- Remove the redundant `"TypeName: "` prefix from `DescribeToolUse` since `Type` is already a separate field
- Keep width/truncation calculations correct after the layout change

## Tasks

### TASK-034-001: Strip tool-name prefix from DescribeToolUse
**Description:** As a developer, I want `DescribeToolUse` to return only the argument string (path, command, pattern, etc.) so that the description field no longer redundantly embeds the tool name.

**Token Estimate:** ~15k tokens
**Predecessors:** none
**Successors:** TASK-034-002
**Parallel:** no

**Acceptance Criteria:**
- [x] `DescribeToolUse` returns only the argument part for all cases (e.g. `"C:\path\to\file.go"` instead of `"Read: C:\path\to\file.go"`)
- [x] The MCP case returns only `"toolname"` (or the most useful part), not `"MCP servername: toolname"`
- [x] The fallback (unknown tool) still returns the tool name itself as a reasonable description
- [x] `go test ./internal/agent/...` passes

### TASK-034-002: Render [Type] in blue and description in muted white
**Description:** As a user, I want the tool log to show `[Read] C:\path\to\file.go` with the bracketed tool name in blue and the argument in slightly muted white, so I can scan tool types at a glance.

**Token Estimate:** ~25k tokens
**Predecessors:** TASK-034-001
**Successors:** none
**Parallel:** no

**Acceptance Criteria:**
- [x] Each tool log line renders as `[TypeName]` in blue (`statusBlueStyle`) followed by a space and the argument in a slightly muted/dim white style
- [x] The width calculation in `renderSnapshotInPane` accounts for the `[TypeName]` bracket width when computing `maxDesc` for truncation — so long paths still truncate correctly
- [x] The existing `icon` (emoji) is still shown before `[TypeName]`
- [x] `go test ./cmd/...` passes

## Task Dependency Graph

```
TASK-034-001 ──→ TASK-034-002
```

| Task | Estimate | Predecessors | Parallel | Model |
|------|----------|--------------|----------|-------|
| TASK-034-001 | ~15k | none | no | — |
| TASK-034-002 | ~25k | 001 | no | — |

**Total estimated tokens:** ~40k

## Functional Requirements

- FR-1: `SnapshotToolEntry.Description` must contain only the argument, never the tool name prefix
- FR-2: The log line format must be: `  <icon> [TypeName] <argument>   <timestamp>`
- FR-3: `[TypeName]` must be rendered in blue (`statusBlueStyle`)
- FR-4: The argument must be rendered in a slightly muted/dim white style (not the same blue as the type)
- FR-5: Line truncation must still work — `maxDesc` must exclude the width of `[TypeName] ` when computing available space

## Non-Goals

- No changes to the icon/emoji mapping
- No changes to any other pane or tab — only the Output tab tool list
- No changes to how the timestamp is rendered

## Technical Considerations

- `status_rightpane.go:183–194` is the sole rendering site — width arithmetic needs updating to subtract `lipgloss.Width("["+entry.Type+"] ")` from the available description space
- `DescribeToolUse` is only called in one place (`claude.go:117`); no other callers exist
- The `messages_test.go` `TestToolMsg` test sets `Description` directly and is unaffected by this change
