package filebrowser

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// fakeDirEntry implements os.DirEntry for testing.
type fakeDirEntry struct {
	name  string
	isDir bool
}

func (f fakeDirEntry) Name() string      { return f.name }
func (f fakeDirEntry) IsDir() bool       { return f.isDir }
func (f fakeDirEntry) Type() fs.FileMode { return 0 }
func (f fakeDirEntry) Info() (fs.FileInfo, error) {
	return fakeFileInfo{name: f.name, isDir: f.isDir}, nil
}

type fakeFileInfo struct {
	name  string
	isDir bool
}

func (f fakeFileInfo) Name() string       { return f.name }
func (f fakeFileInfo) Size() int64        { return 0 }
func (f fakeFileInfo) Mode() fs.FileMode  { return 0 }
func (f fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (f fakeFileInfo) IsDir() bool        { return f.isDir }
func (f fakeFileInfo) Sys() any           { return nil }

func dirEntries(names ...string) []os.DirEntry {
	var entries []os.DirEntry
	for _, n := range names {
		entries = append(entries, fakeDirEntry{name: n, isDir: true})
	}
	return entries
}

// mockReadDir creates a readDir function that returns the given entries for a path.
// Keys in dirs must already be filepath.Clean'd.
func mockReadDir(dirs map[string][]os.DirEntry) func(string) ([]os.DirEntry, error) {
	return func(path string) ([]os.DirEntry, error) {
		path = filepath.Clean(path)
		if entries, ok := dirs[path]; ok {
			return entries, nil
		}
		return nil, fmt.Errorf("access denied: %s", path)
	}
}

// p normalizes a Unix-style test path for the current OS.
func p(s string) string { return filepath.Clean(s) }

// dirs builds a path→entries map with OS-normalized keys.
func dirs(m map[string][]os.DirEntry) map[string][]os.DirEntry {
	out := make(map[string][]os.DirEntry, len(m))
	for k, v := range m {
		out[p(k)] = v
	}
	return out
}

func newTestModel(startDir string, d map[string][]os.DirEntry) Model {
	m := Model{
		currentDir: p(startDir),
		readDir:    mockReadDir(d),
		width:      80,
		height:     24,
	}
	m.loadEntries()
	return m
}

func TestNew_StartsInGivenDirectory(t *testing.T) {
	dir := t.TempDir()
	m := New(dir)
	if m.currentDir != dir {
		t.Errorf("expected currentDir=%q, got %q", dir, m.currentDir)
	}
}

func TestShowsOnlyDirectories(t *testing.T) {
	d := dirs(map[string][]os.DirEntry{
		"/test": {
			fakeDirEntry{name: "alpha", isDir: true},
			fakeDirEntry{name: "file.txt", isDir: false},
			fakeDirEntry{name: "beta", isDir: true},
			fakeDirEntry{name: "readme.md", isDir: false},
		},
	})
	m := newTestModel("/test", d)

	if len(m.entries) != 3 {
		t.Fatalf("expected 3 entries, got %d: %v", len(m.entries), entryNames(m.entries))
	}
	if m.entries[0].name != ".." {
		t.Errorf("first entry should be '..', got %q", m.entries[0].name)
	}
	if m.entries[1].name != "alpha" {
		t.Errorf("expected 'alpha', got %q", m.entries[1].name)
	}
	if m.entries[2].name != "beta" {
		t.Errorf("expected 'beta', got %q", m.entries[2].name)
	}
}

func TestSortedAlphabetically(t *testing.T) {
	d := dirs(map[string][]os.DirEntry{
		"/test": dirEntries("charlie", "alpha", "bravo"),
	})
	m := newTestModel("/test", d)

	expected := []string{"..", "alpha", "bravo", "charlie"}
	got := entryNames(m.entries)
	for i, name := range expected {
		if got[i] != name {
			t.Errorf("entry %d: expected %q, got %q", i, name, got[i])
		}
	}
}

func TestEnterDescendsIntoDirectory(t *testing.T) {
	d := dirs(map[string][]os.DirEntry{
		"/test":     dirEntries("sub"),
		"/test/sub": dirEntries("deep"),
	})
	m := newTestModel("/test", d)

	// Cursor starts at 0 (..), move down to "sub".
	m, _ = update(m, tea.KeyMsg{Type: tea.KeyDown})
	m, _ = update(m, tea.KeyMsg{Type: tea.KeyEnter})

	if m.currentDir != p("/test/sub") {
		t.Errorf("expected dir=%q, got %q", p("/test/sub"), m.currentDir)
	}
}

func TestBackspaceGoesToParent(t *testing.T) {
	d := dirs(map[string][]os.DirEntry{
		"/test/sub": dirEntries("deep"),
		"/test":     dirEntries("sub"),
	})
	m := newTestModel("/test/sub", d)

	m, _ = update(m, tea.KeyMsg{Type: tea.KeyBackspace})

	if m.currentDir != p("/test") {
		t.Errorf("expected dir=%q, got %q", p("/test"), m.currentDir)
	}
}

func TestDotDotNavigatesToParent(t *testing.T) {
	d := dirs(map[string][]os.DirEntry{
		"/test/sub": dirEntries("deep"),
		"/test":     dirEntries("sub"),
	})
	m := newTestModel("/test/sub", d)

	if m.entries[0].name != ".." {
		t.Fatal("expected first entry to be '..'")
	}
	m, _ = update(m, tea.KeyMsg{Type: tea.KeyEnter})

	if m.currentDir != p("/test") {
		t.Errorf("expected dir=%q, got %q", p("/test"), m.currentDir)
	}
}

func TestSelectConfirmsCurrentDirectory(t *testing.T) {
	d := dirs(map[string][]os.DirEntry{
		"/test": dirEntries("sub"),
	})
	m := newTestModel("/test", d)

	m, cmd := update(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})

	if m.selected != p("/test") {
		t.Errorf("expected selected=%q, got %q", p("/test"), m.selected)
	}
	if m.Selected() != p("/test") {
		t.Errorf("Selected() should return %q", p("/test"))
	}
	if cmd == nil {
		t.Error("expected tea.Quit command")
	}
}

