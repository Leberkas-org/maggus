package runner

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/leberkas-org/maggus/internal/claude2x"
)

// claude2xTickMsg is sent every second while 2x mode is active,
// triggering a refresh of the cached 2x countdown in the banner.
type claude2xTickMsg struct{}

// next2xTick returns a tea.Cmd that emits a claude2xTickMsg after one second.
func next2xTick() tea.Cmd {
	return tea.Tick(time.Second, func(_ time.Time) tea.Msg {
		return claude2xTickMsg{}
	})
}

// handle2xTick refreshes the banner's 2x status from the cached value
// and schedules the next tick if 2x mode is still active.
func (m *TUIModel) handle2xTick() tea.Cmd {
	status := claude2x.FetchStatus()
	if status.Is2x {
		m.banner.TwoXExpiresIn = status.TwoXWindowExpiresIn
		return next2xTick()
	}
	m.banner.TwoXExpiresIn = ""
	return nil
}
