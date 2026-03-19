package globalconfig

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFrom_NonExistent(t *testing.T) {
	cfg, err := LoadFrom(filepath.Join(t.TempDir(), "nope.yml"))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(cfg.Repositories) != 0 {
		t.Fatalf("expected empty repos, got %d", len(cfg.Repositories))
	}
	if cfg.LastOpened != "" {
		t.Fatalf("expected empty last_opened, got %q", cfg.LastOpened)
	}
}

func TestSaveAndLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "repositories.yml")

	cfg := GlobalConfig{
		Repositories: []Repository{
			{Path: "/home/user/project-a"},
			{Path: "/home/user/project-b"},
		},
		LastOpened: "/home/user/project-a",
	}

	if err := SaveTo(cfg, path); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if len(loaded.Repositories) != 2 {
		t.Fatalf("expected 2 repos, got %d", len(loaded.Repositories))
	}
	if loaded.Repositories[0].Path != "/home/user/project-a" {
		t.Fatalf("expected project-a, got %q", loaded.Repositories[0].Path)
	}
	if loaded.Repositories[1].Path != "/home/user/project-b" {
		t.Fatalf("expected project-b, got %q", loaded.Repositories[1].Path)
	}
	if loaded.LastOpened != "/home/user/project-a" {
		t.Fatalf("expected last_opened project-a, got %q", loaded.LastOpened)
	}
}

func TestLoadFrom_InvalidYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.yml")
	os.WriteFile(path, []byte(":::not yaml\n\t\tbad"), 0o644)

	_, err := LoadFrom(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestAddRepository(t *testing.T) {
	var cfg GlobalConfig

	added := cfg.AddRepository("/repo/one")
	if !added {
		t.Fatal("expected first add to return true")
	}
	if len(cfg.Repositories) != 1 {
		t.Fatalf("expected 1 repo, got %d", len(cfg.Repositories))
	}

	// Duplicate should not be added
	added = cfg.AddRepository("/repo/one")
	if added {
		t.Fatal("expected duplicate add to return false")
	}
	if len(cfg.Repositories) != 1 {
		t.Fatalf("expected still 1 repo, got %d", len(cfg.Repositories))
	}

	// Different path should be added
	added = cfg.AddRepository("/repo/two")
	if !added {
		t.Fatal("expected second add to return true")
	}
	if len(cfg.Repositories) != 2 {
		t.Fatalf("expected 2 repos, got %d", len(cfg.Repositories))
	}
}

func TestRemoveRepository(t *testing.T) {
	cfg := GlobalConfig{
		Repositories: []Repository{
			{Path: "/repo/a"},
			{Path: "/repo/b"},
			{Path: "/repo/c"},
		},
		LastOpened: "/repo/b",
	}

	// Remove middle element
	removed := cfg.RemoveRepository("/repo/b")
	if !removed {
		t.Fatal("expected remove to return true")
	}
	if len(cfg.Repositories) != 2 {
		t.Fatalf("expected 2 repos, got %d", len(cfg.Repositories))
	}
	// last_opened should be cleared when removing that repo
	if cfg.LastOpened != "" {
		t.Fatalf("expected last_opened cleared, got %q", cfg.LastOpened)
	}

	// Remove non-existent
	removed = cfg.RemoveRepository("/repo/missing")
	if removed {
		t.Fatal("expected remove of missing to return false")
	}
	if len(cfg.Repositories) != 2 {
		t.Fatalf("expected still 2 repos, got %d", len(cfg.Repositories))
	}
}

func TestRemoveRepository_KeepsLastOpened(t *testing.T) {
	cfg := GlobalConfig{
		Repositories: []Repository{
			{Path: "/repo/a"},
			{Path: "/repo/b"},
		},
		LastOpened: "/repo/a",
	}

	// Removing a different repo should not clear last_opened
	cfg.RemoveRepository("/repo/b")
	if cfg.LastOpened != "/repo/a" {
		t.Fatalf("expected last_opened preserved, got %q", cfg.LastOpened)
	}
}

func TestSetLastOpened(t *testing.T) {
	var cfg GlobalConfig
	cfg.SetLastOpened("/some/path")
	if cfg.LastOpened != "/some/path" {
		t.Fatalf("expected /some/path, got %q", cfg.LastOpened)
	}
}

func TestHasRepository(t *testing.T) {
	cfg := GlobalConfig{
		Repositories: []Repository{
			{Path: "/repo/yes"},
		},
	}

	if !cfg.HasRepository("/repo/yes") {
		t.Fatal("expected HasRepository to return true")
	}
	if cfg.HasRepository("/repo/no") {
		t.Fatal("expected HasRepository to return false")
	}
}

func TestSaveTo_CreatesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "deep", "nested", "dir")
	path := filepath.Join(dir, "repositories.yml")

	cfg := GlobalConfig{
		Repositories: []Repository{{Path: "/test"}},
	}

	if err := SaveTo(cfg, path); err != nil {
		t.Fatalf("save: %v", err)
	}

	// Verify directory was created and file exists
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file should exist: %v", err)
	}
}

func TestLoadSaveRoundTrip_EmptyConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.yml")

	if err := SaveTo(GlobalConfig{}, path); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(loaded.Repositories) != 0 {
		t.Fatalf("expected empty repos, got %d", len(loaded.Repositories))
	}
	if loaded.LastOpened != "" {
		t.Fatalf("expected empty last_opened, got %q", loaded.LastOpened)
	}
}
