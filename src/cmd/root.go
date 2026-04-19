package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/term"
	"github.com/leberkas-org/maggus/internal/agent"
	"github.com/leberkas-org/maggus/internal/config"
	"github.com/leberkas-org/maggus/internal/daemon"
	"github.com/leberkas-org/maggus/internal/git"
	"github.com/leberkas-org/maggus/internal/ipc"
	appui "github.com/leberkas-org/maggus/internal/tui"
	"github.com/spf13/cobra"
)

var (
	Version  = "dev"
	BuildTime = ""
	detached  bool
)

func init() {
	if Version == "dev" && BuildTime != "" {
		Version = "dev-" + BuildTime
	}
	rootCmd.Version = Version
	rootCmd.Flags().BoolVarP(&detached, "detach", "d", false, "Start daemon in detached mode (no TUI)")
}

var rootCmd = &cobra.Command{
	Use:   "maggus",
	Short: "Your best and worst co-worker — a junior dev that just works",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		if err := cfg.Validate(); err != nil {
			return err
		}

		globalDir, err := config.GlobalDir()
		if err != nil {
			return err
		}

		if detached {
			return runDaemon(cfg)
		}

		// Start daemon in background, attach TUI
		if !term.IsTerminal(os.Stdout.Fd()) {
			return cmd.Help()
		}

		go func() {
			_ = runDaemon(cfg)
		}()

		reader := ipc.NewFileStateReader(globalDir)
		m := appui.NewApp(reader)
		p := tea.NewProgram(m, tea.WithAltScreen())
		_, err = p.Run()
		return err
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runDaemon(cfg config.Config) error {
	gitCmd := git.NewCommander()
	gitOps := git.New(gitCmd, cfg.Git.ProtectedBranches)
	agents := agent.NewRegistry()

	d := daemon.New(cfg, gitOps, agents)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	fmt.Println("maggus daemon starting...")
	return d.Run(ctx)
}
