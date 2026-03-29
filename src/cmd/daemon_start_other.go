//go:build !windows

package cmd

import (
	"os"
	"os/exec"
	"syscall"
)

// launchDaemon starts the daemon process on Unix, detached from the current
// terminal. Setsid creates a new session so the daemon becomes session leader
// and loses its controlling terminal.
func launchDaemon(exe string, args []string, logFile *os.File, dir string) (int, error) {
	cmd := exec.Command(exe, args...)
	cmd.Dir = dir
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	return cmd.Process.Pid, nil
}

// isProcessRunning returns true if the process with the given PID is still alive.
// On Unix, sending signal 0 to a PID checks existence without delivering a signal.
func isProcessRunning(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}
