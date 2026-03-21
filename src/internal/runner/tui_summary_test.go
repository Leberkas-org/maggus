package runner

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestHandleSummaryKeysMenuNavigation(t *testing.T) {
	s := summaryState{menuChoice: 0}

	// Down moves to 1
	s.handleSummaryKeys(tea.KeyMsg{Type: tea.KeyDown})
	if s.menuChoice != 1 {
		t.Errorf("menuChoice after down = %d, want 1", s.menuChoice)
	}

	// Down again stays at 1 (max)
	s.handleSummaryKeys(tea.KeyMsg{Type: tea.KeyDown})
	if s.menuChoice != 1 {
		t.Errorf("menuChoice should not exceed 1, got %d", s.menuChoice)
	}

	// Up moves back to 0
	s.handleSummaryKeys(tea.KeyMsg{Type: tea.KeyUp})
	if s.menuChoice != 0 {
		t.Errorf("menuChoice after up = %d, want 0", s.menuChoice)
	}

	// Up again stays at 0 (min)
	s.handleSummaryKeys(tea.KeyMsg{Type: tea.KeyUp})
	if s.menuChoice != 0 {
		t.Errorf("menuChoice should not go below 0, got %d", s.menuChoice)
	}
}

func TestHandleSummaryKeysJKNavigation(t *testing.T) {
	s := summaryState{menuChoice: 0}

	// j moves down
	s.handleSummaryKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if s.menuChoice != 1 {
		t.Errorf("menuChoice after j = %d, want 1", s.menuChoice)
	}

	// k moves up
	s.handleSummaryKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if s.menuChoice != 0 {
		t.Errorf("menuChoice after k = %d, want 0", s.menuChoice)
	}
}

func TestHandleSummaryKeysTabNavigation(t *testing.T) {
	s := summaryState{menuChoice: 0}

	// Tab moves down
	s.handleSummaryKeys(tea.KeyMsg{Type: tea.KeyTab})
	if s.menuChoice != 1 {
		t.Errorf("menuChoice after tab = %d, want 1", s.menuChoice)
	}

	// Shift+Tab moves up
	s.handleSummaryKeys(tea.KeyMsg{Type: tea.KeyShiftTab})
	if s.menuChoice != 0 {
		t.Errorf("menuChoice after shift+tab = %d, want 0", s.menuChoice)
	}
}

func TestHandleSummaryKeysEnterOnExitTriggersQuit(t *testing.T) {
	s := summaryState{menuChoice: 0}

	quitting, cmd := s.handleSummaryKeys(tea.KeyMsg{Type: tea.KeyEnter})
	if !quitting {
		t.Error("Enter on Exit should set quitting=true")
	}
	if cmd == nil {
		t.Error("Enter on Exit should return a tea.Cmd (tea.Quit)")
	}
}

func TestHandleSummaryKeysEnterOnRunAgainStartsEditing(t *testing.T) {
	s := summaryState{menuChoice: 1}

	quitting, _ := s.handleSummaryKeys(tea.KeyMsg{Type: tea.KeyEnter})
	if quitting {
		t.Error("Enter on Run Again should not quit")
	}
	if !s.editingCount {
		t.Error("expected editingCount to be true")
	}
	if s.countInput != "" {
		t.Errorf("countInput should be empty, got %q", s.countInput)
	}
}

func TestEditingCountAcceptsDigitsRejectsLetters(t *testing.T) {
	s := summaryState{editingCount: true}

	// Digits are accepted
	s.handleSummaryKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	if s.countInput != "3" {
		t.Errorf("countInput = %q, want %q", s.countInput, "3")
	}

	s.handleSummaryKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'5'}})
	if s.countInput != "35" {
		t.Errorf("countInput = %q, want %q", s.countInput, "35")
	}

	// Letters are rejected
	s.handleSummaryKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if s.countInput != "35" {
		t.Errorf("countInput = %q after letter, want %q", s.countInput, "35")
	}

	s.handleSummaryKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'Z'}})
	if s.countInput != "35" {
		t.Errorf("countInput = %q after letter, want %q", s.countInput, "35")
	}
}

func TestEditingCountBackspaceDeletesLastChar(t *testing.T) {
	s := summaryState{editingCount: true, countInput: "12"}

	s.handleSummaryKeys(tea.KeyMsg{Type: tea.KeyBackspace})
	if s.countInput != "1" {
		t.Errorf("countInput = %q after backspace, want %q", s.countInput, "1")
	}

	s.handleSummaryKeys(tea.KeyMsg{Type: tea.KeyBackspace})
	if s.countInput != "" {
		t.Errorf("countInput = %q after second backspace, want empty", s.countInput)
	}
}

func TestEditingCountEnterWithValidNumberTriggersRunAgain(t *testing.T) {
	s := summaryState{editingCount: true, countInput: "5"}

	quitting, cmd := s.handleSummaryKeys(tea.KeyMsg{Type: tea.KeyEnter})
	if !quitting {
		t.Error("Enter with valid count should quit")
	}
	if cmd == nil {
		t.Error("Enter with valid count should return tea.Quit cmd")
	}
	if !s.runAgain.RunAgain {
		t.Error("runAgain.RunAgain should be true")
	}
	if s.runAgain.TaskCount != 5 {
		t.Errorf("runAgain.TaskCount = %d, want 5", s.runAgain.TaskCount)
	}
}

func TestEditingCountEnterWithInvalidInputClearsInput(t *testing.T) {
	s := summaryState{editingCount: true, countInput: ""}

	quitting, _ := s.handleSummaryKeys(tea.KeyMsg{Type: tea.KeyEnter})
	if quitting {
		t.Error("Enter with empty input should not quit")
	}
	if s.countInput != "" {
		t.Errorf("countInput should be cleared, got %q", s.countInput)
	}
}

func TestEditingCountEnterWithZeroClearsInput(t *testing.T) {
	s := summaryState{editingCount: true, countInput: "0"}

	quitting, _ := s.handleSummaryKeys(tea.KeyMsg{Type: tea.KeyEnter})
	if quitting {
		t.Error("Enter with 0 should not quit")
	}
}

func TestEditingCountEscapeCancelsEditing(t *testing.T) {
	s := summaryState{editingCount: true, countInput: "42"}

	quitting, _ := s.handleSummaryKeys(tea.KeyMsg{Type: tea.KeyEscape})
	if quitting {
		t.Error("Escape should not quit")
	}
	if s.editingCount {
		t.Error("editingCount should be false after Escape")
	}
	if s.countInput != "" {
		t.Errorf("countInput should be cleared after Escape, got %q", s.countInput)
	}
}

func TestEditingCountMaxLength(t *testing.T) {
	s := summaryState{editingCount: true, countInput: "9999"}

	// Should not accept 5th digit
	s.handleSummaryKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	if s.countInput != "9999" {
		t.Errorf("countInput = %q, should be capped at 4 digits", s.countInput)
	}
}

func TestSummaryQKeyQuits(t *testing.T) {
	s := summaryState{menuChoice: 0}

	quitting, _ := s.handleSummaryKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if !quitting {
		t.Error("q key should trigger quit")
	}
}

func TestSummaryEscapeQuits(t *testing.T) {
	s := summaryState{menuChoice: 0}

	quitting, cmd := s.handleSummaryKeys(tea.KeyMsg{Type: tea.KeyEscape})
	if !quitting {
		t.Error("Escape should trigger quit")
	}
	if cmd == nil {
		t.Error("Escape should return tea.Quit cmd")
	}
}
