package globalconfig

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

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
