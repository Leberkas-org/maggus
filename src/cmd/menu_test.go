package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/leberkas-org/maggus/internal/globalconfig"
	"github.com/leberkas-org/maggus/internal/updater"
)

func TestActiveMenuItems_WithClaude(t *testing.T) {
	origCaps := caps
	t.Cleanup(func() { caps = origCaps })
	caps.HasClaude = true

	// Use a temp dir without .maggus so isInitialized() returns false.
	dir := t.TempDir()
	t.Chdir(dir)

	items := activeMenuItems()

	// Should include requiresClaude items (vision, architecture, plan).
	found := map[string]bool{}
	for _, item := range items {
		found[item.name] = true
	}
	for _, name := range []string{"vision", "architecture", "plan"} {
		if !found[name] {
			t.Errorf("expected %q to be present when HasClaude=true", name)
		}
	}
	// Should include init when not initialized.
	if !found["init"] {
		t.Error("expected init to be present when not initialized")
	}
}

func TestActiveMenuItems_WithoutClaude(t *testing.T) {
	origCaps := caps
	t.Cleanup(func() { caps = origCaps })
	caps.HasClaude = false

	dir := t.TempDir()
	t.Chdir(dir)

	items := activeMenuItems()

	for _, item := range items {
		if item.requiresClaude {
			t.Errorf("item %q should be filtered out when HasClaude=false", item.name)
		}
	}
}

func TestActiveMenuItems_HidesInitWhenInitialized(t *testing.T) {
	origCaps := caps
	t.Cleanup(func() { caps = origCaps })
	caps.HasClaude = false

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".maggus"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	items := activeMenuItems()

	for _, item := range items {
		if item.name == "init" {
			t.Error("init should be hidden when .maggus/ exists")
		}
	}
}

func TestActiveMenuItems_FirstItemNeverHasSeparator(t *testing.T) {
	origCaps := caps
	t.Cleanup(func() { caps = origCaps })
	caps.HasClaude = false

	dir := t.TempDir()
	t.Chdir(dir)

	items := activeMenuItems()

	if len(items) == 0 {
		t.Fatal("expected at least one menu item")
	}
	if items[0].separator {
		t.Error("first visible item should not have separator=true")
	}
}

func TestActiveMenuItems_AlwaysIncludesExit(t *testing.T) {
	origCaps := caps
	t.Cleanup(func() { caps = origCaps })
	caps.HasClaude = false

	dir := t.TempDir()
	t.Chdir(dir)

	items := activeMenuItems()

	hasExit := false
	for _, item := range items {
		if item.isExit {
			hasExit = true
		}
	}
	if !hasExit {
		t.Error("menu should always include an exit item")
	}
}

func TestBuildSubMenus(t *testing.T) {
	subs := buildSubMenus()

	// "work" should have 2 options.
	workDef, ok := subs["work"]
	if !ok {
		t.Fatal("expected sub-menu definition for 'work'")
	}
	if len(workDef.options) != 2 {
		t.Errorf("work sub-menu: got %d options, want 2", len(workDef.options))
	}

	// "worktree" should have 1 option.
	wtDef, ok := subs["worktree"]
	if !ok {
		t.Fatal("expected sub-menu definition for 'worktree'")
	}
	if len(wtDef.options) != 1 {
		t.Errorf("worktree sub-menu: got %d options, want 1", len(wtDef.options))
	}
}

