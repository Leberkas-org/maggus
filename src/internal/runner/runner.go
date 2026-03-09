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
	"sync"
	"time"

	"golang.org/x/term"
)

// ErrInterrupted is returned when the user presses Ctrl+C.
var ErrInterrupted = fmt.Errorf("interrupted by user")

// ANSI color codes
const (
	colorReset   = "\033[0m"
	colorGreen   = "\033[32m"
	colorYellow  = "\033[33m"
	colorBlue    = "\033[34m"
	colorCyan    = "\033[36m"
	colorGray    = "\033[90m"
	colorRed     = "\033[31m"
	colorBold    = "\033[1m"
)

type streamEvent struct {
	Type    string          `json:"type"`
	Subtype string          `json:"subtype"`
	Message json.RawMessage `json:"message"`
	Result  string          `json:"result"`
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

// display holds the compact status block state.
type display struct {
	mu          sync.Mutex
	status      string
	toolHistory []string // last N tools used
	output      string
	extras      string // skills + MCPs
	toolCount   int
	skills      []string
	mcps        []string
	startTime   time.Time
	rendered    bool
	lastLines   int // how many lines we rendered last time
	frame       int
	done        chan struct{}
}

func termWidth() int {
	w, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || w <= 0 {
		return 120
	}
	return w
}

func newDisplay() *display {
	d := &display{
		status:    "Starting...",
		output:    "-",
		startTime: time.Now(),
		done:      make(chan struct{}),
	}
	// Start spinner goroutine
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-d.done:
				return
			case <-ticker.C:
				d.mu.Lock()
				d.frame = (d.frame + 1) % len(spinnerFrames)
				d.renderLocked()
				d.mu.Unlock()
			}
		}
	}()
	return d
}

func (d *display) stop() {
	close(d.done)
}

func (d *display) render() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.renderLocked()
}

func (d *display) renderLocked() {
	elapsed := time.Since(d.startTime).Truncate(time.Second)
	w := termWidth()

	// Label "    Tools:   " = 13 chars
	contentWidth := w - 13
	if contentWidth < 20 {
		contentWidth = 20
	}

	spinner := colorCyan + spinnerFrames[d.frame] + colorReset

	statusColor := colorYellow
	if d.status == "Done" {
		statusColor = colorGreen
		spinner = colorGreen + "✓" + colorReset
	} else if d.status == "Failed" {
		statusColor = colorRed
		spinner = colorRed + "✗" + colorReset
	}

	extrasStr := d.extras
	if extrasStr == "" {
		extrasStr = "-"
	}

	var lines []string
	lines = append(lines, fmt.Sprintf("  %s %sStatus:%s  %s%s%s", spinner, colorBold, colorReset, statusColor, d.status, colorReset))
	lines = append(lines, fmt.Sprintf("    %sOutput:%s  %s", colorBold, colorReset, truncate(d.output, contentWidth)))

	// Tool history
	lines = append(lines, fmt.Sprintf("    %sTools:%s   %s(%d total)%s", colorBold, colorReset, colorGray, d.toolCount, colorReset))
	for i, t := range d.toolHistory {
		prefix := colorGray + "│" + colorReset
		if i == len(d.toolHistory)-1 {
			prefix = colorBlue + "▶" + colorReset
		}
		lines = append(lines, fmt.Sprintf("    %s %s%s%s", prefix, colorBlue, truncate(t, contentWidth), colorReset))
	}
	// Pad empty lines if fewer than maxToolHistory
	for i := len(d.toolHistory); i < maxToolHistory; i++ {
		lines = append(lines, "")
	}

	lines = append(lines, fmt.Sprintf("    %sExtras:%s  %s%s%s", colorBold, colorReset, colorCyan, truncate(extrasStr, contentWidth), colorReset))
	lines = append(lines, fmt.Sprintf("    %sElapsed:%s %s%s%s", colorBold, colorReset, colorGray, elapsed, colorReset))

	// Move cursor up to overwrite previous block
	if d.rendered && d.lastLines > 0 {
		fmt.Printf("\033[%dA", d.lastLines)
	}
	d.rendered = true
	d.lastLines = len(lines)

	for _, line := range lines {
		fmt.Printf("\033[2K%s\n", line)
	}
}

