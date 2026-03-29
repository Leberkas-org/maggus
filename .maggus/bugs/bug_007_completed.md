<!-- maggus-id: 40b41166-44f8-42a1-b9a1-07f935122682 -->
# Bug: Log message in progress tab not truncated to account for timestamp width

## Summary

In the Progress tab's compact tool list, log messages are truncated using a fixed `contentWidth-2` calculation that ignores the icon, type label, separators, and timestamp. This causes the composed line to exceed the terminal width and wrap unexpectedly.

## Steps to Reproduce

1. Run `maggus work` with a narrow-ish terminal (e.g. 100 columns)
2. Watch the Progress tab compact tool list while Claude executes tasks with long tool descriptions
3. Observe: some lines wrap to the next line, pushing the timestamp down

## Expected Behavior

Each compact tool line should fit on a single line. The description should be truncated so that the entire composed string (icon + type + description + timestamp) fits within the terminal width.

## Root Cause

`renderProgressTab()` in `src/internal/runner/tui_render.go:690` truncates `entry.Description` to `contentWidth-2`, where `contentWidth = w - 11`:

```go
// Line 649
contentWidth := w - 11

// Line 690
desc := styles.Truncate(entry.Description, contentWidth-2)

// Lines 691-695
toolLines[i] = fmt.Sprintf("  %s %s: %s  %s",
    icon,
    cyanStyle.Render(entry.Type),
    blueStyle.Render(desc),
    grayStyle.Render(ts))
```

The composed format string contains fixed and variable overhead that is not accounted for:

| Element | Width |
|---------|-------|
| `"  "` indent | 2 |
| `icon` | ~2 (ANSI + glyph, must be measured) |
| `" "` separator | 1 |
| `entry.Type` (e.g. `"Write"`) | variable, ~4-10 chars |
| `": "` | 2 |
| `"  "` before timestamp | 2 |
| timestamp `"15:04:05"` | 8 |

With a 100-column terminal, `desc` is truncated to `87` chars, but the fixed overhead easily adds another 15–20+ chars, producing lines well over 100 columns.

The detail panel (lines 500–515 of the same file) implements the correct pattern: measure every fixed element with `lipgloss.Width()`, sum them, subtract from `w`, then truncate `desc` to the remainder:

```go
iconW := lipgloss.Width(styledIcon)
tsW := 8
fixedCols := iconW + 1 + 1 + tsW + emojiMargin
maxDesc := w - fixedCols
desc = styles.Truncate(desc, maxDesc)
```

The progress tab compact list needs the same measurement-based approach.

## User Stories

### BUG-007-001: Fix description truncation in progress tab compact tool list

**Description:** As a user, I want compact tool lines in the Progress tab to always fit on one line so the output stays readable at any terminal width.

**Acceptance Criteria:**
- [x] `entry.Description` is truncated to `w` minus the measured widths of the icon, type label, fixed separators, and timestamp
- [x] No compact tool line wraps to the next line at any reasonable terminal width (≥ 40 columns)
- [x] The timestamp remains visible and right-aligned (or at least on the same line)
- [x] No regression in the detail panel rendering (lines 500–525)
- [x] `go vet ./...` and `go test ./...` pass