func TestBuildArgs_Work(t *testing.T) {
	tests := []struct {
		name     string
		opts     []subMenuOption
		wantArgs []string
	}{
		{
			name: "default (3 tasks, worktree off)",
			opts: []subMenuOption{
				{label: "Tasks", values: []string{"1", "3", "5", "10", "all"}, current: 1},
				{label: "Worktree", values: []string{"off", "on"}, current: 0},
			},
			wantArgs: []string{"--count", "3"},
		},
		{
			name: "all tasks",
			opts: []subMenuOption{
				{label: "Tasks", values: []string{"1", "3", "5", "10", "all"}, current: 4},
				{label: "Worktree", values: []string{"off", "on"}, current: 0},
			},
			wantArgs: []string{"--count", "999"},
		},
		{
			name: "1 task with worktree on",
			opts: []subMenuOption{
				{label: "Tasks", values: []string{"1", "3", "5", "10", "all"}, current: 0},
				{label: "Worktree", values: []string{"off", "on"}, current: 1},
			},
			wantArgs: []string{"--count", "1", "--worktree"},
		},
		{
			name: "10 tasks",
			opts: []subMenuOption{
				{label: "Tasks", values: []string{"1", "3", "5", "10", "all"}, current: 3},
				{label: "Worktree", values: []string{"off", "on"}, current: 0},
			},
			wantArgs: []string{"--count", "10"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildArgs("work", tt.opts)
			if len(got) != len(tt.wantArgs) {
				t.Fatalf("got %v, want %v", got, tt.wantArgs)
			}
			for i := range got {
				if got[i] != tt.wantArgs[i] {
					t.Errorf("arg[%d]: got %q, want %q", i, got[i], tt.wantArgs[i])
				}
			}
		})
	}
}

func TestBuildArgs_Worktree(t *testing.T) {
	tests := []struct {
		name     string
		opts     []subMenuOption
		wantArgs []string
	}{
		{
			name: "list action",
			opts: []subMenuOption{
				{label: "Action", values: []string{"list", "clean"}, current: 0},
			},
			wantArgs: []string{"list"},
		},
		{
			name: "clean action",
			opts: []subMenuOption{
				{label: "Action", values: []string{"list", "clean"}, current: 1},
			},
			wantArgs: []string{"clean"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildArgs("worktree", tt.opts)
			if len(got) != len(tt.wantArgs) {
				t.Fatalf("got %v, want %v", got, tt.wantArgs)
			}
			for i := range got {
				if got[i] != tt.wantArgs[i] {
					t.Errorf("arg[%d]: got %q, want %q", i, got[i], tt.wantArgs[i])
				}
			}
		})
	}
}

func TestBuildArgs_UnknownCommand(t *testing.T) {
	got := buildArgs("unknown", nil)
	if got != nil {
		t.Errorf("expected nil for unknown command, got %v", got)
	}
}

func TestCenterLine(t *testing.T) {
	tests := []struct {
		name  string
		line  string
		width int
		want  string
	}{
		{
			name:  "centers short text",
			line:  "hi",
			width: 10,
			want:  "    hi",
		},
		{
			name:  "no padding when text equals width",
			line:  "1234567890",
			width: 10,
			want:  "1234567890",
		},
		{
			name:  "no padding when text exceeds width",
			line:  "this is longer than width",
			width: 5,
			want:  "this is longer than width",
		},
		{
			name:  "empty string",
			line:  "",
			width: 10,
			want:  "     ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := centerLine(tt.line, tt.width)
			if got != tt.want {
				t.Errorf("centerLine(%q, %d) = %q, want %q", tt.line, tt.width, got, tt.want)
			}
		})
	}
}

func TestCenterBlock(t *testing.T) {
	block := "ab\ncd\nef"
	result := centerBlock(block, 10)
	lines := strings.Split(result, "\n")

	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}
	// Each 2-char line should be padded with 4 spaces.
	for i, line := range lines {
		if !strings.HasPrefix(line, "    ") {
			t.Errorf("line %d: expected 4-space prefix, got %q", i, line)
		}
	}
}

func TestCenterBlock_TrailingNewlines(t *testing.T) {
	block := "abc\ndef\n\n"
	result := centerBlock(block, 10)
	lines := strings.Split(result, "\n")

	// TrimRight removes trailing newlines, so "abc\ndef\n\n" → ["abc","def"]
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines (trailing newlines trimmed), got %d: %v", len(lines), lines)
	}
}

func TestIsInitialized(t *testing.T) {
	t.Run("returns true when .maggus exists", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(dir, ".maggus"), 0o755); err != nil {
			t.Fatal(err)
		}
		t.Chdir(dir)

		if !isInitialized() {
			t.Error("expected isInitialized() to return true")
		}
	})

	t.Run("returns false when .maggus does not exist", func(t *testing.T) {
		dir := t.TempDir()
		t.Chdir(dir)

		if isInitialized() {
			t.Error("expected isInitialized() to return false")
		}
	})

	t.Run("returns false when .maggus is a file not a directory", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, ".maggus"), []byte("not a dir"), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Chdir(dir)

		if isInitialized() {
			t.Error("expected isInitialized() to return false when .maggus is a file")
		}
	})
}

