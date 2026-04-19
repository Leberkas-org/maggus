package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/leberkas-org/maggus/internal/tui/styles"
)

type OverlayKind int

const (
	OverlayNone OverlayKind = iota
	OverlayMenu
	OverlayRepoList
	OverlayAddRepo
	OverlayBrowseRepo
	OverlayImportPlan
	OverlayQuit
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
	return box.Render(content)
}