func (d *display) setStatus(s string) {
	d.mu.Lock()
	d.status = s
	d.mu.Unlock()
}

func (d *display) setTool(t string) {
	d.mu.Lock()
	d.toolHistory = append(d.toolHistory, t)
	if len(d.toolHistory) > maxToolHistory {
		d.toolHistory = d.toolHistory[len(d.toolHistory)-maxToolHistory:]
	}
	d.toolCount++
	d.mu.Unlock()
}

func (d *display) addSkill(name string) {
	d.mu.Lock()
	// Deduplicate
	for _, s := range d.skills {
		if s == name {
			d.mu.Unlock()
			return
		}
	}
	d.skills = append(d.skills, name)
	d.rebuildExtras()
	d.mu.Unlock()
}

func (d *display) addMCP(name string) {
	d.mu.Lock()
	for _, m := range d.mcps {
		if m == name {
			d.mu.Unlock()
			return
		}
	}
	d.mcps = append(d.mcps, name)
	d.rebuildExtras()
	d.mu.Unlock()
}

func (d *display) rebuildExtras() {
	var parts []string
	for _, s := range d.skills {
		parts = append(parts, "skill:"+s)
	}
	for _, m := range d.mcps {
		parts = append(parts, "mcp:"+m)
	}
	d.extras = strings.Join(parts, "  ")
}

func (d *display) setOutput(o string) {
	// Take last non-empty line
	o = strings.TrimSpace(o)
	if idx := strings.LastIndex(o, "\n"); idx >= 0 {
		o = strings.TrimSpace(o[idx+1:])
	}
	if o == "" {
		return
	}
	d.mu.Lock()
	d.output = o
	d.mu.Unlock()
}

// RunClaude invokes `claude -p <prompt>` with stream-json output and displays compact progress.
// The context can be used to kill the claude process (e.g., on Ctrl+C).
func RunClaude(ctx context.Context, prompt string) error {
	path, err := exec.LookPath("claude")
	if err != nil {
		return fmt.Errorf("claude not found on PATH: %w\nMake sure Claude Code CLI is installed and available", err)
	}

	cmd := exec.CommandContext(ctx, path,
		"-p", prompt,
		"--output-format", "stream-json",
		"--verbose",
		"--dangerously-skip-permissions",
	)
	cmd.Stderr = os.Stderr
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

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start claude: %w", err)
	}

	// Hide cursor during display
	fmt.Print("\033[?25l")

	d := newDisplay()

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()

		var event streamEvent
		if err := json.Unmarshal(line, &event); err != nil {
			continue
		}

		switch event.Type {
		case "assistant":
			var msg assistantMessage
			if err := json.Unmarshal(event.Message, &msg); err != nil {
				continue
			}
			for _, block := range msg.Content {
				switch block.Type {
				case "text":
					d.setStatus("Thinking...")
					d.setOutput(block.Text)
				case "tool_use":
					var input toolInput
					json.Unmarshal(block.Input, &input)
					desc := describeToolUse(block.Name, input)
					d.setStatus("Running tool")
					d.setTool(desc)

					// Track skills and MCPs
					if block.Name == "Skill" && input.Skill != "" {
						d.addSkill(input.Skill)
					}
					if strings.HasPrefix(block.Name, "mcp__") {
						// mcp__servername__toolname → extract server name
						parts := strings.SplitN(block.Name, "__", 3)
						if len(parts) >= 2 {
							d.addMCP(parts[1])
						}
					}
				}
			}

		case "result":
			if event.Subtype == "success" {
				d.setStatus("Done")
			} else {
				d.setStatus("Failed")
				d.setOutput(event.Result)
			}
		}
	}

	// Stop spinner, final render, restore cursor
	d.stop()
	d.render()
	fmt.Print("\033[?25h")
	fmt.Println()

	if ctx.Err() != nil {
		d.setStatus("Interrupted")
		cmd.Wait() // clean up
		return ErrInterrupted
	}

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("claude exited with error: %w", err)
	}

	return nil
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
