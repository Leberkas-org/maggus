package cmd

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/leberkas-org/maggus/internal/parser"
	"github.com/leberkas-org/maggus/internal/runner"
	"github.com/leberkas-org/maggus/internal/tui/styles"
)

var rightPaneTabNames = []string{"Output", "Feature Details", "Current Task", "Metrics"}

// renderRightPaneTabBar renders the tab bar at the top of the right pane.
// Format: `2 Output  3 Feature Details  4 Current Task  5 Metrics`
// The active tab has bold text and underline in primary color; inactive tabs are muted.
// Number prefixes are always dimmed.
func (m statusModel) renderRightPaneTabBar() string {
	dimStyle := lipgloss.NewStyle().Faint(true)
	activeStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.Primary).Underline(true)
	inactiveStyle := lipgloss.NewStyle().Foreground(styles.Muted)

	var parts []string
	for i, name := range rightPaneTabNames {
		numStr := dimStyle.Render(fmt.Sprintf("%d", i+2))
		var nameStr string
		if m.leftFocused {
			nameStr = inactiveStyle.Render(name)
		} else if i == m.activeTab {
			nameStr = activeStyle.Render(name)
		} else {
			nameStr = inactiveStyle.Render(name)
		}
		parts = append(parts, numStr+" "+nameStr)
	}
	return " " + strings.Join(parts, "  ")
}

// renderRightPane renders the right pane with tab bar, separator, and tab content.
func (m statusModel) renderRightPane(width, height int) string {
	if width <= 0 {
		return lipgloss.NewStyle().Height(height).Render("")
	}

	tabBar := m.renderRightPaneTabBar()
	sep := " " + styles.Separator(min(42, width-2))

	// Content height: total height minus tab bar line and separator line.
	contentH := height - 2
	if contentH < 1 {
		contentH = 1
	}

	var content string
	switch m.activeTab {
	case 0:
		content = m.renderOutputTab(width, contentH)
	case 1:
		content = m.renderFeatureDetailsTab(width, contentH)
	case 2:
		content = m.renderCurrentTaskTab(width, contentH)
	case 3:
		content = m.renderMetricsTab(width, contentH)
	}

	full := tabBar + "\n" + sep + "\n" + content
	rendered := lipgloss.NewStyle().Width(width).Height(height).Render(full)
	borderStyle := lipgloss.NewStyle().Foreground(styles.ThemeColor(m.is2x))
	borderLine := strings.Repeat(borderStyle.Render("─"), width)
	return rendered + "\n" + borderLine
}

// rightPaneContentHeight returns the content height available for tab content in the right pane
// (innerH minus the tab bar line and separator line).
func (m *statusModel) rightPaneContentHeight() int {
	if m.width == 0 || m.height == 0 {
		return 20
	}
	_, innerH := styles.FullScreenInnerSize(m.width, m.height)
	h := innerH - 3
	if h < 1 {
		h = 1
	}
	return h
}

// outputTabScrollableLines returns the number of scrollable lines available
// in the Output tab's middle zone (tool list or log lines).
func (m *statusModel) outputTabScrollableLines() int {
	contentH := m.rightPaneContentHeight()
	if m.snapshot != nil && m.daemon.Running {
		// Fixed lines consumed by the rich snapshot view:
		//   top:    blank(1) + status(1) + task(1) + separator(1) = 4
		//   bottom: separator(1) + tokens(1) + cost(1) + run(1) + task(1) = 5
		overhead := 9
		avail := contentH - overhead
		if avail < 3 {
			avail = 3
		}
		return avail
	}
	// Plain log: blank(1) + title(1) + separator(1) = 3 overhead
	overhead := 3
	avail := contentH - overhead
	if avail < 1 {
		avail = 1
	}
	return avail
}

// renderOutputTab renders the Output tab content: rich snapshot view when the
// daemon is running and a snapshot is available, plain log fallback otherwise.
func (m statusModel) renderOutputTab(width, contentH int) string {
	if m.snapshot != nil && m.daemon.Running {
		return m.renderSnapshotInPane(width, contentH)
	}
	return m.renderPlainLogInPane(width, contentH)
}