func TestEscCancels(t *testing.T) {
	d := dirs(map[string][]os.DirEntry{
		"/test": dirEntries("sub"),
	})
	m := newTestModel("/test", d)

	m, cmd := update(m, tea.KeyMsg{Type: tea.KeyEsc})

	if !m.cancelled {
		t.Error("expected cancelled=true")
	}
	if !m.Cancelled() {
		t.Error("Cancelled() should return true")
	}
	if m.selected != "" {
		t.Errorf("expected no selection, got %q", m.selected)
	}
	if cmd == nil {
		t.Error("expected tea.Quit command")
	}
}

func TestHiddenDirectoriesMarkedHidden(t *testing.T) {
	d := dirs(map[string][]os.DirEntry{
		"/test": dirEntries(".hidden", "visible", ".git"),
	})
	m := newTestModel("/test", d)

	for _, e := range m.entries {
		if e.name == ".hidden" && !e.hidden {
			t.Error(".hidden should be marked as hidden")
		}
		if e.name == ".git" && !e.hidden {
			t.Error(".git should be marked as hidden")
		}
		if e.name == "visible" && e.hidden {
			t.Error("visible should not be marked as hidden")
		}
	}
}

func TestPermissionErrorShowsInline(t *testing.T) {
	d := dirs(map[string][]os.DirEntry{
		"/test": dirEntries("denied"),
		// /test/denied is NOT in the map, so readDir will return an error.
	})
	m := newTestModel("/test", d)

	// Navigate into "denied" which will fail.
	m, _ = update(m, tea.KeyMsg{Type: tea.KeyDown}) // cursor to "denied"
	m, _ = update(m, tea.KeyMsg{Type: tea.KeyEnter})

	if m.errMsg == "" {
		t.Error("expected error message for permission denied directory")
	}
	if !strings.Contains(m.errMsg, "access denied") {
		t.Errorf("error should mention access denied, got: %s", m.errMsg)
	}
	// Should not crash — model is still usable.
	_ = m.View()
}

func TestCursorBoundaries(t *testing.T) {
	d := dirs(map[string][]os.DirEntry{
		"/test": dirEntries("a", "b", "c"),
	})
	m := newTestModel("/test", d)

	// Move up at top — should stay at 0.
	m, _ = update(m, tea.KeyMsg{Type: tea.KeyUp})
	if m.cursor != 0 {
		t.Errorf("cursor should stay at 0 when at top, got %d", m.cursor)
	}

	// Move to bottom.
	for range 10 {
		m, _ = update(m, tea.KeyMsg{Type: tea.KeyDown})
	}
	if m.cursor != len(m.entries)-1 {
		t.Errorf("cursor should be at last entry %d, got %d", len(m.entries)-1, m.cursor)
	}

	// Move down at bottom — should stay.
	m, _ = update(m, tea.KeyMsg{Type: tea.KeyDown})
	if m.cursor != len(m.entries)-1 {
		t.Errorf("cursor should stay at last entry, got %d", m.cursor)
	}
}

