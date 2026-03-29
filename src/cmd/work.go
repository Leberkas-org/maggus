package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/leberkas-org/maggus/internal/claude2x"
	"github.com/leberkas-org/maggus/internal/discord"
	"github.com/leberkas-org/maggus/internal/gitsync"
	"github.com/leberkas-org/maggus/internal/gitutil"
	"github.com/leberkas-org/maggus/internal/globalconfig"
	"github.com/leberkas-org/maggus/internal/parser"
	"github.com/leberkas-org/maggus/internal/runlog"
	"github.com/leberkas-org/maggus/internal/runner"
	"github.com/leberkas-org/maggus/internal/stores"
	"github.com/leberkas-org/maggus/internal/usage"
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
	countFlag int
	modelFlag string
	agentFlag string
	taskFlag  string

	// Daemon-mode flags (hidden; set by 'maggus start', not users directly).
	daemonRunFlag   bool
	daemonRunIDFlag string
)

// resetWorkFlags resets all work command flags to their zero/default values.
// This must be called before ParseFlags in menu-driven and dispatch contexts
// so that flags from a previous invocation do not leak into the next one.
func resetWorkFlags() {
	countFlag = defaultTaskCount
	modelFlag = ""
	agentFlag = ""
	taskFlag = ""
	daemonRunFlag = false
	daemonRunIDFlag = ""
}

