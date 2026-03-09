package cmd

import (
	"fmt"
	"os"
	"strconv"

	"github.com/dirnei/maggus/internal/parser"
	"github.com/dirnei/maggus/internal/prompt"
	"github.com/dirnei/maggus/internal/runner"
	"github.com/spf13/cobra"
)

const defaultTaskCount = 5

var countFlag int

var workCmd = &cobra.Command{
	Use:   "work [count]",
	Short: "Work on the next N tasks from the implementation plan",
	Long: `Reads the implementation plan and works through the next N incomplete tasks
by prompting Claude Code. Defaults to 5 tasks if no count is specified.

Examples:
  maggus work        # work on the next 5 tasks
  maggus work 10     # work on the next 10 tasks
  maggus work -c 3   # work on the next 3 tasks`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		count := countFlag

		if len(args) > 0 {
			n, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid task count %q: must be a positive integer", args[0])
			}
			if n <= 0 {
				return fmt.Errorf("task count must be a positive integer, got %d", n)
			}
			count = n
		}

		dir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}

		tasks, err := parser.ParsePlans(dir)
		if err != nil {
			return fmt.Errorf("parse plans: %w", err)
		}

		if len(tasks) == 0 {
			fmt.Println("No plan files found in .maggus/")
			return nil
		}

		completed := 0
		for i := 0; i < count; i++ {
			next := parser.FindNextIncomplete(tasks)
			if next == nil {
				fmt.Println("All tasks are complete!")
				break
			}

			fmt.Printf("[%d/%d] Working on %s: %s...\n", i+1, count, next.ID, next.Title)

			p := prompt.Build(next)
			if err := runner.RunClaude(p); err != nil {
				return fmt.Errorf("task %s failed: %w", next.ID, err)
			}

			// Re-parse to pick up any changes the agent made
			tasks, err = parser.ParsePlans(dir)
			if err != nil {
				return fmt.Errorf("re-parse plans: %w", err)
			}

			completed++
		}

		// Count remaining incomplete tasks
		remaining := 0
		for _, t := range tasks {
			if !t.IsComplete() {
				remaining++
			}
		}

		fmt.Printf("Completed %d/%d tasks. %d tasks remaining.\n", completed, count, remaining)
		return nil
	},
}

func init() {
	workCmd.Flags().IntVarP(&countFlag, "count", "c", defaultTaskCount, "number of tasks to work on")
	rootCmd.AddCommand(workCmd)
}