// renderPlainLogInPane renders the plain JSONL log view sized for the right pane.
func (m statusModel) renderPlainLogInPane(width, height int) string {
	var sb strings.Builder

	sb.WriteString("\n")
	logTitle := styles.Title.Render(" Live Log")
	if m.daemon.LogPath != "" {
		logTitle += statusDimStyle.Render("  " + m.daemon.RunID + "/run.log")
	}
	sb.WriteString(logTitle + "\n")
	sb.WriteString(" " + styles.Separator(min(42, width-2)))

	// Lines available for scrollable log content (height minus blank+title+separator).
	available := height - 3
	if available < 1 {
		available = 1
	}

	if len(m.logLines) == 0 {
		sb.WriteString("\n")
		sb.WriteString(statusDimStyle.Render("  No active run"))
	} else {
		end := min(m.logScroll+available, len(m.logLines))
		start := max(m.logScroll, 0)
		for _, line := range m.logLines[start:end] {
			sb.WriteString("\n ")
			sb.WriteString(formatLogLine(line))
		}
		if len(m.logLines) > available {
			scrollHint := fmt.Sprintf(" [%d-%d of %d]", start+1, end, len(m.logLines))
			if m.logAutoScroll {
				scrollHint += " (auto)"
			}
			sb.WriteString("\n")
			sb.WriteString(statusDimStyle.Render(scrollHint))
		}
	}

	return lipgloss.NewStyle().Width(width).Height(height).Render(sb.String())
}

// renderSnapshotInPane renders the rich live TUI from a state.json snapshot,
// sized for the right pane content area.
func (m statusModel) renderSnapshotInPane(width, height int) string {
	snap := m.snapshot
	var sb strings.Builder

	contentWidth := width - 4
	if contentWidth < 20 {
		contentWidth = 20
	}

	// ── Top zone (fixed): blank + spinner/status + task ID/title + separator ──
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
	sb.WriteString(" " + styles.Separator(min(42, width-2)) + "\n")

	// ── Middle zone (scrollable tool list) ──
	// Fixed overhead consumed by top(4) + bottom(5) zones = 9 lines.
	available := height - 9
	if available < 3 {
		available = 3
	}

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

	// ── Bottom zone (fixed): separator + tokens + cost + run + task elapsed ──
	sb.WriteString(" " + styles.Separator(min(42, width-2)) + "\n")

	totalIn := snap.TokenInput
	if totalIn > 0 || snap.TokenOutput > 0 {
		tokenStr := fmt.Sprintf("%s in / %s out",
			runner.FormatTokens(totalIn), runner.FormatTokens(snap.TokenOutput))
		sb.WriteString(fmt.Sprintf("  %s  %s\n", statusBoldStyle.Render("Tokens:"), statusDimStyle.Render(tokenStr)))
		costStr := "N/A"
		if snap.TokenCost > 0 {
			costStr = runner.FormatCost(snap.TokenCost)
		}
		sb.WriteString(fmt.Sprintf("  %s    %s\n", statusBoldStyle.Render("Cost:"), statusDimStyle.Render(costStr)))
	} else {
		sb.WriteString(fmt.Sprintf("  %s  %s\n", statusBoldStyle.Render("Tokens:"), statusDimStyle.Render("N/A")))
		sb.WriteString(fmt.Sprintf("  %s    %s\n", statusBoldStyle.Render("Cost:"), statusDimStyle.Render("N/A")))
	}

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

	return lipgloss.NewStyle().Width(width).Height(height).Render(sb.String())
}

// renderCurrentTaskContent returns the rendered detail content for the next workable task.
// Returns an empty string when nextTaskID is empty or the task cannot be found.
func renderCurrentTaskContent(nextTaskID, nextTaskFile string) string {
	if nextTaskID == "" {
		return ""
	}
	t := reloadTask(nextTaskFile, nextTaskID)
	if t == nil {
		return ""
	}
	return renderDetailContent(*t, nil)
}

// loadCurrentTaskDetail loads the next workable task into the currentTaskViewport.
func (m *statusModel) loadCurrentTaskDetail() {
	content := renderCurrentTaskContent(m.nextTaskID, m.nextTaskFile)
	m.currentTaskViewport.SetContent(content)
}

// renderCurrentTaskTab renders Tab 3: a read-only detail view of the next workable task.
// When no task is pending, shows a centered "No pending tasks" message.
func (m statusModel) renderCurrentTaskTab(width, height int) string {
	if m.nextTaskID == "" {
		mutedStyle := lipgloss.NewStyle().Foreground(styles.Muted)
		msg := mutedStyle.Render("No pending tasks")
		return lipgloss.NewStyle().Width(width).Height(height).
			Align(lipgloss.Center, lipgloss.Center).Render(msg)
	}
	return lipgloss.NewStyle().Width(width).Height(height).Render(m.currentTaskViewport.View())
}

