<!-- maggus-id: 7c207472-3e0d-4861-92c3-9a456b557a50 -->
# Feature 017: Plain List Command

## Introduction

Add a CLI-only `list` command that prints all active (non-completed) features and bugs as a plain tab-separated list. Each line includes the filename, plan ID, human-readable title, and approval status. This is intended for scripting and quick overviews without launching the TUI.

## Goals

- Provide a fast, scriptable way to see all active plans and their approval status
- Output tab-separated columns so the output can be piped into `cut`, `awk`, etc.
- No TUI, no interactivity — pure stdout

## Tasks

### TASK-017-001: Add Title field to Plan struct
**Description:** As a developer, I want the `Plan` struct to carry the parsed feature/bug title so that commands can display it without re-reading the file.

**Token Estimate:** ~15k tokens
**Predecessors:** none
**Successors:** TASK-017-002
**Parallel:** no
**Model:** haiku

**Acceptance Criteria:**
- [x] `Plan` struct in `internal/parser/plan.go` gains a `Title string` field
- [x] `LoadPlans` in `internal/parser/plan.go` populates `Title` by calling `ParseFileTitle(f)` for each plan file
- [x] Existing tests in `internal/parser/` still pass (`go test ./internal/parser`)
- [x] `go vet ./...` passes

---

### TASK-017-002: Implement `maggus list` command
**Description:** As a user, I want to run `maggus list` and get a tab-separated list of all active features and bugs so that I can quickly audit plan status from the terminal or scripts.

**Token Estimate:** ~25k tokens
**Predecessors:** TASK-017-001
**Successors:** none
**Parallel:** no

**Acceptance Criteria:**
- [ ] New file `src/cmd/list.go` registers a `list` cobra command on `rootCmd`
- [ ] Command loads all active (non-completed) plans via `stores.NewFileFeatureStore` and `stores.NewFileBugStore`, using `LoadAll(false)` (which skips completed plans)
- [ ] Command loads approval status via `approval.Load(dir)`
- [ ] Each plan is printed as one tab-separated line with exactly four fields in this order: `filename`, `id`, `title`, `approved` — where `filename` is `filepath.Base(plan.File)`, `id` is `plan.ID`, `title` is `plan.Title` (empty string if not set), and `approved` is either `approved` or `unapproved`
- [ ] Output goes to `cmd.OutOrStdout()` (not `fmt.Print`) so it is testable
- [ ] If there are no active plans, the command prints nothing and exits 0
- [ ] `go test ./cmd` passes (add at least one test covering the tab-separated output format using in-memory fakes or a temp directory)
- [ ] `go vet ./...` passes

## Task Dependency Graph

```
TASK-017-001 ──→ TASK-017-002
```

| Task | Estimate | Predecessors | Parallel | Model |
|------|----------|--------------|----------|-------|
| TASK-017-001 | ~15k | none | no | haiku |
| TASK-017-002 | ~25k | 001 | no | — |

**Total estimated tokens:** ~40k

## Functional Requirements

- FR-1: Output is tab-separated with four fields per line: `filename`, `id`, `title`, `approved`
- FR-2: `filename` is the base name of the plan file (e.g. `feature_003.md`), not the full path
- FR-3: `id` is the plan ID (e.g. `feature_003`)
- FR-4: `title` is the human-readable title from the `# Feature NNN: Title` or `# Bug NNN: Title` heading; empty string if the heading is missing
- FR-5: `approved` is the literal string `approved` if the plan's approval key resolves to true in the approval store, otherwise `unapproved`
- FR-6: Only active (non-completed) plans are listed; completed plans are excluded
- FR-7: Bugs appear before features (matching the existing load order from `loadAllPlans`)

## Non-Goals

- No filtering flags (e.g. `--bugs-only`, `--approved-only`) in this iteration
- No color or formatting — plain text only
- No `--all` flag to include completed plans in this iteration
- No changes to the interactive menu or TUI

## Technical Considerations

- Use the existing `loadAllPlans` helper in `cmd/approve.go` (or duplicate the pattern) — `loadAllPlans` calls `bugStore.LoadAll(false)` then `featureStore.LoadAll(false)`, which already excludes completed plans
- `Plan.Title` will be populated in TASK-017-001; the list command just reads `plan.Title`
- Approval lookup: `approval.Load(dir)` returns `map[string]bool`; use `plan.ApprovalKey()` to index it
- Register the command via `init()` in `list.go`, consistent with `approve.go` and other commands

## Success Metrics

- `maggus list` outputs one line per active plan, tab-separated, with correct approval status
- Output can be parsed by standard Unix tools (`cut -f3` returns the title column)
