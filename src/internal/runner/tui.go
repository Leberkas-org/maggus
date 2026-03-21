package runner

import (
	"fmt"
	"sort"
	"strings"
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
	iterInputTokens   int             // current iteration input tokens
	iterOutputTokens  int             // current iteration output tokens
	totalInputTokens  int             // cumulative input tokens
	totalOutputTokens int             // cumulative output tokens
	hasUsageData      bool            // true if any usage data was received
	taskUsages        []TaskUsage     // per-task usage history
	onTaskUsage       func(TaskUsage) // called immediately when a task's usage is finalized

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
	m.onTaskUsage = fn
}

// saveIterationUsage saves the current iteration's token usage and invokes the callback.
// Called from Update (value receiver), so it must operate on the value directly.
func saveIterationUsage(m *TUIModel) {
	if m.taskID == "" || (m.iterInputTokens == 0 && m.iterOutputTokens == 0) {
		return
	}
	tu := TaskUsage{
		TaskID:       m.taskID,
		TaskTitle:    m.taskTitle,
		PlanFile:     m.taskPlanFile,
		InputTokens:  m.iterInputTokens,
		OutputTokens: m.iterOutputTokens,
		StartTime:    m.startTime,
		EndTime:      time.Now(),
	}
	m.taskUsages = append(m.taskUsages, tu)
	if m.onTaskUsage != nil {
		m.onTaskUsage(tu)
	}
}

// Result returns the user's choice from the summary menu.
func (m TUIModel) Result() RunAgainResult {
	return m.summary.runAgain
}

