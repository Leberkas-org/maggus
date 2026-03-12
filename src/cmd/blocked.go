package cmd

import (
	"fmt"
	"os"

	"github.com/dirnei/maggus/internal/parser"
	"github.com/spf13/cobra"
)

var blockedCmd = &cobra.Command{
	Use:   "blocked",
	Short: "Interactive wizard to manage blocked tasks",
	Long: `Walks through each blocked task in your plan files and lets you
unblock, resolve, or skip each blocked criterion interactively.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}
		return runBlocked(cmd, dir)
	},
}

// collectBlockedTasks parses all active plans and returns only blocked tasks,
// ordered by plan file name then document order within each plan.
func collectBlockedTasks(dir string) ([]parser.Task, error) {
	tasks, err := parser.ParsePlans(dir)
	if err != nil {
		return nil, err
	}

	var blocked []parser.Task
	for _, t := range tasks {
		if t.IsBlocked() {
			blocked = append(blocked, t)
		}
	}
	return blocked, nil
}

func runBlocked(cmd *cobra.Command, dir string) error {
	out := cmd.OutOrStdout()

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		fmt.Fprintln(out, "No blocked tasks found.")
		return nil
	}

	blocked, err := collectBlockedTasks(dir)
	if err != nil {
		return err
	}

	if len(blocked) == 0 {
		fmt.Fprintln(out, "No blocked tasks found.")
		return nil
	}

	fmt.Fprintf(out, "Found %d blocked task(s).\n", len(blocked))
	return nil
}

func init() {
	rootCmd.AddCommand(blockedCmd)
}
