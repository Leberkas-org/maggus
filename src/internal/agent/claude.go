package agent

import (
	"bufio"
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

// ClaudeAgent implements the Agent interface for Claude Code.
type ClaudeAgent struct{}

// NewClaude creates a new ClaudeAgent.
func NewClaude() *ClaudeAgent {
	return &ClaudeAgent{}
}

func (a *ClaudeAgent) Name() string { return "claude" }

func (a *ClaudeAgent) Validate() error {
	_, err := exec.LookPath("claude")
	if err != nil {
		return fmt.Errorf("claude not found on PATH: %w\nMake sure Claude Code CLI is installed and available", err)
	}
	return nil
}

// Run invokes `claude -p <prompt>` with stream-json output and sends progress events
// to the provided bubbletea program. If model is non-empty, --model <model> is added.
func (a *ClaudeAgent) Run(ctx context.Context, prompt string, model string, p *tea.Program) error {
	path, err := exec.LookPath("claude")
	if err != nil {
		return fmt.Errorf("claude not found on PATH: %w\nMake sure Claude Code CLI is installed and available", err)
	}

	args := []string{
		"--output-format", "stream-json",
		"--verbose",
		"--dangerously-skip-permissions",
	}

	if model != "" {
		args = append(args, "--model", model)
	}

	args = append(args, "-p", prompt)
	cmd := exec.CommandContext(ctx, path, args...)
	setProcAttr(cmd)

	var stderrBuf strings.Builder
	cmd.Stderr = &stderrWriter{tee: os.Stderr, buf: &stderrBuf}
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

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start claude: %w", err)
	}

	type scanResult struct {
		eventCount int
		scanErr    error
	}
	scanDone := make(chan scanResult, 1)

	go func() {
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

		eventCount := 0
		for scanner.Scan() {
			line := scanner.Bytes()

			var event streamEvent
			if err := json.Unmarshal(line, &event); err != nil {
				continue
			}

			eventCount++

			switch event.Type {
			case "assistant":
				var msg assistantMessage
				if err := json.Unmarshal(event.Message, &msg); err != nil {
					continue
				}
				for _, block := range msg.Content {
					switch block.Type {
					case "text":
						p.Send(StatusMsg{Status: "Thinking..."})
						p.Send(OutputMsg{Text: block.Text})
					case "tool_use":
						var input ToolInput
						json.Unmarshal(block.Input, &input)
						desc := DescribeToolUse(block.Name, input)
						p.Send(StatusMsg{Status: "Running tool"})
						p.Send(ToolMsg{
							Description: desc,
							Type:        block.Name,
							Params:      buildToolParams(block.Name, input),
							Timestamp:   time.Now(),
						})

						if block.Name == "Skill" && input.Skill != "" {
							p.Send(SkillMsg{Name: input.Skill})
						}
						if strings.HasPrefix(block.Name, "mcp__") {
							parts := strings.SplitN(block.Name, "__", 3)
							if len(parts) >= 2 {
								p.Send(MCPMsg{Name: parts[1]})
							}
						}
					}
				}

			case "result":
				if event.Usage != nil {
					p.Send(UsageMsg{
						InputTokens:              event.Usage.InputTokens,
						OutputTokens:             event.Usage.OutputTokens,
						CacheCreationInputTokens: event.Usage.CacheCreationInputTokens,
						CacheReadInputTokens:     event.Usage.CacheReadInputTokens,
						CostUSD:                  event.CostUSD,
					})
				}
				if event.Subtype == "success" {
					p.Send(StatusMsg{Status: "Done"})
				} else {
					p.Send(OutputMsg{Text: event.Result})
					p.Send(StatusMsg{Status: "Failed"})
				}
			}
		}
		scanDone <- scanResult{eventCount: eventCount, scanErr: scanner.Err()}
	}()

	var result scanResult
	select {
	case result = <-scanDone:
	case <-ctx.Done():
		// Context cancelled (e.g. Ctrl+C) — wait briefly for scanner to finish
		// after the process is killed, then proceed to cmd.Wait().
		select {
		case result = <-scanDone:
		case <-time.After(5 * time.Second):
		}
	case <-time.After(10 * time.Minute):
	}

	if err := cmd.Wait(); err != nil {
		if ctx.Err() != nil {
			return ErrInterrupted
		}
		stderr := strings.TrimSpace(stderrBuf.String())
		if stderr != "" {
			return fmt.Errorf("claude exited with error: %w\nstderr: %s", err, stderr)
		}
		return fmt.Errorf("claude exited with error: %w", err)
	}

	if result.scanErr != nil {
		return fmt.Errorf("reading claude output: %w", result.scanErr)
	}

	if result.eventCount == 0 {
		stderr := strings.TrimSpace(stderrBuf.String())
		msg := "claude produced no output (0 events received). Possible causes:\n" +
			"  - Claude CLI not authenticated (run 'claude' interactively to check)\n" +
			"  - Claude CLI version mismatch (run 'claude --version')\n" +
			"  - API key or network issue on this machine"
		if stderr != "" {
			msg += fmt.Sprintf("\n  stderr: %s", stderr)
		}
		return fmt.Errorf("%s", msg)
	}

	return nil
}

