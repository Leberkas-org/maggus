package pane

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/leberkas-org/maggus/internal/tui/styles"
)

type FooterPane struct {
	BasePane
	StatusText string
	KeyHints   string
}

func NewFooterPane() *FooterPane {
	return &FooterPane{
		KeyHints: "tab: switch pane  a: approve  x: skip  q: quit",
	}
}

func (p *FooterPane) Init() tea.Cmd { return nil }

func (p *FooterPane) Update(msg tea.Msg) (Pane, tea.Cmd) {
	return p, nil
}

func (p *FooterPane) View() string {
	status := lipgloss.NewStyle().Foreground(styles.Primary).Render(p.StatusText)
	hints := styles.StatusBar.Render(p.KeyHints)

	left := lipgloss.NewStyle().Width(p.Width / 2).Render(status)
	right := lipgloss.NewStyle().Width(p.Width / 2).Align(lipgloss.Right).Render(hints)

	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

func (p *FooterPane) SetStatus(text string) {
	p.StatusText = text
}

func (p *FooterPane) SetKeyHints(hints string) {
	p.KeyHints = hints
}
