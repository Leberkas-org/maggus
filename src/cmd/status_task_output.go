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

// ── Shared tool-line rendering helpers ──────────────────────────────────────

// truncateToWidth truncates a plain-text string to fit within maxWidth visible
// columns, appending "..." if truncated. Column widths are measured via
// lipgloss.Width so wide characters (CJK, emoji) are counted correctly.
func truncateToWidth(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= maxWidth {
		return s
	}
	if maxWidth <= 3 {
		// Not enough room for ellipsis; hard-cut at maxWidth columns.
		var out []rune
		w := 0
		for _, r := range s {
			rw := lipgloss.Width(string(r))
			if w+rw > maxWidth {
				break
			}
			out = append(out, r)
			w += rw
		}
		return string(out)
	}
	var out []rune
	w := 0
	for _, r := range s {
		rw := lipgloss.Width(string(r))
		if w+rw > maxWidth-3 {
			break
		}
		out = append(out, r)
		w += rw
	}
	return string(out) + "..."
}

// buildToolLines renders snapshot tool entries into styled strings for display.
// Extracted from renderSnapshotInPane so both running and completed task views
// share the same formatting logic.
func buildToolLines(entries []runlog.SnapshotToolEntry, contentWidth int) []string {
	toolLines := make([]string, len(entries))
	for i, entry := range entries {
		ts := entry.Timestamp
		if t, err := time.Parse(time.RFC3339, entry.Timestamp); err == nil {
			ts = t.Local().Format("15:04:05")
		}
		icon := entry.Icon
		if icon == "" {
			icon = "🥚"
		}
		typeName := entry.Type
		if typeName == "" {
			typeName = "Tool"
		}
		bracketedType := fmt.Sprintf("[%s]", typeName)
		styledTs := statusDimStyle.Render(ts)
		tsW := lipgloss.Width(styledTs)
		iconW := lipgloss.Width(icon)
		bracketedTypeW := lipgloss.Width(bracketedType)
		fixedCols := 2 + iconW + 1 + bracketedTypeW + 1 + 1 + tsW
		maxDesc := contentWidth - fixedCols
		if maxDesc < 0 {
			maxDesc = 0
		}
		desc := truncateToWidth(entry.Description, maxDesc)
		leftPart := fmt.Sprintf("  %s %s %s", icon, statusBlueStyle.Render(bracketedType), statusDimStyle.Render(desc))
		toolLines[i] = styles.RightAlign(leftPart, styledTs, contentWidth)
	}
	return toolLines
}

// renderScrollableToolList writes a scrollable tool list into sb.
// It handles scroll offset, indicators, and padding for both running and completed views.
func (m statusModel) renderScrollableToolList(sb *strings.Builder, toolLines []string, totalTools, available int) {
	if totalTools == 0 {
		sb.WriteString(statusDimStyle.Render("  No tool invocations recorded.") + "\n")
		for i := 1; i < available; i++ {
			sb.WriteString("\n")
		}
		return
	}

	offset := m.logScroll
	maxOffset := totalTools - available
	if maxOffset < 0 {
		maxOffset = 0
	}
	if offset > maxOffset {
		offset = maxOffset
	}
	if offset < 0 {
		offset = 0
	}

	if totalTools > available {
		end := offset + available
		if end > totalTools {
			end = totalTools
		}
		indicator := statusDimStyle.Render(fmt.Sprintf("[%d-%d of %d]", offset+1, end, totalTools))
		if m.logAutoScroll {
			indicator += statusDimStyle.Render(" (auto)")
		}
		sb.WriteString(indicator + "\n")
		viewH := available - 1
		if viewH < 1 {
			viewH = 1
		}
		end = offset + viewH
		if end > totalTools {
			end = totalTools
		}
		for _, line := range toolLines[offset:end] {
			sb.WriteString(line + "\n")
		}
		rendered := end - offset
		for i := rendered; i < viewH; i++ {
			sb.WriteString("\n")
		}
	} else {
		for _, line := range toolLines {
			sb.WriteString(line + "\n")
		}
		for i := totalTools; i < available; i++ {
			sb.WriteString("\n")
		}
	}
}

