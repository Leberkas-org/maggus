package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config holds settings read from .maggus/config.yml.
type Config struct {
	Model   string   `yaml:"model"`
	Include []string `yaml:"include"`
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
