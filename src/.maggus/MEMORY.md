# Maggus Project Memory

## Project Structure
- Go source lives in `src/` (module root); run `go` commands from there
- CLI commands in `src/cmd/`, internal packages in `src/internal/`
- Plan files in `.maggus/plan_*.md`, config in `.maggus/config.yml`
- Run data in `.maggus/runs/<RUN_ID>/`

## Build & Test
- Build: `cd src && go build -o maggus .`
- Test: `cd src && go test ./...`
- Vet/Lint: `cd src && go vet ./...`
- CI runs build + test on PRs to master

## Architecture Notes
- Agent abstraction in `internal/agent/` — `ClaudeAgent` implements streaming JSON parsing
- Stream events use `streamEvent` struct; usage data in `streamUsage` (snake_case JSON tags)
- Messages to bubbletea TUI via typed msgs: `StatusMsg`, `OutputMsg`, `ToolMsg`, `UsageMsg`, etc.
- Go files use tabs, CRLF line endings (Windows repo)

## Token Usage Tracking (In Progress)
- Plan: `.maggus/plan_token_usage_tracking.md`
- TASK-001 complete: cache token fields added to `streamUsage` and `UsageMsg`
- Remaining: cost field (TASK-002), per-model usage (TASK-003), TUI state (TASK-004/005), display (TASK-006/007), CSV persistence (TASK-008)
