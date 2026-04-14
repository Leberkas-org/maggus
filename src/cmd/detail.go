package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/leberkas-org/maggus/internal/parser"
	"github.com/leberkas-org/maggus/internal/tui/styles"
)

// detailState holds auxiliary state for the task detail view.
// Reserved for future use; criteria mode has been removed.
type detailState struct{}

// renderDetailContent builds the detail view content for a task.
func renderDetailContent(t parser.Task) string {
	var sb strings.Builder

	titleStyle := styles.Title
	labelStyle := styles.Label.Width(10).Align(lipgloss.Right)
	mutedStyle := lipgloss.NewStyle().Foreground(styles.Muted)
	successStyle := lipgloss.NewStyle().Foreground(styles.Success)
	warningStyle := lipgloss.NewStyle().Foreground(styles.Warning)

	sb.WriteString(titleStyle.Render(fmt.Sprintf("%s: %s", t.ID, t.Title)))
	sb.WriteString("\n\n")

	// Metadata
	sb.WriteString(fmt.Sprintf("%s  %s\n", labelStyle.Render("Plan:"), mutedStyle.Render(filepath.Base(t.SourceFile))))

	// Status
	var statusText string
	var statusStyle lipgloss.Style
	if t.IsComplete() && t.IsBlocked() {
		statusText = "Complete (blocked)"
		statusStyle = warningStyle
	} else if t.IsComplete() {
		statusText = "Complete"
		statusStyle = successStyle
	} else if t.IsBlocked() {
		statusText = "Blocked"
		statusStyle = warningStyle
	} else if t.IsSkipped() {
		statusText = "Skipped"
		statusStyle = mutedStyle
	} else {
		statusText = "Pending"
		statusStyle = mutedStyle
	}
	sb.WriteString(fmt.Sprintf("%s  %s\n", labelStyle.Render("Status:"), statusStyle.Render(statusText)))

	// Criteria counts
	done := 0
	blocked := 0
	for _, c := range t.Criteria {
		if c.Checked {
			done++
		}
		if c.Blocked {
			blocked++
		}
	}
	sb.WriteString(fmt.Sprintf("%s  %s\n", labelStyle.Render("Criteria:"),
		mutedStyle.Render(fmt.Sprintf("%d total, %d done, %d blocked", len(t.Criteria), done, blocked))))

	// Description
	if t.Description != "" {
		sb.WriteString("\n")
		sb.WriteString(styles.Subtitle.Render("Description"))
		sb.WriteString("\n")
		for _, line := range strings.Split(strings.TrimSpace(t.Description), "\n") {
			sb.WriteString("  " + line + "\n")
		}
	}

	// Acceptance criteria
	if len(t.Criteria) > 0 {
		sb.WriteString("\n")
		sb.WriteString(styles.Subtitle.Render("Acceptance Criteria"))
		sb.WriteString("\n")
		for _, c := range t.Criteria {
			var checkbox string
			if c.Blocked {
				// Blocked takes priority over Checked — [~] criteria are both
				// Checked=true and Blocked=true, but should display as blocked.
				checkbox = warningStyle.Render("⊘")
			} else if c.Checked {
				checkbox = successStyle.Render("✓")
			} else if c.Skipped {
				checkbox = mutedStyle.Render(">")
			} else {
				checkbox = mutedStyle.Render("○")
			}
			sb.WriteString(fmt.Sprintf("  %s %s\n", checkbox, c.Text))
		}
	}

	return sb.String()
}

// reloadTask re-parses a plan file and returns the task matching the given ID.
// Returns the updated task or nil if not found.
func reloadTask(sourceFile, taskID string) *parser.Task {
	tasks, err := parser.ParseFile(sourceFile)
	if err != nil {
		return nil
	}
	for _, t := range tasks {
		if t.ID == taskID {
			return &t
		}
	}
	return nil
}

// detailFooter returns the appropriate footer for the detail view state.
func detailFooter(scrollable bool) string {
	var parts []string
	if scrollable {
		parts = append(parts, "↑/↓: scroll")
	}
	parts = append(parts, "pgup/pgdn: prev/next task")
	parts = append(parts, "alt+r: run · alt+bksp: delete · q: back")
	return styles.StatusBar.Render(strings.Join(parts, " · "))
}
