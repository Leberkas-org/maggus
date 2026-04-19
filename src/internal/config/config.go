package config

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Agent         string             `yaml:"agent"`
	Model         string             `yaml:"model"`
	MaxWorkers    int                `yaml:"max_workers"`
	AutoApprove   bool               `yaml:"auto_approve"`
	Git           GitConfig          `yaml:"git"`
	Bryan         *BryanConfig       `yaml:"bryan"`
	Notifications NotificationConfig `yaml:"notifications"`
	Repos         []RepoEntry        `yaml:"repos"`
}

type GitConfig struct {
	ProtectedBranches []string `yaml:"protected_branches"`
}

type BryanConfig struct {
	Address   string `yaml:"address"`
	MachineID string `yaml:"machine_id"`
}

type NotificationConfig struct {
	OnComplete bool `yaml:"on_complete"`
	OnFailure  bool `yaml:"on_failure"`
}

type RepoEntry struct {
	Path string `yaml:"path"`
	URL  string `yaml:"url"`
}

func GlobalDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}
	return filepath.Join(home, ".maggus"), nil
}

func globalConfigPath() (string, error) {
	dir, err := GlobalDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yml"), nil
}

func defaults() Config {
	return Config{
		Agent:      "claude",
		Model:      "sonnet",
		MaxWorkers: 1,
		Git: GitConfig{
			ProtectedBranches: []string{"main", "master", "dev"},
		},
	}
}

func Load() (Config, error) {
	path, err := globalConfigPath()
	if err != nil {
		return Config{}, err
	}
	return loadFile(path, defaults())
}

func LoadRepo(dir string) (Config, error) {
	base, err := Load()
	if err != nil {
		return Config{}, fmt.Errorf("load global config: %w", err)
	}

	repoPath := filepath.Join(dir, ".maggus", "config.yml")
	return loadFile(repoPath, base)
}

func loadFile(path string, base Config) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return base, nil
		}
		return Config{}, fmt.Errorf("read %s: %w", path, err)
	}

	if err := yaml.Unmarshal(data, &base); err != nil {
		return Config{}, fmt.Errorf("parse %s: %w", path, err)
	}

	return base, nil
}

func (c Config) Validate() error {
	if c.MaxWorkers < 1 {
		return fmt.Errorf("max_workers must be >= 1, got %d", c.MaxWorkers)
	}
	if c.Agent != "claude" && c.Agent != "opencode" {
		return fmt.Errorf("agent must be 'claude' or 'opencode', got %q", c.Agent)
	}
	if c.Bryan != nil {
		if c.Bryan.Address == "" {
			return fmt.Errorf("bryan.address is required when bryan is configured")
		}
		if c.Bryan.MachineID == "" {
			return fmt.Errorf("bryan.machine_id is required when bryan is configured")
		}
	}
	return nil
}

func (c Config) IsProtectedBranch(branch string) bool {
	return slices.Contains(c.Git.ProtectedBranches, branch)
}

func (c Config) BryanEnabled() bool {
	return c.Bryan != nil
}
