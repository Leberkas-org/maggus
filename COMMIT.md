feat: add --model CLI flag to work command (TASK-304)

Add a --model flag that overrides the config file model for a single run.
Accepts both short aliases (opus, sonnet, haiku) and full model IDs.
When neither CLI flag nor config file specifies a model, no --model flag
is passed to the Claude CLI.
