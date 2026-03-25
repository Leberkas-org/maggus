package cmd

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/leberkas-org/maggus/internal/runner"
	"github.com/leberkas-org/maggus/internal/tui/styles"
)

func (m statusModel) View() string {
	if len(m.features) == 0 {
		return m.viewEmpty()
	}
	if m.confirmDeleteFeature {
		return m.viewConfirmDeleteFeature()
	}
	if v := m.taskListComponent.View(); v != "" {
		return v
	}
	if m.showLog {
		return m.viewLog()
	}
	return m.viewStatus()
}

// viewConfirmDeleteFeature renders the feature-level delete confirmation dialog.
func (m statusModel) viewConfirmDeleteFeature() string {
	visible := m.visibleFeatures()
	if m.selectedFeature >= len(visible) {
		return ""
	}
	f := visible[m.selectedFeature]

	warnStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.Warning)
	mutedStyle := lipgloss.NewStyle().Foreground(styles.Muted)

	var sb strings.Builder
	sb.WriteString(warnStyle.Render(fmt.Sprintf("Delete %s?", filepath.Base(f.File))))
	sb.WriteString("\n\n")
	sb.WriteString("  This will permanently delete the file from disk.\n\n")
	sb.WriteString(fmt.Sprintf("  %s / %s",
		lipgloss.NewStyle().Bold(true).Render("y/enter: confirm"),
		mutedStyle.Render("n/esc: cancel")))

	bc := m.effectiveBorderColor()
	if m.Width > 0 && m.Height > 0 {
		return styles.FullScreenColor(sb.String(), "", m.Width, m.Height, bc)
	}
	return styles.Box.BorderForeground(bc).Render(sb.String()) + "\n"
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
		if p.IsBug && !needsSep {
			needsSep = true
			if len(tabs) > 0 {
				tabs = append(tabs, statusDimStyle.Render(" ┃ "))
			}
		}

		done := p.DoneCount()
		total := len(p.Tasks)
		name := strings.TrimSuffix(filepath.Base(p.File), ".md")
		isApproved := isPlanApproved(p, m.approvals, m.approvalRequired)
		approvalMark := "✓"
		if !isApproved {
			approvalMark = "✗"
		}
		label := fmt.Sprintf(" %s %s %d/%d ", approvalMark, name, done, total)
		if i == m.selectedFeature {
			if !isApproved {
				tabs = append(tabs, unapprovedTabStyle.Bold(true).Render(label))
			} else if p.IsBug {
				tabs = append(tabs, selectedBugStyle.Render(label))
			} else {
				tabs = append(tabs, selectedStyle.Render(label))
			}
		} else {
			if !isApproved {
				tabs = append(tabs, unapprovedTabStyle.Render(label))
			} else if p.IsBug {
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
		totalTasks += len(f.Tasks)
		totalDone += f.DoneCount()
		totalBlocked += f.BlockedCount()
		if f.IsBug {
			totalBugs++
			if !f.Completed {
				activeBugs++
			}
		} else {
			if !f.Completed {
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
		totalTasks += len(f.Tasks)
		totalDone += f.DoneCount()
		totalBlocked += f.BlockedCount()
		if f.IsBug {
			totalBugs++
			if !f.Completed {
				activeBugs++
			}
		} else {
			if !f.Completed {
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
		done := p.DoneCount()
		total := len(p.Tasks)
		blocked := p.BlockedCount()
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
		if p.Completed {
			sb.WriteString(statusDimGreen.Render(fmt.Sprintf(" Tasks — %s (archived)", filepath.Base(p.File))))
		} else {
			fmt.Fprintf(&sb, " Tasks — %s", filepath.Base(p.File))
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
				if p.Completed {
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

			if p.Completed {
				style = statusDimStyle
			}

			// Cursor indicator
			var prefix string
			if taskIdx == m.Cursor {
				prefix = " ▸ "
				if !p.Completed {
					style = lipgloss.NewStyle().Bold(true).Foreground(styles.Primary)
				}
			} else {
				prefix = "   "
			}

			line := fmt.Sprintf("%s%s  %s: %s", prefix, icon, t.ID, t.Title)
			sb.WriteString("\n")
			sb.WriteString(style.Render(line))

			if t.IsBlocked() && !p.Completed {
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

	// Status note (e.g. "feature approved") or delete error
	if m.deleteFeatureErr != "" {
		sb.WriteString("\n")
		sb.WriteString(statusRedStyle.Render("  Error: " + m.deleteFeatureErr))
	} else if m.statusNote != "" {
		sb.WriteString("\n")
		sb.WriteString(statusDimStyle.Render("  " + m.statusNote))
	}

	toggleHint := "alt+a: show all"
	if m.showAll {
		toggleHint = "alt+a: hide completed"
	}
	footer := styles.StatusBar.Render("←/→: switch feature · ↑/↓: navigate · enter: details · " + toggleHint + " · alt+p: approve/unapprove feature · alt+d: delete feature · alt+r: run · alt+bksp: delete · tab: live log · q/esc: exit")

	borderColor := styles.ThemeColor(m.is2x)
	if m.Width > 0 && m.Height > 0 {
		return styles.FullScreenColor(sb.String(), footer, m.Width, m.Height, borderColor)
	}
	return styles.Box.BorderForeground(borderColor).Render(sb.String()+"\n\n"+footer) + "\n"
}
