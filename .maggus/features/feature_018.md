<!-- maggus-id: 67145c17-20a0-4a35-ba6a-b6f1b30590d6 -->
# Feature 018: Right-align timestamps in the Output tab

## Introduction

In the `maggus status` Output tab, timestamps (`HH:MM:SS`) currently appear either
at the left side of each log line (plain log view) or loosely appended at the end of
tool-entry lines without true right-alignment (snapshot/live view). This makes the
output harder to scan: the timestamp competes visually with the main content.

This feature moves all timestamps in the Output tab to the right edge of the
terminal, right-aligned to the terminal width. The main content occupies the left
side; the timestamp floats to the right — identical to how modern log viewers and
terminal multiplexers present timestamped output.

### Architecture Context

- **Components involved:** `src/cmd/status_runlog.go` (formatLogLine, renderSnapshotInPane,
  renderPlainLogInPane), `src/internal/tui/styles/` (shared style helpers).
- **Vision alignment:** Clean, information-dense TUI output.
- **New patterns:** Introduces a `RightAlign(left, right string, width int) string`
  helper in `src/internal/tui/styles/` for reuse across all TUI views.

## Goals

- Timestamps are visually separated from content and anchored to the right edge.
- The change covers both the **snapshot view** (daemon running) and the **plain log
  view** (daemon stopped / historical log).
- A reusable helper handles the right-alignment logic so other views can adopt it.

## Tasks

### TASK-018-001: Add RightAlign helper to styles package

**Description:** As a developer, I want a shared `RightAlign` helper in the styles
package so that any TUI view can right-align a secondary piece of text without
duplicating the padding logic.

**Token Estimate:** ~15k tokens
**Predecessors:** none
**Successors:** TASK-018-002, TASK-018-003
**Parallel:** yes — but trivially small; successors depend on it

**Acceptance Criteria:**
- [x] `RightAlign(left, right string, width int) string` is added to `src/internal/tui/styles/`
  (a suitable existing file or a new small file `align.go`).
- [x] The function computes `pad = width - lipgloss.Width(left) - lipgloss.Width(right)`.
  If `pad < 1` (no room), it returns `left` unchanged (timestamp is silently dropped rather than wrapping).
- [x] The function is pure (no side effects, no global state).
- [x] A unit test covers: normal case (pad > 0), tight fit (pad == 0), overflow (pad < 0), empty strings.
- [x] `go test ./internal/tui/styles/...` passes.

---

### TASK-018-002: Right-align timestamps in the snapshot tool list

**Description:** As a user, I want the `HH:MM:SS` timestamp in the live tool list
(Output tab, daemon running) to be right-aligned to the pane width so that
timestamps are easy to scan without competing with the tool description.

**Token Estimate:** ~20k tokens
**Predecessors:** TASK-018-001
**Successors:** none
**Parallel:** no — depends on TASK-018-001; can run in parallel with TASK-018-003 if 001 is done first

**Context:**
In `src/cmd/status_runlog.go`, `renderSnapshotInPane` builds each tool line as:

```go
toolLines[i] = fmt.Sprintf("  %s %s: %s  %s",
    icon,
    statusCyanStyle.Render(entry.Type),
    statusBlueStyle.Render(desc),
    statusDimStyle.Render(ts))
```

The timestamp `ts` sits at the end of the string but is not padded to the right edge.
The available width is `contentWidth` (= `width - 4`).

**Acceptance Criteria:**
- [ ] Each tool line is built using `styles.RightAlign(leftPart, tsStr, contentWidth)` (or
  equivalent), where `leftPart` is everything except the timestamp.
- [ ] The timestamp is rendered at the right edge of `contentWidth`.
- [ ] When the terminal is very narrow and there is no room for the timestamp, the line
  degrades gracefully (timestamp dropped, content preserved, no panic).
- [ ] `go build ./...` and `go test ./...` pass.

---

### TASK-018-003: Right-align timestamps in the plain log view

**Description:** As a user, I want the `HH:MM:SS` timestamp in the plain log view
(Output tab, daemon not running) to be right-aligned so that the format is
consistent with the live snapshot view.

**Token Estimate:** ~30k tokens
**Predecessors:** TASK-018-001
**Successors:** none
**Parallel:** yes — can run in parallel with TASK-018-002 once TASK-018-001 is complete

**Context:**
`formatLogLine` in `src/cmd/status_runlog.go` currently builds every line with the
timestamp on the left:

```go
tsStr := logTimestampStyle.Render(ts)
// ...
return fmt.Sprintf("%s%s %s %s", tsStr, taskID, toolTag, ...)
```

To right-align the timestamp, `formatLogLine` must be refactored to accept the
available `width int` and use `styles.RightAlign`. The call sites in
`renderPlainLogInPane` pass the already-known `width`.

**Acceptance Criteria:**
- [ ] `formatLogLine` signature changes to `formatLogLine(raw string, width int) string`.
- [ ] The timestamp is removed from the left and appended right-aligned using
  `styles.RightAlign`.
- [ ] All call sites of `formatLogLine` are updated to pass the correct width.
- [ ] When `width == 0` (unknown), the function falls back to the current left-aligned
  format so nothing breaks in headless / test contexts.
- [ ] `go build ./...` and `go test ./...` pass.

---

## Task Dependency Graph

```
TASK-018-001 ──→ TASK-018-002
             └──→ TASK-018-003
```

| Task         | Estimate | Predecessors | Parallel                        | Model |
|--------------|----------|--------------|----------------------------------|-------|
| TASK-018-001 | ~15k     | none         | yes (start immediately)          | haiku |
| TASK-018-002 | ~20k     | 001          | yes (with 003, after 001)        | —     |
| TASK-018-003 | ~30k     | 001          | yes (with 002, after 001)        | —     |

**Total estimated tokens:** ~65k

## Functional Requirements

- FR-1: In the snapshot tool list, each line must end with a right-aligned `HH:MM:SS`
  timestamp flush with the right edge of `contentWidth`.
- FR-2: In the plain log view, each log line must end with a right-aligned `HH:MM:SS`
  timestamp flush with the right edge of the pane width.
- FR-3: When terminal width is too narrow to fit both content and timestamp, the content
  is preserved and the timestamp is silently omitted (no wrapping, no panic).
- FR-4: `styles.RightAlign` must handle lipgloss-styled strings correctly by using
  `lipgloss.Width()` (which strips ANSI escape codes) for width calculations.

## Non-Goals

- No changes to the `maggus list` command.
- No changes to the `maggus work` runner spinner output.
- No changes to tab 2 (Item Details), tab 3 (Current Task), or tab 4 (Metrics).
- No new timestamp formats — `HH:MM:SS` stays as-is.

## Technical Considerations

- `lipgloss.Width(s)` must be used for all width calculations, not `len(s)`, because
  rendered strings contain invisible ANSI escape sequences.
- `contentWidth` in `renderSnapshotInPane` is `width - 4`; the timestamp should be
  right-aligned within that, not the full pane width.
- `renderPlainLogInPane` already truncates lines to `width-2`; after this change the
  right-alignment padding must be computed before truncation.

## Success Metrics

- Timestamps are visually anchored to the right edge at all terminal widths ≥ 60.
- No visual regression in existing tests.
- Side-by-side comparison: timestamp column is perfectly aligned across consecutive
  log lines.

## Open Questions

_(none)_
