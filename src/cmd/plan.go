package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"time"

	"github.com/leberkas-org/maggus/internal/gitutil"
	"github.com/leberkas-org/maggus/internal/session"
	"github.com/leberkas-org/maggus/internal/usage"
)

const (
	maggusPluginID       = "maggus@maggus"
	maggusMarketplace    = "maggus"
	maggusMarketplaceURL = "https://github.com/Leberkas-org/maggus-skills.git"
)

// extractSkillUsage detects the session file created during an interactive skill session,
// extracts token usage, and appends a record to the global usage directory.
// The kind parameter identifies the session type (e.g. "plan", "bugreport", "prompt").
// Errors are printed as warnings but never cause a non-zero exit.
func extractSkillUsage(dir, model, agentName, kind string, info *SessionInfo) {
	sessionFile, err := session.DetectSessionFile(dir, info.BeforeSnapshot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not detect session file: %v\n", err)
		return
	}
	if sessionFile == "" {
		fmt.Fprintln(os.Stderr, "Warning: no new Claude session file found; skipping usage extraction")
		return
	}

	summary, err := session.ExtractUsage(sessionFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not extract usage from session: %v\n", err)
		return
	}

	runID := info.StartTime.Format("20060102-150405")
	repoURL := gitutil.RepoURL(dir)

	rec := usage.Record{
		RunID:                    runID,
		Repository:               repoURL,
		Kind:                     kind,
		Model:                    model,
		Agent:                    agentName,
		InputTokens:              summary.InputTokens,
		OutputTokens:             summary.OutputTokens,
		CacheCreationInputTokens: summary.CacheCreationInputTokens,
		CacheReadInputTokens:     summary.CacheReadInputTokens,
		CostUSD:                  0,
		ModelUsage:               summary.ModelUsage,
		StartTime:                info.StartTime,
		EndTime:                  info.EndTime,
	}

	if err := usage.Append([]usage.Record{rec}); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not write usage record: %v\n", err)
	}
}

// SessionInfo holds timing and snapshot data from an interactive session,
// allowing callers to extract usage from the detected session file afterward.
type SessionInfo struct {
	BeforeSnapshot map[string]bool
	StartTime      time.Time
	EndTime        time.Time
}

// launchInteractive launches the given agent CLI interactively with an optional initial prompt.
// It connects stdin/stdout/stderr directly so the user has full control.
// The dir parameter is the working directory used to locate the Claude session directory
// for snapshotting. When skipPermissions is true, --dangerously-skip-permissions is passed.
// When model is non-empty, --model is passed. Session timing info is returned so callers
// can extract usage afterward. Snapshotting errors are logged as warnings and do not
// prevent the session from launching.
//
// When prompt is non-empty, it is passed as a positional argument to the Claude CLI,
// which starts an interactive session with the prompt as the initial message.
func launchInteractive(agentName, prompt, dir string, skipPermissions bool, model string) (*SessionInfo, error) {
	path, err := exec.LookPath(agentName)
	if err != nil {
		return nil, fmt.Errorf("%s not found on PATH: %w", agentName, err)
	}

	// Snapshot session directory before launching to detect new session files afterward.
	sessionDir, sdErr := session.SessionDir(dir)
	var beforeSnapshot map[string]bool
	if sdErr != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not resolve session directory: %v\n", sdErr)
	} else {
		var snapErr error
		beforeSnapshot, snapErr = session.SnapshotDir(sessionDir)
		if snapErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not snapshot session directory: %v\n", snapErr)
		}
	}

	startTime := time.Now()

	// Forward interrupt signals to the child process by ignoring them in
	// the parent — the terminal delivers SIGINT to the entire process group,
	// so the agent receives it directly.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, shutdownSignals...)
	defer signal.Stop(sigCh)

	var args []string
	if skipPermissions {
		args = append(args, "--dangerously-skip-permissions")
	}
	if model != "" {
		args = append(args, "--model", model)
	}
	if prompt != "" {
		args = append(args, prompt)
	}

	return runInteractiveCmd(path, args, agentName, beforeSnapshot, startTime)
}

