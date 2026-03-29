package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/term"
	"github.com/leberkas-org/maggus/internal/capabilities"
	"github.com/leberkas-org/maggus/internal/discord"
	"github.com/leberkas-org/maggus/internal/globalconfig"
	"github.com/leberkas-org/maggus/internal/resolver"
	"github.com/spf13/cobra"
)

// Version is set at build time via -ldflags.
// For dev builds, use: go build -ldflags "-X github.com/leberkas-org/maggus/cmd.BuildTime=$(date +%H%M%S)"
var Version = "dev"

// BuildTime is set at build time via -ldflags for dev build counters.
var BuildTime = ""

func init() {
	if Version == "dev" && BuildTime != "" {
		Version = "dev-" + BuildTime
	}
	rootCmd.Version = Version
}

// caps holds the detected tool capabilities for this run.
var caps capabilities.Capabilities

// daemonCache is a package-level cache for the daemon's PID/running state,
// shared across menu iterations within a single runMenu invocation.
var daemonCache *DaemonStateCache

// sharedPresence holds a Discord Presence instance created by the root menu
// and shared with subcommands (prompt, work). When non-nil, subcommands use
// this instead of creating their own connection.
var sharedPresence *discord.Presence

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

	// Initialise Discord Rich Presence for the menu if enabled.
	// Connect in the background so the TUI renders instantly.
	var presence *discord.Presence
	var presenceReady <-chan struct{}
	if gs, err := globalconfig.LoadSettings(); err == nil && gs.DiscordPresence {
		presence = &discord.Presence{}
		ch := make(chan struct{})
		presenceReady = ch
		menuStart := time.Now()
		go func() {
			_ = presence.Connect()
			close(ch)
			// Send the initial "In Main Menu" update that the main
			// goroutine may have skipped while we were connecting.
			_ = presence.Update(discord.PresenceState{
				FeatureTitle: "In Main Menu",
				StartTime:    menuStart,
			})
		}()
	}
	defer func() {
		if presence != nil {
			<-presenceReady // wait for Connect to finish before closing
			_ = presence.Close()
		}
	}()

	// Initialise the daemon state cache once for the lifetime of runMenu.
	// If it fails (e.g. .maggus/ does not exist yet), daemonCache stays nil
	// and all call sites handle nil gracefully.
	cwd, _ := os.Getwd()
	if cache, err := NewDaemonStateCache(cwd); err == nil {
		daemonCache = cache
		defer func() {
			daemonCache.Stop()
			daemonCache = nil
		}()
	}

	for {
		// Show idle presence while in the main menu.
		if presence != nil {
			select {
			case <-presenceReady:
				_ = presence.Update(discord.PresenceState{
					FeatureTitle: "In Main Menu",
					StartTime:    time.Now(),
				})
			default:
				// Still connecting in background; the goroutine will
				// send the initial update once connected.
			}
		}
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

		// Unsubscribe the model's daemon cache channel before the next iteration
		// creates a new model with a fresh subscription.
		if daemonCache != nil {
			daemonCache.Unsubscribe(final.daemonCacheCh)
		}

		if final.quitting || final.selected == "" {
			return nil
		}

		cmdArgs := append([]string{final.selected}, final.args...)

		// Direct dispatch for TUI commands no longer registered with cobra.
		directDispatch := map[string]func() error{
			"config": runConfig,
			"prompt": runPrompt,
			"repos":  runRepos,
			"status": runStatus,
		}
		if fn, ok := directDispatch[final.selected]; ok {
			if presence != nil {
				select {
				case <-presenceReady:
					sharedPresence = presence
				default:
				}
			}
			_ = fn()
			sharedPresence = nil
			continue
		}

		sub, remaining, err := rootCmd.Find(cmdArgs)
		if err != nil {
			return err
		}
		// Reset work command flags so previous invocations don't leak.
		resetWorkFlags()
		if err := sub.ParseFlags(remaining); err != nil {
			return err
		}
		// Share the menu's Discord presence with the subcommand.
		if presence != nil {
			select {
			case <-presenceReady:
				sharedPresence = presence
			default:
				// Still connecting — subcommand will create its own.
			}
		}

		// Run the command; ignore errors so we return to the menu
		_ = sub.RunE(sub, sub.Flags().Args())

		// Reclaim presence ownership so the next loop iteration resets to "In Main Menu".
		sharedPresence = nil
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

// shouldSkipResolver returns true for subcommands that must operate on the
// literal current directory (e.g. start, stop) rather than the resolved
// repository. This prevents the resolver from silently changing to
// last_opened and making per-directory guards ineffective.
func shouldSkipResolver() bool {
	if len(os.Args) < 2 {
		return false
	}
	switch os.Args[1] {
	case "start", "stop":
		return true
	}
	return false
}

func Execute() {
	// Detect and cache available CLI tools on startup.
	caps = capabilities.Detect()

	// Resolve working directory based on global repository config.
	// Skip resolution for commands that should operate on the literal cwd.
	if !shouldSkipResolver() {
		resolveWorkingDirectory()
	}

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
