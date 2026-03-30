package cmd

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/leberkas-org/maggus/internal/globalconfig"
	"github.com/leberkas-org/maggus/internal/updater"
)

func setupUpdateTest(_ *testing.T) func() {
	origVersion := Version
	origCheck := checkLatestVersion
	origApply := applyUpdate
	origLoad := loadGlobalSettings
	origSave := saveGlobalSettings
	// Default test stubs: return notify mode, discard saves.
	loadGlobalSettings = func() (globalconfig.Settings, error) {
		return globalconfig.Settings{AutoUpdate: globalconfig.AutoUpdateNotify}, nil
	}
	saveGlobalSettings = func(_ globalconfig.Settings) error { return nil }
	return func() {
		Version = origVersion
		checkLatestVersion = origCheck
		applyUpdate = origApply
		loadGlobalSettings = origLoad
		saveGlobalSettings = origSave
	}
}

func TestUpdate_DevVersion(t *testing.T) {
	defer setupUpdateTest(t)()
	Version = "dev"
	checkLatestVersion = func(v string) updater.UpdateInfo {
		return updater.UpdateInfo{
			TagName:     "v1.5.0",
			DownloadURL: "https://example.com/maggus.zip",
			IsNewer:     true,
			Body:        "New release",
		}
	}

	m := newUpdateModel(Version)
	// Simulate version check result
	model, _ := m.Update(updateCheckMsg{info: checkLatestVersion("dev")})
	um := model.(updateModel)

	if um.phase != phaseConfirm {
		t.Errorf("expected phaseConfirm, got %d", um.phase)
	}

	// Check view shows "dev → v1.5.0"
	um.width = 120
	um.height = 40
	view := um.View()
	if got := view; !contains(got, "dev") || !contains(got, "v1.5.0") {
		t.Errorf("expected dev and v1.5.0 in view, got: %s", got)
	}
}

func TestUpdate_DevTimestampVersion(t *testing.T) {
	defer setupUpdateTest(t)()
	Version = "dev-143027"
	checkLatestVersion = func(v string) updater.UpdateInfo {
		return updater.UpdateInfo{
			TagName:     "v1.5.0",
			DownloadURL: "https://example.com/maggus.zip",
			IsNewer:     true,
			Body:        "New release",
		}
	}

	m := newUpdateModel(Version)
	model, _ := m.Update(updateCheckMsg{info: checkLatestVersion("dev-143027")})
	um := model.(updateModel)

	if um.phase != phaseConfirm {
		t.Errorf("expected phaseConfirm, got %d", um.phase)
	}

	// Check view shows "dev-143027 → v1.5.0" (no "v" prefix on dev versions)
	um.width = 120
	um.height = 40
	view := um.View()
	if got := view; !contains(got, "dev-143027") || !contains(got, "v1.5.0") {
		t.Errorf("expected dev-143027 and v1.5.0 in view, got: %s", got)
	}
}

func TestUpdate_DevVersion_ApplySuccessful(t *testing.T) {
	defer setupUpdateTest(t)()
	Version = "dev"
	var appliedURL string
	checkLatestVersion = func(v string) updater.UpdateInfo {
		if v != "dev" {
			t.Errorf("expected currentVersion 'dev', got: %s", v)
		}
		return updater.UpdateInfo{
			TagName:     "v2.1.0",
			DownloadURL: "https://example.com/maggus_v2.1.0.zip",
			IsNewer:     true,
			Body:        "Bug fixes",
		}
	}
	applyUpdate = func(url string) error {
		appliedURL = url
		return nil
	}

	m := newUpdateModel(Version)
	info := checkLatestVersion("dev")

	// Check phase
	model, _ := m.Update(updateCheckMsg{info: info})
	um := model.(updateModel)
	if um.phase != phaseConfirm {
		t.Fatalf("expected phaseConfirm, got %d", um.phase)
	}

	// Confirm with 'y' key
	model, cmd := um.Update(tea.KeyMsg{Runes: []rune{'y'}})
	um = model.(updateModel)
	if um.phase != phaseDownloading {
		t.Fatalf("expected phaseDownloading, got %d", um.phase)
	}

	// Execute the command (simulates apply)
	if cmd != nil {
		msg := cmd()
		model, _ = um.Update(msg)
		um = model.(updateModel)
	}

	if um.phase != phaseSuccess {
		t.Errorf("expected phaseSuccess, got %d", um.phase)
	}
	if appliedURL != "https://example.com/maggus_v2.1.0.zip" {
		t.Errorf("expected apply with download URL, got: %s", appliedURL)
	}

	// Check view shows success
	um.width = 120
	um.height = 40
	view := um.View()
	if !contains(view, "Successfully updated") {
		t.Errorf("expected success message in view")
	}
}

