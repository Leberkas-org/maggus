package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/term"
	"github.com/leberkas-org/maggus/internal/capabilities"
	"github.com/leberkas-org/maggus/internal/globalconfig"
	"github.com/leberkas-org/maggus/internal/resolver"
	"github.com/spf13/cobra"
)

// Version is set at build time via -ldflags.
var Version = "dev"

// caps holds the detected tool capabilities for this run.
var caps capabilities.Capabilities

var rootCmd = &cobra.Command{
	Use:     "maggus",
	Short:   "Your best and worst co-worker — a junior dev that just works",
	Version: Version,
	Long: `Maggus reads feature files and works through tasks one-by-one
by prompting an AI agent (Claude Code). Provide a feature and let Maggus work.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if err := globalconfig.IncrementMetrics(globalconfig.Metrics{StartupCount: 1}); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to update metrics: %v\n", err)
		}
	},
}

func init() {
	rootCmd.RunE = runMenu
}

func runMenu(cmd *cobra.Command, args []string) error {
	if !term.IsTerminal(os.Stdout.Fd()) {
		return cmd.Help()
	}

	for {
		m := newMenuModel(loadFeatureSummary())
		p := tea.NewProgram(m, tea.WithAltScreen())
		result, err := p.Run()

		// Clean up the file watcher before processing the result.
		if m.watcher != nil {
			m.watcher.Close()
			close(m.watcherCh)
		}

		if err != nil {
			return err
		}

		final := result.(menuModel)
		if final.quitting || final.selected == "" {
			return nil
		}

		cmdArgs := append([]string{final.selected}, final.args...)
		sub, remaining, err := rootCmd.Find(cmdArgs)
		if err != nil {
			return err
		}
		if err := sub.ParseFlags(remaining); err != nil {
			return err
		}
		// Run the command; ignore errors so we return to the menu
		_ = sub.RunE(sub, sub.Flags().Args())
	}
}

// resolveWorkingDirectory runs the startup directory resolution logic.
// It determines which repository to work in based on global config,
// current directory, and user input.
var resolveWorkingDirectory = func() {
	cwd, err := os.Getwd()
	if err != nil {
		return
	}

	deps := resolver.DefaultDeps()
	// Only prompt when running in an interactive terminal.
	if term.IsTerminal(os.Stdin.Fd()) {
		deps.Prompt = promptYesNo
	}

	result, err := resolver.Resolve(cwd, deps)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: directory resolution failed: %v\n", err)
		return
	}

	if result.Changed {
		fmt.Fprintf(os.Stderr, "Switched to repository: %s\n", result.Dir)
	}
}

// promptYesNo asks a yes/no question on stdin and returns true for yes.
func promptYesNo(question string) bool {
	fmt.Fprintf(os.Stderr, "%s [y/N] ", question)
	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(strings.ToLower(answer))
	return answer == "y" || answer == "yes"
}

func Execute() {
	// Detect and cache available CLI tools on startup.
	caps = capabilities.Detect()

	// Resolve working directory based on global repository config.
	resolveWorkingDirectory()

	// Register skill commands only when claude is available.
	if caps.HasClaude {
		rootCmd.AddCommand(planCmd)
		rootCmd.AddCommand(visionCmd)
		rootCmd.AddCommand(architectureCmd)
		rootCmd.AddCommand(promptCmd)
	}

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
