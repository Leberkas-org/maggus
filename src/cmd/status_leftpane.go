package cmd

import (
	"fmt"
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

// selectedBgColor is the background color applied to the selected row in the left pane.
const selectedBgColor lipgloss.Color = "#1f2937"

// withBg returns a copy of style with the given background color applied.
func withBg(style lipgloss.Style, bg lipgloss.Color) lipgloss.Style {
	return style.Background(bg)
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

	whiteStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff"))
	greenStyle := lipgloss.NewStyle().Foreground(styles.Success)
	orangeStyle := lipgloss.NewStyle().Foreground(styles.Warning)
	errorStyle := lipgloss.NewStyle().Foreground(styles.Error)
	primaryStyle := lipgloss.NewStyle().Foreground(styles.Primary)

	allItems := m.buildTreeItems()
	availH := m.treeAvailableHeight()

	// Slice to visible window based on scroll offset.
	scrollOff := m.treeScrollOffset
	if scrollOff > len(allItems) {
		scrollOff = len(allItems)
	}
	end := scrollOff + availH
	if end > len(allItems) {
		end = len(allItems)
	}
	items := allItems[scrollOff:end]

	var lines []string

	// Header row: matches right pane tab bar style — dimmed number prefix + styled label.
	dimStyle := lipgloss.NewStyle().Faint(true)
	var labelStyle lipgloss.Style
	if m.leftFocused {
		labelStyle = lipgloss.NewStyle().Bold(true).Foreground(styles.Primary).Underline(true)
	} else {
		labelStyle = mutedStyle
	}
	headerContent := " " + dimStyle.Render("[1]") + " " + labelStyle.Render("Items")
	lines = append(lines, padToWidth(headerContent, contentW))

	// Horizontal separator under header.
	lines = append(lines, mutedStyle.Render(strings.Repeat("─", contentW-1)))

	if m.exitDaemonOverlay || m.daemonStopOverlay {
		if m.exitDaemonOverlay {
			lines = append(lines, lipgloss.NewStyle().Foreground(styles.Primary).Render("Daemon still running!!"))
			lines = append(lines, "")
			lines = append(lines, lipgloss.NewStyle().Foreground(styles.Success).Render("[d] keep running"))
		} else {
			lines = append(lines, lipgloss.NewStyle().Foreground(styles.Primary).Render("Really want to stop daemon?"))
			lines = append(lines, "")
		}

		lines = append(lines, lipgloss.NewStyle().Foreground(styles.Warning).Render("[y] stop"))
		lines = append(lines, lipgloss.NewStyle().Foreground(styles.Error).Render("[ctrl+c] kill"))
		lines = append(lines, lipgloss.NewStyle().Foreground(styles.Muted).Render("[esc] no/cancel "))
	} else {
		lines = append(lines, "")
	}

	// Daemon status line immediately below the header.
	var daemonLine string
	if !m.exitDaemonOverlay && !m.daemonStopOverlay {
		if m.daemon.Running {
			daemonLine = lipgloss.NewStyle().Foreground(styles.Success).Render("● Running")
			if m.daemon.CurrentTask != "" {
				// Available width: contentW minus the visible width of indicator+label minus 2 spaces gap
				indicatorW := lipgloss.Width(daemonLine)
				taskMaxW := contentW - indicatorW - 3
				if taskMaxW > 0 {
					blub := m.daemon.CurrentFeature + " " + m.daemon.CurrentTask
					task := leftPaneTruncate(blub, taskMaxW)
					daemonLine += "  " + mutedStyle.Render(task)
				}
			}
		} else {
			daemonLine = mutedStyle.Render("○ Stopped")
		}
	}
	lines = append(lines, padToWidth(" "+daemonLine, contentW))

	// Horizontal separator under daemon status line.
	lines = append(lines, mutedStyle.Render(strings.Repeat("─", contentW-1)))

	// Empty state.
	if len(items) == 0 {
		lines = append(lines, mutedStyle.Render("  No features found"))
	}

	// Spinner character — reserved unconditionally (1 char) to prevent layout jitter.
	spinnerChar := styles.SpinnerFrames[m.spinnerFrame]

	// Plan rows: features first, separator, then bugs (or bugs first based on ordering).
	// Track when we cross the boundary between bugs and non-bugs sections.
	bugSepAdded := false
	bugAdded := false

	for i, item := range items {
		isSelected := (i + scrollOff) == m.treeCursor

		// Per-row helpers that embed the selection background into styled strings.
		addBg := func(s lipgloss.Style) lipgloss.Style {
			if isSelected {
				return withBg(s, selectedBgColor)
			}
			return s
		}
		bgStr := func(s string) string {
			if isSelected {
				return lipgloss.NewStyle().Background(selectedBgColor).Render(s)
			}
			return s
		}

		if item.kind == treeItemKindPlan {
			plan := item.plan

			// Insert separator between bugs section and features section.
			if !plan.IsBug && bugAdded && !bugSepAdded {
				bugSepAdded = true
				lines = append(lines, mutedStyle.Render(strings.Repeat("─", contentW-1)))
			}
			if plan.IsBug && !bugAdded {
				bugAdded = true
			}

			// Expand/collapse icon (1 visual char).
			var expandIcon string
			if len(plan.Tasks) == 0 {
				expandIcon = bgStr(" ")
			} else if isSelected && m.leftFocused {
				if m.expandedPlans[plan.ID] {
					expandIcon = addBg(whiteStyle).Render("▼")
				} else {
					expandIcon = addBg(whiteStyle).Render("▶")
				}
			} else if isSelected {
				if m.expandedPlans[plan.ID] {
					expandIcon = addBg(mutedStyle).Render("▼")
				} else {
					expandIcon = addBg(mutedStyle).Render("▶")
				}
			} else {
				if m.expandedPlans[plan.ID] {
					expandIcon = mutedStyle.Render("▼")
				} else {
					expandIcon = mutedStyle.Render("▶")
				}
			}

			// Spinner column (1 char, always reserved).
			var spinStr string
			if m.daemon.Running && m.daemon.CurrentFeature == plan.ID {
				spinStr = addBg(primaryStyle).Render(spinnerChar)
			} else {
				spinStr = bgStr(" ")
			}

			// Approval badge.
			var badge string
			if plan.Completed {
				badge = addBg(mutedStyle).Render("✓")
			} else if isPlanApproved(plan, m.approvals, m.approvalRequired) {
				badge = addBg(greenStyle).Render("✓")
			} else {
				badge = addBg(orangeStyle).Render("○")
			}
			badgeW := lipgloss.Width(badge)

			// Progress badge (N/T), hidden if no tasks.
			var progBadge string
			progBadgeW := 0
			if len(plan.Tasks) > 0 {
				progStr := fmt.Sprintf("%d/%d", plan.DoneCount(), len(plan.Tasks))
				progBadge = addBg(mutedStyle).Render(progStr)
				progBadgeW = len(progStr) + 1 // +1 for the space before it
			}

			// Layout: expand(1) + space(1) + spinner(1) + space(1) + title + space(1) + [progBadge + space(1)] + badge + space(1)
			// Fixed overhead on right = (progBadgeW) + 1 + badgeW + 1
			// Fixed overhead on left  = 1 + 1 + 1 = 3
			titleMaxW := contentW - 4 - progBadgeW - 1 - badgeW - 1
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
				titleStr = addBg(mutedStyle).Render(title)
			case plan.IsBug:
				titleStr = addBg(errorStyle).Render(title)
			default:
				if isSelected && m.leftFocused {
					titleStr = addBg(whiteStyle).Render(title)
				} else {
					titleStr = bgStr(title)
				}
			}

			// Pad title to fill its allocated width so badges stay right-aligned.
			titlePad := titleMaxW - lipgloss.Width(title)
			if titlePad < 0 {
				titlePad = 0
			}

			rowContent := expandIcon + bgStr(" ") + spinStr + bgStr(" ") + titleStr + bgStr(strings.Repeat(" ", titlePad))
			if progBadgeW > 0 {
				rowContent += bgStr(" ") + progBadge
			}
			rowContent += bgStr(" ") + badge + bgStr(" ")

			// Pad any remaining width with bg-aware spaces then emit the row.
			if rw := lipgloss.Width(rowContent); rw < contentW {
				rowContent += bgStr(strings.Repeat(" ", contentW-rw))
			}
			lines = append(lines, rowContent)

		} else {
			// Task row.
			task := item.task

			// Spinner column (1 char, always reserved).
			var spinStr string
			if m.daemon.Running && m.daemon.CurrentTask == task.ID {
				spinStr = addBg(primaryStyle).Render(spinnerChar)
			} else if task.IsComplete() {
				spinStr = addBg(greenStyle).Render("✓")
			} else {
				spinStr = bgStr(" ")
			}

			// Layout: indent(3) + spinner(1) + taskID + space(1) + taskTitle
			// Fixed = 4; allocate task ID up to half remaining, rest for title.
			avail := contentW - 4
			if avail < 0 {
				avail = 0
			}
			taskIDMaxW := avail / 2
			if taskIDMaxW < 1 {
				taskIDMaxW = 1
			}
			taskIDStr := leftPaneTruncate(task.ID, taskIDMaxW)
			taskIDVisW := lipgloss.Width(taskIDStr)

			titleAvail := avail - taskIDVisW - 1
			if titleAvail < 0 {
				titleAvail = 0
			}
			taskTitleStr := leftPaneTruncate(task.Title, titleAvail)

			var taskIDRendered, taskTitleRendered string
			if isSelected && m.leftFocused {
				taskIDRendered = addBg(whiteStyle).Render(taskIDStr)
				taskTitleRendered = addBg(whiteStyle).Render(taskTitleStr)
			} else if isSelected {
				taskIDRendered = addBg(mutedStyle).Render(taskIDStr)
				taskTitleRendered = addBg(mutedStyle).Render(taskTitleStr)
			} else {
				taskIDRendered = mutedStyle.Render(taskIDStr)
				taskTitleRendered = mutedStyle.Render(taskTitleStr)
			}
			rowContent := bgStr("   ") + spinStr + taskIDRendered + bgStr(" ") + taskTitleRendered

			// Pad any remaining width with bg-aware spaces then emit the row.
			if rw := lipgloss.Width(rowContent); rw < contentW {
				rowContent += bgStr(strings.Repeat(" ", contentW-rw))
			}
			lines = append(lines, rowContent)
		}
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
