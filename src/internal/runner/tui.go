package runner

import (
	"sync/atomic"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/leberkas-org/maggus/internal/agent"
	"github.com/leberkas-org/maggus/internal/tui/styles"
)

// Message types for the bubbletea model.
// Agent-produced types (StatusMsg, OutputMsg, ToolMsg, SkillMsg, MCPMsg, UsageMsg)
// are defined in the agent package.

// ProgressMsg is sent when iteration progress changes.
type ProgressMsg struct {
	Current int
	Total   int
}

// TaskInfoMsg is sent when the current task changes.
type TaskInfoMsg struct {
	ID    string
	Title string
}

// CommitMsg is sent when a commit completes, to display in the recent commits section.
type CommitMsg struct {
	Message string
}

// TaskUsage, tokenState, FormatTokens, and token-related methods are defined in tui_tokens.go.

// InfoMsg displays an informational message in the TUI.
type InfoMsg struct {
	Text string
}

// SummaryData, SummaryMsg, PushStatusMsg, QuitMsg, RunAgainResult, StopReason,
// RemainingTask, FailedTask types are defined in tui_summary.go.

// TaskCriterion holds a single acceptance criterion for display in the task detail view.
type TaskCriterion struct {
	Text    string
	Checked bool
	Blocked bool
}

// IterationStartMsg resets per-iteration state when a new iteration begins.
type IterationStartMsg struct {
	Current         int
	Total           int
	TaskID          string
	TaskTitle       string
	PlanFile        string
	TaskDescription string
	TaskCriteria    []TaskCriterion
}

// tickMsg is sent by the spinner ticker.
type tickMsg time.Time

// BannerInfo holds startup information displayed in the TUI's initial view.
type BannerInfo struct {
	Iterations    int
	Branch        string
	RunID         string
	RunDir        string
	Worktree      string // empty if not using worktree
	Agent         string // agent name (e.g. "claude", "opencode")
	TwoXExpiresIn string // e.g. "17h 54m 44s"; empty when not in 2x mode
}

// TUIModel is the bubbletea model that replaces the old display struct.
type TUIModel struct {
	// Header fields
	version     string
	fingerprint string
	currentIter int
	totalIters  int

	// Banner / startup info
	banner       BannerInfo
	infoMessages []string
	done         bool

	// Summary state (post-run screen)
	summary summaryState

	// Task info
	taskDescription string
	taskCriteria    []TaskCriterion
	taskID          string
	taskTitle       string
	taskPlanFile    string

	// Recent commits
	commits []string

	// Token usage tracking
	tokens tokenState

	status             string
	toolEntries        []agent.ToolMsg // full tool messages for left-side list and detail panel
	output             string
	extras             string
	model              string
	toolCount          int
	skills             []string
	mcps               []string
	startTime          time.Time
	frame              int
	width              int
	height             int
	activeTab          int          // 0 = Progress, 1 = Detail, 2 = Task, 3 = Commits
	detailScrollOffset int          // scroll offset for the detail tab (in lines)
	detailAutoScroll   bool         // true when detail tab auto-scrolls to bottom
	detailTotalLines   int          // total rendered lines in last detail render (for scroll indicator)
	stopAfterTask      bool         // when true, work stops after current task completes
	confirmingStop     bool         // when true, showing "stop after task?" confirmation prompt
	stopFlag           *atomic.Bool // shared flag readable from the work loop goroutine
	cancelFunc         func()       // called on Ctrl+C to cancel the context
	quitting           bool

	// Sync check state (between-task remote sync)
	sync syncState
}

// SetSyncDir sets the directory used for git sync operations between tasks.
func (m *TUIModel) SetSyncDir(dir string) {
	m.sync.dir = dir
}

// NewTUIModel creates a new TUI model. The cancelFunc is called on Ctrl+C to cancel the work context.
func NewTUIModel(model string, version string, fingerprint string, cancelFunc func(), banner BannerInfo) TUIModel {
	if model == "" {
		model = "default"
	}
	return TUIModel{
		version:          version,
		fingerprint:      fingerprint,
		banner:           banner,
		status:           "Waiting...",
		output:           "-",
		model:            model,
		startTime:        time.Now(),
		width:            120,
		height:           40,
		detailAutoScroll: true,
		stopFlag:         &atomic.Bool{},
		cancelFunc:       cancelFunc,
	}
}

