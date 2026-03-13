package runner

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/leberkas-org/maggus/internal/agent"
)

// ErrInterrupted is returned when the user presses Ctrl+C.
// Deprecated: Use agent.ErrInterrupted instead.
var ErrInterrupted = agent.ErrInterrupted

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

const maxToolHistory = 10
const maxCommitHistory = 5

// RunClaude invokes Claude Code in streaming mode. This is a thin wrapper
// around agent.ClaudeAgent.Run for backwards compatibility.
func RunClaude(ctx context.Context, prompt string, model string, p *tea.Program) error {
	a := agent.NewClaude()
	return a.Run(ctx, prompt, model, p)
}