// TaskUsages returns the per-task token usage records.
func (m TUIModel) TaskUsages() []TaskUsage {
	return m.taskUsages
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

func (m TUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Sync screen captures all keys when active
		if m.sync.active {
			cmd, interrupting := m.sync.handleSyncKeys(msg, &m.cancelFunc)
			if interrupting {
				m.status = "Interrupting..."
			}
			return m, cmd
		}
		if m.done {
			quitting, cmd := m.summary.handleSummaryKeys(msg)
			if quitting {
				m.quitting = true
			}
			return m, cmd
		}
		if m.summary.show && msg.Type == tea.KeyCtrlC {
			// Ctrl+C on summary exits immediately
			m.quitting = true
			return m, tea.Quit
		}
		if msg.Type == tea.KeyCtrlC {
			m.confirmingStop = false
			m.status = "Interrupting..."
			if m.cancelFunc != nil {
				m.cancelFunc()
				m.cancelFunc = nil // prevent double-cancel
			}
			return m, nil
		}
		// Handle stop-after-task confirmation prompt
		if m.confirmingStop {
			if len(msg.Runes) == 1 {
				switch msg.Runes[0] {
				case 'y', 'Y':
					m.confirmingStop = false
					m.stopAfterTask = true
					m.stopFlag.Store(true)
					return m, nil
				case 'n', 'N':
					m.confirmingStop = false
					return m, nil
				}
			}
			if msg.Type == tea.KeyEscape {
				m.confirmingStop = false
				return m, nil
			}
			return m, nil
		}
		// Alt+S toggles stop-after-task (confirm to enable, instant to revert)
		if m.taskID != "" && !m.summary.show && msg.Alt && len(msg.Runes) == 1 && (msg.Runes[0] == 's' || msg.Runes[0] == 'S') {
			if m.stopAfterTask {
				m.stopAfterTask = false
				m.stopFlag.Store(false)
			} else {
				m.confirmingStop = true
			}
			return m, nil
		}
		// Detail tab scrolling (tab 1)
		if m.activeTab == 1 && m.taskID != "" {
			switch msg.Type {
			case tea.KeyUp:
				if m.detailScrollOffset > 0 {
					m.detailScrollOffset--
					m.detailAutoScroll = false
				}
				return m, nil
			case tea.KeyDown:
				m.detailScrollOffset++
				clampDetailScroll(&m)
				return m, nil
			case tea.KeyHome:
				m.detailScrollOffset = 0
				m.detailAutoScroll = false
				return m, nil
			case tea.KeyEnd:
				m.detailScrollOffset = m.detailTotalLines
				m.detailAutoScroll = true
				clampDetailScroll(&m)
				return m, nil
			}
		}
		// Tab switching: arrow keys and number keys
		if m.taskID != "" {
			const maxTab = 3
			switch msg.Type {
			case tea.KeyLeft:
				if m.activeTab > 0 {
					m.activeTab--
				}
				return m, nil
			case tea.KeyRight:
				if m.activeTab < maxTab {
					m.activeTab++
				}
				return m, nil
			}
			if len(msg.Runes) == 1 {
				switch msg.Runes[0] {
				case '1':
					m.activeTab = 0
					return m, nil
				case '2':
					m.activeTab = 1
					return m, nil
				case '3':
					m.activeTab = 2
					return m, nil
				case '4':
					m.activeTab = 3
					return m, nil
				}
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		clampDetailScroll(&m)
		return m, tea.ClearScreen

	case tickMsg:
		m.frame = (m.frame + 1) % len(spinnerFrames)
		return m, tickCmd()

	case SummaryMsg, PushStatusMsg, QuitMsg:
		if m.summary.handleSummaryMsg(msg, &m) {
			return m, nil
		}

	case InfoMsg:
		m.infoMessages = append(m.infoMessages, msg.Text)

	case agent.UsageMsg:
		m.iterInputTokens += msg.InputTokens
		m.iterOutputTokens += msg.OutputTokens
		m.totalInputTokens += msg.InputTokens
		m.totalOutputTokens += msg.OutputTokens
		if msg.InputTokens > 0 || msg.OutputTokens > 0 {
			m.hasUsageData = true
		}

	case IterationStartMsg:
		// Save previous iteration's usage before resetting.
		saveIterationUsage(&m)
		m.currentIter = msg.Current
		m.totalIters = msg.Total
		m.taskID = msg.TaskID
		m.taskTitle = msg.TaskTitle
		m.taskPlanFile = msg.PlanFile
		m.taskDescription = msg.TaskDescription
		m.taskCriteria = msg.TaskCriteria
		// Reset per-iteration state
		m.status = "Starting..."
		m.output = "-"
		m.toolEntries = nil
		m.toolCount = 0
		m.extras = ""
		m.skills = nil
		m.mcps = nil
		m.iterInputTokens = 0
		m.iterOutputTokens = 0
		m.detailScrollOffset = 0
		m.detailAutoScroll = true
		m.detailTotalLines = 0
		m.startTime = time.Now()

	case agent.StatusMsg:
		m.status = msg.Status

	case agent.OutputMsg:
		text := strings.TrimSpace(msg.Text)
		if idx := strings.LastIndex(text, "\n"); idx >= 0 {
			text = strings.TrimSpace(text[idx+1:])
		}
		if text != "" {
			m.output = text
		}

	case agent.ToolMsg:
		m.toolEntries = append(m.toolEntries, msg)
		m.toolCount++
		// Update total lines and auto-scroll if enabled
		m.detailTotalLines = m.countDetailLines()
		if m.detailAutoScroll {
			m.detailScrollOffset = m.detailTotalLines // will be clamped
		}
		clampDetailScroll(&m)

	case agent.SkillMsg:
		for _, s := range m.skills {
			if s == msg.Name {
				return m, nil
			}
		}
		m.skills = append(m.skills, msg.Name)
		m.rebuildExtras()

	case agent.MCPMsg:
		for _, s := range m.mcps {
			if s == msg.Name {
				return m, nil
			}
		}
		m.mcps = append(m.mcps, msg.Name)
		m.rebuildExtras()

	case ProgressMsg:
		m.currentIter = msg.Current
		m.totalIters = msg.Total

	case TaskInfoMsg:
		m.taskID = msg.ID
		m.taskTitle = msg.Title

	case CommitMsg:
		m.commits = append(m.commits, msg.Message)
		if len(m.commits) > maxCommitHistory {
			m.commits = m.commits[len(m.commits)-maxCommitHistory:]
		}

	case SyncCheckMsg, syncActionDoneMsg:
		if handled, cmd := m.sync.handleSyncMsg(msg, &m.infoMessages); handled {
			return m, cmd
		}
	}

	return m, nil
}

func (m *TUIModel) rebuildExtras() {
	var parts []string
	for _, s := range m.skills {
		parts = append(parts, "skill:"+s)
	}
	for _, s := range m.mcps {
		parts = append(parts, "mcp:"+s)
	}
	m.extras = strings.Join(parts, "  ")
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

func (m TUIModel) renderBannerView() string {
	innerW, _ := styles.FullScreenInnerSize(m.width, m.height)

	var b strings.Builder
	b.WriteString(m.renderHeaderInner(innerW))
	b.WriteString("\n")
	if m.banner.Agent != "" {
		b.WriteString(fmt.Sprintf("%s  %s\n", boldStyle.Render("Agent:"), m.banner.Agent))
	}
	b.WriteString(fmt.Sprintf("%s  %s\n", boldStyle.Render("Model:"), m.model))
	b.WriteString(fmt.Sprintf("%s  %d\n", boldStyle.Render("Tasks:"), m.banner.Iterations))
	if m.banner.Branch != "" {
		b.WriteString(fmt.Sprintf("%s %s\n", boldStyle.Render("Branch:"), m.banner.Branch))
	}
	b.WriteString(fmt.Sprintf("%s %s\n", boldStyle.Render("Run ID:"), m.banner.RunID))
	if m.banner.Worktree != "" {
		b.WriteString(fmt.Sprintf("%s  %s\n", boldStyle.Render("Worktree:"), m.banner.Worktree))
	}
	b.WriteString("\n")
	for _, msg := range m.infoMessages {
		b.WriteString(fmt.Sprintf("%s\n", msg))
	}
	if len(m.infoMessages) == 0 {
		b.WriteString(fmt.Sprintf("%s\n", grayStyle.Render("Starting...")))
	}

	footer := styles.StatusBar.Render("ctrl+c stop")

	if m.width > 0 && m.height > 0 {
		return styles.FullScreenLeft(b.String(), footer, m.width, m.height)
	}
	return styles.Box.Render(b.String()) + "\n"
}

// handleSummaryKeys, renderSummaryView, renderSummaryMenu are defined in tui_summary.go.

// renderHeaderInner renders the header content for use inside a bordered box.
func (m TUIModel) renderHeaderInner(w int) string {
	if w < 40 {
		w = 40
	}

	var b strings.Builder

	// Line 1: version left, fingerprint right
	left := boldStyle.Render(fmt.Sprintf("Maggus v%s", m.version))
	right := ""
	if m.fingerprint != "" {
		right = grayStyle.Render(m.fingerprint)
	}
	leftRaw := fmt.Sprintf("Maggus v%s", m.version)
	rightRaw := m.fingerprint
	padding := w - len(leftRaw) - len(rightRaw)
	if padding < 2 {
		padding = 2
	}
	b.WriteString(fmt.Sprintf("%s%s%s\n", left, strings.Repeat(" ", padding), right))

	// Line 2: 2x remaining time (only when active)
	if m.banner.TwoXExpiresIn != "" {
		twoXStyle := lipgloss.NewStyle().Foreground(styles.Warning)
		b.WriteString(twoXStyle.Render(fmt.Sprintf("2x: %s", m.banner.TwoXExpiresIn)) + "\n")
	}

	// Line 3: progress bar
	if m.totalIters > 0 {
		barWidth := 20
		bar := styles.ProgressBar(m.currentIter, m.totalIters, barWidth)
		progress := fmt.Sprintf("[%s] %s", bar,
			greenStyle.Render(fmt.Sprintf("%d/%d Tasks", m.currentIter, m.totalIters)))
		b.WriteString(progress + "\n")
	}

	// Separator line
	b.WriteString(styles.Separator(w) + "\n")

	return b.String()
}

// renderTabBar renders the horizontal tab bar for the work view.
func (m TUIModel) renderTabBar(w int) string {
	selectedStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.Primary)
	unselectedStyle := lipgloss.NewStyle().Foreground(styles.Muted)
	sep := grayStyle.Render("│")

	labels := []string{
		" Progress ",
		fmt.Sprintf(" Detail (%d) ", m.toolCount),
		" Task ",
		" Commits ",
	}
	if len(m.commits) > 0 {
		labels[3] = fmt.Sprintf(" Commits (%d) ", len(m.commits))
	}

	var parts []string
	for i, label := range labels {
		if i == m.activeTab {
			parts = append(parts, selectedStyle.Render(label))
		} else {
			parts = append(parts, unselectedStyle.Render(label))
		}
	}

	return strings.Join(parts, sep) + "\n" + styles.Separator(w) + "\n"
}

// toolIcon returns an emoji icon for a tool type.
func toolIcon(toolType string) string {
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
		return "▶️"
	}
}

// detailAvailableHeight returns the number of visible lines in the detail panel viewport.
func (m TUIModel) detailAvailableHeight() int {
	_, innerH := styles.FullScreenInnerSize(m.width, m.height)
	// Reserve lines for: header section (~5), task info (2), detail header+separator (2), footer (1)
	available := innerH - 10
	if available < 1 {
		available = 1
	}
	return available
}

// clampDetailScroll ensures detailScrollOffset is within valid bounds and updates auto-scroll state.
func clampDetailScroll(m *TUIModel) {
	available := m.detailAvailableHeight()
	maxOffset := m.detailTotalLines - available
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.detailScrollOffset > maxOffset {
		m.detailScrollOffset = maxOffset
	}
	if m.detailScrollOffset < 0 {
		m.detailScrollOffset = 0
	}
	// Re-enable auto-scroll if scrolled to bottom
	if m.detailScrollOffset >= maxOffset && maxOffset > 0 {
		m.detailAutoScroll = true
	}
}

// countDetailLines calculates the total number of rendered lines for the current tool entries.
func (m TUIModel) countDetailLines() int {
	total := 0
	for i, entry := range m.toolEntries {
		if i > 0 {
			total++ // separator line
		}
		total++ // header line
		total += len(entry.Params)
	}
	return total
}

// renderDetailPanel renders the right-side tool detail panel content.
func (m TUIModel) renderDetailPanel(w, h int) string {
	if w < 10 {
		w = 10
	}

	var b strings.Builder

	if len(m.toolEntries) == 0 {
		b.WriteString(grayStyle.Render("No tool invocations yet.") + "\n")
		return b.String()
	}

	// Build all entry lines
	var entryLines []string
	for i, entry := range m.toolEntries {
		if i > 0 {
			entryLines = append(entryLines, grayStyle.Render(strings.Repeat("·", w)))
		}

		icon := toolIcon(entry.Type)
		styledIcon := cyanStyle.Render(icon)
		ts := entry.Timestamp.Format("15:04:05")
		styledTs := grayStyle.Render(ts)
		desc := entry.Description
		// Reserve 2 extra chars of margin so emojis with inconsistent
		// terminal widths don't push the timestamp to the next line.
		const emojiMargin = 2
		iconW := lipgloss.Width(styledIcon)
		tsW := 8 // "15:04:05" is always 8 chars
		fixedCols := iconW + 1 + 1 + tsW + emojiMargin
		maxDesc := w - fixedCols
		if maxDesc < 0 {
			maxDesc = 0
		}
		desc = styles.Truncate(desc, maxDesc)
		styledDesc := blueStyle.Render(desc)
		// Right-align timestamp: measure the composed left part and pad.
		// Subtract emojiMargin from available width so the ts sits 2 chars from the edge.
		leftW := lipgloss.Width(styledIcon) + 1 + lipgloss.Width(styledDesc)
		pad := (w - emojiMargin) - leftW - tsW
		if pad < 1 {
			pad = 1
		}
		header := styledIcon + " " + styledDesc + strings.Repeat(" ", pad) + styledTs
		entryLines = append(entryLines, header)

		// Sort param keys for stable render order
		paramKeys := make([]string, 0, len(entry.Params))
		for k := range entry.Params {
			paramKeys = append(paramKeys, k)
		}
		sort.Strings(paramKeys)
		for _, k := range paramKeys {
			v := entry.Params[k]
			// "  " indent=2 + key + ":" + space=1 = len(k)+4
			maxVal := w - len(k) - 4
			if maxVal < 0 {
				maxVal = 0
			}
			paramLine := fmt.Sprintf("  %s %s", grayStyle.Render(k+":"), styles.Truncate(v, maxVal))
			entryLines = append(entryLines, paramLine)
		}
	}

	// Viewport calculation
	available := h - 3 // header + separator lines
	if available < 1 {
		available = 1
	}

	// Clamp offset for this render
	offset := m.detailScrollOffset
	maxOffset := len(entryLines) - available
	if maxOffset < 0 {
		maxOffset = 0
	}
	if offset > maxOffset {
		offset = maxOffset
	}
	if offset < 0 {
		offset = 0
	}

	// Scroll indicator when content overflows
	if len(entryLines) > available {
		end := offset + available
		if end > len(entryLines) {
			end = len(entryLines)
		}
		indicator := grayStyle.Render(fmt.Sprintf("[%d-%d of %d]", offset+1, end, len(entryLines)))
		b.WriteString(indicator + "\n")
	}

	// Render visible window
	end := offset + available
	if end > len(entryLines) {
		end = len(entryLines)
	}
	for _, line := range entryLines[offset:end] {
		b.WriteString(line + "\n")
	}

	return b.String()
}

// renderTaskTab renders the task description and acceptance criteria for the Task tab.
func (m TUIModel) renderTaskTab(w int) string {
	var b strings.Builder

	// Task metadata
	labelStyle := styles.Label.Width(12).Align(lipgloss.Right)
	valStyle := lipgloss.NewStyle().Foreground(styles.Muted)

	b.WriteString(fmt.Sprintf("%s  %s\n", labelStyle.Render("Plan:"), valStyle.Render(m.taskPlanFile)))

	done := 0
	for _, c := range m.taskCriteria {
		if c.Checked {
			done++
		}
	}
	b.WriteString(fmt.Sprintf("%s  %s\n",
		labelStyle.Render("Criteria:"),
		valStyle.Render(fmt.Sprintf("%d/%d", done, len(m.taskCriteria)))))

	b.WriteString("\n")

	// Description
	if m.taskDescription != "" {
		b.WriteString(styles.Subtitle.Render("Description") + "\n")
		b.WriteString(styles.Separator(w) + "\n")
		for _, line := range strings.Split(m.taskDescription, "\n") {
			if len(line) > w {
				line = styles.Truncate(line, w)
			}
			b.WriteString(line + "\n")
		}
		b.WriteString("\n")
	}

	// Acceptance criteria
	if len(m.taskCriteria) > 0 {
		b.WriteString(styles.Subtitle.Render("Acceptance Criteria") + "\n")
		b.WriteString(styles.Separator(w) + "\n")
		for _, c := range m.taskCriteria {
			var icon string
			if c.Checked {
				icon = greenStyle.Render("✓")
			} else if c.Blocked {
				icon = redStyle.Render("⚠")
			} else {
				icon = grayStyle.Render("○")
			}
			text := styles.Truncate(c.Text, w-4)
			b.WriteString(fmt.Sprintf("  %s %s\n", icon, text))
		}
	}

	return b.String()
}

func (m TUIModel) renderView() string {
	elapsed := time.Since(m.startTime).Truncate(time.Second)
	innerW, innerH := styles.FullScreenInnerSize(m.width, m.height)

	spinner := cyanStyle.Render(spinnerFrames[m.frame])
	sColor := statusStyle
	if m.status == "Done" {
		sColor = greenStyle
		spinner = greenStyle.Render("✓")
	} else if m.status == "Failed" {
		sColor = redStyle
		spinner = redStyle.Render("✗")
	} else if m.status == "Interrupted" {
		sColor = redStyle
		spinner = redStyle.Render("⊘")
	}

	extrasStr := m.extras
	if extrasStr == "" {
		extrasStr = "-"
	}

	contentWidth := innerW - 11
	if contentWidth < 20 {
		contentWidth = 20
	}

	var b strings.Builder

	// Render header inside the box (full width)
	b.WriteString(m.renderHeaderInner(innerW))

	// Render task info (full width)
	if m.taskID != "" {
		taskLine := fmt.Sprintf("%s %s", cyanStyle.Render(m.taskID+":"), m.taskTitle)
		b.WriteString(taskLine + "\n\n")
	}

	// Tab bar
	b.WriteString(m.renderTabBar(innerW))

	// Tab content
	switch m.activeTab {
	case 0: // Progress
		b.WriteString(fmt.Sprintf("%s %s  %s\n", spinner, boldStyle.Render("Status:"), sColor.Render(m.status)))
		b.WriteString(fmt.Sprintf("  %s  %s\n", boldStyle.Render("Output:"), styles.Truncate(m.output, contentWidth)))

		b.WriteString(fmt.Sprintf("  %s   %s\n", boldStyle.Render("Tools:"), grayStyle.Render(fmt.Sprintf("(%d total)", m.toolCount))))
		recentStart := 0
		if len(m.toolEntries) > maxToolHistory {
			recentStart = len(m.toolEntries) - maxToolHistory
		}
		recentTools := m.toolEntries[recentStart:]
		for i, entry := range recentTools {
			prefix := grayStyle.Render("│")
			if i == len(recentTools)-1 {
				prefix = blueStyle.Render("▶")
			}
			b.WriteString(fmt.Sprintf("  %s %s\n", prefix, blueStyle.Render(styles.Truncate(entry.Description, contentWidth))))
		}
		for i := len(recentTools); i < maxToolHistory; i++ {
			b.WriteString("\n")
		}

		b.WriteString(fmt.Sprintf("  %s  %s\n", boldStyle.Render("Extras:"), cyanStyle.Render(styles.Truncate(extrasStr, contentWidth))))
		b.WriteString(fmt.Sprintf("  %s   %s\n", boldStyle.Render("Model:"), grayStyle.Render(m.model)))
		b.WriteString(fmt.Sprintf("  %s %s\n", boldStyle.Render("Elapsed:"), grayStyle.Render(elapsed.String())))

		if m.hasUsageData {
			tokenStr := fmt.Sprintf("%s in / %s out", FormatTokens(m.totalInputTokens), FormatTokens(m.totalOutputTokens))
			b.WriteString(fmt.Sprintf("  %s  %s\n", boldStyle.Render("Tokens:"), grayStyle.Render(tokenStr)))
		} else {
			b.WriteString(fmt.Sprintf("  %s  %s\n", boldStyle.Render("Tokens:"), grayStyle.Render("N/A")))
		}

	case 1: // Detail (tool log)
		b.WriteString(m.renderDetailPanel(innerW, innerH-8))

	case 2: // Task
		b.WriteString(m.renderTaskTab(innerW))

	case 3: // Commits
		if len(m.commits) == 0 {
			b.WriteString(grayStyle.Render("No commits yet.") + "\n")
		} else {
			for _, c := range m.commits {
				line := styles.Truncate(c, innerW-4)
				b.WriteString(fmt.Sprintf("  %s %s\n",
					grayStyle.Render("•"),
					grayStyle.Render(line)))
			}
		}
	}

	// Footer with context-sensitive keybindings
	var footer string
	if m.confirmingStop {
		footer = lipgloss.NewStyle().Foreground(styles.Warning).Bold(true).Render("Stop after current task? (y/n)")
	} else {
		var footerParts []string
		footerParts = append(footerParts, "←/→ tabs")
		if m.activeTab == 1 {
			footerParts = append(footerParts, "↑/↓ scroll · home/end jump")
		}
		if m.stopAfterTask {
			footerParts = append(footerParts, "alt+s resume")
		} else {
			footerParts = append(footerParts, "alt+s stop after task")
		}
		footerParts = append(footerParts, "ctrl+c stop now")
		footer = styles.StatusBar.Render(strings.Join(footerParts, " · "))
	}

	// Use warning border color when stop-after-task is active
	if m.width > 0 && m.height > 0 {
		borderColor := styles.Primary
		if m.stopAfterTask || m.confirmingStop {
			borderColor = styles.Warning
		}
		return styles.FullScreenLeftColor(b.String(), footer, m.width, m.height, borderColor)
	}
	return styles.Box.Render(b.String()) + "\n"
}
