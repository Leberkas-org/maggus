package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/leberkas-org/maggus/internal/config"
	"gopkg.in/yaml.v3"
)

// optionRows returns only the setting rows (not action buttons) from both slices.
func optionRows(m configModel) []configRow {
	var opts []configRow
	for _, r := range m.projectRows {
		if r.isOption() {
			opts = append(opts, r)
		}
	}
	for _, r := range m.globalRows {
		if r.isOption() {
			opts = append(opts, r)
		}
	}
	return opts
}

func TestIndexOf(t *testing.T) {
	tests := []struct {
		name   string
		values []string
		target string
		want   int
	}{
		{"found first", []string{"a", "b", "c"}, "a", 0},
		{"found middle", []string{"a", "b", "c"}, "b", 1},
		{"found last", []string{"a", "b", "c"}, "c", 2},
		{"not found returns 0", []string{"a", "b", "c"}, "z", 0},
		{"empty slice returns 0", []string{}, "a", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := indexOf(tt.values, tt.target)
			if got != tt.want {
				t.Errorf("indexOf(%v, %q) = %d, want %d", tt.values, tt.target, got, tt.want)
			}
		})
	}
}

func TestNewConfigModel_Defaults(t *testing.T) {
	cfg := config.Config{Agent: "claude"}
	m := newConfigModel(cfg, "")

	opts := optionRows(m)
	if len(opts) != 14 {
		t.Fatalf("expected 14 option rows, got %d", len(opts))
	}

	// Agent defaults to claude (index 0)
	if opts[0].label != "Agent" {
		t.Errorf("opts[0].label = %q, want Agent", opts[0].label)
	}
	if opts[0].current != 0 {
		t.Errorf("agent current = %d, want 0 (claude)", opts[0].current)
	}

	// Model defaults to (default) (index 0)
	if opts[1].current != 0 {
		t.Errorf("model current = %d, want 0 ((default))", opts[1].current)
	}

	// Worktree defaults to off (index 1)
	if opts[2].current != 1 {
		t.Errorf("worktree current = %d, want 1 (off)", opts[2].current)
	}

	// Auto-approve defaults to disabled (index 0)
	if opts[3].current != 0 {
		t.Errorf("auto-approve current = %d, want 0 (disabled)", opts[3].current)
	}

	// Auto-branch defaults to on (index 0)
	if opts[4].current != 0 {
		t.Errorf("auto-branch current = %d, want 0 (on)", opts[4].current)
	}

	// Check sync defaults to on (index 0)
	if opts[5].current != 0 {
		t.Errorf("check-sync current = %d, want 0 (on)", opts[5].current)
	}

	// Discord presence defaults to off (index 1)
	if opts[6].current != 1 {
		t.Errorf("discord-presence current = %d, want 1 (off)", opts[6].current)
	}

	// Sound defaults to off (index 1)
	if opts[7].current != 1 {
		t.Errorf("sound current = %d, want 1 (off)", opts[7].current)
	}

	// Notification sub-options default to on (index 0) when nil
	for i := 8; i <= 10; i++ {
		if opts[i].current != 0 {
			t.Errorf("opts[%d].current = %d, want 0 (on)", i, opts[i].current)
		}
	}

	// On-complete defaults to rename (index 0)
	if opts[11].label != "  Feature" || opts[11].current != 0 {
		t.Errorf("on-complete feature: label=%q current=%d, want '  Feature' / 0", opts[11].label, opts[11].current)
	}
	if opts[12].label != "  Bug" || opts[12].current != 0 {
		t.Errorf("on-complete bug: label=%q current=%d, want '  Bug' / 0", opts[12].label, opts[12].current)
	}

	if m.cursor != 0 {
		t.Errorf("cursor = %d, want 0", m.cursor)
	}
	if m.action != configActionNone {
		t.Errorf("action = %d, want configActionNone", m.action)
	}
}

