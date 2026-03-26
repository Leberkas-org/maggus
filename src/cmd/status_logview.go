package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/leberkas-org/maggus/internal/runner"
	"github.com/leberkas-org/maggus/internal/tui/styles"
)

// viewLog renders the live log panel, replacing the task list content area.
// When the daemon is running and a state.json snapshot exists, it renders a rich
// TUI with spinner, tool list, and token stats. Otherwise it falls back to the
// plain JSONL log reader.
func (m statusModel) viewLog() string {
	var sb strings.Builder

	// Re-use the same header structure as viewStatus (title + daemon line + tabs + progress).
	visible := m.visiblePlans()

	totalTasks := 0
	totalDone := 0
	totalBlocked := 0
	activeFeatures := 0
	totalBugs := 0
	activeBugs := 0
	for _, f := range m.plans {
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
	featureCount := len(m.plans) - totalBugs

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
