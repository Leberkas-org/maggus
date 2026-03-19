package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/leberkas-org/maggus/internal/config"
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
		return launchInteractive(agentName, prompt)
	}
}

// launchInteractive launches the given agent CLI interactively with a prefilled prompt.
// It connects stdin/stdout/stderr directly so the user has full control.
func launchInteractive(agentName string, prompt string) error {
	path, err := exec.LookPath(agentName)
	if err != nil {
		return fmt.Errorf("%s not found on PATH: %w", agentName, err)
	}

	// Launch interactively: pass prompt as positional arg (not -p).
	cmd := exec.Command(path, prompt)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
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
