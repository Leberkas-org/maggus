# Plan: Developer Experience — Status, List & Include Validation

## Introduction

Right now, the only way to know "where am I in my plan?" is to manually open markdown files and eyeball them.
This plan adds three developer-experience improvements that make day-to-day use of Maggus significantly smoother:

1. **`maggus status`** — a terminal dashboard showing task progress across all plans, with a compact summary and a detailed task list below it.
2. **`maggus list [N]`** — a quick preview of the next N upcoming tasks without running Claude.
3. **Include file validation** — when starting a run, warn the user if any file listed in `config.yml` under `include` doesn't actually exist.

None of these require a network call or Claude. They are fast, local, purely informational commands.

---

## Goals

- Give users a clear answer to "where am I?" within one second
- Surface blocked tasks so they don't silently stall a run
- Make it easy to preview upcoming work before committing to a `maggus work` run
- Catch misconfigured `include` entries early, before they silently produce incomplete prompts
- Support both humans (rich color output) and scripts (plain text via `--plain`)

---

## User Stories

### TASK-401: `maggus status` — compact summary section
**Description:** As a developer, I want to run `maggus status` and immediately see a high-level summary of my plan progress so that I know how much work is left at a glance.

**Acceptance Criteria:**
- [x] `maggus status` is a registered subcommand of the root Cobra command
- [x] Output starts with a compact summary block showing totals: total tasks, completed, pending, blocked
- [x] Each plan file gets one line showing: plan filename, a progress bar, and "X/Y tasks" count
  - Example: `plan_3.md  [████████░░]  4/6 tasks`
- [x] Progress bar width is fixed (e.g., 10 characters) and fills proportionally
- [x] Completed plans (filename contains `_completed`) are shown with a distinct indicator (e.g., ✓) and skipped from the count of active plans
- [x] Colors are used: green for fully done, yellow for in-progress, red/orange for any blocked tasks
- [x] Runs without error when `.maggus/` has no plan files (prints "No plans found.")
- [x] Typecheck/lint passes
- [x] ⚠️ BLOCKED: Verify in browser using dev-browser skill — dev-browser skill is not available in the skills list; maggus status is a CLI tool with no browser component

### TASK-402: `maggus status` — detailed task list section
**Description:** As a developer, I want `maggus status` to also show me all individual tasks with their status so that I can see exactly which tasks are done, pending, or blocked.

**Acceptance Criteria:**
- [x] Below the compact summary, a detailed section lists every task across all active plans
- [x] Each task line shows: status icon, task ID, task title
  - `✓ TASK-003: Do the thing` (completed)
  - `○ TASK-004: Do the next thing` (pending)
  - `⚠ TASK-005: Blocked task` (blocked)
- [x] The next workable task (the one `maggus work` would pick up) is visually highlighted (e.g., `→` arrow prefix, bold or cyan color)
- [x] Tasks are grouped under their plan file heading
- [x] Completed plans are still shown but visually dimmed or marked as archived
- [x] Typecheck/lint passes
- [x] ⚠️ BLOCKED: Verify in browser using dev-browser skill — dev-browser skill is not available; maggus status is a CLI tool with no browser component

### TASK-403: `--plain` flag for `maggus status`
**Description:** As a developer, I want to run `maggus status --plain` and get clean, uncolored text output so that I can pipe the output into other tools or scripts.

**Acceptance Criteria:**
- [ ] `maggus status` accepts a `--plain` flag (boolean, default: false)
- [ ] When `--plain` is set, all ANSI color codes are stripped from output
- [ ] When `--plain` is set, Unicode box/arrow characters are replaced with ASCII equivalents (e.g., `[####......] 4/10` instead of `[████░░░░░░]`, `->` instead of `→`, `[x]` instead of `✓`)
- [ ] Progress bar in plain mode uses `#` for filled and `.` for empty
- [ ] Output is otherwise identical in structure (same lines, same information)
- [ ] Typecheck/lint passes

### TASK-404: `maggus list [N]` command
**Description:** As a developer, I want to run `maggus list` to preview the next N upcoming workable tasks so that I can plan my session before starting `maggus work`.

**Acceptance Criteria:**
- [ ] `maggus list` is a registered subcommand
- [ ] Default N is 5; can be overridden: `maggus list 10` or `maggus list --count 10`
- [ ] Only shows tasks that are **workable** (incomplete AND not blocked) — skips completed and blocked tasks
- [ ] Output format per task:
  ```
  #1  TASK-004: Title of task
      As a user, I want ... (first line of description, truncated to 80 chars if needed)
  ```
- [ ] If there are no workable tasks, prints: "No pending tasks found. All done!"
- [ ] If fewer than N tasks are available, shows what exists without error
- [ ] Accepts `--plain` flag (same behavior as in `status`)
- [ ] Typecheck/lint passes
- [ ] Verify in browser using dev-browser skill

### TASK-405: Include file validation with warnings
**Description:** As a developer, I want Maggus to warn me if a file listed in `config.yml` under `include` doesn't exist so that I notice misconfiguration before wasting a full Claude run.

**Acceptance Criteria:**
- [ ] Before starting the work loop in `cmd/work.go`, after loading config, validate each entry in `config.Include`
- [ ] For each path in `config.Include`, check if the file exists relative to the current working directory
- [ ] If a file is missing, print a **warning** (not an error) to stderr:
  ```
  Warning: included file not found: docs/PATTERNS.md (skipping)
  ```
