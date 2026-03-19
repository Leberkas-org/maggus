# Plan: Auto-Update, Drive Switching & Documentation

## Introduction

This plan covers three areas: (1) an auto-update mechanism that checks GitHub Releases for new versions and can update the binary automatically, (2) drive letter navigation in the TUI file browser for Windows users with multiple drives, and (3) a comprehensive getting-started documentation overhaul so new users can go from zero to productive.

## Goals

- Allow users to stay on the latest release without manual downloads
- Skip update checks for local dev builds (version = "dev")
- Support fully automatic updates via config, or interactive confirmation by default
- Let Windows users switch between drives (C:, D:, etc.) in the file browser
- Provide clear, platform-agnostic documentation that gets a new user from install to first completed task

## User Stories

### TASK-001: Add update check package
**Description:** As a developer, I want an internal package that checks the GitHub Releases API for the latest maggus version so that other commands can use it.

**Acceptance Criteria:**
- [x] New package `src/internal/updater/` created
- [x] `CheckLatestVersion()` function calls `https://api.github.com/repos/leberkas-org/maggus/releases/latest` (unauthenticated)
- [x] Returns a struct with `TagName`, `DownloadURL` (for the current OS/arch), and `IsNewer` bool
- [x] Compares semver of current `cmd.Version` against the release tag
- [x] Returns `IsNewer = false` immediately when `cmd.Version == "dev"` (skip check for local builds)
- [x] Handles network errors gracefully (returns no-update, no error surfaced to user)
- [x] Selects the correct asset URL based on `runtime.GOOS` and `runtime.GOARCH`
- [x] Unit tests with mocked HTTP responses cover: newer version available, already up-to-date, dev version, network error, malformed response

### TASK-002: Add binary self-replacement logic
**Description:** As a developer, I want a function that downloads a release asset and replaces the running binary so that updates can be applied.

**Acceptance Criteria:**
- [x] `updater.Apply(downloadURL string)` downloads the asset to a temp file
- [x] Extracts the binary from the archive (tar.gz on Linux/macOS, zip on Windows)
- [x] Replaces the currently running executable (`os.Executable()` path) using a safe rename-swap strategy
- [x] On Windows, uses the rename-on-next-start pattern (rename old binary to `.old`, write new one, old gets cleaned up on next run)
- [x] Returns an error if permissions prevent writing to the binary location
- [x] Unit tests cover: successful replacement (using temp dir), permission error, corrupt archive

### TASK-003: Add `maggus update` command
**Description:** As a user, I want a `maggus update` command so that I can manually check for and install updates.

**Acceptance Criteria:**
- [x] New cobra command `update` registered in `src/cmd/update.go`
- [x] Calls `updater.CheckLatestVersion()` and displays current vs latest version
- [x] If no update available, prints "Already up to date (vX.Y.Z)"
- [x] If update available, shows changelog summary and asks for confirmation (y/n)
- [x] On confirmation, calls `updater.Apply()` with a progress indicator
- [x] Prints success message with new version and suggests restarting
- [x] When version is "dev", prints "Skipping update check — running a local dev build" and exits
- [x] Unit tests for the command output in each scenario (mock updater)

### TASK-004: Add auto-update config option
**Description:** As a user, I want to configure automatic updates in my global config so that maggus stays up-to-date without me running a command.

**Acceptance Criteria:**
- [x] New field `auto_update` in global config (`~/.maggus/config.yml`) with values: `"off"`, `"notify"` (default), `"auto"`
- [x] `globalconfig` package updated to read/write the new field
- [x] A `lastUpdateCheck` timestamp is stored in `~/.maggus/update_state.json` to enforce a 24h cooldown
- [x] Unit tests for config parsing with all three values and the cooldown logic

### TASK-005: Integrate update check into startup
**Description:** As a user, I want maggus to check for updates on startup so that I'm notified or auto-updated based on my config.

**Acceptance Criteria:**
- [ ] On startup (in `root.go` Execute or menu init), if version != "dev" and cooldown has passed (24h):
  - `notify` mode: show a one-line banner "Update available: vX.Y.Z → vA.B.C — run `maggus update` to install"
  - `auto` mode: silently download and apply update, then print "Updated to vA.B.C — restart maggus to use the new version"
  - `off` mode: skip entirely
- [ ] The update check runs in a goroutine so it doesn't block startup
- [ ] The banner is shown in the TUI menu as a styled info line (not blocking the UI)
- [ ] Updates `lastUpdateCheck` timestamp after a successful check
- [ ] Startup time is not noticeably affected (< 100ms added)
- [ ] Unit tests for each mode with mocked updater

### TASK-006: Add drive letter navigation to file browser (Windows)
**Description:** As a Windows user, I want to see available drives when I navigate to the root of my current drive so that I can switch between drives.

