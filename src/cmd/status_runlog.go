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

// findLatestRunLog returns the run ID and run.log path of the most recently created
// run directory under .maggus/runs/. Returns empty strings if none is found.
func findLatestRunLog(dir string) (runID, logPath string) {
	runsDir := filepath.Join(dir, ".maggus", "runs")
	entries, err := os.ReadDir(runsDir)
	if err != nil {
		return "", ""
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e.Name())
		}
	}
	if len(dirs) == 0 {
		return "", ""
	}
	sort.Strings(dirs)
	latest := dirs[len(dirs)-1]
	candidate := filepath.Join(runsDir, latest, "run.log")
	if _, err := os.Stat(candidate); err != nil {
		return "", ""
	}
	return latest, candidate
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
		desc := entry.Description
		if desc == "" {
			desc = entry.Tool
		}
		return fmt.Sprintf("%s%s %s %s", tsStr, taskID, toolTag, logInfoStyle.Render(desc))

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

	default:
		// Unknown event — render the whole line muted with whatever fields are available
		var parts []string
		if entry.Text != "" {
			parts = append(parts, entry.Text)
		}
		if entry.Description != "" {
			parts = append(parts, entry.Description)
		}
		if len(parts) == 0 {
			parts = append(parts, entry.Event)
		}
		return fmt.Sprintf("%s%s %s", tsStr, taskID, logInfoStyle.Render(strings.Join(parts, " ")))
	}
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