func TestNewConfigModel_CustomValues(t *testing.T) {
	f := false
	cfg := config.Config{
		Agent:    "opencode",
		Model:    "opus",
		Worktree: true,
		Notifications: config.NotificationsConfig{
			Sound:          true,
			OnTaskComplete: &f,
			OnRunComplete:  &f,
			OnError:        &f,
		},
	}
	m := newConfigModel(cfg, "")
	opts := optionRows(m)

	if opts[0].current != 1 {
		t.Errorf("agent current = %d, want 1 (opencode)", opts[0].current)
	}
	if opts[1].current != 2 {
		t.Errorf("model current = %d, want 2 (opus)", opts[1].current)
	}
	if opts[2].current != 0 {
		t.Errorf("worktree current = %d, want 0 (on)", opts[2].current)
	}
	// Auto-approve defaults to disabled (index 0) — not set in config
	if opts[3].current != 0 {
		t.Errorf("auto-approve current = %d, want 0 (disabled)", opts[3].current)
	}
	// Auto-branch defaults to on (index 0) — not set in config
	if opts[4].current != 0 {
		t.Errorf("auto-branch current = %d, want 0 (on)", opts[4].current)
	}
	// Check sync defaults to on (index 0) — not set in config
	if opts[5].current != 0 {
		t.Errorf("check-sync current = %d, want 0 (on)", opts[5].current)
	}
	// Discord presence defaults to off (index 1) — not set in config
	if opts[6].current != 1 {
		t.Errorf("discord-presence current = %d, want 1 (off)", opts[6].current)
	}
	if opts[7].current != 0 {
		t.Errorf("sound current = %d, want 0 (on)", opts[7].current)
	}
	// All notification sub-options set to false → off (index 1)
	for i := 8; i <= 10; i++ {
		if opts[i].current != 1 {
			t.Errorf("opts[%d].current = %d, want 1 (off)", i, opts[i].current)
		}
	}
}

func TestBuildConfig_Defaults(t *testing.T) {
	m := newConfigModel(config.Config{Agent: "claude"}, "")
	cfg := m.buildConfig()

	if cfg.Agent != "claude" {
		t.Errorf("agent = %q, want claude", cfg.Agent)
	}
	if cfg.Model != "" {
		t.Errorf("model = %q, want empty (default)", cfg.Model)
	}
	if cfg.Worktree {
		t.Error("worktree should be false")
	}
	if cfg.Notifications.Sound {
		t.Error("sound should be false")
	}
	// When on (default), notification pointers should be nil
	if cfg.Notifications.OnTaskComplete != nil {
		t.Errorf("OnTaskComplete = %v, want nil", cfg.Notifications.OnTaskComplete)
	}
	if cfg.Notifications.OnRunComplete != nil {
		t.Errorf("OnRunComplete = %v, want nil", cfg.Notifications.OnRunComplete)
	}
	if cfg.Notifications.OnError != nil {
		t.Errorf("OnError = %v, want nil", cfg.Notifications.OnError)
	}
}

func TestBuildConfig_CustomValues(t *testing.T) {
	f := false
	cfg := config.Config{
		Agent:    "opencode",
		Model:    "opus",
		Worktree: true,
		Notifications: config.NotificationsConfig{
			Sound:          true,
			OnTaskComplete: &f,
			OnRunComplete:  &f,
			OnError:        &f,
		},
	}
	m := newConfigModel(cfg, "")
	result := m.buildConfig()

	if result.Agent != "opencode" {
		t.Errorf("agent = %q, want opencode", result.Agent)
	}
	if result.Model != "opus" {
		t.Errorf("model = %q, want opus", result.Model)
	}
	if !result.Worktree {
		t.Error("worktree should be true")
	}
	if !result.Notifications.Sound {
		t.Error("sound should be true")
	}
	if result.Notifications.OnTaskComplete == nil || *result.Notifications.OnTaskComplete {
		t.Error("OnTaskComplete should be false")
	}
	if result.Notifications.OnRunComplete == nil || *result.Notifications.OnRunComplete {
		t.Error("OnRunComplete should be false")
	}
	if result.Notifications.OnError == nil || *result.Notifications.OnError {
		t.Error("OnError should be false")
	}
}

func TestBuildConfig_GitSettings(t *testing.T) {
	f := false
	cfg := config.Config{
		Agent: "claude",
		Git: config.GitConfig{
			AutoBranch:        &f,
			CheckSync:         &f,
			ProtectedBranches: []string{"main", "release"},
		},
	}
	m := newConfigModel(cfg, "")
	result := m.buildConfig()

	if result.Git.AutoBranch == nil || *result.Git.AutoBranch {
		t.Error("AutoBranch should be false")
	}
	if result.Git.CheckSync == nil || *result.Git.CheckSync {
		t.Error("CheckSync should be false")
	}
	if len(result.Git.ProtectedBranches) != 2 || result.Git.ProtectedBranches[0] != "main" {
		t.Errorf("ProtectedBranches = %v, want [main release]", result.Git.ProtectedBranches)
	}
}

