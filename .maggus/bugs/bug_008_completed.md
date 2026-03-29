<!-- maggus-id: dfa74672-c5bc-4482-aad6-f237891019e9 -->
# Bug: ESC closes instead of cancelling the "Stop daemon?" prompt in main menu

## Summary

When the "Stop daemon?" confirmation prompt is shown in the main menu, pressing ESC exits the application (without stopping the daemon) instead of returning to the main menu. ESC should cancel the prompt. Additionally, the prompt should offer `d` as an explicit detached-exit option, consistent with the status view's exit overlay.

## Steps to Reproduce

1. Start the daemon so it is running
2. Run `maggus` (main menu)
3. Press `q` or select "exit" to trigger the "Stop daemon?" prompt
4. Press `ESC`
5. Observe: the application exits without stopping the daemon

## Expected Behavior

- `ESC` — cancel the prompt and return to the main menu
- `y` — stop the daemon and exit
- `n` or `d` — exit without stopping the daemon (detached)

## Root Cause

In `src/cmd/menu_update.go:326`, `tea.KeyEscape` is grouped with `tea.KeyEnter` and treated as the default "N" action (exit without stopping):

```go
case tea.KeyEnter, tea.KeyEscape:
    // Default is N — exit without stopping the daemon.
    m.quitting = true
    return m, tea.Quit
```

To cancel the prompt, `confirmStopDaemon` must be set back to `false` and the model returned without quitting. The view text at `src/cmd/menu_view.go:149` also reflects the wrong behavior (`N/enter/esc: exit without stopping`) and must be updated to show the correct key hints.

## User Stories

### BUG-008-001: ESC cancels the "Stop daemon?" prompt and returns to main menu

**Description:** As a user, I want pressing ESC on the "Stop daemon?" prompt to cancel and return me to the main menu so that I don't accidentally exit the application.

**Acceptance Criteria:**
- [x] Pressing `ESC` in the "Stop daemon?" prompt sets `confirmStopDaemon = false` and does NOT quit
- [x] Pressing `enter` in the "Stop daemon?" prompt also cancels (same as ESC) — it should not exit
- [x] Pressing `y`/`Y` stops the daemon and exits (unchanged)
- [x] Pressing `n`/`N` exits without stopping the daemon (unchanged)
- [x] Pressing `d`/`D` exits without stopping the daemon (detached, same as `n`)
- [x] The prompt hint text in `menu_view.go` is updated to reflect: `y: stop and exit / n/d: exit detached / esc: cancel`
- [x] Existing tests in `menu_test.go` are updated to match new ESC/enter behavior
- [x] New tests cover: ESC cancels prompt, enter cancels prompt, `d` exits detached
- [x] No regression in `y` / `n` behavior
- [x] `go vet ./...` and `go test ./...` pass