**Acceptance Criteria:**
- [ ] When on Windows and the user navigates up from the root of a drive (e.g., `C:\`), the file browser shows all available drive letters as entries instead of ".."
- [ ] Available drives are detected by checking which drive letters (A:-Z:) exist (using `os.Stat` on `X:\`)
- [ ] Each drive is shown as an entry like `C:\`, `D:\`, `E:\` with a drive icon
- [ ] Selecting a drive navigates into that drive's root directory
- [ ] On non-Windows platforms, this code is not compiled (build tags: `//go:build windows`)
- [ ] The ".." entry at a drive root is replaced with a "Drives" or "Switch Drive" entry that triggers the drive listing
- [ ] Unit tests cover: drive listing on Windows (mocked), navigation from drive list into a drive, no-op on non-Windows

### TASK-007: Rewrite getting-started guide
**Description:** As a new user, I want a clear getting-started guide so that I can install maggus and run my first task within minutes.

**Acceptance Criteria:**
- [ ] `docs/guide/getting-started.md` is rewritten with these sections:
  - **Prerequisites** — what you need (an AI agent CLI, git, a terminal)
  - **Installation** — three methods: pre-built binary download, `go install`, build from source. Platform-agnostic with OS-specific callouts where needed.
  - **First Project Setup** — `cd` into a git repo, run `maggus init`, explain what it creates
  - **Writing Your First Plan** — minimal example plan with 1-2 tasks, explain the format briefly and link to the writing-plans guide
  - **Running Maggus** — `maggus work`, what to expect (branch creation, agent invocation, commit), show sample output
  - **Understanding the Output** — explain the spinner, status messages, what "completed" means
  - **Next Steps** — links to concepts, writing-plans, configuration
- [ ] All code examples work on Windows, macOS, and Linux (use platform-agnostic commands, add callouts for differences like path separators)
- [ ] No broken internal links

### TASK-008: Add first-project tutorial page
**Description:** As a new user, I want a step-by-step tutorial that walks me through a real example project so that I can see maggus in action end-to-end.

**Acceptance Criteria:**
- [ ] New file `docs/guide/tutorial.md`
- [ ] Walks through a concrete example: creating a small project, writing a plan with 2-3 tasks, running `maggus work`, inspecting the results
- [ ] Includes the example plan inline (something simple like "add a greeting function and a test")
- [ ] Shows expected terminal output at each step (simplified/abbreviated)
- [ ] Explains how to check task status with `maggus status`
- [ ] Explains what happens when a task is blocked and how to resolve it
- [ ] Added to the VitePress sidebar navigation

### TASK-009: Update configuration reference docs
**Description:** As a user, I want up-to-date configuration reference docs so that I can understand all available settings.

**Acceptance Criteria:**
- [ ] `docs/reference/configuration.md` updated to include:
  - All `.maggus/config.yml` fields with descriptions and defaults
  - Global config (`~/.maggus/config.yml`) fields including the new `auto_update` setting
  - `~/.maggus/repositories.yml` format explanation
  - Model alias table (sonnet, opus, haiku → full model IDs)
- [ ] `docs/reference/commands.md` updated to include the new `maggus update` command
- [ ] All existing command docs are verified to be current

### TASK-010: Update VitePress sidebar and navigation
**Description:** As a user, I want consistent navigation in the docs site so that I can find all pages.

**Acceptance Criteria:**
- [ ] VitePress config (`.vitepress/config.js` or similar) updated with sidebar entries for:
  - Guide: Getting Started, Tutorial, Concepts, Writing Plans, Maggus Plan Skill
  - Reference: Commands, Configuration
- [ ] All sidebar links resolve correctly
- [ ] Home page hero "Get Started" button links to the updated getting-started page

## Functional Requirements

- FR-1: The updater must never run when `Version == "dev"` — this is the guard for local builds
- FR-2: The GitHub API call must be unauthenticated and handle rate limiting gracefully (back off, don't error)
- FR-3: Binary replacement must be atomic where possible — no half-written binaries on crash
- FR-4: The `auto_update` config default is `"notify"` — users must opt-in to fully automatic updates
- FR-5: Drive detection on Windows must not hang on disconnected/slow network drives — use a short timeout or skip non-local drives
- FR-6: The file browser drive listing must only appear on Windows; on other platforms the root (`/`) is just a normal directory
- FR-7: Documentation must not assume any specific OS unless in a clearly marked callout block
- FR-8: The update banner in the TUI menu must not interfere with the menu layout or steal focus

## Non-Goals

- No auto-update for dev builds — the `"dev"` version is always skipped
- No delta/patch updates — always download the full binary
- No rollback mechanism — if an update is bad, the user downloads the previous version manually
- No update channel selection (stable/beta) — always uses the latest GitHub Release
- No drive browsing on Linux/macOS — the root filesystem (`/`) is the natural boundary
- No video tutorials or interactive documentation — text-based guides only

## Technical Considerations

- Use `net/http` for the GitHub API call — no external dependencies needed
- Use `archive/tar` + `compress/gzip` for tar.gz and `archive/zip` for zip extraction
- On Windows, the running binary cannot be overwritten directly — rename it to `maggus.exe.old` first, write the new one, then clean up `.old` on next startup
- Drive detection: iterate A-Z and `os.Stat(letter + ":\\")` — skip drives that return error
- The goreleaser archive naming convention is `maggus_<version>_<os>_<arch>.tar.gz` (or `.zip`) — the updater needs to match this pattern
- VitePress sidebar config is typically in `docs/.vitepress/config.js` or `config.mts`

## Success Metrics

- A user on an outdated version sees the update banner within 1 second of launching maggus
- `maggus update` successfully replaces the binary and the new version is confirmed on next run
- A Windows user can navigate from `C:\projects\myrepo` to `D:\other\repo` using only the file browser
- A new user can follow the getting-started guide from zero to a completed maggus task without needing to consult other resources

## Open Questions

- Should the updater verify download integrity via checksums (GoReleaser generates `checksums.txt`)?
- Should there be a `maggus update --check` flag that only checks without installing?
- Should the drive listing show volume labels (e.g., "D: (Data)") or just drive letters?
