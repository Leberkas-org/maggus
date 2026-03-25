package cmd

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/leberkas-org/maggus/internal/tui/styles"
)

func (m configModel) renderTabBar() string {
	tabNames := []string{"Project", "Global"}

	activeStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(styles.Primary).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.Primary).
		Padding(0, 1)

	inactiveStyle := lipgloss.NewStyle().
		Foreground(styles.Muted).
		Padding(0, 1)

	focusedActiveStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("0")).
		Background(styles.Primary).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.Primary).
		Padding(0, 1)

	var tabs []string
	for i, name := range tabNames {
		if i == m.activeTab {
			if m.tabFocused {
				tabs = append(tabs, focusedActiveStyle.Render(name))
			} else {
				tabs = append(tabs, activeStyle.Render(name))
			}
		} else {
			tabs = append(tabs, inactiveStyle.Render(name))
		}
	}

	return lipgloss.JoinHorizontal(lipgloss.Bottom, tabs...)
}

func (m configModel) View() string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.Primary)
	cursorStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.Primary)
	activeValueStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.Success)
	activeOffStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.Error)
	mutedStyle := lipgloss.NewStyle().Foreground(styles.Muted)
	normalStyle := lipgloss.NewStyle()
	saveStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.Success)

	// Sections without a path display just the title
	sectionTitleOnly := map[string]bool{
		"On complete behaviour": true,
	}

	var sb strings.Builder

	// Render tab bar
	sb.WriteString(m.renderTabBar())
	sb.WriteString("\n\n")

	rows := *m.activeRows()
	for i, row := range rows {
		// Section header
		if row.section != "" {
			if i > 0 {
				sb.WriteString("\n")
			}
			if sectionTitleOnly[row.section] {
				sb.WriteString(titleStyle.Render(row.section) + "\n")
			} else {
				sb.WriteString(titleStyle.Render(row.section) + "\n")
			}
		}

		if row.isOption() {
			label := fmt.Sprintf("%-22s", row.label)

			var valueParts []string
			for vi, v := range row.values {
				if vi == row.current {
					if v == "off" {
						valueParts = append(valueParts, activeOffStyle.Render(v))
					} else {
						valueParts = append(valueParts, activeValueStyle.Render(v))
					}
				} else {
					valueParts = append(valueParts, mutedStyle.Render(v))
				}
			}
			valueStr := strings.Join(valueParts, mutedStyle.Render(" / "))

			if i == m.cursor {
				fmt.Fprintf(&sb, "  %s %s  %s\n",
					cursorStyle.Render("->"),
					normalStyle.Render(label),
					valueStr,
				)
			} else {
				fmt.Fprintf(&sb, "     %s  %s\n",
					normalStyle.Render(label),
					valueStr,
				)
			}
		} else if row.isDisplay() {
			label := fmt.Sprintf("%-22s", row.label)
			hint := mutedStyle.Render("(edit in config.yml)")
			displayVal := mutedStyle.Render(row.display)
			if i == m.cursor {
				fmt.Fprintf(&sb, "  %s %s  %s  %s\n",
					cursorStyle.Render("->"),
					normalStyle.Render(label),
					displayVal,
					hint,
				)
			} else {
				fmt.Fprintf(&sb, "     %s  %s  %s\n",
					normalStyle.Render(label),
					displayVal,
					hint,
				)
			}
		} else {
			// Action button
			btnStyle := normalStyle
			if row.isSave {
				btnStyle = saveStyle
			}
			if i == m.cursor {
				fmt.Fprintf(&sb, "  %s %s\n", cursorStyle.Render("->"), btnStyle.Render(row.label))
			} else {
				fmt.Fprintf(&sb, "     %s\n", mutedStyle.Render(row.label))
			}
		}
	}

	// Status feedback
	if m.statusText != "" {
		sb.WriteString("\n")
		statusStyle := lipgloss.NewStyle().Foreground(styles.Success)
		if strings.HasPrefix(m.statusText, "Error:") {
			statusStyle = lipgloss.NewStyle().Foreground(styles.Error)
		}
		sb.WriteString("  " + statusStyle.Render(m.statusText) + "\n")
	}

	content := sb.String()
	footer := styles.StatusBar.Render("1/2: switch tab | up/down: navigate | left/right: change value | enter: select | q/esc: exit")

	borderColor := styles.ThemeColor(m.is2x)
	if m.width > 0 && m.height > 0 {
		return styles.FullScreenColor(content, footer, m.width, m.height, borderColor)
	}
	return styles.Box.BorderForeground(borderColor).Render(content+"\n\n"+footer) + "\n"
}
