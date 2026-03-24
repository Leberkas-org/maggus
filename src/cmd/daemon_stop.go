package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/leberkas-org/maggus/internal/globalconfig"
	"github.com/spf13/cobra"
)

var stopAllFlag bool

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the running maggus daemon",
	Long: `Sends a graceful shutdown signal to the running daemon and waits up to 10s
for it to exit cleanly. If the daemon does not stop within 10s, it is
force-killed.

Use --all to stop daemons across all registered repositories.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if stopAllFlag {
			return stopAllDaemons(cmd)
		}
		return stopCurrentDaemon(cmd)
	},
}

func init() {
	stopCmd.Flags().BoolVar(&stopAllFlag, "all", false, "stop daemons in all registered repositories")
	rootCmd.AddCommand(stopCmd)
}

func stopCurrentDaemon(cmd *cobra.Command) error {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	// Guard: refuse to stop if the current directory is not a registered repository.
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("resolve absolute path: %w", err)
	}
	cfg, err := globalconfig.Load()
	if err != nil {
		return fmt.Errorf("load global config: %w", err)
	}
	if !cfg.HasRepository(absDir) {
		cmd.Println("Not in a registered repository. Use 'maggus repos' to add one.")
		return nil
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
}

func stopAllDaemons(cmd *cobra.Command) error {
	cfg, err := globalconfig.Load()
	if err != nil {
		return fmt.Errorf("load global config: %w", err)
	}

	if len(cfg.Repositories) == 0 {
		cmd.Println("No repositories registered.")
		return nil
	}

	var errs []error
	for _, repo := range cfg.Repositories {
		pid, readErr := readDaemonPID(repo.Path)
		if readErr != nil {
			cmd.Printf("Error reading daemon status in %s: %v\n", repo.Path, readErr)
			errs = append(errs, fmt.Errorf("%s: %w", repo.Path, readErr))
			continue
		}

		if pid == 0 || !isProcessRunning(pid) {
			if pid != 0 {
				removeDaemonPID(repo.Path)
			}
			cmd.Printf("No daemon running in %s\n", repo.Path)
			continue
		}

		cmd.Printf("Stopping daemon in %s (PID %d)...\n", repo.Path, pid)
		if stopErr := stopDaemonGracefully(repo.Path); stopErr != nil {
			cmd.Printf("Error stopping daemon in %s: %v\n", repo.Path, stopErr)
			errs = append(errs, fmt.Errorf("%s: %w", repo.Path, stopErr))
			continue
		}
		cmd.Printf("Stopped daemon in %s (PID %d)\n", repo.Path, pid)
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to stop %d daemon(s); see output above for details", len(errs))
	}
	return nil
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