// resizeCurrentTaskViewport resizes the currentTaskViewport to the right pane content area.
func (m *statusModel) resizeCurrentTaskViewport() {
	if m.width == 0 || m.height == 0 {
		return
	}
	innerW, _ := styles.FullScreenInnerSize(m.width, m.height)
	leftW := m.width / 3
	if leftW > 50 {
		leftW = 50
	}
	rightW := innerW - leftW
	if rightW < 1 {
		rightW = 1
	}
	contentH := m.rightPaneContentHeight()
	if contentH < 1 {
		contentH = 1
	}
	m.currentTaskViewport.Width = rightW
	m.currentTaskViewport.Height = contentH
}

// renderFeatureDetailsTab renders Tab 2 content: task list or inline detail view.
func (m statusModel) renderFeatureDetailsTab(width, height int) string {
	c := &m.taskListComponent
	if c.ConfirmDelete {
		return m.renderTab2ConfirmDelete(width, height)
	}
	if c.ShowDetail && c.detailReady {
		return m.renderTab2Detail(width, height)
	}
	return m.renderTab2TaskList(width, height, m.selectedPlan())
}

// renderTab2Detail renders the task detail view inline within the right pane.
func (m statusModel) renderTab2Detail(width, height int) string {
	c := &m.taskListComponent
	if !c.detailReady || height < 2 {
		return lipgloss.NewStyle().Width(width).Height(height).Render("")
	}

	return lipgloss.NewStyle().Width(width).Height(height).Render(c.detailViewport.View())
}

// renderTab2ConfirmDelete renders the task delete confirmation inline.
func (m statusModel) renderTab2ConfirmDelete(width, height int) string {
	c := &m.taskListComponent
	if c.Cursor >= len(c.Tasks) {
		return lipgloss.NewStyle().Width(width).Height(height).Render("")
	}
	t := c.Tasks[c.Cursor]
	warnStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.Warning)
	mutedStyle := lipgloss.NewStyle().Foreground(styles.Muted)

	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(warnStyle.Render(fmt.Sprintf("  Delete %s: %s?", t.ID, t.Title)))
	sb.WriteString("\n\n")
	sb.WriteString(mutedStyle.Render(fmt.Sprintf("  Plan: %s", filepath.Base(t.SourceFile))))
	sb.WriteString("\n\n")
	sb.WriteString("  This will permanently remove the task from the plan file.\n\n")
	sb.WriteString(fmt.Sprintf("  %s / %s",
		lipgloss.NewStyle().Bold(true).Render("y/enter: confirm"),
		mutedStyle.Render("n/esc: cancel")))

	return lipgloss.NewStyle().Width(width).Height(height).Render(sb.String())
}

// renderTab2TaskList renders the task list with feature header for Tab 2.
func (m statusModel) renderTab2TaskList(width, height int, plan parser.Plan) string {
	var sb strings.Builder

	// Header: title + progress bar + done/total count
	done := plan.DoneCount()
	total := len(plan.Tasks)
	planID := plan.ID
	if planID == "" {
		planID = filepath.Base(plan.File)
	}
	titleStr := styles.Truncate(planID, width-2)
	bar := buildProgressBar(done, total)
	count := statusDimStyle.Render(fmt.Sprintf(" %d/%d", done, total))

	sb.WriteString("\n " + styles.Title.Render(titleStr) + "\n")
	sb.WriteString(" " + bar + count + "\n")
	sb.WriteString(" " + styles.Separator(min(42, width-2)) + "\n")

	// Header occupies 4 lines
	headerLines := 4
	listH := height - headerLines
	if listH < 1 {
		listH = 1
	}

	tasks := m.taskListComponent.Tasks
	cursor := m.taskListComponent.Cursor
	scrollOffset := m.taskListComponent.ScrollOffset

	if len(tasks) == 0 {
		sb.WriteString(statusDimStyle.Render("  No tasks") + "\n")
	} else {
		end := min(scrollOffset+listH, len(tasks))
		for i := scrollOffset; i < end; i++ {
			t := tasks[i]
			var icon string
			var style lipgloss.Style
			if t.IsComplete() {
				icon = "✓"
				style = statusGreenStyle
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

			var prefix string
			if i == cursor {
				prefix = " ▸ "
				style = lipgloss.NewStyle().Bold(true).Foreground(styles.Primary)
			} else {
				prefix = "   "
			}

			line := fmt.Sprintf("%s%s  %s: %s", prefix, icon, t.ID, t.Title)
			sb.WriteString(style.Render(line) + "\n")
		}
		// Scroll indicator
		if len(tasks) > listH {
			hint := fmt.Sprintf(" [%d-%d of %d]", scrollOffset+1, min(scrollOffset+listH, len(tasks)), len(tasks))
			sb.WriteString(statusDimStyle.Render(hint) + "\n")
		}
	}

	return lipgloss.NewStyle().Width(width).Height(height).Render(sb.String())
}
