package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaults(t *testing.T) {
	cfg := defaults()
	if cfg.Agent != "claude" {
		t.Errorf("default agent = %q, want %q", cfg.Agent, "claude")
	}
	if cfg.MaxWorkers != 1 {
		t.Errorf("default max_workers = %d, want 1", cfg.MaxWorkers)
	}
	if cfg.AutoApprove {
		t.Error("default auto_approve should be false")
	}
	if len(cfg.Git.ProtectedBranches) != 3 {
		t.Errorf("default protected branches = %d, want 3", len(cfg.Git.ProtectedBranches))
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name:    "valid defaults",
			cfg:     defaults(),
			wantErr: false,
		},
		{
			name:    "invalid max_workers",
			cfg:     Config{Agent: "claude", MaxWorkers: 0},
			wantErr: true,
		},
		{
			name:    "invalid agent",
			cfg:     Config{Agent: "unknown", MaxWorkers: 1},
			wantErr: true,
		},
		{
			name: "bryan missing address",
			cfg: Config{
				Agent:      "claude",
				MaxWorkers: 1,
				Bryan:      &BryanConfig{MachineID: "abc"},
			},
			wantErr: true,
		},
		{
			name: "bryan missing machine_id",
			cfg: Config{
				Agent:      "claude",
				MaxWorkers: 1,
				Bryan:      &BryanConfig{Address: "localhost:443"},
			},
			wantErr: true,
		},
		{
			name: "valid with bryan",
			cfg: Config{
				Agent:      "claude",
				MaxWorkers: 2,
				Bryan:      &BryanConfig{Address: "localhost:443", MachineID: "abc"},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

func TestLoadFile_NotExist(t *testing.T) {
	cfg, err := loadFile("/nonexistent/config.yml", defaults())
	if err != nil {
		t.Fatalf("loadFile nonexistent should return defaults, got error: %v", err)
	}
	if cfg.Agent != "claude" {
		t.Errorf("expected defaults, got agent = %q", cfg.Agent)
	}
}

func TestLoadFile_ValidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")
	content := []byte("agent: opencode\nmax_workers: 4\nauto_approve: true\n")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := loadFile(path, defaults())
	if err != nil {
		t.Fatalf("loadFile: %v", err)
	}
	if cfg.Agent != "opencode" {
		t.Errorf("agent = %q, want %q", cfg.Agent, "opencode")
	}
	if cfg.MaxWorkers != 4 {
		t.Errorf("max_workers = %d, want 4", cfg.MaxWorkers)
	}
	if !cfg.AutoApprove {
		t.Error("auto_approve should be true")
	}
	// defaults should be preserved for unset fields
	if len(cfg.Git.ProtectedBranches) != 3 {
		t.Errorf("protected branches should be preserved from defaults, got %d", len(cfg.Git.ProtectedBranches))
	}
}

func TestLoadFile_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(path, []byte("{{invalid"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := loadFile(path, defaults())
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestIsProtectedBranch(t *testing.T) {
	cfg := defaults()
	if !cfg.IsProtectedBranch("main") {
		t.Error("main should be protected")
	}
	if !cfg.IsProtectedBranch("master") {
		t.Error("master should be protected")
	}
	if cfg.IsProtectedBranch("feature/foo") {
		t.Error("feature/foo should not be protected")
	}
}

func TestBryanEnabled(t *testing.T) {
	cfg := defaults()
	if cfg.BryanEnabled() {
		t.Error("should be false without bryan config")
	}
	cfg.Bryan = &BryanConfig{Address: "localhost:443", MachineID: "abc"}
	if !cfg.BryanEnabled() {
		t.Error("should be true with bryan config")
	}
}

func TestLoadRepo_Overlay(t *testing.T) {
	// Set up a temporary global config
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("USERPROFILE", tmpHome) // Windows

	globalDir := filepath.Join(tmpHome, ".maggus")
	if err := os.MkdirAll(globalDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(globalDir, "config.yml"), []byte("agent: claude\nmax_workers: 2\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Set up repo config that overrides agent
	repoDir := t.TempDir()
	repoMaggus := filepath.Join(repoDir, ".maggus")
	if err := os.MkdirAll(repoMaggus, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoMaggus, "config.yml"), []byte("agent: opencode\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadRepo(repoDir)
	if err != nil {
		t.Fatalf("LoadRepo: %v", err)
	}
	if cfg.Agent != "opencode" {
		t.Errorf("agent = %q, want %q (repo override)", cfg.Agent, "opencode")
	}
	if cfg.MaxWorkers != 2 {
		t.Errorf("max_workers = %d, want 2 (from global)", cfg.MaxWorkers)
	}
}
