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

	tea "github.com/charmbracelet/bubbletea"
	"github.com/leberkas-org/maggus/internal/agent"
	"github.com/leberkas-org/maggus/internal/claude2x"
	"github.com/leberkas-org/maggus/internal/config"
	"github.com/leberkas-org/maggus/internal/fingerprint"
	"github.com/leberkas-org/maggus/internal/gitbranch"
	"github.com/leberkas-org/maggus/internal/gitcommit"
	"github.com/leberkas-org/maggus/internal/gitignore"
	"github.com/leberkas-org/maggus/internal/gitsync"
	"github.com/leberkas-org/maggus/internal/notify"
	"github.com/leberkas-org/maggus/internal/parser"
	"github.com/leberkas-org/maggus/internal/prompt"
	"github.com/leberkas-org/maggus/internal/runner"
	"github.com/leberkas-org/maggus/internal/runtracker"
	"github.com/leberkas-org/maggus/internal/tasklock"
	"github.com/leberkas-org/maggus/internal/usage"
	"github.com/leberkas-org/maggus/internal/worktree"
	"github.com/spf13/cobra"
)

const defaultTaskCount = 5

var (
	countFlag       int
	noBootstrapFlag bool
	modelFlag       string
	agentFlag       string
	taskFlag        string
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

		if taskFlag != "" {
			count = 1
		} else if len(args) > 0 {
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

		// Validate includes: collect warnings for missing files, skip them from prompt
		validIncludes := config.ValidateIncludes(cfg.Include, dir)
		var includeWarnings []string
		for _, inc := range cfg.Include {
			found := false
			for _, v := range validIncludes {
				if v == inc {
					found = true
					break
				}
			}
			if !found {
				includeWarnings = append(includeWarnings, fmt.Sprintf("Warning: included file not found: %s", inc))
			}
		}

		// Resolve agent: CLI flag > config > default ("claude")
		agentName := cfg.Agent
		if agentFlag != "" {
			agentName = agentFlag
		}
		activeAgent, err := agent.New(agentName)
		if err != nil {
			return err
		}

		// Validate agent CLI is installed before starting work
		if err := activeAgent.Validate(); err != nil {
			return fmt.Errorf("agent %q not available: %w", activeAgent.Name(), err)
		}

		// Resolve model: CLI flag overrides config file
		modelInput := cfg.Model
		if modelFlag != "" {
			modelInput = modelFlag
		}
		resolvedModel := config.ResolveModel(modelInput)

		// Create notifier for sound notifications.
		notifier := notify.New(cfg.Notifications)

		// Resolve worktree mode: --no-worktree > --worktree > config > default (false)
		useWorktree := cfg.Worktree
		if worktreeFlag {
			useWorktree = true
		}
		if noWorktreeFlag {
			useWorktree = false
		}

		// Ensure .gitignore has required entries
		if _, err := gitignore.EnsureEntries(dir); err != nil {
			return fmt.Errorf("check gitignore: %w", err)
		}

		// Get host fingerprint
		hostFingerprint, _ := fingerprint.Get()
		if hostFingerprint == "" {
			hostFingerprint = "unknown"
		}

		modelDisplay := resolvedModel
		if modelDisplay == "" {
			modelDisplay = "default"
		}

		// repoDir always points to the original repository root.
		// workDir is where Claude Code operates — either the worktree or the repo itself.
		repoDir := dir
		workDir := dir

		// Git sync check: detect remote changes and uncommitted work before starting.
		syncDir := dir
		var syncInfoMsg string

		fetchErr := gitsync.FetchRemote(syncDir)
		remoteStatus, _ := gitsync.RemoteStatus(syncDir)
		workTreeStatus, _ := gitsync.WorkingTreeStatus(syncDir)

		hasDirty := workTreeStatus.HasUncommittedChanges || workTreeStatus.HasUntrackedFiles
		isBehind := remoteStatus.HasRemote && remoteStatus.Behind > 0

		if !remoteStatus.HasRemote {
			// No remote configured: silently skip
		} else if isBehind || hasDirty {
			// Behind remote or uncommitted changes: show interactive sync TUI
			result, syncErr := runGitSyncTUI(syncDir)
			if syncErr != nil {
				return syncErr
			}
			if result.action == syncAbort {
				return nil
			}
			if result.message != "" {
				syncInfoMsg = result.message
			}
		} else if fetchErr != nil {
			syncInfoMsg = "⚠ Could not reach remote — working offline"
		} else {
			syncInfoMsg = fmt.Sprintf("✓ Branch up to date with %s", remoteStatus.RemoteBranch)
		}

		// Run-again loop: allows the user to start another batch from the summary screen.
		for {
			tasks, err := parser.ParsePlans(dir)
			if err != nil {
				return fmt.Errorf("parse plans: %w", err)
			}

			if len(tasks) == 0 {
				cmd.Println("No plan files found in .maggus/")
				return nil
			}

			// Mark any fully completed plan files before checking for work
			_ = parser.MarkCompletedPlans(dir)

			// Check if there are any tasks to work on.
			// When --task targets a specific task, look for that task directly;
			// otherwise find the next workable (incomplete + not blocked) task.
			var nextTask *parser.Task
			if taskFlag != "" {
				nextTask = findTaskByID(tasks, taskFlag)
				if nextTask == nil {
					cmd.Printf("Task %s not found or already complete.\n", taskFlag)
					return nil
				}
			} else {
				nextTask = parser.FindNextIncomplete(tasks)
				if nextTask == nil {
					// Distinguish between "all complete" and "remaining are blocked/ignored".
					hasIgnored := false
					hasBlocked := false
					for i := range tasks {
						if !tasks[i].IsComplete() {
							if tasks[i].Ignored {
								hasIgnored = true
							}
							if tasks[i].IsBlocked() {
								hasBlocked = true
							}
						}
					}
					if hasIgnored || hasBlocked {
						msg := "No workable tasks — remaining tasks are"
						switch {
						case hasBlocked && hasIgnored:
							msg += " blocked or ignored."
						case hasIgnored:
							msg += " ignored."
						default:
							msg += " blocked."
						}
						cmd.Println(msg)
					} else {
						cmd.Println("All tasks are complete! Nothing to do.")
					}
					return nil
				}
			}

			// Cap count to the number of workable tasks so the progress bar is accurate.
			// When --task is set, always allow at least 1 (the user explicitly chose it).
			if taskFlag != "" {
				count = 1
			} else {
				workable := 0
				for i := range tasks {
					if tasks[i].IsWorkable() {
						workable++
					}
				}
				if workable < count {
					count = workable
				}
			}

			// Create run tracker
			run, err := runtracker.New(dir, modelDisplay, count)
			if err != nil {
				return fmt.Errorf("create run tracker: %w", err)
			}

			var branchMsg string

			if useWorktree {
				// Clean up stale worktrees from previous crashed runs.
				cleanStaleWorktrees(repoDir)

				// Create worktree at .maggus-work/<run-id>/ on a new feature branch
				branchName := gitbranch.FeatureBranchName(nextTask.ID)
				wtPath := filepath.Join(repoDir, ".maggus-work", run.ID)
				if err := worktree.Create(repoDir, wtPath, branchName); err != nil {
					return fmt.Errorf("create worktree: %w", err)
				}
				workDir = wtPath

				// Deferred cleanup: remove worktree on exit (best-effort on interrupt).
				defer func() {
					_ = worktree.Remove(repoDir, wtPath)
				}()
			} else {
				// Non-worktree mode: check for protected branch and create feature branch if needed
				_, msg, err := gitbranch.EnsureFeatureBranch(dir, nextTask.ID)
				if err != nil {
					return fmt.Errorf("ensure feature branch: %w", err)
				}
				branchMsg = msg
			}

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

			// Fetch 2x status (non-blocking with 3s timeout inside FetchStatus).
			twoXStatus := claude2x.FetchStatus()

			// Create the persistent TUI with banner info — starts immediately, no countdown.
			banner := runner.BannerInfo{
				Iterations: count,
				Branch:     run.Branch,
				RunID:      run.ID,
				RunDir:     run.RelativeDir(dir),
				Agent:      activeAgent.Name(),
			}
			if useWorktree {
				banner.Worktree = workDir
			}
			if twoXStatus.Is2x {
				banner.TwoXExpiresIn = twoXStatus.TwoXWindowExpiresIn
			}
			// Wire gitsync functions into the runner TUI for between-task sync checks.
			runner.InitSyncFuncs(gitsync.Pull, gitsync.PullRebase, gitsync.ForcePull)

			m := runner.NewTUIModel(resolvedModel, Version, hostFingerprint, tuiCancel, banner)
			m.SetSyncDir(workDir)
			stopFlag := m.StopFlag()
			m.SetOnTaskUsage(func(tu runner.TaskUsage) {
				planRel := tu.PlanFile
				if rel, err := filepath.Rel(dir, tu.PlanFile); err == nil {
					planRel = rel
				}
				_ = usage.Append(dir, []usage.Record{{
					RunID:        run.ID,
					TaskID:       tu.TaskID,
					TaskTitle:    tu.TaskTitle,
					PlanFile:     planRel,
					Model:        modelDisplay,
					Agent:        activeAgent.Name(),
					InputTokens:  tu.InputTokens,
					OutputTokens: tu.OutputTokens,
					StartTime:    tu.StartTime,
					EndTime:      tu.EndTime,
				}})
			})
			p := tea.NewProgram(m, tea.WithAltScreen())

			// Capture the starting commit hash before work begins.
			startHashCmd := exec.Command("git", "rev-parse", "--short", "HEAD")
			startHashCmd.Dir = workDir
			startHashBytes, _ := startHashCmd.Output()
			startHash := strings.TrimSpace(string(startHashBytes))

			// Run the work loop in a goroutine, sending events to the TUI.
			var workErr error
			completed := 0
			go func() {
				defer func() {
					// Finalize run log before signaling done.
					_ = run.Finalize(workDir)
					p.Send(runner.QuitMsg{})
				}()

				// Send startup info messages to the TUI.
				if activeAgent.Name() == "claude" {
					p.Send(runner.InfoMsg{Text: "⚠ Using --dangerously-skip-permissions (Claude Code)"})
				}
				for _, w := range includeWarnings {
					p.Send(runner.InfoMsg{Text: w})
				}
				if branchMsg != "" {
					p.Send(runner.InfoMsg{Text: branchMsg})
				}
				if syncInfoMsg != "" {
					p.Send(runner.InfoMsg{Text: syncInfoMsg})
				}

				stopReason := runner.StopReasonComplete
				var errorDetail string
				var warnings []string

				// Log ignored tasks at the start so the user knows what's being skipped.
				for i := range tasks {
					if !tasks[i].IsComplete() && tasks[i].Ignored {
						p.Send(runner.InfoMsg{Text: fmt.Sprintf("Skipping %s: ignored", tasks[i].ID)})
					}
				}

				for i := 0; i < count; i++ {
					if workCtx.Err() != nil {
						stopReason = runner.StopReasonInterrupted
						break
					}
					// Check if user requested stop after previous task
					if i > 0 && stopFlag.Load() {
						stopReason = runner.StopReasonUserStop
						break
					}

					// Find next workable task. --task targets a specific ID; otherwise pick next incomplete.
					var next *parser.Task
					if taskFlag != "" {
						next = findTaskByID(tasks, taskFlag)
					} else if useWorktree {
						next = findNextUnlocked(tasks, repoDir)
					} else {
						next = parser.FindNextIncomplete(tasks)
					}
					if next == nil {
						if completed == 0 {
							stopReason = runner.StopReasonNoTasks
						}
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
					// Convert parser criteria to runner criteria for the TUI.
					tuiCriteria := make([]runner.TaskCriterion, len(next.Criteria))
					for ci, c := range next.Criteria {
						tuiCriteria[ci] = runner.TaskCriterion{
							Text:    c.Text,
							Checked: c.Checked,
							Blocked: c.Blocked,
						}
					}
					p.Send(runner.IterationStartMsg{
						Current:         i + 1,
						Total:           count,
						TaskID:          next.ID,
						TaskTitle:       next.Title,
						PlanFile:        next.SourceFile,
						TaskDescription: next.Description,
						TaskCriteria:    tuiCriteria,
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
					if err := activeAgent.Run(workCtx, builtPrompt, resolvedModel, p); err != nil {
						if useWorktree {
							lock.Release()
						}
						if workCtx.Err() != nil {
							stopReason = runner.StopReasonInterrupted
							break
						}
						notifier.PlayError()
						stopReason = runner.StopReasonError
						errorDetail = fmt.Sprintf("task %s failed: %v", next.ID, err)
						workErr = fmt.Errorf("%s", errorDetail)
						return
					}

					// Re-parse to pick up any changes the agent made
					var parseErr error
					tasks, parseErr = parser.ParsePlans(workDir)
					if parseErr != nil {
						if useWorktree {
							lock.Release()
						}
						stopReason = runner.StopReasonError
						errorDetail = fmt.Sprintf("re-parse plans: %v", parseErr)
						workErr = fmt.Errorf("%s", errorDetail)
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
					commitResult, commitErr := gitcommit.CommitIteration(workDir, next.ID+": "+next.Title)
					if commitErr != nil {
						if useWorktree {
							lock.Release()
						}
						stopReason = runner.StopReasonError
						errorDetail = fmt.Sprintf("commit after %s: %v", next.ID, commitErr)
						workErr = fmt.Errorf("%s", errorDetail)
						return
					}
					if commitResult.Committed {
						p.Send(runner.CommitMsg{Message: commitResult.Message})
						notifier.PlayTaskComplete()
						completed++
					} else {
						msg := commitResult.Message
						if msg == "" {
							msg = "commit skipped (unknown reason)"
						}
						w := fmt.Sprintf("%s: %s", next.ID, msg)
						warnings = append(warnings, w)
						p.Send(runner.InfoMsg{Text: "⚠ " + w})
					}

					// Release task lock after successful commit.
					if useWorktree {
						lock.Release()
					}

					// Update progress to reflect completed iteration.
					p.Send(runner.ProgressMsg{Current: i + 1, Total: count})

					// Between-task sync check: detect if remote changed while working.
					// Skip on the final iteration (push happens next anyway).
					if i < count-1 {
						if workCtx.Err() != nil {
							break
						}
						fetchErr := gitsync.FetchRemote(workDir)
						if fetchErr != nil {
							// Fetch failed (offline) — warn and continue
							p.Send(runner.InfoMsg{Text: "⚠ Could not reach remote between tasks — continuing offline"})
						} else {
							rs, rsErr := gitsync.RemoteStatus(workDir)
							if rsErr == nil && rs.HasRemote && rs.Behind > 0 {
								// Remote is ahead — show sync TUI and block until resolved
								resultCh := make(chan runner.SyncCheckResult, 1)
								p.Send(runner.SyncCheckMsg{
									Behind:       rs.Behind,
									Ahead:        rs.Ahead,
									RemoteBranch: rs.RemoteBranch,
									ResultCh:     resultCh,
								})
								// Wait for user's choice (or context cancellation)
								select {
								case result := <-resultCh:
									if result.Action == runner.SyncAbort {
										stopReason = runner.StopReasonInterrupted
										break
									}
									if result.Message != "" {
										p.Send(runner.InfoMsg{Text: result.Message})
									}
								case <-workCtx.Done():
									stopReason = runner.StopReasonInterrupted
								}
								if stopReason == runner.StopReasonInterrupted {
									break
								}
							}
							// Up-to-date: continue without interruption
						}
					}
				}

				// If nothing was accomplished, surface a meaningful reason.
				if completed == 0 && stopReason == runner.StopReasonComplete {
					if len(warnings) > 0 {
						stopReason = runner.StopReasonError
						errorDetail = "agent ran but produced no commits"
					} else {
						stopReason = runner.StopReasonNoTasks
						// Add diagnostic detail about task states.
						total, done, blocked, ignored := 0, 0, 0, 0
						for i := range tasks {
							total++
							switch {
							case tasks[i].IsComplete():
								done++
							case tasks[i].IsBlocked():
								blocked++
							case tasks[i].Ignored:
								ignored++
							}
						}
						errorDetail = fmt.Sprintf(
							"tasks: %d total, %d complete, %d blocked, %d ignored, count=%d",
							total, done, blocked, ignored, count)
					}
				}

				// Build summary data.
				endHashCmd := exec.Command("git", "rev-parse", "--short", "HEAD")
				endHashCmd.Dir = workDir
				endHashBytes, _ := endHashCmd.Output()
				endHash := strings.TrimSpace(string(endHashBytes))

				// Determine current branch name.
				branchNameCmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
				branchNameCmd.Dir = workDir
				branchNameOut, _ := branchNameCmd.Output()
				currentBranch := strings.TrimSpace(string(branchNameOut))

				// Collect remaining incomplete tasks.
				var remaining []runner.RemainingTask
				latestTasks, _ := parser.ParsePlans(workDir)
				for _, t := range latestTasks {
					if t.IsWorkable() {
						remaining = append(remaining, runner.RemainingTask{ID: t.ID, Title: t.Title})
					}
				}

				summaryData := runner.SummaryData{
					RunID:          run.ID,
					Branch:         currentBranch,
					Model:          modelDisplay,
					StartTime:      run.StartTime,
					TasksCompleted: completed,
					TasksTotal:     count,
					CommitStart:    startHash,
					CommitEnd:      endHash,
					RemainingTasks: remaining,
					Reason:         stopReason,
					ErrorDetail:    errorDetail,
					Warnings:       warnings,
				}

				// Notify run complete.
				notifier.PlayRunComplete()

				// Show summary view, then push in the background.
				p.Send(runner.SummaryMsg{Data: summaryData})

				// Push commits to remote in the background while summary is displayed.
				if completed > 0 {
					var push *exec.Cmd
					if currentBranch != "" {
						push = exec.Command("git", "push", "--set-upstream", "origin", currentBranch)
					} else {
						push = exec.Command("git", "push")
					}
					push.Dir = workDir
					if pushOut, pushErr := push.CombinedOutput(); pushErr != nil {
						p.Send(runner.PushStatusMsg{
							Status: fmt.Sprintf("Push failed: %v", pushErr),
							Done:   true,
						})
					} else {
						msg := fmt.Sprintf("Pushed to origin/%s", currentBranch)
						if s := strings.TrimSpace(string(pushOut)); s != "" {
							_ = s // push output available but branch name is more useful
						}
						p.Send(runner.PushStatusMsg{Status: msg, Done: true})
					}
				} else {
					p.Send(runner.PushStatusMsg{Status: "Nothing to push", Done: true})
				}
			}()

			// Run the TUI (blocks until user dismisses the summary screen).
			finalModel, tuiErr := p.Run()
			if tuiErr != nil {
				return fmt.Errorf("TUI error: %w", tuiErr)
			}

			if workErr != nil {
				return workErr
			}

			// Usage is now written per-task via the onTaskUsage callback.
			if tm, ok := finalModel.(runner.TUIModel); ok {
				// Check if user chose "Run again" from the summary menu.
				result := tm.Result()
				if result.RunAgain {
					count = result.TaskCount
					taskFlag = ""            // clear so next batch finds the next workable task
					workDir = dir // reset workDir for next iteration
					continue
				}
			}

			return nil
		} // end run-again loop
	},
}

func init() {
	workCmd.Flags().IntVarP(&countFlag, "count", "c", defaultTaskCount, "number of tasks to work on")
	workCmd.Flags().BoolVar(&noBootstrapFlag, "no-bootstrap", false, "skip reading CLAUDE.md/AGENTS.md/PROJECT_CONTEXT.md/TOOLING.md")
	workCmd.Flags().StringVar(&modelFlag, "model", "", "model to use (e.g. opus, sonnet, haiku, or a full model ID)")
	workCmd.Flags().StringVar(&agentFlag, "agent", "", "agent to use (e.g. claude, opencode)")
	workCmd.Flags().StringVar(&taskFlag, "task", "", "run a specific task by ID (e.g. TASK-001)")
	workCmd.Flags().BoolVar(&worktreeFlag, "worktree", false, "run in an isolated git worktree")
	workCmd.Flags().BoolVar(&noWorktreeFlag, "no-worktree", false, "force disable worktree mode (overrides config)")
	rootCmd.AddCommand(workCmd)
}

// cleanStaleWorktrees removes worktrees in .maggus-work/ whose lock files are
// all stale (older than 2 hours), indicating a crashed or interrupted session.
func cleanStaleWorktrees(repoDir string) {
	workDir := filepath.Join(repoDir, maggusWorkDir)
	entries, err := os.ReadDir(workDir)
	if err != nil {
		return
	}

	// Get branch info before removal.
	details, _ := worktree.ListDetailed(repoDir)
	branchByPath := make(map[string]string)
	for _, d := range details {
		branchByPath[filepath.ToSlash(d.Path)] = d.Branch
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		// Only clean up if all locks are stale (no active session).
		if !tasklock.AllStale(repoDir) {
			continue
		}

		wtPath := filepath.Join(workDir, e.Name())
		normalizedPath := filepath.ToSlash(wtPath)

		if err := worktree.Remove(repoDir, wtPath); err != nil {
			continue
		}

		// Delete associated branch.
		if branch, ok := branchByPath[normalizedPath]; ok {
			shortBranch := strings.TrimPrefix(branch, "refs/heads/")
			worktree.DeleteBranch(repoDir, shortBranch)
		}
	}

	// Prune and clean locks.
	worktree.Prune(repoDir)
	tasklock.CleanAll(repoDir)
}

// findTaskByID returns the task with the given ID, or nil if not found or already complete.
func findTaskByID(tasks []parser.Task, id string) *parser.Task {
	for i := range tasks {
		if tasks[i].ID == id && !tasks[i].IsComplete() {
			return &tasks[i]
		}
	}
	return nil
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
