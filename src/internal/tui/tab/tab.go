package tab

import tea "github.com/charmbracelet/bubbletea"

type Tab interface {
	Name() string
	Update(msg tea.Msg) (Tab, tea.Cmd)
	View(width, height int) string
	SetData(data any)
}
