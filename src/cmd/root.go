package cmd

import (
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"
)

// Version is set at build time via -ldflags.
var Version = "dev"

var rootCmd = &cobra.Command{
	Use:     "maggus",
	Short:   "Your best and worst co-worker — a junior dev that just works",
	Version: Version,
	Long: `Maggus reads implementation plans and works through tasks one-by-one
by prompting an AI agent (Claude Code). Provide a plan and let Maggus work.`,
}

func init() {
	rootCmd.RunE = runMenu
}

func runMenu(cmd *cobra.Command, args []string) error {
	if !term.IsTerminal(os.Stdout.Fd()) {
		return cmd.Help()
	}

	m := newMenuModel(loadPlanSummary())
	p := tea.NewProgram(m, tea.WithAltScreen())
	result, err := p.Run()
	if err != nil {
		return err
	}

	final := result.(menuModel)
	if final.quitting || final.selected == "" {
		return nil
	}

	cmdArgs := append([]string{final.selected}, final.args...)
	sub, remaining, err := rootCmd.Find(cmdArgs)
	if err != nil {
		return err
	}
	sub.ParseFlags(remaining)
	return sub.RunE(sub, sub.Flags().Args())
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
