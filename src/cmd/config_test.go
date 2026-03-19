package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/leberkas-org/maggus/internal/config"
	"gopkg.in/yaml.v3"
)

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
	m := newConfigModel(cfg)

	if len(m.options) != 7 {
		t.Fatalf("expected 7 options, got %d", len(m.options))
	}

	// Agent defaults to claude (index 0)
	if m.options[0].label != "Agent" {
		t.Errorf("options[0].label = %q, want Agent", m.options[0].label)
	}
	if m.options[0].current != 0 {
		t.Errorf("agent current = %d, want 0 (claude)", m.options[0].current)
	}

	// Model defaults to (default) (index 0)
	if m.options[1].current != 0 {
		t.Errorf("model current = %d, want 0 ((default))", m.options[1].current)
	}

	// Worktree defaults to off (index 1)
	if m.options[2].current != 1 {
		t.Errorf("worktree current = %d, want 1 (off)", m.options[2].current)
	}

	// Sound defaults to off (index 1)
	if m.options[3].current != 1 {
		t.Errorf("sound current = %d, want 1 (off)", m.options[3].current)
	}

	// Notification sub-options default to on (index 0) when nil
	for i := 4; i <= 6; i++ {
		if m.options[i].current != 0 {
			t.Errorf("options[%d].current = %d, want 0 (on)", i, m.options[i].current)
		}
	}

	if m.cursor != 0 {
		t.Errorf("cursor = %d, want 0", m.cursor)
	}
	if m.saved {
		t.Error("saved should be false")
	}
	if m.openEditor {
		t.Error("openEditor should be false")
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
	m := newConfigModel(cfg)

	if m.options[0].current != 1 {
		t.Errorf("agent current = %d, want 1 (opencode)", m.options[0].current)
	}
	if m.options[1].current != 2 {
		t.Errorf("model current = %d, want 2 (opus)", m.options[1].current)
	}
	if m.options[2].current != 0 {
		t.Errorf("worktree current = %d, want 0 (on)", m.options[2].current)
	}
	if m.options[3].current != 0 {
		t.Errorf("sound current = %d, want 0 (on)", m.options[3].current)
	}
	// All notification sub-options set to false → off (index 1)
	for i := 4; i <= 6; i++ {
		if m.options[i].current != 1 {
			t.Errorf("options[%d].current = %d, want 1 (off)", i, m.options[i].current)
		}
	}
}

func TestBuildConfig_Defaults(t *testing.T) {
	m := newConfigModel(config.Config{Agent: "claude"})
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
	m := newConfigModel(cfg)
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

func TestBuildConfig_RoundTrip(t *testing.T) {
	// A config round-tripped through newConfigModel→buildConfig should preserve values.
	original := config.Config{
		Agent:    "claude",
		Model:    "sonnet",
		Worktree: true,
		Notifications: config.NotificationsConfig{
			Sound: true,
		},
	}
	m := newConfigModel(original)
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
	// buildConfig should not set Include — that's preserved separately in runConfig.
	m := newConfigModel(config.Config{Agent: "claude", Include: []string{"foo.md"}})
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

	// .maggus/ doesn't exist yet
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

	// Save once
	if err := saveConfig(dir, config.Config{Agent: "claude", Model: "sonnet"}); err != nil {
		t.Fatalf("first saveConfig() error: %v", err)
	}

	// Overwrite with different values
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

func TestConfigOption_Fields(t *testing.T) {
	opt := configOption{
		label:   "Agent",
		key:     "agent",
		values:  []string{"claude", "opencode"},
		current: 0,
	}

	if opt.label != "Agent" {
		t.Errorf("label = %q, want Agent", opt.label)
	}
	if opt.key != "agent" {
		t.Errorf("key = %q, want agent", opt.key)
	}
	if len(opt.values) != 2 {
		t.Errorf("values len = %d, want 2", len(opt.values))
	}
	if opt.current != 0 {
		t.Errorf("current = %d, want 0", opt.current)
	}
}

func TestNewConfigModel_OptionKeys(t *testing.T) {
	m := newConfigModel(config.Config{Agent: "claude"})

	expectedKeys := []string{
		"agent",
		"model",
		"worktree",
		"notifications.sound",
		"notifications.on_task_complete",
		"notifications.on_run_complete",
		"notifications.on_error",
	}

	for i, want := range expectedKeys {
		if m.options[i].key != want {
			t.Errorf("options[%d].key = %q, want %q", i, m.options[i].key, want)
		}
	}
}

func TestNewConfigModel_ModelHaiku(t *testing.T) {
	cfg := config.Config{Agent: "claude", Model: "haiku"}
	m := newConfigModel(cfg)

	if m.options[1].current != 3 {
		t.Errorf("model current = %d, want 3 (haiku)", m.options[1].current)
	}
}

func TestNewConfigModel_UnknownModel(t *testing.T) {
	// Unknown model value falls back to index 0 via indexOf
	cfg := config.Config{Agent: "claude", Model: "unknown-model"}
	m := newConfigModel(cfg)

	if m.options[1].current != 0 {
		t.Errorf("model current = %d, want 0 (fallback for unknown)", m.options[1].current)
	}
}
