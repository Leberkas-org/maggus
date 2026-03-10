feat: verify end-to-end config wiring in work command (TASK-306)

All config integration was already implemented across TASK-302 through
TASK-305. Verified that config loading, model override, model resolution,
and include passthrough are correctly wired in the work command. Build
and all tests pass.