func TestUpdate_AlreadyUpToDate(t *testing.T) {
	defer setupUpdateTest(t)()
	Version = "1.0.0"
	checkLatestVersion = func(v string) updater.UpdateInfo {
		return updater.UpdateInfo{TagName: "v1.0.0", IsNewer: false}
	}

	m := newUpdateModel(Version)
	model, _ := m.Update(updateCheckMsg{info: checkLatestVersion("1.0.0")})
	um := model.(updateModel)

	if um.phase != phaseUpToDate {
		t.Errorf("expected phaseUpToDate, got %d", um.phase)
	}

	um.width = 120
	um.height = 40
	view := um.View()
	if !contains(view, "Already up to date") {
		t.Errorf("expected up-to-date message in view, got: %s", view)
	}
}

func TestUpdate_UpdateAvailable_Confirmed(t *testing.T) {
	defer setupUpdateTest(t)()
	Version = "1.0.0"
	checkLatestVersion = func(v string) updater.UpdateInfo {
		return updater.UpdateInfo{
			TagName:     "v2.0.0",
			DownloadURL: "https://example.com/maggus.zip",
			IsNewer:     true,
			Body:        "Bug fixes and improvements",
		}
	}
	applyUpdate = func(url string) error { return nil }

	m := newUpdateModel(Version)
	info := checkLatestVersion("1.0.0")

	// Version check
	model, _ := m.Update(updateCheckMsg{info: info})
	um := model.(updateModel)
	if um.phase != phaseConfirm {
		t.Fatalf("expected phaseConfirm, got %d", um.phase)
	}

	// View should show version comparison and changelog
	um.width = 120
	um.height = 40
	view := um.View()
	if !contains(view, "v1.0.0") || !contains(view, "v2.0.0") {
		t.Errorf("expected version comparison in view")
	}
	if !contains(view, "Bug fixes and improvements") {
		t.Errorf("expected changelog in view")
	}

	// Confirm with enter (menuChoice 0 = Install)
	model, cmd := um.Update(tea.KeyMsg{Type: tea.KeyEnter})
	um = model.(updateModel)
	if um.phase != phaseDownloading {
		t.Fatalf("expected phaseDownloading, got %d", um.phase)
	}

	// Execute command
	if cmd != nil {
		msg := cmd()
		model, _ = um.Update(msg)
		um = model.(updateModel)
	}

	if um.phase != phaseSuccess {
		t.Errorf("expected phaseSuccess, got %d", um.phase)
	}
	um.width = 120
	um.height = 40
	view = um.View()
	if !contains(view, "Successfully updated") || !contains(view, "restart") {
		t.Errorf("expected success + restart message in view")
	}
}

func TestUpdate_UpdateAvailable_Declined(t *testing.T) {
	defer setupUpdateTest(t)()
	Version = "1.0.0"
	checkLatestVersion = func(v string) updater.UpdateInfo {
		return updater.UpdateInfo{
			TagName:     "v2.0.0",
			DownloadURL: "https://example.com/maggus.zip",
			IsNewer:     true,
		}
	}

	m := newUpdateModel(Version)
	model, _ := m.Update(updateCheckMsg{info: checkLatestVersion("1.0.0")})
	um := model.(updateModel)

	// Select Cancel (→ moves to index 1, then enter)
	model, _ = um.Update(tea.KeyMsg{Type: tea.KeyRight})
	um = model.(updateModel)
	if um.menuChoice != 1 {
		t.Errorf("expected menuChoice 1, got %d", um.menuChoice)
	}

	model, cmd := um.Update(tea.KeyMsg{Type: tea.KeyEnter})
	_ = model.(updateModel)

	// Should quit
	if cmd == nil {
		t.Fatal("expected quit command")
	}
}

