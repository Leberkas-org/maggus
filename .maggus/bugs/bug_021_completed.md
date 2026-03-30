<!-- maggus-id: 05fe27a8-7b63-495e-a87f-3cebb3ac479e -->
# Bug: Update, config, and prompt screens render as small box instead of fullscreen

## Summary

After the screen router refactoring, the update, config, and prompt picker screens render in a small `styles.Box` instead of fullscreen. The window size seeded in `navigateTo` is silently discarded due to a value-receiver discard bug.

## Steps to Reproduce

1. Run `maggus` to open the TUI
2. Navigate to the config screen, prompt picker, or update screen
3. Observe: screen renders as a small box, not fullscreen

## Expected Behavior

All three screens should render fullscreen, the same as they did when run as standalone `tea.NewProgram` instances before the app router refactoring.

## Root Cause

`navigateTo` in `src/cmd/app_model.go` (line 239) is a **value receiver** method. After calling `m.initScreen(target)` (which sets the sub-model with `width=0, height=0`), it attempts to seed the stored window size by calling `forwardToActive`. However, the returned updated model is discarded:

```go
// Line 251 — the updated appModel (with sub-model dimensions set) is thrown away
_, sizeCmd := m.forwardToActive(sizeMsg)
```

`forwardToActive` (line 183) is also a value receiver. It updates the sub-model inside its local copy of `m` and returns that updated `m` as `tea.Model`. Since the return is ignored with `_`, the sub-model's `width` and `height` remain `0` in the `m` that `navigateTo` returns.

All three affected screens have a guard in `View()` that falls back to `styles.Box` when `width == 0 || height == 0`:

- `src/cmd/config_view.go` line 158–161
- `src/cmd/prompt_picker.go` line 296–299
- `src/cmd/update.go` line 495–498

So every render hits the fallback path.

**Why this didn't happen before the refactor:** Each screen previously ran as its own `tea.NewProgram`, which always delivers a `tea.WindowSizeMsg` as the first message before any `View()` call.

## User Stories

### BUG-021-001: Capture updated model from forwardToActive in navigateTo

**Description:** As a user, I want the config, prompt picker, and update screens to render fullscreen so I can use them comfortably in the terminal.

**Acceptance Criteria:**
- [x] In `navigateTo`, the return value of `forwardToActive(sizeMsg)` is captured and the local `m` is updated from it, so the sub-model's `width` and `height` are correctly set before returning
- [x] Config screen renders fullscreen when navigated to from the menu
- [x] Prompt picker screen renders fullscreen when navigated to from the menu
- [x] Update screen renders fullscreen when navigated to from the menu
- [x] No regression in menu, status, or repos screen rendering
- [x] `go vet ./...` and `go test ./...` pass
