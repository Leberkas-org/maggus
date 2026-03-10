feat: add config file parsing for .maggus/config.yml (TASK-301)

Add internal/config package that reads Model and Include settings
from .maggus/config.yml. Returns zero-value Config when file is
missing, and a descriptive error for invalid YAML.

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
