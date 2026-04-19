package component

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
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
	v.Offset = 0
}

func (v *Viewport) AppendLine(line string) {
	v.lines = append(v.lines, line)
	v.Content = strings.Join(v.lines, "\n")
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
	if len(v.lines) == 0 || v.Height <= 0 || v.Width <= 0 {
		return strings.Repeat("\n", max(v.Height-1, 0))
	}

	// Clamp offset to valid range
	maxOff := max(0, len(v.lines)-v.Height)
	if v.Offset > maxOff {
		v.Offset = maxOff
	}
	if v.Offset < 0 {
		v.Offset = 0
	}

	end := min(v.Offset+v.Height, len(v.lines))
	visible := v.lines[v.Offset:end]

	var rendered []string
	for _, line := range visible {
		// ANSI-aware truncation
		if lipgloss.Width(line) > v.Width {
			line = ansi.Truncate(line, v.Width, "")
		}
		rendered = append(rendered, line)
	}

	for len(rendered) < v.Height {
		rendered = append(rendered, "")
	}

	return strings.Join(rendered, "\n")
}
