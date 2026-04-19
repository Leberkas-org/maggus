package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
)

type ClaudeAgent struct{}

func (a *ClaudeAgent) Name() string { return "claude" }

func (a *ClaudeAgent) Validate() error {
	_, err := exec.LookPath("claude")
	if err != nil {
		return fmt.Errorf("claude CLI not found in PATH")
	}
	return nil
}

func (a *ClaudeAgent) Run(ctx context.Context, opts RunOptions) error {
	args := []string{
		"--print",
		"--output-format", "stream-json",
		"--max-turns", "100",
	}
	if opts.Model != "" {
		args = append(args, "--model", opts.Model)
	}
	args = append(args, opts.Prompt)

	cmd := exec.CommandContext(ctx, "claude", args...)
	cmd.Dir = opts.WorkDir

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start claude: %w", err)
	}

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if opts.Output != nil {
			parseClaudeEvent(line, opts.Output)
		}
	}

	if err := cmd.Wait(); err != nil {
		if opts.Output != nil {
			opts.Output.OnComplete(false)
		}
		return fmt.Errorf("claude exited: %w", err)
	}

	if opts.Output != nil {
		opts.Output.OnComplete(true)
	}
	return nil
}

type claudeEvent struct {
	Type    string          `json:"type"`
	Content json.RawMessage `json:"content"`
	Tool    string          `json:"tool"`
	Input   json.RawMessage `json:"input"`
	Usage   *claudeUsage    `json:"usage"`
}

type claudeUsage struct {
	InputTokens       int     `json:"input_tokens"`
	OutputTokens      int     `json:"output_tokens"`
	CacheReadTokens   int     `json:"cache_read_input_tokens"`
	CacheCreateTokens int     `json:"cache_creation_input_tokens"`
	CostUSD           float64 `json:"cost_usd"`
	Model             string  `json:"model"`
}

func parseClaudeEvent(data []byte, sink OutputSink) {
	var ev claudeEvent
	if err := json.Unmarshal(data, &ev); err != nil {
		return
	}

	switch ev.Type {
	case "assistant":
		var text string
		_ = json.Unmarshal(ev.Content, &text)
		if text != "" {
			sink.OnOutput(text)
		}
	case "tool_use":
		sink.OnTool(ToolEvent{
			Name:  ev.Tool,
			Input: string(ev.Input),
		})
	case "result":
		if ev.Usage != nil {
			sink.OnUsage(UsageEvent{
				InputTokens:       ev.Usage.InputTokens,
				OutputTokens:      ev.Usage.OutputTokens,
				CacheReadTokens:   ev.Usage.CacheReadTokens,
				CacheCreateTokens: ev.Usage.CacheCreateTokens,
				CostUSD:           ev.Usage.CostUSD,
				Model:             ev.Usage.Model,
			})
		}
	}
}
