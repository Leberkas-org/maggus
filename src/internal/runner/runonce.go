package runner

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// RunOnce invokes `claude -p <prompt> --output-format text` and returns the text output.
// This is a simpler alternative to RunClaude for one-shot invocations that don't need
// streaming or TUI support.
func RunOnce(ctx context.Context, prompt string, model string) (string, error) {
	path, err := exec.LookPath("claude")
	if err != nil {
		return "", fmt.Errorf("claude not found on PATH: %w\nMake sure Claude Code CLI is installed and available", err)
	}

	args := []string{
		"-p", prompt,
		"--output-format", "text",
		"--verbose",
		"--dangerously-skip-permissions",
	}
	if model != "" {
		args = append(args, "--model", model)
	}

	cmd := exec.CommandContext(ctx, path, args...)
	setProcAttr(cmd)

	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.WaitDelay = 5 * time.Second

	out, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			return "", ErrInterrupted
		}
		return "", fmt.Errorf("claude exited with error: %w", err)
	}

	return strings.TrimSpace(string(out)), nil
}
