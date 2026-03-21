package runner

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestHandleSummaryKeysEnterExits(t *testing.T) {
	s := summaryState{}

	quitting, cmd := s.handleSummaryKeys(tea.KeyMsg{Type: tea.KeyEnter})
	if !quitting {
		t.Error("Enter should set quitting=true")
	}
	if cmd == nil {
		t.Error("Enter should return a tea.Cmd (tea.Quit)")
	}
}

func TestHandleSummaryKeysEscapeExits(t *testing.T) {
	s := summaryState{}

	quitting, cmd := s.handleSummaryKeys(tea.KeyMsg{Type: tea.KeyEscape})
	if !quitting {
		t.Error("Escape should trigger quit")
	}
	if cmd == nil {
		t.Error("Escape should return tea.Quit cmd")
	}
}

func TestHandleSummaryKeysCtrlCExits(t *testing.T) {
	s := summaryState{}

	quitting, cmd := s.handleSummaryKeys(tea.KeyMsg{Type: tea.KeyCtrlC})
	if !quitting {
		t.Error("Ctrl+C should trigger quit")
	}
	if cmd == nil {
		t.Error("Ctrl+C should return tea.Quit cmd")
	}
}

func TestHandleSummaryKeysQExits(t *testing.T) {
	s := summaryState{}

	quitting, _ := s.handleSummaryKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if !quitting {
		t.Error("q key should trigger quit")
	}
}

func TestHandleSummaryKeysUpperQExits(t *testing.T) {
	s := summaryState{}

	quitting, _ := s.handleSummaryKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'Q'}})
	if !quitting {
		t.Error("Q key should trigger quit")
	}
}

func TestHandleSummaryKeysOtherKeyDoesNotExit(t *testing.T) {
	s := summaryState{}

	quitting, _ := s.handleSummaryKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if quitting {
		t.Error("arbitrary key should not trigger quit")
	}
}