// runInteractiveCmd starts an interactive CLI session with the given args and waits for it to finish.
func runInteractiveCmd(path string, args []string, agentName string, beforeSnapshot map[string]bool, startTime time.Time) (*SessionInfo, error) {
	cmd := exec.Command(path, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start %s: %w", agentName, err)
	}

	waitErr := cmd.Wait()
	info := buildSessionInfo(beforeSnapshot, startTime)

	if waitErr != nil {
		if isUserExit(cmd) {
			return info, nil
		}
		return info, fmt.Errorf("%s exited with error: %w", agentName, waitErr)
	}

	return info, nil
}

// isUserExit checks whether the command exited due to user-initiated cancellation (Ctrl+C).
func isUserExit(cmd *exec.Cmd) bool {
	if cmd.ProcessState != nil {
		code := cmd.ProcessState.ExitCode()
		return code == 130 || code == 2
	}
	return false
}

// buildSessionInfo creates a SessionInfo with the current time as the end time.
func buildSessionInfo(beforeSnapshot map[string]bool, startTime time.Time) *SessionInfo {
	return &SessionInfo{
		BeforeSnapshot: beforeSnapshot,
		StartTime:      startTime,
		EndTime:        time.Now(),
	}
}

// pluginInfo represents a single entry from `claude plugin list --json`.
type pluginInfo struct {
	ID      string `json:"id"`
	Enabled bool   `json:"enabled"`
}

// marketplaceInfo represents a single entry from `claude plugin marketplace list --json`.
type marketplaceInfo struct {
	Name string `json:"name"`
}

// ensureMaggusPlugin checks if the maggus plugin is installed and enabled
// in Claude Code. If the marketplace isn't added, it adds it first.
// If the plugin isn't installed, it installs it. If disabled, it enables it.
func ensureMaggusPlugin() error {
	claudePath, err := exec.LookPath("claude")
	if err != nil {
		return fmt.Errorf("claude not found on PATH: %w", err)
	}

	// List installed plugins as JSON.
	listCmd := exec.Command(claudePath, "plugin", "list", "--json")
	out, err := listCmd.Output()
	if err != nil {
		// If plugin list fails, skip the check and let the skill command fail naturally.
		return nil
	}

	var plugins []pluginInfo
	if err := json.Unmarshal(out, &plugins); err != nil {
		return nil
	}

	// Check if maggus plugin is present.
	for _, p := range plugins {
		if p.ID == maggusPluginID {
			if p.Enabled {
				return nil
			}
			// Installed but disabled — enable it.
			fmt.Println("Enabling maggus plugin...")
			enableCmd := exec.Command(claudePath, "plugin", "enable", maggusPluginID)
			enableCmd.Stdout = os.Stdout
			enableCmd.Stderr = os.Stderr
			return enableCmd.Run()
		}
	}

	// Not installed — ensure marketplace is added first.
	if err := ensureMaggusMarketplace(claudePath); err != nil {
		return err
	}

	// Install the plugin.
	fmt.Println("Installing maggus plugin...")
	installCmd := exec.Command(claudePath, "plugin", "install", maggusPluginID)
	installCmd.Stdout = os.Stdout
	installCmd.Stderr = os.Stderr
	return installCmd.Run()
}

// ensureMaggusMarketplace checks if the maggus marketplace is configured
// in Claude Code and adds it if missing.
func ensureMaggusMarketplace(claudePath string) error {
	listCmd := exec.Command(claudePath, "plugin", "marketplace", "list", "--json")
	out, err := listCmd.Output()
	if err != nil {
		// Can't list marketplaces — try adding anyway, it'll fail with a clear error.
		return addMaggusMarketplace(claudePath)
	}

	var marketplaces []marketplaceInfo
	if err := json.Unmarshal(out, &marketplaces); err != nil {
		return addMaggusMarketplace(claudePath)
	}

	for _, m := range marketplaces {
		if m.Name == maggusMarketplace {
			return nil
		}
	}

	return addMaggusMarketplace(claudePath)
}

func addMaggusMarketplace(claudePath string) error {
	fmt.Println("Adding maggus marketplace...")
	addCmd := exec.Command(claudePath, "plugin", "marketplace", "add", maggusMarketplaceURL)
	addCmd.Stdout = os.Stdout
	addCmd.Stderr = os.Stderr
	return addCmd.Run()
}
