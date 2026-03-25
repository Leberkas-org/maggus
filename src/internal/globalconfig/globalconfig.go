package globalconfig

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// AutoUpdateMode represents the auto-update behavior setting.
type AutoUpdateMode string

const (
	AutoUpdateOff    AutoUpdateMode = "off"
	AutoUpdateNotify AutoUpdateMode = "notify"
	AutoUpdateAuto   AutoUpdateMode = "auto"
)

// ValidAutoUpdateModes returns all valid auto-update mode values.
func ValidAutoUpdateModes() []AutoUpdateMode {
	return []AutoUpdateMode{AutoUpdateOff, AutoUpdateNotify, AutoUpdateAuto}
}

// IsValid returns true if the mode is a recognized value.
func (m AutoUpdateMode) IsValid() bool {
	switch m {
	case AutoUpdateOff, AutoUpdateNotify, AutoUpdateAuto:
		return true
	}
	return false
}

// Metrics holds lifetime usage counters that accumulate across all sessions.
type Metrics struct {
	StartupCount      int64 `yaml:"startup_count,omitempty" json:"-"`
	WorkRuns          int64 `yaml:"work_runs,omitempty" json:"-"`
	TasksCompleted    int64 `yaml:"tasks_completed,omitempty" json:"-"`
	TasksFailed       int64 `yaml:"tasks_failed,omitempty" json:"-"`
	TasksSkipped      int64 `yaml:"tasks_skipped,omitempty" json:"-"`
	FeaturesCompleted int64 `yaml:"features_completed,omitempty" json:"-"`
	BugsCompleted     int64 `yaml:"bugs_completed,omitempty" json:"-"`
	TokensUsed        int64 `yaml:"tokens_used,omitempty" json:"-"`
	AgentErrors       int64 `yaml:"agent_errors,omitempty" json:"-"`
	GitCommits        int64 `yaml:"git_commits,omitempty" json:"-"`
}

// isZero returns true if all fields are zero (no increment needed).
func (m Metrics) isZero() bool {
	return m == Metrics{}
}

// add returns a new Metrics with each field summed from m and delta.
func (m Metrics) add(delta Metrics) Metrics {
	return Metrics{
		StartupCount:      m.StartupCount + delta.StartupCount,
		WorkRuns:          m.WorkRuns + delta.WorkRuns,
		TasksCompleted:    m.TasksCompleted + delta.TasksCompleted,
		TasksFailed:       m.TasksFailed + delta.TasksFailed,
		TasksSkipped:      m.TasksSkipped + delta.TasksSkipped,
		FeaturesCompleted: m.FeaturesCompleted + delta.FeaturesCompleted,
		BugsCompleted:     m.BugsCompleted + delta.BugsCompleted,
		TokensUsed:        m.TokensUsed + delta.TokensUsed,
		AgentErrors:       m.AgentErrors + delta.AgentErrors,
		GitCommits:        m.GitCommits + delta.GitCommits,
	}
}

// Settings holds the global Maggus settings stored at ~/.maggus/config.yml.
type Settings struct {
	AutoUpdate AutoUpdateMode `yaml:"auto_update,omitempty"`
}

// DefaultSettings returns settings with default values.
func DefaultSettings() Settings {
	return Settings{
		AutoUpdate: AutoUpdateNotify,
	}
}

// SettingsFilePath returns the full path to the global settings file.
func SettingsFilePath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yml"), nil
}

// LoadSettings reads the global settings from ~/.maggus/config.yml.
// If the file does not exist, it returns default settings and no error.
func LoadSettings() (Settings, error) {
	path, err := SettingsFilePath()
	if err != nil {
		return DefaultSettings(), err
	}
	return LoadSettingsFrom(path)
}

