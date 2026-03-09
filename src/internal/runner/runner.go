package runner

import (
	"fmt"
	"os"
	"os/exec"
)

// RunClaude invokes `claude -p <prompt>` and streams stdout/stderr to the terminal.
// Returns an error if claude is not found or exits with a non-zero code.
func RunClaude(prompt string) error {
	path, err := exec.LookPath("claude")
	if err != nil {
		return fmt.Errorf("claude not found on PATH: %w\nMake sure Claude Code CLI is installed and available", err)
	}

	cmd := exec.Command(path, "-p", prompt)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("claude exited with error: %w", err)
	}

	return nil
}
