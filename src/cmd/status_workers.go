package cmd

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/leberkas-org/maggus/internal/runlog"
	"github.com/leberkas-org/maggus/internal/tui/styles"
)

// refreshWorkerSnapshots reads the workers index and per-worker snapshots.
// Sets m.workerIndex to nil when no workers are active (parallel orchestrator
// or dispatched workers). Does not require the daemon to be running — dispatched
// workers write to the same index independently of the daemon.
func (m *statusModel) refreshWorkerSnapshots() {
	idx := runlog.ReadWorkersIndex(m.dir)
	if len(idx) == 0 {
		m.workerIndex = nil
		m.workerSnapshots = nil
		m.workerSpinners = nil
		return
	}
	m.workerIndex = idx
	if m.workerSnapshots == nil {
		m.workerSnapshots = make(map[string]*runlog.StateSnapshot)
	}
	if m.workerSpinners == nil {
		m.workerSpinners = make(map[string]int)
	}
	for _, w := range idx {
		snap, err := runlog.ReadWorkerSnapshot(m.dir, w.TaskID)
		if err == nil {
			m.workerSnapshots[w.TaskID] = snap
		}
		// Initialize spinner frame for new workers.
		if _, ok := m.workerSpinners[w.TaskID]; !ok {
			m.workerSpinners[w.TaskID] = 0
		}
	}
}

// isParallelMode returns true when there are active workers (parallel
// orchestrator workers or dispatched workers).
func (m statusModel) isParallelMode() bool {
	return len(m.workerIndex) > 0
}

// advanceWorkerSpinners increments spinner frames for all active workers.
func (m *statusModel) advanceWorkerSpinners() {
	for _, w := range m.workerIndex {
		if w.Status == "working" {
			m.workerSpinners[w.TaskID] = (m.workerSpinners[w.TaskID] + 1) % len(styles.SpinnerFrames)
		}
	}
}

// workerStatusIndicator returns the spinner/icon and style for a worker status.
func workerStatusIndicator(status string, spinnerFrame int) (string, lipgloss.Style) {
	switch status {
	case "done":
		return statusGreenStyle.Render("✓"), statusGreenStyle
	case "failed":
		return statusRedStyle.Render("✗"), statusRedStyle
	case "blocked":
		return statusRedStyle.Render("⚠"), statusRedStyle
	default: // "working"
		frame := styles.SpinnerFrames[spinnerFrame%len(styles.SpinnerFrames)]
		return statusCyanStyle.Render(frame), statusCyanStyle
	}
}