func TestScrollingWithManyEntries(t *testing.T) {
	names := make([]string, 50)
	for i := range names {
		names[i] = fmt.Sprintf("dir_%02d", i)
	}
	d := dirs(map[string][]os.DirEntry{
		"/test": dirEntries(names...),
	})
	m := newTestModel("/test", d)
	m.height = 10 // small viewport

	// Move cursor far down.
	for range 30 {
		m, _ = update(m, tea.KeyMsg{Type: tea.KeyDown})
	}

	// Cursor should be visible within the scroll window.
	vis := m.visibleLines()
	if m.cursor < m.scrollOffset || m.cursor >= m.scrollOffset+vis {
		t.Errorf("cursor %d out of visible range [%d, %d)", m.cursor, m.scrollOffset, m.scrollOffset+vis)
	}
}

func TestViewRendersPath(t *testing.T) {
	d := dirs(map[string][]os.DirEntry{
		"/test": dirEntries("sub"),
	})
	m := newTestModel("/test", d)
	m.width = 80
	m.height = 24

	view := m.View()
	// On Windows, /test becomes \test or C:\test — just check it appears.
	if !strings.Contains(view, "test") {
		t.Error("view should contain the current path")
	}
}

func TestViewRendersEntries(t *testing.T) {
	d := dirs(map[string][]os.DirEntry{
		"/test": dirEntries("mydir"),
	})
	m := newTestModel("/test", d)
	m.width = 80
	m.height = 24

	view := m.View()
	if !strings.Contains(view, "mydir") {
		t.Error("view should contain directory name 'mydir'")
	}
	if !strings.Contains(view, "..") {
		t.Error("view should contain '..' parent entry")
	}
}