func TestUpdate_ApplyError(t *testing.T) {
	defer setupUpdateTest(t)()
	Version = "1.0.0"
	checkLatestVersion = func(v string) updater.UpdateInfo {
		return updater.UpdateInfo{
			TagName:     "v2.0.0",
			DownloadURL: "https://example.com/maggus.zip",
			IsNewer:     true,
		}
	}
	applyUpdate = func(url string) error { return fmt.Errorf("permission denied") }

	m := newUpdateModel(Version)
	model, _ := m.Update(updateCheckMsg{info: checkLatestVersion("1.0.0")})
	um := model.(updateModel)

	// Confirm install
	model, cmd := um.Update(tea.KeyMsg{Runes: []rune{'y'}})
	um = model.(updateModel)

	// Execute
	if cmd != nil {
		msg := cmd()
		model, _ = um.Update(msg)
		um = model.(updateModel)
	}

	if um.phase != phaseError {
		t.Errorf("expected phaseError, got %d", um.phase)
	}
	if !contains(um.errorMsg, "permission denied") {
		t.Errorf("expected 'permission denied' in error, got: %s", um.errorMsg)
	}
}

func TestUpdate_NoDownloadURL(t *testing.T) {
	defer setupUpdateTest(t)()
	Version = "1.0.0"
	checkLatestVersion = func(v string) updater.UpdateInfo {
		return updater.UpdateInfo{
			TagName:     "v2.0.0",
			DownloadURL: "", // no asset for this platform
			IsNewer:     true,
		}
	}

	m := newUpdateModel(Version)
	model, _ := m.Update(updateCheckMsg{info: checkLatestVersion("1.0.0")})
	um := model.(updateModel)

	if um.phase != phaseError {
		t.Errorf("expected phaseError, got %d", um.phase)
	}
	if !contains(um.errorMsg, "No download available") {
		t.Errorf("expected 'No download available' in error, got: %s", um.errorMsg)
	}
}

func TestUpdate_NoChangelog(t *testing.T) {
	defer setupUpdateTest(t)()
	Version = "1.0.0"
	checkLatestVersion = func(v string) updater.UpdateInfo {
		return updater.UpdateInfo{
			TagName:     "v1.1.0",
			DownloadURL: "https://example.com/maggus.zip",
			IsNewer:     true,
			Body:        "", // no changelog
		}
	}
	applyUpdate = func(url string) error { return nil }

	m := newUpdateModel(Version)
	model, _ := m.Update(updateCheckMsg{info: checkLatestVersion("1.0.0")})
	um := model.(updateModel)

	um.width = 120
	um.height = 40
	view := um.View()
	if contains(view, "Changelog") {
		t.Errorf("should not show changelog section when body is empty")
	}

	// Confirm and apply
	model, cmd := um.Update(tea.KeyMsg{Runes: []rune{'y'}})
	um = model.(updateModel)
	if cmd != nil {
		msg := cmd()
		model, _ = um.Update(msg)
		um = model.(updateModel)
	}

	if um.phase != phaseSuccess {
		t.Errorf("expected phaseSuccess, got %d", um.phase)
	}
}

