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
	"github.com/leberkas-org/maggus/internal/runlog"
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

