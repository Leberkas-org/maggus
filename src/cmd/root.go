package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/term"
	"github.com/leberkas-org/maggus/internal/agent"
	"github.com/leberkas-org/maggus/internal/config"
	"github.com/leberkas-org/maggus/internal/daemon"
	"github.com/leberkas-org/maggus/internal/git"
	"github.com/leberkas-org/maggus/internal/ipc"
	"github.com/leberkas-org/maggus/internal/logging"
	appui "github.com/leberkas-org/maggus/internal/tui"
	"github.com/spf13/cobra"
)

var (
	Version   = "dev"
	BuildTime = ""
	detached  bool
	daemonRun bool // internal flag — used by the spawned child process
)

func init() {
	if Version == "dev" && BuildTime != "" {
		Version = "dev-" + BuildTime
	}
	rootCmd.Version = Version
	rootCmd.Flags().BoolVarP(&detached, "detach", "d", false, "Start daemon in detached mode (no TUI)")
	rootCmd.Flags().BoolVar(&daemonRun, "daemon-run", false, "Internal: run daemon in-process")
	rootCmd.Flags().MarkHidden("daemon-run")
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

		if daemonRun {
			// Internal: actually run the daemon in this process
			return runDaemon(cfg)
		}

		if detached {
			// User-facing: spawn daemon as background process and exit
			if err := ensureDaemon(globalDir); err != nil {
				return fmt.Errorf("start daemon: %w", err)
			}
			fmt.Println("Daemon started.")
			return nil
		}

		if !term.IsTerminal(os.Stdout.Fd()) {
			return cmd.Help()
		}

		// Ensure daemon is running as a separate process
		if err := ensureDaemon(globalDir); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not start daemon: %v\n", err)
		}

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
	logger, cleanup, err := logging.SetupDaemon()
	if err != nil {
		return fmt.Errorf("setup logging: %w", err)
	}
	defer cleanup()
	logger = logger.With("component", "daemon", "pid", os.Getpid())
	slog.SetDefault(logger)

	gitCmd := git.NewCommander()
	gitOps := git.New(gitCmd, cfg.Git.ProtectedBranches, logger)
	agents := agent.NewRegistry()

	d := daemon.New(cfg, gitOps, agents, logger)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	logger.Info("maggus daemon starting")
	if err := d.Run(ctx); err != nil {
		logger.Error("daemon exited", "error", err)
		return err
	}
	return nil
}

func ensureDaemon(globalDir string) error {
	if daemon.IsLocked() {
		return nil // daemon already running
	}

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("find executable: %w", err)
	}

	child := exec.Command(exe, "--daemon-run")
	child.Stdin = nil
	child.Stdout = nil
	child.Stderr = nil
	child.SysProcAttr = daemonSysProcAttr()

	if err := child.Start(); err != nil {
		return fmt.Errorf("start daemon: %w", err)
	}

	_ = child.Process.Release()
	return nil
}
