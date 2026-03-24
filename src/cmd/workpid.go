package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// workPIDPath returns the path to the work-run PID file.
func workPIDPath(dir string) string {
	return filepath.Join(dir, ".maggus", "work.pid")
}

// readWorkPID reads the PID from the work PID file.
// Returns 0, nil if the file does not exist.
func readWorkPID(dir string) (int, error) {
	path := workPIDPath(dir)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("read work.pid: %w", err)
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

// writeWorkPID writes the given PID to the work PID file.
func writeWorkPID(dir string, pid int) error {
	path := workPIDPath(dir)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create .maggus dir: %w", err)
	}
	return os.WriteFile(path, []byte(strconv.Itoa(pid)+"\n"), 0644)
}

// removeWorkPID removes the work PID file, silently ignoring not-found errors.
func removeWorkPID(dir string) {
	_ = os.Remove(workPIDPath(dir))
}
