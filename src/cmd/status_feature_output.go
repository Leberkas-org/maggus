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
//
// Each task is allocated a fixed line budget (linesPerTask = max(contentH/taskCount, 7));
// only the last linesPerTask entries are rendered per task, but the full entry count is
// always shown in the task header as "[N tools]".
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

	// Per-task line budget: divide available height evenly, minimum 7 lines per task.
	taskCount := len(plan.Tasks)
	if taskCount < 1 {
		taskCount = 1
	}
	linesPerTask := contentH / taskCount
	if linesPerTask < 7 {
		linesPerTask = 7
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

		// Retain the full entry count for the header regardless of display capping.
		totalTools := 0
		if snap != nil {
			totalTools = len(snap.ToolEntries)
		}

		// Separator header for this task — always includes "[N tools]" when totalTools > 0.
		allLines = append(allLines, buildFeatureTaskHeader(task, snap, isRunning, width, totalTools))

		// Tool entries below the header (pending/blocked tasks with no data show header only).
		if snap != nil && len(snap.ToolEntries) > 0 {
			entries := snap.ToolEntries
			// Cap to last linesPerTask entries; retain full count in header via totalTools.
			start := len(entries) - linesPerTask
			if start < 0 {
				start = 0
			}
			entries = entries[start:]

			capped := make([]runlog.SnapshotToolEntry, len(entries))
			copy(capped, entries)
			// For the running task: latest tool entry gets the spinner character.
			if isRunning {
				last := len(capped) - 1
				spinChar := statusCyanStyle.Render(styles.SpinnerFrames[spinnerFrame%len(styles.SpinnerFrames)])
				capped[last].Icon = spinChar
			}
			allLines = append(allLines, buildToolLines(capped, contentWidth)...)
		}
	}

	var sb strings.Builder
	m.renderScrollableToolList(&sb, allLines, len(allLines), contentH)
	return sb.String()
}

// buildFeatureTaskHeader builds the separator header line for one task in the feature
// output tab. Format: ─── TASK-ID: Title <icon> [tokens/out] [$cost] [duration] [N tools] ──
// Icon: ✓ green for done, ▶ yellow for running, ○ dim for pending/blocked.
// Token, cost, duration, and tool count fields are only shown when non-zero/available.
// totalTools is the full entry count for the task — shown as "[N tools]" when > 0.
func buildFeatureTaskHeader(task parser.Task, snap *runlog.StateSnapshot, isRunning bool, width, totalTools int) string {
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

	// Metadata from the snapshot (tokens, cost, duration, tool count).
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
	if totalTools > 0 {
		metaParts = append(metaParts, fmt.Sprintf("[%d tools]", totalTools))
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
