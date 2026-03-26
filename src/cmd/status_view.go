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
	// Full-screen component takeover only in compact (non-split) mode.
	// In split mode, task detail and confirm-delete are rendered inline in Tab 2.
	if m.width == 0 || m.height == 0 {
		if v := m.taskListComponent.View(); v != "" {
			return v
		}
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
	// New: split-pane layout when terminal dimensions are known.
	if m.width > 0 && m.height > 0 {
		return m.viewStatusSplit()
	}

	// Fallback: compact rendering without known terminal dimensions.
	var sb strings.Builder
	visible := m.visiblePlans()

	// Compute totals
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
	totalPending := totalTasks - totalDone - totalBlocked
	featureCount := len(m.plans) - totalBugs

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
	if m.planCursor < len(visible) {
		p := visible[m.planCursor]
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
	if m.planCursor < len(visible) {
		p := visible[m.planCursor]

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
	return styles.Box.BorderForeground(borderColor).Render(sb.String()+"\n\n"+footer) + "\n"
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
	return styles.FullScreenLeftColor(content, "", m.width, m.height, borderColor)
}