- [ ] Missing files are **skipped** from the prompt — execution continues normally
- [ ] If all included files are missing, execution still continues (the warning is enough)
- [ ] The validation warning is printed before Claude is invoked, not silently ignored
- [ ] Add a helper function `ValidateIncludes(includes []string, baseDir string) []string` in `internal/config/` that returns only the valid (existing) paths — this is the list actually passed to the prompt builder
- [ ] Unit tests for `ValidateIncludes`: empty list, all valid, some missing, all missing
- [ ] Typecheck/lint passes

### TASK-406: Wire up new commands and ensure end-to-end behavior
**Description:** As a developer, I want all new commands to be properly registered and working end-to-end so that `maggus --help` lists them and they behave correctly in a real project.

**Acceptance Criteria:**
- [ ] `maggus --help` shows `status` and `list` as available subcommands with short descriptions
- [ ] `maggus status --help` shows the `--plain` flag
- [ ] `maggus list --help` shows the `--count` flag
- [ ] Running `maggus status` in a directory with no `.maggus/` folder prints a helpful message and exits 0 (not a panic)
- [ ] Running `maggus list` in a directory with no `.maggus/` folder prints a helpful message and exits 0
- [ ] Include validation warnings appear before the first iteration spinner, not after
- [ ] All existing tests still pass (`go test ./...`)
- [ ] Typecheck/lint passes (`go vet ./...`)

---

## Functional Requirements

- FR-1: `maggus status` must parse all plan files in `.maggus/` using the existing `parser.ParsePlans()` function — do not duplicate parsing logic
- FR-2: The progress bar must always be exactly 10 characters wide and fill proportionally (round down for partial fills)
- FR-3: Color output must use ANSI escape codes directly (no third-party color library unless already present) — check if one is already used first
- FR-4: The `--plain` flag must disable all ANSI codes AND replace Unicode symbols with ASCII equivalents
- FR-5: `maggus list` must use the same task ordering as `maggus work` (i.e., same plan file order, same task order within each plan)
- FR-6: Include validation must happen in `cmd/work.go` after `config.Load()`, before the first iteration
- FR-7: `ValidateIncludes()` must return only paths that pass `os.Stat()` without error — do not attempt to open/read the files
- FR-8: The "next task" highlight in `maggus status` must use the same `FindNextIncomplete()` logic already in `internal/parser/` — do not reimplement
- FR-9: All new commands must handle missing `.maggus/` directory gracefully (no panic, helpful message, exit 0)

---

## Non-Goals

- No interactive TUI (no cursor movement, no keyboard navigation) — static output only
- No run history in `maggus status` — only plan/task progress
- No `maggus status --watch` mode (auto-refresh) — out of scope for this plan
- No JSON output format — `--plain` is sufficient for scripting
- No per-task description in `maggus status` detail view — title only to keep it compact
- Include validation does NOT block execution — it only warns

---

## Design Considerations

**`maggus status` output layout (color mode):**
```
Maggus Status — 3 plans, 14 tasks total

 Plans
 ──────────────────────────────────────────
 ✓ plan_1_completed.md    [██████████]  7/7   done
 ✓ plan_2_completed.md    [██████████]  6/6   done
   plan_3.md              [████████░░]  4/6   in progress

 Summary: 17/19 tasks complete · 2 pending · 0 blocked

 Tasks — plan_3.md
 ──────────────────────────────────────────
 ✓  TASK-301: Add config file parsing
 ✓  TASK-302: Add model alias resolution
 ✓  TASK-303: Pass model to Claude CLI
 ✓  TASK-304: Add --model CLI flag
→  TASK-305: Add custom markdown includes     ← next up (highlighted cyan)
 ○  TASK-306: Verify end-to-end config wiring
```

**`maggus list` output layout:**
```
Next 5 tasks:

 #1  TASK-305: Add custom markdown includes
     As a developer, I want to include custom markdown files in the prompt...

 #2  TASK-306: Verify end-to-end config wiring
     As a developer, I want to verify the full config system works...
```

**Color scheme:**
- Green (`\033[32m`) — completed tasks/plans
- Cyan (`\033[36m`) — next task, "→" arrow
- Yellow (`\033[33m`) — in-progress plans
- Red (`\033[31m`) — blocked tasks
- Dim (`\033[2m`) — completed plan archives
- Reset (`\033[0m`) — always reset after colored segments

---

## Technical Considerations

- Reuse `internal/parser.ParsePlans()` — do not duplicate plan parsing
- Reuse `internal/parser.FindNextIncomplete()` — do not reimplement next-task logic
- New commands live in `cmd/status.go` and `cmd/list.go`
- Include validation helper lives in `internal/config/validate.go`
- Terminal width is already available via `golang.org/x/term` (used in runner) — use it to cap line width if needed
- No new external dependencies — use only what's already in `go.mod`

---

## Success Metrics

- Running `maggus status` takes under 100ms on a project with 5 plans and 40 tasks
- A developer can answer "what's next?" without opening any file
- A misconfigured `include` entry surfaces as a visible warning on every run — no more silent empty includes
- `maggus list 3` is a fast alternative to `maggus work --dry-run` for previewing upcoming work

---

## Open Questions

- Should `maggus status` also show a "last run" one-liner (e.g., "Last run: 2026-03-10, 3 tasks, branch feature/maggustask-301")? Currently excluded per scope, but easy to add from run tracking data.
- Should `maggus list` accept a `--from TASK-ID` flag to preview from a specific task rather than the next one? Deferred for now.