func TestQuitKeyAlsoCancels(t *testing.T) {
	d := dirs(map[string][]os.DirEntry{
		"/test": dirEntries("sub"),
	})
	m := newTestModel("/test", d)

	m, cmd := update(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if !m.cancelled {
		t.Error("q should cancel")
	}
	if cmd == nil {
		t.Error("expected tea.Quit command")
	}
}

func TestWindowSizeUpdatesViewport(t *testing.T) {
	d := dirs(map[string][]os.DirEntry{
		"/test": dirEntries("a"),
	})
	m := newTestModel("/test", d)

	m, _ = update(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	if m.width != 120 || m.height != 40 {
		t.Errorf("expected 120x40, got %dx%d", m.width, m.height)
	}
}

// --- drive navigation tests ---

// newDriveTestModel creates a model at a drive root with mocked drive support.
func newDriveTestModel(startDir string, d map[string][]os.DirEntry, drives []string) Model {
	m := Model{
		currentDir:     p(startDir),
		readDir:        mockReadDir(d),
		listDrives:     func() []string { return drives },
		canSwitchDrive: func(dir string) bool { return dir == p(startDir) },
		width:          80,
		height:         24,
	}
	m.loadEntries()
	return m
}

func TestDriveRoot_ShowsSwitchDriveEntry(t *testing.T) {
	d := dirs(map[string][]os.DirEntry{
		"C:/": dirEntries("Users", "Windows"),
	})
	m := newDriveTestModel("C:/", d, []string{`C:\`, `D:\`})

	names := entryNames(m.entries)
	if names[0] != switchDriveLabel {
		t.Errorf("expected first entry to be %q, got %q", switchDriveLabel, names[0])
	}
	if len(m.entries) != 3 { // switchDrive + Users + Windows
		t.Errorf("expected 3 entries, got %d: %v", len(m.entries), names)
	}
}

func TestDriveRoot_SelectSwitchDriveShowsDriveList(t *testing.T) {
	d := dirs(map[string][]os.DirEntry{
		"C:/": dirEntries("Users"),
	})
	m := newDriveTestModel("C:/", d, []string{`C:\`, `D:\`, `E:\`})

	// First entry is "⮤ Drives", select it.
	m, _ = update(m, tea.KeyMsg{Type: tea.KeyEnter})

	if !m.showDrives {
		t.Error("expected showDrives=true after selecting switch drive")
	}
	names := entryNames(m.entries)
	expected := []string{`C:\`, `D:\`, `E:\`}
	if len(names) != len(expected) {
		t.Fatalf("expected %d drives, got %d: %v", len(expected), len(names), names)
	}
	for i, exp := range expected {
		if names[i] != exp {
			t.Errorf("drive %d: expected %q, got %q", i, exp, names[i])
		}
	}
}

func TestDriveList_SelectDriveNavigatesInto(t *testing.T) {
	d := dirs(map[string][]os.DirEntry{
		"C:/": dirEntries("Users"),
		"D:/": dirEntries("Data", "Games"),
	})
	m := newDriveTestModel("C:/", d, []string{`C:\`, `D:\`})

	// Open drive list.
	m, _ = update(m, tea.KeyMsg{Type: tea.KeyEnter})
	if !m.showDrives {
		t.Fatal("expected showDrives=true")
	}

	// Select D:\ (second entry, index 1).
	m, _ = update(m, tea.KeyMsg{Type: tea.KeyDown})
	m, _ = update(m, tea.KeyMsg{Type: tea.KeyEnter})

	if m.showDrives {
		t.Error("expected showDrives=false after selecting a drive")
	}
	if m.currentDir != p("D:/") {
		t.Errorf("expected currentDir=%q, got %q", p("D:/"), m.currentDir)
	}
	// Should list D:\ contents.
	names := entryNames(m.entries)
	if len(names) < 2 {
		t.Fatalf("expected entries for D:\\, got %v", names)
	}
}

func TestDriveList_BackspaceCancels(t *testing.T) {
	d := dirs(map[string][]os.DirEntry{
		"C:/": dirEntries("Users"),
	})
	m := newDriveTestModel("C:/", d, []string{`C:\`, `D:\`})

	// Open drive list.
	m, _ = update(m, tea.KeyMsg{Type: tea.KeyEnter})
	if !m.showDrives {
		t.Fatal("expected showDrives=true")
	}

	// Backspace cancels drive listing.
	m, _ = update(m, tea.KeyMsg{Type: tea.KeyBackspace})
	if m.showDrives {
		t.Error("expected showDrives=false after backspace")
	}
	if m.currentDir != p("C:/") {
		t.Errorf("expected currentDir=%q, got %q", p("C:/"), m.currentDir)
	}
}

func TestBackspaceAtDriveRoot_ShowsDrives(t *testing.T) {
	d := dirs(map[string][]os.DirEntry{
		"C:/": dirEntries("Users"),
	})
	m := newDriveTestModel("C:/", d, []string{`C:\`, `D:\`})

	// Backspace at drive root should show drive list.
	m, _ = update(m, tea.KeyMsg{Type: tea.KeyBackspace})
	if !m.showDrives {
		t.Error("expected showDrives=true after backspace at drive root")
	}
}

func TestDriveViewShowsAvailableDrivesHeader(t *testing.T) {
	d := dirs(map[string][]os.DirEntry{
		"C:/": dirEntries("Users"),
	})
	m := newDriveTestModel("C:/", d, []string{`C:\`, `D:\`})

	// Open drive list.
	m, _ = update(m, tea.KeyMsg{Type: tea.KeyEnter})

	view := m.View()
	if !strings.Contains(view, "Available Drives") {
		t.Error("drive list view should show 'Available Drives' header")
	}
}

func TestDriveViewShowsDriveIcon(t *testing.T) {
	d := dirs(map[string][]os.DirEntry{
		"C:/": dirEntries("Users"),
	})
	m := newDriveTestModel("C:/", d, []string{`C:\`, `D:\`})

	// Open drive list.
	m, _ = update(m, tea.KeyMsg{Type: tea.KeyEnter})

	view := m.View()
	if !strings.Contains(view, "💿") {
		t.Error("drive list view should show drive icon 💿")
	}
}

func TestNoDriveSupport_NormalBehavior(t *testing.T) {
	// When canSwitchDrive is nil (non-Windows), behavior is unchanged.
	d := dirs(map[string][]os.DirEntry{
		"/": dirEntries("usr", "etc"),
	})
	m := Model{
		currentDir: p("/"),
		readDir:    mockReadDir(d),
		width:      80,
		height:     24,
		// listDrives and canSwitchDrive are nil (non-Windows default in tests).
	}
	m.loadEntries()

	names := entryNames(m.entries)
	// No ".." and no switchDriveLabel at root.
	for _, n := range names {
		if n == ".." || n == switchDriveLabel {
			t.Errorf("unexpected nav entry %q at root without drive support", n)
		}
	}

	// Backspace at root should be a no-op.
	before := m.currentDir
	m, _ = update(m, tea.KeyMsg{Type: tea.KeyBackspace})
	if m.currentDir != before {
		t.Errorf("backspace at root should be no-op, dir changed to %q", m.currentDir)
	}
	if m.showDrives {
		t.Error("showDrives should remain false without drive support")
	}
}

// --- helpers ---

func update(m Model, msg tea.Msg) (Model, tea.Cmd) {
	newM, cmd := m.Update(msg)
	return newM.(Model), cmd
}

func entryNames(entries []dirEntry) []string {
	var names []string
	for _, e := range entries {
		names = append(names, e.name)
	}
	return names
}
