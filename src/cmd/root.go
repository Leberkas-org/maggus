package cmd

import (
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"
)

var Version = "dev"

var BuildTime = ""

func init() {
	if Version == "dev" && BuildTime != "" {
		Version = "dev-" + BuildTime
	}
	rootCmd.Version = Version
}

var rootCmd = &cobra.Command{
	Use:   "maggus",
	Short: "Your best and worst co-worker — a junior dev that just works",
	RunE: func(cmd *cobra.Command, args []string) error {
		if !term.IsTerminal(os.Stdout.Fd()) {
			return cmd.Help()
		}
		m := newMainModel()
		p := tea.NewProgram(m, tea.WithAltScreen())
		_, err := p.Run()
		return err
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
