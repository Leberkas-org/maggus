package tab

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/leberkas-org/maggus/internal/tui/component"
)

type LogTab struct {
	viewport *component.Viewport
}

func NewLogTab() *LogTab {
	return &LogTab{viewport: component.NewViewport()}
}

func (t *LogTab) Name() string { return "Log" }

func (t *LogTab) Update(msg tea.Msg) (Tab, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "shift+up":
			t.viewport.ScrollUp(1)
		case "shift+down":
			t.viewport.ScrollDown(1)
		case "g":
			t.viewport.ScrollToTop()
		case "G":
			t.viewport.ScrollToBottom()
		}
	}
	return t, nil
}

func (t *LogTab) View(width, height int) string {
	t.viewport.Width = width
	t.viewport.Height = height
	return t.viewport.View()
}

func (t *LogTab) SetData(data any) {
	if s, ok := data.(string); ok {
		t.viewport.SetContent(s)
	}
}