// RunOnce invokes `claude -p <prompt> --output-format text` and returns the full response.
func (a *ClaudeAgent) RunOnce(ctx context.Context, prompt string, model string) (string, error) {
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

// Internal types for Claude Code's streaming JSON format.

type streamEvent struct {
	Type    string          `json:"type"`
	Subtype string          `json:"subtype"`
	Message json.RawMessage `json:"message"`
	Result  string          `json:"result"`
	Usage   *streamUsage    `json:"usage,omitempty"`
	CostUSD float64         `json:"total_cost_usd"`
}

type streamUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
}

type assistantMessage struct {
	Content []contentBlock `json:"content"`
}

type contentBlock struct {
	Type  string          `json:"type"`
	Text  string          `json:"text"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

// ToolInput represents the input fields for a tool invocation in Claude Code's streaming format.
type ToolInput struct {
	Command     string `json:"command"`
	Description string `json:"description"`
	Pattern     string `json:"pattern"`
	FilePath    string `json:"file_path"`
	Skill       string `json:"skill"`
}

// stderrWriter tees writes to both the terminal and a buffer for diagnostics.
type stderrWriter struct {
	tee *os.File
	buf *strings.Builder
}

func (w *stderrWriter) Write(p []byte) (n int, err error) {
	w.buf.Write(p)
	return w.tee.Write(p)
}

// buildToolParams extracts key parameters from a tool invocation for display in the detail panel.
func buildToolParams(tool string, input ToolInput) map[string]string {
	params := make(map[string]string)
	switch tool {
	case "Bash":
		if input.Command != "" {
			params["command"] = input.Command
		}
		if input.Description != "" {
			params["description"] = input.Description
		}
	case "Read":
		if input.FilePath != "" {
			params["file"] = input.FilePath
		}
	case "Edit":
		if input.FilePath != "" {
			params["file"] = input.FilePath
		}
	case "Write":
		if input.FilePath != "" {
			params["file"] = input.FilePath
		}
	case "Glob":
		if input.Pattern != "" {
			params["pattern"] = input.Pattern
		}
	case "Grep":
		if input.Pattern != "" {
			params["pattern"] = input.Pattern
		}
	case "Skill":
		if input.Skill != "" {
			params["skill"] = input.Skill
		}
	}
	return params
}

// DescribeToolUse returns a human-readable description for a tool invocation.
func DescribeToolUse(tool string, input ToolInput) string {
	switch tool {
	case "Bash":
		if input.Description != "" {
			return fmt.Sprintf("Bash: %s", input.Description)
		}
		if input.Command != "" {
			return fmt.Sprintf("Bash: %s", input.Command)
		}
	case "Read":
		return fmt.Sprintf("Read: %s", input.FilePath)
	case "Edit":
		return fmt.Sprintf("Edit: %s", input.FilePath)
	case "Write":
		return fmt.Sprintf("Write: %s", input.FilePath)
	case "Glob":
		return fmt.Sprintf("Glob: %s", input.Pattern)
	case "Grep":
		return fmt.Sprintf("Grep: %s", input.Pattern)
	case "Skill":
		if input.Skill != "" {
			return fmt.Sprintf("Skill: %s", input.Skill)
		}
	}
	if strings.HasPrefix(tool, "mcp__") {
		parts := strings.SplitN(tool, "__", 3)
		if len(parts) == 3 {
			return fmt.Sprintf("MCP %s: %s", parts[1], parts[2])
		}
	}
	return tool
}
