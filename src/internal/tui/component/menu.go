package component

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/leberkas-org/maggus/internal/tui/styles"
)

type MenuItem struct {
	Label  string
	Key    string
	Action string
}

type Menu struct {
	Title         string
	Items         []MenuItem
	Cursor        int
	Width, Height int
}

func NewMenu(title string, items []MenuItem) *Menu {
	return &Menu{
		Title: title,
		Items: items,
	}
}

func (m *Menu) MoveUp() {
	if m.Cursor > 0 {
		m.Cursor--
	} else {
		m.Cursor = len(m.Items) - 1
	}
}

func (m *Menu) MoveDown() {
	if m.Cursor < len(m.Items)-1 {
		m.Cursor++
	} else {
		m.Cursor = 0
	}
}

func (m *Menu) Selected() MenuItem {
	if m.Cursor < len(m.Items) {
		return m.Items[m.Cursor]
	}
	return MenuItem{}
}

func (m *Menu) View() string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.Primary)
	normalStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("15"))
	selectedStyle := lipgloss.NewStyle().
		Background(styles.Primary).
		Foreground(lipgloss.Color("0")).
		Bold(true)
	keyStyle := lipgloss.NewStyle().Foreground(styles.Muted)

	var lines []string
	lines = append(lines, titleStyle.Render(m.Title))
	lines = append(lines, "")

	maxLabel := 0
	for _, item := range m.Items {
		if len(item.Label) > maxLabel {
			maxLabel = len(item.Label)
		}
	}

	for i, item := range m.Items {
		label := item.Label
		padding := strings.Repeat(" ", maxLabel-len(label)+2)

		var line string
		if i == m.Cursor {
			line = selectedStyle.Render(" > " + label + padding)
			if item.Key != "" {
				line += " " + keyStyle.Render(item.Key)
			}
		} else {
			line = normalStyle.Render("   " + label + padding)
			if item.Key != "" {
				line += " " + keyStyle.Render(item.Key)
			}
		}
		lines = append(lines, line)
	}

	content := strings.Join(lines, "\n")

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.Primary).
		Padding(1, 2)

	return box.Render(content)
}
