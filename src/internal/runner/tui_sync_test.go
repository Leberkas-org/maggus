package runner

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestBuildOptionsSetsCorrectMenuItems(t *testing.T) {
	s := syncState{}
	s.buildOptions()

	expectedLabels := []string{"Pull", "Pull with rebase", "Force pull", "Skip", "Abort"}
	if len(s.options) != len(expectedLabels) {
		t.Fatalf("got %d options, want %d", len(s.options), len(expectedLabels))
	}
	for i, want := range expectedLabels {
		if s.options[i].label != want {
			t.Errorf("option[%d].label = %q, want %q", i, s.options[i].label, want)
		}
	}
	if s.cursor != 0 {
		t.Errorf("cursor = %d after buildOptions, want 0", s.cursor)
	}
}

func TestHandleSyncKeysNavigationUpDown(t *testing.T) {
	s := syncState{}
	s.buildOptions()
	cancel := func() {}

	// Move down
	s.handleSyncKeys(tea.KeyMsg{Type: tea.KeyDown}, &cancel)
	if s.cursor != 1 {
		t.Errorf("cursor after down = %d, want 1", s.cursor)
	}

	s.handleSyncKeys(tea.KeyMsg{Type: tea.KeyDown}, &cancel)
	if s.cursor != 2 {
		t.Errorf("cursor after second down = %d, want 2", s.cursor)
	}

	// Move up
	s.handleSyncKeys(tea.KeyMsg{Type: tea.KeyUp}, &cancel)
	if s.cursor != 1 {
		t.Errorf("cursor after up = %d, want 1", s.cursor)
	}

	// Can't go above 0
	s.handleSyncKeys(tea.KeyMsg{Type: tea.KeyUp}, &cancel)
	s.handleSyncKeys(tea.KeyMsg{Type: tea.KeyUp}, &cancel)
	if s.cursor != 0 {
		t.Errorf("cursor should not go below 0, got %d", s.cursor)
	}
}

func TestHandleSyncKeysNavigationJK(t *testing.T) {
	s := syncState{}
	s.buildOptions()
	cancel := func() {}

	// j moves down
	s.handleSyncKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}, &cancel)
	if s.cursor != 1 {
		t.Errorf("cursor after j = %d, want 1", s.cursor)
	}

	// k moves up
	s.handleSyncKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}}, &cancel)
	if s.cursor != 0 {
		t.Errorf("cursor after k = %d, want 0", s.cursor)
	}
}

func TestHandleSyncKeysDownDoesNotExceedBounds(t *testing.T) {
	s := syncState{}
	s.buildOptions()
	cancel := func() {}

	// Move to last option
	for i := 0; i < 10; i++ {
		s.handleSyncKeys(tea.KeyMsg{Type: tea.KeyDown}, &cancel)
	}
	if s.cursor != len(s.options)-1 {
		t.Errorf("cursor = %d, want %d (last option)", s.cursor, len(s.options)-1)
	}
}

func TestSyncAbortSendsCorrectResultOnChannel(t *testing.T) {
	resultCh := make(chan SyncCheckResult, 1)
	s := syncState{
		active:   true,
		resultCh: resultCh,
	}
	s.buildOptions()
	cancel := func() {}

	// Move cursor to "Abort" (index 4)
	s.cursor = 4

	s.handleSyncKeys(tea.KeyMsg{Type: tea.KeyEnter}, &cancel)

	select {
	case result := <-resultCh:
		if result.Action != SyncAbort {
			t.Errorf("expected SyncAbort, got %v", result.Action)
		}
	default:
		t.Fatal("expected result on channel, got nothing")
	}

	if s.active {
		t.Error("sync should be inactive after abort")
	}
}

func TestSyncSkipSendsProceedResult(t *testing.T) {
	resultCh := make(chan SyncCheckResult, 1)
	s := syncState{
		active:   true,
		resultCh: resultCh,
	}
	s.buildOptions()
	cancel := func() {}

	// Move cursor to "Skip" (index 3)
	s.cursor = 3
	s.handleSyncKeys(tea.KeyMsg{Type: tea.KeyEnter}, &cancel)

	select {
	case result := <-resultCh:
		if result.Action != SyncProceed {
			t.Errorf("expected SyncProceed, got %v", result.Action)
		}
		if result.Message != "Skipped git sync" {
			t.Errorf("unexpected message: %q", result.Message)
		}
	default:
		t.Fatal("expected result on channel, got nothing")
	}
}

func TestSyncEscAbortsRun(t *testing.T) {
	resultCh := make(chan SyncCheckResult, 1)
	cancelCalled := false
	cancelFn := func() { cancelCalled = true }
	s := syncState{
		active:   true,
		resultCh: resultCh,
	}
	s.buildOptions()

	_, interrupting := s.handleSyncKeys(tea.KeyMsg{Type: tea.KeyEsc}, &cancelFn)

	if !interrupting {
		t.Error("Esc should signal interrupting=true")
	}
	if s.active {
		t.Error("sync should be inactive after Esc")
	}
	if !cancelCalled {
		t.Error("cancel function should have been called")
	}

	select {
	case result := <-resultCh:
		if result.Action != SyncAbort {
			t.Errorf("expected SyncAbort, got %v", result.Action)
		}
	default:
		t.Fatal("expected abort result on channel")
	}
}

func TestSyncCtrlCAbortsRun(t *testing.T) {
	resultCh := make(chan SyncCheckResult, 1)
	cancelCalled := false
	cancelFn := func() { cancelCalled = true }
	s := syncState{
		active:   true,
		resultCh: resultCh,
	}
	s.buildOptions()

	_, interrupting := s.handleSyncKeys(tea.KeyMsg{Type: tea.KeyCtrlC}, &cancelFn)

	if !interrupting {
		t.Error("Ctrl+C should signal interrupting=true")
	}
	if !cancelCalled {
		t.Error("cancel function should have been called")
	}

	select {
	case result := <-resultCh:
		if result.Action != SyncAbort {
			t.Errorf("expected SyncAbort from Ctrl+C, got %v", result.Action)
		}
	default:
		t.Fatal("expected abort result on channel")
	}
}

func TestSyncForcePullShowsConfirmation(t *testing.T) {
	s := syncState{active: true}
	s.buildOptions()
	cancel := func() {}

	// Select "Force pull" (index 2)
	s.cursor = 2
	s.handleSyncKeys(tea.KeyMsg{Type: tea.KeyEnter}, &cancel)

	if !s.confirmForce {
		t.Error("expected confirmForce to be true after selecting Force pull")
	}

	// Press 'n' to cancel
	s.handleSyncKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}}, &cancel)
	if s.confirmForce {
		t.Error("expected confirmForce to be false after pressing 'n'")
	}
}
