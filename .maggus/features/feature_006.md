# Feature 006: Auto-Work

## Introduction

When maggus is idling in the main menu and a new feature or bug file appears (or is updated) on disk, automatically trigger the `work` command — the same as if the user had selected it manually. The behavior is controlled by an `auto_work` setting in `.maggus/config.yml` and is configurable via the existing config TUI.

### Architecture Context

- **Vision alignment:** Reduces friction between planning and execution — the user creates a feature/bug and maggus picks it up without manual intervention.
- **Components involved:**
  - `internal/config/config.go` — Config struct and YAML parsing
  - `cmd/config.go` — Config editor TUI (Project tab)
  - `cmd/menu.go` — Main menu TUI model, file watcher integration, work dispatch
- **Existing patterns reused:**
  - `internal/filewatcher` already sends `featureSummaryUpdateMsg` to the menu on file changes — auto-work hooks into this same message
  - Work dispatch already done via `rootCmd.Find(["work", "--count", "999"])` in `root.go`
  - Config TUI already has cycling option rows — add one more row

## Goals

- When auto-work is `enabled`, immediately dispatch `maggus work` when workable tasks appear while idling in the main menu
- When auto-work is `delayed`, show a 5-second countdown in the menu with "press any key to cancel" before dispatching
- When auto-work is `disabled` (default), behave exactly as today — no change
- Setting is configurable via the existing Project tab in the config TUI and stored in `.maggus/config.yml`

## Tasks

### TASK-006-001: Add `AutoWork` field to Config struct

**Description:** As a developer, I want the config struct to support an `auto_work` field so that the setting can be persisted in `.maggus/config.yml` and read by the menu.

**Token Estimate:** ~15k tokens
**Predecessors:** none
**Successors:** TASK-006-002, TASK-006-003
**Parallel:** no — foundational; 002 and 003 must wait for this

**Acceptance Criteria:**
- [x] `Config` struct in `internal/config/config.go` has a new `AutoWork string` field with YAML tag `auto_work`
- [x] Valid values are `"disabled"`, `"enabled"`, `"delayed"`
- [x] Default value (when field is absent in YAML) is `"disabled"`
- [x] A const or typed string block documents the three valid values in the same file
- [x] `go vet ./...` passes

---

### TASK-006-002: Add Auto-work option row to config TUI

**Description:** As a user, I want to see and change the `auto_work` setting in the existing config editor so that I can toggle auto-work without editing the YAML file manually.

**Token Estimate:** ~30k tokens
**Predecessors:** TASK-006-001
**Successors:** none
**Parallel:** yes — can run alongside TASK-006-003

**Acceptance Criteria:**
- [x] A new option row labeled `"Auto-work"` is added to the **Project** tab in `cmd/config.go`, positioned logically near other behavioral settings (e.g. after "Worktree" or "Sound")
- [x] The row cycles through values: `disabled` → `enabled` → `delayed` → `disabled` (left/right arrow keys)
- [x] The current value is loaded from `config.Load()` when the config screen opens
- [x] Saving the config writes the selected value back to `.maggus/config.yml` under the `auto_work` key
- [x] Display labels are user-friendly: `"disabled"`, `"enabled"`, `"delayed (5s)"`
- [x] `go vet ./...` passes

---

### TASK-006-003: Implement auto-work trigger logic in main menu

**Description:** As a user, I want maggus to automatically start working when I'm in the main menu and new workable tasks arrive, according to my auto-work setting.

**Token Estimate:** ~75k tokens
**Predecessors:** TASK-006-001
**Successors:** none
**Parallel:** yes — can run alongside TASK-006-002
**Model:** opus

