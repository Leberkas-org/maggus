package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// NotificationsConfig holds sound notification settings.
type NotificationsConfig struct {
	Sound          bool  `yaml:"sound"`
	OnTaskComplete *bool `yaml:"on_task_complete"`
	OnRunComplete  *bool `yaml:"on_run_complete"`
	OnError        *bool `yaml:"on_error"`
}

// IsTaskCompleteEnabled returns true if task-complete sound should play.
// Defaults to true when sound is enabled and on_task_complete is not explicitly set.
func (n NotificationsConfig) IsTaskCompleteEnabled() bool {
	if !n.Sound {
		return false
	}
	if n.OnTaskComplete != nil {
		return *n.OnTaskComplete
	}
	return true
}

// IsRunCompleteEnabled returns true if run-complete sound should play.
func (n NotificationsConfig) IsRunCompleteEnabled() bool {
	if !n.Sound {
		return false
	}
	if n.OnRunComplete != nil {
		return *n.OnRunComplete
	}
	return true
}

// IsErrorEnabled returns true if error sound should play.
func (n NotificationsConfig) IsErrorEnabled() bool {
	if !n.Sound {
		return false
	}
	if n.OnError != nil {
		return *n.OnError
	}
	return true
}

// Config holds settings read from .maggus/config.yml.
type Config struct {
	Model         string              `yaml:"model"`
	Include       []string            `yaml:"include"`
	Worktree      bool                `yaml:"worktree"`
	Notifications NotificationsConfig `yaml:"notifications"`
}

// Load reads .maggus/config.yml from dir. If the file does not exist,
// it returns a zero-value Config and no error. If the file exists but
// contains invalid YAML, it returns a descriptive error.
func Load(dir string) (Config, error) {
	path := filepath.Join(dir, ".maggus", "config.yml")

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Config{}, nil
		}
		return Config{}, fmt.Errorf("read config %s: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config %s: %w", path, err)
	}

	return cfg, nil
}

// modelAliases maps short alias names to full model IDs.
var modelAliases = map[string]string{
	"sonnet": "claude-sonnet-4-6",
	"opus":   "claude-opus-4-6",
	"haiku":  "claude-haiku-4-5-20251001",
}

// ResolveModel maps a short alias to its full model ID.
// If the input is not a known alias, it is returned unchanged.
// An empty string returns an empty string.
func ResolveModel(input string) string {
	if id, ok := modelAliases[input]; ok {
		return id
	}
	return input
}

// ValidateIncludes checks each path in includes relative to baseDir.
// It returns only the paths that exist. Callers should warn about any
// paths that are dropped (i.e. present in includes but not in the result).
func ValidateIncludes(includes []string, baseDir string) []string {
	valid := make([]string, 0, len(includes))
	for _, p := range includes {
		abs := filepath.Join(baseDir, p)
		if _, err := os.Stat(abs); err == nil {
			valid = append(valid, p)
		}
	}
	return valid
}