func TestBuildConfig_GitDefaults(t *testing.T) {
	m := newConfigModel(config.Config{Agent: "claude"}, "")
	result := m.buildConfig()

	// When on (default), git pointers should be nil
	if result.Git.AutoBranch != nil {
		t.Errorf("AutoBranch = %v, want nil", result.Git.AutoBranch)
	}
	if result.Git.CheckSync != nil {
		t.Errorf("CheckSync = %v, want nil", result.Git.CheckSync)
	}
	// Protected branches should be preserved (defaults)
	if len(result.Git.ProtectedBranches) != 3 {
		t.Errorf("ProtectedBranches len = %d, want 3", len(result.Git.ProtectedBranches))
	}
}

func TestBuildConfig_RoundTrip(t *testing.T) {
	original := config.Config{
		Agent:    "claude",
		Model:    "sonnet",
		Worktree: true,
		Notifications: config.NotificationsConfig{
			Sound: true,
		},
	}
	m := newConfigModel(original, "")
	result := m.buildConfig()

	if result.Agent != original.Agent {
		t.Errorf("agent = %q, want %q", result.Agent, original.Agent)
	}
	if result.Model != original.Model {
		t.Errorf("model = %q, want %q", result.Model, original.Model)
	}
	if result.Worktree != original.Worktree {
		t.Errorf("worktree = %v, want %v", result.Worktree, original.Worktree)
	}
	if result.Notifications.Sound != original.Notifications.Sound {
		t.Errorf("sound = %v, want %v", result.Notifications.Sound, original.Notifications.Sound)
	}
}

func TestBuildConfig_IncludeNotSet(t *testing.T) {
	m := newConfigModel(config.Config{Agent: "claude", Include: []string{"foo.md"}}, "")
	result := m.buildConfig()

	if len(result.Include) != 0 {
		t.Errorf("Include = %v, want empty (buildConfig should not set Include)", result.Include)
	}
}

func TestSaveConfig(t *testing.T) {
	dir := t.TempDir()

	cfg := config.Config{
		Agent:    "claude",
		Model:    "sonnet",
		Worktree: true,
		Include:  []string{"README.md"},
		Notifications: config.NotificationsConfig{
			Sound: true,
		},
	}

	if err := saveConfig(dir, cfg); err != nil {
		t.Fatalf("saveConfig() error: %v", err)
	}

	path := filepath.Join(dir, ".maggus", "config.yml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read saved config: %v", err)
	}

	var loaded config.Config
	if err := yaml.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("unmarshal saved config: %v", err)
	}

	if loaded.Agent != cfg.Agent {
		t.Errorf("agent = %q, want %q", loaded.Agent, cfg.Agent)
	}
	if loaded.Model != cfg.Model {
		t.Errorf("model = %q, want %q", loaded.Model, cfg.Model)
	}
	if loaded.Worktree != cfg.Worktree {
		t.Errorf("worktree = %v, want %v", loaded.Worktree, cfg.Worktree)
	}
	if len(loaded.Include) != 1 || loaded.Include[0] != "README.md" {
		t.Errorf("include = %v, want [README.md]", loaded.Include)
	}
}

func TestSaveConfig_CreatesDirectory(t *testing.T) {
	dir := t.TempDir()

	maggusDir := filepath.Join(dir, ".maggus")
	if _, err := os.Stat(maggusDir); !os.IsNotExist(err) {
		t.Fatal(".maggus/ should not exist before saveConfig")
	}

	if err := saveConfig(dir, config.Config{Agent: "claude"}); err != nil {
		t.Fatalf("saveConfig() error: %v", err)
	}

	if _, err := os.Stat(maggusDir); err != nil {
		t.Errorf(".maggus/ should exist after saveConfig: %v", err)
	}
}

