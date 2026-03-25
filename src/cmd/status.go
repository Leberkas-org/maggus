package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/leberkas-org/maggus/internal/approval"
	"github.com/leberkas-org/maggus/internal/claude2x"
	"github.com/leberkas-org/maggus/internal/config"
	"github.com/leberkas-org/maggus/internal/filewatcher"
	"github.com/leberkas-org/maggus/internal/parser"
	"github.com/leberkas-org/maggus/internal/runlog"
	"github.com/leberkas-org/maggus/internal/runner"
	"github.com/leberkas-org/maggus/internal/tui/styles"

	"github.com/spf13/cobra"
)

const statusHeaderLines = 11 // title + daemon line + blank + tab bar (~2) + separator + blank + progress + blank + tasks header + separator

var statusSpinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// Lipgloss styles for the status command.
var (
	statusGreenStyle = lipgloss.NewStyle().Foreground(styles.Success)
	statusCyanStyle  = lipgloss.NewStyle().Foreground(styles.Primary)
	statusRedStyle   = lipgloss.NewStyle().Foreground(styles.Error)
	statusDimStyle   = lipgloss.NewStyle().Faint(true)
	statusDimGreen   = lipgloss.NewStyle().Faint(true).Foreground(styles.Success)
	statusBoldStyle  = lipgloss.NewStyle().Bold(true)
	statusBlueStyle  = lipgloss.NewStyle().Foreground(styles.Accent)
)

// statusModel is the bubbletea model for the interactive status TUI.
type statusModel struct {
	taskListComponent

	features     []featureInfo
	showAll      bool
	nextTaskID   string
	nextTaskFile string
	agentName    string

	// Feature tab selection
	selectedFeature int // index into visibleFeatures()

	dir       string             // working directory for file operations
	approvals approval.Approvals // cached approvals; reloaded on reloadFeatures

	approvalRequired bool // from config; used when reloading features

	is2x bool // true when Claude is in 2x mode (border turns yellow)

	// Temporary status note (e.g. "feature approved")
	statusNote string

	// Live log panel
	showLog       bool
	logLines      []string
	logScroll     int
	logAutoScroll bool
	daemon        daemonStatus

	// Rich live view from state.json
	snapshot     *runlog.StateSnapshot // nil when no snapshot available
	spinnerFrame int

	// File watcher for live feature reload
	watcher   *filewatcher.Watcher
	watcherCh <-chan bool
}

func newStatusModel(features []featureInfo, showAll bool, nextTaskID, nextTaskFile, agentName, dir string, showLog bool, approvalRequired bool) statusModel {
	m := statusModel{
		taskListComponent: taskListComponent{
			HeaderLines: statusHeaderLines,
		},
		features:         features,
		showAll:          showAll,
		nextTaskID:       nextTaskID,
		nextTaskFile:     nextTaskFile,
		agentName:        agentName,
		dir:              dir,
		showLog:          showLog,
		approvalRequired: approvalRequired,
		logAutoScroll:    true,
	}
	visible := m.visibleFeatures()
	if len(visible) > 0 {
		m.Tasks = buildSelectableTasksForFeature(visible[0], showAll)
	}
	return m
}

// visibleFeatures returns the features that should be shown based on the showAll flag.
func (m statusModel) visibleFeatures() []featureInfo {
	var visible []featureInfo
	for _, f := range m.features {
		if f.completed && !m.showAll {
			continue
		}
		visible = append(visible, f)
	}
	return visible
}

// rebuildForSelectedFeature rebuilds the selectable tasks and resets the cursor
// for the currently selected feature.
func (m *statusModel) rebuildForSelectedFeature() {
	visible := m.visibleFeatures()
	if m.selectedFeature >= len(visible) {
		m.selectedFeature = 0
	}
	if len(visible) > 0 {
		m.Tasks = buildSelectableTasksForFeature(visible[m.selectedFeature], m.showAll)
	} else {
		m.Tasks = nil
	}
	m.Cursor = 0
	m.ScrollOffset = 0
}