// SetOnTaskUsage sets a callback that is invoked each time a task's usage is finalized.
func (m *TUIModel) SetOnTaskUsage(fn func(TaskUsage)) {
	m.tokens.onUsage = fn
}

// Result returns the user's choice from the summary menu.
func (m TUIModel) Result() RunAgainResult {
	return m.summary.runAgain
}

// TaskUsages returns the per-task token usage records.
func (m TUIModel) TaskUsages() []TaskUsage {
	return m.tokens.usages
}

// StopFlag returns the shared atomic flag that the work loop can poll
// to check if the user requested to stop after the current task.
func (m TUIModel) StopFlag() *atomic.Bool {
	return m.stopFlag
}

func (m TUIModel) Init() tea.Cmd {
	return tickCmd()
}

func tickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// Update is a thin message router that delegates to sub-component handlers.
func (m TUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Sync screen captures all keys when active
	if m.sync.active {
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			cmd, interrupting := m.sync.handleSyncKeys(keyMsg, &m.cancelFunc)
			if interrupting {
				m.status = "Interrupting..."
			}
			return m, cmd
		}
	}

	// Summary/done screen captures key events
	if m.done {
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			quitting, cmd := m.summary.handleSummaryKeys(keyMsg)
			if quitting {
				m.quitting = true
			}
			return m, cmd
		}
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyMsg(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		clampDetailScroll(&m)
		return m, tea.ClearScreen

	case tickMsg:
		m.frame = (m.frame + 1) % len(spinnerFrames)
		return m, tickCmd()

	case SummaryMsg, PushStatusMsg, QuitMsg:
		m.summary.handleSummaryMsg(msg, &m)

	case InfoMsg:
		m.infoMessages = append(m.infoMessages, msg.Text)

	case agent.UsageMsg:
		m.tokens.addUsage(msg)

	case IterationStartMsg:
		m.handleIterationStart(msg)

	case agent.StatusMsg:
		m.status = msg.Status

	case agent.OutputMsg:
		m.handleOutputMsg(msg)

	case agent.ToolMsg:
		m.handleToolMsg(msg)

	case agent.SkillMsg:
		m.handleSkillMsg(msg)

	case agent.MCPMsg:
		m.handleMCPMsg(msg)

	case ProgressMsg:
		m.currentIter = msg.Current
		m.totalIters = msg.Total

	case TaskInfoMsg:
		m.taskID = msg.ID
		m.taskTitle = msg.Title

	case CommitMsg:
		m.handleCommitMsg(msg)

	case SyncCheckMsg, syncActionDoneMsg:
		if handled, cmd := m.sync.handleSyncMsg(msg, &m.infoMessages); handled {
			return m, cmd
		}
	}

	return m, nil
}

// View dispatches to the appropriate view renderer.
func (m TUIModel) View() string {
	if m.sync.active {
		return m.sync.renderSyncView(&m)
	}
	if m.summary.show || m.done {
		return m.summary.renderSummaryView(&m)
	}
	if m.taskID == "" {
		return m.renderBannerView()
	}
	return m.renderView()
}

// Styles — aliases to the shared style package for concise rendering code.
var (
	boldStyle   = styles.Label
	statusStyle = lipgloss.NewStyle().Foreground(styles.Warning)
	greenStyle  = lipgloss.NewStyle().Foreground(styles.Success)
	redStyle    = lipgloss.NewStyle().Foreground(styles.Error)
	cyanStyle   = lipgloss.NewStyle().Foreground(styles.Primary)
	blueStyle   = lipgloss.NewStyle().Foreground(styles.Accent)
	grayStyle   = lipgloss.NewStyle().Foreground(styles.Muted)
)

// handleSummaryKeys, renderSummaryView, renderSummaryMenu are defined in tui_summary.go.
// handleKeyMsg, handleStopConfirmation, handleDetailScroll, handleTabSwitch are defined in tui_keys.go.
// handleIterationStart, handleOutputMsg, handleToolMsg, handleSkillMsg, handleMCPMsg, handleCommitMsg, rebuildExtras are defined in tui_messages.go.
// renderBannerView, renderHeaderInner, renderTabBar, renderDetailPanel, renderTaskTab, renderView are defined in tui_render.go.
// toolIcon, detailAvailableHeight, clampDetailScroll, countDetailLines are defined in tui_render.go.

// Sync check state is embedded in TUIModel via syncState (defined in tui_sync.go).
