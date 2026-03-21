package runner

import (
	"fmt"
	"strings"
	"time"

	"github.com/leberkas-org/maggus/internal/agent"
)

// TaskUsage records token usage for a single task/iteration.
type TaskUsage struct {
	TaskID                   string
	TaskTitle                string
	FeatureFile                 string
	InputTokens              int
	OutputTokens             int
	CacheCreationInputTokens int
	CacheReadInputTokens     int
	CostUSD                  float64
	ModelUsage               map[string]agent.ModelTokens
	StartTime                time.Time
	EndTime                  time.Time
}

// tokenState holds all token usage tracking state for the TUI.
type tokenState struct {
	iterInput          int                          // current iteration input tokens
	iterOutput         int                          // current iteration output tokens
	iterCacheCreation  int                          // current iteration cache creation tokens
	iterCacheRead      int                          // current iteration cache read tokens
	iterCost           float64                      // current iteration cost in USD
	totalInput         int                          // cumulative input tokens
	totalOutput        int                          // cumulative output tokens
	totalCacheCreation int                          // cumulative cache creation tokens
	totalCacheRead     int                          // cumulative cache read tokens
	totalCost          float64                      // cumulative cost in USD
	hasData            bool                         // true if any usage data was received
	iterModelUsage     map[string]agent.ModelTokens // per-iteration per-model usage
	totalModelUsage    map[string]agent.ModelTokens // cumulative per-model usage
	usages             []TaskUsage                  // per-task usage history
	onUsage            func(TaskUsage)              // called immediately when a task's usage is finalized
}

// addUsage accumulates token counts from a usage message.
func (t *tokenState) addUsage(msg agent.UsageMsg) {
	t.iterInput += msg.InputTokens
	t.iterOutput += msg.OutputTokens
	t.iterCacheCreation += msg.CacheCreationInputTokens
	t.iterCacheRead += msg.CacheReadInputTokens
	t.iterCost += msg.CostUSD
	t.totalInput += msg.InputTokens
	t.totalOutput += msg.OutputTokens
	t.totalCacheCreation += msg.CacheCreationInputTokens
	t.totalCacheRead += msg.CacheReadInputTokens
	t.totalCost += msg.CostUSD
	if msg.InputTokens > 0 || msg.OutputTokens > 0 || msg.CacheCreationInputTokens > 0 || msg.CacheReadInputTokens > 0 || msg.CostUSD > 0 {
		t.hasData = true
	}
}

// addModelUsage accumulates per-model token usage into both iter and total maps.
func (t *tokenState) addModelUsage(msg agent.ModelUsageMsg) {
	if t.iterModelUsage == nil {
		t.iterModelUsage = make(map[string]agent.ModelTokens)
	}
	if t.totalModelUsage == nil {
		t.totalModelUsage = make(map[string]agent.ModelTokens)
	}
	for name, mt := range msg.Models {
		existing := t.iterModelUsage[name]
		existing.InputTokens += mt.InputTokens
		existing.OutputTokens += mt.OutputTokens
		existing.CacheCreationInputTokens += mt.CacheCreationInputTokens
		existing.CacheReadInputTokens += mt.CacheReadInputTokens
		existing.CostUSD += mt.CostUSD
		t.iterModelUsage[name] = existing

		total := t.totalModelUsage[name]
		total.InputTokens += mt.InputTokens
		total.OutputTokens += mt.OutputTokens
		total.CacheCreationInputTokens += mt.CacheCreationInputTokens
		total.CacheReadInputTokens += mt.CacheReadInputTokens
		total.CostUSD += mt.CostUSD
		t.totalModelUsage[name] = total
	}
}

// saveAndReset saves the current iteration's token usage and resets iteration counters.
// taskID, taskTitle, featureFile, and startTime come from the parent model since tokenState
// doesn't track task metadata.
func (t *tokenState) saveAndReset(taskID, taskTitle, featureFile string, startTime time.Time) {
	if taskID == "" || (t.iterInput == 0 && t.iterOutput == 0 && t.iterCacheCreation == 0 && t.iterCacheRead == 0) {
		return
	}
	tu := TaskUsage{
		TaskID:                   taskID,
		TaskTitle:                taskTitle,
		FeatureFile:                 featureFile,
		InputTokens:              t.iterInput,
		OutputTokens:             t.iterOutput,
		CacheCreationInputTokens: t.iterCacheCreation,
		CacheReadInputTokens:     t.iterCacheRead,
		CostUSD:                  t.iterCost,
		ModelUsage:               t.iterModelUsage,
		StartTime:                startTime,
		EndTime:                  time.Now(),
	}
	t.usages = append(t.usages, tu)
	if t.onUsage != nil {
		t.onUsage(tu)
	}
	t.iterInput = 0
	t.iterOutput = 0
	t.iterCacheCreation = 0
	t.iterCacheRead = 0
	t.iterCost = 0
	t.iterModelUsage = nil
}

// FormatCost formats a USD cost value for display.
// Uses 2 decimal places for values >= $1.00, 4 decimal places otherwise.
func FormatCost(cost float64) string {
	if cost >= 1.0 {
		return fmt.Sprintf("$%.2f", cost)
	}
	return fmt.Sprintf("$%.4f", cost)
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
