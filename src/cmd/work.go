package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dirnei/maggus/internal/config"
	"github.com/dirnei/maggus/internal/fingerprint"
	"github.com/dirnei/maggus/internal/gitbranch"
	"github.com/dirnei/maggus/internal/gitcommit"
	"github.com/dirnei/maggus/internal/gitignore"
	"github.com/dirnei/maggus/internal/parser"
	"github.com/dirnei/maggus/internal/prompt"
	"github.com/dirnei/maggus/internal/runner"
	"github.com/dirnei/maggus/internal/runtracker"
	"github.com/dirnei/maggus/internal/tasklock"
	"github.com/dirnei/maggus/internal/worktree"
	"github.com/spf13/cobra"
)

const defaultTaskCount = 5

var (
	countFlag       int
	noBootstrapFlag bool
	modelFlag       string
	worktreeFlag    bool
	noWorktreeFlag  bool
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

		// Resolve worktree mode: --no-worktree > --worktree > config > default (false)
		useWorktree := cfg.Worktree
		if worktreeFlag {
			useWorktree = true
		}
		if noWorktreeFlag {
			useWorktree = false
		}

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

		// repoDir always points to the original repository root.
		// workDir is where Claude Code operates — either the worktree or the repo itself.
		repoDir := dir
		workDir := dir

		if useWorktree {
			// Create worktree at .maggus-work/<run-id>/ on a new feature branch
			branchName := gitbranch.FeatureBranchName(nextTask.ID)
			wtPath := filepath.Join(repoDir, ".maggus-work", run.ID)
			if err := worktree.Create(repoDir, wtPath, branchName); err != nil {
				return fmt.Errorf("create worktree: %w", err)
			}
			workDir = wtPath

			// Deferred cleanup: remove worktree on exit (best-effort on interrupt).
			defer func() {
				fmt.Fprintf(os.Stderr, "Cleaning up worktree at %s...\n", wtPath)
				if rmErr := worktree.Remove(repoDir, wtPath); rmErr != nil {
					fmt.Fprintf(os.Stderr, "Warning: could not remove worktree: %v\n", rmErr)
				}
			}()
		} else {
			// Non-worktree mode: check for protected branch and create feature branch if needed
			_, msg, err := gitbranch.EnsureFeatureBranch(dir, nextTask.ID)
			if err != nil {
				return fmt.Errorf("ensure feature branch: %w", err)
			}
			fmt.Println(msg)
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
		if useWorktree {
			fmt.Printf("  Worktree:     %s\n", workDir)
		}
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

		// Set up signal handling for the work loop.
		// During bubbletea's alt-screen (raw mode), Ctrl+C is captured as a key
		// event rather than generating SIGINT. The TUI's KeyCtrlC handler calls
		// workCancel() to cancel the work context. The signal handler covers
		// the time between iterations when bubbletea is not consuming input.
		sigCtx, sigStop := signal.NotifyContext(context.Background(), shutdownSignals...)
		defer sigStop()

		workCtx, workCancel := context.WithCancel(context.Background())
		defer workCancel()

		// Propagate signal cancellation to the work context.
		go func() {
			<-sigCtx.Done()
			sigStop()    // reset signals so second Ctrl+C force-kills
			workCancel() // cancel the work context
		}()

		// TUI cancel function: resets signals for force-quit + cancels work context.
		tuiCancel := func() {
			sigStop()
			workCancel()
		}

		// Create the persistent TUI that lives across all iterations.
		m := runner.NewTUIModel(resolvedModel, Version, hostFingerprint, tuiCancel)
		p := tea.NewProgram(m, tea.WithAltScreen())

		// Run the work loop in a goroutine, sending events to the TUI.
		var workErr error
		completed := 0
		go func() {
			defer func() {
				p.Send(runner.QuitMsg{})
			}()

			for i := 0; i < count; i++ {
				if workCtx.Err() != nil {
					break
				}

				// Find next workable task. In worktree mode, skip locked tasks.
				var next *parser.Task
				if useWorktree {
					next = findNextUnlocked(tasks, repoDir)
				} else {
					next = parser.FindNextIncomplete(tasks)
				}
				if next == nil {
					break
				}

				// Acquire task lock in worktree mode.
				var lock tasklock.Lock
				if useWorktree {
					var lockErr error
					lock, lockErr = tasklock.Acquire(repoDir, next.ID, run.ID)
					if lockErr != nil {
						// Another session grabbed it between check and acquire; retry.
						i--
						continue
					}
				}

				// Signal iteration start to the TUI (resets per-iteration state).
				p.Send(runner.IterationStartMsg{
					Current:   i + 1,
					Total:     count,
					TaskID:    next.ID,
					TaskTitle: next.Title,
				})

				opts := prompt.Options{
					NoBootstrap: noBootstrapFlag,
					Include:     validIncludes,
					RunID:       run.ID,
					RunDir:      run.RelativeDir(workDir),
					Iteration:   i + 1,
					IterLog:     run.RelativeIterationLogPath(workDir, i+1),
				}

				builtPrompt := prompt.Build(next, opts)
				if err := runner.RunClaude(workCtx, builtPrompt, resolvedModel, p); err != nil {
					if useWorktree {
						lock.Release()
					}
					if workCtx.Err() != nil {
						break
					}
					workErr = fmt.Errorf("task %s failed: %w", next.ID, err)
					return
				}

				// Re-parse to pick up any changes the agent made
				var parseErr error
				tasks, parseErr = parser.ParsePlans(workDir)
				if parseErr != nil {
					if useWorktree {
						lock.Release()
					}
					workErr = fmt.Errorf("re-parse plans: %w", parseErr)
					return
				}

				// Rename fully completed plan files before committing
				if markErr := parser.MarkCompletedPlans(workDir); markErr != nil {
					// non-fatal
				}

				// Stage any plan renames so they are included in the commit
				stagePlans := exec.Command("git", "add", "--", ".maggus/")
				stagePlans.Dir = workDir
				stagePlans.CombinedOutput() // ignore errors

				// Commit using COMMIT.md
				commitResult, commitErr := gitcommit.CommitIteration(workDir)
				if commitErr != nil {
					if useWorktree {
						lock.Release()
					}
					workErr = fmt.Errorf("commit after %s: %w", next.ID, commitErr)
					return
				}
				if commitResult.Committed {
					p.Send(runner.CommitMsg{Message: commitResult.Message})
				}

				// Release task lock after successful commit.
				if useWorktree {
					lock.Release()
				}

				completed++

				// Update progress to reflect completed iteration.
				p.Send(runner.ProgressMsg{Current: i + 1, Total: count})
			}
		}()

		// Run the TUI (blocks until QuitMsg or Ctrl+C).
		if _, tuiErr := p.Run(); tuiErr != nil {
			return fmt.Errorf("TUI error: %w", tuiErr)
		}

		if workErr != nil {
			return workErr
		}

		// Finalize run log
		if err := run.Finalize(workDir); err != nil {
			fmt.Printf("Warning: could not finalize run log: %v\n", err)
		}

		// Print summary banner
		run.PrintSummary(workDir)

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

		// Push commits to remote (from workDir, which may be a worktree).
		// In worktree mode, push happens before the deferred worktree removal.
		if completed > 0 {
			fmt.Println("Pushing to remote...")
			branchCmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
			branchCmd.Dir = workDir
			branchOut, branchErr := branchCmd.Output()
			var push *exec.Cmd
			if branchErr == nil {
				branch := strings.TrimSpace(string(branchOut))
				push = exec.Command("git", "push", "--set-upstream", "origin", branch)
			} else {
				push = exec.Command("git", "push")
			}
			push.Dir = workDir
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
	workCmd.Flags().BoolVar(&worktreeFlag, "worktree", false, "run in an isolated git worktree")
	workCmd.Flags().BoolVar(&noWorktreeFlag, "no-worktree", false, "force disable worktree mode (overrides config)")
	rootCmd.AddCommand(workCmd)
}

// findNextUnlocked returns the first workable task that is not locked by another session.
func findNextUnlocked(tasks []parser.Task, repoDir string) *parser.Task {
	for i := range tasks {
		if tasks[i].IsWorkable() && !tasklock.IsLocked(repoDir, tasks[i].ID) {
			return &tasks[i]
		}
	}
	return nil
}
