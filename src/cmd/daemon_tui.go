package cmd

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/leberkas-org/maggus/internal/agent"
	"github.com/leberkas-org/maggus/internal/runner"
)

// nullTUIModel is a minimal bubbletea model used in daemon mode.
// It discards all display messages but correctly handles QuitMsg to
// terminate the event loop, and auto-responds to SyncCheckMsg so the
// work goroutine is never left waiting for user input.
type nullTUIModel struct {
	taskID   string
	onToolUse func(taskID, toolType, description string)
	onOutput  func(taskID, text string)
}

// SetOnToolUse sets a callback invoked on each tool use event.
func (m *nullTUIModel) SetOnToolUse(fn func(taskID, toolType, description string)) {
	m.onToolUse = fn
}

// SetOnOutput sets a callback invoked on each agent output event.
func (m *nullTUIModel) SetOnOutput(fn func(taskID, text string)) {
	m.onOutput = fn
}

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
	case runner.IterationStartMsg:
		m.taskID = msg.TaskID
	case agent.ToolMsg:
		if m.onToolUse != nil {
			m.onToolUse(m.taskID, msg.Type, msg.Description)
		}
	case agent.OutputMsg:
		if m.onOutput != nil {
			m.onOutput(m.taskID, strings.TrimSpace(msg.Text))
		}
	}
	return m, nil
}

func (m nullTUIModel) View() string { return "" }