// LoadSettingsFrom reads the global settings from the given path.
// If the file does not exist, it returns default settings and no error.
func LoadSettingsFrom(path string) (Settings, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return DefaultSettings(), nil
		}
		return DefaultSettings(), fmt.Errorf("read global settings %s: %w", path, err)
	}

	s := DefaultSettings()
	if err := yaml.Unmarshal(data, &s); err != nil {
		return DefaultSettings(), fmt.Errorf("parse global settings %s: %w", path, err)
	}

	// Validate auto_update; fall back to default if unrecognized
	if s.AutoUpdate == "" {
		s.AutoUpdate = AutoUpdateNotify
	} else if !s.AutoUpdate.IsValid() {
		return DefaultSettings(), fmt.Errorf("invalid auto_update value %q in %s", s.AutoUpdate, path)
	}

	return s, nil
}

// SaveSettings writes the global settings to ~/.maggus/config.yml.
func SaveSettings(s Settings) error {
	path, err := SettingsFilePath()
	if err != nil {
		return err
	}
	return SaveSettingsTo(s, path)
}

// SaveSettingsTo writes the global settings to the given path.
func SaveSettingsTo(s Settings, path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create directory %s: %w", dir, err)
	}

	data, err := yaml.Marshal(s)
	if err != nil {
		return fmt.Errorf("marshal global settings: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write global settings %s: %w", path, err)
	}
	return nil
}

const (
	lockRetries    = 10
	lockRetryDelay = 50 * time.Millisecond
	staleLockAge   = 30 * time.Second
)

// metricsMu serializes in-process access; the file lock handles cross-process.
var metricsMu sync.Mutex

// LoadMetricsFrom reads metrics from the given path.
// If the file does not exist, it returns zero metrics and no error.
func LoadMetricsFrom(path string) (Metrics, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Metrics{}, nil
		}
		return Metrics{}, fmt.Errorf("read metrics %s: %w", path, err)
	}

	var m Metrics
	if err := yaml.Unmarshal(data, &m); err != nil {
		return Metrics{}, fmt.Errorf("parse metrics %s: %w", path, err)
	}
	return m, nil
}

// SaveMetricsTo writes metrics to the given path.
func SaveMetricsTo(m Metrics, path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create directory %s: %w", dir, err)
	}

	data, err := yaml.Marshal(m)
	if err != nil {
		return fmt.Errorf("marshal metrics: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write metrics %s: %w", path, err)
	}
	return nil
}

// settingsWithMetrics is used only for reading legacy config.yml files that
// contain an embedded metrics section, for one-time migration.
type settingsWithMetrics struct {
	AutoUpdate AutoUpdateMode `yaml:"auto_update,omitempty"`
	Metrics    Metrics        `yaml:"metrics,omitempty"`
}

// migrateMetricsFromConfig checks if config.yml contains a metrics section
// and metrics.yml does not exist. If so, it copies the metrics to metrics.yml
// and rewrites config.yml without the metrics section.
func migrateMetricsFromConfig(dir string) {
	metricsPath := filepath.Join(dir, "metrics.yml")
	configPath := filepath.Join(dir, "config.yml")

	// Only migrate if metrics.yml doesn't exist yet.
	if _, err := os.Stat(metricsPath); err == nil {
		return
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return
	}

	var legacy settingsWithMetrics
	if err := yaml.Unmarshal(data, &legacy); err != nil {
		return
	}

	if legacy.Metrics.isZero() {
		return
	}

	// Write metrics to new file.
	if err := SaveMetricsTo(legacy.Metrics, metricsPath); err != nil {
		return
	}

	// Rewrite config.yml without metrics.
	clean := Settings{AutoUpdate: legacy.AutoUpdate}
	if clean.AutoUpdate == "" {
		clean.AutoUpdate = AutoUpdateNotify
	}
	_ = SaveSettingsTo(clean, configPath)
}

// IncrementMetrics atomically increments the global metrics by delta.
// It acquires a file lock, reads current metrics, adds delta, and writes back.
func IncrementMetrics(delta Metrics) error {
	dir, err := Dir()
	if err != nil {
		return err
	}
	return IncrementMetricsIn(dir, delta)
}

