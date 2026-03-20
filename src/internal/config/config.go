package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
	Agent         string              `yaml:"agent"`
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
			return Config{Agent: "claude"}, nil
		}
		return Config{}, fmt.Errorf("read config %s: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config %s: %w", path, err)
	}

	if cfg.Agent == "" {
		cfg.Agent = "claude"
	}

	return cfg, nil
}

// DefaultAgent returns the default agent name used when none is configured.
func DefaultAgent() string {
	return "claude"
}

// modelAliases maps short alias names to full model IDs.
var modelAliases = map[string]string{
	"sonnet": "claude-sonnet-4-6",
	"opus":   "claude-opus-4-6",
	"haiku":  "claude-haiku-4-5-20251001",
}

// ResolveModel resolves a model specification to its canonical provider/model format.
// It handles:
//   - Short aliases: "sonnet" → "claude-sonnet-4-6"
//   - Already in provider/model format: returned unchanged
//   - Bare model IDs (no provider prefix): returned unchanged for backwards compatibility
//   - Empty string: returns empty string
func ResolveModel(input string) string {
	if input == "" {
		return ""
	}
	if id, ok := modelAliases[input]; ok {
		return id
	}
	return input
}

// ModelID extracts the model ID portion from a provider/model string.
// If there is no provider prefix, the input is returned unchanged.
// For example: "anthropic/claude-sonnet-4-6" → "claude-sonnet-4-6"
func ModelID(model string) string {
	if _, after, ok := strings.Cut(model, "/"); ok {
		return after
	}
	return model
}

// ModelProvider extracts the provider portion from a provider/model string.
// If there is no provider prefix, it returns an empty string.
// For example: "anthropic/claude-sonnet-4-6" → "anthropic"
func ModelProvider(model string) string {
	if before, _, ok := strings.Cut(model, "/"); ok {
		return before
	}
	return ""
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
