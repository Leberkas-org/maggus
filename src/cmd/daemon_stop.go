package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the running maggus daemon",
	Long: `Sends a graceful shutdown signal to the running daemon and waits up to 10s
for it to exit cleanly. If the daemon does not stop within 10s, it is
force-killed.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}

		pid, err := readDaemonPID(dir)
		if err != nil {
			return fmt.Errorf("read daemon PID: %w", err)
		}
		if pid == 0 {
			cmd.Println("No daemon is running.")
			return nil
		}

		if !isProcessRunning(pid) {
			cmd.Println("No daemon is running (stale PID file removed).")
			removeDaemonPID(dir)
			return nil
		}

		cmd.Printf("Stopping daemon (PID %d)...\n", pid)

		// Send graceful shutdown signal.
		if signalErr := sendGracefulSignal(pid, dir); signalErr != nil {
			cmd.Printf("Warning: could not send graceful signal: %v\n", signalErr)
		}

		// Wait up to 10s for the daemon to exit gracefully.
		deadline := time.Now().Add(10 * time.Second)
		for time.Now().Before(deadline) {
			if !isProcessRunning(pid) {
				removeDaemonPID(dir)
				cmd.Println("Daemon stopped.")
				return nil
			}
			time.Sleep(200 * time.Millisecond)
		}

		// Grace period expired — force-kill.
		cmd.Println("Daemon did not stop in time; force-killing...")
		if killErr := forceKill(pid); killErr != nil {
			return fmt.Errorf("force kill daemon: %w", killErr)
		}

		// Wait briefly for the force-kill to take effect.
		for range 15 {
			time.Sleep(200 * time.Millisecond)
			if !isProcessRunning(pid) {
				break
			}
		}

		removeDaemonPID(dir)
		cmd.Println("Daemon force-killed.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(stopCmd)
}

// stopDaemonGracefully stops the daemon identified by the PID file in dir.
// It sends a graceful signal, waits up to 10s, then force-kills if needed.
// Returns nil if the daemon was stopped or was not running.
func stopDaemonGracefully(dir string) error {
	pid, err := readDaemonPID(dir)
	if err != nil {
		return fmt.Errorf("read daemon PID: %w", err)
	}
	if pid == 0 || !isProcessRunning(pid) {
		removeDaemonPID(dir)
		return nil
	}

	if signalErr := sendGracefulSignal(pid, dir); signalErr != nil {
		// Continue anyway — force-kill will handle it.
		_ = signalErr
	}

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if !isProcessRunning(pid) {
			removeDaemonPID(dir)
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}

	if killErr := forceKill(pid); killErr != nil {
		return fmt.Errorf("force kill daemon: %w", killErr)
	}

	for range 15 {
		time.Sleep(200 * time.Millisecond)
		if !isProcessRunning(pid) {
			break
		}
	}

	removeDaemonPID(dir)
	return nil
}
