package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "maggus",
	Short: "Your best and worst co-worker — a junior dev that just works",
	Long: `Maggus reads implementation plans and works through tasks one-by-one
by prompting an AI agent (Claude Code). Provide a plan and let Maggus work.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
