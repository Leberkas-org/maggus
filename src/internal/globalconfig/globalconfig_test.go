package globalconfig

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
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

func TestRepository_IsAutoStartEnabled(t *testing.T) {
	// Default (zero value) → auto-start enabled
	r := Repository{Path: "/repo"}
	if !r.IsAutoStartEnabled() {
		t.Fatal("expected auto-start enabled by default")
	}

	// Explicitly disabled
	r.AutoStartDisabled = true
	if r.IsAutoStartEnabled() {
		t.Fatal("expected auto-start disabled when AutoStartDisabled is true")
	}
}

func TestAutoStartDisabled_OmittedFromYAML(t *testing.T) {
	// When AutoStartDisabled is false (default), it should be omitted from YAML
	path := filepath.Join(t.TempDir(), "repos.yml")
	cfg := GlobalConfig{
		Repositories: []Repository{
			{Path: "/repo/default"},
			{Path: "/repo/disabled", AutoStartDisabled: true},
		},
	}

	if err := SaveTo(cfg, path); err != nil {
		t.Fatalf("save: %v", err)
	}

	// Reload and verify
	loaded, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(loaded.Repositories) != 2 {
		t.Fatalf("expected 2 repos, got %d", len(loaded.Repositories))
	}
	if loaded.Repositories[0].AutoStartDisabled {
		t.Fatal("expected default repo to have auto-start enabled")
	}
	if !loaded.Repositories[1].AutoStartDisabled {
		t.Fatal("expected disabled repo to have auto-start disabled")
	}

	// Verify the YAML content: default repo should not contain auto_start_disabled
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	content := string(data)
	// The first repo entry should not have auto_start_disabled at all
	// The second repo entry should have auto_start_disabled: true
	var raw []map[string]interface{}
	if err := yaml.Unmarshal(data, &struct {
		Repos *[]map[string]interface{} `yaml:"repositories"`
	}{Repos: &raw}); err != nil {
		t.Fatalf("parse raw: %v", err)
	}
	if _, ok := raw[0]["auto_start_disabled"]; ok {
		t.Fatalf("default repo should omit auto_start_disabled, got yaml: %s", content)
	}
}

