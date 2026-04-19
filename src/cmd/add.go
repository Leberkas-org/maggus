package cmd

import (
	"fmt"

	"github.com/leberkas-org/maggus/internal/config"
	"github.com/spf13/cobra"
)

var addCmd = &cobra.Command{
	Use:   "add <path>",
	Short: "Register a repository with maggus",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := config.AddRepo(args[0]); err != nil {
			return err
		}
		fmt.Printf("Repository added: %s\n", args[0])
		return nil
	},
}

func init() {
	rootCmd.AddCommand(addCmd)
}