// ── Snapshot selection helpers ───────────────────────────────────────────────

// snapshotForSelectedTask returns the live StateSnapshot for the currently
// selected running task. Per-worker snapshots (from parallel orchestrator or
// dispatched workers) take precedence; falls back to the main daemon snapshot
// only when its TaskID matches the selected task.
func (m statusModel) snapshotForSelectedTask() *runlog.StateSnapshot {
	task := m.selectedTask()
	if task == nil {
		return nil
	}
	// Per-worker snapshot takes precedence (parallel orchestrator or dispatched workers).
	if snap, ok := m.workerSnapshots[task.ID]; ok {
		return snap
	}
	// Sequential daemon mode: only return the main snapshot when its TaskID
	// matches the selected task. A mismatched TaskID means the snapshot belongs
	// to a different task (e.g. stale data from a previous run) and must not be shown.
	if m.snapshot != nil && m.snapshot.TaskID == task.ID {
		return m.snapshot
	}
	return nil
}

// spinnerFrameForTask returns the spinner frame index for the given task ID.
// In parallel mode, each worker has its own spinner frame.
func (m statusModel) spinnerFrameForTask(taskID string) int {
	if frame, ok := m.workerSpinners[taskID]; ok {
		return frame
	}
	return m.spinnerFrame
}

// ── Completed task output loading ───────────────────────────────────────────

// ensureCompletedTaskOutput loads the completed task output from run log JSONL
// files if the cache doesn't already contain data for the selected task.
// Resets logScroll when switching to a different task.
func (m *statusModel) ensureCompletedTaskOutput() {
	task := m.selectedTask()
	if task == nil {
		m.cachedTaskOutput = nil
		m.cachedTaskOutputID = ""
		return
	}
	if m.cachedTaskOutputID == task.ID && m.cachedTaskOutput != nil {
		return // cache is valid
	}
	// Task changed: reset scroll position.
	m.logScroll = 0
	m.logAutoScroll = true
	m.cachedTaskOutputID = task.ID
	m.cachedTaskOutput = loadCompletedTaskOutput(m.dir, m.maggusIDForSelectedTask(), task.ID)
}

// loadFeatureOutput loads output snapshots for all tasks of a feature.
// It calls loadCompletedTaskOutput for each task and returns a slice with one entry per task.
// Tasks with no log data have a nil entry. If maggusID is empty, all entries are nil.
func loadFeatureOutput(dir, maggusID string, tasks []parser.Task) []*runlog.StateSnapshot {
	result := make([]*runlog.StateSnapshot, len(tasks))
	if maggusID == "" {
		return result
	}
	for i, task := range tasks {
		result[i] = loadCompletedTaskOutput(dir, maggusID, task.ID)
	}
	return result
}

// ensureFeatureOutput loads the feature-level output cache for the selected plan
// if the cache doesn't already hold data for the current plan's MaggusID.
// Resets logScroll and logAutoScroll when the cache is invalidated.
func (m *statusModel) ensureFeatureOutput() {
	plan := m.selectedPlan()
	if plan.MaggusID == "" {
		if m.cachedFeatureOutputID != "" {
			m.cachedFeatureOutput = nil
			m.cachedFeatureOutputID = ""
			m.logScroll = 0
			m.logAutoScroll = true
		}
		return
	}
	if m.cachedFeatureOutputID == plan.MaggusID {
		return // cache is valid
	}
	// Plan changed: reset scroll state and reload.
	m.logScroll = 0
	m.logAutoScroll = true
	m.cachedFeatureOutputID = plan.MaggusID
	m.cachedFeatureOutput = loadFeatureOutput(m.dir, plan.MaggusID, plan.Tasks)
}

