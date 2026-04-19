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
	ActiveRepo    string             `yaml:"active_repo,omitempty"`
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

func Save(cfg Config) error {
	path, err := globalConfigPath()
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	return os.WriteFile(path, data, 0o644)
}

func AddRepo(repoPath string) error {
	cfg, err := Load()
	if err != nil {
		return err
	}

	abs, err := filepath.Abs(repoPath)
	if err != nil {
		return fmt.Errorf("resolve path: %w", err)
	}

	for _, r := range cfg.Repos {
		existing, _ := filepath.Abs(r.Path)
		if existing == abs {
			return fmt.Errorf("repo already registered: %s", abs)
		}
	}

	cfg.Repos = append(cfg.Repos, RepoEntry{Path: abs})
	return Save(cfg)
}

func RemoveRepo(repoPath string) error {
	cfg, err := Load()
	if err != nil {
		return err
	}

	abs, err := filepath.Abs(repoPath)
	if err != nil {
		return fmt.Errorf("resolve path: %w", err)
	}

	filtered := cfg.Repos[:0]
	found := false
	for _, r := range cfg.Repos {
		existing, _ := filepath.Abs(r.Path)
		if existing == abs {
			found = true
			continue
		}
		filtered = append(filtered, r)
	}

	if !found {
		return fmt.Errorf("repo not registered: %s", abs)
	}

	cfg.Repos = filtered
	return Save(cfg)
}

func ListRepos() ([]RepoEntry, error) {
	cfg, err := Load()
	if err != nil {
		return nil, err
	}
	return cfg.Repos, nil
}

func SetActiveRepo(repoPath string) error {
	cfg, err := Load()
	if err != nil {
		return err
	}
	abs, err := filepath.Abs(repoPath)
	if err != nil {
		return err
	}
	cfg.ActiveRepo = abs
	return Save(cfg)
}

func GetActiveRepo() *RepoEntry {
	cfg, err := Load()
	if err != nil || cfg.ActiveRepo == "" {
		if len(cfg.Repos) > 0 {
			return &cfg.Repos[0]
		}
		return nil
	}
	for i := range cfg.Repos {
		abs, _ := filepath.Abs(cfg.Repos[i].Path)
		if abs == cfg.ActiveRepo {
			return &cfg.Repos[i]
		}
	}
	if len(cfg.Repos) > 0 {
		return &cfg.Repos[0]
	}
	return nil
}
