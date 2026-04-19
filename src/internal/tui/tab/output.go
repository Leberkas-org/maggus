package tab

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/leberkas-org/maggus/internal/tui/component"
)

type OutputTab struct {
	viewport *component.Viewport
}

func NewOutputTab() *OutputTab {
	return &OutputTab{viewport: component.NewViewport()}
}

func (t *OutputTab) Name() string { return "Output" }

func (t *OutputTab) Update(msg tea.Msg) (Tab, tea.Cmd) {
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

func (t *OutputTab) View(width, height int) string {
	t.viewport.Width = width
	t.viewport.Height = height
	return t.viewport.View()
}

func (t *OutputTab) SetData(data any) {
	if s, ok := data.(string); ok {
		t.viewport.SetContent(s)
	}
}

func (t *OutputTab) AppendLine(line string) {
	t.viewport.AppendLine(line)
}
