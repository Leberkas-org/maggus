package config

import (
	"fmt"
	"os"
	"path/filepath"
)

type RepoRegistry struct {
	repos []RepoEntry
}

func NewRepoRegistry(cfg Config) (*RepoRegistry, error) {
	r := &RepoRegistry{}
	for _, entry := range cfg.Repos {
		abs, err := filepath.Abs(entry.Path)
		if err != nil {
			return nil, fmt.Errorf("resolve repo path %q: %w", entry.Path, err)
		}
		entry.Path = abs
		r.repos = append(r.repos, entry)
	}
	return r, nil
}

func (r *RepoRegistry) All() []RepoEntry {
	out := make([]RepoEntry, len(r.repos))
	copy(out, r.repos)
	return out
}

func (r *RepoRegistry) ByPath(path string) (RepoEntry, bool) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return RepoEntry{}, false
	}
	for _, repo := range r.repos {
		if repo.Path == abs {
			return repo, true
		}
	}
	return RepoEntry{}, false
}

func (r *RepoRegistry) ByURL(url string) (RepoEntry, bool) {
	for _, repo := range r.repos {
		if repo.URL == url {
			return repo, true
		}
	}
	return RepoEntry{}, false
}

func (r *RepoRegistry) TasksDir(repo RepoEntry) string {
	return filepath.Join(repo.Path, ".maggus", "tasks")
}

func (r *RepoRegistry) LogsDir(repo RepoEntry) string {
	return filepath.Join(repo.Path, ".maggus", "logs")
}

func (r *RepoRegistry) WorktreesDir(repo RepoEntry) string {
	return filepath.Join(repo.Path, ".maggus", "worktrees")
}

func (r *RepoRegistry) EnsureDirs(repo RepoEntry) error {
	dirs := []string{
		r.TasksDir(repo),
		r.LogsDir(repo),
		r.WorktreesDir(repo),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create dir %s: %w", dir, err)
		}
	}
	return nil
}
