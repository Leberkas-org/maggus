package runner

import (
	"context"

	"github.com/leberkas-org/maggus/internal/agent"
)

// RunOnce invokes Claude Code in text mode. This is a thin wrapper
// around agent.ClaudeAgent.RunOnce for backwards compatibility.
func RunOnce(ctx context.Context, prompt string, model string) (string, error) {
	a := agent.NewClaude()
	return a.RunOnce(ctx, prompt, model)
}
