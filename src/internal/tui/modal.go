package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/leberkas-org/maggus/internal/tui/styles"
)

type ModalAction int

const (
	ModalNone ModalAction = iota
	ModalStop
	ModalDetach
	ModalCancel
)

type QuitModal struct {
	width, height int
}

func (m *QuitModal) View() string {
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.Warning).
		Padding(1, 2).
		Width(30)

	title := lipgloss.NewStyle().Bold(true).Render("Stop daemon or detach?")
	options := []string{
		"",
		"[S] Stop everything",
		"[D] Detach (daemon stays)",
		"[Esc] Cancel",
	}

	content := title + "\n" + strings.Join(options, "\n")
	rendered := box.Render(content)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, rendered)
}
