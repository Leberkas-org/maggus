package cmd

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/leberkas-org/maggus/internal/claude2x"
)

// claude2xTickMsg is sent every second while nerfed hours are active,
// triggering a recomputation of the current status.
type claude2xTickMsg struct{}

// next2xTick returns a tea.Cmd that emits a claude2xTickMsg after one second.
func next2xTick() tea.Cmd {
	return tea.Tick(time.Second, func(_ time.Time) tea.Msg {
		return claude2xTickMsg{}
	})
}

// fetch2xAndUpdate computes the current rate window status and returns the updated
// isNerfed flag, expires string, and a tea.Cmd to schedule the next tick if nerfed.
func fetch2xAndUpdate() (isNerfed bool, expiresIn string, tickCmd tea.Cmd) {
	status := claude2x.FetchStatus()
	if status.IsNerfed {
		return true, status.TwoXWindowExpiresIn, next2xTick()
	}
	return false, "", nil
}
