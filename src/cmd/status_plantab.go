package cmd

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/leberkas-org/maggus/internal/parser"
	"github.com/leberkas-org/maggus/internal/tui/styles"
)

// renderPlanTab renders the Plan tab content showing the execution order for the
// selected feature or bug plan. Each execution step is shown as a labelled header
// ("Step N" or "Step N (parallel)") followed by a row per task.
//
// Task rows show:
//   - ✓  completed
//   - spinner char  running (daemon is actively working on this task)
//   - ⚠  blocked
//   - >  skipped
//   - ○  pending
//
// Unresolvable tasks (unknown predecessors) appear in a final "(unresolved)" step.
// The estimated token total (sum of all task TokenEstimate fields) is shown at the
// bottom when non-zero. The content is scrollable via planTabScroll.
func (m statusModel) renderPlanTab(width, height int) string {
	plan := m.selectedPlan()
	if len(plan.Tasks) == 0 {
		mutedStyle := lipgloss.NewStyle().Foreground(styles.Muted)
		msg := mutedStyle.Render("  No tasks")
		return lipgloss.NewStyle().Width(width).Height(height).Render(msg)
	}

	steps := buildExecutionPlan(plan.Tasks)

	// Build a task lookup map.
	taskByID := make(map[string]parser.Task, len(plan.Tasks))
	for _, t := range plan.Tasks {
		taskByID[t.ID] = t
	}

	stepHeaderStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.Primary)
	unresolvedHeaderStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.Warning)

	// Build all renderable lines up-front so we can apply scroll.
	var lines []string
	for _, step := range steps {
		label := fmt.Sprintf("Phase %d", step.StepNumber)
		if step.Parallel {
			label += " (parallel)"
		}
		if step.Unresolved {
			lines = append(lines, unresolvedHeaderStyle.Render(" "+label+" (unresolved)"))
		} else {
			lines = append(lines, stepHeaderStyle.Render(" "+label))
		}

		for _, taskID := range step.TaskIDs {
			t := taskByID[taskID]
			icon := m.planTabTaskIcon(t)
			maxTitleW := width - 24
			if maxTitleW < 10 {
				maxTitleW = 10
			}
			title := styles.Truncate(t.Title, maxTitleW)
			wtBadge := ""
			if t.Parallel {
				wtBadge = " 🌲🪓"
			}
			row := fmt.Sprintf("   %s  %s  %s%s", icon, taskID, statusDimStyle.Render(title), wtBadge)
			lines = append(lines, row)
		}

		// Blank separator between steps.
		lines = append(lines, "")
	}

	// Sum token estimates across all tasks.
	totalTokens := 0
	for _, t := range plan.Tasks {
		totalTokens += t.TokenEstimate
	}

	// Footer (always rendered below the scrollable area).
	var footerLines []string
	if totalTokens > 0 {
		footerLines = append(footerLines, " "+styles.Separator(width-2))
		tokenStr := fmt.Sprintf("~%s", FormatTokens(totalTokens))
		footerLines = append(footerLines,
			statusDimStyle.Render("  Estimated tokens: ")+statusBoldStyle.Render(tokenStr))
	}

	footerH := len(footerLines)
	contentH := height - footerH
	if contentH < 1 {
		contentH = 1
	}

	// Apply scroll offset, clamped to valid range.
	offset := m.planTabScroll
	if offset < 0 || offset >= len(lines) {
		offset = 0
	}
	visible := lines[offset:]
	if len(visible) > contentH {
		visible = visible[:contentH]
	}

	// Build the final string: visible content + footer.
	var sb strings.Builder
	for _, line := range visible {
		sb.WriteString(line + "\n")
	}
	if len(footerLines) > 0 {
		sb.WriteString(strings.Join(footerLines, "\n"))
	}

	return lipgloss.NewStyle().Width(width).Height(height).Render(sb.String())
}

// planTabTaskIcon returns the styled status icon for a task row in the Plan tab.
func (m statusModel) planTabTaskIcon(t parser.Task) string {
	if t.IsComplete() && t.IsBlocked() {
		return lipgloss.NewStyle().Foreground(styles.Warning).Render("⊘")
	}
	if t.IsComplete() {
		return statusGreenStyle.Render("✓")
	}
	if m.isTaskRunning(t.ID) {
		frame := styles.SpinnerFrames[m.spinnerFrame%len(styles.SpinnerFrames)]
		return statusCyanStyle.Render(frame)
	}
	if t.IsBlocked() {
		return statusRedStyle.Render("⚠")
	}
	if t.IsSkipped() {
		return lipgloss.NewStyle().Foreground(styles.Muted).Faint(true).Render(">")
	}
	return lipgloss.NewStyle().Foreground(styles.Muted).Render("○")
}
