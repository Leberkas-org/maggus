package tab

import (
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/leberkas-org/maggus/internal/tui/component"
)

type SummaryTab struct {
	viewport *component.Viewport
	rawMD    string
	lastW    int
}

func NewSummaryTab() *SummaryTab {
	return &SummaryTab{viewport: component.NewViewport()}
}

func (t *SummaryTab) Name() string { return "Summary" }

func (t *SummaryTab) Update(msg tea.Msg) (Tab, tea.Cmd) {
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

func (t *SummaryTab) View(width, height int) string {
	if width != t.lastW && t.rawMD != "" {
		t.lastW = width
		t.renderMarkdown(width)
	}
	t.viewport.Width = width
	t.viewport.Height = height
	return t.viewport.View()
}

func (t *SummaryTab) SetData(data any) {
	if s, ok := data.(string); ok && s != t.rawMD {
		t.rawMD = s
		t.lastW = 0
	}
}

func (t *SummaryTab) renderMarkdown(width int) {
	rendered := renderMD(t.rawMD, width)
	t.viewport.SetContent(rendered)
}

func renderMD(md string, width int) string {
	w := max(width-4, 20)
	r, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(w),
	)
	if err != nil {
		return md
	}
	rendered, err := r.Render(md)
	if err != nil {
		return md
	}
	return clampLines(rendered, width)
}

func clampLines(s string, maxW int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i, line := range lines {
		if lipgloss.Width(line) > maxW {
			lines[i] = ansi.Truncate(line, maxW, "")
		}
	}
	return strings.Join(lines, "\n")
}