// IncrementMetricsIn atomically increments metrics in the given config directory.
func IncrementMetricsIn(dir string, delta Metrics) error {
	if delta.isZero() {
		return nil
	}

	metricsMu.Lock()
	defer metricsMu.Unlock()

	var err error

	metricsPath := filepath.Join(dir, "metrics.yml")
	lockPath := filepath.Join(dir, "metrics.lock")

	if err = os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	// Acquire lock with retries.
	var lockFile *os.File
	for i := 0; i < lockRetries; i++ {
		// Remove stale locks.
		if info, err := os.Stat(lockPath); err == nil {
			if time.Since(info.ModTime()) > staleLockAge {
				_ = os.Remove(lockPath)
			}
		}

		lockFile, err = os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			break
		}
		// On Windows, concurrent access may return "Access is denied"
		// instead of os.ErrExist. Retry on any error.
		if i < lockRetries-1 {
			time.Sleep(lockRetryDelay)
		}
	}
	if lockFile == nil {
		return fmt.Errorf("acquire metrics lock: timed out after %d retries", lockRetries)
	}
	_ = lockFile.Close()

	// Ensure lock is always released.
	defer func() { _ = os.Remove(lockPath) }()

	// Migrate legacy metrics from config.yml if needed.
	migrateMetricsFromConfig(dir)

	// Read current metrics.
	current, err := LoadMetricsFrom(metricsPath)
	if err != nil {
		return err
	}

	// Add delta.
	updated := current.add(delta)

	// Write to temp file then rename (atomic).
	tmpPath := metricsPath + ".tmp"
	data, err := yaml.Marshal(updated)
	if err != nil {
		return fmt.Errorf("marshal metrics: %w", err)
	}

	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return fmt.Errorf("write temp metrics: %w", err)
	}

	// On Windows, os.Rename fails if dest exists and is locked.
	// The lockfile prevents concurrent writers, so remove+rename is safe.
	if runtime.GOOS == "windows" {
		_ = os.Remove(metricsPath)
	}

	if err := os.Rename(tmpPath, metricsPath); err != nil {
		return fmt.Errorf("rename metrics: %w", err)
	}

	return nil
}

// UpdateState holds the persistent state for update checks.
type UpdateState struct {
	LastUpdateCheck time.Time `json:"last_update_check"`
}

// UpdateStateFilePath returns the full path to ~/.maggus/update_state.json.
func UpdateStateFilePath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "update_state.json"), nil
}

// LoadUpdateState reads the update state from ~/.maggus/update_state.json.
// If the file does not exist, it returns a zero-value state and no error.
func LoadUpdateState() (UpdateState, error) {
	path, err := UpdateStateFilePath()
	if err != nil {
		return UpdateState{}, err
	}
	return LoadUpdateStateFrom(path)
}

// LoadUpdateStateFrom reads the update state from the given path.
func LoadUpdateStateFrom(path string) (UpdateState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return UpdateState{}, nil
		}
		return UpdateState{}, fmt.Errorf("read update state %s: %w", path, err)
	}

	var state UpdateState
	if err := json.Unmarshal(data, &state); err != nil {
		return UpdateState{}, fmt.Errorf("parse update state %s: %w", path, err)
	}
	return state, nil
}

// SaveUpdateState writes the update state to ~/.maggus/update_state.json.
func SaveUpdateState(state UpdateState) error {
	path, err := UpdateStateFilePath()
	if err != nil {
		return err
	}
	return SaveUpdateStateTo(state, path)
}

// SaveUpdateStateTo writes the update state to the given path.
func SaveUpdateStateTo(state UpdateState, path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create directory %s: %w", dir, err)
	}

	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("marshal update state: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write update state %s: %w", path, err)
	}
	return nil
}

