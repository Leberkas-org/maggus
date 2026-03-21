package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"

	"github.com/leberkas-org/maggus/internal/config"
	"github.com/spf13/cobra"
)

var promptModelFlag string

var promptCmd = &cobra.Command{
	Use:   "prompt",
	Short: "Launch an interactive Claude Code session with usage tracking",
	Long: `Launches Claude Code interactively with full terminal passthrough.
stdin, stdout, and stderr are connected directly so you get the normal
Claude Code experience. Usage data is extracted after the session ends.`,
	RunE: runPrompt,
}

func init() {
	promptCmd.Flags().StringVar(&promptModelFlag, "model", "", "model to use (e.g. opus, sonnet, haiku, or a full model ID)")
}

func runPrompt(cmd *cobra.Command, args []string) error {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	// Load config for default model.
	cfg, err := config.Load(dir)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Resolve model: CLI flag overrides config file.
	modelInput := cfg.Model
	if promptModelFlag != "" {
		modelInput = promptModelFlag
	}
	resolvedModel := config.ResolveModel(modelInput)

	// Build claude command args for interactive mode (no -p, no --output-format).
	claudeArgs := []string{"--dangerously-skip-permissions"}
	if resolvedModel != "" {
		claudeArgs = append(claudeArgs, "--model", resolvedModel)
	}

	claudePath, err := exec.LookPath("claude")
	if err != nil {
		return fmt.Errorf("claude not found on PATH: %w\nMake sure Claude Code CLI is installed and available", err)
	}

	proc := exec.Command(claudePath, claudeArgs...)

	// Full terminal passthrough: connect stdin, stdout, stderr directly.
	proc.Stdin = os.Stdin
	proc.Stdout = os.Stdout
	proc.Stderr = os.Stderr

	// Forward interrupt signals to the child process by ignoring them in
	// the parent — the terminal delivers SIGINT to the entire process group,
	// so Claude receives it directly.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, shutdownSignals...)
	defer signal.Stop(sigCh)

	if err := proc.Start(); err != nil {
		return fmt.Errorf("start claude: %w", err)
	}

	// Wait for Claude to exit.
	waitErr := proc.Wait()

	// Stop capturing signals.
	signal.Stop(sigCh)

	// TODO(TASK-004): Capture Claude session ID from session files.
	// TODO(TASK-005): Extract usage data and write to usage_prompt.jsonl.

	if waitErr != nil {
		// If Claude exited due to user Ctrl+C, that's not an error.
		if proc.ProcessState != nil && proc.ProcessState.ExitCode() == 130 {
			return nil
		}
		// Exit code 2 is also common for user-initiated exits in Claude.
		if proc.ProcessState != nil && proc.ProcessState.ExitCode() == 2 {
			return nil
		}
		return fmt.Errorf("claude exited with error: %w", waitErr)
	}

	return nil
}
