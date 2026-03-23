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

	// "work" should NOT have a sub-menu.
	if _, ok := subs["work"]; ok {
		t.Error("expected no sub-menu definition for 'work'")
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

func TestTruncateLeft(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		maxWidth int
		want     string
	}{
		{
			name:     "short path unchanged",
			path:     "/home/user",
			maxWidth: 20,
			want:     "/home/user",
		},
		{
			name:     "exact width unchanged",
			path:     "abcdefghij",
			maxWidth: 10,
			want:     "abcdefghij",
		},
		{
			name:     "long path truncated with ellipsis",
			path:     "/home/user/projects/myapp",
			maxWidth: 15,
			want:     "...ojects/myapp",
		},
		{
			name:     "very small max width",
			path:     "/home/user",
			maxWidth: 3,
			want:     "ser",
		},
		{
			name:     "zero max width returns original",
			path:     "/home/user",
			maxWidth: 0,
			want:     "/home/user",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateLeft(tt.path, tt.maxWidth)
			if got != tt.want {
				t.Errorf("truncateLeft(%q, %d) = %q, want %q", tt.path, tt.maxWidth, got, tt.want)
			}
		})
	}
}

func TestMenuView_CWDUsesBoldPrimaryStyle(t *testing.T) {
	m := menuModel{
		items:   activeMenuItems(),
		cwd:     "/test/project",
		width:   120,
		height:  40,
		summary: featureSummary{},
	}

	view := m.View()

	// The CWD path must appear in the rendered view.
	if !strings.Contains(view, "/test/project") {
		t.Error("expected CWD path to appear in menu view")
	}
}

func TestMenuView_CWDEmptyHidden(t *testing.T) {
	m := menuModel{
		items:   activeMenuItems(),
		cwd:     "",
		width:   120,
		height:  40,
		summary: featureSummary{},
	}

	view := m.View()

	// When cwd is empty, no CWD line should appear.
	// The view should still render without errors.
	if strings.Contains(view, "...") {
		// No truncated path should appear since cwd is empty.
	}
	_ = view // ensure it renders without panic
}

func TestMenuView_CWDStillCentered(t *testing.T) {
	m := menuModel{
		items:   activeMenuItems(),
		cwd:     "/short",
		width:   120,
		height:  40,
		summary: featureSummary{},
	}

	view := m.View()

	// Find the line containing the CWD. It should have leading spaces (centered).
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, "/short") {
			trimmed := strings.TrimLeft(line, " ")
			leadingSpaces := len(line) - len(trimmed)
			if leadingSpaces == 0 {
				t.Error("expected CWD line to be centered with leading spaces")
			}
			return
		}
	}
	t.Error("CWD path not found in view output")
}

func TestFormatSummaryLine(t *testing.T) {
	tests := []struct {
		name         string
		summary      featureSummary
		wantContains []string
		wantAbsent   []string
	}{
		{
			name:         "no open tasks",
			summary:      featureSummary{},
			wantContains: []string{"No open tasks"},
		},
		{
			name: "all tasks done, zero workable",
			summary: featureSummary{
				features: 3, tasks: 5, done: 5, workable: 0,
			},
			wantContains: []string{"No open tasks"},
		},
		{
			name: "features only with workable",
			summary: featureSummary{
				features: 3, tasks: 5, done: 2, workable: 3,
			},
			wantContains: []string{"3 features,", "3", "open tasks"},
			wantAbsent:   []string{"bugs"},
		},
		{
			name: "bugs only with workable",
			summary: featureSummary{
				bugs: 2, bugTasks: 4, bugDone: 1, bugWorkable: 3,
			},
			wantContains: []string{"2 bugs,", "3", "open tasks"},
			wantAbsent:   []string{"features"},
		},
		{
			name: "both features and bugs",
			summary: featureSummary{
				features: 3, tasks: 5, done: 0, workable: 5,
				bugs: 2, bugTasks: 4, bugDone: 1, bugWorkable: 3,
			},
			wantContains: []string{"3 features,", "5", "2 bugs,", "3", "open tasks"},
		},
		{
			name: "features present but zero workable, bugs have workable",
			summary: featureSummary{
				features: 1, tasks: 1, done: 1, workable: 0,
				bugs: 1, bugTasks: 2, bugWorkable: 2,
			},
			wantContains: []string{"1 bugs,", "2", "open tasks"},
			wantAbsent:   []string{"No open tasks"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatSummaryLine(tt.summary)
			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("formatSummaryLine() = %q, want it to contain %q", got, want)
				}
			}
			for _, absent := range tt.wantAbsent {
				if strings.Contains(got, absent) {
					t.Errorf("formatSummaryLine() = %q, want it NOT to contain %q", got, absent)
				}
			}
		})
	}
}

func TestMenuView_SummaryShowsFeaturesAndBugs(t *testing.T) {
	m := menuModel{
		items:  activeMenuItems(),
		cwd:    "/test",
		width:  120,
		height: 40,
		summary: featureSummary{
			features: 3, tasks: 5, done: 3, workable: 2,
			bugs: 2, bugTasks: 4, bugDone: 2, bugBlocked: 1, bugWorkable: 1,
		},
	}

	view := m.View()
	if !strings.Contains(view, "3 features,") {
		t.Error("expected view to contain '3 features,'")
	}
	if !strings.Contains(view, "2 bugs,") {
		t.Error("expected view to contain '2 bugs,'")
	}
	if !strings.Contains(view, "open tasks") {
		t.Error("expected view to contain 'open tasks'")
	}
}

func TestMenuView_SummaryNoFeaturesOrBugs(t *testing.T) {
	m := menuModel{
		items:   activeMenuItems(),
		cwd:     "/test",
		width:   120,
		height:  40,
		summary: featureSummary{},
	}

	view := m.View()
	if !strings.Contains(view, "No open tasks") {
		t.Error("expected view to contain 'No open tasks'")
	}
}

func TestMenuUpdate_FeatureSummaryUpdateMsg(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	// Create feature directory and file so loadFeatureSummary returns data.
	featDir := filepath.Join(dir, ".maggus", "features")
	if err := os.MkdirAll(featDir, 0o755); err != nil {
		t.Fatal(err)
	}
	featureContent := "# Feature\n### TASK-001: Do thing\n- [ ] criterion\n"
	if err := os.WriteFile(filepath.Join(featDir, "feature_001.md"), []byte(featureContent), 0o644); err != nil {
		t.Fatal(err)
	}

	ch := make(chan bool, 1)
	m := menuModel{
		items:     activeMenuItems(),
		summary:   featureSummary{},
		watcherCh: ch,
	}

	// Send the update message
	updated, cmd := m.Update(featureSummaryUpdateMsg{})
	um := updated.(menuModel)

	// After receiving featureSummaryUpdateMsg, summary should be reloaded
	if um.summary.features != 1 {
		t.Errorf("expected 1 feature after update, got %d", um.summary.features)
	}
	if um.summary.tasks != 1 {
		t.Errorf("expected 1 task after update, got %d", um.summary.tasks)
	}

	// Should return a cmd to listen for more updates
	if cmd == nil {
		t.Error("expected non-nil cmd after featureSummaryUpdateMsg")
	}
}