**Acceptance Criteria:**
- [ ] `menuModel` in `cmd/menu.go` loads config on init (or reuses existing summary-load flow) to read `AutoWork`
- [ ] When `featureSummaryUpdateMsg` fires and `AutoWork == "enabled"` and workable task count > 0: dispatch work immediately (same path as manually selecting "work" → `m.selected = "work"`, `m.args = []string{"--count", "999"}`)
- [ ] When `featureSummaryUpdateMsg` fires and `AutoWork == "delayed"` and workable task count > 0: enter a countdown state:
  - A countdown message is rendered in the menu view: `"Auto-work starting in Xs… press any key to cancel"` (counts 5→4→3→2→1)
  - A `tea.Tick` drives the countdown, firing once per second
  - Any key press while the countdown is active cancels it and returns to the normal menu
  - When the countdown reaches 0, dispatch work (same as "enabled" path)
- [ ] When `AutoWork == "disabled"`, file change events behave exactly as today (only update header count)
- [ ] Auto-work does NOT re-trigger if a countdown is already active (debounced)
- [ ] Auto-work does NOT trigger if workable task count is 0
- [ ] `go vet ./...` and `go test ./...` pass

---

## Task Dependency Graph

```
TASK-006-001 ──→ TASK-006-002
             └─→ TASK-006-003
```

| Task | Estimate | Predecessors | Parallel | Model |
|------|----------|--------------|----------|-------|
| TASK-006-001 | ~15k | none | no (foundational) | — |
| TASK-006-002 | ~30k | 001 | yes (with 003) | — |
| TASK-006-003 | ~75k | 001 | yes (with 002) | opus |

**Total estimated tokens:** ~120k

## Functional Requirements

- FR-1: `Config.AutoWork` must accept exactly three string values: `"disabled"`, `"enabled"`, `"delayed"`. Any other value (including empty) must be treated as `"disabled"`.
- FR-2: The default value of `auto_work` when not present in `config.yml` must be `"disabled"`.
- FR-3: The config TUI must display the current value and allow cycling left/right through all three options.
- FR-4: Saving in the config TUI must persist the value to `.maggus/config.yml`.
- FR-5: When `auto_work` is `"enabled"`, maggus must dispatch `work --count 999` the moment a `featureSummaryUpdateMsg` arrives with workable tasks > 0.
- FR-6: When `auto_work` is `"delayed"`, maggus must start a 5-second countdown, render the remaining seconds in the menu view, and only dispatch if no key is pressed before the countdown expires.
- FR-7: Any key press during the delayed countdown must cancel it cleanly, leaving the user in the normal main menu state.
- FR-8: If the countdown is already active and another file change arrives, the existing countdown must not reset or duplicate — it continues as-is.
- FR-9: The dispatched work command must be identical to what happens when the user manually selects "work" from the menu.

## Non-Goals

- No auto-work trigger on startup (only reacts to file changes while idling, not to tasks that already existed when maggus opened)
- No per-feature-file auto-work configuration — the setting is global for the project
- No audio/notification integration specific to auto-work (the existing notification system handles post-run notifications)
- No "auto-work for bugs only" or "auto-work for features only" granularity in this iteration

## Technical Considerations

- The countdown in `"delayed"` mode requires adding a new `autoWorkCountdown int` field to `menuModel` and a `autoWorkActive bool` guard. A `tickMsg` (from `tea.Tick`) drives the decrement.
- Config must be re-read on each `featureSummaryUpdateMsg` (or cached in `menuModel` and refreshed alongside the summary) so that changes made in the config TUI take effect without restarting maggus.
- The dispatch path (`m.selected = "work"`, `m.args = [...]`) already exists in `cmd/menu.go` — auto-work reuses this exact mechanism so `root.go`'s `runMenu` picks it up naturally via `final.selected`.
- The `featureSummaryUpdateMsg` handler already calls `loadFeatureSummary()` — use the returned workable count from this existing call rather than re-parsing separately.

## Success Metrics

- User creates a new `.maggus/features/feature_*.md` file while maggus is open in the main menu → maggus starts working within 0s (enabled) or ≤6s (delayed) without any manual input
- User can toggle the setting in the config TUI and the change is reflected on the next file event without restarting
- Setting `auto_work: disabled` (or omitting it) preserves all existing behavior exactly

## Open Questions

— none
