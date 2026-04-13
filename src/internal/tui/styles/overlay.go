package styles

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// OverlayCenter places fg centered over bg, both of which are fully rendered
// ANSI strings. termW and termH are the terminal dimensions. Lines in bg that
// fall within the fg bounding box are replaced (left and right flanks of the
// bg line are preserved). Returns the composited string.
func OverlayCenter(bg, fg string, termW, termH int) string {
	fgLines := strings.Split(fg, "\n")
	fgHeight := len(fgLines)

	// Determine the visual width of the widest fg line.
	fgWidth := 0
	for _, line := range fgLines {
		if w := ansi.StringWidth(line); w > fgWidth {
			fgWidth = w
		}
	}

	startX := (termW - fgWidth) / 2
	startY := (termH - fgHeight) / 2

	// If the modal is larger than the terminal in either dimension,
	// fall back to returning fg placed centrally via lipgloss.
	if startX < 0 || startY < 0 {
		return lipgloss.Place(termW, termH, lipgloss.Center, lipgloss.Center, fg)
	}

	bgLines := strings.Split(bg, "\n")

	// Pad bg with empty lines so the overlay region is always reachable.
	for len(bgLines) < startY+fgHeight {
		bgLines = append(bgLines, "")
	}

	result := make([]string, len(bgLines))
	for i, bgLine := range bgLines {
		if i >= startY && i < startY+fgHeight {
			fgLine := fgLines[i-startY]
			bgW := ansi.StringWidth(bgLine)

			// Left flank: keep the first startX visual columns of bgLine.
			var left string
			if bgW <= startX {
				left = bgLine + strings.Repeat(" ", startX-bgW)
			} else {
				left = ansi.Truncate(bgLine, startX, "")
			}

			// Right flank: keep everything after startX+fgWidth.
			var right string
			if rightStart := startX + fgWidth; bgW > rightStart {
				right = ansi.Cut(bgLine, rightStart, bgW)
			}

			result[i] = left + fgLine + right
		} else {
			result[i] = bgLine
		}
	}

	return strings.Join(result, "\n")
}
