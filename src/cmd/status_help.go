package cmd

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/leberkas-org/maggus/internal/tui/styles"
)

// helpSection groups related keyboard shortcuts for the help modal.
type helpSection struct {
	title string
	rows  []helpRow
}

// helpRow describes a single key binding entry in the help modal.
type helpRow struct {
	key  string
	desc string
}

// statusHelpSections is the ordered list of keyboard shortcut sections shown
// in the F1 help popup.
var statusHelpSections = []helpSection{
	{
		title: "Navigation",
		rows: []helpRow{
			{"↑ / ↓", "Move selection up / down"},
			{"pgup / pgdn", "Previous / next feature"},
			{"enter", "Open task details"},
			{"esc", "Close / go back"},
			{"q", "Quit status view"},
		},
	},
	{
		title: "Tabs",
		rows: []helpRow{
			{"tab", "Next tab"},
			{"shift+tab", "Previous tab"},
			{"1-5", "Jump to tab by number"},
		},
	},
	{
		title: "Output",
		rows: []helpRow{
			{"shift+↑", "Scroll output up"},
			{"shift+↓", "Scroll output down"},
			{"g", "Jump to top"},
			{"G", "Jump to bottom"},
		},
	},
	{
		title: "Actions",
		rows: []helpRow{
			{"a", "Approve / unapprove feature"},
			{"alt+d", "Delete feature"},
			{"x", "Skip / unskip task"},
			{"b", "Unblock all blocked criteria"},
			{"e", "Open plan file in editor"},
			{"alt+r", "Run task now"},
			{"alt+a", "Toggle completed features"},
			{"alt+s", "Stop daemon after current task"},
		},
	},
	{
		title: "Daemon",
		rows: []helpRow{
			{"s", "Start / stop daemon"},
			{"ctrl+c", "Kill daemon immediately"},
		},
	},
}

// buildHelpModal returns a fully styled, border-framed modal string listing
// all status view keyboard shortcuts. The modal width is capped at
// min(width-8, 72) and height at height-6. When there are more content rows
// than fit inside the modal, the oldest rows are dropped from the top
// (simple top-truncation, no scroll bar).
func buildHelpModal(width, height int) string {
	// --- Compute modal dimensions ---
	modalWidth := width - 8
	if modalWidth > 72 {
		modalWidth = 72
	}
	if modalWidth < 20 {
		modalWidth = 20
	}
	modalHeight := height - 6
	if modalHeight < 4 {
		modalHeight = 4
	}

	// The lipgloss box style uses Padding(0, 1) (1 char padding each side)
	// and a 1-char border on each side. Total horizontal overhead = 4.
	// Width() sets the inner content width, so:
	//   total rendered width = innerW + 2 (padding) + 2 (border) = innerW + 4
	// Solve for innerW so total = modalWidth:
	innerW := modalWidth - 4
	if innerW < 10 {
		innerW = 10
	}

	// Border consumes one line top and one line bottom.
	innerH := modalHeight - 2
	if innerH < 2 {
		innerH = 2
	}

	// --- Define styles ---
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.Primary)
	sepStyle := lipgloss.NewStyle().Foreground(styles.Muted)
	sectionStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.Primary)
	keyStyle := lipgloss.NewStyle().Foreground(styles.Warning)
	descStyle := lipgloss.NewStyle().Foreground(styles.Muted)

	// --- Header: title + separator ---
	titleLine := titleStyle.Render(" Keyboard Shortcuts ")
	sepLine := sepStyle.Render(strings.Repeat("─", innerW))

	// --- Key column width: fixed at 14 visual columns ---
	// Longest key entry is "pgup / pgdn" (11 chars) or "shift+tab" (9 chars).
	// 14 columns gives comfortable alignment with a 2-column gap.
	const keyColW = 14

	// --- Build content lines ---
	var lines []string
	lines = append(lines, titleLine)
	lines = append(lines, sepLine)

	for i, sec := range statusHelpSections {
		if i > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, sectionStyle.Render(sec.title))
		for _, row := range sec.rows {
			keyRendered := keyStyle.Render(row.key)
			keyVisW := lipgloss.Width(keyRendered)
			pad := keyColW - keyVisW
			if pad < 0 {
				pad = 0
			}
			descRendered := descStyle.Render(row.desc)
			lines = append(lines, keyRendered+strings.Repeat(" ", pad)+descRendered)
		}
	}

	// --- Top-truncate to fit innerH ---
	// Drop lines from the top when content overflows (no scroll bar needed
	// in the first iteration).
	if len(lines) > innerH {
		lines = lines[len(lines)-innerH:]
	}

	content := strings.Join(lines, "\n")

	// --- Render bordered box ---
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.Primary).
		Padding(0, 1).
		Width(innerW)

	return boxStyle.Render(content)
}
