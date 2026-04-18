package cmd

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/leberkas-org/maggus/internal/tui/styles"
)

func (m mainModel) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}
	return m.viewSplit()
}

func (m mainModel) viewSplit() string {
	innerW, innerH := styles.FullScreenInnerSize(m.width, m.height)

	leftW := min(m.width/3, 50)
	rightW := max(innerW-leftW, 0)

	contentH := innerH - 1
	leftPane := m.renderLeftPane(leftW, contentH)
	rightPane := m.renderRightPane(rightW, contentH)

	content := lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)
	footer := styles.StatusBar.Render("q: quit")
	return styles.FullScreenLeftColor(content, footer, m.width, m.height, styles.Primary)
}

func (m mainModel) renderLeftPane(width, height int) string {
	paneW := max(width-2, 0)
	paneH := height - 1
	left := lipgloss.NewStyle().
		Width(paneW).
		Height(paneH).
		Render("")
	left += "\n" + styles.Separator(paneW)
	divStyle := lipgloss.NewStyle().Foreground(styles.Muted)
	divider := divStyle.Render(strings.Repeat("│\n", height-1) + "┴")
	return lipgloss.JoinHorizontal(lipgloss.Top, left, divider)
}

func (m mainModel) renderRightPane(width, height int) string {
	paneH := height - 1
	content := lipgloss.NewStyle().
		Width(width).
		Height(paneH).
		Render("")
	return content + "\n" + styles.Separator(width)
}
