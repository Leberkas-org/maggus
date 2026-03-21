package cmd

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/leberkas-org/maggus/internal/claude2x"
)

// claude2xTickMsg is sent every second while 2x mode is active,
// triggering a recomputation of the cached 2x status.
type claude2xTickMsg struct{}

// next2xTick returns a tea.Cmd that emits a claude2xTickMsg after one second.
func next2xTick() tea.Cmd {
	return tea.Tick(time.Second, func(_ time.Time) tea.Msg {
		return claude2xTickMsg{}
	})
}

// fetch2xAndUpdate calls claude2x.FetchStatus() (cached, no API call) and
// returns the updated is2x flag, expires string, and a tea.Cmd to schedule the
// next tick if still in 2x mode.
func fetch2xAndUpdate() (bool, string, tea.Cmd) {
	status := claude2x.FetchStatus()
	if status.Is2x {
		return true, status.TwoXWindowExpiresIn, next2xTick()
	}
	return false, "", nil
}
