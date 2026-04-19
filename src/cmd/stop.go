package cmd

import (
	"fmt"

	"github.com/leberkas-org/maggus/internal/config"
	"github.com/leberkas-org/maggus/internal/ipc"
	"github.com/spf13/cobra"
)

var stopAll bool

var stopCmd = &cobra.Command{
	Use:   "stop [repo]",
	Short: "Stop running tasks",
	Long:  "Stop tasks for a specific repo, or all tasks with -a/--all.",
	RunE: func(cmd *cobra.Command, args []string) error {
		globalDir, err := config.GlobalDir()
		if err != nil {
			return err
		}
		writer := ipc.NewFileCommandWriter(globalDir)

		if stopAll {
			if err := writer.StopAll(); err != nil {
				return fmt.Errorf("stop all: %w", err)
			}
			fmt.Println("Sent stop-all signal.")
			return nil
		}

		if len(args) == 0 {
			return fmt.Errorf("specify a repo or use --all")
		}

		if err := writer.StopRepo(args[0]); err != nil {
			return fmt.Errorf("stop repo: %w", err)
		}
		fmt.Printf("Sent stop signal for %s.\n", args[0])
		return nil
	},
}

func init() {
	stopCmd.Flags().BoolVarP(&stopAll, "all", "a", false, "Stop all tasks and daemon")
	rootCmd.AddCommand(stopCmd)
}
