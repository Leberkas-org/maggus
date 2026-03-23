package cmd

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/leberkas-org/maggus/internal/runner"
)

// nullTUIModel is a minimal bubbletea model used in daemon mode.
// It discards all display messages but correctly handles QuitMsg to
// terminate the event loop, and auto-responds to SyncCheckMsg so the
// work goroutine is never left waiting for user input.
type nullTUIModel struct{}

func (m nullTUIModel) Init() tea.Cmd { return nil }

func (m nullTUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case runner.QuitMsg:
		_ = msg
		return m, tea.Quit
	case runner.SyncCheckMsg:
		// Auto-continue in daemon mode: skip the interactive sync screen.
		go func(ch chan<- runner.SyncCheckResult) {
			ch <- runner.SyncCheckResult{
				Action:  runner.SyncProceed,
				Message: "⚠ Remote sync skipped (daemon mode)",
			}
		}(msg.ResultCh)
	}
	return m, nil
}

func (m nullTUIModel) View() string { return "" }
