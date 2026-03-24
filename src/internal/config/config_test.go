package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
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

func TestGitConfig_IsAutoBranchEnabled(t *testing.T) {
	tests := []struct {
		name string
		val  *bool
		want bool
	}{
		{"nil defaults to true", nil, true},
		{"explicit true", boolPtr(true), true},
		{"explicit false", boolPtr(false), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gc := GitConfig{AutoBranch: tt.val}
			if got := gc.IsAutoBranchEnabled(); got != tt.want {
				t.Errorf("IsAutoBranchEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGitConfig_IsCheckSyncEnabled(t *testing.T) {
	tests := []struct {
		name string
		val  *bool
		want bool
	}{
		{"nil defaults to true", nil, true},
		{"explicit true", boolPtr(true), true},
		{"explicit false", boolPtr(false), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gc := GitConfig{CheckSync: tt.val}
			if got := gc.IsCheckSyncEnabled(); got != tt.want {
				t.Errorf("IsCheckSyncEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGitConfig_ProtectedBranchList(t *testing.T) {
	tests := []struct {
		name string
		list []string
		want []string
	}{
		{"nil returns defaults", nil, []string{"main", "master", "dev"}},
		{"empty returns defaults", []string{}, []string{"main", "master", "dev"}},
		{"custom list returned as-is", []string{"main", "release"}, []string{"main", "release"}},
		{"filters empty strings", []string{"main", "", "dev"}, []string{"main", "dev"}},
		{"all empty strings returns defaults", []string{"", ""}, []string{"main", "master", "dev"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gc := GitConfig{ProtectedBranches: tt.list}
			got := gc.ProtectedBranchList()
			if len(got) != len(tt.want) {
				t.Fatalf("ProtectedBranchList() len = %d, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("ProtectedBranchList()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestOnCompleteConfig_FeatureAction(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{"zero value returns rename", "", "rename"},
		{"rename returns rename", "rename", "rename"},
		{"delete returns delete", "delete", "delete"},
		{"unknown returns rename", "archive", "rename"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oc := OnCompleteConfig{Feature: tt.value}
			if got := oc.FeatureAction(); got != tt.want {
				t.Errorf("FeatureAction() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestOnCompleteConfig_BugAction(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{"zero value returns rename", "", "rename"},
		{"rename returns rename", "rename", "rename"},
		{"delete returns delete", "delete", "delete"},
		{"unknown returns rename", "archive", "rename"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oc := OnCompleteConfig{Bug: tt.value}
			if got := oc.BugAction(); got != tt.want {
				t.Errorf("BugAction() = %q, want %q", got, tt.want)
			}
		})
	}
}

func boolPtr(b bool) *bool { return &b }

func TestConfig_IsApprovalRequired(t *testing.T) {
	tests := []struct {
		name string
		mode string
		want bool
	}{
		{"zero value defaults to opt-in (required)", "", true},
		{"opt-in requires approval", "opt-in", true},
		{"opt-out does not require approval", "opt-out", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{ApprovalMode: tt.mode}
			if got := cfg.IsApprovalRequired(); got != tt.want {
				t.Errorf("IsApprovalRequired() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConfig_IsAutoContinueEnabled(t *testing.T) {
	tests := []struct {
		name string
		val  *bool
		want bool
	}{
		{"nil defaults to false", nil, false},
		{"explicit true enables auto-continue", boolPtr(true), true},
		{"explicit false disables auto-continue", boolPtr(false), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{AutoContinue: tt.val}
			if got := cfg.IsAutoContinueEnabled(); got != tt.want {
				t.Errorf("IsAutoContinueEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLoad_ApprovalModeDefault(t *testing.T) {
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
	if cfg.ApprovalMode != "opt-in" {
		t.Errorf("ApprovalMode = %q, want %q", cfg.ApprovalMode, "opt-in")
	}
	if !cfg.IsApprovalRequired() {
		t.Error("expected IsApprovalRequired() to be true by default")
	}
}

func TestLoad_ApprovalModeOptOut(t *testing.T) {
	dir := t.TempDir()
	maggusDir := filepath.Join(dir, ".maggus")
	if err := os.MkdirAll(maggusDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := `approval_mode: opt-out
`
	if err := os.WriteFile(filepath.Join(maggusDir, "config.yml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.ApprovalMode != "opt-out" {
		t.Errorf("ApprovalMode = %q, want %q", cfg.ApprovalMode, "opt-out")
	}
	if cfg.IsApprovalRequired() {
		t.Error("expected IsApprovalRequired() to be false for opt-out")
	}
}

func TestLoad_AutoContinueDefault(t *testing.T) {
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
	if cfg.AutoContinue != nil {
		t.Errorf("expected AutoContinue to be nil by default, got %v", *cfg.AutoContinue)
	}
	if cfg.IsAutoContinueEnabled() {
		t.Error("expected IsAutoContinueEnabled() to be false by default")
	}
}

func TestLoad_AutoContinueTrue(t *testing.T) {
	dir := t.TempDir()
	maggusDir := filepath.Join(dir, ".maggus")
	if err := os.MkdirAll(maggusDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := `auto_continue: true
`
	if err := os.WriteFile(filepath.Join(maggusDir, "config.yml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !cfg.IsAutoContinueEnabled() {
		t.Error("expected IsAutoContinueEnabled() to be true")
	}
}

func TestLoad_WithGitConfig(t *testing.T) {
	dir := t.TempDir()
	maggusDir := filepath.Join(dir, ".maggus")
	if err := os.MkdirAll(maggusDir, 0o755); err != nil {
		t.Fatal(err)
	}

	content := `model: sonnet
git:
  auto_branch: false
  protected_branches:
    - main
    - release
  check_sync: false
`
	if err := os.WriteFile(filepath.Join(maggusDir, "config.yml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.Git.IsAutoBranchEnabled() {
		t.Error("expected auto_branch to be disabled")
	}
	if cfg.Git.IsCheckSyncEnabled() {
		t.Error("expected check_sync to be disabled")
	}
	got := cfg.Git.ProtectedBranchList()
	if len(got) != 2 || got[0] != "main" || got[1] != "release" {
		t.Errorf("ProtectedBranchList() = %v, want [main release]", got)
	}
}

func TestLoad_GitConfigDefaults(t *testing.T) {
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
	if !cfg.Git.IsAutoBranchEnabled() {
		t.Error("expected auto_branch to default to enabled")
	}
	if !cfg.Git.IsCheckSyncEnabled() {
		t.Error("expected check_sync to default to enabled")
	}
	got := cfg.Git.ProtectedBranchList()
	if len(got) != 3 {
		t.Errorf("expected 3 default protected branches, got %d", len(got))
	}
}

func TestDaemonPollIntervalDuration_Default(t *testing.T) {
	cfg := Config{}
	if got := cfg.DaemonPollIntervalDuration(); got != 5*time.Minute {
		t.Errorf("DaemonPollIntervalDuration() = %v, want 5m", got)
	}
}

func TestDaemonPollIntervalDuration_Configured(t *testing.T) {
	cfg := Config{DaemonPollInterval: "30s"}
	if got := cfg.DaemonPollIntervalDuration(); got != 30*time.Second {
		t.Errorf("DaemonPollIntervalDuration() = %v, want 30s", got)
	}
}

func TestDaemonPollIntervalDuration_Invalid(t *testing.T) {
	cfg := Config{DaemonPollInterval: "not-a-duration"}
	if got := cfg.DaemonPollIntervalDuration(); got != 5*time.Minute {
		t.Errorf("DaemonPollIntervalDuration() = %v, want 5m (default)", got)
	}
}

func TestLoad_DaemonPollInterval(t *testing.T) {
	dir := t.TempDir()
	maggusDir := filepath.Join(dir, ".maggus")
	if err := os.MkdirAll(maggusDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := `daemon_poll_interval: "2m"
`
	if err := os.WriteFile(filepath.Join(maggusDir, "config.yml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.DaemonPollInterval != "2m" {
		t.Errorf("DaemonPollInterval = %q, want %q", cfg.DaemonPollInterval, "2m")
	}
	if got := cfg.DaemonPollIntervalDuration(); got != 2*time.Minute {
		t.Errorf("DaemonPollIntervalDuration() = %v, want 2m", got)
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
