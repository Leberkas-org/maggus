package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/leberkas-org/maggus/internal/parser"
	"github.com/leberkas-org/maggus/internal/runlog"
	"github.com/leberkas-org/maggus/internal/tui/styles"
)

// renderFeatureOutputTab renders the Output tab when a feature/bug plan row is selected.
// It builds a combined flat list of all task headers and tool lines, then applies the
// single logScroll offset across the entire list. Each task gets a separator header
// followed by its tool entries (indented 2 spaces). For the currently running task the
// live snapshot is used instead of the cache; the latest tool entry shows the spinner.
// Returns a placeholder when no tasks have any output and none is running.
func (m statusModel) renderFeatureOutputTab(width, contentH int) string {
	plan := m.selectedPlan()

	contentWidth := width - 4
	if contentWidth < 20 {
		contentWidth = 20
	}

	// Quick scan: determine whether there is anything to display.
	hasAnyOutput := false
	anyRunning := false
	for i, task := range plan.Tasks {
		if m.isTaskRunning(task.ID) {
			anyRunning = true
		} else if i < len(m.cachedFeatureOutput) && m.cachedFeatureOutput[i] != nil {
			hasAnyOutput = true
		}
	}
	if !hasAnyOutput && !anyRunning {
		return statusDimStyle.Render("  No output history")
	}

	// Build the combined flat list: header + tool lines per task.
	var allLines []string
	for i, task := range plan.Tasks {
		var snap *runlog.StateSnapshot
		spinnerFrame := 0
		isRunning := m.isTaskRunning(task.ID)

		if isRunning {
			// Same lookup logic as snapshotForSelectedTask but for the feature.
			if ws, ok := m.workerSnapshots[task.ID]; ok {
				snap = ws
			} else if m.snapshot != nil && m.snapshot.TaskID == task.ID {
				snap = m.snapshot
			}
			spinnerFrame = m.spinnerFrameForTask(task.ID)
		} else if i < len(m.cachedFeatureOutput) {
			snap = m.cachedFeatureOutput[i]
		}

		// Separator header for this task.
		allLines = append(allLines, buildFeatureTaskHeader(task, snap, isRunning, width))

		// Tool entries below the header (pending/blocked tasks with no data show header only).
		if snap != nil && len(snap.ToolEntries) > 0 {
			entries := make([]runlog.SnapshotToolEntry, len(snap.ToolEntries))
			copy(entries, snap.ToolEntries)
			// For the running task: latest tool entry gets the spinner character.
			if isRunning {
				last := len(entries) - 1
				spinChar := statusCyanStyle.Render(styles.SpinnerFrames[spinnerFrame%len(styles.SpinnerFrames)])
				entries[last].Icon = spinChar
			}
			allLines = append(allLines, buildToolLines(entries, contentWidth)...)
		}
	}

	var sb strings.Builder
	m.renderScrollableToolList(&sb, allLines, len(allLines), contentH)
	return sb.String()
}

// buildFeatureTaskHeader builds the separator header line for one task in the feature
// output tab. Format: ─── TASK-ID: Title <icon> [tokens/out] [$cost] [duration] ──────
// Icon: ✓ green for done, ▶ yellow for running, ○ dim for pending/blocked.
// Token, cost, and duration fields are only shown when non-zero/available.
func buildFeatureTaskHeader(task parser.Task, snap *runlog.StateSnapshot, isRunning bool, width int) string {
	dimStyle := lipgloss.NewStyle().Faint(true)
	warnStyle := lipgloss.NewStyle().Foreground(styles.Warning)

	// Status icon.
	var icon string
	if isRunning {
		icon = warnStyle.Render("▶")
	} else if task.IsComplete() {
		icon = statusGreenStyle.Render("✓")
	} else {
		icon = dimStyle.Render("○")
	}

	// Metadata from the snapshot (tokens, cost, duration).
	var metaParts []string
	if snap != nil {
		if snap.TokenInput > 0 || snap.TokenOutput > 0 {
			metaParts = append(metaParts, fmt.Sprintf("[%s/%s]",
				FormatTokens(snap.TokenInput), FormatTokens(snap.TokenOutput)))
		}
		if snap.TokenCost > 0 {
			metaParts = append(metaParts, fmt.Sprintf("[%s]", FormatCost(snap.TokenCost)))
		}
		if !isRunning && snap.TaskStartedAt != "" && snap.UpdatedAt != "" {
			t1, e1 := time.Parse(time.RFC3339, snap.TaskStartedAt)
			t2, e2 := time.Parse(time.RFC3339, snap.UpdatedAt)
			if e1 == nil && e2 == nil && t2.After(t1) {
				metaParts = append(metaParts, fmt.Sprintf("[%s]", formatHumanDuration(t2.Sub(t1))))
			}
		}
	}
	meta := ""
	if len(metaParts) > 0 {
		meta = " " + dimStyle.Render(strings.Join(metaParts, " "))
	}

	// Compute how much width is available for the task title.
	taskIDStr := statusCyanStyle.Render(task.ID)
	const leftSepW = 4   // "─── "
	const rightSepMinW = 4 // " ───"
	// center = " " + taskID + ": " + title + " " + icon + meta
	fixedW := leftSepW + 1 + lipgloss.Width(taskIDStr) + 2 + 1 + lipgloss.Width(icon) + lipgloss.Width(meta) + rightSepMinW
	titleMaxW := width - fixedW
	if titleMaxW < 1 {
		titleMaxW = 1
	}
	title := truncateToWidth(task.Title, titleMaxW)

	// Assemble center content and compute right fill.
	center := fmt.Sprintf(" %s: %s %s%s", taskIDStr, title, icon, meta)
	rightFillW := width - leftSepW - lipgloss.Width(center)
	if rightFillW < 3 {
		rightFillW = 3
	}
	rightSep := " " + strings.Repeat("─", rightFillW-1)

	return dimStyle.Render("─── ") + center + dimStyle.Render(rightSep)
}
