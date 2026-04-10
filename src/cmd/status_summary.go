package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/leberkas-org/maggus/internal/parser"
	"github.com/leberkas-org/maggus/internal/runlog"
	"github.com/leberkas-org/maggus/internal/tui/styles"
)

// renderSummaryTab renders the Summary tab content for feature and task selections.
// It is never called for selNone or selRunningTask — those contexts do not include
// the Summary tab in availableTabs().
func (m statusModel) renderSummaryTab(width, height int) string {
	switch m.selectionCtx() {
	case selFeature:
		return m.renderFeatureSummary(width, height)
	case selCompletedTask:
		return m.renderTaskSummary(width, height)
	default:
		return lipgloss.NewStyle().Width(width).Height(height).Render("")
	}
}

// renderFeatureSummary renders the Summary tab when a feature or bug plan is selected.
// Shows title, progress bar, task breakdown, daemon state, parallel workers, and aggregate metrics.
func (m statusModel) renderFeatureSummary(width, height int) string {
	plan := m.selectedPlan()

	labelStyle := lipgloss.NewStyle().Foreground(styles.Muted)
	valueStyle := lipgloss.NewStyle().Bold(true)
	sectionStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.Primary)

	var sb strings.Builder

	// ── Header: title and filename ──
	sb.WriteString("\n")
	title := plan.Title
	if title == "" {
		title = plan.ID
	}
	titleRendered := statusBoldStyle.Render(styles.Truncate("  "+title, width-2))
	sb.WriteString(titleRendered + "\n")
	sb.WriteString(statusDimStyle.Render("  "+filepath.Base(plan.File)) + "\n")
	sb.WriteString("\n")

	// ── Progress bar ──
	done := plan.DoneCount()
	total := len(plan.Tasks)
	bar := buildProgressBar(done, total)
	count := statusDimStyle.Render(fmt.Sprintf(" %d/%d", done, total))
	sb.WriteString("  " + bar + count + "\n")
	sb.WriteString("\n")

	// ── Task breakdown ──
	pending := 0
	blocked := plan.BlockedCount()
	for _, t := range plan.Tasks {
		if !t.IsComplete() && !t.IsBlocked() {
			pending++
		}
	}

	sb.WriteString(summaryRow(labelStyle, valueStyle, "  Done", fmt.Sprintf("%d", done)))
	sb.WriteString(summaryRow(labelStyle, valueStyle, "  Pending", fmt.Sprintf("%d", pending)))
	sb.WriteString(summaryRow(labelStyle, valueStyle, "  Blocked", fmt.Sprintf("%d", blocked)))

	// ── Active daemon state (running on this feature) ──
	activeTaskID, elapsed, spinnerStr := m.featureDaemonState(plan)
	if activeTaskID != "" {
		sb.WriteString("\n")
		sb.WriteString(" " + styles.Separator(width-1) + "\n")
		indicator := fmt.Sprintf("  %s  %s", spinnerStr, statusCyanStyle.Render(activeTaskID))
		if elapsed != "" {
			indicator += "   " + statusDimStyle.Render(elapsed)
		}
		sb.WriteString(indicator + "\n")
	}

	// ── Parallel workers for this feature ──
	featureWorkers := m.featureWorkers(plan)
	if len(featureWorkers) > 0 {
		if activeTaskID == "" {
			sb.WriteString("\n")
			sb.WriteString(" " + styles.Separator(width-1) + "\n")
		}
		sb.WriteString(sectionStyle.Render("  Workers") + "\n")
		for _, w := range featureWorkers {
			icon, wStyle := workerStatusIndicator(w.Status, m.workerSpinners[w.TaskID])
			line := fmt.Sprintf("  %s %s", icon, wStyle.Render(w.TaskID))
			if w.TaskTitle != "" {
				line += " " + statusDimStyle.Render(styles.Truncate(w.TaskTitle, width-lipgloss.Width(line)-4))
			}
			sb.WriteString(line + "\n")
		}
	}

	// ── Aggregate metrics ──
	fm := m.cachedFeatureMetrics
	if fm.totalTokens > 0 || fm.totalCostUSD > 0 {
		sb.WriteString("\n")
		sb.WriteString(" " + styles.Separator(width-1) + "\n")
		sb.WriteString(summaryRow(labelStyle, valueStyle, "  Tokens", FormatTokens(int(fm.totalTokens))))
		sb.WriteString(summaryRow(labelStyle, valueStyle, "  Cost", FormatCost(fm.totalCostUSD)))
	}

	return lipgloss.NewStyle().Width(width).Height(height).Render(sb.String())
}

