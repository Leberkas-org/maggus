package cmd

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
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
