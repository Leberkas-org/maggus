<!-- maggus-id: 78cf12c9-3f09-4364-b6df-3fe6483f402b -->
# Feature 030: Single-program TUI architecture — eliminate menu transition flickering

## Introduction

Every navigation from the main menu to a sub-screen (status, config, repos, prompt) and back
causes a visible flicker: the terminal flashes between the alt-screen and the normal buffer
because each view is its own `tea.NewProgram(…, tea.WithAltScreen())`. The fix is to replace the
current "start-stop-restart" architecture with a single, long-lived BubbleTea program that swaps
views internally — the alt-screen is entered once at startup and left once at exit, with zero
intermediate teardowns.

### Architecture Context

- **Components involved:** `cmd/root.go` (program entry point), `menu_model.go`, `status_cmd.go`,
  `config.go`, `repos.go`, `prompt.go`, `prompt_picker.go`
- **New pattern introduced:** A top-level `appModel` acting as a screen router; navigation is
  driven by BubbleTea messages (`navigateToMsg`, `navigateBackMsg`) instead of program restarts.
  Subprocess execution (prompt → claude, status → dispatchWork) is handled via BubbleTea's
  `tea.ExecProcess` so the TUI suspends cleanly rather than exiting.

## Goals

- Eliminate all flickering caused by alt-screen teardown/rebuild between menu and sub-screens.
- Keep the existing behaviour of every sub-screen identical from the user's perspective.
- Replace the `for {}` program loop in `root.go` with a single `tea.NewProgram` call.
- Use `tea.ExecProcess` for any place where an external process currently requires the TUI to exit
  (prompt → `launchInteractive`, status → `dispatchWork`).

## Tasks

### TASK-030-001: Create the app router model
**Description:** As a developer, I want a top-level `appModel` in `cmd/app_model.go` that acts as
a screen router so that a single `tea.NewProgram` can host all sub-screens without restart.

**Token Estimate:** ~60k tokens
**Predecessors:** none
**Successors:** TASK-030-002, TASK-030-003, TASK-030-004, TASK-030-005, TASK-030-006
**Parallel:** no — all other tasks depend on the types defined here

**Acceptance Criteria:**
- [x] New file `src/cmd/app_model.go` is created
- [x] `screenID` type and constants defined: `screenMenu`, `screenStatus`, `screenConfig`,
  `screenRepos`, `screenPrompt`
- [x] `navigateToMsg{screen screenID}` and `navigateBackMsg{}` message types defined
- [x] `execProcessMsg{cmd *exec.Cmd; onDone func(error)}` message type defined for subprocess
  delegation (used by prompt and status dispatch)
- [x] `appModel` struct holds the active `screenID` and lazy-initialised sub-models
  (`*menuModel`, `*statusModel`, `*configModel`, `*reposModel`, `*promptPickerModel`)
- [x] `appModel.Init()` delegates to the active screen's `Init()`
- [x] `appModel.Update()` intercepts `navigateToMsg` / `navigateBackMsg` / `execProcessMsg`
  and otherwise delegates to the active screen
- [x] When `navigateToMsg` is received, the target sub-model is initialised (or re-initialised)
  and `Init()` is called for it; the previous model's teardown (e.g. watcher close) happens here
- [x] When `navigateBackMsg` is received from any sub-screen, control returns to `screenMenu`
  and the menu model is re-initialised
- [x] `appModel.View()` delegates to the active screen's `View()`
- [x] `tea.WindowSizeMsg` is forwarded to the active sub-model
- [x] `go build ./...` passes with no errors

---

### TASK-030-002: Adapt status model to use navigation messages
**Description:** As a developer, I want the status model to emit `navigateBackMsg` on quit and
`execProcessMsg` when the user triggers a task run, so it works inside the app router without
exiting the program.

**Token Estimate:** ~50k tokens
**Predecessors:** TASK-030-001
**Successors:** TASK-030-007
**Parallel:** yes — can run alongside TASK-030-003, TASK-030-004, TASK-030-005, TASK-030-006

**Acceptance Criteria:**
- [x] All `tea.Quit` calls in `status_update.go` are replaced with `return m, func() tea.Msg { return navigateBackMsg{} }`
- [x] The `RunTaskID` return-value pattern (currently read from the final model after `prog.Run()`)
  is replaced: when the user triggers "run task", the model emits
  `execProcessMsg` carrying the work dispatch command and a `navigateBackMsg` callback so
  the router can exec the process then return to menu
- [x] `runStatus()` in `status_cmd.go` no longer calls `tea.NewProgram`; it instead builds a
  `statusModel` and returns it so `appModel` can embed it — OR `runStatus()` is removed and
  status initialisation moves to `appModel`
