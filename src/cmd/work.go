package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/leberkas-org/maggus/internal/claude2x"
	"github.com/leberkas-org/maggus/internal/gitsync"
	"github.com/leberkas-org/maggus/internal/parser"
	"github.com/leberkas-org/maggus/internal/runner"
	"github.com/leberkas-org/maggus/internal/tasklock"
	"github.com/leberkas-org/maggus/internal/worktree"
	"github.com/spf13/cobra"
)

const defaultTaskCount = 0 // 0 means "all workable tasks"

// failedTask records a task that the agent failed to complete.
type failedTask struct {
	ID     string
	Title  string
	Reason string
}

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
	Short: "Work on the next N tasks from the feature files",
	Long: `Reads feature files and works through all incomplete tasks
by prompting Claude Code. Use --count or a positional argument to limit the number.

Examples:
  maggus work        # work on all workable tasks
  maggus work 3      # work on the next 3 tasks
  maggus work -c 3   # work on the next 3 tasks
  maggus work --model opus   # override model for this run`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		wc, err := workSetup(cmd, args)
		if err != nil {
			return err
		}

		dir := wc.dir
		repoDir := dir
		workDir := dir

		// Git sync check: detect remote changes and uncommitted work before starting.
		var syncInfoMsg string
		if wc.cfg.Git.IsCheckSyncEnabled() {
			var shouldAbort bool
			var syncErr error
			syncInfoMsg, shouldAbort, syncErr = checkSync(dir)
			if syncErr != nil {
				return syncErr
			}
			if shouldAbort {
				return nil
			}
		}

		count := wc.count

		setup, err := initIteration(cmd, dir, wc.modelDisplay, count)
		if err != nil {
			return err
		}
		if setup == nil {
			return nil
		}
		count = setup.count
		run := setup.run

		branchMsg, err := setupBranch(wc.useWorktree, repoDir, setup.next, run, wc.cfg.Git)
		if err != nil {
			return err
		}
		if wc.useWorktree {
			workDir = filepath.Join(repoDir, ".maggus-work", run.ID)
			defer func() {
				_ = worktree.Remove(repoDir, workDir)
			}()
		}

		// Set up signal handling and cancellation.
		sigCtx, sigStop := signal.NotifyContext(context.Background(), shutdownSignals...)
		defer sigStop()

		workCtx, workCancel := context.WithCancel(context.Background())
		defer workCancel()

		go func() {
			<-sigCtx.Done()
			sigStop()
			workCancel()
		}()

		tuiCancel := func() {
			sigStop()
			workCancel()
		}

		// Build and start the TUI.
		twoXStatus := claude2x.FetchStatus()
		cwd, _ := os.Getwd()
		banner := runner.BannerInfo{
			Iterations: count,
			Branch:     run.Branch,
			RunID:      run.ID,
			RunDir:     run.RelativeDir(dir),
			Agent:      wc.activeAgent.Name(),
			CWD:        cwd,
		}
		if wc.useWorktree {
			banner.Worktree = workDir
		}
		if twoXStatus.Is2x {
			banner.TwoXExpiresIn = twoXStatus.TwoXWindowExpiresIn
		}
		runner.InitSyncFuncs(gitsync.Pull, gitsync.PullRebase, gitsync.ForcePull)

		m := runner.NewTUIModel(wc.resolvedModel, Version, wc.hostFingerprint, tuiCancel, banner)
		m.SetSyncDir(workDir)
		m.SetWatcher(workDir)
		setupUsageCallback(&m, dir, run, wc.modelDisplay, wc.activeAgent.Name())
		p := tea.NewProgram(m, tea.WithAltScreen())

		tc := taskContext{
			workCtx:       workCtx,
			p:             p,
			run:           run,
			activeAgent:   wc.activeAgent,
			resolvedModel: wc.resolvedModel,
			notifier:      wc.notifier,
			validIncludes: wc.validIncludes,
			useWorktree:   wc.useWorktree,
			repoDir:       repoDir,
			workDir:       workDir,
			runID:         run.ID,
		}

		runWorkGoroutine(workLoopParams{
			tc:            tc,
			tasks:         setup.tasks,
			count:         count,
			run:           run,
			p:             p,
			stopFlag:      m.StopFlag(),
			stopAtTaskID:  m.StopAtTaskIDFlag(),
			includeWarns:  wc.includeWarnings,
			branchMsg:     branchMsg,
			syncInfoMsg:   syncInfoMsg,
			activeAgentNm: wc.activeAgent.Name(),
			startHash:     captureStartHash(workDir),
			modelDisplay:  wc.modelDisplay,
			dir:           dir,
		})

		_, tuiErr := p.Run()
		m.CloseWatcher()
		if tuiErr != nil {
			return fmt.Errorf("TUI error: %w", tuiErr)
		}

		return nil
	},
}

func init() {
	workCmd.Flags().IntVarP(&countFlag, "count", "c", defaultTaskCount, "number of tasks to work on (0 = all)")
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
