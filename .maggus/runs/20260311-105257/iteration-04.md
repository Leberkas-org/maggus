# Iteration 04 Log

## Task
**TASK-405: Include file validation with warnings**

## Changes Made

### `src/internal/config/config.go`
- Added `ValidateIncludes(includes []string, baseDir string) []string` helper
- Uses `os.Stat` to check each path relative to `baseDir`
- Returns only the paths that exist

### `src/internal/config/config_test.go`
- Added `TestValidateIncludes_Empty` — empty input returns empty slice
- Added `TestValidateIncludes_AllValid` — all existing files returned as-is
- Added `TestValidateIncludes_SomeMissing` — only existing files returned
- Added `TestValidateIncludes_AllMissing` — returns empty slice, no error

### `src/cmd/work.go`
- After `config.Load`, call `config.ValidateIncludes(cfg.Include, dir)`
- For each path dropped (missing), print warning to `os.Stderr`:
  `Warning: included file not found and will be skipped: <path>`
- Pass `validIncludes` (not `cfg.Include`) to `prompt.Options.Include`
- Warnings are printed before the startup banner / work loop

## Commands Run
- `go test ./internal/config/... -v` — all 10 tests pass (including 4 new)
- `go build ./...` — succeeds with no errors

## Deviations / Notes
- Warning format uses "will be skipped" rather than "(skipping)" as shown in the plan example; the intent is identical
- No new files created; `ValidateIncludes` added directly to `config.go` (plan mentioned a `validate.go` file but the acceptance criterion just says `internal/config/` — keeping it in `config.go` avoids file proliferation)
- All acceptance criteria met
