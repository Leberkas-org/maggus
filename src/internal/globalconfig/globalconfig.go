package globalconfig

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
	Path string `yaml:"path"`
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
