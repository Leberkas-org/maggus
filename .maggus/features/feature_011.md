<!-- maggus-id: ead56b5f-e31c-4115-9e33-f19277d78c85 -->
# Feature 011: Multiline Log Message Support in Status Right Panel

## Introduction

The Output tab's plain log view in the status right panel renders each log entry as a single line. However, log entries — particularly those with `event: "output"` — can contain embedded newline characters. These are not detected, causing the entry to spill across multiple visual lines while the layout only budgets one line per entry. The result is that log content overflows the available area, displacing or hiding the scroll indicator and corrupting the visual line count.

### Architecture Context

- **Components touched:** `cmd/status_rightpane.go` (rendering of plain log in pane), `cmd/status_runlog.go` (`formatLogLine`)
- **No new components or patterns introduced**
- **Model state unchanged:** `m.logLines` continues to hold raw JSONL strings; `m.logScroll` continues to index by log entry. Expansion to visual lines happens at render time only.

## Goals

- Multiline log entries are expanded into multiple visual lines during rendering
- The available-lines budget is consumed per visual line, not per log entry
- `styles.Truncate` is applied per sub-line so no line overflows horizontally
- The scroll indicator (e.g. `[1-12 of 30]`) continues to count in log entries (user-visible, stable)
- No visual regressions for single-line entries

## Tasks

### TASK-011-001: Expand multiline log entries in renderPlainLogInPane
**Description:** As a user viewing the Output tab, I want multiline log messages to expand across multiple display lines so that the full content is visible and the panel layout stays intact.

**Token Estimate:** ~30k tokens
**Predecessors:** none
**Successors:** none
**Parallel:** no

**Acceptance Criteria:**
- [ ] In `renderPlainLogInPane` (status_rightpane.go), after calling `formatLogLine(line)`, split the result on `\n` to obtain sub-lines
- [ ] Each sub-line is individually passed through `styles.Truncate(subLine, width-2)` before writing
- [ ] A `remaining` counter (initialized to `available`) tracks visual lines consumed; it is decremented once per sub-line rendered; rendering stops when `remaining <= 0`
- [ ] Log entries that begin at `m.logScroll` are rendered starting from sub-line 0 of that entry (no partial-entry mid-scroll; the scroll granularity stays at entry level)
- [ ] Empty sub-lines produced by a trailing `\n` are skipped (not rendered as blank lines)
- [ ] The scroll indicator line `[start+1 – end of total]` still counts in terms of log entries, not visual lines, and is only shown when `len(m.logLines) > available` (available in terms of entries, capped at visual budget)
- [ ] `go build ./...` passes
- [ ] `go vet ./...` passes
- [ ] Existing tests in `cmd/` pass (`go test ./cmd/...`)

## Task Dependency Graph

```
TASK-011-001
```

| Task | Estimate | Predecessors | Parallel | Model |
|------|----------|--------------|----------|-------|
| TASK-011-001 | ~30k | none | no | — |

**Total estimated tokens:** ~30k

## Functional Requirements

- FR-1: When a formatted log line contains one or more `\n` characters, each segment between newlines is treated as a separate visual line
- FR-2: Each visual line is truncated to `width-2` characters independently before being written to the output buffer
- FR-3: The rendering loop tracks a visual-line budget (`remaining`); once exhausted, no further sub-lines or entries are rendered
- FR-4: Empty segments (empty string after splitting on `\n`) are silently skipped and do not consume budget
- FR-5: Scroll position (`m.logScroll`) and the scroll indicator continue to reference log entry indices, not visual line indices

## Non-Goals

- No changes to `m.logScroll` semantics (it stays entry-indexed)
- No changes to `formatLogLine` itself — expansion is purely a render-time concern
- No changes to the rich snapshot view (`renderSnapshotInPane`) — tool entries are single-line by construction
- No changes to log loading (`readLastNLogLines`) or model state

## Technical Considerations

- `styles.Truncate` must be called on each sub-line individually; calling it on the full multiline string would silently truncate at the wrong position (or not at all, depending on implementation)
- The `available` variable computed at the top of `renderPlainLogInPane` (currently `height - 4`) represents visual lines. After this fix, the loop must consume from this budget per visual line rendered, not per entry.
- Because scroll granularity stays at entry level, a single very long entry (many sub-lines) could consume the entire budget. This is acceptable and correct behaviour.
- The `outputTabScrollableLines()` helper (status_rightpane.go:92) returns an entry-count estimate used to decide when to show the scroll indicator. Since it can't know how many visual lines entries will expand to without pre-rendering, leave it unchanged. The scroll indicator threshold can remain entry-count based.

## Success Metrics

- A log entry whose `text` field contains `\n` renders all its lines inside the panel without overflowing the allocated height
- No blank-line gaps appear between entries that were previously single-line
- The scroll indicator position is not displaced by multiline entries
