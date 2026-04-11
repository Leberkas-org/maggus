package cmd

import (
	"github.com/leberkas-org/maggus/internal/globalconfig"
	"github.com/leberkas-org/maggus/internal/parser"
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
	countFlag    int
	modelFlag    string
	agentFlag    string
	taskFlag     string
	parallelFlag bool

	// Daemon-mode flags (hidden; set by 'maggus start', not users directly).
	daemonRunFlag   bool
	daemonRunIDFlag string

	// Dispatch-mode flag (hidden; set by dispatchTask, not users directly).
	// When non-empty, this process is a dispatched worker and should write
	// per-worker state files to this directory (the main repo root).
	dispatchRepoFlag string
)

// resetRunFlags resets all run command flags to their zero/default values.
// This must be called before ParseFlags in menu-driven and dispatch contexts
// so that flags from a previous invocation do not leak into the next one.
func resetRunFlags() {
	countFlag = defaultTaskCount
	modelFlag = ""
	agentFlag = ""
	taskFlag = ""
	parallelFlag = false
	daemonRunFlag = false
	daemonRunIDFlag = ""
	dispatchRepoFlag = ""
}

var runCmd = &cobra.Command{
	Use:    "run [count]",
	Short:  "Work on the next N approved features from the feature files",
	Hidden: true,
	Long: `Reads feature files and works through all approved features one at a time.
Each feature's tasks are completed before moving to the next. Use --count or a
positional argument to limit the number of features worked. By default, one
feature is worked per run (override with auto_continue: true in config).

Examples:
  maggus run        # work on the next approved feature (or all if auto_continue: true)
  maggus run 3      # work on the next 3 approved features
  maggus run -c 3   # work on the next 3 approved features
  maggus run --model opus   # override model for this run`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		_ = globalconfig.IncrementMetrics(globalconfig.Metrics{WorkRuns: 1})

		wc, err := runSetup(cmd, args)
		if err != nil {
			return err
		}

		if daemonRunFlag {
			return runDaemonLoop(cmd, wc)
		}

		cmd.Println("Use 'maggus start' to start the daemon.")
		return nil
	},
}

func init() {
	runCmd.Flags().IntVarP(&countFlag, "count", "c", defaultTaskCount, "number of features to work on (0 = all or 1 if auto_continue is false)")
	runCmd.Flags().StringVar(&modelFlag, "model", "", "model to use (e.g. opus, sonnet, haiku, or a full model ID)")
	runCmd.Flags().StringVar(&agentFlag, "agent", "", "agent to use (e.g. claude, opencode)")
	runCmd.Flags().StringVar(&taskFlag, "task", "", "run a specific task by ID (e.g. TASK-001)")
	runCmd.Flags().BoolVar(&parallelFlag, "parallel", false, "enable parallel task execution (overrides config)")

	// Hidden flags used internally by 'maggus start' to launch the daemon work loop.
	runCmd.Flags().BoolVar(&daemonRunFlag, "daemon-run", false, "run the work loop as a daemon (no TUI)")
	runCmd.Flags().StringVar(&daemonRunIDFlag, "daemon-run-id", "", "run ID to use in daemon mode")
	runCmd.Flags().StringVar(&dispatchRepoFlag, "dispatch-repo", "", "main repo dir for dispatched worker state files")
	_ = runCmd.Flags().MarkHidden("daemon-run")
	_ = runCmd.Flags().MarkHidden("daemon-run-id")
	_ = runCmd.Flags().MarkHidden("dispatch-repo")

	rootCmd.AddCommand(runCmd)
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
