package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"github.com/dirnei/maggus/internal/config"
	"github.com/dirnei/maggus/internal/fingerprint"
	"github.com/dirnei/maggus/internal/gitbranch"
	"github.com/dirnei/maggus/internal/gitcommit"
	"github.com/dirnei/maggus/internal/gitignore"
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
	modelFlag       string
)

var workCmd = &cobra.Command{
	Use:   "work [count]",
	Short: "Work on the next N tasks from the implementation plan",
	Long: `Reads the implementation plan and works through the next N incomplete tasks
by prompting Claude Code. Defaults to 5 tasks if no count is specified.

Examples:
  maggus work        # work on the next 5 tasks
  maggus work 10     # work on the next 10 tasks
  maggus work -c 3   # work on the next 3 tasks
  maggus work --model opus   # override model for this run`,
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

		// Load config
		cfg, err := config.Load(dir)
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		// Validate includes: warn about missing files, skip them from prompt
		validIncludes := config.ValidateIncludes(cfg.Include, dir)
		for _, p := range cfg.Include {
			found := false
			for _, v := range validIncludes {
				if v == p {
					found = true
					break
				}
			}
			if !found {
				fmt.Fprintf(os.Stderr, "Warning: included file not found and will be skipped: %s\n", p)
			}
		}

		// Resolve model: CLI flag overrides config file
		modelInput := cfg.Model
		if modelFlag != "" {
			modelInput = modelFlag
		}
		resolvedModel := config.ResolveModel(modelInput)

		// Ensure .gitignore has required entries
		added, err := gitignore.EnsureEntries(dir)
		if err != nil {
			return fmt.Errorf("check gitignore: %w", err)
		}
		for _, entry := range added {
			fmt.Printf("Added to .gitignore: %s\n", entry)
		}

		tasks, err := parser.ParsePlans(dir)
		if err != nil {
			return fmt.Errorf("parse plans: %w", err)
		}

		if len(tasks) == 0 {
			fmt.Println("No plan files found in .maggus/")
			return nil
		}

		// Mark any fully completed plan files before checking for work
		if err := parser.MarkCompletedPlans(dir); err != nil {
			fmt.Printf("Warning: could not mark completed plans: %v\n", err)
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

		// Get host fingerprint
		hostFingerprint, err := fingerprint.Get()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not get host fingerprint: %v\n", err)
			hostFingerprint = "unknown"
		}

		// Create run tracker
		modelDisplay := resolvedModel
		if modelDisplay == "" {
			modelDisplay = "default"
		}
		run, err := runtracker.New(dir, modelDisplay, count)
		if err != nil {
			return fmt.Errorf("create run tracker: %w", err)
		}

		// Startup banner
		fmt.Println()
		fmt.Println("══════════════════════════════════════════")
		fmt.Printf("  Maggus Work Session (v%s)\n", Version)
		fmt.Println("══════════════════════════════════════════")
		fmt.Printf("  Model:        %s\n", modelDisplay)
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

		pauseCtx, pauseCancel := signal.NotifyContext(context.Background(), shutdownSignals...)

		select {
		case <-pauseCtx.Done():
			pauseCancel()
			fmt.Println("\nAborted.")
			return nil
		case <-time.After(3 * time.Second):
			// Stop intercepting signals so the next NotifyContext can take over
			pauseCancel()
		}

		fmt.Println()

		// Set up Ctrl+C handling for the work loop
		ctx, stop := signal.NotifyContext(context.Background(), shutdownSignals...)
		defer stop()

		completed := 0
		for i := 0; i < count; i++ {
			// Check if interrupted between iterations
			if ctx.Err() != nil {
				fmt.Println("Shutting down...")
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
				Include:     validIncludes,
				RunID:       run.ID,
				RunDir:      run.RelativeDir(dir),
				Iteration:   i + 1,
				IterLog:     run.RelativeIterationLogPath(dir, i+1),
			}

			p := prompt.Build(next, opts)
			if err := runner.RunClaude(ctx, stop, p, resolvedModel, Version, hostFingerprint, i+1, count); err != nil {
				if ctx.Err() != nil {
					fmt.Println("Shutting down...")
					break
				}
				return fmt.Errorf("task %s failed: %w", next.ID, err)
			}

			// Re-parse to pick up any changes the agent made
			tasks, err = parser.ParsePlans(dir)
			if err != nil {
				return fmt.Errorf("re-parse plans: %w", err)
			}

			// Rename fully completed plan files before committing so the rename is included
			if err := parser.MarkCompletedPlans(dir); err != nil {
				fmt.Printf("Warning: could not mark completed plans: %v\n", err)
			}

			// Stage any plan renames so they are included in the commit
			stagePlans := exec.Command("git", "add", "--", ".maggus/")
			stagePlans.Dir = dir
			stagePlans.CombinedOutput() // ignore errors

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

		// Push commits to remote
		if completed > 0 {
			fmt.Println("Pushing to remote...")
			// Get current branch name for --set-upstream
			branchCmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
			branchCmd.Dir = dir
			branchOut, branchErr := branchCmd.Output()
			var push *exec.Cmd
			if branchErr == nil {
				branch := strings.TrimSpace(string(branchOut))
				push = exec.Command("git", "push", "--set-upstream", "origin", branch)
			} else {
				push = exec.Command("git", "push")
			}
			push.Dir = dir
			push.Stdout = os.Stdout
			push.Stderr = os.Stderr
			if err := push.Run(); err != nil {
				fmt.Printf("Warning: git push failed: %v\n", err)
			} else {
				fmt.Println("Pushed successfully.")
			}
		}

		return nil
	},
}

func init() {
	workCmd.Flags().IntVarP(&countFlag, "count", "c", defaultTaskCount, "number of tasks to work on")
	workCmd.Flags().BoolVar(&noBootstrapFlag, "no-bootstrap", false, "skip reading CLAUDE.md/AGENTS.md/PROJECT_CONTEXT.md/TOOLING.md")
	workCmd.Flags().StringVar(&modelFlag, "model", "", "model to use (e.g. opus, sonnet, haiku, or a full model ID)")
	rootCmd.AddCommand(workCmd)
}
