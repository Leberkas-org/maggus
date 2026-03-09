# Iteration 01

## Task
**TASK-004:** Git commit after each iteration

## Commands Run
1. `go vet ./...` — passed, no issues
2. `go test ./internal/gitcommit/ -v` — all 6 tests passed (3 unit for StripCoAuthoredBy, 3 integration for CommitIteration)
3. `go test ./...` — all packages pass
4. `git add` — staged new and modified files

## Changes Made
- **New:** `src/internal/gitcommit/gitcommit.go` — package with `StripCoAuthoredBy` and `CommitIteration` functions
- **New:** `src/internal/gitcommit/gitcommit_test.go` — tests for stripping Co-Authored-By lines, successful commit, missing COMMIT.md, and nothing-staged error
- **Modified:** `src/cmd/work.go` — wired `gitcommit.CommitIteration` into the work loop after each Claude Code invocation

## Acceptance Criteria
- [x] After Claude Code exits successfully, Maggus checks if COMMIT.md exists
- [x] If COMMIT.md exists, Maggus runs `git commit -F COMMIT.md`
- [x] Co-Authored-By lines are stripped before committing
- [x] Commit failure stops the loop (returns error)
- [x] Missing COMMIT.md logs warning and continues
- [x] COMMIT.md deleted after successful commit
- [x] Typecheck/lint passes

## Deviations
None.
