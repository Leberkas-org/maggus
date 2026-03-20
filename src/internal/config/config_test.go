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
	if cfg.Worktree {
		t.Errorf("expected Worktree to be false, got true")
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

func TestLoad_WithWorktreeTrue(t *testing.T) {
	dir := t.TempDir()
	maggusDir := filepath.Join(dir, ".maggus")
	if err := os.MkdirAll(maggusDir, 0o755); err != nil {
		t.Fatal(err)
	}

	content := `model: opus
worktree: true
`
	if err := os.WriteFile(filepath.Join(maggusDir, "config.yml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.Model != "opus" {
		t.Errorf("Model = %q, want %q", cfg.Model, "opus")
	}
	if !cfg.Worktree {
		t.Errorf("expected Worktree to be true, got false")
	}
}

func TestLoad_WithWorktreeFalse(t *testing.T) {
	dir := t.TempDir()
	maggusDir := filepath.Join(dir, ".maggus")
	if err := os.MkdirAll(maggusDir, 0o755); err != nil {
		t.Fatal(err)
	}

	content := `model: sonnet
worktree: false
`
	if err := os.WriteFile(filepath.Join(maggusDir, "config.yml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.Worktree {
		t.Errorf("expected Worktree to be false, got true")
	}
}

func TestLoad_WithoutWorktreeKey(t *testing.T) {
	dir := t.TempDir()
	maggusDir := filepath.Join(dir, ".maggus")
	if err := os.MkdirAll(maggusDir, 0o755); err != nil {
		t.Fatal(err)
	}

	content := `model: haiku
include:
  - README.md
`
	if err := os.WriteFile(filepath.Join(maggusDir, "config.yml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.Worktree {
		t.Errorf("expected Worktree to default to false when key is absent, got true")
	}
}

func TestLoad_WithNotifications(t *testing.T) {
	dir := t.TempDir()
	maggusDir := filepath.Join(dir, ".maggus")
	if err := os.MkdirAll(maggusDir, 0o755); err != nil {
		t.Fatal(err)
	}

	content := `model: sonnet
notifications:
  sound: true
  on_task_complete: true
  on_run_complete: false
  on_error: true
`
	if err := os.WriteFile(filepath.Join(maggusDir, "config.yml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !cfg.Notifications.Sound {
		t.Error("expected Sound to be true")
	}
	if !cfg.Notifications.IsTaskCompleteEnabled() {
		t.Error("expected task complete to be enabled")
	}
	if cfg.Notifications.IsRunCompleteEnabled() {
		t.Error("expected run complete to be disabled")
	}
	if !cfg.Notifications.IsErrorEnabled() {
		t.Error("expected error to be enabled")
	}
}

func TestLoad_NotificationsDefaults(t *testing.T) {
	dir := t.TempDir()
	maggusDir := filepath.Join(dir, ".maggus")
	if err := os.MkdirAll(maggusDir, 0o755); err != nil {
		t.Fatal(err)
	}

	content := `model: sonnet
`
	if err := os.WriteFile(filepath.Join(maggusDir, "config.yml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.Notifications.Sound {
		t.Error("expected Sound to default to false")
	}
	if cfg.Notifications.IsTaskCompleteEnabled() {
		t.Error("expected task complete to be disabled when sound is false")
	}
	if cfg.Notifications.IsRunCompleteEnabled() {
		t.Error("expected run complete to be disabled when sound is false")
	}
	if cfg.Notifications.IsErrorEnabled() {
		t.Error("expected error to be disabled when sound is false")
	}
}

func TestLoad_WithAgentField(t *testing.T) {
	dir := t.TempDir()
	maggusDir := filepath.Join(dir, ".maggus")
	if err := os.MkdirAll(maggusDir, 0o755); err != nil {
		t.Fatal(err)
	}

	content := `agent: opencode
model: anthropic/claude-sonnet-4-6
`
	if err := os.WriteFile(filepath.Join(maggusDir, "config.yml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.Agent != "opencode" {
		t.Errorf("Agent = %q, want %q", cfg.Agent, "opencode")
	}
	if cfg.Model != "anthropic/claude-sonnet-4-6" {
		t.Errorf("Model = %q, want %q", cfg.Model, "anthropic/claude-sonnet-4-6")
	}
}

func TestLoad_DefaultAgent(t *testing.T) {
	dir := t.TempDir()
	maggusDir := filepath.Join(dir, ".maggus")
	if err := os.MkdirAll(maggusDir, 0o755); err != nil {
		t.Fatal(err)
	}

	content := `model: sonnet
`
	if err := os.WriteFile(filepath.Join(maggusDir, "config.yml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.Agent != "claude" {
		t.Errorf("Agent = %q, want %q", cfg.Agent, "claude")
	}
}

func TestLoad_MissingFileDefaultAgent(t *testing.T) {
	dir := t.TempDir()

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.Agent != "claude" {
		t.Errorf("Agent = %q, want %q", cfg.Agent, "claude")
	}
}

func TestResolveModel_ProviderModelFormat(t *testing.T) {
	cases := map[string]string{
		"anthropic/claude-sonnet-4-6": "anthropic/claude-sonnet-4-6",
		"openai/gpt-4.1":              "openai/gpt-4.1",
		"google/gemini-pro":           "google/gemini-pro",
	}
	for input, want := range cases {
		if got := ResolveModel(input); got != want {
			t.Errorf("ResolveModel(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestResolveModel_LegacyAliasesResolveToModelID(t *testing.T) {
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

func TestResolveModel_BareModelIDPassthrough(t *testing.T) {
	inputs := []string{"claude-sonnet-4-6", "gpt-4", "my-custom-model"}
	for _, input := range inputs {
		if got := ResolveModel(input); got != input {
			t.Errorf("ResolveModel(%q) = %q, want %q", input, got, input)
		}
	}
}

func TestModelID(t *testing.T) {
	cases := map[string]string{
		"anthropic/claude-sonnet-4-6": "claude-sonnet-4-6",
		"openai/gpt-4.1":              "gpt-4.1",
		"claude-sonnet-4-6":           "claude-sonnet-4-6",
		"":                            "",
	}
	for input, want := range cases {
		if got := ModelID(input); got != want {
			t.Errorf("ModelID(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestModelProvider(t *testing.T) {
	cases := map[string]string{
		"anthropic/claude-sonnet-4-6": "anthropic",
		"openai/gpt-4.1":              "openai",
		"claude-sonnet-4-6":           "",
		"":                            "",
	}
	for input, want := range cases {
		if got := ModelProvider(input); got != want {
			t.Errorf("ModelProvider(%q) = %q, want %q", input, got, want)
		}
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
