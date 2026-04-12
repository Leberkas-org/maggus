package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// logPollTick returns a tea.Cmd that fires logFileUpdateMsg after 200ms.
// Used as a fallback when LogFileWatcher cannot be initialized.
func logPollTick() tea.Cmd {
	return tea.Tick(200*time.Millisecond, func(_ time.Time) tea.Msg {
		return logFileUpdateMsg{}
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
	PID               int
	Running           bool
	StoppingAfterTask bool
	CurrentFeature    string
	CurrentTask       string
}

// findLogsForMaggusID returns the sorted list of .log file paths in
// .maggus/logs/<maggusID>/. Returns nil when the directory is missing or empty.
func findLogsForMaggusID(dir, maggusID string) []string {
	if maggusID == "" {
		return nil
	}
	logDir := filepath.Join(dir, ".maggus", "logs", maggusID)
	entries, err := os.ReadDir(logDir)
	if err != nil {
		return nil
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".log") {
			files = append(files, filepath.Join(logDir, e.Name()))
		}
	}
	return files
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
