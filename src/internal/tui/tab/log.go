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
		case "alt+up":
			t.viewport.ScrollUp(1)
		case "alt+down":
			t.viewport.ScrollDown(1)
		case "pgup":
			t.viewport.ScrollUp(t.viewport.Height / 2)
		case "pgdown":
			t.viewport.ScrollDown(t.viewport.Height / 2)
		case "home":
			t.viewport.ScrollToTop()
		case "end":
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
