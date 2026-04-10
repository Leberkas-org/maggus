package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/leberkas-org/maggus/internal/runlog"
	"github.com/leberkas-org/maggus/internal/tui/styles"
)

// refreshWorkerSnapshots reads the workers index and per-worker snapshots.
// Sets m.workerIndex to nil when no parallel workers are active.
func (m *statusModel) refreshWorkerSnapshots() {
	if !m.daemon.Running {
		m.workerIndex = nil
		m.workerSnapshots = nil
		m.workerSpinners = nil
		return
	}
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

// isParallelMode returns true when the daemon is running parallel workers.
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

// renderWorkerPanes renders the parallel worker split pane view.
// Panes are arranged horizontally first; when they exceed the available width,
// they stack vertically.
func (m statusModel) renderWorkerPanes(width, height int) string {
	workers := m.workerIndex
	if len(workers) == 0 {
		return statusDimStyle.Render("  No active workers")
	}

	// Each pane has a rounded border that adds 2 chars horizontally and 2 lines vertically.
	const borderW, borderH = 2, 2
	const minPaneWidth = 28 // minimum content width (without border)

	// Determine layout: how many panes fit horizontally.
	maxCols := width / (minPaneWidth + borderW)
	if maxCols < 1 {
		maxCols = 1
	}
	if maxCols > len(workers) {
		maxCols = len(workers)
	}

	// Compute number of rows needed.
	rows := (len(workers) + maxCols - 1) / maxCols
	// Content dimensions per pane (excluding border).
	paneContentW := width/maxCols - borderW
	if paneContentW < minPaneWidth {
		paneContentW = minPaneWidth
	}
	paneContentH := height/rows - borderH
	if paneContentH < 6 {
		paneContentH = 6
	}

	var rowStrings []string
	for row := 0; row < rows; row++ {
		var panes []string
		startIdx := row * maxCols
		endIdx := startIdx + maxCols
		if endIdx > len(workers) {
			endIdx = len(workers)
		}
		for i := startIdx; i < endIdx; i++ {
			w := workers[i]
			snap := m.workerSnapshots[w.TaskID]
			pane := m.renderSingleWorkerPane(w, snap, paneContentW, paneContentH)
			panes = append(panes, pane)
		}
		rowStrings = append(rowStrings, lipgloss.JoinHorizontal(lipgloss.Top, panes...))
	}

	return lipgloss.JoinVertical(lipgloss.Left, rowStrings...)
}

// renderSingleWorkerPane renders one worker's status pane.
// width and height are the content dimensions (excluding the border).
func (m statusModel) renderSingleWorkerPane(w runlog.WorkerIndexEntry, snap *runlog.StateSnapshot, width, height int) string {
	var sb strings.Builder
	contentW := width - 2 // left+right padding inside border
	if contentW < 10 {
		contentW = 10
	}

	// ── Status indicator + Task ID ──
	indicator, indicatorStyle := workerStatusIndicator(w.Status, m.workerSpinners[w.TaskID])
	taskIDStr := statusCyanStyle.Render(w.TaskID)
	sb.WriteString(fmt.Sprintf(" %s %s\n", indicator, taskIDStr))

	// ── Task title (truncated) ──
	title := w.TaskTitle
	if snap != nil && snap.TaskTitle != "" {
		title = snap.TaskTitle
	}
	if title != "" {
		title = styles.Truncate(title, contentW)
		sb.WriteString(" " + indicatorStyle.Render(title) + "\n")
	} else {
		sb.WriteString("\n")
	}

	sb.WriteString(" " + styles.Separator(contentW) + "\n")

	// ── Current tool being invoked ──
	if snap != nil && len(snap.ToolEntries) > 0 {
		latest := snap.ToolEntries[len(snap.ToolEntries)-1]
		icon := latest.Icon
		if icon == "" {
			icon = "🥚"
		}
		desc := styles.Truncate(latest.Description, contentW-6)
		toolStr := fmt.Sprintf(" %s %s", icon, statusDimStyle.Render(desc))
		sb.WriteString(toolStr + "\n")
	} else {
		sb.WriteString(statusDimStyle.Render(" Waiting...") + "\n")
	}

	// ── Token usage ──
	if snap != nil && (snap.TokenInput > 0 || snap.TokenOutput > 0) {
		tokenStr := fmt.Sprintf("%s in / %s out",
			FormatTokens(snap.TokenInput), FormatTokens(snap.TokenOutput))
		sb.WriteString(fmt.Sprintf(" %s %s\n", statusBoldStyle.Render("T:"), statusDimStyle.Render(tokenStr)))
	} else {
		sb.WriteString(fmt.Sprintf(" %s %s\n", statusBoldStyle.Render("T:"), statusDimStyle.Render("—")))
	}

	// ── Elapsed time ──
	elapsed := "—"
	if snap != nil && snap.TaskStartedAt != "" {
		if t, err := time.Parse(time.RFC3339, snap.TaskStartedAt); err == nil {
			elapsed = formatHumanDuration(time.Since(t))
		}
	} else if w.StartedAt != "" {
		if t, err := time.Parse(time.RFC3339, w.StartedAt); err == nil {
			elapsed = formatHumanDuration(time.Since(t))
		}
	}
	sb.WriteString(fmt.Sprintf(" %s %s", statusBoldStyle.Render("E:"), statusDimStyle.Render(elapsed)))

	// Wrap in a fixed-size box with a border accent.
	content := sb.String()
	borderColor := styles.ThemeColor(m.isNerfed)
	if w.Status == "done" {
		borderColor = styles.Success
	} else if w.Status == "failed" || w.Status == "blocked" {
		borderColor = styles.Error
	}

	paneStyle := lipgloss.NewStyle().
		Width(width).
		Height(height).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(0, 0)

	return paneStyle.Render(content)
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