func TestUpdate_AutoUpdateToggle(t *testing.T) {
	defer setupUpdateTest(t)()
	Version = "1.0.0"
	checkLatestVersion = func(v string) updater.UpdateInfo {
		return updater.UpdateInfo{TagName: "v1.0.0", IsNewer: false}
	}

	// Track what gets saved
	var savedMode globalconfig.AutoUpdateMode
	saveGlobalSettings = func(s globalconfig.Settings) error {
		savedMode = s.AutoUpdate
		return nil
	}

	m := newUpdateModel(Version)

	// Starts at "notify" (index 1)
	if autoUpdateModes[m.autoUpdateIdx] != globalconfig.AutoUpdateNotify {
		t.Fatalf("expected initial mode notify, got %s", autoUpdateModes[m.autoUpdateIdx])
	}

	// Move to phaseUpToDate
	model, _ := m.Update(updateCheckMsg{info: checkLatestVersion("1.0.0")})
	um := model.(updateModel)

	// Press 'a' to cycle: notify → auto
	model, _ = um.Update(tea.KeyMsg{Runes: []rune{'a'}})
	um = model.(updateModel)
	if autoUpdateModes[um.autoUpdateIdx] != globalconfig.AutoUpdateAuto {
		t.Errorf("expected auto after first toggle, got %s", autoUpdateModes[um.autoUpdateIdx])
	}
	if !um.autoUpdateDirty {
		t.Error("expected dirty flag after change")
	}

	// Press 'a' again: auto → off
	model, _ = um.Update(tea.KeyMsg{Runes: []rune{'a'}})
	um = model.(updateModel)
	if autoUpdateModes[um.autoUpdateIdx] != globalconfig.AutoUpdateOff {
		t.Errorf("expected off after second toggle, got %s", autoUpdateModes[um.autoUpdateIdx])
	}

	// Press 'a' again: off → notify (back to original)
	model, _ = um.Update(tea.KeyMsg{Runes: []rune{'a'}})
	um = model.(updateModel)
	if autoUpdateModes[um.autoUpdateIdx] != globalconfig.AutoUpdateNotify {
		t.Errorf("expected notify after third toggle, got %s", autoUpdateModes[um.autoUpdateIdx])
	}
	if um.autoUpdateDirty {
		t.Error("expected dirty flag cleared when back to original")
	}

	// Change to auto and exit — should save
	model, _ = um.Update(tea.KeyMsg{Runes: []rune{'a'}})
	um = model.(updateModel)
	// Exit (any key other than 'a')
	model, _ = um.Update(tea.KeyMsg{Type: tea.KeyEnter})
	_ = model.(updateModel)

	if savedMode != globalconfig.AutoUpdateAuto {
		t.Errorf("expected saved mode auto, got %s", savedMode)
	}
}

// TestUpdate_ScrollOffsetZeroAfterCheckMsg verifies that scrollOffset is 0
// after the version check completes (BUG-022-001 diagnostic).
func TestUpdate_ScrollOffsetZeroAfterCheckMsg(t *testing.T) {
	defer setupUpdateTest(t)()
	m := newUpdateModel("1.0.0")
	// Simulate receiving the check result with a long changelog
	info := updater.UpdateInfo{
		TagName:     "v2.0.0",
		DownloadURL: "https://example.com/maggus.zip",
		IsNewer:     true,
		Body:        strings.Repeat("This is a very long changelog line that exceeds typical terminal width by far.\n", 20),
	}
	model, _ := m.Update(updateCheckMsg{info: info})
	um := model.(updateModel)
	if um.scrollOffset != 0 {
		t.Errorf("expected scrollOffset 0 after updateCheckMsg, got %d", um.scrollOffset)
	}
	if um.phase != phaseConfirm {
		t.Errorf("expected phaseConfirm, got %d", um.phase)
	}
}

