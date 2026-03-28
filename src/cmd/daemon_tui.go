package cmd

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/leberkas-org/maggus/internal/agent"
	"github.com/leberkas-org/maggus/internal/runlog"
	"github.com/leberkas-org/maggus/internal/runner"
)

// nullTUIModel is a minimal bubbletea model used in daemon mode.
// It discards all display messages but correctly handles QuitMsg to
// terminate the event loop, and auto-responds to SyncCheckMsg so the
// work goroutine is never left waiting for user input.
// It also tracks token usage via UsageMsg/ModelUsageMsg and flushes
// a usage record when a task boundary is reached (IterationStartMsg or QuitMsg).
// Additionally, it writes a state.json snapshot on each significant event
// so the status view can render a rich live TUI.
type nullTUIModel struct {
	taskID       string
	taskTitle    string
	itemID       string // stable UUID from <!-- maggus-id: ... -->
	itemShort    string // e.g. "feature_001"
	itemTitle    string // parsed H1 title from the feature/bug file
	startTime    time.Time
	runStartedAt time.Time
	status       string
	onToolUse    func(taskID, toolType string, params map[string]string)
	onOutput     func(taskID, text string)
	onTaskUsage  func(runner.TaskUsage)

	// Snapshot state — written to state.json on each event.
	snapshotDir   string // project root directory
	snapshotRunID string // run ID for the snapshot path
	toolEntries   []runlog.SnapshotToolEntry
	commits       []string

	// Token accumulation for current iteration.
	iterInput         int
	iterOutput        int
	iterCacheCreation int
	iterCacheRead     int
	iterCost          float64
	iterModelUsage    map[string]agent.ModelTokens
}

// SetOnToolUse sets a callback invoked on each tool use event.
func (m *nullTUIModel) SetOnToolUse(fn func(taskID, toolType string, params map[string]string)) {
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
		m.itemID = msg.ItemID
		m.itemShort = msg.ItemShort
		m.itemTitle = msg.ItemTitle
		m.startTime = time.Now()
		m.status = "Starting"
		m.toolEntries = nil
		m.commits = nil
		m.writeSnapshot()
	case runner.CommitMsg:
		m.commits = append(m.commits, msg.Message)
		m.writeSnapshot()
	case agent.StatusMsg:
		m.status = msg.Status
		m.writeSnapshot()
	case agent.UsageMsg:
		m.iterInput += msg.InputTokens
		m.iterOutput += msg.OutputTokens
		m.iterCacheCreation += msg.CacheCreationInputTokens
		m.iterCacheRead += msg.CacheReadInputTokens
		m.iterCost += msg.CostUSD
		m.writeSnapshot()
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
		m.toolEntries = append(m.toolEntries, runlog.SnapshotToolEntry{
			Type:        msg.Type,
			Icon:        toolIconForSnapshot(msg.Type),
			Description: msg.Description,
			Timestamp:   msg.Timestamp.UTC().Format(time.RFC3339),
		})
		if m.onToolUse != nil {
			m.onToolUse(m.taskID, msg.Type, msg.Params)
		}
		m.writeSnapshot()
	case agent.OutputMsg:
		if m.onOutput != nil {
			m.onOutput(m.taskID, strings.TrimSpace(msg.Text))
		}
	}
	return m, nil
}

// writeSnapshot writes the current state to state.json.
func (m *nullTUIModel) writeSnapshot() {
	if m.snapshotDir == "" || m.snapshotRunID == "" {
		return
	}
	snap := runlog.StateSnapshot{
		RunID:          m.snapshotRunID,
		TaskID:         m.taskID,
		TaskTitle:      m.taskTitle,
		ItemTitle:      m.itemTitle,
		Status:         m.status,
		ToolEntries:    m.toolEntries,
		TokenInput:     m.iterInput,
		TokenOutput:    m.iterOutput,
		TokenCost:      m.iterCost,
		ModelBreakdown: m.iterModelUsage,
		Commits:        m.commits,
		RunStartedAt:   m.runStartedAt.UTC().Format(time.RFC3339),
		TaskStartedAt:  m.startTime.UTC().Format(time.RFC3339),
	}
	// Best-effort write; errors are not fatal for the daemon.
	_ = runlog.WriteSnapshot(m.snapshotDir, snap)
}

// flushUsage saves accumulated token usage for the current task and resets counters.
func (m *nullTUIModel) flushUsage() {
	if m.taskID == "" || (m.iterInput == 0 && m.iterOutput == 0 && m.iterCacheCreation == 0 && m.iterCacheRead == 0) {
		return
	}
	tu := runner.TaskUsage{
		ItemID:                   m.itemID,
		ItemShort:                m.itemShort,
		ItemTitle:                m.itemTitle,
		TaskShort:                m.taskID,
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

// toolIconForSnapshot maps tool types to display icons for the state snapshot.
func toolIconForSnapshot(toolType string) string {
	switch toolType {
	case "Read":
		return "📖"
	case "Edit":
		return "✏️"
	case "Write":
		return "📝"
	case "Bash":
		return "⚡"
	case "Glob":
		return "🔍"
	case "Grep":
		return "🔎"
	case "Skill":
		return "🎯"
	case "Agent":
		return "🤖"
	default:
		if strings.HasPrefix(toolType, "mcp__") {
			return "🔌"
		}
		return "🥚"
	}
}
