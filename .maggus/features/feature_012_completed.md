<!-- maggus-id: ede813c8-8fb7-49da-a52a-213cb90e3aef -->
# Feature 012: Deduplicate Spinner Frames

## Introduction

The braille spinner frame sequence `[]string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}` is defined
as a package-level variable in four separate files. This is unnecessary duplication — a change to the
frames (e.g. speed, style) requires editing four places. Extract it into a single shared constant in
`internal/tui/styles` and update all consumers to reference that constant.

## Goals

- Single source of truth for spinner frames across the whole codebase
- No behavior change — purely a code deduplication refactor

## Tasks

### TASK-012-001: Extract spinner frames to shared constant and update all references
**Description:** As a developer, I want a single `SpinnerFrames` constant in `internal/tui/styles` so that
spinner frame definitions are never duplicated.

**Token Estimate:** ~20k tokens
**Predecessors:** none
**Successors:** none
**Parallel:** no

**Acceptance Criteria:**
- [x] `var SpinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}` is added to `src/internal/tui/styles/styles.go`
- [x] `src/cmd/gitsync.go` — remove `syncSpinner` local var, replace all usages with `styles.SpinnerFrames`
- [x] `src/internal/runner/runner.go` — remove `spinnerFrames` local var, replace all usages with `styles.SpinnerFrames`
- [x] `src/cmd/status_model.go` — remove `statusSpinnerFrames` local var, replace all usages with `styles.SpinnerFrames`
- [x] `src/cmd/update.go` — remove `spinnerFrames` local var, replace all usages with `styles.SpinnerFrames`
- [x] `cd src && go build ./...` passes with no errors
- [x] `cd src && go test ./...` passes with no failures

## Task Dependency Graph

```
TASK-012-001
```

| Task | Estimate | Predecessors | Parallel | Model |
|------|----------|--------------|----------|-------|
| TASK-012-001 | ~20k | none | no | haiku |

**Total estimated tokens:** ~20k

## Functional Requirements

- FR-1: `styles.SpinnerFrames` must be exported (capital S) so it is accessible from both `cmd/` and `internal/runner/`
- FR-2: The frame sequence must remain identical: `{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}`
- FR-3: No new files should be created — add the var to the existing `styles.go`

## Non-Goals

- No changes to spinner speed, animation logic, or timing
- No changes to how spinners are rendered or ticked
- No other refactoring beyond the spinner frames deduplication

## Success Metrics

- Zero occurrences of the literal string `"⠋", "⠙"` remain outside of `styles.go` (verifiable with grep)
- Build and tests pass clean