// loadCompletedTaskOutput scans run log JSONL files in .maggus/logs/<maggusID>/ for
// entries matching taskID and builds a synthetic StateSnapshot. If maggusID is empty
// it falls back to scanning all subdirectories under .maggus/logs/. Returns nil when
// no matching entries are found.
func loadCompletedTaskOutput(dir, maggusID, taskID string) *runlog.StateSnapshot {
	var logFiles []string

	if maggusID != "" {
		// Targeted lookup: only scan the feature's own log directory.
		logFiles = findLogsForMaggusID(dir, maggusID)
	} else {
		// Fallback: scan all feature subdirectories under .maggus/logs/.
		logsBase := filepath.Join(dir, ".maggus", "logs")
		subdirs, err := os.ReadDir(logsBase)
		if err == nil {
			for _, sub := range subdirs {
				if sub.IsDir() {
					logFiles = append(logFiles, findLogsForMaggusID(dir, sub.Name())...)
				}
			}
		}
	}

	// Sort descending (most recent first) to find the latest matching entries.
	sort.Sort(sort.Reverse(sort.StringSlice(logFiles)))

	for _, logPath := range logFiles {
		snap := scanLogForTask(logPath, taskID)
		if snap != nil {
			return snap
		}
	}
	return nil
}

// scanLogForTask reads a JSONL log file and returns a synthetic StateSnapshot
// built from entries matching taskID. Returns nil when no matching entries found.
func scanLogForTask(logPath, taskID string) *runlog.StateSnapshot {
	f, err := os.Open(logPath)
	if err != nil {
		return nil
	}
	defer f.Close()

	var snap runlog.StateSnapshot
	snap.TaskID = taskID
	found := false

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		var entry runlog.Entry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue
		}

		// task_usage events may lack TaskID but follow task tool_use entries.
		if entry.TaskID != taskID {
			if entry.Event == "task_usage" && found {
				snap.TokenInput = entry.InputTokens
				snap.TokenOutput = entry.OutputTokens
				snap.TokenCost = entry.CostUSD
			}
			continue
		}

		found = true
		switch entry.Event {
		case "task_start":
			snap.TaskTitle = entry.Title
			snap.TaskStartedAt = entry.Ts
			snap.RunStartedAt = entry.Ts
		case "tool_use":
			desc := formatToolDescription(entry.Tool, entry.Input)
			snap.ToolEntries = append(snap.ToolEntries, runlog.SnapshotToolEntry{
				Type:        entry.Tool,
				Icon:        toolIconForSnapshot(entry.Tool),
				Description: desc,
				Timestamp:   entry.Ts,
			})
		case "task_complete":
			snap.Status = "Done"
			snap.UpdatedAt = entry.Ts
			if entry.Commit != "" {
				snap.Commits = append(snap.Commits, entry.Commit)
			}
		case "task_failed":
			snap.Status = "Failed"
			snap.UpdatedAt = entry.Ts
		case "task_usage":
			snap.TokenInput = entry.InputTokens
			snap.TokenOutput = entry.OutputTokens
			snap.TokenCost = entry.CostUSD
		}
	}

	if !found {
		return nil
	}
	if snap.Status == "" {
		snap.Status = "Done"
	}
	return &snap
}

// formatToolDescription builds a readable description from tool input params.
func formatToolDescription(toolType string, input map[string]string) string {
	if len(input) == 0 {
		return toolType
	}
	switch toolType {
	case "Read":
		if p, ok := input["file_path"]; ok {
			return filepath.Base(p)
		}
		if p, ok := input["path"]; ok {
			return filepath.Base(p)
		}
	case "Edit", "Write":
		if p, ok := input["file_path"]; ok {
			return filepath.Base(p)
		}
	case "Bash":
		if cmd, ok := input["command"]; ok {
			return cmd
		}
		if desc, ok := input["description"]; ok {
			return desc
		}
	case "Glob":
		if p, ok := input["pattern"]; ok {
			return p
		}
	case "Grep":
		if p, ok := input["pattern"]; ok {
			return p
		}
	case "Agent":
		if d, ok := input["description"]; ok {
			return d
		}
	case "Skill":
		if s, ok := input["skill"]; ok {
			return s
		}
	}
	// Fallback: join all values.
	var parts []string
	for _, v := range input {
		parts = append(parts, v)
	}
	return strings.Join(parts, " ")
}

// ── Completed task output rendering ─────────────────────────────────────────

