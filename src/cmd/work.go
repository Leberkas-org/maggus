package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"time"

	"github.com/dirnei/maggus/internal/gitbranch"
	"github.com/dirnei/maggus/internal/gitcommit"
	"github.com/dirnei/maggus/internal/parser"
	"github.com/dirnei/maggus/internal/prompt"
	"github.com/dirnei/maggus/internal/runner"
	"github.com/dirnei/maggus/internal/runtracker"
	"github.com/spf13/cobra"
)

const defaultTaskCount = 5

var (
	countFlag       int
	noBootstrapFlag bool
)

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

		// Check if there are any incomplete tasks
		nextTask := parser.FindNextIncomplete(tasks)
		if nextTask == nil {
			fmt.Println("All tasks are complete! Nothing to do.")
			return nil
		}

		// Check for protected branch and create feature branch if needed
		{
			branch, msg, err := gitbranch.EnsureFeatureBranch(dir, nextTask.ID)
			if err != nil {
				return fmt.Errorf("ensure feature branch: %w", err)
			}
			fmt.Println(msg)
			_ = branch
		}

		// Create run tracker
		run, err := runtracker.New(dir, "claude", count)
		if err != nil {
			return fmt.Errorf("create run tracker: %w", err)
		}

		// Startup banner
		fmt.Println()
		fmt.Println("══════════════════════════════════════════")
		fmt.Println("  Maggus Work Session")
		fmt.Println("══════════════════════════════════════════")
		fmt.Printf("  Iterations:   %d\n", count)
		fmt.Printf("  Branch:       %s\n", run.Branch)
		fmt.Printf("  Run ID:       %s\n", run.ID)
		fmt.Printf("  Run Dir:      %s\n", run.RelativeDir(dir))
		fmt.Printf("  Permissions:  --dangerously-skip-permissions\n")
		fmt.Println("══════════════════════════════════════════")
		fmt.Println()
		fmt.Println("WARNING: Running with --dangerously-skip-permissions")
		fmt.Println()
		fmt.Println("Press Ctrl+C within 3 seconds to abort...")

		pauseCtx, pauseCancel := signal.NotifyContext(context.Background(), os.Interrupt)
		defer pauseCancel()

		select {
		case <-pauseCtx.Done():
			pauseCancel()
			// Reset signal handling so future Ctrl+C works normally
			fmt.Println("\nAborted.")
			return nil
		case <-time.After(3 * time.Second):
			pauseCancel()
		}

		fmt.Println()

		// Set up Ctrl+C handling for the work loop
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
		defer cancel()

		completed := 0
		for i := 0; i < count; i++ {
			// Check if interrupted between iterations
			if ctx.Err() != nil {
				fmt.Println("\nInterrupted. Stopping loop.")
				break
			}

			next := parser.FindNextIncomplete(tasks)
			if next == nil {
				fmt.Println("All tasks are complete!")
				break
			}

			fmt.Printf("========== Iteration %d of %d ==========\n", i+1, count)
			fmt.Printf("Working on %s: %s...\n", next.ID, next.Title)

			opts := prompt.Options{
				NoBootstrap: noBootstrapFlag,
				RunID:       run.ID,
				RunDir:      run.RelativeDir(dir),
				Iteration:   i + 1,
				IterLog:     run.RelativeIterationLogPath(dir, i+1),
			}

			p := prompt.Build(next, opts)
			if err := runner.RunClaude(ctx, p); err != nil {
				if err == runner.ErrInterrupted {
					fmt.Println("\nInterrupted. Stopping loop.")
					break
				}
				return fmt.Errorf("task %s failed: %w", next.ID, err)
			}

			// Commit using COMMIT.md
			commitResult, err := gitcommit.CommitIteration(dir)
			if err != nil {
				return fmt.Errorf("commit after %s: %w", next.ID, err)
			}
			if commitResult.Committed {
				fmt.Printf("Committed: %s\n", commitResult.Message)
			} else {
				fmt.Println(commitResult.Message)
			}

			// Re-parse to pick up any changes the agent made
			tasks, err = parser.ParsePlans(dir)
			if err != nil {
				return fmt.Errorf("re-parse plans: %w", err)
			}

			completed++
		}

		// Finalize run log
		if err := run.Finalize(dir); err != nil {
			fmt.Printf("Warning: could not finalize run log: %v\n", err)
		}

		// Print summary banner
		run.PrintSummary(dir)

		// Collect remaining incomplete tasks
		var remaining []string
		for _, t := range tasks {
			if !t.IsComplete() {
				remaining = append(remaining, t.Title)
			}
		}

		if len(remaining) > 0 {
			fmt.Println("Remaining incomplete tasks:")
			limit := len(remaining)
			if limit > 5 {
				limit = 5
			}
			for _, title := range remaining[:limit] {
				fmt.Printf("  - %s\n", title)
			}
			if len(remaining) > 5 {
				fmt.Printf("  ... and %d more\n", len(remaining)-5)
			}
		}

		fmt.Printf("Completed %d/%d tasks. %d tasks remaining.\n", completed, count, len(remaining))
		return nil
	},
}

func init() {
	workCmd.Flags().IntVarP(&countFlag, "count", "c", defaultTaskCount, "number of tasks to work on")
	workCmd.Flags().BoolVar(&noBootstrapFlag, "no-bootstrap", false, "skip reading CLAUDE.md/AGENTS.md/PROJECT_CONTEXT.md/TOOLING.md")
	rootCmd.AddCommand(workCmd)
}