- [x] File watchers and log watchers in the status model are still closed when the model is
  torn down (moved to the `navigateBackMsg` handler in `appModel`)
- [x] `go build ./...` passes

---

### TASK-030-003: Adapt config model to use navigation messages
**Description:** As a developer, I want the config model to emit `navigateBackMsg` on exit so it
works inside the app router.

**Token Estimate:** ~20k tokens
**Predecessors:** TASK-030-001
**Successors:** TASK-030-007
**Parallel:** yes — can run alongside TASK-030-002, TASK-030-004, TASK-030-005, TASK-030-006

**Acceptance Criteria:**
- [x] All `tea.Quit` calls in the config model are replaced with `navigateBackMsg`
- [x] `runConfig()` no longer calls `tea.NewProgram`; config initialisation moves to `appModel`
- [x] Any post-exit actions (e.g. external editor launch) are handled via `execProcessMsg` or
  kept as non-TUI side effects triggered from the app model on `navigateBackMsg`
- [x] `go build ./...` passes

---

### TASK-030-004: Adapt repos model to use navigation messages
**Description:** As a developer, I want the repos model to emit `navigateBackMsg` on exit and
clean up its daemon caches via the app model, so it works inside the app router.

**Token Estimate:** ~25k tokens
**Predecessors:** TASK-030-001
**Successors:** TASK-030-007
**Parallel:** yes — can run alongside TASK-030-002, TASK-030-003, TASK-030-005, TASK-030-006

**Acceptance Criteria:**
- [x] All `tea.Quit` calls in the repos model are replaced with `navigateBackMsg`
- [x] Daemon cache cleanup (currently done by iterating `final.daemonCaches` after `prog.Run()`)
  is triggered from `appModel` when it receives `navigateBackMsg` from the repos screen
- [x] `runRepos()` no longer calls `tea.NewProgram`; repos initialisation moves to `appModel`
- [x] `go build ./...` passes

---

### TASK-030-005: Adapt prompt picker to use navigation messages and tea.ExecProcess
**Description:** As a developer, I want the prompt picker to navigate back on cancel and trigger
`execProcessMsg` on confirm, replacing the current exit-and-relaunch pattern.

**Token Estimate:** ~40k tokens
**Predecessors:** TASK-030-001
**Successors:** TASK-030-007
**Parallel:** yes — can run alongside TASK-030-002, TASK-030-003, TASK-030-004, TASK-030-006

**Acceptance Criteria:**
- [x] On cancel, prompt picker emits `navigateBackMsg` instead of `tea.Quit`
- [x] On skill selection, prompt picker emits `execProcessMsg` carrying the `*exec.Cmd` for
  `launchInteractive` (or equivalent) and a done-callback that returns `navigateBackMsg`
- [x] The app model handles `execProcessMsg` using `tea.ExecProcess` so the TUI suspends, claude
  runs in the foreground, and the TUI resumes with the menu shown
- [x] All post-process work (usage extraction, presence update) is done in the `onDone` callback
  or as a follow-up message to the app model
- [x] `runPrompt()` no longer calls `tea.NewProgram`; prompt initialisation moves to `appModel`
- [x] `go build ./...` passes

---

### TASK-030-006: Adapt menu model to emit navigation messages
**Description:** As a developer, I want the menu model to emit `navigateToMsg` when the user
selects a sub-command, replacing the current pattern where the model quits with a `selected`
string that the `for {}` loop in `root.go` reads.

**Token Estimate:** ~35k tokens
**Predecessors:** TASK-030-001
**Successors:** TASK-030-007
**Parallel:** yes — can run alongside TASK-030-002, TASK-030-003, TASK-030-004, TASK-030-005

**Acceptance Criteria:**
- [x] When the user selects status/config/repos/prompt, the menu model emits
  `navigateToMsg{screen: screenXxx}` instead of setting `m.selected` and calling `tea.Quit`
- [x] The `selected` field and the quit-on-select logic are removed from the menu model
- [x] "Exit" still emits `tea.Quit` (this is the only path that actually terminates the program)
- [x] Non-TUI cobra subcommands dispatched from the menu (e.g. `work`, `list`, `clean`,
  `release`, `update`) continue to work — the menu model emits an `execProcessMsg` or the
  app model runs them as cobra sub-invocations; the exact mechanism is left to the implementer
  to choose the cleanest approach
- [x] The `for {}` loop structure in `root.go` is no longer needed for TUI sub-command dispatch
- [x] `go build ./...` passes

---

### TASK-030-007: Wire root.go — single program, remove the for loop
**Description:** As a developer, I want `runMenu` in `root.go` to create exactly one
`tea.NewProgram(appModel, tea.WithAltScreen())` call so the alt-screen is entered once and all
navigation happens inside that program.

