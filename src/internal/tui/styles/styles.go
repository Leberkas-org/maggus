// Package styles provides shared lipgloss styles, color palette, and layout
// helpers for consistent TUI rendering across all Maggus commands.
package styles

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Color palette — named constants using ANSI 256 colors for broad terminal compatibility.
var (
	Primary = lipgloss.Color("6") // cyan
	Success = lipgloss.Color("2") // green
	Warning = lipgloss.Color("3") // yellow
	Error   = lipgloss.Color("1") // red
	Muted   = lipgloss.Color("8") // gray
	Accent  = lipgloss.Color("4") // blue
)

// Reusable lipgloss styles.
var (
	Title     = lipgloss.NewStyle().Bold(true).Foreground(Primary)
	Subtitle  = lipgloss.NewStyle().Foreground(Primary)
	Label     = lipgloss.NewStyle().Bold(true)
	Value     = lipgloss.NewStyle().Foreground(Muted)
	Box       = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(Primary).Padding(0, 1)
	StatusBar = lipgloss.NewStyle().Foreground(Muted)
)

// SeparatorChar is the character used for horizontal rules.
const SeparatorChar = '─'

// Separator renders a styled horizontal rule of the given width.
func Separator(width int) string {
	if width <= 0 {
		return ""
	}
	line := strings.Repeat(string(SeparatorChar), width)
	return lipgloss.NewStyle().Foreground(Muted).Render(line)
}

// ProgressBar renders a styled progress bar. done is the number of completed
// items, total is the total count, and width is the bar width in characters.
func ProgressBar(done, total, width int) string {
	if width <= 0 {
		return ""
	}
	filled := 0
	if total > 0 {
		filled = (done * width) / total
	}
	if filled > width {
		filled = width
	}

	filledStyle := lipgloss.NewStyle().Foreground(Success)
	emptyStyle := lipgloss.NewStyle().Foreground(Muted)

	return filledStyle.Render(strings.Repeat("█", filled)) +
		emptyStyle.Render(strings.Repeat("░", width-filled))
}

// ProgressBarPlain renders an ASCII progress bar without styling (for --plain mode).
func ProgressBarPlain(done, total, width int) string {
	if width <= 0 {
		return ""
	}
	filled := 0
	if total > 0 {
		filled = (done * width) / total
	}
	if filled > width {
		filled = width
	}
	return strings.Repeat("#", filled) + strings.Repeat(".", width-filled)
}

// FullScreen wraps content in a bordered box that fills the terminal with a
// small margin, then centers the result on screen. Use this as the outermost
// wrapper in every View() to get a consistent layout across all commands.
func FullScreen(content string, width, height int) string {
	const margin = 2 // chars on each side

	innerW := width - margin*2 - 2  // 2 for border chars
	innerH := height - margin*2 - 2 // 2 for border chars
	if innerW < 0 {
		innerW = 0
	}
	if innerH < 0 {
		innerH = 0
	}

	box := Box.
		Width(innerW).
		Height(innerH).
		Render(content)

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)
}

// FullScreenInnerSize returns the usable content width and height inside a
// FullScreen box for the given terminal dimensions.
func FullScreenInnerSize(width, height int) (int, int) {
	const margin = 2
	innerW := width - margin*2 - 2 - 2 // margin + border + padding
	innerH := height - margin*2 - 2     // margin + border
	if innerW < 0 {
		innerW = 0
	}
	if innerH < 0 {
		innerH = 0
	}
	return innerW, innerH
}

// Truncate truncates text to maxWidth characters, adding "..." if truncated.
func Truncate(text string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	if len(text) <= maxWidth {
		return text
	}
	if maxWidth <= 3 {
		return text[:maxWidth]
	}
	return text[:maxWidth-3] + "..."
}
