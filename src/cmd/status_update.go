package cmd

import (
	"os"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/leberkas-org/maggus/internal/approval"
	"github.com/leberkas-org/maggus/internal/claude2x"
	"github.com/leberkas-org/maggus/internal/discord"
	"github.com/leberkas-org/maggus/internal/parser"
	"github.com/leberkas-org/maggus/internal/runlog"
	"github.com/leberkas-org/maggus/internal/tui/styles"
)

func (m statusModel) Init() tea.Cmd {
	if m.presence != nil {
		_ = m.presence.Update(discord.PresenceState{
			FeatureTitle: "Viewing Status",
			StartTime:    time.Now(),
		})
	}
	return tea.Batch(
		func() tea.Msg {
			return claude2xResultMsg{status: claude2x.FetchStatus()}
		},
		logPollTick(),
		spinnerTick(),
		listenForWatcherUpdate(m.watcherCh),
	)
}

func (m statusModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.HandleResize(msg.Width, msg.Height)
		m.currentTaskViewport.Width = msg.Width
		m.currentTaskViewport.Height = msg.Height
		return m, nil

	case claude2xResultMsg:
		m.is2x = msg.status.Is2x
		m.BorderColor = styles.ThemeColor(m.is2x)
		if m.is2x {
			return m, next2xTick()
		}
		return m, nil
	case claude2xTickMsg:
		is2x, _, tickCmd := fetch2xAndUpdate()
		m.is2x = is2x
		m.BorderColor = styles.ThemeColor(m.is2x)
		return m, tickCmd

	case spinnerTickMsg:
		if m.daemon.Running && m.snapshot != nil {
			m.spinnerFrame = (m.spinnerFrame + 1) % len(statusSpinnerFrames)
			return m, spinnerTick()
		}
		// Keep ticking even when idle so the spinner starts immediately when daemon resumes.
		return m, spinnerTick()

	case logPollTickMsg:
		m.daemon = loadDaemonStatus(m.dir)
		if m.daemon.Running && m.daemon.RunID != "" {
			snap, err := runlog.ReadSnapshot(m.dir, m.daemon.RunID)
			if err == nil {
				m.snapshot = snap
			}
			// else: keep previous snapshot or nil
		} else if !m.daemon.Running {
			m.snapshot = nil
		}
		if m.daemon.LogPath != "" {
			newLines := readLastNLogLines(m.daemon.LogPath, 50)
			m.applyLogLines(newLines)
		} else {
			m.logLines = nil
		}
		return m, logPollTick()

	case featureSummaryUpdateMsg:
		// Preserve selected feature, cursor, and scroll across reload
		visible := m.visiblePlans()
		var selectedFilename string
		if m.planCursor < len(visible) {
			selectedFilename = filepath.Base(visible[m.planCursor].File)
		}
		prevCursor := m.Cursor
		prevScroll := m.ScrollOffset
		m.reloadPlans()
		// Restore selection by filename
		if selectedFilename != "" {
			for i, f := range m.visiblePlans() {
				if filepath.Base(f.File) == selectedFilename {
					m.planCursor = i
					m.Tasks = buildSelectableTasksForFeature(f, m.showAll)
					// Clamp cursor and scroll to new bounds
					if prevCursor < len(m.Tasks) {
						m.Cursor = prevCursor
					} else if len(m.Tasks) > 0 {
						m.Cursor = len(m.Tasks) - 1
					}
					m.ScrollOffset = prevScroll
					break
				}
			}
		}
		return m, listenForWatcherUpdate(m.watcherCh)

	case tea.KeyMsg:
		if m.confirmDeleteFeature {
			return m.updateStatusConfirmDeleteFeature(msg)
		}
		if m.ConfirmDelete {
			return m.updateStatusConfirmDelete(msg)
		}
		if m.ShowDetail {
			return m.updateStatusDetail(msg)
		}
		return m.updateList(msg)
	}

	cmd := m.UpdateViewport(msg)
	return m, cmd
}

