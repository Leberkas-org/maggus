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

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/leberkas-org/maggus/internal/runlog"
	"github.com/leberkas-org/maggus/internal/runner"
	"github.com/leberkas-org/maggus/internal/tui/styles"
)

// logPollTickMsg is sent every 200ms to refresh the live log panel.
type logPollTickMsg struct{}

// logPollTick returns a tea.Cmd that fires logPollTickMsg after 200ms.
func logPollTick() tea.Cmd {
	return tea.Tick(200*time.Millisecond, func(_ time.Time) tea.Msg {
		return logPollTickMsg{}
	})
}

// spinnerTickMsg drives the animated spinner in the rich live view.
type spinnerTickMsg struct{}

// spinnerTick returns a tea.Cmd that fires spinnerTickMsg after 80ms.
func spinnerTick() tea.Cmd {
	return tea.Tick(80*time.Millisecond, func(_ time.Time) tea.Msg {
		return spinnerTickMsg{}
	})
}

// daemonStatus holds the current daemon state for display in the status header and log panel.
type daemonStatus struct {
	PID            int
	Running        bool
	RunID          string
	LogPath        string
	CurrentFeature string
	CurrentTask    string
}

// loadDaemonStatus reads the daemon PID file, checks whether the process is alive,
// finds the latest run log, and parses the last entries for current feature/task state.
func loadDaemonStatus(dir string) daemonStatus {
	pid, _ := readDaemonPID(dir)
	running := pid != 0 && isProcessRunning(pid)

	runID, logPath := findLatestRunLog(dir)
	info := daemonStatus{
		PID:     pid,
		Running: running,
		RunID:   runID,
		LogPath: logPath,
	}
	if logPath != "" {
		lines := readLastNLogLines(logPath, 200)
		info.CurrentFeature, info.CurrentTask = parseLogForCurrentState(lines)
	}
	return info
}

// findLatestRunLog returns the run ID (from the fixed-path state.json snapshot)
// and the path to the latest flat .log file under .maggus/runs/.
// Each is found independently; either may be empty if none exists.
func findLatestRunLog(dir string) (runID, logPath string) {
	runsDir := filepath.Join(dir, ".maggus", "runs")
	entries, err := os.ReadDir(runsDir)
	if err != nil {
		return "", ""
	}

	// Collect flat .log files (excluding daemon.log) for logPath.
	var logFiles []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".log") && e.Name() != "daemon.log" {
			logFiles = append(logFiles, e.Name())
		}
	}

	// Latest log file (lexicographic sort — timestamp prefix ensures correct order).
	if len(logFiles) > 0 {
		sort.Strings(logFiles)
		logPath = filepath.Join(runsDir, logFiles[len(logFiles)-1])
	}

	// Check the fixed-path state.json directly instead of scanning subdirectories.
	statePath := filepath.Join(runsDir, "state.json")
	if _, err := os.Stat(statePath); err == nil {
		// Extract RunID from the snapshot struct for log-file lookup.
		if snap, err := runlog.ReadSnapshot(dir); err == nil && snap.RunID != "" {
			runID = snap.RunID
			// If the run-specific log file exists, prefer it over the latest scanned log.
			candidate := filepath.Join(runsDir, runID+".log")
			if _, err := os.Stat(candidate); err == nil {
				logPath = candidate
			}
		}
	}

	return runID, logPath
}

// readLastNLogLines returns the last n lines of the file at path.
// Returns nil on error or if the file is empty.
func readLastNLogLines(path string, n int) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return lines
}

// Log line styles — reuse palette from styles package.
var (
	logTimestampStyle = lipgloss.NewStyle().Foreground(styles.Muted)
	logTaskIDStyle    = lipgloss.NewStyle().Bold(true).Foreground(styles.Primary)
	logToolStyle      = lipgloss.NewStyle().Foreground(styles.Accent)
	logErrorStyle     = lipgloss.NewStyle().Foreground(styles.Error)
	logInfoStyle      = lipgloss.NewStyle().Foreground(styles.Muted)
)

