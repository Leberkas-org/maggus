package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/leberkas-org/maggus/internal/parser"
	"github.com/leberkas-org/maggus/internal/tui/styles"
)

// criteriaAction represents the user's choice for a blocked criterion.
type criteriaAction int

const (
	criteriaActionUnblock criteriaAction = iota
	criteriaActionResolve
	criteriaActionDelete
	criteriaActionSkip
)

var criteriaActions = []criteriaAction{criteriaActionUnblock, criteriaActionResolve, criteriaActionDelete, criteriaActionSkip}

func (a criteriaAction) String() string {
	switch a {
	case criteriaActionUnblock:
		return "Unblock"
	case criteriaActionResolve:
		return "Resolve"
	case criteriaActionDelete:
		return "Delete"
	case criteriaActionSkip:
		return "Skip"
	}
	return ""
}

func (a criteriaAction) Description() string {
	switch a {
	case criteriaActionUnblock:
		return "Remove BLOCKED: prefix, keep unchecked"
	case criteriaActionResolve:
		return "Mark as done (remove block + check)"
	case criteriaActionDelete:
		return "Remove criterion entirely"
	case criteriaActionSkip:
		return "Do nothing"
	}
	return ""
}

// detailState holds the state for criteria mode in the task detail view.
type detailState struct {
	criteriaMode     bool
	criteriaCursor   int
	blockedIndices   []int // indices into task.Criteria that are blocked
	showActionPicker bool
	actionCursor     int
	noBlockedMsg     bool // briefly show "no blocked criteria" message
}

// initCriteriaMode sets up criteria mode for the given task.
// Returns false if the task has no blocked criteria.
func (d *detailState) initCriteriaMode(task parser.Task) bool {
	d.blockedIndices = nil
	for i, c := range task.Criteria {
		if c.Blocked {
			d.blockedIndices = append(d.blockedIndices, i)
		}
	}
	if len(d.blockedIndices) == 0 {
		return false
	}
	d.criteriaMode = true
	d.criteriaCursor = 0
	d.showActionPicker = false
	d.actionCursor = 0
	return true
}

// exitCriteriaMode returns to scroll mode.
func (d *detailState) exitCriteriaMode() {
	d.criteriaMode = false
	d.criteriaCursor = 0
	d.showActionPicker = false
	d.actionCursor = 0
	d.blockedIndices = nil
}

// performAction executes the selected action on the blocked criterion.
// Returns true if the plan file was modified (needs refresh).
func (d *detailState) performAction(task parser.Task, action criteriaAction) (modified bool, err error) {
	if d.criteriaCursor >= len(d.blockedIndices) {
		return false, nil
	}
	criterionIdx := d.blockedIndices[d.criteriaCursor]
	c := task.Criteria[criterionIdx]

	switch action {
	case criteriaActionUnblock:
		if err := parser.UnblockCriterion(task.SourceFile, c); err != nil {
			return false, err
		}
		return true, nil
	case criteriaActionResolve:
		if err := parser.ResolveCriterion(task.SourceFile, c); err != nil {
			return false, err
		}
		return true, nil
	case criteriaActionDelete:
		if err := parser.DeleteCriterion(task.SourceFile, c); err != nil {
			return false, err
		}
		return true, nil
	case criteriaActionSkip:
		return false, nil
	}
	return false, nil
}

// renderDetailContent builds the detail view content for a task, with optional
// criteria mode highlighting.
func renderDetailContent(t parser.Task, ds *detailState) string {
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
	if t.IsComplete() {
		statusText = "Complete"
		statusStyle = successStyle
	} else if t.IsBlocked() {
		statusText = "Blocked"
		statusStyle = warningStyle
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
		for i, c := range t.Criteria {
			var checkbox string
			if c.Checked {
				checkbox = successStyle.Render("✓")
			} else if c.Blocked {
				checkbox = warningStyle.Render("⊘")
			} else {
				checkbox = mutedStyle.Render("○")
			}

			// In criteria mode, highlight the selected blocked criterion
			if ds != nil && ds.criteriaMode && c.Blocked {
				blockedIdx := -1
				for bi, idx := range ds.blockedIndices {
					if idx == i {
						blockedIdx = bi
						break
					}
				}
				if blockedIdx == ds.criteriaCursor {
					if ds.showActionPicker {
						// Show action picker inline below this criterion
						sb.WriteString(fmt.Sprintf("  %s %s\n", warningStyle.Render("▸"), lipgloss.NewStyle().Bold(true).Foreground(styles.Warning).Render(c.Text)))
						sb.WriteString(renderInlineActionPicker(ds.actionCursor))
					} else {
						sb.WriteString(fmt.Sprintf("  %s %s\n", warningStyle.Render("▸"), lipgloss.NewStyle().Bold(true).Foreground(styles.Warning).Render(c.Text)))
					}
				} else {
					sb.WriteString(fmt.Sprintf("  %s %s\n", checkbox, c.Text))
				}
			} else {
				sb.WriteString(fmt.Sprintf("  %s %s\n", checkbox, c.Text))
			}
		}
	}

	// Show no-blocked message if applicable
	if ds != nil && ds.noBlockedMsg {
		sb.WriteString("\n")
		sb.WriteString(fmt.Sprintf("  %s\n", mutedStyle.Render("No blocked criteria to manage")))
	}

	return sb.String()
}

// renderInlineActionPicker renders the action picker inline.
func renderInlineActionPicker(cursor int) string {
	successStyle := lipgloss.NewStyle().Foreground(styles.Success)
	warningStyle := lipgloss.NewStyle().Foreground(styles.Warning)
	errorStyle := lipgloss.NewStyle().Foreground(styles.Error)
	mutedStyle := lipgloss.NewStyle().Foreground(styles.Muted)

	var sb strings.Builder
	for i, a := range criteriaActions {
		prefix := "  "
		if i == cursor {
			prefix = "> "
		}
		var label string
		switch a {
		case criteriaActionUnblock:
			label = successStyle.Render(a.String())
		case criteriaActionResolve:
			label = warningStyle.Render(a.String())
		case criteriaActionDelete:
			label = errorStyle.Render(a.String())
		default:
			label = mutedStyle.Render(a.String())
		}
		desc := mutedStyle.Render(" " + a.Description())
		sb.WriteString(fmt.Sprintf("      %s%s%s\n", prefix, label, desc))
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
func detailFooter(ds *detailState, scrollable bool) string {
	if ds != nil && ds.criteriaMode {
		if ds.showActionPicker {
			return styles.StatusBar.Render("↑/↓: select action · enter: confirm · esc: cancel")
		}
		return styles.StatusBar.Render("↑/↓: navigate blocked · enter: action · tab: scroll mode · q: back")
	}

	var parts []string
	if scrollable {
		parts = append(parts, "↑/↓: scroll")
	}
	parts = append(parts, "pgup/pgdn: prev/next task")
	parts = append(parts, "tab: manage blocked")
	parts = append(parts, "alt+r: run · alt+bksp: delete · q: back")
	return styles.StatusBar.Render(strings.Join(parts, " · "))
}
