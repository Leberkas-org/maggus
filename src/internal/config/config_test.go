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

func TestResolveModel_KnownAliases(t *testing.T) {
	cases := map[string]string{
		"sonnet": "claude-sonnet-4-6",
		"opus":   "claude-opus-4-6",
		"haiku":  "claude-haiku-4-5-20251001",
	}
	for alias, want := range cases {
		if got := ResolveModel(alias); got != want {
			t.Errorf("ResolveModel(%q) = %q, want %q", alias, got, want)
		}
	}
}

func TestResolveModel_UnknownPassthrough(t *testing.T) {
	inputs := []string{"claude-sonnet-4-6", "gpt-4", "my-custom-model"}
	for _, input := range inputs {
		if got := ResolveModel(input); got != input {
			t.Errorf("ResolveModel(%q) = %q, want %q", input, got, input)
		}
	}
}

func TestResolveModel_EmptyString(t *testing.T) {
	if got := ResolveModel(""); got != "" {
		t.Errorf("ResolveModel(\"\") = %q, want \"\"", got)
	}
}

func TestValidateIncludes_Empty(t *testing.T) {
	dir := t.TempDir()
	result := ValidateIncludes([]string{}, dir)
	if len(result) != 0 {
		t.Errorf("expected empty result, got %v", result)
	}
}

func TestValidateIncludes_AllValid(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a.md", "b.md"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(""), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	result := ValidateIncludes([]string{"a.md", "b.md"}, dir)
	if len(result) != 2 {
		t.Errorf("expected 2 results, got %v", result)
	}
}

func TestValidateIncludes_SomeMissing(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "exists.md"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	result := ValidateIncludes([]string{"exists.md", "missing.md"}, dir)
	if len(result) != 1 || result[0] != "exists.md" {
		t.Errorf("expected [exists.md], got %v", result)
	}
}

func TestValidateIncludes_AllMissing(t *testing.T) {
	dir := t.TempDir()
	result := ValidateIncludes([]string{"nope.md", "also-nope.md"}, dir)
	if len(result) != 0 {
		t.Errorf("expected empty result, got %v", result)
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
