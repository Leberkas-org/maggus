package runner

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

// ErrInterrupted is returned when the user presses Ctrl+C.
var ErrInterrupted = fmt.Errorf("interrupted by user")

type streamEvent struct {
	Type    string          `json:"type"`
	Subtype string          `json:"subtype"`
	Message json.RawMessage `json:"message"`
	Result  string          `json:"result"`
	Usage   *streamUsage    `json:"usage,omitempty"`
}

type streamUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
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

type toolInput struct {
	Command     string `json:"command"`
	Description string `json:"description"`
	Pattern     string `json:"pattern"`
	FilePath    string `json:"file_path"`
	Skill       string `json:"skill"`
}

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

const maxToolHistory = 10
const maxCommitHistory = 5

// RunClaude invokes `claude -p <prompt>` with stream-json output and sends progress events
// to the provided bubbletea program. The caller owns the TUI lifecycle.
// If model is non-empty, --model <model> is added to the command arguments.
func RunClaude(ctx context.Context, prompt string, model string, p *tea.Program) error {
	path, err := exec.LookPath("claude")
	if err != nil {
		return fmt.Errorf("claude not found on PATH: %w\nMake sure Claude Code CLI is installed and available", err)
	}

	args := []string{
		"-p", prompt,
		"--output-format", "stream-json",
		"--verbose",
		"--dangerously-skip-permissions",
	}
	if model != "" {
		args = append(args, "--model", model)
	}

	cmd := exec.CommandContext(ctx, path, args...)

	// On Windows, put the child in a new process group so Ctrl+C goes only
	// to the Go process. We then kill the child tree via cmd.Cancel.
	setProcAttr(cmd)

	// Capture stderr for diagnostics while still showing it on terminal.
	var stderrBuf strings.Builder
	cmd.Stderr = &stderrWriter{tee: os.Stderr, buf: &stderrBuf}
	cmd.Stdin = os.Stdin
	// Kill the entire process tree on cancel so child processes don't keep stdout open.
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		if runtime.GOOS == "windows" {
			// taskkill /T kills the process tree, /F forces termination
			kill := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid))
			kill.Stderr = os.Stderr
			return kill.Run()
		}
		return cmd.Process.Kill()
	}
	// After Cancel runs, wait up to 5s then forcibly close I/O pipes so
	// cmd.Wait() doesn't hang on orphaned grandchild processes.
	cmd.WaitDelay = 5 * time.Second

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start claude: %w", err)
	}

	// Read stdout in a goroutine and send events to the bubbletea program.
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
						var input toolInput
						json.Unmarshal(block.Input, &input)
						desc := describeToolUse(block.Name, input)
						p.Send(StatusMsg{Status: "Running tool"})
						p.Send(ToolMsg{Description: desc})

						// Track skills and MCPs
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
						InputTokens:  event.Usage.InputTokens,
						OutputTokens: event.Usage.OutputTokens,
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

	// Wait for scanner to finish.
	var result scanResult
	select {
	case result = <-scanDone:
	case <-time.After(10 * time.Minute):
		// Safety timeout
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

	// Check for scanner errors
	if result.scanErr != nil {
		return fmt.Errorf("reading claude output: %w", result.scanErr)
	}

	// Detect silent failures
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

// stderrWriter tees writes to both the terminal and a buffer for diagnostics.
type stderrWriter struct {
	tee *os.File
	buf *strings.Builder
}

func (w *stderrWriter) Write(p []byte) (n int, err error) {
	w.buf.Write(p)
	return w.tee.Write(p)
}

func describeToolUse(tool string, input toolInput) string {
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
	// MCP tools: mcp__server__tool → "MCP server: tool"
	if strings.HasPrefix(tool, "mcp__") {
		parts := strings.SplitN(tool, "__", 3)
		if len(parts) == 3 {
			return fmt.Sprintf("MCP %s: %s", parts[1], parts[2])
		}
	}
	return tool
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
