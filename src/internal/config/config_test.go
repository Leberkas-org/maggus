package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_MissingFile(t *testing.T) {
	dir := t.TempDir()

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.Model != "" {
		t.Errorf("expected empty Model, got %q", cfg.Model)
	}
	if len(cfg.Include) != 0 {
		t.Errorf("expected empty Include, got %v", cfg.Include)
	}
}

func TestLoad_ValidYAML(t *testing.T) {
	dir := t.TempDir()
	maggusDir := filepath.Join(dir, ".maggus")
	if err := os.MkdirAll(maggusDir, 0o755); err != nil {
		t.Fatal(err)
	}

	content := `model: sonnet
include:
  - ARCHITECTURE.md
  - docs/PATTERNS.md
`
	if err := os.WriteFile(filepath.Join(maggusDir, "config.yml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.Model != "sonnet" {
		t.Errorf("Model = %q, want %q", cfg.Model, "sonnet")
	}
	if len(cfg.Include) != 2 {
		t.Fatalf("Include length = %d, want 2", len(cfg.Include))
	}
	if cfg.Include[0] != "ARCHITECTURE.md" {
		t.Errorf("Include[0] = %q, want %q", cfg.Include[0], "ARCHITECTURE.md")
	}
	if cfg.Include[1] != "docs/PATTERNS.md" {
		t.Errorf("Include[1] = %q, want %q", cfg.Include[1], "docs/PATTERNS.md")
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	maggusDir := filepath.Join(dir, ".maggus")
	if err := os.MkdirAll(maggusDir, 0o755); err != nil {
		t.Fatal(err)
	}

	content := `model: [invalid
  this is not valid yaml: :::
`
	if err := os.WriteFile(filepath.Join(maggusDir, "config.yml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}
}