func TestSaveConfig_Overwrite(t *testing.T) {
	dir := t.TempDir()

	if err := saveConfig(dir, config.Config{Agent: "claude", Model: "sonnet"}); err != nil {
		t.Fatalf("first saveConfig() error: %v", err)
	}

	if err := saveConfig(dir, config.Config{Agent: "opencode", Model: "opus"}); err != nil {
		t.Fatalf("second saveConfig() error: %v", err)
	}

	path := filepath.Join(dir, ".maggus", "config.yml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	var loaded config.Config
	if err := yaml.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if loaded.Agent != "opencode" {
		t.Errorf("agent = %q, want opencode", loaded.Agent)
	}
	if loaded.Model != "opus" {
		t.Errorf("model = %q, want opus", loaded.Model)
	}
}

func TestConfigRow_Fields(t *testing.T) {
	row := configRow{
		label:   "Agent",
		values:  []string{"claude", "opencode"},
		current: 0,
	}

	if row.label != "Agent" {
		t.Errorf("label = %q, want Agent", row.label)
	}
	if len(row.values) != 2 {
		t.Errorf("values len = %d, want 2", len(row.values))
	}
	if row.current != 0 {
		t.Errorf("current = %d, want 0", row.current)
	}
	if !row.isOption() {
		t.Error("row with values should be an option")
	}

	action := configRow{label: "Save", action: configActionSaveProject}
	if action.isOption() {
		t.Error("action row should not be an option")
	}
}

func TestNewConfigModel_OptionLabels(t *testing.T) {
	m := newConfigModel(config.Config{Agent: "claude"}, "")
	opts := optionRows(m)

	expectedLabels := []string{
		"Agent",
		"Model",
		"Worktree",
		"Auto-approve",
		"Auto-branch",
		"Check sync",
		"Discord presence",
		"Sound",
		"  On task complete",
		"  On run complete",
		"  On error",
		"  Feature",
		"  Bug",
		"Auto-update",
	}

	for i, want := range expectedLabels {
		if i >= len(opts) {
			t.Fatalf("missing option at index %d, want label %q", i, want)
		}
		if opts[i].label != want {
			t.Errorf("opts[%d].label = %q, want %q", i, opts[i].label, want)
		}
	}
}

func TestBuildConfig_OnComplete(t *testing.T) {
	// Default: rename (empty fields)
	m := newConfigModel(config.Config{Agent: "claude"}, "")
	result := m.buildConfig()
	if result.OnComplete.Feature != "" {
		t.Errorf("Feature = %q, want empty (rename default)", result.OnComplete.Feature)
	}
	if result.OnComplete.Bug != "" {
		t.Errorf("Bug = %q, want empty (rename default)", result.OnComplete.Bug)
	}

	// With delete configured
	cfg := config.Config{
		Agent:      "claude",
		OnComplete: config.OnCompleteConfig{Feature: "delete", Bug: "delete"},
	}
	m2 := newConfigModel(cfg, "")
	result2 := m2.buildConfig()
	if result2.OnComplete.Feature != "delete" {
		t.Errorf("Feature = %q, want delete", result2.OnComplete.Feature)
	}
	if result2.OnComplete.Bug != "delete" {
		t.Errorf("Bug = %q, want delete", result2.OnComplete.Bug)
	}
}

func TestSaveConfig_OnComplete(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{
		Agent:      "claude",
		OnComplete: config.OnCompleteConfig{Feature: "delete", Bug: "delete"},
	}
	if err := saveConfig(dir, cfg); err != nil {
		t.Fatalf("saveConfig() error: %v", err)
	}

	path := filepath.Join(dir, ".maggus", "config.yml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read saved config: %v", err)
	}

	var loaded config.Config
	if err := yaml.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if loaded.OnComplete.FeatureAction() != "delete" {
		t.Errorf("FeatureAction = %q, want delete", loaded.OnComplete.FeatureAction())
	}
	if loaded.OnComplete.BugAction() != "delete" {
		t.Errorf("BugAction = %q, want delete", loaded.OnComplete.BugAction())
	}
}

func TestNewConfigModel_ModelHaiku(t *testing.T) {
	cfg := config.Config{Agent: "claude", Model: "haiku"}
	m := newConfigModel(cfg, "")
	opts := optionRows(m)

	if opts[1].current != 3 {
		t.Errorf("model current = %d, want 3 (haiku)", opts[1].current)
	}
}

func TestNewConfigModel_UnknownModel(t *testing.T) {
	cfg := config.Config{Agent: "claude", Model: "unknown-model"}
	m := newConfigModel(cfg, "")
	opts := optionRows(m)

	if opts[1].current != 0 {
		t.Errorf("model current = %d, want 0 (fallback for unknown)", opts[1].current)
	}
}