func TestLoadFrom_WithoutAutoStartField(t *testing.T) {
	// Simulate an existing repositories.yml without the auto_start_disabled field
	path := filepath.Join(t.TempDir(), "repos.yml")
	legacy := "repositories:\n  - path: /repo/old\n"
	os.WriteFile(path, []byte(legacy), 0o644)

	loaded, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(loaded.Repositories) != 1 {
		t.Fatalf("expected 1 repo, got %d", len(loaded.Repositories))
	}
	if !loaded.Repositories[0].IsAutoStartEnabled() {
		t.Fatal("existing repo without field should have auto-start enabled")
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

// --- Settings tests ---

func TestLoadSettingsFrom_NonExistent(t *testing.T) {
	s, err := LoadSettingsFrom(filepath.Join(t.TempDir(), "nope.yml"))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if s.AutoUpdate != AutoUpdateNotify {
		t.Fatalf("expected default notify, got %q", s.AutoUpdate)
	}
}

func TestLoadSettingsFrom_AllValues(t *testing.T) {
	for _, mode := range []AutoUpdateMode{AutoUpdateOff, AutoUpdateNotify, AutoUpdateAuto} {
		path := filepath.Join(t.TempDir(), string(mode)+".yml")
		os.WriteFile(path, []byte("auto_update: "+string(mode)+"\n"), 0o644)

		s, err := LoadSettingsFrom(path)
		if err != nil {
			t.Fatalf("mode %q: %v", mode, err)
		}
		if s.AutoUpdate != mode {
			t.Fatalf("expected %q, got %q", mode, s.AutoUpdate)
		}
	}
}

func TestLoadSettingsFrom_EmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.yml")
	os.WriteFile(path, []byte(""), 0o644)

	s, err := LoadSettingsFrom(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.AutoUpdate != AutoUpdateNotify {
		t.Fatalf("expected default notify for empty file, got %q", s.AutoUpdate)
	}
}

func TestLoadSettingsFrom_InvalidValue(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.yml")
	os.WriteFile(path, []byte("auto_update: bogus\n"), 0o644)

	_, err := LoadSettingsFrom(path)
	if err == nil {
		t.Fatal("expected error for invalid auto_update value")
	}
}

func TestLoadSettingsFrom_InvalidYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.yml")
	os.WriteFile(path, []byte(":::not yaml\n\t\tbad"), 0o644)

	_, err := LoadSettingsFrom(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestSaveAndLoadSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yml")

	s := Settings{AutoUpdate: AutoUpdateAuto}
	if err := SaveSettingsTo(s, path); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := LoadSettingsFrom(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.AutoUpdate != AutoUpdateAuto {
		t.Fatalf("expected auto, got %q", loaded.AutoUpdate)
	}
}

// --- UpdateState tests ---

func TestLoadUpdateStateFrom_NonExistent(t *testing.T) {
	state, err := LoadUpdateStateFrom(filepath.Join(t.TempDir(), "nope.json"))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !state.LastUpdateCheck.IsZero() {
		t.Fatalf("expected zero time, got %v", state.LastUpdateCheck)
	}
}

func TestSaveAndLoadUpdateState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	now := time.Date(2026, 3, 19, 12, 0, 0, 0, time.UTC)

	if err := SaveUpdateStateTo(UpdateState{LastUpdateCheck: now}, path); err != nil {
		t.Fatalf("save: %v", err)
	}

	state, err := LoadUpdateStateFrom(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !state.LastUpdateCheck.Equal(now) {
		t.Fatalf("expected %v, got %v", now, state.LastUpdateCheck)
	}
}

func TestLoadUpdateStateFrom_InvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	os.WriteFile(path, []byte("{invalid"), 0o644)

	_, err := LoadUpdateStateFrom(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

// --- Cooldown tests ---

func TestShouldCheckUpdate_ZeroTime(t *testing.T) {
	if !ShouldCheckUpdate(UpdateState{}, time.Now()) {
		t.Fatal("should check when LastUpdateCheck is zero")
	}
}

func TestShouldCheckUpdate_RecentCheck(t *testing.T) {
	now := time.Now()
	state := UpdateState{LastUpdateCheck: now.Add(-1 * time.Hour)}
	if ShouldCheckUpdate(state, now) {
		t.Fatal("should NOT check when last check was 1 hour ago")
	}
}

func TestShouldCheckUpdate_OldCheck(t *testing.T) {
	now := time.Now()
	state := UpdateState{LastUpdateCheck: now.Add(-25 * time.Hour)}
	if !ShouldCheckUpdate(state, now) {
		t.Fatal("should check when last check was 25 hours ago")
	}
}

func TestShouldCheckUpdate_Exactly24h(t *testing.T) {
	now := time.Now()
	state := UpdateState{LastUpdateCheck: now.Add(-24 * time.Hour)}
	if !ShouldCheckUpdate(state, now) {
		t.Fatal("should check when exactly 24h have passed")
	}
}

// --- AutoUpdateMode validation ---

func TestAutoUpdateMode_IsValid(t *testing.T) {
	if !AutoUpdateOff.IsValid() {
		t.Fatal("off should be valid")
	}
	if !AutoUpdateNotify.IsValid() {
		t.Fatal("notify should be valid")
	}
	if !AutoUpdateAuto.IsValid() {
		t.Fatal("auto should be valid")
	}
	if AutoUpdateMode("bogus").IsValid() {
		t.Fatal("bogus should not be valid")
	}
}

// --- Metrics tests ---

func TestMetrics_isZero(t *testing.T) {
	if !(Metrics{}).isZero() {
		t.Fatal("empty Metrics should be zero")
	}
	if (Metrics{WorkRuns: 1}).isZero() {
		t.Fatal("non-empty Metrics should not be zero")
	}
}

func TestIncrementMetricsIn_ZeroDeltaNoOp(t *testing.T) {
	dir := t.TempDir()
	metricsPath := filepath.Join(dir, "metrics.yml")

	// Write initial metrics.
	if err := SaveMetricsTo(Metrics{WorkRuns: 1}, metricsPath); err != nil {
		t.Fatalf("save: %v", err)
	}
	info, _ := os.Stat(metricsPath)
	modBefore := info.ModTime()

	// Zero delta should be a no-op — file should not be rewritten.
	if err := IncrementMetricsIn(dir, Metrics{}); err != nil {
		t.Fatalf("increment: %v", err)
	}

	info2, _ := os.Stat(metricsPath)
	if !info2.ModTime().Equal(modBefore) {
		t.Fatal("zero delta should not rewrite the file")
	}
}

func TestIncrementMetricsIn_SingleIncrement(t *testing.T) {
	dir := t.TempDir()
	metricsPath := filepath.Join(dir, "metrics.yml")

	if err := IncrementMetricsIn(dir, Metrics{WorkRuns: 3, TasksCompleted: 1}); err != nil {
		t.Fatalf("increment: %v", err)
	}

	m, err := LoadMetricsFrom(metricsPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if m.WorkRuns != 3 {
		t.Fatalf("expected WorkRuns=3, got %d", m.WorkRuns)
	}
	if m.TasksCompleted != 1 {
		t.Fatalf("expected TasksCompleted=1, got %d", m.TasksCompleted)
	}
	// Untouched fields should remain zero.
	if m.GitCommits != 0 {
		t.Fatalf("expected GitCommits=0, got %d", m.GitCommits)
	}
}

func TestIncrementMetricsIn_AccumulatesMultipleCalls(t *testing.T) {
	dir := t.TempDir()
	metricsPath := filepath.Join(dir, "metrics.yml")

	for range 5 {
		if err := IncrementMetricsIn(dir, Metrics{StartupCount: 1}); err != nil {
			t.Fatalf("increment: %v", err)
		}
	}

	m, err := LoadMetricsFrom(metricsPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if m.StartupCount != 5 {
		t.Fatalf("expected StartupCount=5, got %d", m.StartupCount)
	}
}

func TestIncrementMetricsIn_DoesNotTouchConfigYml(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yml")

	// Write initial config with auto_update set.
	if err := SaveSettingsTo(Settings{AutoUpdate: AutoUpdateAuto}, configPath); err != nil {
		t.Fatalf("save: %v", err)
	}
	infoBefore, _ := os.Stat(configPath)
	modBefore := infoBefore.ModTime()

	if err := IncrementMetricsIn(dir, Metrics{WorkRuns: 1}); err != nil {
		t.Fatalf("increment: %v", err)
	}

	// config.yml should be untouched.
	infoAfter, _ := os.Stat(configPath)
	if !infoAfter.ModTime().Equal(modBefore) {
		t.Fatal("IncrementMetricsIn should not modify config.yml")
	}

	// Settings should not contain metrics.
	s, err := LoadSettingsFrom(configPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if s.AutoUpdate != AutoUpdateAuto {
		t.Fatalf("expected auto_update=auto, got %q", s.AutoUpdate)
	}

	// Metrics should be in metrics.yml.
	m, err := LoadMetricsFrom(filepath.Join(dir, "metrics.yml"))
	if err != nil {
		t.Fatalf("load metrics: %v", err)
	}
	if m.WorkRuns != 1 {
		t.Fatalf("expected WorkRuns=1, got %d", m.WorkRuns)
	}
}

func TestIncrementMetricsIn_ConcurrentIncrements(t *testing.T) {
	dir := t.TempDir()
	metricsPath := filepath.Join(dir, "metrics.yml")

	const goroutines = 50
	var wg sync.WaitGroup
	errs := make(chan error, goroutines)

	wg.Add(goroutines)
	for range goroutines {
		go func() {
			defer wg.Done()
			if err := IncrementMetricsIn(dir, Metrics{WorkRuns: 1}); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		t.Fatalf("concurrent increment error: %v", err)
	}

	m, err := LoadMetricsFrom(metricsPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if m.WorkRuns != goroutines {
		t.Fatalf("expected WorkRuns=%d, got %d", goroutines, m.WorkRuns)
	}
}

func TestIncrementMetricsIn_MigratesFromConfigYml(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yml")

	// Write a legacy config.yml with embedded metrics.
	legacy := "auto_update: auto\nmetrics:\n  startup_count: 10\n  work_runs: 5\n"
	if err := os.WriteFile(configPath, []byte(legacy), 0o644); err != nil {
		t.Fatalf("write legacy config: %v", err)
	}

	// Increment should trigger migration.
	if err := IncrementMetricsIn(dir, Metrics{WorkRuns: 1}); err != nil {
		t.Fatalf("increment: %v", err)
	}

	// metrics.yml should contain migrated + incremented values.
	m, err := LoadMetricsFrom(filepath.Join(dir, "metrics.yml"))
	if err != nil {
		t.Fatalf("load metrics: %v", err)
	}
	if m.StartupCount != 10 {
		t.Fatalf("expected migrated StartupCount=10, got %d", m.StartupCount)
	}
	if m.WorkRuns != 6 {
		t.Fatalf("expected migrated+incremented WorkRuns=6, got %d", m.WorkRuns)
	}

	// config.yml should no longer contain metrics.
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if contains := string(data); len(data) > 0 {
		var check settingsWithMetrics
		if err := yaml.Unmarshal(data, &check); err != nil {
			t.Fatalf("parse config: %v", err)
		}
		if !check.Metrics.isZero() {
			t.Fatalf("config.yml should not contain metrics after migration, got %+v", check.Metrics)
		}
		_ = contains
	}
}

func TestLoadMetricsFrom_NonExistent(t *testing.T) {
	m, err := LoadMetricsFrom(filepath.Join(t.TempDir(), "nope.yml"))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !m.isZero() {
		t.Fatalf("expected zero metrics, got %+v", m)
	}
}

func TestSaveAndLoadMetrics(t *testing.T) {
	path := filepath.Join(t.TempDir(), "metrics.yml")
	m := Metrics{WorkRuns: 3, TokensUsed: 1000}

	if err := SaveMetricsTo(m, path); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := LoadMetricsFrom(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.WorkRuns != 3 {
		t.Fatalf("expected WorkRuns=3, got %d", loaded.WorkRuns)
	}
	if loaded.TokensUsed != 1000 {
		t.Fatalf("expected TokensUsed=1000, got %d", loaded.TokensUsed)
	}
}