// UpdateCooldown is the minimum duration between update checks.
const UpdateCooldown = 24 * time.Hour

// ShouldCheckUpdate returns true if enough time has passed since the last check.
func ShouldCheckUpdate(state UpdateState, now time.Time) bool {
	if state.LastUpdateCheck.IsZero() {
		return true
	}
	return now.Sub(state.LastUpdateCheck) >= UpdateCooldown
}

// Repository represents a single configured repository entry.
type Repository struct {
	Path              string `yaml:"path"`
	AutoStartDisabled bool   `yaml:"auto_start_disabled,omitempty"`
}

// IsAutoStartEnabled returns true when auto-start is enabled for this repository.
// The zero value of AutoStartDisabled is false, so auto-start is enabled by default.
func (r Repository) IsAutoStartEnabled() bool {
	return !r.AutoStartDisabled
}

// GlobalConfig holds the global Maggus configuration stored at ~/.maggus/repositories.yml.
type GlobalConfig struct {
	Repositories []Repository `yaml:"repositories"`
	LastOpened   string       `yaml:"last_opened,omitempty"`
}

// Dir returns the global config directory path (~/.maggus/).
func Dir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home directory: %w", err)
	}
	return filepath.Join(home, ".maggus"), nil
}

// FilePath returns the full path to the repositories config file.
func FilePath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "repositories.yml"), nil
}

// Load reads the global config from ~/.maggus/repositories.yml.
// If the file does not exist, it returns an empty config and no error.
func Load() (GlobalConfig, error) {
	path, err := FilePath()
	if err != nil {
		return GlobalConfig{}, err
	}
	return LoadFrom(path)
}

// LoadFrom reads the global config from the given path.
// If the file does not exist, it returns an empty config and no error.
func LoadFrom(path string) (GlobalConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return GlobalConfig{}, nil
		}
		return GlobalConfig{}, fmt.Errorf("read global config %s: %w", path, err)
	}

	var cfg GlobalConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return GlobalConfig{}, fmt.Errorf("parse global config %s: %w", path, err)
	}
	return cfg, nil
}

// Save writes the global config to ~/.maggus/repositories.yml.
// It creates the ~/.maggus/ directory if it doesn't exist.
func Save(cfg GlobalConfig) error {
	path, err := FilePath()
	if err != nil {
		return err
	}
	return SaveTo(cfg, path)
}

// SaveTo writes the global config to the given path.
// It creates the parent directory if it doesn't exist.
func SaveTo(cfg GlobalConfig, path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create directory %s: %w", dir, err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal global config: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write global config %s: %w", path, err)
	}
	return nil
}

// AddRepository adds a repository to the config if it's not already present.
// Returns true if the repository was added, false if it already existed.
func (cfg *GlobalConfig) AddRepository(absPath string) bool {
	for _, r := range cfg.Repositories {
		if r.Path == absPath {
			return false
		}
	}
	cfg.Repositories = append(cfg.Repositories, Repository{Path: absPath})
	return true
}

// RemoveRepository removes a repository from the config by path.
// Returns true if the repository was found and removed, false otherwise.
func (cfg *GlobalConfig) RemoveRepository(absPath string) bool {
	for i, r := range cfg.Repositories {
		if r.Path == absPath {
			cfg.Repositories = append(cfg.Repositories[:i], cfg.Repositories[i+1:]...)
			if cfg.LastOpened == absPath {
				cfg.LastOpened = ""
			}
			return true
		}
	}
	return false
}

// SetLastOpened updates the last_opened field to the given path.
func (cfg *GlobalConfig) SetLastOpened(absPath string) {
	cfg.LastOpened = absPath
}

// HasRepository returns true if the given path is in the repository list.
func (cfg *GlobalConfig) HasRepository(absPath string) bool {
	for _, r := range cfg.Repositories {
		if r.Path == absPath {
			return true
		}
	}
	return false
}