// renderCompletedTaskOutput renders the Output tab for a completed or pending task.
// It uses tool history loaded from run log JSONL files.
// Returns raw string without wrapping — caller (renderRightPane) applies final sizing.
func (m statusModel) renderCompletedTaskOutput(width, height int) string {
	task := m.selectedTask()
	if task == nil {
		return statusDimStyle.Render("  No task selected")
	}

	snap := m.cachedTaskOutput
	if snap == nil || snap.TaskID != task.ID {
		return statusDimStyle.Render("  No output history available")
	}

	var sb strings.Builder

	contentWidth := width - 4
	if contentWidth < 20 {
		contentWidth = 20
	}

	// ── Top zone (fixed): status + task ID/title + separator ──
	statusIcon := "✓"
	sColor := statusGreenStyle
	if snap.Status == "Failed" {
		statusIcon = "✗"
		sColor = statusRedStyle
	}
	sb.WriteString(fmt.Sprintf(" %s  %s  %s\n",
		statusBoldStyle.Render("Status:"), sColor.Render(statusIcon), sColor.Render(snap.Status)))

	if snap.TaskID != "" {
		taskLabel := statusBoldStyle.Render("Task:")
		taskLabelW := lipgloss.Width(taskLabel)
		taskIDRendered := statusCyanStyle.Render(snap.TaskID)
		taskIDW := lipgloss.Width(taskIDRendered)
		maxTitleW := width - 1 - taskLabelW - 4 - taskIDW - 3

		if maxTitleW <= 0 {
			sb.WriteString(fmt.Sprintf(" %s    %s\n", taskLabel, taskIDRendered))
		} else {
			truncatedTitle := styles.Truncate(snap.TaskTitle, maxTitleW)
			sb.WriteString(fmt.Sprintf(" %s    %s - %s\n", taskLabel, taskIDRendered, truncatedTitle))
		}
	}
	sb.WriteString(" " + styles.Separator(width-1) + "\n")

	// ── Middle zone (scrollable tool list) ──
	// Overhead: top=3 (status+task+sep) + bottom=5 (sep+tokens+cost+duration+tools) = 8
	available := height - 8
	if available < 3 {
		available = 3
	}
	totalTools := len(snap.ToolEntries)
	toolLines := buildToolLines(snap.ToolEntries, contentWidth)
	m.renderScrollableToolList(&sb, toolLines, totalTools, available)

	// ── Bottom zone (fixed): separator + tokens + cost + duration + tools count ──
	sb.WriteString(" " + styles.Separator(width-1) + "\n")

	if snap.TokenInput > 0 || snap.TokenOutput > 0 {
		tokenStr := fmt.Sprintf("%s in / %s out",
			FormatTokens(snap.TokenInput), FormatTokens(snap.TokenOutput))
		sb.WriteString(fmt.Sprintf("  %s  %s\n", statusBoldStyle.Render("Tokens:"), statusDimStyle.Render(tokenStr)))
		costStr := "N/A"
		if snap.TokenCost > 0 {
			costStr = FormatCost(snap.TokenCost)
		}
		sb.WriteString(fmt.Sprintf("  %s    %s\n", statusBoldStyle.Render("Cost:"), statusDimStyle.Render(costStr)))
	} else {
		sb.WriteString(fmt.Sprintf("  %s  %s\n", statusBoldStyle.Render("Tokens:"), statusDimStyle.Render("N/A")))
		sb.WriteString(fmt.Sprintf("  %s    %s\n", statusBoldStyle.Render("Cost:"), statusDimStyle.Render("N/A")))
	}

	// Duration (computed from task_start to task_complete/task_failed timestamps).
	duration := "—"
	if snap.TaskStartedAt != "" && snap.UpdatedAt != "" {
		startT, err1 := time.Parse(time.RFC3339, snap.TaskStartedAt)
		endT, err2 := time.Parse(time.RFC3339, snap.UpdatedAt)
		if err1 == nil && err2 == nil && endT.After(startT) {
			duration = formatHumanDuration(endT.Sub(startT))
		}
	}
	sb.WriteString(fmt.Sprintf("  %s    %s\n", statusBoldStyle.Render("Task:"), statusDimStyle.Render(duration)))
	sb.WriteString(fmt.Sprintf("  %s   %s", statusBoldStyle.Render("Tools:"), statusDimStyle.Render(fmt.Sprintf("%d", totalTools))))

	return sb.String()
}
