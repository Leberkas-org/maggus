# Bug: 2x countdown timer is static in the work TUI

## Summary

The 2x expires countdown in the work screen shows the initial value from when `maggus work` started and never updates. It should count down every second, matching the behavior in the main menu.

## Steps to Reproduce

1. Run `maggus work` while claude 2x mode is active
2. Observe the 2x countdown in the header/banner area
3. Wait 30+ seconds
4. The countdown value does not change — it's frozen at the initial value

## Expected Behavior

The 2x countdown should decrement every second, matching the main menu behavior. When 2x expires, the banner should disappear.

## Root Cause

The work TUI receives the 2x status once at initialization and never refreshes it.

**In `src/cmd/work.go` (lines 118-133):** The initial 2x status is fetched synchronously and embedded in `BannerInfo`:
```go
twoXStatus := claude2x.FetchStatus()
if twoXStatus.Is2x {
    banner.TwoXExpiresIn = twoXStatus.TwoXWindowExpiresIn
}
```

**In `src/internal/runner/tui.go` (lines 201-288):** The `TUIModel.Update()` method has no case handler for `claude2xTickMsg`. It never receives tick messages to refresh the countdown.

**In `src/internal/runner/tui_render.go` (lines 91-94):** The render always uses the static `m.banner.TwoXExpiresIn` value, which never changes after creation.

**Contrast with the working menu implementation:** The menu model (`src/cmd/menu.go`, lines 272-283) handles both `claude2xResultMsg` and `claude2xTickMsg`, calling `fetch2xAndUpdate()` every second to refresh the countdown and scheduling the next tick.

The `next2xTick()` and `fetch2xAndUpdate()` functions in `src/cmd/claude2x_tick.go` already exist and work correctly — they're just never used by the work TUI.

## User Stories

### BUG-001-001: Add 2x countdown tick to the work TUI

**Description:** As a user running `maggus work`, I want the 2x countdown to update every second so I can see how much 2x time remains while tasks are running.

**Acceptance Criteria:**
- [x] The work TUI's `Init()` or model setup schedules a `claude2xTickMsg` when 2x mode is active
- [x] `TUIModel.Update()` handles the tick message, refreshes `banner.TwoXExpiresIn` (and `banner.Is2x`), and schedules the next tick
- [x] The countdown decrements every second in the work view header, matching the menu behavior
- [x] When 2x expires during a run, the banner updates accordingly (removes countdown or shows expired)
- [x] No regression in existing work TUI behavior (tabs, spinner, tool display, summary screen)
- [x] `go vet ./...` and `go test ./...` pass
