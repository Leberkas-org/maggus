# Iteration 01 — TASK-301: Config File Parsing

## Task
TASK-301: Config File Parsing — Add `internal/config` package to read `.maggus/config.yml`.

## Actions
1. Read existing codebase structure and conventions (parser package as reference).
2. Created `src/internal/config/config.go` with `Config` struct and `Load(dir)` function.
3. Created `src/internal/config/config_test.go` with three unit tests.
4. Added `gopkg.in/yaml.v3` dependency via `go get`.
5. Ran `go test ./internal/config/ -v` — all 3 tests pass.
6. Ran `go test ./...` — all tests pass; pre-existing build error in `internal/runner` (unrelated).
7. Staged files, wrote COMMIT.md, updated plan checkboxes.

## Acceptance Criteria
- [x] New package `internal/config` with Config struct (Model, Include fields)
- [x] `config.Load(dir string)` reads `.maggus/config.yml`
- [x] Missing file returns zero-value Config (no error)
- [x] Invalid YAML returns descriptive error
- [x] Config file format matches spec (model + include list)
- [x] Unit test: missing file returns empty config
- [x] Unit test: valid YAML parses correctly
- [x] Unit test: invalid YAML returns error

## Deviations
None.
