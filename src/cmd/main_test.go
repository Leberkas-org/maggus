package cmd

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNewMainModel(t *testing.T) {
	m := newMainModel()
	if m.width != 0 || m.height != 0 {
		t.Errorf("expected zero dimensions, got %dx%d", m.width, m.height)
	}
}

func TestMainUpdate_WindowSize(t *testing.T) {
	m := newMainModel()
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	mm := updated.(mainModel)
	if mm.width != 120 || mm.height != 40 {
		t.Errorf("expected 120x40, got %dx%d", mm.width, mm.height)
	}
}

func TestMainUpdate_Quit(t *testing.T) {
	m := newMainModel()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if cmd == nil {
		t.Fatal("expected quit command, got nil")
	}
}

func TestMainView_ZeroDimensions(t *testing.T) {
	m := newMainModel()
	if v := m.View(); v != "" {
		t.Errorf("expected empty string for zero dimensions, got %q", v)
	}
}

func TestMainView_SplitPane(t *testing.T) {
	m := mainModel{width: 100, height: 30}
	v := m.View()
	if !strings.Contains(v, "│") {
		t.Error("expected split pane divider │")
	}
	if !strings.Contains(v, "q: quit") {
		t.Error("expected footer text 'q: quit'")
	}
}
