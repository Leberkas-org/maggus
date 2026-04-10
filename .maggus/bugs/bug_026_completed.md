<!-- maggus-id: 11287e5b-0700-4c18-a7b8-3da1862e66cc -->
# Bug: Nerf state not propagated correctly to menu re-init and prompt subview

## Summary

Two related rendering bugs affect the nerf (rate-limit window) indicator. When navigating back to the main menu during nerfed hours, the border flickers cyan for a frame before turning red. The prompt subview always shows a cyan border regardless of nerf state, because it neither tracks `isNerfed` nor uses `ThemeColor`.

## Steps to Reproduce

**Flicker:**
1. Enter maggus during nerfed hours (weekday 13:00–19:00 UTC) — border is red
2. Navigate to any subscreen (e.g. prompt via `o`)
3. Press `q`/`esc` to return to the main menu
4. Observe: one or more frames where the border is cyan (Primary) before switching back to red

**Prompt subview color:**
1. Enter maggus during nerfed hours
2. Verify main menu border is red
3. Press `o` to open the prompt subview
4. Observe: border is cyan, not red

## Expected Behavior

- The main menu border should be red immediately on re-entry when nerfed; no cyan flash.
- The prompt subview border should match the active theme color: red when nerfed, cyan otherwise.

## Root Cause

**Flicker:** `newMenuModel()` at `src/cmd/menu_model.go:261` initializes `isNerfed` to its zero value (`false`). `Init()` then starts an async goroutine (via `tea.Batch`) that calls `claude2x.FetchStatus()` and delivers a `claude2xResultMsg`. Until that goroutine's message is processed, `View()` renders with `isNerfed = false` → `styles.Primary` (cyan) border. Since `claude2x.FetchStatus()` is a pure local time computation with no I/O, it can be called synchronously in `newMenuModel` to seed the correct state immediately.

The same gap affects `Init()`: the tick chain (`next2xTick()`) currently only starts after `claude2xResultMsg` is received (`menu_update.go:134-136`). If `isNerfed` is seeded eagerly, `Init()` must also start `next2xTick()` when `m.isNerfed` is already true at construction time.

**Prompt subview color:** `promptPickerModel` at `src/cmd/prompt_picker.go` has no `isNerfed` field, no `claude2xResultMsg`/`claude2xTickMsg` handlers in `Update()`, and no nerf fetch in `Init()`. `View()` at line 297 hardcodes `styles.Primary`:

```go
return styles.FullScreenColor(content, footer, m.width, m.height, styles.Primary)
```

Every other sub-screen (`statusModel`, `configModel`, `reposModel`, `updateModel`) follows the pattern of tracking `isNerfed` and calling `styles.ThemeColor(m.isNerfed)`. The prompt subview was never wired up.

## User Stories

### BUG-026-001: Seed nerf state eagerly in newMenuModel to eliminate border flicker

**Description:** As a user, I want the main menu border to immediately show the correct color when I return from a subscreen during nerfed hours, so there is no cyan flash before the red border appears.

**Acceptance Criteria:**
- [x] `newMenuModel` calls `claude2x.FetchStatus()` synchronously and sets `isNerfed` and `twoXExpiresIn` on the returned model
- [x] `Init()` starts `next2xTick()` when `m.isNerfed` is already true at construction time (in addition to the existing path via `claude2xResultMsg`)
- [x] Navigating back to the menu during nerfed hours shows the red border on the first rendered frame — no cyan flash visible
- [x] Navigating back outside nerfed hours still shows the cyan border correctly
- [x] No regression in existing nerf tick behavior (timer still counts down every second)
- [x] `go vet ./...` and `go test ./...` pass

### BUG-026-002: Wire nerf state into promptPickerModel so border respects ThemeColor

**Description:** As a user, I want the prompt subview border to turn red during nerfed hours, consistent with all other screens.

**Acceptance Criteria:**
- [x] `promptPickerModel` has an `isNerfed bool` field
- [x] `Init()` starts an async `claude2xResultMsg` fetch (matching the pattern used by `menuModel`, `configModel`, etc.)
- [x] `Update()` handles `claude2xResultMsg` and `claude2xTickMsg`, updating `isNerfed` and scheduling the next tick when nerfed
- [x] `View()` at line 297 uses `styles.ThemeColor(m.isNerfed)` instead of `styles.Primary`
- [x] `newPromptPickerModel` (or `initScreen`) seeds `isNerfed` synchronously via `claude2x.FetchStatus()` so the first frame renders with the correct border color
- [x] Prompt subview border is red during nerfed hours and cyan otherwise
- [x] No regression in prompt navigation, skill selection, or toggle behavior
- [x] `go vet ./...` and `go test ./...` pass