// saveStartupUpdateTestState saves and returns a cleanup function for all injectable vars.
func saveStartupUpdateTestState(t *testing.T) {
	t.Helper()
	origVersion := Version
	origCheck := checkLatestVersion
	origApply := applyUpdate
	origLoadSettings := loadSettings
	origLoadState := loadUpdateState
	origSaveState := saveUpdateState
	origTimeNow := timeNow
	t.Cleanup(func() {
		Version = origVersion
		checkLatestVersion = origCheck
		applyUpdate = origApply
		loadSettings = origLoadSettings
		loadUpdateState = origLoadState
		saveUpdateState = origSaveState
		timeNow = origTimeNow
	})
}

func TestStartupUpdateCheck_DevVersion(t *testing.T) {
	saveStartupUpdateTestState(t)
	Version = "dev"

	result := startupUpdateCheck()
	if result != "" {
		t.Errorf("expected empty banner for dev version, got: %q", result)
	}
}

func TestStartupUpdateCheck_OffMode(t *testing.T) {
	saveStartupUpdateTestState(t)
	Version = "1.0.0"
	loadSettings = func() (globalconfig.Settings, error) {
		return globalconfig.Settings{AutoUpdate: globalconfig.AutoUpdateOff}, nil
	}

	result := startupUpdateCheck()
	if result != "" {
		t.Errorf("expected empty banner for off mode, got: %q", result)
	}
}

func TestStartupUpdateCheck_CooldownNotPassed(t *testing.T) {
	saveStartupUpdateTestState(t)
	Version = "1.0.0"
	now := time.Date(2026, 3, 19, 12, 0, 0, 0, time.UTC)
	timeNow = func() time.Time { return now }
	loadSettings = func() (globalconfig.Settings, error) {
		return globalconfig.Settings{AutoUpdate: globalconfig.AutoUpdateNotify}, nil
	}
	loadUpdateState = func() (globalconfig.UpdateState, error) {
		// Last check was 1 hour ago — within 24h cooldown.
		return globalconfig.UpdateState{LastUpdateCheck: now.Add(-1 * time.Hour)}, nil
	}

	result := startupUpdateCheck()
	if result != "" {
		t.Errorf("expected empty banner when cooldown not passed, got: %q", result)
	}
}

func TestStartupUpdateCheck_NotifyMode_UpdateAvailable(t *testing.T) {
	saveStartupUpdateTestState(t)
	Version = "1.0.0"
	now := time.Date(2026, 3, 19, 12, 0, 0, 0, time.UTC)
	timeNow = func() time.Time { return now }
	loadSettings = func() (globalconfig.Settings, error) {
		return globalconfig.Settings{AutoUpdate: globalconfig.AutoUpdateNotify}, nil
	}
	loadUpdateState = func() (globalconfig.UpdateState, error) {
		return globalconfig.UpdateState{}, nil // zero time = never checked
	}
	var savedState *globalconfig.UpdateState
	saveUpdateState = func(state globalconfig.UpdateState) error {
		savedState = &state
		return nil
	}
	checkLatestVersion = func(v string) updater.UpdateInfo {
		return updater.UpdateInfo{TagName: "v2.0.0", IsNewer: true, DownloadURL: "https://example.com/maggus.zip"}
	}

	result := startupUpdateCheck()

	if !strings.Contains(result, "v1.0.0") || !strings.Contains(result, "v2.0.0") {
		t.Errorf("expected banner with version info, got: %q", result)
	}
	if !strings.Contains(result, "maggus update") {
		t.Errorf("expected banner to suggest maggus update, got: %q", result)
	}
	if savedState == nil {
		t.Fatal("expected update state to be saved")
	}
	if !savedState.LastUpdateCheck.Equal(now) {
		t.Errorf("expected lastUpdateCheck=%v, got %v", now, savedState.LastUpdateCheck)
	}
}

