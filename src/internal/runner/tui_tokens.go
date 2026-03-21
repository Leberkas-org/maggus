package runner

import (
	"fmt"
	"strings"
	"time"

	"github.com/leberkas-org/maggus/internal/agent"
)

// TaskUsage records token usage for a single task/iteration.
type TaskUsage struct {
	TaskID       string
	TaskTitle    string
	PlanFile     string
	InputTokens  int
	OutputTokens int
	StartTime    time.Time
	EndTime      time.Time
}

// tokenState holds all token usage tracking state for the TUI.
type tokenState struct {
	iterInput   int             // current iteration input tokens
	iterOutput  int             // current iteration output tokens
	totalInput  int             // cumulative input tokens
	totalOutput int             // cumulative output tokens
	hasData     bool            // true if any usage data was received
	usages      []TaskUsage     // per-task usage history
	onUsage     func(TaskUsage) // called immediately when a task's usage is finalized
}

// addUsage accumulates token counts from a usage message.
func (t *tokenState) addUsage(msg agent.UsageMsg) {
	t.iterInput += msg.InputTokens
	t.iterOutput += msg.OutputTokens
	t.totalInput += msg.InputTokens
	t.totalOutput += msg.OutputTokens
	if msg.InputTokens > 0 || msg.OutputTokens > 0 {
		t.hasData = true
	}
}

// saveAndReset saves the current iteration's token usage and resets iteration counters.
// taskID, taskTitle, planFile, and startTime come from the parent model since tokenState
// doesn't track task metadata.
func (t *tokenState) saveAndReset(taskID, taskTitle, planFile string, startTime time.Time) {
	if taskID == "" || (t.iterInput == 0 && t.iterOutput == 0) {
		return
	}
	tu := TaskUsage{
		TaskID:       taskID,
		TaskTitle:    taskTitle,
		PlanFile:     planFile,
		InputTokens:  t.iterInput,
		OutputTokens: t.iterOutput,
		StartTime:    startTime,
		EndTime:      time.Now(),
	}
	t.usages = append(t.usages, tu)
	if t.onUsage != nil {
		t.onUsage(tu)
	}
	t.iterInput = 0
	t.iterOutput = 0
}

// FormatTokens formats a token count with a `k` suffix for thousands.
// e.g., 234 → "234", 1500 → "1.5k", 12345 → "12.3k"
func FormatTokens(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	v := float64(n) / 1000.0
	// Use one decimal place, but drop trailing zero (e.g., 2.0k → "2k")
	s := fmt.Sprintf("%.1f", v)
	s = strings.TrimSuffix(s, ".0")
	return s + "k"
}