// reloadFeatures reloads all features, bugs, and approvals from disk and rebuilds the current view.
func (m *statusModel) reloadFeatures() {
	a, err := approval.Load(m.dir)
	if err == nil {
		m.approvals = a
	}
	features, err := parseFeatures(m.dir, m.approvalRequired)
	if err != nil {
		m.rebuildForSelectedFeature()
		return
	}
	bugs, err := parseBugs(m.dir, m.approvalRequired)
	if err == nil {
		features = append(features, bugs...)
	}
	m.features = features
	pruneStaleApprovals(m.dir, features)
	m.nextTaskID, m.nextTaskFile = findNextTask(features)
	m.rebuildForSelectedFeature()
}

// syncDetailSuffix updates the component's DetailSuffix from statusNote.
func (m *statusModel) syncDetailSuffix() {
	if m.statusNote != "" {
		mutedStyle := lipgloss.NewStyle().Foreground(styles.Muted)
		m.DetailSuffix = "\n" + mutedStyle.Render("  "+m.statusNote)
	} else {
		m.DetailSuffix = ""
	}
}

func (m statusModel) Init() tea.Cmd {
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
		m.HandleResize(msg.Width, msg.Height)
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
		visible := m.visibleFeatures()
		var selectedFilename string
		if m.selectedFeature < len(visible) {
			selectedFilename = visible[m.selectedFeature].filename
		}
		prevCursor := m.Cursor
		prevScroll := m.ScrollOffset
		m.reloadFeatures()
		// Restore selection by filename
		if selectedFilename != "" {
			for i, f := range m.visibleFeatures() {
				if f.filename == selectedFilename {
					m.selectedFeature = i
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
		m.reloadFeatures()
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
		return m, nil
	case "right":
		visible := m.visibleFeatures()
		if len(visible) > 1 {
			m.selectedFeature = (m.selectedFeature + 1) % len(visible)
			m.rebuildForSelectedFeature()
		}
		return m, nil
	case "shift+tab", "left":
		visible := m.visibleFeatures()
		if len(visible) > 1 {
			m.selectedFeature--
			if m.selectedFeature < 0 {
				m.selectedFeature = len(visible) - 1
			}
			m.rebuildForSelectedFeature()
		}
		return m, nil
	case "alt+a":
		m.showAll = !m.showAll
		features, err := parseFeatures(m.dir, m.approvalRequired)
		if err == nil {
			bugs, bugErr := parseBugs(m.dir, m.approvalRequired)
			if bugErr == nil {
				features = append(features, bugs...)
			}
			m.features = features
			pruneStaleApprovals(m.dir, features)
		}
		m.nextTaskID, m.nextTaskFile = findNextTask(m.features)
		m.rebuildForSelectedFeature()
		return m, nil
	case "alt+p":
		return m.handleApproveToggle()
	}

	// Delegate to component for shared navigation
	cmd, action := m.taskListComponent.Update(msg)
	switch action {
	case taskListQuit, taskListRun:
		return m, tea.Quit
	case taskListDeleted:
		m.reloadFeatures()
	}
	return m, cmd
}

func (m statusModel) handleApproveToggle() (tea.Model, tea.Cmd) {
	m.statusNote = ""
	visible := m.visibleFeatures()
	if m.selectedFeature >= len(visible) {
		return m, nil
	}
	f := visible[m.selectedFeature]
	if f.completed {
		m.statusNote = "cannot approve a completed feature"
		return m, nil
	}
	key := f.approvalKey()
	var err error
	if f.approved {
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
	m.reloadFeatures()
	return m, nil
}

func (m statusModel) View() string {
	if len(m.features) == 0 {
		return m.viewEmpty()
	}
	if v := m.taskListComponent.View(); v != "" {
		return v
	}
	if m.showLog {
		return m.viewLog()
	}
	return m.viewStatus()
}

func (m statusModel) viewEmpty() string {
	mutedStyle := lipgloss.NewStyle().Foreground(styles.Muted)

	var sb strings.Builder
	sb.WriteString(styles.Title.Render("Status") + "\n\n")
	sb.WriteString(mutedStyle.Render("No features found.") + "\n\n")
	sb.WriteString(mutedStyle.Render("Create a feature with ") +
		lipgloss.NewStyle().Bold(true).Foreground(styles.Primary).Render("maggus plan") +
		mutedStyle.Render(" to get started.") + "\n")

	footer := styles.StatusBar.Render("q/esc: exit")

	borderColor := styles.ThemeColor(m.is2x)
	if m.Width > 0 && m.Height > 0 {
		return styles.FullScreenColor(sb.String(), footer, m.Width, m.Height, borderColor)
	}
	return styles.Box.BorderForeground(borderColor).Render(sb.String()) + "\n"
}

// renderTabBar renders the horizontal feature tab bar.
func (m statusModel) renderTabBar() string {
	visible := m.visibleFeatures()
	if len(visible) == 0 {
		return ""
	}

	selectedStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.Primary)
	selectedBugStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.Error)
	unselectedStyle := lipgloss.NewStyle().Foreground(styles.Muted)
	unselectedBugStyle := lipgloss.NewStyle().Foreground(styles.Muted).Faint(true)
	unapprovedTabStyle := lipgloss.NewStyle().Foreground(styles.Warning).Faint(true)

	var tabs []string
	needsSep := false
	for i, p := range visible {
		// Insert separator between features and bugs
		if p.isBug && !needsSep {
			needsSep = true
			if len(tabs) > 0 {
				tabs = append(tabs, statusDimStyle.Render(" ┃ "))
			}
		}

		done := p.doneCount()
		total := len(p.tasks)
		name := strings.TrimSuffix(p.filename, ".md")
		approvalMark := "✓"
		if !p.approved {
			approvalMark = "✗"
		}
		label := fmt.Sprintf(" %s %s %d/%d ", approvalMark, name, done, total)
		if i == m.selectedFeature {
			if !p.approved {
				tabs = append(tabs, unapprovedTabStyle.Bold(true).Render(label))
			} else if p.isBug {
				tabs = append(tabs, selectedBugStyle.Render(label))
			} else {
				tabs = append(tabs, selectedStyle.Render(label))
			}
		} else {
			if !p.approved {
				tabs = append(tabs, unapprovedTabStyle.Render(label))
			} else if p.isBug {
				tabs = append(tabs, unselectedBugStyle.Render(label))
			} else {
				tabs = append(tabs, unselectedStyle.Render(label))
			}
		}
	}

	// Join tabs with a separator, wrapping to next line if needed
	sep := statusDimStyle.Render("│")
	maxWidth := m.Width - 8
	if maxWidth <= 0 {
		maxWidth = 80
	}

	var lines []string
	var currentLine string
	currentVisualWidth := 0
	for _, tab := range tabs {
		tabWidth := lipgloss.Width(tab)
		sepWidth := 0
		if currentLine != "" {
			sepWidth = 1
		}
		if currentVisualWidth+sepWidth+tabWidth > maxWidth && currentLine != "" {
			lines = append(lines, currentLine)
			currentLine = tab
			currentVisualWidth = tabWidth
		} else {
			if currentLine != "" {
				currentLine += sep
				currentVisualWidth += sepWidth
			}
			currentLine += tab
			currentVisualWidth += tabWidth
		}
	}
	if currentLine != "" {
		lines = append(lines, currentLine)
	}

	return " " + strings.Join(lines, "\n ")
}

// renderDaemonStatusLine returns a one-line string showing daemon state and
// current feature/task progress (for use in the status header).
func (m statusModel) renderDaemonStatusLine() string {
	if m.daemon.Running {
		indicator := statusCyanStyle.Render("●")
		line := fmt.Sprintf(" %s daemon running (PID %d)", indicator, m.daemon.PID)
		if m.daemon.CurrentFeature != "" {
			line += statusDimStyle.Render(" · " + m.daemon.CurrentFeature)
		}
		if m.daemon.CurrentTask != "" {
			line += statusDimStyle.Render(" · " + m.daemon.CurrentTask)
		}
		return line
	}
	if m.daemon.RunID != "" {
		return statusDimStyle.Render(fmt.Sprintf(" ○ daemon not running · last run: %s", m.daemon.RunID))
	}
	return statusDimStyle.Render(" ○ daemon not running")
}

// viewLog renders the live log panel, replacing the task list content area.
// When the daemon is running and a state.json snapshot exists, it renders a rich
// TUI with spinner, tool list, and token stats. Otherwise it falls back to the
// plain JSONL log reader.
func (m statusModel) viewLog() string {
	var sb strings.Builder

	// Re-use the same header structure as viewStatus (title + daemon line + tabs + progress).
	visible := m.visibleFeatures()

	totalTasks := 0
	totalDone := 0
	totalBlocked := 0
	activeFeatures := 0
	totalBugs := 0
	activeBugs := 0
	for _, f := range m.features {
		totalTasks += len(f.tasks)
		totalDone += f.doneCount()
		totalBlocked += f.blockedCount()
		if f.isBug {
			totalBugs++
			if !f.completed {
				activeBugs++
			}
		} else {
			if !f.completed {
				activeFeatures++
			}
		}
	}
	featureCount := len(m.features) - totalBugs

	headerParts := fmt.Sprintf("%d features (%d active)", featureCount, activeFeatures)
	if totalBugs > 0 {
		headerParts += fmt.Sprintf(", %d bugs (%d active)", totalBugs, activeBugs)
	}
	header := styles.Title.Render(fmt.Sprintf("Maggus Status — %s, %d tasks total", headerParts, totalTasks))
	sb.WriteString(header)
	sb.WriteString("\n")
	sb.WriteString(m.renderDaemonStatusLine())
	sb.WriteString("\n")

	if len(visible) > 0 {
		sb.WriteString(m.renderTabBar())
		sb.WriteString("\n")
		sb.WriteString(" " + styles.Separator(42))
		sb.WriteString("\n")
	}

	// Decide: rich snapshot view or plain log fallback
	if m.snapshot != nil && m.daemon.Running {
		sb.WriteString(m.renderSnapshotPanel())
	} else {
		sb.WriteString(m.renderPlainLogPanel())
	}

	footer := styles.StatusBar.Render("j/↓: scroll down · k/↑: scroll up · g: top · G: bottom (auto) · tab: features · q/esc: exit")
	borderColor := styles.ThemeColor(m.is2x)
	if m.Width > 0 && m.Height > 0 {
		return styles.FullScreenColor(sb.String(), footer, m.Width, m.Height, borderColor)
	}
	return styles.Box.BorderForeground(borderColor).Render(sb.String()+"\n\n"+footer) + "\n"
}

// renderPlainLogPanel renders the original plain JSONL log view.
func (m statusModel) renderPlainLogPanel() string {
	var sb strings.Builder

	sb.WriteString("\n")
	logTitle := styles.Title.Render(" Live Log")
	if m.daemon.LogPath != "" {
		logTitle += statusDimStyle.Render("  " + m.daemon.RunID + "/run.log")
	}
	sb.WriteString(logTitle)
	sb.WriteString("\n")
	sb.WriteString(" " + styles.Separator(42))

	visibleLines := m.visibleTaskLines()

	if len(m.logLines) == 0 {
		sb.WriteString("\n")
		sb.WriteString(statusDimStyle.Render("  No active run"))
	} else {
		end := min(m.logScroll+visibleLines, len(m.logLines))
		start := max(m.logScroll, 0)
		for _, line := range m.logLines[start:end] {
			sb.WriteString("\n ")
			sb.WriteString(formatLogLine(line))
		}
		if len(m.logLines) > visibleLines {
			scrollHint := fmt.Sprintf(" [%d-%d of %d]", start+1, end, len(m.logLines))
			if m.logAutoScroll {
				scrollHint += " (auto)"
			}
			sb.WriteString("\n")
			sb.WriteString(statusDimStyle.Render(scrollHint))
		}
	}

	return sb.String()
}

// renderSnapshotPanel renders the rich live TUI from a state.json snapshot,
// matching the layout of renderProgressTab in the work TUI.
func (m statusModel) renderSnapshotPanel() string {
	snap := m.snapshot
	var sb strings.Builder

	contentWidth := m.Width - 11
	if contentWidth < 20 {
		contentWidth = 20
	}

	// ── Top zone (fixed): spinner + status, task ID + title ──
	sb.WriteString("\n")
	spinnerStr := statusCyanStyle.Render(statusSpinnerFrames[m.spinnerFrame])
	sColor := lipgloss.NewStyle().Foreground(styles.Warning)
	switch snap.Status {
	case "Done":
		sColor = statusGreenStyle
		spinnerStr = statusGreenStyle.Render("✓")
	case "Failed":
		sColor = statusRedStyle
		spinnerStr = statusRedStyle.Render("✗")
	case "Interrupted":
		sColor = statusRedStyle
		spinnerStr = statusRedStyle.Render("⊘")
	}
	sb.WriteString(fmt.Sprintf(" %s %s  %s\n", spinnerStr, statusBoldStyle.Render("Status:"), sColor.Render(snap.Status)))

	if snap.TaskID != "" {
		sb.WriteString(fmt.Sprintf("   %s  %s: %s\n", statusBoldStyle.Render("Task:"), statusCyanStyle.Render(snap.TaskID), snap.TaskTitle))
	}
	sb.WriteString(" " + styles.Separator(42) + "\n")

	// ── Middle zone (scrollable tool list) ──
	available := m.logVisibleLines()
	totalTools := len(snap.ToolEntries)

	if totalTools == 0 {
		sb.WriteString(statusDimStyle.Render("  No tool invocations yet.") + "\n")
		for i := 1; i < available; i++ {
			sb.WriteString("\n")
		}
	} else {
		toolLines := make([]string, totalTools)
		for i, entry := range snap.ToolEntries {
			ts := entry.Timestamp
			if t, err := time.Parse(time.RFC3339, entry.Timestamp); err == nil {
				ts = t.Local().Format("15:04:05")
			}
			icon := entry.Icon
			if icon == "" {
				icon = "▶️"
			}
			desc := styles.Truncate(entry.Description, contentWidth-2)
			toolLines[i] = fmt.Sprintf("  %s %s: %s  %s",
				icon,
				statusCyanStyle.Render(entry.Type),
				statusBlueStyle.Render(desc),
				statusDimStyle.Render(ts))
		}

		offset := m.logScroll
		maxOffset := totalTools - available
		if maxOffset < 0 {
			maxOffset = 0
		}
		if offset > maxOffset {
			offset = maxOffset
		}
		if offset < 0 {
			offset = 0
		}

		if totalTools > available {
			end := offset + available
			if end > totalTools {
				end = totalTools
			}
			indicator := statusDimStyle.Render(fmt.Sprintf("[%d-%d of %d]", offset+1, end, totalTools))
			if m.logAutoScroll {
				indicator += statusDimStyle.Render(" (auto)")
			}
			sb.WriteString(indicator + "\n")
			viewH := available - 1
			if viewH < 1 {
				viewH = 1
			}
			end = offset + viewH
			if end > totalTools {
				end = totalTools
			}
			for _, line := range toolLines[offset:end] {
				sb.WriteString(line + "\n")
			}
			rendered := end - offset
			for i := rendered; i < viewH; i++ {
				sb.WriteString("\n")
			}
		} else {
			for _, line := range toolLines {
				sb.WriteString(line + "\n")
			}
			for i := totalTools; i < available; i++ {
				sb.WriteString("\n")
			}
		}
	}

	// ── Bottom zone (fixed): model, tokens, cost, elapsed ──
	sb.WriteString(" " + styles.Separator(42) + "\n")

	// Tokens
	totalIn := snap.TokenInput
	if totalIn > 0 || snap.TokenOutput > 0 {
		tokenStr := fmt.Sprintf("%s in / %s out",
			runner.FormatTokens(totalIn), runner.FormatTokens(snap.TokenOutput))
		sb.WriteString(fmt.Sprintf("  %s  %s\n", statusBoldStyle.Render("Tokens:"), statusDimStyle.Render(tokenStr)))

		// Per-model breakdown (one line per model)
		if len(snap.ModelBreakdown) > 0 {
			sb.WriteString(m.formatSnapshotModelTokens())
		}

		costStr := "N/A"
		if snap.TokenCost > 0 {
			costStr = runner.FormatCost(snap.TokenCost)
		}
		sb.WriteString(fmt.Sprintf("  %s    %s\n", statusBoldStyle.Render("Cost:"), statusDimStyle.Render(costStr)))
	} else {
		sb.WriteString(fmt.Sprintf("  %s  %s\n", statusBoldStyle.Render("Tokens:"), statusDimStyle.Render("N/A")))
		sb.WriteString(fmt.Sprintf("  %s    %s\n", statusBoldStyle.Render("Cost:"), statusDimStyle.Render("N/A")))
	}

	// Run and task elapsed times
	runElapsed := "—"
	if snap.RunStartedAt != "" {
		if t, err := time.Parse(time.RFC3339, snap.RunStartedAt); err == nil {
			runElapsed = formatHumanDuration(time.Since(t))
		}
	}
	sb.WriteString(fmt.Sprintf("  %s     %s\n", statusBoldStyle.Render("Run:"), statusDimStyle.Render(runElapsed)))

	taskElapsed := "—"
	if snap.TaskStartedAt != "" {
		if t, err := time.Parse(time.RFC3339, snap.TaskStartedAt); err == nil {
			taskElapsed = formatHumanDuration(time.Since(t))
		}
	}
	sb.WriteString(fmt.Sprintf("  %s    %s\n", statusBoldStyle.Render("Task:"), statusDimStyle.Render(taskElapsed)))

	return sb.String()
}

// formatHumanDuration formats a duration as human-friendly text (e.g. "5m 32s", "1h 12m 5s").
func formatHumanDuration(d time.Duration) string {
	d = d.Truncate(time.Second)
	if d < time.Second {
		return "0s"
	}

	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60

	switch {
	case h > 0:
		return fmt.Sprintf("%dh %dm %ds", h, m, s)
	case m > 0:
		return fmt.Sprintf("%dm %ds", m, s)
	default:
		return fmt.Sprintf("%ds", s)
	}
}

// formatSnapshotModelTokens formats per-model token breakdown from the snapshot.
// Returns multi-line output with one line per model, indented to align under "Tokens:".
func (m statusModel) formatSnapshotModelTokens() string {
	if m.snapshot == nil || len(m.snapshot.ModelBreakdown) == 0 {
		return ""
	}
	// Sort model names for stable output
	names := make([]string, 0, len(m.snapshot.ModelBreakdown))
	for name := range m.snapshot.ModelBreakdown {
		names = append(names, name)
	}
	sort.Strings(names)

	var sb strings.Builder
	for _, name := range names {
		usage := m.snapshot.ModelBreakdown[name]
		totalIn := usage.InputTokens + usage.CacheCreationInputTokens + usage.CacheReadInputTokens
		costStr := runner.FormatCost(usage.CostUSD)
		line := fmt.Sprintf("  %s: %s in / %s out (%s)",
			name, runner.FormatTokens(totalIn), runner.FormatTokens(usage.OutputTokens), costStr)
		sb.WriteString("  " + statusDimStyle.Render(line) + "\n")
	}
	return sb.String()
}

func (m statusModel) viewStatus() string {
	var sb strings.Builder

	visible := m.visibleFeatures()

	// Compute totals
	totalTasks := 0
	totalDone := 0
	totalBlocked := 0
	activeFeatures := 0
	totalBugs := 0
	activeBugs := 0
	for _, f := range m.features {
		totalTasks += len(f.tasks)
		totalDone += f.doneCount()
		totalBlocked += f.blockedCount()
		if f.isBug {
			totalBugs++
			if !f.completed {
				activeBugs++
			}
		} else {
			if !f.completed {
				activeFeatures++
			}
		}
	}
	totalPending := totalTasks - totalDone - totalBlocked
	featureCount := len(m.features) - totalBugs

	// Header
	headerParts := fmt.Sprintf("%d features (%d active)", featureCount, activeFeatures)
	if totalBugs > 0 {
		headerParts += fmt.Sprintf(", %d bugs (%d active)", totalBugs, activeBugs)
	}
	header := styles.Title.Render(fmt.Sprintf("Maggus Status — %s, %d tasks total",
		headerParts, totalTasks))
	sb.WriteString(header)
	sb.WriteString("\n")
	sb.WriteString(m.renderDaemonStatusLine())
	sb.WriteString("\n")

	// Tab bar
	if len(visible) > 0 {
		sb.WriteString(m.renderTabBar())
		sb.WriteString("\n")
		sb.WriteString(" " + styles.Separator(42))
		sb.WriteString("\n")
	}

	// Progress bar and summary for selected feature
	if m.selectedFeature < len(visible) {
		p := visible[m.selectedFeature]
		done := p.doneCount()
		total := len(p.tasks)
		blocked := p.blockedCount()
		pending := total - done - blocked
		sb.WriteString("\n " + buildProgressBar(done, total))
		summary := fmt.Sprintf("  %d/%d tasks · %d pending · %d blocked",
			done, total, pending, blocked)
		sb.WriteString(statusDimStyle.Render(summary))
	} else {
		sb.WriteString("\n " + buildProgressBar(totalDone, totalTasks))
		summary := fmt.Sprintf("  %d/%d tasks · %d pending · %d blocked",
			totalDone, totalTasks, totalPending, totalBlocked)
		sb.WriteString(statusDimStyle.Render(summary))
	}

	// Task list for selected feature
	if m.selectedFeature < len(visible) {
		p := visible[m.selectedFeature]

		sb.WriteString("\n\n")
		if p.completed {
			sb.WriteString(statusDimGreen.Render(fmt.Sprintf(" Tasks — %s (archived)", p.filename)))
		} else {
			fmt.Fprintf(&sb, " Tasks — %s", p.filename)
		}
		sb.WriteString("\n")
		sb.WriteString(" " + styles.Separator(42))

		// Determine visible window for scrolling
		visibleLines := m.visibleTaskLines()
		end := min(m.ScrollOffset+visibleLines, len(m.Tasks))

		for taskIdx := m.ScrollOffset; taskIdx < end; taskIdx++ {
			t := m.Tasks[taskIdx]

			var icon string
			var style lipgloss.Style

			if t.IsComplete() {
				icon = "✓"
				if p.completed {
					style = statusDimGreen
				} else {
					style = statusGreenStyle
				}
			} else if t.IsBlocked() {
				icon = "⚠"
				style = statusRedStyle
			} else if t.ID == m.nextTaskID && t.SourceFile == m.nextTaskFile {
				icon = "→"
				style = statusCyanStyle
			} else {
				icon = "○"
				style = lipgloss.NewStyle().Foreground(styles.Muted)
			}

			if p.completed {
				style = statusDimStyle
			}

			// Cursor indicator
			var prefix string
			if taskIdx == m.Cursor {
				prefix = " ▸ "
				if !p.completed {
					style = lipgloss.NewStyle().Bold(true).Foreground(styles.Primary)
				}
			} else {
				prefix = "   "
			}

			line := fmt.Sprintf("%s%s  %s: %s", prefix, icon, t.ID, t.Title)
			sb.WriteString("\n")
			sb.WriteString(style.Render(line))

			if t.IsBlocked() && !p.completed {
				for _, c := range t.Criteria {
					if !c.Blocked {
						continue
					}
					reason := strings.TrimPrefix(c.Text, "⚠️ BLOCKED: ")
					reason = strings.TrimPrefix(reason, "BLOCKED: ")
					blockedLine := fmt.Sprintf("         BLOCKED: %s", reason)
					sb.WriteString("\n")
					sb.WriteString(statusRedStyle.Render(blockedLine))
				}
			}
		}

		// Scroll indicator
		if len(m.Tasks) > visibleLines {
			scrollHint := fmt.Sprintf(" [%d-%d of %d]", m.ScrollOffset+1, end, len(m.Tasks))
			sb.WriteString("\n")
			sb.WriteString(statusDimStyle.Render(scrollHint))
		}
	}

	// Status note (e.g. "feature approved")
	if m.statusNote != "" {
		sb.WriteString("\n")
		sb.WriteString(statusDimStyle.Render("  " + m.statusNote))
	}

	toggleHint := "alt+a: show all"
	if m.showAll {
		toggleHint = "alt+a: hide completed"
	}
	footer := styles.StatusBar.Render("←/→: switch feature · ↑/↓: navigate · enter: details · " + toggleHint + " · alt+p: approve/unapprove feature · alt+r: run · alt+bksp: delete · tab: live log · q/esc: exit")

	borderColor := styles.ThemeColor(m.is2x)
	if m.Width > 0 && m.Height > 0 {
		return styles.FullScreenColor(sb.String(), footer, m.Width, m.Height, borderColor)
	}
	return styles.Box.BorderForeground(borderColor).Render(sb.String()+"\n\n"+footer) + "\n"
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show a compact summary of feature progress",
	Long:  `Reads all feature files in .maggus/ and displays a compact progress summary.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		plain, err := cmd.Flags().GetBool("plain")
		if err != nil {
			return err
		}
		all, err := cmd.Flags().GetBool("all")
		if err != nil {
			return err
		}
		showLog, err := cmd.Flags().GetBool("show-log")
		if err != nil {
			return err
		}

		dir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}

		cfg, err := config.Load(dir)
		if err != nil {
			return err
		}
		agentName := cfg.Agent
		approvalRequired := cfg.IsApprovalRequired()

		features, err := parseFeatures(dir, approvalRequired)
		if err != nil {
			return err
		}
		bugs, bugErr := parseBugs(dir, approvalRequired)
		if bugErr != nil {
			return bugErr
		}
		features = append(features, bugs...)
		pruneStaleApprovals(dir, features)

		if len(features) == 0 {
			if plain {
				fmt.Fprintln(cmd.OutOrStdout(), "No features found.")
				return nil
			}
			// TUI mode: show empty status view
			features = []featureInfo{}
		}

		nextTaskID, nextTaskFile := findNextTask(features)

		if plain {
			var sb strings.Builder
			renderStatusPlain(&sb, features, all, nextTaskID, nextTaskFile, agentName)
			fmt.Fprint(cmd.OutOrStdout(), sb.String())
			return nil
		}

		// TUI mode: interactive status with detail view
		watcherCh := make(chan bool, 1)
		w, _ := filewatcher.New(dir, func(msg any) {
			hasNew := false
			if u, ok := msg.(filewatcher.UpdateMsg); ok {
				hasNew = u.HasNewFile
			}
			select {
			case watcherCh <- hasNew:
			default: // don't block if channel already has a pending update
			}
		}, 300*time.Millisecond)

		m := newStatusModel(features, all, nextTaskID, nextTaskFile, agentName, dir, showLog, approvalRequired)
		m.watcherCh = watcherCh
		m.watcher = w
		prog := tea.NewProgram(m, tea.WithAltScreen())
		result, err := prog.Run()
		if w != nil {
			w.Close()
		}
		if err != nil {
			return err
		}
		if final, ok := result.(statusModel); ok && final.RunTaskID != "" {
			return dispatchWork(final.RunTaskID)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
	statusCmd.Flags().Bool("plain", false, "Strip colors and use ASCII characters for scripting/piping")
	statusCmd.Flags().Bool("all", false, "Show completed features in task sections and Features table")
	statusCmd.Flags().Bool("show-log", false, "Open the live log panel immediately on startup")
}
