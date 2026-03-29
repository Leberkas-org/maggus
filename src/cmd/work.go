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

		if daemonRunFlag {
			return runDaemonLoop(cmd, wc)
		}

		cmd.Println("Use 'maggus start' to start the daemon.")
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