// formatLogLine parses a JSONL log line and returns a color-coded string.
// Non-JSON lines are returned as-is in muted style.
func formatLogLine(raw string) string {
	var entry runlog.Entry
	if err := json.Unmarshal([]byte(raw), &entry); err != nil {
		// Graceful fallback: render plain text in muted style
		return logInfoStyle.Render(raw)
	}

	// Format timestamp as HH:MM:SS (compact, subordinate)
	ts := entry.Ts
	if t, err := time.Parse(time.RFC3339, entry.Ts); err == nil {
		ts = t.Local().Format("15:04:05")
	}
	tsStr := logTimestampStyle.Render(ts)

	// Format task ID if present
	taskID := ""
	if entry.TaskID != "" {
		taskID = " " + logTaskIDStyle.Render(entry.TaskID)
	}

	switch entry.Event {
	case "tool_use":
		toolTag := logToolStyle.Render(fmt.Sprintf("[%s]", entry.Tool))
		return fmt.Sprintf("%s%s %s %s", tsStr, taskID, toolTag, logInfoStyle.Render(formatToolInput(entry.Tool, entry.Input)))

	case "output":
		// Output is the most important content — render at full contrast (default).
		text := entry.Text
		if len(text) > 200 {
			text = text[:200] + "…"
		}
		return fmt.Sprintf("%s%s %s", tsStr, taskID, text)

	case "task_failed":
		reason := entry.Reason
		if reason == "" {
			reason = "unknown error"
		}
		return fmt.Sprintf("%s%s %s", tsStr, taskID, logErrorStyle.Render("FAILED: "+reason))

	case "error":
		text := entry.Text
		if text == "" {
			text = entry.Reason
		}
		return fmt.Sprintf("%s%s %s", tsStr, taskID, logErrorStyle.Render(text))

	case "feature_start":
		if entry.FeatureID != "" {
			return fmt.Sprintf("%s %s %s %s", tsStr, logInfoStyle.Render("feature"), logTaskIDStyle.Render(entry.FeatureID), logInfoStyle.Render("started"))
		}
		return fmt.Sprintf("%s %s", tsStr, logInfoStyle.Render("feature started"))

	case "feature_complete":
		if entry.FeatureID != "" {
			return fmt.Sprintf("%s %s %s %s", tsStr, logInfoStyle.Render("feature"), logTaskIDStyle.Render(entry.FeatureID), logInfoStyle.Render("complete"))
		}
		return fmt.Sprintf("%s %s", tsStr, logInfoStyle.Render("feature complete"))

	case "task_start":
		title := entry.Title
		if title == "" {
			title = "started"
		}
		return fmt.Sprintf("%s%s %s", tsStr, taskID, logInfoStyle.Render(title))

	case "task_complete":
		detail := "complete"
		if entry.Commit != "" {
			detail = fmt.Sprintf("complete [%s]", entry.Commit)
		}
		return fmt.Sprintf("%s%s %s", tsStr, taskID, logInfoStyle.Render(detail))

	case "info":
		text := entry.Text
		if text == "" {
			text = "info"
		}
		return fmt.Sprintf("%s%s %s", tsStr, taskID, logInfoStyle.Render(text))

	case "task_usage":
		totalIn := entry.InputTokens + entry.CacheCreationInputTokens + entry.CacheReadInputTokens
		line := fmt.Sprintf("usage: %s in / %s out  %s",
			runner.FormatTokens(totalIn), runner.FormatTokens(entry.OutputTokens), runner.FormatCost(entry.CostUSD))
		return fmt.Sprintf("%s%s %s", tsStr, taskID, logInfoStyle.Render(line))

	default:
		// Unknown event — render the whole line muted with whatever fields are available
		desc := entry.Text
		if desc == "" && len(entry.Input) > 0 {
			desc = formatToolInput(entry.Event, entry.Input)
		}
		if desc == "" {
			desc = entry.Event
		}
		return fmt.Sprintf("%s%s %s", tsStr, taskID, logInfoStyle.Render(desc))
	}
}