var workCmd = &cobra.Command{
	Use:    "work [count]",
	Short:  "Work on the next N approved features from the feature files",
	Hidden: true,
	Long: `Reads feature files and works through all approved features one at a time.
Each feature's tasks are completed before moving to the next. Use --count or a
positional argument to limit the number of features worked. By default, one
feature is worked per run (override with auto_continue: true in config).

Examples:
  maggus work        # work on the next approved feature (or all if auto_continue: true)
  maggus work 3      # work on the next 3 approved features
  maggus work -c 3   # work on the next 3 approved features
  maggus work --model opus   # override model for this run`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		_ = globalconfig.IncrementMetrics(globalconfig.Metrics{WorkRuns: 1})

		wc, err := workSetup(cmd, args)
		if err != nil {
			return err
		}

		dir := wc.dir

		// Daemon mode: delegate to the keep-alive loop which handles
		// watching for file changes and retrying when no work is found.
		if daemonRunFlag {
			return runDaemonLoop(cmd, wc)
		}

		repoDir := dir
		workDir := dir

		// Migrate any legacy per-project usage data to the global store (once, before the loop).
		_ = usage.MigrateProject(dir)

		// Mutual exclusion: prevent work from running while a daemon is active.
		daemonPID, pidErr := readDaemonPID(dir)
		if pidErr != nil {
			return fmt.Errorf("check daemon status: %w", pidErr)
		}
		if daemonPID != 0 {
			if isProcessRunning(daemonPID) {
				return fmt.Errorf("daemon is running (PID %d) — stop it first with 'maggus stop'", daemonPID)
			}
			// Stale PID file — clean it up silently.
			removeDaemonPID(dir)
		}

		// Write our PID so the daemon can detect us.
		if pidErr := writeWorkPID(dir, os.Getpid()); pidErr != nil {
			return fmt.Errorf("write work PID: %w", pidErr)
		}
		defer removeWorkPID(dir)

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

		featureStore := stores.NewFileFeatureStore(dir)
		bugStore := stores.NewFileBugStore(dir)

		setup, err := initIteration(cmd, dir, wc.modelDisplay, count, featureStore, bugStore)
		if err != nil {
			return err
		}
		if setup == nil {
			return nil
		}
		runID := setup.runID

		// Build approved plans with approval filtering (bugs first, then features).
		featureGroups, fgErr := buildApprovedPlans(dir, wc.cfg, featureStore, bugStore)
		if fgErr != nil {
			return fmt.Errorf("build approved plans: %w", fgErr)
		}

		// When --task is set, restrict to the single plan containing that task.
		if taskFlag != "" {
			targetGroup := findGroupForTask(featureGroups, taskFlag)
			if targetGroup == nil {
				cmd.Println(fmt.Sprintf("Task %s not found in any approved feature or bug.", taskFlag))
				return nil
			}
			featureGroups = []parser.Plan{*targetGroup}
		}

		// Remove plans with no workable tasks.
		var workableGroups []parser.Plan
		for _, g := range featureGroups {
			if countWorkable(g.Tasks) > 0 {
				workableGroups = append(workableGroups, g)
			}
		}
		featureGroups = workableGroups

		if len(featureGroups) == 0 {
			if wc.cfg.IsApprovalRequired() {
				cmd.Println("No approved features available. Use 'maggus approve' to approve features for execution.")
			} else {
				cmd.Println("No workable features or bugs found.")
			}
			return nil
		}

		// Determine feature count for banner and loop cap.
		featureCount := len(featureGroups)
		if count > 0 && count < featureCount {
			featureCount = count
		}
		if !wc.cfg.IsAutoContinueEnabled() && count == 0 {
			featureCount = 1 // default: 1 feature unless count overrides
		}

		// Find the first workable task for branch naming.
		branchTask := firstWorkableTask(featureGroups)
		if branchTask == nil {
			branchTask = setup.next // fallback
		}

		branchMsg, err := setupBranch(repoDir, branchTask, wc.cfg.Git)
		if err != nil {
			return err
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

		// Initialise sync functions used by the between-task sync check.
		runner.InitSyncFuncs(gitsync.Pull, gitsync.PullRebase, gitsync.ForcePull)

		// Open structured run log; failures are non-fatal.
		runLogger, logErr := runlog.Open(featureGroups[0].MaggusID, dir, wc.cfg.LogMaxFiles())
		if logErr != nil {
			cmd.Printf("Warning: could not open run log: %v\n", logErr)
		}
		defer func() { _ = runLogger.Close() }()

		// Use shared presence from root menu if available; otherwise create our own.
		presence := sharedPresence
		ownPresence := false
		if presence == nil && wc.globalSettings.DiscordPresence {
			presence = &discord.Presence{}
			_ = presence.Connect()
			ownPresence = true
		}
		defer func() {
			if ownPresence && presence != nil {
				_ = presence.Close()
			}
		}()

		// Build the bubbletea program (interactive TUI).
		twoXStatus := claude2x.FetchStatus()
		cwd, _ := os.Getwd()

		branchCmd := gitutil.Command("rev-parse", "--abbrev-ref", "HEAD")
		branchCmd.Dir = dir
		branchOut, _ := branchCmd.Output()
		currentBranch := strings.TrimSpace(string(branchOut))

		banner := runner.BannerInfo{
			Iterations: featureCount,
			Branch:     currentBranch,
			RunID:      runID,
			Agent:      wc.activeAgent.Name(),
			CWD:        cwd,
		}
		if twoXStatus.Is2x {
			banner.TwoXExpiresIn = twoXStatus.TwoXWindowExpiresIn
		}

		m := runner.NewTUIModel(wc.resolvedModel, Version, wc.hostFingerprint, tuiCancel, banner)
		m.SetSyncDir(workDir)
		m.SetWatcher(workDir)
		m.SetStores(featureStore, bugStore)
		setupUsageCallback(&m, runID, workDir, wc.modelDisplay, wc.activeAgent.Name(), runLogger)
		m.SetOnToolUse(func(taskID, toolType string, params map[string]string) {
			runLogger.ToolUse(taskID, toolType, params)
		})
		m.SetOnOutput(func(taskID, text string) {
			runLogger.Output(taskID, text)
		})

		p := tea.NewProgram(m, tea.WithAltScreen())
		stopFlagAtomic := m.StopFlag()
		stopAtTaskIDAtomic := m.StopAtTaskIDFlag()

		tc := taskContext{
			workCtx:       workCtx,
			p:             p,
			activeAgent:   wc.activeAgent,
			resolvedModel: wc.resolvedModel,
			notifier:      wc.notifier,
			validIncludes: wc.validIncludes,
			repoDir:       repoDir,
			workDir:       workDir,
			runID:         runID,
			onComplete:    wc.cfg.OnComplete,
			hooks:         wc.cfg.Hooks,
			logger:        runLogger,
			presence:      presence,
			featureStore:  featureStore,
			bugStore:      bugStore,
		}

		runWorkGoroutine(workLoopParams{
			tc:            tc,
			tasks:         setup.tasks,
			featureGroups: featureGroups,
			count:         wc.count, // 0 = unlimited/auto via autoContinue; N = explicit feature limit
			autoContinue:  wc.cfg.IsAutoContinueEnabled(),
			runID:         runID,
			startTime:     setup.startTime,
			p:             p,
			stopFlag:      stopFlagAtomic,
			stopAtTaskID:  stopAtTaskIDAtomic,
			includeWarns:  wc.includeWarnings,
			branchMsg:     branchMsg,
			syncInfoMsg:   syncInfoMsg,
			activeAgentNm: wc.activeAgent.Name(),
			startHash:     captureStartHash(workDir),
			modelDisplay:  wc.modelDisplay,
			dir:           dir,
			featureStore:  featureStore,
			bugStore:      bugStore,
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
	workCmd.Flags().IntVarP(&countFlag, "count", "c", defaultTaskCount, "number of features to work on (0 = all or 1 if auto_continue is false)")
	workCmd.Flags().StringVar(&modelFlag, "model", "", "model to use (e.g. opus, sonnet, haiku, or a full model ID)")
	workCmd.Flags().StringVar(&agentFlag, "agent", "", "agent to use (e.g. claude, opencode)")
	workCmd.Flags().StringVar(&taskFlag, "task", "", "run a specific task by ID (e.g. TASK-001)")

	// Hidden flags used internally by 'maggus start' to launch the daemon work loop.
	workCmd.Flags().BoolVar(&daemonRunFlag, "daemon-run", false, "run the work loop as a daemon (no TUI)")
	workCmd.Flags().StringVar(&daemonRunIDFlag, "daemon-run-id", "", "run ID to use in daemon mode")
	_ = workCmd.Flags().MarkHidden("daemon-run")
	_ = workCmd.Flags().MarkHidden("daemon-run-id")

	rootCmd.AddCommand(workCmd)
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

// setupUsageCallback configures the TUI model to record per-task usage.
func setupUsageCallback(m *runner.TUIModel, runID string, dir, modelDisplay, agentName string, runLogger *runlog.Logger) {
	repoURL := gitutil.RepoURL(dir)
	m.SetOnTaskUsage(func(tu runner.TaskUsage) {
		_ = usage.Append([]usage.Record{{
			RunID:                    runID,
			Repository:               repoURL,
			Kind:                     tu.Kind,
			ItemID:                   tu.ItemID,
			ItemShort:                tu.ItemShort,
			ItemTitle:                tu.ItemTitle,
			TaskShort:                tu.TaskShort,
			Model:                    modelDisplay,
			Agent:                    agentName,
			InputTokens:              tu.InputTokens,
			OutputTokens:             tu.OutputTokens,
			CacheCreationInputTokens: tu.CacheCreationInputTokens,
			CacheReadInputTokens:     tu.CacheReadInputTokens,
			CostUSD:                  tu.CostUSD,
			ModelUsage:               tu.ModelUsage,
			StartTime:                tu.StartTime,
			EndTime:                  tu.EndTime,
		}})
		totalTokens := int64(tu.InputTokens + tu.OutputTokens + tu.CacheCreationInputTokens + tu.CacheReadInputTokens)
		if totalTokens > 0 {
			_ = globalconfig.IncrementMetrics(globalconfig.Metrics{TokensUsed: totalTokens})
		}
		modelUsage := make(map[string]runlog.ModelTokensEntry, len(tu.ModelUsage))
		for name, mt := range tu.ModelUsage {
			modelUsage[name] = runlog.ModelTokensEntry{
				InputTokens:              mt.InputTokens,
				OutputTokens:             mt.OutputTokens,
				CacheCreationInputTokens: mt.CacheCreationInputTokens,
				CacheReadInputTokens:     mt.CacheReadInputTokens,
				CostUSD:                  mt.CostUSD,
			}
		}
		runLogger.TaskUsage(runlog.TaskUsageData{
			InputTokens:              tu.InputTokens,
			OutputTokens:             tu.OutputTokens,
			CacheCreationInputTokens: tu.CacheCreationInputTokens,
			CacheReadInputTokens:     tu.CacheReadInputTokens,
			CostUSD:                  tu.CostUSD,
			ModelUsage:               modelUsage,
		})
	})
}
