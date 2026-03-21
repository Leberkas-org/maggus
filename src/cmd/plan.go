package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"time"

	"github.com/leberkas-org/maggus/internal/config"
	"github.com/leberkas-org/maggus/internal/session"
	"github.com/spf13/cobra"
)

const (
	maggusPluginID       = "maggus@maggus"
	maggusMarketplace    = "maggus"
	maggusMarketplaceURL = "https://github.com/Leberkas-org/maggus-skills.git"
)

var planCmd = &cobra.Command{
	Use:   "plan [description...]",
	Short: "Open an interactive AI session to create an implementation plan",
	Long: `Launches Claude Code (or the configured agent) interactively with the
/maggus-plan skill pre-filled. You provide the feature description and the
AI walks you through clarifying questions before generating the plan.

Examples:
  maggus plan Add OAuth2 authentication with Google provider
  maggus plan "Refactor the parser to support nested tasks"`,
	Args: cobra.MinimumNArgs(1),
	RunE: runSkillCommand("/maggus-plan"),
}

var visionCmd = &cobra.Command{
	Use:   "vision [description...]",
	Short: "Open an interactive AI session to create or improve VISION.md",
	Long: `Launches Claude Code (or the configured agent) interactively with the
/maggus-vision skill pre-filled. You provide context about your project and the
AI guides you through creating or refining a VISION.md.

Examples:
  maggus vision A CLI tool for orchestrating AI agents
  maggus vision "Improve the vision for our e-commerce platform"`,
	Args: cobra.MinimumNArgs(1),
	RunE: runSkillCommand("/maggus-vision"),
}

var architectureCmd = &cobra.Command{
	Use:   "architecture [description...]",
	Short: "Open an interactive AI session to create or improve ARCHITECTURE.md",
	Long: `Launches Claude Code (or the configured agent) interactively with the
/maggus-architecture skill pre-filled. You provide context about your project
and the AI guides you through creating or refining an ARCHITECTURE.md.

Examples:
  maggus architecture A Go CLI with plugin system and streaming output
  maggus architecture "Review and improve our current architecture"`,
	Aliases: []string{"arch"},
	Args:    cobra.MinimumNArgs(1),
	RunE:    runSkillCommand("/maggus-architecture"),
}

// runSkillCommand returns a cobra RunE that launches the configured agent
// interactively with the given skill and the user's description as prompt.
func runSkillCommand(skill string) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		description := strings.Join(args, " ")

		dir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}

		cfg, err := config.Load(dir)
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		agentName := cfg.Agent
		if agentName == "" {
			agentName = "claude"
		}

		// Ensure the maggus plugin is installed and enabled in Claude Code.
		if agentName == "claude" {
			if err := ensureMaggusPlugin(); err != nil {
				return err
			}
		}

		prompt := fmt.Sprintf("%s %s", skill, description)
		_, err = launchInteractive(agentName, prompt, dir)
		return err
	}
}

// SessionInfo holds timing and snapshot data from an interactive session,
// allowing callers to extract usage from the detected session file afterward.
type SessionInfo struct {
	BeforeSnapshot map[string]bool
	StartTime      time.Time
	EndTime        time.Time
}

// launchInteractive launches the given agent CLI interactively with a prefilled prompt.
// It connects stdin/stdout/stderr directly so the user has full control.
// The dir parameter is the working directory used to locate the Claude session directory
// for snapshotting. Session timing info is returned so callers can extract usage afterward.
// Snapshotting errors are logged as warnings and do not prevent the session from launching.
func launchInteractive(agentName, prompt, dir string) (*SessionInfo, error) {
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

	// Launch interactively: pass prompt as positional arg (not -p).
	cmd := exec.Command(path, prompt)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Forward interrupt signals to the child process by ignoring them in
	// the parent — the terminal delivers SIGINT to the entire process group,
	// so the agent receives it directly.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, shutdownSignals...)
	defer signal.Stop(sigCh)

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start %s: %w", agentName, err)
	}

	waitErr := cmd.Wait()
	endTime := time.Now()

	signal.Stop(sigCh)

	info := &SessionInfo{
		BeforeSnapshot: beforeSnapshot,
		StartTime:      startTime,
		EndTime:        endTime,
	}

	if waitErr != nil {
		// User-initiated exits (Ctrl+C or exit code 2) are not errors.
		if cmd.ProcessState != nil {
			code := cmd.ProcessState.ExitCode()
			if code == 130 || code == 2 {
				return info, nil
			}
		}
		return info, fmt.Errorf("%s exited with error: %w", agentName, waitErr)
	}

	return info, nil
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
