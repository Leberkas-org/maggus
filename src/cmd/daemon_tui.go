package cmd

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/leberkas-org/maggus/internal/agent"
	"github.com/leberkas-org/maggus/internal/runner"
)

// nullTUIModel is a minimal bubbletea model used in daemon mode.
// It discards all display messages but correctly handles QuitMsg to
// terminate the event loop, and auto-responds to SyncCheckMsg so the
// work goroutine is never left waiting for user input.
// It also tracks token usage via UsageMsg/ModelUsageMsg and flushes
// a usage record when a task boundary is reached (IterationStartMsg or QuitMsg).
type nullTUIModel struct {
	taskID          string
	taskTitle       string
	taskFeatureFile string
	startTime       time.Time
	onToolUse       func(taskID, toolType, description string)
	onOutput        func(taskID, text string)
	onTaskUsage     func(runner.TaskUsage)

	// Token accumulation for current iteration.
	iterInput         int
	iterOutput        int
	iterCacheCreation int
	iterCacheRead     int
	iterCost          float64
	iterModelUsage    map[string]agent.ModelTokens
}

// SetOnToolUse sets a callback invoked on each tool use event.
func (m *nullTUIModel) SetOnToolUse(fn func(taskID, toolType, description string)) {
	m.onToolUse = fn
}

// SetOnOutput sets a callback invoked on each agent output event.
func (m *nullTUIModel) SetOnOutput(fn func(taskID, text string)) {
	m.onOutput = fn
}

// SetOnTaskUsage sets a callback invoked when a task's token usage is finalized.
func (m *nullTUIModel) SetOnTaskUsage(fn func(runner.TaskUsage)) {
	m.onTaskUsage = fn
}

func (m nullTUIModel) Init() tea.Cmd { return nil }

func (m nullTUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case runner.QuitMsg:
		_ = msg
		m.flushUsage()
		return m, tea.Quit
	case runner.SyncCheckMsg:
		// Auto-continue in daemon mode: skip the interactive sync screen.
		go func(ch chan<- runner.SyncCheckResult) {
			ch <- runner.SyncCheckResult{
				Action:  runner.SyncProceed,
				Message: "⚠ Remote sync skipped (daemon mode)",
			}
		}(msg.ResultCh)
	case runner.IterationStartMsg:
		m.flushUsage()
		m.taskID = msg.TaskID
		m.taskTitle = msg.TaskTitle
		m.taskFeatureFile = msg.FeatureFile
		m.startTime = time.Now()
	case agent.UsageMsg:
		m.iterInput += msg.InputTokens
		m.iterOutput += msg.OutputTokens
		m.iterCacheCreation += msg.CacheCreationInputTokens
		m.iterCacheRead += msg.CacheReadInputTokens
		m.iterCost += msg.CostUSD
	case agent.ModelUsageMsg:
		if m.iterModelUsage == nil {
			m.iterModelUsage = make(map[string]agent.ModelTokens)
		}
		for name, entry := range msg.Models {
			existing := m.iterModelUsage[name]
			existing.InputTokens += entry.InputTokens
			existing.OutputTokens += entry.OutputTokens
			existing.CacheCreationInputTokens += entry.CacheCreationInputTokens
			existing.CacheReadInputTokens += entry.CacheReadInputTokens
			existing.CostUSD += entry.CostUSD
			m.iterModelUsage[name] = existing
		}
	case agent.ToolMsg:
		if m.onToolUse != nil {
			m.onToolUse(m.taskID, msg.Type, msg.Description)
		}
	case agent.OutputMsg:
		if m.onOutput != nil {
			m.onOutput(m.taskID, strings.TrimSpace(msg.Text))
		}
	}
	return m, nil
}

// flushUsage saves accumulated token usage for the current task and resets counters.
func (m *nullTUIModel) flushUsage() {
	if m.taskID == "" || (m.iterInput == 0 && m.iterOutput == 0 && m.iterCacheCreation == 0 && m.iterCacheRead == 0) {
		return
	}
	tu := runner.TaskUsage{
		TaskID:                   m.taskID,
		TaskTitle:                m.taskTitle,
		FeatureFile:              m.taskFeatureFile,
		InputTokens:              m.iterInput,
		OutputTokens:             m.iterOutput,
		CacheCreationInputTokens: m.iterCacheCreation,
		CacheReadInputTokens:     m.iterCacheRead,
		CostUSD:                  m.iterCost,
		ModelUsage:               m.iterModelUsage,
		StartTime:                m.startTime,
		EndTime:                  time.Now(),
	}
	if m.onTaskUsage != nil {
		m.onTaskUsage(tu)
	}
	m.iterInput = 0
	m.iterOutput = 0
	m.iterCacheCreation = 0
	m.iterCacheRead = 0
	m.iterCost = 0
	m.iterModelUsage = nil
}

func (m nullTUIModel) View() string { return "" }
