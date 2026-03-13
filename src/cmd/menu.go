package cmd

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/leberkas-org/maggus/internal/tui/styles"
)

// menuItem represents a single entry in the main menu.
type menuItem struct {
	name string
	desc string
}

var menuItems = []menuItem{
	{name: "work", desc: "Work on the next N tasks from the implementation plan"},
	{name: "list", desc: "Preview the next N upcoming workable tasks"},
	{name: "status", desc: "Show a compact summary of plan progress"},
	{name: "blocked", desc: "Interactive wizard to manage blocked tasks"},
	{name: "clean", desc: "Remove completed plan files and finished run directories"},
	{name: "release", desc: "Generate RELEASE.md with changelog and AI summary"},
	{name: "worktree", desc: "Manage Maggus worktrees"},
}

// menuModel is the bubbletea model for the interactive main menu.
type menuModel struct {
	cursor   int
	selected string // command name chosen by the user, empty if quit
	quitting bool
}

func (m menuModel) Init() tea.Cmd {
	return nil
}

func (m menuModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			} else {
				m.cursor = len(menuItems) - 1
			}
		case "down", "j":
			if m.cursor < len(menuItems)-1 {
				m.cursor++
			} else {
				m.cursor = 0
			}
		case "home":
			m.cursor = 0
		case "end":
			m.cursor = len(menuItems) - 1
		case "enter":
			m.selected = menuItems[m.cursor].name
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m menuModel) View() string {
	titleStyle := styles.Title.MarginBottom(1)
	header := titleStyle.Render(fmt.Sprintf("Maggus %s", Version))

	selectedStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.Primary)
	cursorStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.Primary)
	descStyle := lipgloss.NewStyle().Foreground(styles.Muted)
	normalStyle := lipgloss.NewStyle()

	var sb strings.Builder
	for i, item := range menuItems {
		if i == m.cursor {
			fmt.Fprintf(&sb, "  %s %s  %s\n",
				cursorStyle.Render("→"),
				selectedStyle.Render(item.name),
				descStyle.Render(item.desc),
			)
		} else {
			fmt.Fprintf(&sb, "    %s  %s\n",
				normalStyle.Render(item.name),
				descStyle.Render(item.desc),
			)
		}
	}

	footer := styles.StatusBar.Render("↑/↓: navigate · enter: select · q/esc: exit")

	content := header + "\n" + sb.String() + "\n" + footer
	return styles.Box.Render(content) + "\n"
}
