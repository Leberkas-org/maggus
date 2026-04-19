package component

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type Viewport struct {
	Content       string
	Width, Height int
	Offset        int
	lines         []string
}

func NewViewport() *Viewport {
	return &Viewport{}
}

func (v *Viewport) SetContent(content string) {
	v.Content = content
	v.lines = strings.Split(content, "\n")
}

func (v *Viewport) AppendLine(line string) {
	v.lines = append(v.lines, line)
	v.Content = strings.Join(v.lines, "\n")
	// Auto-scroll to bottom
	if len(v.lines) > v.Height {
		v.Offset = len(v.lines) - v.Height
	}
}

func (v *Viewport) ScrollUp(n int) {
	v.Offset = max(0, v.Offset-n)
}

func (v *Viewport) ScrollDown(n int) {
	maxOff := max(0, len(v.lines)-v.Height)
	v.Offset = min(v.Offset+n, maxOff)
}

func (v *Viewport) ScrollToTop() {
	v.Offset = 0
}

func (v *Viewport) ScrollToBottom() {
	v.Offset = max(0, len(v.lines)-v.Height)
}

func (v *Viewport) View() string {
	if len(v.lines) == 0 {
		return ""
	}

	end := min(v.Offset+v.Height, len(v.lines))
	visible := v.lines[v.Offset:end]

	var rendered []string
	for _, line := range visible {
		if len(line) > v.Width {
			line = line[:v.Width]
		}
		rendered = append(rendered, line)
	}

	// Pad to fill height
	for len(rendered) < v.Height {
		rendered = append(rendered, "")
	}

	return lipgloss.NewStyle().
		Width(v.Width).
		Height(v.Height).
		Render(strings.Join(rendered, "\n"))
}
