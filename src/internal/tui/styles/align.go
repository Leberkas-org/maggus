package styles

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// RightAlign places left on the left and right on the right within width columns,
// padding the gap between them with spaces. If there is no room for right (pad < 1),
// left is returned unchanged and right is silently dropped.
func RightAlign(left, right string, width int) string {
	pad := width - lipgloss.Width(left) - lipgloss.Width(right)
	if pad < 1 {
		return left
	}
	return left + strings.Repeat(" ", pad) + right
}
