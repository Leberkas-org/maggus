package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var (
	startModelFlag string
	startAgentFlag string
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Launch the work loop as a background daemon",
	Long: `Starts maggus in daemon mode, running the feature-centric work loop
unattended in the background. Agent output is logged to
.maggus/runs/<RUN_ID>/daemon.log.

Use 'maggus stop' to terminate the daemon.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}

		// Ensure .maggus directory exists.
		if err := os.MkdirAll(filepath.Join(dir, ".maggus"), 0755); err != nil {
			return fmt.Errorf("create .maggus dir: %w", err)
		}

		// Check if a daemon is already running.
		existingPID, err := readDaemonPID(dir)
		if err != nil {
			return fmt.Errorf("check daemon status: %w", err)
		}
		if existingPID != 0 && isProcessRunning(existingPID) {
			return fmt.Errorf("daemon is already running (PID %d)\nUse 'maggus stop' to terminate it first", existingPID)
		}
		// Stale PID file — clean it up silently.
		if existingPID != 0 {
			removeDaemonPID(dir)
		}

		// Mutual exclusion: prevent daemon from starting while a work run is active.
		workPID, wpErr := readWorkPID(dir)
		if wpErr != nil {
			return fmt.Errorf("check work status: %w", wpErr)
		}
		if workPID != 0 {
			if isProcessRunning(workPID) {
				return fmt.Errorf("a work run is active (PID %d) — wait for it to finish", workPID)
			}
			// Stale PID file — clean it up silently.
			removeWorkPID(dir)
		}

		// Generate run ID and create the run directory + daemon.log.
		runID := generateDaemonRunID()
		runDir := filepath.Join(dir, ".maggus", "runs", runID)
		if err := os.MkdirAll(runDir, 0755); err != nil {
			return fmt.Errorf("create run directory: %w", err)
		}
		daemonLogPath := daemonLogPathFor(dir, runID)
		logFile, err := os.OpenFile(daemonLogPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return fmt.Errorf("create daemon log: %w", err)
		}
		defer logFile.Close()

		// Locate the current executable.
		exe, err := os.Executable()
		if err != nil {
			return fmt.Errorf("get executable path: %w", err)
		}

		// Build the daemon work command arguments.
		daemonArgs := []string{"work", "--daemon-run", "--daemon-run-id=" + runID}
		if startModelFlag != "" {
			daemonArgs = append(daemonArgs, "--model="+startModelFlag)
		}
		if startAgentFlag != "" {
			daemonArgs = append(daemonArgs, "--agent="+startAgentFlag)
		}

		// Launch the daemon process (platform-specific detach).
		pid, err := launchDaemon(exe, daemonArgs, logFile, dir)
		if err != nil {
			return fmt.Errorf("launch daemon: %w", err)
		}

		// Write PID file so stop/status commands can find the daemon.
		if err := writeDaemonPID(dir, pid); err != nil {
			return fmt.Errorf("write daemon PID: %w", err)
		}

		cmd.Printf("Daemon started (PID: %d)\n", pid)
		cmd.Printf("Logs: %s\n", daemonLogPath)
		return nil
	},
}

func init() {
	startCmd.Flags().StringVar(&startModelFlag, "model", "", "model to use (e.g. opus, sonnet, haiku)")
	startCmd.Flags().StringVar(&startAgentFlag, "agent", "", "agent to use (e.g. claude, opencode)")
	rootCmd.AddCommand(startCmd)
}
