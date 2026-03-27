package cmd

import (
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/leberkas-org/maggus/internal/tui/styles"
)

// leftPaneTruncate truncates s to maxW visible characters, appending "…" if needed.
// Uses a single Unicode ellipsis rather than "...".
func leftPaneTruncate(s string, maxW int) string {
	if maxW <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxW {
		return s
	}
	if maxW == 1 {
		return "…"
	}
	return string(runes[:maxW-1]) + "…"
}

// padToWidth pads a (possibly ANSI-styled) string to exactly width visual characters.
func padToWidth(s string, width int) string {
	w := lipgloss.Width(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}

// renderLeftPane renders the left pane plan list with a right border │.
// paneWidth is the total width including the │ border character.
// height is the number of lines the pane must fill.
func (m statusModel) renderLeftPane(paneWidth, height int) string {
	contentW := paneWidth - 1 // content width, excluding the │ border
	if contentW < 4 {
		contentW = 4
	}

	// Border dims when the right pane has focus.
	borderStyle := lipgloss.NewStyle().Foreground(styles.ThemeColor(m.is2x))

	mutedStyle := lipgloss.NewStyle().Foreground(styles.Muted)
	selectedBg := lipgloss.NewStyle().Background(lipgloss.Color("#1f2937"))

	cursorStyle := lipgloss.NewStyle().Foreground(styles.Primary)
	greenStyle := lipgloss.NewStyle().Foreground(styles.Success)
	orangeStyle := lipgloss.NewStyle().Foreground(styles.Warning)
	errorStyle := lipgloss.NewStyle().Foreground(styles.Error)

	visible := m.visiblePlans()
	var lines []string

	// Header row: matches right pane tab bar style — dimmed number prefix + styled label.
	dimStyle := lipgloss.NewStyle().Faint(true)
	var labelStyle lipgloss.Style
	if m.leftFocused {
		labelStyle = lipgloss.NewStyle().Bold(true).Foreground(styles.Primary).Underline(true)
	} else {
		labelStyle = mutedStyle
	}
	headerContent := " " + dimStyle.Render("1") + " " + labelStyle.Render("Features & Bugs")
	lines = append(lines, padToWidth(headerContent, contentW))

	// Horizontal separator under daemon status line.
	lines = append(lines, mutedStyle.Render(strings.Repeat("─", contentW-1)))
	lines = append(lines, "")

	// Daemon status line immediately below the header.
	var daemonLine string
	if m.daemon.Running {
		indicator := lipgloss.NewStyle().Foreground(styles.Success).Render("●")
		label := lipgloss.NewStyle().Foreground(styles.Success).Render(" Running")
		daemonLine = indicator + label
		if m.daemon.CurrentTask != "" {
			// Available width: contentW minus the visible width of indicator+label minus 2 spaces gap
			indicatorW := lipgloss.Width(indicator + label)
			taskMaxW := contentW - indicatorW - 2
			if taskMaxW > 0 {
				task := leftPaneTruncate(m.daemon.CurrentTask, taskMaxW)
				daemonLine += "  " + mutedStyle.Render(task)
			}
		}
	} else {
		daemonLine = mutedStyle.Render("○ Stopped")
	}
	lines = append(lines, padToWidth(" "+daemonLine, contentW))

	// Horizontal separator under daemon status line.
	lines = append(lines, mutedStyle.Render(strings.Repeat("─", contentW-1)))

	// Empty state.
	if len(visible) == 0 {
		lines = append(lines, mutedStyle.Render("  No features found"))
	}

	// Plan rows: features first, separator, then bugs.
	bugSepAdded := false
	bugAdded := false
	for i, plan := range visible {
		if !plan.IsBug && bugAdded && !bugSepAdded {
			bugSepAdded = true
			lines = append(lines, mutedStyle.Render(strings.Repeat("─", contentW-1)))
		}

		if plan.IsBug && !bugAdded {
			bugAdded = true
		}

		isSelected := i == m.planCursor

		// Cursor indicator (1 visual char).
		var cursorChar string
		if isSelected && m.leftFocused {
			cursorChar = cursorStyle.Render("▸")
		} else {
			cursorChar = " "
		}

		// Right-aligned approval badge.
		var badge string
		if plan.Completed {
			badge = mutedStyle.Render("✓")
		} else if isPlanApproved(plan, m.approvals, m.approvalRequired) {
			badge = greenStyle.Render("✓")
		} else {
			badge = orangeStyle.Render("○")
		}
		badgeW := lipgloss.Width(badge)

		// Layout: cursor(1) + space(1) + title + space(1) + badge + space(1) = contentW
		// → titleMaxW = contentW - 4 - badgeW
		titleMaxW := contentW - 4 - badgeW
		if titleMaxW < 0 {
			titleMaxW = 0
		}
		title := plan.ID
		if title == "" {
			title = filepath.Base(plan.File)
		}
		title = leftPaneTruncate(title, titleMaxW)

		// Apply per-type styling to title.
		var titleStr string
		switch {
		case plan.Completed:
			titleStr = mutedStyle.Render(title)
		case plan.IsBug:
			titleStr = errorStyle.Render(title)
		default:
			titleStr = title
		}

		// Pad title to fill its allocated width so the badge is right-aligned.
		titlePad := titleMaxW - lipgloss.Width(titleStr)
		if titlePad < 0 {
			titlePad = 0
		}
		rowContent := cursorChar + " " + titleStr + strings.Repeat(" ", titlePad) + " " + badge + " "

		// Apply selected-row highlight (background + left accent via ▶ cursor in primary color).
		var rowLine string
		if isSelected {
			rowLine = selectedBg.Render(padToWidth(rowContent, contentW))
		} else {
			rowLine = padToWidth(rowContent, contentW)
		}
		lines = append(lines, rowLine)
	}

	// Trim or pad to exact height.
	if len(lines) > height {
		lines = lines[:height]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}

	// Attach right border │ to each line.
	bChar := borderStyle.Render("│")
	result := make([]string, len(lines))
	for i, line := range lines {
		w := lipgloss.Width(line)

		if w < contentW {
			line += strings.Repeat(" ", contentW-w)
		}
		result[i] = line + bChar
	}

	bChar = borderStyle.Render("─")

	lastLine := strings.Repeat(bChar, contentW) + borderStyle.Render("┴")
	result = append(result, lastLine)
	return strings.Join(result, "\n")
}
