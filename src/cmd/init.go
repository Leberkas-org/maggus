package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/leberkas-org/maggus/internal/gitignore"
	"github.com/spf13/cobra"
)

const defaultConfig = `# Maggus configuration
# See https://github.com/Leberkas-org/maggus for documentation.

# AI agent to use: "claude" or "opencode"
# agent: claude

# Model alias or full model ID (e.g. sonnet, opus, haiku)
# model: sonnet

# Run tasks in isolated git worktrees
# worktree: false

# Additional context files to include in prompts
# include:
#   - docs/ARCHITECTURE.md
#   - docs/VISION.md

# Git workflow settings
# git:
#   auto_branch: true
#   check_sync: true
#   protected_branches:
#     - main
#     - master
#     - dev

# Sound notifications
# notifications:
#   sound: false
`

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a .maggus project in the current directory",
	Long: `Sets up the .maggus/ directory structure, creates a default config.yml,
updates .gitignore with required entries, and installs the maggus plugin
in Claude Code if available.

This is the recommended first step when setting up Maggus in a new project.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}

		maggusDir := filepath.Join(dir, ".maggus")

		// 1. Create .maggus/ directory
		if err := os.MkdirAll(maggusDir, 0o755); err != nil {
			return fmt.Errorf("create .maggus/: %w", err)
		}
		fmt.Println("Created .maggus/")

		// 2. Create config.yml if it doesn't exist
		configPath := filepath.Join(maggusDir, "config.yml")
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			if err := os.WriteFile(configPath, []byte(defaultConfig), 0o644); err != nil {
				return fmt.Errorf("create config.yml: %w", err)
			}
			fmt.Println("Created .maggus/config.yml")
		} else {
			fmt.Println("Skipped .maggus/config.yml (already exists)")
		}

		// 3. Update .gitignore
		added, err := gitignore.EnsureEntries(dir)
		if err != nil {
			return fmt.Errorf("update .gitignore: %w", err)
		}
		if len(added) > 0 {
			fmt.Printf("Updated .gitignore (+%d entries)\n", len(added))
		} else {
			fmt.Println("Skipped .gitignore (already up to date)")
		}

		// 4. Install maggus plugin in Claude Code if available
		if caps.HasClaude {
			if err := ensureMaggusPlugin(); err != nil {
				fmt.Printf("Warning: could not set up maggus plugin: %v\n", err)
			} else {
				fmt.Println("Maggus plugin ready in Claude Code")
			}
		}

		fmt.Println("\nDone! You can now:")
		fmt.Println("  maggus plan <description>   Create an implementation plan")
		fmt.Println("  maggus work                  Start working on tasks")

		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
