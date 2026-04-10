<!-- maggus-id: add712a6-6f41-44b6-8659-2a7d79ff96b4 -->
# Bug: Task detail viewport is 1 line too tall, corrupting the footer

## Summary

The task detail view renders one line too many, pushing the footer out of the box boundary. The viewport height is set to `innerH - 1` but `viewport.View()` returns a trailing newline that inflates the line count, leaving no room for the footer.

## Steps to Reproduce

1. Run `maggus list` (or any command that opens the task detail view)
2. Open a task detail with `enter`
3. Observe the footer line — it is displaced or overwrites the bottom border

## Expected Behavior

The footer line renders cleanly inside the box, 1 line above the bottom border.

## Root Cause

In `src/cmd/tasklist.go:132` and `:151`, the detail viewport is created with height `h - 1` (where `h = innerH` from `styles.FullScreenInnerSize`):

```go
c.detailViewport = viewport.New(w, h-1)  // line 132
c.detailViewport.Height = h - 1           // line 151
```

`FullScreenLeftColor` in `src/internal/tui/styles/styles.go:152` calculates content line count as:

```go
contentLines := strings.Count(content, "\n") + 1
```

Bubble Tea's `viewport.View()` returns the visible lines **with a trailing newline**. For a viewport of height `innerH - 1`, `View()` returns `innerH - 2` separating newlines plus 1 trailing newline = `innerH - 1` newlines total. This makes `contentLines = innerH` instead of the expected `innerH - 1`.

The gap calculation at line 159 then becomes:

```
gap = innerH - contentLines - footerLineCount
    = innerH - innerH - 1
    = -1  →  clamped to 0
```

`body` ends up as `content` (effectively `innerH` lines due to trailing newline) plus `footer` (1 line) = `innerH + 1` lines rendered inside a `Box.Height(innerH)` — the footer overflows into the border row.

The fix is to set viewport height to `h - 2` so the line-count math lands correctly:

```
contentLines = (innerH-2) lines + trailing \n = innerH-1
gap = innerH - (innerH-1) - 1 = 0
body = innerH-1 content lines + 1 footer line = innerH  ✓
```

## User Stories

### BUG-024-001: Fix detail viewport height off-by-one in tasklist.go

**Description:** As a user, I want the task detail footer to render correctly so the view doesn't overflow the box boundary.

**Acceptance Criteria:**
- [x] `openDetail()` in `tasklist.go:132` creates viewport with `h-2` instead of `h-1`
- [x] `HandleResize()` in `tasklist.go:151` sets `detailViewport.Height = h - 2` instead of `h - 1`
- [x] Footer line renders cleanly at the bottom of the detail box without overflowing
- [x] No regression in scrollable detail content (scroll indicators, PgUp/PgDn)
- [x] No regression in the status split-pane detail view (`resizeTab2DetailViewport` is unaffected — it uses a separate code path)
- [x] `go vet ./...` and `go test ./...` pass
