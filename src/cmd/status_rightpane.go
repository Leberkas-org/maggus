package cmd

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/leberkas-org/maggus/internal/parser"
	"github.com/leberkas-org/maggus/internal/runlog"
	"github.com/leberkas-org/maggus/internal/tui/styles"
)

// renderRightPaneTabBar renders the tab bar at the top of the right pane.
// It uses the dynamic availableTabs() list so only context-relevant tabs are shown.
// Format: `[2] Output  [3] Details  [4] Metrics`
// The active tab has bold text and underline in primary color; inactive tabs are muted.
// Number prefixes are always dimmed.
func (m statusModel) renderRightPaneTabBar() string {
	dimStyle := lipgloss.NewStyle().Faint(true)
	activeStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.Primary).Underline(true)
	inactiveStyle := lipgloss.NewStyle().Foreground(styles.Muted)

	tabs := m.availableTabs()
	activeIdx := m.activeTabIndex()
	var parts []string
	for i, td := range tabs {
		numStr := dimStyle.Render(fmt.Sprintf("[%d]", i+1))
		var nameStr string
		if i == activeIdx {
			nameStr = activeStyle.Render(td.name)
		} else {
			nameStr = inactiveStyle.Render(td.name)
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
	sep := " " + styles.Separator(width-1)

	// Content height: total height minus tab bar line and separator line.
	contentH := height - 2
	if contentH < 1 {
		contentH = 1
	}

	tabs := m.availableTabs()
	tabKey := ""
	if activeIdx := m.activeTabIndex(); activeIdx >= 0 && activeIdx < len(tabs) {
		tabKey = tabs[activeIdx].key
	}
	// When a task detail is open (entered via tree), render it regardless of active tab.
	var content string
	c := &m.taskListComponent
	if c.ShowDetail && c.detailReady {
		content = m.renderTab2Detail(width, contentH)
	} else {
		switch tabKey {
		case "output":
			content = m.renderOutputTab(width, contentH)
		case "summary":
			content = m.renderSummaryTab(width, contentH)
		case "plan":
			content = m.renderPlanTab(width, contentH)
		case "details":
			content = m.renderFeatureDetailsTab(width, contentH)
		case "taskdetails":
			content = m.renderCurrentTaskTab(width, contentH)
		case "metrics":
			content = m.renderMetricsTab(width, contentH)
		default:
			content = lipgloss.NewStyle().Width(width).Height(contentH).Render("")
		}
	}

	full := tabBar + "\n" + sep + "\n" + content
	// MaxHeight clips any overflow that word-wrapping (from Width) may introduce,
	// preventing content from pushing the outer border off-screen.
	rendered := lipgloss.NewStyle().Width(width).Height(height).MaxHeight(height).Render(full)
	borderStyle := lipgloss.NewStyle().Foreground(styles.ThemeColor(m.isNerfed))
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
// in the Output tab's tool list.
func (m *statusModel) outputTabScrollableLines() int {
	contentH := m.rightPaneContentHeight()
	// Fixed lines consumed by the rich snapshot view:
	//   top:    status(1) + task(1) + separator(1) = 3
	//   bottom: separator(1) + tokens(1) + cost(1) + run(1) + task(1) = 5
	overhead := 8
	avail := contentH - overhead
	if avail < 3 {
		avail = 3
	}
	return avail
}

// renderOutputTab renders the Output tab for the selected task.
// For running tasks: shows the live snapshot view (same as before, but per-task).
// For completed tasks: loads and displays tool history from run log JSONL files.
// The old parallel worker card grid (renderWorkerPanes) is no longer used here.
func (m statusModel) renderOutputTab(width, contentH int) string {
	ctx := m.selectionCtx()

	if ctx == selRunningTask {
		snap := m.snapshotForSelectedTask()
		if snap != nil {
			spinnerFrame := m.spinnerFrameForTask(snap.TaskID)
			return m.renderSnapshotInPane(snap, spinnerFrame, width, contentH)
		}
		msg := statusDimStyle.Render("  Waiting for agent output...")
		return lipgloss.NewStyle().Width(width).Height(contentH).Render(msg)
	}

	if ctx == selCompletedTask {
		return m.renderCompletedTaskOutput(width, contentH)
	}

	msg := statusDimStyle.Render("  No active run")
	return lipgloss.NewStyle().Width(width).Height(contentH).Render(msg)
}

// renderSnapshotInPane renders the rich live TUI from a state snapshot,
// sized for the right pane content area. Accepts the snapshot and spinner
// frame so it works for both sequential (main snapshot) and parallel (per-worker) modes.
func (m statusModel) renderSnapshotInPane(snap *runlog.StateSnapshot, spinnerFrame, width, height int) string {
	var sb strings.Builder

	contentWidth := width - 4
	if contentWidth < 20 {
		contentWidth = 20
	}

	// ── Top zone (fixed): spinner/status + task ID/title + separator ──
	spinnerStr := statusCyanStyle.Render(styles.SpinnerFrames[spinnerFrame%len(styles.SpinnerFrames)])
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
	sb.WriteString(fmt.Sprintf(" %s  %s  %s\n", statusBoldStyle.Render("Status:"), spinnerStr, sColor.Render(snap.Status)))
	if snap.TaskID != "" {
		taskLabel := statusBoldStyle.Render("Task:")
		taskLabelW := lipgloss.Width(taskLabel)
		taskIDRendered := statusCyanStyle.Render(snap.TaskID)
		taskIDW := lipgloss.Width(taskIDRendered)
		maxTitleW := width - 1 - taskLabelW - 4 - taskIDW - 3

		if maxTitleW <= 0 {
			sb.WriteString(fmt.Sprintf(" %s    %s\n", taskLabel, taskIDRendered))
		} else {
			truncatedTitle := styles.Truncate(snap.TaskTitle, maxTitleW)
			sb.WriteString(fmt.Sprintf(" %s    %s - %s\n", taskLabel, taskIDRendered, truncatedTitle))
		}
	}
	sb.WriteString(" " + styles.Separator(width-1) + "\n")

	// ── Middle zone (scrollable tool list) ──
	// Overhead: top=3 (status+task+sep) + bottom=5 (sep+tokens+cost+run+task) = 8
	available := height - 8
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
		toolLines := buildToolLines(snap.ToolEntries, contentWidth)
		m.renderScrollableToolList(&sb, toolLines, totalTools, available)
	}

	// ── Bottom zone (fixed): separator + tokens + cost + run + task elapsed ──
	sb.WriteString(" " + styles.Separator(width-1) + "\n")

	totalIn := snap.TokenInput
	if totalIn > 0 || snap.TokenOutput > 0 {
		tokenStr := fmt.Sprintf("%s in / %s out",
			FormatTokens(totalIn), FormatTokens(snap.TokenOutput))
		sb.WriteString(fmt.Sprintf("  %s  %s\n", statusBoldStyle.Render("Tokens:"), statusDimStyle.Render(tokenStr)))
		costStr := "N/A"
		if snap.TokenCost > 0 {
			costStr = FormatCost(snap.TokenCost)
		}
		sb.WriteString(fmt.Sprintf("  %s    %s\n", statusBoldStyle.Render("Cost:"), statusDimStyle.Render(costStr)))
	} else {
		sb.WriteString(fmt.Sprintf("  %s  %s\n", statusBoldStyle.Render("Tokens:"), statusDimStyle.Render("N/A")))
		sb.WriteString(fmt.Sprintf("  %s    %s\n", statusBoldStyle.Render("Cost:"), statusDimStyle.Render("N/A")))
	}

	isTerminal := snap.Status == "Done" || snap.Status == "Failed" || snap.Status == "Interrupted"

	runElapsed := "—"
	if isTerminal && m.frozenRunElapsed != "" {
		runElapsed = m.frozenRunElapsed
	} else if snap.RunStartedAt != "" {
		if t, err := time.Parse(time.RFC3339, snap.RunStartedAt); err == nil {
			runElapsed = formatHumanDuration(time.Since(t))
		}
	}
	sb.WriteString(fmt.Sprintf("  %s     %s\n", statusBoldStyle.Render("Run:"), statusDimStyle.Render(runElapsed)))

	taskElapsed := "—"
	if isTerminal && m.frozenTaskElapsed != "" {
		taskElapsed = m.frozenTaskElapsed
	} else if snap.TaskStartedAt != "" {
		if t, err := time.Parse(time.RFC3339, snap.TaskStartedAt); err == nil {
			taskElapsed = formatHumanDuration(time.Since(t))
		}
	}
	sb.WriteString(fmt.Sprintf("  %s    %s", statusBoldStyle.Render("Task:"), statusDimStyle.Render(taskElapsed)))

	return sb.String()
}

// renderCurrentTaskContent returns the rendered detail content for a task.
// Returns an empty string when taskID is empty or the task cannot be found.
func renderCurrentTaskContent(taskID, taskFile string) string {
	if taskID == "" {
		return ""
	}
	t := reloadTask(taskFile, taskID)
	if t == nil {
		return ""
	}
	return renderDetailContent(*t, nil)
}

// loadCurrentTaskDetail loads the selected task (or fallback to next workable) into the currentTaskViewport.
func (m *statusModel) loadCurrentTaskDetail() {
	taskID := m.nextTaskID
	taskFile := m.nextTaskFile
	// If a task is selected in the tree, show that task instead of the global next.
	if t := m.selectedTask(); t != nil {
		taskID = t.ID
		taskFile = t.SourceFile
	}
	content := renderCurrentTaskContent(taskID, taskFile)
	m.currentTaskViewport.SetContent(content)
}

// renderCurrentTaskTab renders the Details tab: detail view of the selected task,
// or the next workable task if no task is selected. Shows a placeholder when nothing applies.
// Returns raw content without Width/Height wrapping — renderRightPane applies final sizing
// with MaxHeight so that long lines (e.g. acceptance criteria) do not push the border off-screen.
func (m statusModel) renderCurrentTaskTab(width, height int) string {
	// Check if a task is selected or if there's a next workable task.
	hasTask := m.selectedTask() != nil || m.nextTaskID != ""
	if !hasTask {
		mutedStyle := lipgloss.NewStyle().Foreground(styles.Muted)
		msg := mutedStyle.Render("No task selected")
		return lipgloss.NewStyle().Width(width).Height(height).
			Align(lipgloss.Center, lipgloss.Center).Render(msg)
	}
	return m.currentTaskViewport.View()
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
// Returns raw viewport content without Width/Height wrapping — renderRightPane applies
// final sizing with MaxHeight so that long lines do not push the border off-screen.
func (m statusModel) renderTab2Detail(width, height int) string {
	c := &m.taskListComponent
	if !c.detailReady || height < 2 {
		return lipgloss.NewStyle().Width(width).Height(height).Render("")
	}

	return c.detailViewport.View()
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

	sb.WriteString(" " + styles.Title.Render(titleStr) + "\n")
	sb.WriteString(" " + bar + count + "\n")
	sb.WriteString(" " + styles.Separator(width-1) + "\n")

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
			if t.IsComplete() && t.IsBlocked() {
				icon = "⊘"
				style = lipgloss.NewStyle().Foreground(styles.Warning)
			} else if t.IsComplete() {
				icon = "✓"
				style = statusGreenStyle
			} else if t.IsBlocked() {
				icon = "⚠"
				style = statusRedStyle
			} else if t.IsSkipped() {
				icon = ">"
				style = lipgloss.NewStyle().Foreground(styles.Muted).Faint(true)
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
		} else {
			sb.WriteString("\n")
		}
	}

	return lipgloss.NewStyle().Width(width).Height(height).Render(sb.String())
}