// renderTaskSummary renders the Summary tab when a task (completed, pending, or blocked) is selected.
// Shows task ID, title, outcome, duration, token usage, cost, and commit hash if available.
func (m statusModel) renderTaskSummary(width, height int) string {
	task := m.selectedTask()
	if task == nil {
		return lipgloss.NewStyle().Width(width).Height(height).Render("")
	}

	labelStyle := lipgloss.NewStyle().Foreground(styles.Muted)
	valueStyle := lipgloss.NewStyle().Bold(true)

	var sb strings.Builder
	sb.WriteString("\n")

	// ── Task ID and title ──
	sb.WriteString(statusBoldStyle.Render("  "+task.ID) + "\n")
	if task.Title != "" {
		sb.WriteString(statusDimStyle.Render("  "+styles.Truncate(task.Title, width-4)) + "\n")
	}
	sb.WriteString("\n")

	// ── Status ──
	statusStr, statusRendered := taskStatusDisplay(task)
	sb.WriteString(summaryRow(labelStyle, statusRendered, "  Status", statusStr))

	// ── Metrics from cached task data ──
	tm := m.cachedTaskMetrics
	if tm.durationSecs > 0 {
		sb.WriteString(summaryRow(labelStyle, valueStyle, "  Duration", formatDurationSecs(tm.durationSecs)))
	} else {
		sb.WriteString(summaryRow(labelStyle, lipgloss.NewStyle().Foreground(styles.Muted), "  Duration", "—"))
	}

	if tm.inputTokens > 0 || tm.outputTokens > 0 {
		tokenStr := fmt.Sprintf("%s in / %s out",
			FormatTokens(int(tm.inputTokens)), FormatTokens(int(tm.outputTokens)))
		sb.WriteString(summaryRow(labelStyle, valueStyle, "  Tokens", tokenStr))
		sb.WriteString(summaryRow(labelStyle, valueStyle, "  Cost", FormatCost(tm.costUSD)))
		if tm.model != "" {
			short := tm.model
			if idx := strings.LastIndex(tm.model, "/"); idx >= 0 {
				short = tm.model[idx+1:]
			}
			sb.WriteString(summaryRow(labelStyle, valueStyle, "  Model", short))
		}
	} else {
		sb.WriteString(summaryRow(labelStyle, lipgloss.NewStyle().Foreground(styles.Muted), "  Tokens", "—"))
		sb.WriteString(summaryRow(labelStyle, lipgloss.NewStyle().Foreground(styles.Muted), "  Cost", "—"))
	}

	// ── Commit hash (if available from run logs) ──
	if task.IsComplete() {
		commit := loadTaskCommit(m.dir, task.ID)
		if commit != "" {
			short := commit
			if len(short) > 7 {
				short = short[:7]
			}
			sb.WriteString(summaryRow(labelStyle, valueStyle, "  Commit", short))
		}
	}

	return lipgloss.NewStyle().Width(width).Height(height).Render(sb.String())
}