**Token Estimate:** ~30k tokens
**Predecessors:** TASK-030-002, TASK-030-003, TASK-030-004, TASK-030-005, TASK-030-006
**Successors:** none
**Parallel:** no — requires all sub-model adaptations to be complete

**Acceptance Criteria:**
- [x] `runMenu` creates a single `appModel` starting on `screenMenu` and runs
  `tea.NewProgram(app, tea.WithAltScreen())`
- [x] The `for {}` loop in `runMenu` is removed
- [x] The `directDispatch` map is removed (routing now lives in `appModel`)
- [x] Discord presence lifecycle, daemon cache, and shared-presence wiring are moved into
  `appModel` or its initialisation so they survive across screen transitions
- [x] Opening maggus and navigating menu → status → menu → config → menu produces no visible
  flicker between transitions (alt-screen is held continuously)
- [x] `go test ./...` passes
- [x] `go build ./...` produces a working binary

## Task Dependency Graph

```
TASK-030-001 ──→ TASK-030-002 ──┐
              ├→ TASK-030-003 ──┤
              ├→ TASK-030-004 ──┼──→ TASK-030-007
              ├→ TASK-030-005 ──┤
              └→ TASK-030-006 ──┘
```

| Task | Estimate | Predecessors | Parallel | Model |
|------|----------|--------------|----------|-------|
| TASK-030-001 | ~60k | none | no | opus |
| TASK-030-002 | ~50k | 001 | yes (with 003–006) | — |
| TASK-030-003 | ~20k | 001 | yes (with 002, 004–006) | — |
| TASK-030-004 | ~25k | 001 | yes (with 002–003, 005–006) | — |
| TASK-030-005 | ~40k | 001 | yes (with 002–004, 006) | — |
| TASK-030-006 | ~35k | 001 | yes (with 002–005) | — |
| TASK-030-007 | ~30k | 002–006 | no | — |

**Total estimated tokens:** ~260k

## Functional Requirements

- FR-1: The application MUST enter the terminal alt-screen exactly once per `maggus` invocation.
- FR-2: Navigating from the menu to any sub-screen MUST NOT produce a visible flash or blank
  frame between the two views.
- FR-3: Returning from any sub-screen to the menu MUST NOT produce a visible flash or blank frame.
- FR-4: The menu MUST still support all existing sub-commands (status, config, repos, prompt,
  work, list, clean, release, update, exit).
- FR-5: When a sub-command requires running an external process (e.g. claude CLI, `$EDITOR`),
  the TUI MUST suspend cleanly via `tea.ExecProcess`, run the process, and resume — without
  re-entering the alt-screen.
- FR-6: All existing keyboard shortcuts, navigation, and behaviour within each sub-screen MUST
  remain unchanged.
- FR-7: Discord presence lifecycle, daemon cache subscriptions, and shared-presence wiring MUST
  continue to function correctly across screen transitions.

## Non-Goals

- No changes to the visual appearance of any screen.
- No changes to `maggus work`, `maggus list`, `maggus clean`, `maggus release`, or
  `maggus update` as standalone cobra commands — only their invocation from within the TUI menu
  needs updating.
- No changes to `approve.go` or `daemon_keepalive.go` — these are not part of the menu flow.
- No new features or keyboard shortcuts.

## Technical Considerations

- BubbleTea's `tea.ExecProcess(cmd *exec.Cmd, fn func(error) tea.Msg)` is the correct primitive
  for suspending the TUI to run a subprocess (claude, `$EDITOR`). The program command runs in the
  foreground; when it exits, `fn` is called with the error and the TUI resumes.
- The `appModel` should initialise sub-models lazily (on first navigation) and tear them down
  (close watchers, unsubscribe channels) when navigating away, mirroring what the current
  `runMenu` loop does between iterations.
- Cobra subcommands invoked from the menu that are not TUI-based (`work`, `list`, `clean`,
  `release`, `update`) can continue to be run via `tea.ExecProcess` wrapping the existing cobra
  `RunE` indirectly, or by keeping a thin non-TUI code path. The implementer should choose the
  approach that requires the least restructuring of non-menu code.
- The `xterm.GetSize` pre-sizing trick in `newStatusModel` (and `newMenuModel`) should be
  preserved — it prevents a blank first frame by giving the model real dimensions before the
  first `tea.WindowSizeMsg` arrives.

## Success Metrics

- Zero visible flicker when navigating between any two screens in the menu.
- All existing `go test ./...` tests continue to pass.
- The binary behaves identically to before from the user's perspective.

## Open Questions

_(none — all resolved before saving)_
