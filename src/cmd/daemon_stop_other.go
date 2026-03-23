//go:build !windows

package cmd

import (
	"os"
	"syscall"
)

// sendGracefulSignal on Unix sends SIGTERM to the process.
func sendGracefulSignal(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return proc.Signal(syscall.SIGTERM)
}

// forceKill on Unix sends SIGKILL to the process.
func forceKill(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return proc.Signal(syscall.SIGKILL)
}