// featureDaemonState returns the active task ID, formatted elapsed time, and rendered
// spinner string when the daemon is currently working on a task in the given plan.
// Returns empty strings when the daemon is idle or working on a different feature.
func (m statusModel) featureDaemonState(plan parser.Plan) (taskID, elapsed, spinnerStr string) {
	if !m.daemon.Running {
		return "", "", ""
	}
	// Find if any task in this plan is running.
	for _, t := range plan.Tasks {
		if m.isTaskRunning(t.ID) {
			taskID = t.ID
			break
		}
	}
	if taskID == "" {
		return "", "", ""
	}

	spinnerStr = statusCyanStyle.Render(styles.SpinnerFrames[m.spinnerFrame])

	// Try to get elapsed time from the snapshot.
	if m.snapshot != nil && m.snapshot.TaskStartedAt != "" {
		if t, err := time.Parse(time.RFC3339, m.snapshot.TaskStartedAt); err == nil {
			elapsed = formatHumanDuration(time.Since(t))
		}
	}
	// For parallel mode, check per-worker snapshot.
	if elapsed == "" && m.workerSnapshots != nil {
		if snap, ok := m.workerSnapshots[taskID]; ok && snap != nil && snap.TaskStartedAt != "" {
			if t, err := time.Parse(time.RFC3339, snap.TaskStartedAt); err == nil {
				elapsed = formatHumanDuration(time.Since(t))
			}
		}
	}

	return taskID, elapsed, spinnerStr
}

// featureWorkers returns the worker index entries that belong to the given plan
// (i.e., whose task IDs correspond to tasks in plan.Tasks).
func (m statusModel) featureWorkers(plan parser.Plan) []runlog.WorkerIndexEntry {
	if len(m.workerIndex) == 0 {
		return nil
	}
	taskSet := make(map[string]bool, len(plan.Tasks))
	for _, t := range plan.Tasks {
		taskSet[t.ID] = true
	}
	var result []runlog.WorkerIndexEntry
	for _, w := range m.workerIndex {
		if taskSet[w.TaskID] {
			result = append(result, w)
		}
	}
	return result
}

// taskStatusDisplay returns the display string and style for a task's current status.
func taskStatusDisplay(task *parser.Task) (string, lipgloss.Style) {
	if task.IsComplete() {
		return "✓ Complete", statusGreenStyle
	}
	if task.IsBlocked() {
		return "⚠ Blocked", statusRedStyle
	}
	return "○ Pending", lipgloss.NewStyle().Foreground(styles.Muted)
}

// loadTaskCommit scans run log JSONL files in .maggus/runs/ for a task_complete
// entry matching taskID. Returns the commit hash from the most recent matching entry,
// or "" if none is found.
func loadTaskCommit(dir, taskID string) string {
	runsDir := filepath.Join(dir, ".maggus", "runs")
	entries, err := os.ReadDir(runsDir)
	if err != nil {
		return ""
	}

	var logFiles []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".log") && e.Name() != "daemon.log" {
			logFiles = append(logFiles, e.Name())
		}
	}
	// Sort descending (most recent first) — timestamp-prefixed names sort chronologically.
	sort.Sort(sort.Reverse(sort.StringSlice(logFiles)))

	for _, name := range logFiles {
		commit := scanLogFileForTaskCommit(filepath.Join(runsDir, name), taskID)
		if commit != "" {
			return commit
		}
	}
	return ""
}

// scanLogFileForTaskCommit reads a single JSONL log file and returns the commit
// hash from the last task_complete entry matching taskID.
func scanLogFileForTaskCommit(path, taskID string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	var commit string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var entry runlog.Entry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue
		}
		if entry.Event == "task_complete" && entry.TaskID == taskID && entry.Commit != "" {
			commit = entry.Commit // keep last match
		}
	}
	return commit
}

// summaryRow renders a label + value row in the Summary tab, styled to align
// consistently with the metricsRow helper.
func summaryRow(labelStyle lipgloss.Style, valueStyle lipgloss.Style, label, value string) string {
	const labelWidth = 12
	padded := label
	if len(label) < labelWidth {
		padded = label + strings.Repeat(" ", labelWidth-len(label))
	}
	return labelStyle.Render(padded) + " " + valueStyle.Render(value) + "\n"
}
