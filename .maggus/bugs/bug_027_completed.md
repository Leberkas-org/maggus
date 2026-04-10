<!-- maggus-id: b5e8f09e-9173-4204-81b5-6afc83ff6cb7 -->
# Bug: Parallel config missing from config view and defaults to false instead of true

## Summary

The `parallel` setting is not shown in the project config TUI, making it impossible to toggle without editing `config.yml` by hand. Additionally, the field defaults to `false` (sequential mode) but should default to `true` (parallel on by default).

## Related

- **Commit:** d4a8c68 (feat(TASK-038-001): Add parallel config field and --parallel CLI flags)

## Steps to Reproduce

1. Run `maggus config`
2. Observe the Project tab
3. Notice there is no "Parallel" toggle row
4. Create a new project with no `config.yml` — parallel is off by default

## Expected Behavior

- A "Parallel" toggle row (on / off) appears in the Project config tab.
- When no `parallel` key is set in `config.yml`, `IsParallelEnabled()` returns `true`.

## Root Cause

**Missing config view row:** `newConfigModel()` at `src/cmd/config.go:168` builds `projectRows` but never adds an entry for the `Parallel` field. The field was added to the `Config` struct in commit `d4a8c68` but was never wired into the TUI.

**Wrong default:** `Parallel` is declared as `bool` at `src/internal/config/config.go:143`:

```go
Parallel bool `yaml:"parallel"`
```

A bare `bool` defaults to `false`. To make the default `true` it must become `*bool`, with `IsParallelEnabled()` returning `true` when the pointer is nil — the same pattern used by `IsAutoBranchEnabled()` and `IsCheckSyncEnabled()`.

`buildConfig()` at `src/cmd/config.go:227` also never writes the parallel setting back when saving, so even once the row is added, toggling it in the TUI would have no effect on the saved YAML.

## User Stories

### BUG-027-001: Change Parallel field to *bool and flip default to true

**Description:** As a user, I want parallel mode to be on by default so I get the best performance without needing to configure anything.

**Acceptance Criteria:**
- [x] `Config.Parallel` is changed from `bool` to `*bool` in `src/internal/config/config.go`
- [x] `IsParallelEnabled()` returns `true` when `Parallel` is nil (default)
- [x] `IsParallelEnabled()` returns `false` when `Parallel` is explicitly `false`
- [x] `IsParallelEnabled()` returns `true` when `Parallel` is explicitly `true`
- [x] Existing tests in `config_test.go` updated to match new behaviour
- [x] `go vet ./...` and `go test ./...` pass

### BUG-027-002: Add Parallel toggle row to config TUI

**Description:** As a user, I want to toggle parallel mode from the config view so I don't have to edit `config.yml` by hand.

**Acceptance Criteria:**
- [x] A "Parallel" row with values `["on", "off"]` appears in `projectRows` in `newConfigModel()` (`src/cmd/config.go`)
- [x] The row is initialised from `cfg.IsParallelEnabled()` (on → index 0, off → index 1)
- [x] `buildConfig()` reads the row and sets `cfg.Parallel` accordingly (writes `nil`/omitted when on, `false` pointer when off — or stores the explicit pointer)
- [x] Saving project config persists the parallel value correctly
- [x] No regression in other project config rows
- [x] `go vet ./...` and `go test ./...` pass
