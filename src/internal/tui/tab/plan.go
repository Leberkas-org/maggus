package tab

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/leberkas-org/maggus/internal/tui/component"
)

type PlanTab struct {
	viewport *component.Viewport
	rawMD    string
	lastW    int
}

func NewPlanTab() *PlanTab {
	return &PlanTab{viewport: component.NewViewport()}
}

func (t *PlanTab) Name() string { return "Plan" }

func (t *PlanTab) Update(msg tea.Msg) (Tab, tea.Cmd) {
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

func (t *PlanTab) View(width, height int) string {
	if width != t.lastW && t.rawMD != "" {
		t.lastW = width
		t.viewport.SetContent(renderMD(t.rawMD, width))
	}
	t.viewport.Width = width
	t.viewport.Height = height
	return t.viewport.View()
}

func (t *PlanTab) SetData(data any) {
	if s, ok := data.(string); ok && s != t.rawMD {
		t.rawMD = s
		t.lastW = 0
	}
}
