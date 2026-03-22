# Bug: Metrics are stored in config.yml instead of a dedicated metrics.yml

## Summary

Lifetime usage metrics (startup count, tokens used, tasks completed, etc.) are embedded in the `Settings` struct and persisted to `~/.maggus/config.yml` alongside user configuration like `auto_update`. They should be stored in a separate `~/.maggus/metrics.yml` file to keep user config clean and concerns separated.

## Related

- **Commit:** 2904d84 (feat(globalconfig): add Metrics struct and atomic IncrementMetrics helper, TASK-006-001)
- **Commit:** 7b77860 (feat(metrics): wire metric increments into all callsites, TASK-006-002)

## Steps to Reproduce

1. Run `maggus` and complete some work
2. Open `~/.maggus/config.yml`
3. Observe: metrics counters are mixed in with user settings

```yaml
auto_update: notify
metrics:
  startup_count: 3
  work_runs: 2
  tasks_completed: 4
  tokens_used: 3970819
```

## Expected Behavior

`~/.maggus/config.yml` should contain only user settings (e.g. `auto_update`). Metrics should live in `~/.maggus/metrics.yml`.

## Root Cause

The `Settings` struct at `src/internal/globalconfig/globalconfig.go:74-78` bundles metrics and config together:

```go
type Settings struct {
    AutoUpdate AutoUpdateMode `yaml:"auto_update,omitempty"`
    Metrics    Metrics        `yaml:"metrics,omitempty"`
}
```

`IncrementMetricsIn()` at line 179 reads and writes `config.yml` (line 189):

```go
configPath := filepath.Join(dir, "config.yml")
```

It loads the full `Settings`, increments `settings.Metrics`, and writes the entire struct back — mixing metrics into the user's config file.

## User Stories

### BUG-004-001: Move metrics storage from config.yml to dedicated metrics.yml

**Description:** As a user, I want metrics stored in a separate `metrics.yml` file so that my config.yml stays clean and only contains user settings.

**Acceptance Criteria:**
- [x] New `MetricsSettings` struct (or reuse `Metrics` directly) with its own YAML load/save functions targeting `~/.maggus/metrics.yml`
- [x] `Metrics` field is removed from the `Settings` struct
- [x] `IncrementMetricsIn()` reads/writes `metrics.yml` instead of `config.yml`
- [x] Lock file changes to `metrics.lock` (separate from any future config lock)
- [x] `LoadSettings` / `SaveSettings` no longer touch metrics data
- [x] Existing metrics in `config.yml` are migrated: on first `IncrementMetrics` call, if `metrics.yml` doesn't exist but `config.yml` contains a `metrics:` section, copy the values to `metrics.yml` and remove the section from `config.yml`
- [x] All existing callers of `IncrementMetrics` / `IncrementMetricsIn` continue to work unchanged
- [x] Existing tests in `globalconfig_test.go` are updated for the new file paths
- [x] No regression in concurrent/worktree metric increments (atomic write + lock still works)
- [x] `go vet ./...` and `go test ./...` pass
