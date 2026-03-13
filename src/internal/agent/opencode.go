package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// OpenCodeAgent implements the Agent interface for OpenCode.
// OpenCode (github.com/opencode-ai/opencode) does not stream JSON events —
// it outputs a single JSON blob on completion. The Run method simulates
// progress by sending StatusMsg updates at process start and finish.
// Model selection is handled via OpenCode's config file, not CLI flags.
type OpenCodeAgent struct{}

// NewOpenCode creates a new OpenCodeAgent.
func NewOpenCode() *OpenCodeAgent {
	return &OpenCodeAgent{}
}

func (a *OpenCodeAgent) Name() string { return "opencode" }

func (a *OpenCodeAgent) Validate() error {
	_, err := exec.LookPath("opencode")
	if err != nil {
		return fmt.Errorf("opencode not found on PATH: %w\nInstall OpenCode from https://github.com/opencode-ai/opencode", err)
	}
	return nil
}

// Run invokes `opencode -p <prompt> -f json -q` and sends progress events
// to the provided bubbletea program. OpenCode does not stream events, so
// StatusMsg updates are sent at process start and on completion.
// The model parameter is ignored — OpenCode uses its config file for model selection.
func (a *OpenCodeAgent) Run(ctx context.Context, prompt string, model string, p *tea.Program) error {
	path, err := exec.LookPath("opencode")
	if err != nil {
		return fmt.Errorf("opencode not found on PATH: %w\nInstall OpenCode from https://github.com/opencode-ai/opencode", err)
	}

	args := []string{
		"-p", prompt,
		"-f", "json",
		"-q",
	}

	cmd := exec.CommandContext(ctx, path, args...)
	setProcAttr(cmd)

	var stderrBuf strings.Builder
	cmd.Stderr = &stderrWriter{tee: os.Stderr, buf: &stderrBuf}
	cmd.Stdin = os.Stdin
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		if runtime.GOOS == "windows" {
			kill := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid))
			kill.Stderr = os.Stderr
			return kill.Run()
		}
		return cmd.Process.Kill()
	}
	cmd.WaitDelay = 5 * time.Second

	p.Send(StatusMsg{Status: "Thinking..."})

	out, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			return ErrInterrupted
		}
		stderr := strings.TrimSpace(stderrBuf.String())
		if stderr != "" {
			return fmt.Errorf("opencode exited with error: %w\nstderr: %s", err, stderr)
		}
		return fmt.Errorf("opencode exited with error: %w", err)
	}

	output := strings.TrimSpace(string(out))
	if output == "" {
		stderr := strings.TrimSpace(stderrBuf.String())
		msg := "opencode produced no output. Possible causes:\n" +
			"  - OpenCode not configured (check .opencode/config.json)\n" +
			"  - API key or network issue"
		if stderr != "" {
			msg += fmt.Sprintf("\n  stderr: %s", stderr)
		}
		return fmt.Errorf("%s", msg)
	}

	// Parse the JSON response envelope.
	var resp openCodeResponse
	if err := json.Unmarshal([]byte(output), &resp); err != nil {
		// If JSON parsing fails, treat the raw output as the response text.
		p.Send(OutputMsg{Text: output})
		p.Send(StatusMsg{Status: "Done"})
		return nil
	}

	p.Send(OutputMsg{Text: resp.Response})
	p.Send(StatusMsg{Status: "Done"})
	return nil
}

// RunOnce invokes `opencode -p <prompt> -f text -q` and returns the full response.
// The model parameter is ignored — OpenCode uses its config file for model selection.
func (a *OpenCodeAgent) RunOnce(ctx context.Context, prompt string, model string) (string, error) {
	path, err := exec.LookPath("opencode")
	if err != nil {
		return "", fmt.Errorf("opencode not found on PATH: %w\nInstall OpenCode from https://github.com/opencode-ai/opencode", err)
	}

	args := []string{
		"-p", prompt,
		"-f", "text",
		"-q",
	}

	cmd := exec.CommandContext(ctx, path, args...)
	setProcAttr(cmd)

	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		if runtime.GOOS == "windows" {
			kill := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid))
			kill.Stderr = os.Stderr
			return kill.Run()
		}
		return cmd.Process.Kill()
	}
	cmd.WaitDelay = 5 * time.Second

	out, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			return "", ErrInterrupted
		}
		return "", fmt.Errorf("opencode exited with error: %w", err)
	}

	return strings.TrimSpace(string(out)), nil
}

// openCodeResponse is the JSON envelope returned by `opencode -f json`.
type openCodeResponse struct {
	Response string `json:"response"`
}
