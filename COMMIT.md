feat: pass selected model to Claude CLI (TASK-303)

- Add model parameter to runner.RunClaude; when non-empty, appends
  --model flag to the claude command arguments
- Load config and resolve model alias in the work command
- Display resolved model name (or "default") in the startup banner
- Pass resolved model name to run tracker instead of hardcoded "claude"