func TestStartupUpdateCheck_NotifyMode_NoUpdate(t *testing.T) {
	saveStartupUpdateTestState(t)
	Version = "1.0.0"
	now := time.Date(2026, 3, 19, 12, 0, 0, 0, time.UTC)
	timeNow = func() time.Time { return now }
	loadSettings = func() (globalconfig.Settings, error) {
		return globalconfig.Settings{AutoUpdate: globalconfig.AutoUpdateNotify}, nil
	}
	loadUpdateState = func() (globalconfig.UpdateState, error) {
		return globalconfig.UpdateState{}, nil
	}
	saveUpdateState = func(state globalconfig.UpdateState) error { return nil }
	checkLatestVersion = func(v string) updater.UpdateInfo {
		return updater.UpdateInfo{TagName: "v1.0.0", IsNewer: false}
	}

	result := startupUpdateCheck()
	if result != "" {
		t.Errorf("expected empty banner when no update, got: %q", result)
	}
}

func TestStartupUpdateCheck_AutoMode_UpdateApplied(t *testing.T) {
	saveStartupUpdateTestState(t)
	Version = "1.0.0"
	now := time.Date(2026, 3, 19, 12, 0, 0, 0, time.UTC)
	timeNow = func() time.Time { return now }
	loadSettings = func() (globalconfig.Settings, error) {
		return globalconfig.Settings{AutoUpdate: globalconfig.AutoUpdateAuto}, nil
	}
	loadUpdateState = func() (globalconfig.UpdateState, error) {
		return globalconfig.UpdateState{}, nil
	}
	saveUpdateState = func(state globalconfig.UpdateState) error { return nil }
	checkLatestVersion = func(v string) updater.UpdateInfo {
		return updater.UpdateInfo{TagName: "v2.0.0", IsNewer: true, DownloadURL: "https://example.com/maggus.zip"}
	}
	applyCalled := false
	applyUpdate = func(url string) error {
		applyCalled = true
		return nil
	}

	result := startupUpdateCheck()

	if !applyCalled {
		t.Error("expected applyUpdate to be called in auto mode")
	}
	if !strings.Contains(result, "v2.0.0") || !strings.Contains(result, "restart") {
		t.Errorf("expected auto-update success banner, got: %q", result)
	}
}

func TestStartupUpdateCheck_AutoMode_ApplyError(t *testing.T) {
	saveStartupUpdateTestState(t)
	Version = "1.0.0"
	now := time.Date(2026, 3, 19, 12, 0, 0, 0, time.UTC)
	timeNow = func() time.Time { return now }
	loadSettings = func() (globalconfig.Settings, error) {
		return globalconfig.Settings{AutoUpdate: globalconfig.AutoUpdateAuto}, nil
	}
	loadUpdateState = func() (globalconfig.UpdateState, error) {
		return globalconfig.UpdateState{}, nil
	}
	saveUpdateState = func(state globalconfig.UpdateState) error { return nil }
	checkLatestVersion = func(v string) updater.UpdateInfo {
		return updater.UpdateInfo{TagName: "v2.0.0", IsNewer: true, DownloadURL: "https://example.com/maggus.zip"}
	}
	applyUpdate = func(url string) error {
		return fmt.Errorf("permission denied")
	}

	result := startupUpdateCheck()
	if result != "" {
		t.Errorf("expected empty banner on apply error, got: %q", result)
	}
}

func TestStartupUpdateCheck_AutoMode_NoDownloadURL(t *testing.T) {
	saveStartupUpdateTestState(t)
	Version = "1.0.0"
	now := time.Date(2026, 3, 19, 12, 0, 0, 0, time.UTC)
	timeNow = func() time.Time { return now }
	loadSettings = func() (globalconfig.Settings, error) {
		return globalconfig.Settings{AutoUpdate: globalconfig.AutoUpdateAuto}, nil
	}
	loadUpdateState = func() (globalconfig.UpdateState, error) {
		return globalconfig.UpdateState{}, nil
	}
	saveUpdateState = func(state globalconfig.UpdateState) error { return nil }
	checkLatestVersion = func(v string) updater.UpdateInfo {
		return updater.UpdateInfo{TagName: "v2.0.0", IsNewer: true, DownloadURL: ""}
	}

	result := startupUpdateCheck()
	if result != "" {
		t.Errorf("expected empty banner when no download URL in auto mode, got: %q", result)
	}
}
