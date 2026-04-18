package cmd

import tea "github.com/charmbracelet/bubbletea"

type mainModel struct {
	width  int
	height int
}

func newMainModel() mainModel {
	return mainModel{}
}

func (m mainModel) Init() tea.Cmd {
	return nil
}
