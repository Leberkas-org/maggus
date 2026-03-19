# Plan: Claude 2x Status Integration

## Introduction

Integrate the public API at `https://isclaude2x.com/json` into Maggus to visually indicate when Claude is in "2x mode". When active, the ASCII logo and menu box border change from the default cyan/blue to yellow, and the remaining 2x window time is displayed in both the main menu and the work view.

## Goals

- Fetch Claude 2x status from the API at application startup
- Visually indicate 2x mode by changing the logo and menu box border color to yellow
- Display the `2xWindowExpiresIn` countdown in the main menu (below the logo) and in the work view header
- Fail silently if the API is unreachable — fall back to normal colors with no error shown

## User Stories

### TASK-001: Create an API client package for isclaude2x
**Description:** As a developer, I want a reusable package that fetches and parses the isclaude2x API response so that other parts of the application can query the 2x status.

**Acceptance Criteria:**
- [x] New package `src/internal/claude2x/` created
- [x] Struct defined matching the API JSON response (at minimum: `Is2x bool`, `TwoXWindowExpiresIn string`, `TwoXWindowExpiresInSeconds int`)
- [x] Function `FetchStatus()` performs a GET request to `https://isclaude2x.com/json` and returns the parsed struct
- [x] HTTP request has a reasonable timeout (e.g. 3 seconds) so the UI is not blocked for long
- [x] On any error (network, parse, non-200 status), returns a zero-value struct with `Is2x: false` — no error is surfaced to the caller
- [x] Unit tests are written and successful (test JSON parsing with a mock/httptest server, test error/timeout handling)

### TASK-002: Make logo and box border colors dynamic based on 2x status
**Description:** As a user, I want the menu logo and box border to turn yellow when Claude is in 2x mode so that I can see at a glance whether 2x is active.

**Acceptance Criteria:**
- [x] `styles` package exposes a way to switch between normal (cyan/Primary) and 2x (yellow) color for the logo and box border
- [x] The menu model (`cmd/menu.go`) calls `claude2x.FetchStatus()` once during initialization (in `Init()` or as a Bubble Tea `Cmd`)
- [x] When `Is2x` is `true`, the logo renders in yellow and the menu box border renders in yellow
- [x] When `Is2x` is `false`, the logo and box border render in the default cyan color
- [x] If the API call fails or times out, the default cyan colors are used (no visible error)
- [x] Typecheck/lint passes

### TASK-003: Display 2x remaining time in the main menu
**Description:** As a user, I want to see how much time is left in the 2x window directly in the main menu so I can plan my usage.

**Acceptance Criteria:**
- [x] When `Is2x` is `true`, the `2xWindowExpiresIn` value (e.g. "17h 54m 44s") is displayed in the menu below the logo area
- [x] The display format is clear, e.g. `2x expires in: 17h 54m 44s` styled in yellow
- [x] When `Is2x` is `false`, no 2x-related text is shown in the menu
- [x] Typecheck/lint passes

### TASK-004: Display 2x remaining time in the work view
**Description:** As a user, I want to see the 2x remaining time in the work view header so I'm aware of the 2x window while tasks are running.

**Acceptance Criteria:**
- [x] The work view TUI (`internal/runner/tui.go`) receives the 2x status information
- [x] When `Is2x` is `true`, the header area displays the remaining time (e.g. "2x: 17h 54m 44s") styled in yellow
- [x] When `Is2x` is `false`, no 2x indicator is shown in the work view
- [x] The 2x info does not break the existing header layout (version, fingerprint, progress bar)
- [x] Typecheck/lint passes

## Functional Requirements

- FR-1: The system must perform a single HTTP GET to `https://isclaude2x.com/json` when the menu loads
- FR-2: The HTTP request must have a timeout of no more than 3 seconds
- FR-3: When `is2x` is `true` in the API response, the ASCII logo color must change from cyan (ANSI 6) to yellow (ANSI 3)
- FR-4: When `is2x` is `true`, the menu box border color must change from cyan to yellow
- FR-5: When `is2x` is `true`, the text `2x expires in: <2xWindowExpiresIn>` must be visible below the logo in the main menu
- FR-6: When `is2x` is `true`, the 2x remaining time must be visible in the work view header
- FR-7: Any API failure (timeout, network error, non-200 response, malformed JSON) must result in silent fallback to default cyan colors with no 2x text displayed
- FR-8: The API response field `2xWindowExpiresIn` (string, e.g. "17h 54m 44s") is used as-is for display — no client-side countdown recalculation needed

## Non-Goals

- No periodic polling or live countdown — the value is fetched once and displayed as-is
- No caching of the API response across runs
- No configuration option to disable the 2x check
- No changes to the work loop logic or task execution based on 2x status
- No display of other API fields (peak hours, promo period, etc.) beyond `is2x` and `2xWindowExpiresIn`

## Technical Considerations

- The API returns JSON with a field name starting with a digit (`2xWindowExpiresIn`). In Go, use struct tags: `json:"2xWindowExpiresIn"` to map it to a valid Go field name
- The `net/http` package is not currently used in the project — this will be the first HTTP dependency
- The API call should be non-blocking to the TUI. Use a Bubble Tea `Cmd` to fetch asynchronously so the menu renders immediately and updates when the response arrives
- The `styles` package currently defines colors as package-level constants. Consider adding a function or variables that can be swapped based on 2x status, or pass the color as a parameter to rendering functions
- The work view (`runner/tui.go`) receives its configuration from `cmd/work.go` — the 2x status needs to be passed through this chain

## Success Metrics

- Logo and box border visually change to yellow when the API reports `is2x: true`
- The `2xWindowExpiresIn` value is visible in both menu and work view when 2x is active
- No visible delay or error when the API is unreachable
- All existing tests continue to pass

## Open Questions

- Should the 2x status also be shown in the `list` or `status` commands, or only in the menu and work view?
