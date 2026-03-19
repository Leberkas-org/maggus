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

// FullScreenMargin is the number of characters reserved on each side of the
// full-screen box (outside the border).
const FullScreenMargin = 2

// maxContentWidth is the target column width for centering content inside the
// full-screen box. All views use the same reference so centering is consistent.
const maxContentWidth = 90

// FullScreen wraps content and an optional footer in a bordered box that fills
// the terminal with a small margin. Content is centered as a block (columns
// stay aligned) and the footer is pinned to the bottom line.
func FullScreen(content, footer string, width, height int) string {
	innerW, innerH := fullScreenInner(width, height)

	// Calculate consistent left padding based on a fixed target width.
	// This ensures all views are centered the same way regardless of content.
	padLeft := (innerW - maxContentWidth) / 2
	if padLeft < 0 {
		padLeft = 0
	}

	// Indent all non-empty content lines by the same amount.
	if padLeft > 0 {
		prefix := strings.Repeat(" ", padLeft)
		lines := strings.Split(content, "\n")
		for i, l := range lines {
			if l != "" {
				lines[i] = prefix + l
			}
		}
		content = strings.Join(lines, "\n")
	}

	// Count content lines
	contentLines := strings.Count(content, "\n") + 1

	// Build the composed body: content + gap + footer (centered independently)
	var body string
	if footer != "" {
		centeredFooter := centerFooter(footer, innerW)
		footerLineCount := strings.Count(centeredFooter, "\n") + 1

		gap := innerH - contentLines - footerLineCount
		if gap < 0 {
			gap = 0
		}
		body = content + strings.Repeat("\n", gap) + centeredFooter
	} else {
		body = content
	}

	// Box has Padding(0,1) — add 2 to Width so the content area equals innerW.
	box := Box.
		Width(innerW + 2).
		Height(innerH).
		Render(body)

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)
}

// FullScreenLeft is like FullScreen but keeps content left-aligned inside
// the box (no horizontal centering). Use for detail views and work output.
func FullScreenLeft(content, footer string, width, height int) string {
	return FullScreenLeftColor(content, footer, width, height, Primary)
}

// FullScreenLeftColor is like FullScreenLeft but allows overriding the border color.
func FullScreenLeftColor(content, footer string, width, height int, borderColor lipgloss.Color) string {
	innerW, innerH := fullScreenInner(width, height)

	contentLines := strings.Count(content, "\n") + 1

	var body string
	if footer != "" {
		centeredFooter := centerFooter(footer, innerW)
		footerLineCount := strings.Count(centeredFooter, "\n") + 1

		gap := innerH - contentLines - footerLineCount
		if gap < 0 {
			gap = 0
		}
		body = content + strings.Repeat("\n", gap) + centeredFooter
	} else {
		body = content
	}

	// Box has Padding(0,1) — add 2 to Width so the content area equals innerW.
	box := Box.
		BorderForeground(borderColor).
		Width(innerW + 2).
		Height(innerH).
		Render(body)

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)
}

// centerFooter trims trailing newlines, then centers each line independently.
func centerFooter(footer string, innerW int) string {
	footer = strings.TrimRight(footer, "\n")
	lines := strings.Split(footer, "\n")
	for i, line := range lines {
		lineW := lipgloss.Width(line)
		pad := (innerW - lineW) / 2
		if pad < 0 {
			pad = 0
		}
		lines[i] = strings.Repeat(" ", pad) + line
	}
	return strings.Join(lines, "\n")
}

// FullScreenInnerSize returns the usable content width and height inside a
// FullScreen box for the given terminal dimensions.
func FullScreenInnerSize(width, height int) (int, int) {
	return fullScreenInner(width, height)
}

func fullScreenInner(width, height int) (int, int) {
	innerW := width - FullScreenMargin*2 - 2 - 2 // margin + border + padding
	innerH := height - FullScreenMargin*2 - 2    // margin + border
	if innerW < 0 {
		innerW = 0
	}
	if innerH < 0 {
		innerH = 0
	}
	return innerW, innerH
}

// ThemeColor returns Warning (yellow) when is2x is true, otherwise Primary (cyan).
// Used to dynamically switch logo and border colors based on Claude 2x status.
func ThemeColor(is2x bool) lipgloss.Color {
	if is2x {
		return Warning
	}
	return Primary
}

// FullScreenColor is like FullScreen but allows overriding the border color.
func FullScreenColor(content, footer string, width, height int, borderColor lipgloss.Color) string {
	innerW, innerH := fullScreenInner(width, height)

	padLeft := (innerW - maxContentWidth) / 2
	if padLeft < 0 {
		padLeft = 0
	}

	if padLeft > 0 {
		prefix := strings.Repeat(" ", padLeft)
		lines := strings.Split(content, "\n")
		for i, l := range lines {
			if l != "" {
				lines[i] = prefix + l
			}
		}
		content = strings.Join(lines, "\n")
	}

	contentLines := strings.Count(content, "\n") + 1

	var body string
	if footer != "" {
		centeredFooter := centerFooter(footer, innerW)
		footerLineCount := strings.Count(centeredFooter, "\n") + 1

		gap := innerH - contentLines - footerLineCount
		if gap < 0 {
			gap = 0
		}
		body = content + strings.Repeat("\n", gap) + centeredFooter
	} else {
		body = content
	}

	box := Box.
		BorderForeground(borderColor).
		Width(innerW + 2).
		Height(innerH).
		Render(body)

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)
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
