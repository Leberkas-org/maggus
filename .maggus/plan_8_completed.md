# Plan: Allow Manual Update in Dev Builds

## Introduction

Currently, both the manual `maggus update` command and the automatic startup update check skip execution when running a local dev build (`Version == "dev"`). This change allows the manual update command to work in dev builds — fetching the latest release and offering to install it — while keeping the automatic startup check disabled.

## Goals

- Allow developers running local builds to manually update to the latest release via `maggus update`
- Keep automatic startup update checks disabled for dev builds (no change to menu behavior)
- The installed binary after updating is the official release version (replacing the dev binary)

## User Stories

### TASK-001: Remove dev guard from CheckLatestVersion
**Description:** As a developer running a local build, I want `CheckLatestVersion` to fetch release info even when the current version is "dev" so that the manual update command can find available releases.

**Acceptance Criteria:**
- [x] `CheckLatestVersion` no longer returns early when `currentVersion == "dev"`
- [x] When `currentVersion` is "dev", `IsNewer` is always `true` if a valid release exists (since "dev" cannot be compared via semver, any release is considered newer)
- [x] When `currentVersion` is a valid semver, behavior is unchanged (normal semver comparison)
- [x] Existing unit tests for `CheckLatestVersion` still pass
- [x] New unit test: calling `CheckLatestVersion("dev")` with a mocked release returns `IsNewer: true`

### TASK-002: Remove dev guard from manual update command
**Description:** As a developer running a local build, I want `maggus update` to check for and install updates instead of printing "Skipping update check".

**Acceptance Criteria:**
- [x] The dev build guard (`if currentVersion == "dev"`) is removed from `runUpdate` in `src/cmd/update.go`
- [x] When `Version == "dev"`, running `maggus update` fetches the latest release and shows "Update available: dev → vX.Y.Z"
- [x] The version display in the "Update available" line handles "dev" gracefully (no `v` prefix stripping needed for "dev")
- [x] Confirmation prompt and apply flow work identically to non-dev builds
- [x] The automatic startup check in `src/cmd/menu.go` still returns early for dev builds (no change)
- [x] Update `TestUpdate_DevVersion` to verify that the update flow proceeds instead of skipping
- [x] Add a new test: dev build with mocked newer release shows update available and applies successfully
- [x] Typecheck/lint passes
- [x] Unit tests are written and successful

## Functional Requirements

- FR-1: `CheckLatestVersion("dev")` must fetch the latest GitHub release and return `IsNewer: true` when a valid release tag exists
- FR-2: `semverNewer` must not be called when `currentVersion` is "dev" — any release is newer than a dev build
- FR-3: The manual `maggus update` command must not skip execution for dev builds
- FR-4: The version display line must handle "dev" as current version (e.g., "Update available: dev → v1.2.3")
- FR-5: The automatic startup update check (`startupUpdateCheck` in `menu.go`) must continue to return early for dev builds

## Non-Goals

- No changes to the automatic startup update check behavior
- No changes to the menu item visibility or description for the update command
- No additional warning or confirmation when updating from a dev build
- No changes to the update apply logic (`updater/apply.go`)

## Technical Considerations

- The `semverNewer` function cannot parse "dev" as a valid version, so `CheckLatestVersion` needs a special case: if `currentVersion == "dev"` and a valid release tag was fetched, set `IsNewer = true` directly instead of calling `semverNewer`
- The `startupUpdateCheck` in `menu.go` has its own independent dev guard at line 49 — this remains untouched
- The "Already up to date" message in `runUpdate` uses `strings.TrimPrefix(currentVersion, "v")` — this works fine with "dev" (no-op trim)

## Success Metrics

- Running `maggus update` from a dev build successfully fetches and offers the latest release
- Automatic startup check remains silent for dev builds
- All existing tests continue to pass; new tests cover the dev-build update path
