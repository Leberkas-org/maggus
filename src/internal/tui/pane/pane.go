package pane

import tea "github.com/charmbracelet/bubbletea"

type Pane interface {
	Init() tea.Cmd
	Update(msg tea.Msg) (Pane, tea.Cmd)
	View() string
	Resize(width, height int)
	Focus()
	Blur()
	IsFocused() bool
}

type BasePane struct {
	Width, Height int
	Focused       bool
}

func (b *BasePane) Resize(width, height int) {
	b.Width = width
	b.Height = height
}

func (b *BasePane) Focus()          { b.Focused = true }
func (b *BasePane) Blur()           { b.Focused = false }
func (b *BasePane) IsFocused() bool { return b.Focused }
