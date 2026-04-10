// Package agent defines the Agent interface for invoking AI backends.
// Different backends (Claude Code, OpenCode, etc.) implement this interface
// to provide a unified contract for the work loop.
package agent

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
)

// MessageSender allows sending tea messages to a handler. Both *tea.Program
// and custom per-worker senders implement this interface. This abstraction
// lets parallel workers route agent messages to per-worker snapshot writers
// without changing the agent implementations.
type MessageSender interface {
	Send(msg tea.Msg)
}

// Agent is the interface that AI backend adapters must implement.
// The work loop uses this to invoke the configured agent without
// knowing the specifics of its CLI flags or streaming format.
type Agent interface {
	// Run invokes the agent in streaming mode and sends progress events
	// to the provided message sender. It blocks until the agent process
	// completes or the context is cancelled.
	// When sessionPersistence is false, --no-session-persistence is passed to the subprocess.
	Run(ctx context.Context, prompt string, model string, sessionPersistence bool, p MessageSender) error

	// RunOnce invokes the agent in text mode and returns the full response.
	// This is used for one-shot invocations that don't need streaming or TUI.
	// When sessionPersistence is false, --no-session-persistence is passed to the subprocess.
	RunOnce(ctx context.Context, prompt string, model string, sessionPersistence bool) (string, error)

	// Name returns the agent identifier (e.g. "claude", "opencode").
	Name() string

	// Validate checks whether the agent's CLI tool is available on PATH
	// and returns an error with installation instructions if not found.
	Validate() error
}