// TestUpdate_TitleVisibleAtTopWithLongChangelog verifies that "Maggus Update"
// is visible as the first content line when the changelog is long (BUG-022-001).
// At scrollOffset=0 the rendered box must not overflow the terminal height so
// BubbleTea never drops lines from the top.
func TestUpdate_TitleVisibleAtTopWithLongChangelog(t *testing.T) {
	defer setupUpdateTest(t)()

	longBody := strings.Repeat("This is a very long changelog line that would exceed a typical 80-column terminal width without wrapping.\n", 30)

	info := updater.UpdateInfo{
		TagName:     "v2.0.0",
		DownloadURL: "https://example.com/maggus.zip",
		IsNewer:     true,
		Body:        longBody,
	}

	terminalSizes := [][2]int{
		{80, 24},
		{80, 40},
		{80, 60},
		{120, 24},
		{120, 40},
	}

	for _, sz := range terminalSizes {
		w, h := sz[0], sz[1]
		m := newUpdateModel("1.0.0")
		m.width = w
		m.height = h

		model, _ := m.Update(updateCheckMsg{info: info})
		um := model.(updateModel)
		um.width = w
		um.height = h

		view := um.View()
		viewLines := strings.Split(view, "\n")

		// The rendered output must not exceed the terminal height.
		// If it does, BubbleTea would drop lines from the top.
		if len(viewLines) > h {
			t.Errorf("size %dx%d: rendered view has %d lines, exceeds terminal height %d — would cause top-clipping",
				w, h, len(viewLines), h)
		}

		// "Maggus Update" must appear in the rendered output.
		if !contains(view, "Maggus Update") {
			t.Errorf("size %dx%d: 'Maggus Update' not found in view", w, h)
		}

		// "Maggus Update" must appear BEFORE "Auto:" in the rendered output,
		// confirming the title is not scrolled off the top.
		titleIdx := strings.Index(view, "Maggus Update")
		autoIdx := strings.Index(view, "Auto:")
		if titleIdx < 0 {
			t.Errorf("size %dx%d: 'Maggus Update' not in view", w, h)
		} else if autoIdx >= 0 && titleIdx > autoIdx {
			t.Errorf("size %dx%d: 'Maggus Update' (pos %d) appears after 'Auto:' (pos %d) — title is clipped",
				w, h, titleIdx, autoIdx)
		}
	}
}

// TestUpdate_FooterVisibleWithLongChangelog verifies that the footer (hints/menu)
// remains visible when the changelog is long (BUG-022-001).
func TestUpdate_FooterVisibleWithLongChangelog(t *testing.T) {
	defer setupUpdateTest(t)()

	longBody := strings.Repeat("A long changelog line that is quite verbose and likely to exceed 80 columns.\n", 30)

	m := newUpdateModel("1.0.0")
	m.width = 80
	m.height = 24

	model, _ := m.Update(updateCheckMsg{info: updater.UpdateInfo{
		TagName:     "v2.0.0",
		DownloadURL: "https://example.com/maggus.zip",
		IsNewer:     true,
		Body:        longBody,
	}})
	um := model.(updateModel)
	um.width = 80
	um.height = 24

	view := um.View()
	if !contains(view, "Install update") {
		t.Errorf("footer menu 'Install update' not visible in view")
	}
	if !contains(view, "select") {
		t.Errorf("footer hints not visible in view")
	}
}

// TestUpdate_ScrollingWorksWithLongChangelog verifies that scrolling down and
// back up works correctly in phaseConfirm with a long changelog.
func TestUpdate_ScrollingWorksWithLongChangelog(t *testing.T) {
	defer setupUpdateTest(t)()

	longBody := strings.Repeat("Changelog line number %d here.\n", 40)

	m := newUpdateModel("1.0.0")
	m.width = 80
	m.height = 24

	model, _ := m.Update(updateCheckMsg{info: updater.UpdateInfo{
		TagName:     "v2.0.0",
		DownloadURL: "https://example.com/maggus.zip",
		IsNewer:     true,
		Body:        longBody,
	}})
	um := model.(updateModel)
	um.width = 80
	um.height = 24

	// Scroll down several times.
	for i := 0; i < 5; i++ {
		model, _ = um.Update(tea.KeyMsg{Type: tea.KeyDown})
		um = model.(updateModel)
	}
	if um.scrollOffset == 0 {
		t.Error("expected scrollOffset > 0 after scrolling down")
	}

	// Scroll back to top with Home key.
	model, _ = um.Update(tea.KeyMsg{Type: tea.KeyHome})
	um = model.(updateModel)
	if um.scrollOffset != 0 {
		t.Errorf("expected scrollOffset 0 after Home, got %d", um.scrollOffset)
	}

	// Title should be visible again.
	um.width = 80
	um.height = 24
	view := um.View()
	if !contains(view, "Maggus Update") {
		t.Error("title not visible after scrolling back to top")
	}
}

// contains is a helper for checking substrings in rendered views
// which may contain ANSI escape codes.
func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && stringContains(s, substr)
}

func stringContains(s, substr string) bool {
	// Simple substring check — works even with ANSI codes in the string
	// since we're checking for text fragments that appear between escape sequences.
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
