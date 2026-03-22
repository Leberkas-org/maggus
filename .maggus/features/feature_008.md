# Feature 008: Streamlined Main Menu Status Line

## Introduction

The main menu summary line currently shows verbose statistics (total tasks, done, blocked) in muted gray. This replaces it with a cleaner display showing only open (workable) counts — feature task count in green, bug task count in red — making the most actionable information immediately visible.

## Goals

- Show only open (workable) task counts on the main menu summary line
- Color-code the counts: feature open count in green, bug open count in red
- Keep the format concise and scannable

## Tasks

### TASK-008-001: Refactor featureSummary to track workable counts and update formatSummaryLine

**Description:** As a user, I want the main menu summary line to show only open task counts with colored numbers so I can quickly see what needs work.

**Token Estimate:** ~30k tokens
**Predecessors:** none
**Successors:** none
**Parallel:** no

**Acceptance Criteria:**
- [ ] `featureSummary` struct includes `workable` (feature) and `bugWorkable` (bug) fields computed from `IsWorkable()` on each task
- [ ] `loadFeatureSummary` populates the new workable fields
- [ ] `formatSummaryLine` is replaced by a new function (or updated signature) that returns a lipgloss-styled string with the new format
- [ ] Format when both exist: `3 features, {5} open tasks · 2 bugs, {3} open tasks` where `{5}` is green and `{3}` is red (only the number is colored, surrounding text stays muted)
- [ ] Format when only features: `3 features, {5} open tasks` (green number)
- [ ] Format when only bugs: `2 bugs, {3} open tasks` (red number)
- [ ] When all counts are zero: `No open tasks` in muted gray
- [ ] "open tasks" means workable: incomplete AND not blocked AND not ignored
- [ ] The summary line in `View()` no longer wraps the result in `mutedStyle.Render()` since the function now returns pre-styled output
- [ ] Existing tests in `menu_test.go` for `formatSummaryLine` are updated to match new format and behavior
- [ ] The `TestMenuView_SummaryShowsFeaturesAndBugs` and `TestMenuView_SummaryNoFeaturesOrBugs` tests are updated
- [ ] `go vet ./...` and `go test ./...` pass

## Task Dependency Graph

```
TASK-008-001
```

| Task | Estimate | Predecessors | Parallel | Model |
|------|----------|--------------|----------|-------|
| TASK-008-001 | ~30k | none | — | — |

**Total estimated tokens:** ~30k

## Functional Requirements

- FR-1: The summary line must show the count of workable feature tasks (incomplete, not blocked, not ignored) with the number rendered in green (`styles.Success`)
- FR-2: The summary line must show the count of workable bug tasks with the number rendered in red (`styles.Error`)
- FR-3: Non-number text (e.g. "features,", "open tasks", "·") must render in muted gray (`styles.Muted`)
- FR-4: When there are zero features and zero bugs, display `No open tasks` in muted gray
- FR-5: The feature count and bug count sections are separated by ` · `
- FR-6: Sections with zero plans of that type are omitted (e.g. no bugs → no bug section shown)

## Non-Goals

- No changes to the interactive status view (`status.go`)
- No changes to the file watcher or its summary update mechanism
- No progress bars or percentage indicators on the main menu

## Technical Considerations

- `formatSummaryLine` currently returns a plain string that gets wrapped in `mutedStyle.Render()` at the call site (`menu.go:557`). The new version needs to return a pre-styled string since different parts have different colors. The call site must stop wrapping in mutedStyle.
- Use `styles.Success` (color "2", green) for feature open count numbers and `styles.Error` (color "1", red) for bug open count numbers, from `src/internal/tui/styles/styles.go`.
- The file watcher sends `featureSummaryUpdateMsg` to reload the summary — this mechanism stays unchanged; only the rendering changes.

## Success Metrics

- Main menu summary line is visually cleaner and shows only actionable counts
- Green/red coloring draws attention to the right numbers at a glance

## Open Questions

None.