// formatToolInput returns the most meaningful display value from a tool's input map.
// Priority: file → command → pattern → skill → description → first value → tool name.
func formatToolInput(tool string, input map[string]string) string {
	for _, key := range []string{"file", "command", "pattern", "skill", "description"} {
		if v, ok := input[key]; ok && v != "" {
			return v
		}
	}
	for _, v := range input {
		if v != "" {
			return v
		}
	}
	return tool
}

// parseLogForCurrentState scans log lines from newest to oldest to find the most
// recently started feature and task. Parses JSONL entries; non-JSON lines are silently skipped.
func parseLogForCurrentState(lines []string) (feature, task string) {
	for i := len(lines) - 1; i >= 0; i-- {
		var entry runlog.Entry
		if err := json.Unmarshal([]byte(lines[i]), &entry); err != nil {
			continue // skip non-JSON lines gracefully
		}
		if feature == "" && entry.Event == "feature_start" && entry.FeatureID != "" {
			feature = entry.FeatureID
		}
		if task == "" && entry.Event == "task_start" && entry.TaskID != "" {
			task = entry.TaskID
		}
		if feature != "" && task != "" {
			break
		}
	}
	return feature, task
}

// renderDaemonStatusLine returns a one-line string showing daemon state and
// current feature/task progress (for use in the status header).
func (m statusModel) renderDaemonStatusLine() string {
	if m.daemon.Running {
		indicator := statusCyanStyle.Render("●")
		line := fmt.Sprintf(" %s daemon running (PID %d)", indicator, m.daemon.PID)
		if m.daemon.CurrentFeature != "" {
			line += statusDimStyle.Render(" · " + m.daemon.CurrentFeature)
		}
		if m.daemon.CurrentTask != "" {
			line += statusDimStyle.Render(" · " + m.daemon.CurrentTask)
		}
		return line
	}
	// Show last run info using whichever identifier is available.
	lastRun := m.daemon.RunID
	if lastRun == "" && m.daemon.LogPath != "" {
		// Fall back to the log filename (without directory and extension).
		base := filepath.Base(m.daemon.LogPath)
		lastRun = strings.TrimSuffix(base, ".log")
	}
	if lastRun != "" {
		return statusDimStyle.Render(fmt.Sprintf(" ○ daemon not running · last run: %s", lastRun))
	}
	return statusDimStyle.Render(" ○ daemon not running")
}

// formatHumanDuration formats a duration as human-friendly text (e.g. "5m 32s", "1h 12m 5s").
func formatHumanDuration(d time.Duration) string {
	d = d.Truncate(time.Second)
	if d < time.Second {
		return "0s"
	}

	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60

	switch {
	case h > 0:
		return fmt.Sprintf("%dh %dm %ds", h, m, s)
	case m > 0:
		return fmt.Sprintf("%dm %ds", m, s)
	default:
		return fmt.Sprintf("%ds", s)
	}
}

// formatSnapshotModelTokens formats per-model token breakdown from the snapshot.
// Returns multi-line output with one line per model, indented to align under "Tokens:".
func (m statusModel) formatSnapshotModelTokens() string {
	if m.snapshot == nil || len(m.snapshot.ModelBreakdown) == 0 {
		return ""
	}
	// Sort model names for stable output
	names := make([]string, 0, len(m.snapshot.ModelBreakdown))
	for name := range m.snapshot.ModelBreakdown {
		names = append(names, name)
	}
	sort.Strings(names)

	var sb strings.Builder
	for _, name := range names {
		usage := m.snapshot.ModelBreakdown[name]
		totalIn := usage.InputTokens + usage.CacheCreationInputTokens + usage.CacheReadInputTokens
		costStr := runner.FormatCost(usage.CostUSD)
		line := fmt.Sprintf("  %s: %s in / %s out (%s)",
			name, runner.FormatTokens(totalIn), runner.FormatTokens(usage.OutputTokens), costStr)
		sb.WriteString("  " + statusDimStyle.Render(line) + "\n")
	}
	return sb.String()
}
