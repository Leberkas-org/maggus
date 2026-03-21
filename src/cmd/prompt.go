package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/leberkas-org/maggus/internal/config"
	"github.com/leberkas-org/maggus/internal/session"
	"github.com/leberkas-org/maggus/internal/usage"
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

	// Snapshot session directory before launching Claude (TASK-004).
	sessionDir, _ := session.SessionDir(dir)
	beforeSnapshot, _ := session.SnapshotDir(sessionDir)

	startTime := time.Now()

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
	endTime := time.Now()

	// Stop capturing signals.
	signal.Stop(sigCh)

	// Extract usage data from session files (TASK-004 + TASK-005).
	extractPromptUsage(dir, resolvedModel, beforeSnapshot, startTime, endTime)

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

// extractPromptUsage detects the session file created during the Claude session,
// extracts token usage, and appends a record to usage_prompt.jsonl.
// Errors are printed as warnings but never cause a non-zero exit.
func extractPromptUsage(dir, model string, beforeSnapshot map[string]bool, startTime, endTime time.Time) {
	sessionFile, err := session.DetectSessionFile(dir, beforeSnapshot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not detect session file: %v\n", err)
		return
	}
	if sessionFile == "" {
		fmt.Fprintln(os.Stderr, "Warning: no new Claude session file found; skipping usage extraction")
		return
	}

	summary, err := session.ExtractUsage(sessionFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not extract usage from session: %v\n", err)
		return
	}

	runID := startTime.Format("20060102-150405")
	usagePath := filepath.Join(dir, ".maggus", "usage_prompt.jsonl")

	rec := usage.Record{
		RunID:                    runID,
		Model:                    model,
		Agent:                    "claude",
		InputTokens:              summary.InputTokens,
		OutputTokens:             summary.OutputTokens,
		CacheCreationInputTokens: summary.CacheCreationInputTokens,
		CacheReadInputTokens:     summary.CacheReadInputTokens,
		CostUSD:                  0,
		ModelUsage:               summary.ModelUsage,
		StartTime:                startTime,
		EndTime:                  endTime,
	}

	if err := usage.AppendTo(usagePath, []usage.Record{rec}); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not write usage record: %v\n", err)
	}
}