// applyLogLines updates the log line buffer and adjusts the scroll position.
// If auto-scroll is active, the view is pinned to the bottom.
func (m *statusModel) applyLogLines(newLines []string) {
	prevLen := len(m.logLines)
	m.logLines = newLines
	if m.logAutoScroll {
		m.logScroll = m.maxLogScroll()
	} else if len(newLines) > prevLen {
		// Preserve relative position as new lines arrive.
		m.logScroll += len(newLines) - prevLen
		if m.logScroll > m.maxLogScroll() {
			m.logScroll = m.maxLogScroll()
		}
	}
	if m.logScroll < 0 {
		m.logScroll = 0
	}
}

// maxLogScroll returns the maximum valid scroll offset for the log panel.
// When a snapshot is available, scrolling operates on tool entries.
func (m *statusModel) maxLogScroll() int {
	visible := m.logVisibleLines()
	count := m.logItemCount()
	max := count - visible
	if max < 0 {
		max = 0
	}
	return max
}

// logItemCount returns the number of scrollable items in the log panel.
func (m *statusModel) logItemCount() int {
	if m.snapshot != nil && m.daemon.Running {
		return len(m.snapshot.ToolEntries)
	}
	return len(m.logLines)
}

// logVisibleLines returns the number of visible lines available for the scrollable
// area in the log panel. In rich mode, this accounts for the fixed header/footer zones.
func (m *statusModel) logVisibleLines() int {
	total := m.visibleTaskLines()
	if m.snapshot != nil && m.daemon.Running {
		// Rich view uses fixed lines: status + output + separator (top) = 3
		// Bottom zone: separator + model + tokens + cost + elapsed = 5
		// Log title + separator = 2, plus scroll indicator = 1
		// Total fixed overhead within the log area = ~11
		overhead := 11
		avail := total - overhead
		if avail < 3 {
			avail = 3
		}
		return avail
	}
	return total
}

func (m statusModel) updateStatusConfirmDelete(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y", "enter":
		t := m.Tasks[m.Cursor]
		if err := parser.DeleteTask(t.SourceFile, t.ID); err != nil {
			m.DeleteErr = err.Error()
			m.ConfirmDelete = false
			return m, nil
		}
		m.reloadPlans()
		if m.Cursor >= len(m.Tasks) && m.Cursor > 0 {
			m.Cursor--
		}
		m.ConfirmDelete = false
		m.ShowDetail = false
		if len(m.Tasks) == 0 {
			return m, tea.Quit
		}
		return m, nil
	case "n", "N", "esc", "ctrl+c":
		m.ConfirmDelete = false
		return m, nil
	}
	return m, nil
}

func (m statusModel) updateStatusConfirmDeleteFeature(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y", "enter":
		visible := m.visiblePlans()
		if m.planCursor >= len(visible) {
			m.confirmDeleteFeature = false
			return m, nil
		}
		f := visible[m.planCursor]
		fullPath := m.featureFilePath(f)
		if err := os.Remove(fullPath); err != nil {
			m.deleteFeatureErr = err.Error()
			m.confirmDeleteFeature = false
			return m, nil
		}
		m.confirmDeleteFeature = false
		m.reloadPlans()
		// Clamp planCursor to valid range
		newVisible := m.visiblePlans()
		if m.planCursor >= len(newVisible) {
			m.planCursor = len(newVisible) - 1
		}
		if m.planCursor < 0 {
			m.planCursor = 0
		}
		m.rebuildForSelectedPlan()
		if len(newVisible) == 0 {
			return m, tea.Quit
		}
		return m, nil
	case "n", "N", "esc", "ctrl+c":
		m.confirmDeleteFeature = false
		return m, nil
	}
	return m, nil
}

func (m statusModel) updateStatusDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Intercept status-specific keys before delegating to component
	if msg.String() == "alt+p" {
		return m.handleApproveToggle()
	}
	cmd, action := m.taskListComponent.Update(msg)
	switch action {
	case taskListQuit, taskListRun:
		return m, tea.Quit
	}
	return m, cmd
}

