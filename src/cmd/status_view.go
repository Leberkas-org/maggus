package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/leberkas-org/maggus/internal/tui/styles"
)

func (m statusModel) View() string {
	if len(m.plans) == 0 {
		return m.viewEmpty()
	}
	if m.confirmDeleteFeature {
		return m.viewConfirmDeleteFeature()
	}
	if m.showLog {
		return m.viewLog()
	}
	return m.viewStatus()
}

// viewConfirmDeleteFeature renders the feature-level delete confirmation dialog.
func (m statusModel) viewConfirmDeleteFeature() string {
	visible := m.visiblePlans()
	if m.planCursor >= len(visible) {
		return ""
	}
	f := visible[m.planCursor]

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
	visible := m.visiblePlans()
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
		if i == m.planCursor {
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

func (m statusModel) viewStatus() string {
	// Split-pane layout when terminal dimensions are known.
	if m.width > 0 && m.height > 0 {
		return m.viewStatusSplit()
	}
	// Dimensions not yet known (before first WindowSizeMsg) — render empty frame.
	return ""
}

// viewStatusSplit renders the split-pane status view (left plan list + right content area).
func (m statusModel) viewStatusSplit() string {
	innerW, innerH := styles.FullScreenInnerSize(m.width, m.height)

	leftW := m.width / 3
	if leftW > 50 {
		leftW = 50
	}
	rightW := innerW - leftW
	if rightW < 0 {
		rightW = 0
	}

	leftPane := m.renderLeftPane(leftW, innerH)
	rightPane := m.renderRightPane(rightW, innerH)

	content := lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)
	borderColor := styles.ThemeColor(m.is2x)
	footer := styles.StatusBar.Render(m.statusSplitFooter())
	return styles.FullScreenLeftColor(content, footer, m.width, m.height, borderColor)
}

// statusSplitFooter returns the contextual key hint string for the current split-pane focus state.
func (m statusModel) statusSplitFooter() string {
	if m.leftFocused {
		return "↑/↓ navigate  enter: details  tab: switch pane  alt+p: approve  1: left  2-5: tabs  q: exit"
	}
	switch m.activeTab {
	case 0:
		return "↑/↓ scroll  G: bottom  1: left  2-5: tabs  tab: switch pane  q: exit"
	case 1:
		return "↑/↓ navigate  enter: detail  1: left  2-5: tabs  tab: switch pane  q: exit"
	case 2:
		return "↑/↓ scroll  1: left  2-5: tabs  tab: switch pane  q: exit"
	default:
		return "1: left  2-5: tabs  tab: switch pane  q: exit"
	}
}
