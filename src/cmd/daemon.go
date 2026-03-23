package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// daemonPIDPath returns the path to the daemon PID file.
func daemonPIDPath(dir string) string {
	return filepath.Join(dir, ".maggus", "daemon.pid")
}

// daemonLogPathFor returns the path to the daemon log for a given run.
func daemonLogPathFor(dir, runID string) string {
	return filepath.Join(dir, ".maggus", "runs", runID, "daemon.log")
}

// readDaemonPID reads the PID from the daemon PID file.
// Returns 0, nil if the file does not exist.
func readDaemonPID(dir string) (int, error) {
	path := daemonPIDPath(dir)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("read daemon.pid: %w", err)
	}
	pidStr := strings.TrimSpace(string(data))
	if pidStr == "" {
		return 0, nil
	}
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return 0, nil // malformed — treat as not running
	}
	return pid, nil
}

// writeDaemonPID writes the given PID to the daemon PID file.
func writeDaemonPID(dir string, pid int) error {
	path := daemonPIDPath(dir)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create .maggus dir: %w", err)
	}
	return os.WriteFile(path, []byte(strconv.Itoa(pid)+"\n"), 0644)
}

// removeDaemonPID removes the daemon PID file, silently ignoring not-found errors.
func removeDaemonPID(dir string) {
	_ = os.Remove(daemonPIDPath(dir))
}

// generateDaemonRunID returns a timestamp-based run ID for the daemon session.
func generateDaemonRunID() string {
	return time.Now().Format("20060102-150405")
}