func (m statusModel) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// When the log panel is active, j/k/up/down scroll the log.
	if m.showLog {
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		case "tab":
			m.showLog = false
			if m.presence != nil {
				_ = m.presence.Update(discord.PresenceState{
					FeatureTitle: "Viewing Status",
					StartTime:    time.Now(),
				})
			}
			return m, nil
		case "j", "down":
			m.logAutoScroll = false
			m.logScroll++
			if m.logScroll > m.maxLogScroll() {
				m.logScroll = m.maxLogScroll()
			}
			return m, nil
		case "k", "up":
			m.logAutoScroll = false
			m.logScroll--
			if m.logScroll < 0 {
				m.logScroll = 0
			}
			return m, nil
		case "G":
			m.logAutoScroll = true
			m.logScroll = m.maxLogScroll()
			return m, nil
		case "g":
			m.logAutoScroll = false
			m.logScroll = 0
			return m, nil
		}
		return m, nil
	}

	// Clear deleteFeatureErr on any key press
	if m.deleteFeatureErr != "" {
		m.deleteFeatureErr = ""
		return m, nil
	}

	// Clear status note on any key except alt+p
	if msg.String() != "alt+p" {
		m.statusNote = ""
	}
	m.syncDetailSuffix()

	switch msg.String() {
	case "tab":
		m.showLog = true
		m.logAutoScroll = true
		m.logScroll = m.maxLogScroll()
		if m.presence != nil {
			details := m.daemon.CurrentTask
			if details == "" {
				details = "No active task"
			}
			_ = m.presence.Update(discord.PresenceState{
				FeatureTitle: details,
				Verb:         "Looking at output",
				StartTime:    time.Now(),
			})
		}
		return m, nil
	case "right":
		visible := m.visiblePlans()
		if len(visible) > 1 {
			m.planCursor = styles.CursorDown(m.planCursor, len(visible))
			m.rebuildForSelectedPlan()
		}
		return m, nil
	case "shift+tab", "left":
		visible := m.visiblePlans()
		if len(visible) > 1 {
			m.planCursor = styles.CursorUp(m.planCursor, len(visible))
			m.rebuildForSelectedPlan()
		}
		return m, nil
	case "alt+a":
		m.showAll = !m.showAll
		plans, a, err := loadPlansWithApprovals(m.dir, true)
		if err == nil {
			m.approvals = a
			m.plans = plans
			pruneStaleApprovals(m.dir, plans)
		}
		m.nextTaskID, m.nextTaskFile = findNextTask(m.plans)
		m.rebuildForSelectedPlan()
		return m, nil
	case "alt+p":
		return m.handleApproveToggle()
	case "alt+d":
		visible := m.visiblePlans()
		if len(visible) > 0 && m.planCursor < len(visible) && !m.ConfirmDelete {
			m.confirmDeleteFeature = true
		}
		return m, nil
	}

	// Delegate to component for shared navigation
	cmd, action := m.taskListComponent.Update(msg)
	switch action {
	case taskListQuit, taskListRun:
		return m, tea.Quit
	case taskListDeleted:
		m.reloadPlans()
	}
	return m, cmd
}

func (m statusModel) handleApproveToggle() (tea.Model, tea.Cmd) {
	m.statusNote = ""
	visible := m.visiblePlans()
	if m.planCursor >= len(visible) {
		return m, nil
	}
	f := visible[m.planCursor]
	if f.Completed {
		m.statusNote = "cannot approve a completed feature"
		return m, nil
	}
	key := f.ApprovalKey()
	approved := isPlanApproved(f, m.approvals, m.approvalRequired)
	var err error
	if approved {
		err = approval.Unapprove(m.dir, key)
		if err == nil {
			m.statusNote = "feature unapproved"
		}
	} else {
		err = approval.Approve(m.dir, key)
		if err == nil {
			m.statusNote = "feature approved"
		}
	}
	if err != nil {
		m.statusNote = "error: " + err.Error()
		return m, nil
	}
	m.reloadPlans()
	return m, nil
}
