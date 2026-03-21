package agent

import (
	"fmt"
	"time"
)

// ErrInterrupted is returned when the user presses Ctrl+C.
var ErrInterrupted = fmt.Errorf("interrupted by user")

// Message types sent by agents to the bubbletea program during streaming.

// StatusMsg is sent when the status changes (e.g. "Thinking...", "Running tool", "Done").
type StatusMsg struct {
	Status string
}

// OutputMsg is sent when new assistant text output arrives.
type OutputMsg struct {
	Text string
}

// ToolMsg is sent when a new tool use is detected.
type ToolMsg struct {
	Description string
	Type        string            // tool name, e.g. "Read", "Bash", "Grep"
	Params      map[string]string // key detail parameters for the detail panel
	Timestamp   time.Time         // when the tool was invoked
}

// SkillMsg is sent when a skill is used.
type SkillMsg struct {
	Name string
}

// MCPMsg is sent when an MCP tool is used.
type MCPMsg struct {
	Name string
}

// UsageMsg is sent when a result event contains token usage data.
type UsageMsg struct {
	InputTokens              int
	OutputTokens             int
	CacheCreationInputTokens int
	CacheReadInputTokens     int
	CostUSD                  float64
}
