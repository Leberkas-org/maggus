package runner

import (
	"fmt"
	"strconv"
	"strings"
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

// SummaryData holds information displayed on the post-completion summary screen.
type SummaryData struct {
	RunID          string
	Branch         string
	Model          string
	StartTime      time.Time
	TasksCompleted int
	TasksTotal     int
	CommitStart    string // short hash of first commit
	CommitEnd      string // short hash of last commit
	RemainingTasks []RemainingTask
}

// RemainingTask is a task that was not completed during the run.
type RemainingTask struct {
	ID    string
	Title string
}

// SummaryMsg tells the TUI to transition to the summary view.
type SummaryMsg struct {
	Data SummaryData
}

// PushStatusMsg updates the push status on the summary screen.
type PushStatusMsg struct {
	Status string // e.g. "Pushed to origin/branch" or "Push failed: reason"
	Done   bool
}

// QuitMsg tells the TUI to transition to the "done" state (waiting for keypress to exit).
type QuitMsg struct{}

// IterationStartMsg resets per-iteration state when a new iteration begins.
type IterationStartMsg struct {
	Current   int
	Total     int
	TaskID    string
	TaskTitle string
	PlanFile  string
}

// RunAgainResult holds the user's choice from the summary menu.
type RunAgainResult struct {
	RunAgain  bool
	TaskCount int
}

// tickMsg is sent by the spinner ticker.
type tickMsg time.Time

// BannerInfo holds startup information displayed in the TUI's initial view.
type BannerInfo struct {
	Iterations int
	Branch     string
	RunID      string
	RunDir     string
	Worktree   string // empty if not using worktree
	Agent      string // agent name (e.g. "claude", "opencode")
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

	// Summary state
	showSummary    bool
	summary        SummaryData
	summaryElapsed time.Duration // frozen elapsed time at summary display
	pushStatus     string
	pushDone       bool
	menuChoice   int    // 0 = Exit, 1 = Run again
	editingCount bool   // true when typing task count
	countInput   string // buffer for task count input
	runAgain     RunAgainResult

	// Task info
	taskID       string
	taskTitle    string
	taskPlanFile string

	// Recent commits
	commits []string

	// Token usage tracking
	iterInputTokens   int         // current iteration input tokens
	iterOutputTokens  int         // current iteration output tokens
	totalInputTokens  int         // cumulative input tokens
	totalOutputTokens int         // cumulative output tokens
	hasUsageData      bool        // true if any usage data was received
	taskUsages        []TaskUsage // per-task usage history
	onTaskUsage       func(TaskUsage) // called immediately when a task's usage is finalized

	status      string
	toolHistory []string
	toolEntries []agent.ToolMsg // full tool messages for detail panel
	output      string
	extras      string
	model       string
	toolCount   int
	skills      []string
	mcps        []string
	startTime   time.Time
	frame       int
	width       int
	height      int
	activeTab   int    // 0 = Progress, 1 = Commits
	showDetail  bool   // true when right-side detail panel is visible
	cancelFunc  func() // called on Ctrl+C to cancel the context
	quitting    bool
}

// NewTUIModel creates a new TUI model. The cancelFunc is called on Ctrl+C to cancel the work context.
func NewTUIModel(model string, version string, fingerprint string, cancelFunc func(), banner BannerInfo) TUIModel {
	if model == "" {
		model = "default"
	}
	return TUIModel{
		version:     version,
		fingerprint: fingerprint,
		banner:      banner,
		status:      "Waiting...",
		output:      "-",
		model:       model,
		startTime:   time.Now(),
		width:       120,
		height:      40,
		cancelFunc:  cancelFunc,
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
	return m.runAgain
}

// TaskUsages returns the per-task token usage records.
func (m TUIModel) TaskUsages() []TaskUsage {
	return m.taskUsages
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
		if m.done {
			return m.handleSummaryKeys(msg)
		}
		if m.showSummary && msg.Type == tea.KeyCtrlC {
			// Ctrl+C on summary exits immediately
			m.quitting = true
			return m, tea.Quit
		}
		if msg.Type == tea.KeyCtrlC {
			m.status = "Interrupting..."
			if m.cancelFunc != nil {
				m.cancelFunc()
				m.cancelFunc = nil // prevent double-cancel
			}
			return m, nil
		}
		// Alt+I toggles the detail panel
		if msg.Alt && len(msg.Runes) == 1 && (msg.Runes[0] == 'i' || msg.Runes[0] == 'I') {
			m.showDetail = !m.showDetail
			return m, nil
		}
		// Tab switching in work view (only when a task is active)
		if m.taskID != "" {
			switch msg.Type {
			case tea.KeyLeft:
				if m.activeTab > 0 {
					m.activeTab--
				}
				return m, nil
			case tea.KeyRight:
				if m.activeTab < 1 {
					m.activeTab++
				}
				return m, nil
			default:
				if len(msg.Runes) == 1 {
					switch msg.Runes[0] {
					case '1':
						m.activeTab = 0
						return m, nil
					case '2':
						m.activeTab = 1
						return m, nil
					}
				}
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, tea.ClearScreen

	case tickMsg:
		m.frame = (m.frame + 1) % len(spinnerFrames)
		return m, tickCmd()

	case SummaryMsg:
		// Save last iteration's usage before transitioning to summary.
		saveIterationUsage(&m)
		m.showSummary = true
		m.summary = msg.Data
		m.summaryElapsed = time.Since(msg.Data.StartTime).Truncate(time.Second)
		m.pushStatus = "Pushing to remote..."
		return m, nil

	case PushStatusMsg:
		m.pushStatus = msg.Status
		m.pushDone = msg.Done
		return m, nil

	case QuitMsg:
		m.done = true
		return m, nil

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
		// Reset per-iteration state
		m.status = "Starting..."
		m.output = "-"
		m.toolHistory = nil
		m.toolEntries = nil
		m.toolCount = 0
		m.extras = ""
		m.skills = nil
		m.mcps = nil
		m.iterInputTokens = 0
		m.iterOutputTokens = 0
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
		m.toolHistory = append(m.toolHistory, msg.Description)
		if len(m.toolHistory) > maxToolHistory {
			m.toolHistory = m.toolHistory[len(m.toolHistory)-maxToolHistory:]
		}
		m.toolEntries = append(m.toolEntries, msg)
		m.toolCount++

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
	if m.showSummary || m.done {
		return m.renderSummaryView()
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

	footer := styles.StatusBar.Render("alt+i detail · ctrl+c stop")

	if m.width > 0 && m.height > 0 {
		return styles.FullScreenLeft(b.String(), footer, m.width, m.height)
	}
	return styles.Box.Render(b.String()) + "\n"
}

func (m TUIModel) handleSummaryKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.editingCount {
		switch msg.Type {
		case tea.KeyEscape:
			m.editingCount = false
			m.countInput = ""
			return m, nil
		case tea.KeyEnter:
			n, err := strconv.Atoi(m.countInput)
			if err != nil || n <= 0 {
				// Invalid input, reset
				m.countInput = ""
				return m, nil
			}
			m.runAgain = RunAgainResult{RunAgain: true, TaskCount: n}
			m.quitting = true
			return m, tea.Quit
		case tea.KeyBackspace:
			if len(m.countInput) > 0 {
				m.countInput = m.countInput[:len(m.countInput)-1]
			}
			return m, nil
		case tea.KeyCtrlC:
			m.quitting = true
			return m, tea.Quit
		default:
			if len(msg.Runes) == 1 && msg.Runes[0] >= '0' && msg.Runes[0] <= '9' {
				if len(m.countInput) < 4 { // max 9999 tasks
					m.countInput += string(msg.Runes[0])
				}
			}
			return m, nil
		}
	}

	switch msg.Type {
	case tea.KeyEscape:
		m.quitting = true
		return m, tea.Quit
	case tea.KeyCtrlC:
		m.quitting = true
		return m, tea.Quit
	case tea.KeyUp, tea.KeyShiftTab:
		if m.menuChoice > 0 {
			m.menuChoice--
		}
		return m, nil
	case tea.KeyDown, tea.KeyTab:
		if m.menuChoice < 1 {
			m.menuChoice++
		}
		return m, nil
	case tea.KeyEnter:
		if m.menuChoice == 0 {
			// Exit
			m.quitting = true
			return m, tea.Quit
		}
		// Run again — start editing count
		m.editingCount = true
		m.countInput = ""
		return m, nil
	default:
		if len(msg.Runes) == 1 {
			switch msg.Runes[0] {
			case 'q', 'Q':
				m.quitting = true
				return m, tea.Quit
			case 'j':
				if m.menuChoice < 1 {
					m.menuChoice++
				}
			case 'k':
				if m.menuChoice > 0 {
					m.menuChoice--
				}
			}
		}
		return m, nil
	}
}

func (m TUIModel) renderSummaryView() string {
	innerW, _ := styles.FullScreenInnerSize(m.width, m.height)
	if innerW < 40 {
		innerW = 40
	}

	var content strings.Builder

	// Header inside box
	content.WriteString(m.renderHeaderInner(innerW))

	// Title
	elapsed := m.summaryElapsed
	title := styles.Title.Render("✓ Work Complete")
	if m.summary.TasksCompleted == 0 {
		title = styles.Title.Foreground(styles.Warning).Render("⊘ Work Interrupted")
	}
	content.WriteString(title + "\n\n")

	// Key-value pairs
	labelStyle := styles.Label.Width(10).Align(lipgloss.Right)
	valStyle := lipgloss.NewStyle().Foreground(styles.Muted)

	content.WriteString(fmt.Sprintf("%s  %s\n", labelStyle.Render("Run ID:"), valStyle.Render(m.summary.RunID)))
	content.WriteString(fmt.Sprintf("%s  %s\n", labelStyle.Render("Branch:"), valStyle.Render(m.summary.Branch)))
	content.WriteString(fmt.Sprintf("%s  %s\n", labelStyle.Render("Model:"), valStyle.Render(m.summary.Model)))
	content.WriteString(fmt.Sprintf("%s  %s\n", labelStyle.Render("Elapsed:"), valStyle.Render(elapsed.String())))
	content.WriteString(fmt.Sprintf("%s  %s\n",
		labelStyle.Render("Tasks:"),
		lipgloss.NewStyle().Foreground(styles.Success).Render(
			fmt.Sprintf("%d/%d completed", m.summary.TasksCompleted, m.summary.TasksTotal))))

	// Token usage totals
	if m.hasUsageData {
		tokenStr := fmt.Sprintf("%s in / %s out", FormatTokens(m.totalInputTokens), FormatTokens(m.totalOutputTokens))
		content.WriteString(fmt.Sprintf("%s  %s\n", labelStyle.Render("Tokens:"), valStyle.Render(tokenStr)))
	} else {
		content.WriteString(fmt.Sprintf("%s  %s\n", labelStyle.Render("Tokens:"), valStyle.Render("N/A")))
	}

	// Per-task token breakdown
	if len(m.taskUsages) > 0 {
		content.WriteString("\n")
		content.WriteString(styles.Subtitle.Render("Token Usage") + "\n")
		for _, tu := range m.taskUsages {
			content.WriteString(fmt.Sprintf("  %s %s  %s in / %s out\n",
				lipgloss.NewStyle().Foreground(styles.Muted).Render("•"),
				fmt.Sprintf("%-12s", tu.TaskID),
				FormatTokens(tu.InputTokens),
				FormatTokens(tu.OutputTokens)))
		}
	}

	// Commit range and list
	if len(m.commits) > 0 {
		content.WriteString("\n")
		commitHeader := fmt.Sprintf("Commits (%d)", len(m.commits))
		if m.summary.CommitStart != "" && m.summary.CommitEnd != "" {
			commitHeader += fmt.Sprintf("  %s..%s", m.summary.CommitStart, m.summary.CommitEnd)
		}
		content.WriteString(styles.Subtitle.Render(commitHeader) + "\n")
		for _, c := range m.commits {
			content.WriteString(fmt.Sprintf("  %s %s\n",
				lipgloss.NewStyle().Foreground(styles.Muted).Render("•"),
				styles.Truncate(c, innerW-4)))
		}
	}

	// Remaining incomplete tasks
	if len(m.summary.RemainingTasks) > 0 {
		content.WriteString("\n")
		content.WriteString(lipgloss.NewStyle().Foreground(styles.Warning).Render(
			fmt.Sprintf("Remaining (%d)", len(m.summary.RemainingTasks))) + "\n")
		maxShow := 5
		for i, t := range m.summary.RemainingTasks {
			if i >= maxShow {
				content.WriteString(fmt.Sprintf("  %s\n",
					lipgloss.NewStyle().Foreground(styles.Muted).Render(
						fmt.Sprintf("... and %d more", len(m.summary.RemainingTasks)-maxShow))))
				break
			}
			content.WriteString(fmt.Sprintf("  %s %s\n",
				lipgloss.NewStyle().Foreground(styles.Muted).Render("•"),
				fmt.Sprintf("%s: %s", t.ID, styles.Truncate(t.Title, innerW-len(t.ID)-6))))
		}
	}

	// Push status
	content.WriteString("\n")
	if m.pushDone {
		content.WriteString(lipgloss.NewStyle().Foreground(styles.Success).Render(m.pushStatus) + "\n")
	} else if m.pushStatus != "" {
		spinner := cyanStyle.Render(spinnerFrames[m.frame])
		content.WriteString(fmt.Sprintf("%s %s\n", spinner, m.pushStatus))
	}

	// Footer: summary menu or waiting message
	var footer string
	if m.done {
		footer = m.renderSummaryMenu()
	} else {
		footer = lipgloss.NewStyle().Foreground(styles.Muted).Render("Waiting for push to complete...")
	}

	if m.width > 0 && m.height > 0 {
		return styles.FullScreenLeft(content.String(), footer, m.width, m.height)
	}
	return styles.Box.Render(content.String()) + "\n"
}

func (m TUIModel) renderSummaryMenu() string {
	var menu strings.Builder

	selectedStyle := lipgloss.NewStyle().Foreground(styles.Primary).Bold(true)
	normalStyle := lipgloss.NewStyle().Foreground(styles.Muted)
	hintStyle := lipgloss.NewStyle().Foreground(styles.Muted).Faint(true)

	// Exit option
	if m.menuChoice == 0 {
		menu.WriteString(fmt.Sprintf("  %s %s\n", selectedStyle.Render("▸"), selectedStyle.Render("Exit")))
	} else {
		menu.WriteString(fmt.Sprintf("  %s %s\n", normalStyle.Render(" "), normalStyle.Render("Exit")))
	}

	// Run again option
	if m.menuChoice == 1 {
		if m.editingCount {
			cursor := "█"
			countDisplay := m.countInput + cursor
			menu.WriteString(fmt.Sprintf("  %s %s %s %s\n",
				selectedStyle.Render("▸"),
				selectedStyle.Render("Run again:"),
				selectedStyle.Render(countDisplay),
				hintStyle.Render("tasks (enter to confirm, esc to cancel)")))
		} else {
			menu.WriteString(fmt.Sprintf("  %s %s\n",
				selectedStyle.Render("▸"),
				selectedStyle.Render("Run again")))
		}
	} else {
		menu.WriteString(fmt.Sprintf("  %s %s\n", normalStyle.Render(" "), normalStyle.Render("Run again")))
	}

	menu.WriteString(fmt.Sprintf("\n  %s\n", hintStyle.Render("↑/↓ select · enter confirm · q/esc exit")))

	return menu.String()
}

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

	// Line 2: progress bar
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

	// Build tab labels
	progressLabel := " Progress "
	commitsLabel := " Commits "
	if len(m.commits) > 0 {
		commitsLabel = fmt.Sprintf(" Commits (%d) ", len(m.commits))
	}

	var tabs [2]string
	if m.activeTab == 0 {
		tabs[0] = selectedStyle.Render(progressLabel)
		tabs[1] = unselectedStyle.Render(commitsLabel)
	} else {
		tabs[0] = unselectedStyle.Render(progressLabel)
		tabs[1] = selectedStyle.Render(commitsLabel)
	}

	return tabs[0] + sep + tabs[1] + "\n" + styles.Separator(w) + "\n"
}

// toolIcon returns a short icon for a tool type.
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
		return "▶"
	}
}

// renderDetailPanel renders the right-side tool detail panel content.
func (m TUIModel) renderDetailPanel(w, h int) string {
	if w < 10 {
		w = 10
	}

	var b strings.Builder
	b.WriteString(boldStyle.Render("Tool Detail") + "\n")
	b.WriteString(styles.Separator(w) + "\n")

	if len(m.toolEntries) == 0 {
		b.WriteString(grayStyle.Render("No tool invocations yet.") + "\n")
	} else {
		// Render entries, auto-scrolled to show latest entries that fit
		var entryLines []string
		for i, entry := range m.toolEntries {
			if i > 0 {
				entryLines = append(entryLines, grayStyle.Render(styles.Truncate(strings.Repeat("·", w), w)))
			}

			// Header: icon + description + timestamp
			icon := toolIcon(entry.Type)
			ts := entry.Timestamp.Format("15:04:05")
			desc := entry.Description
			// Available width for description = w - icon - timestamp - spaces
			maxDesc := w - len(ts) - 4
			if maxDesc > 0 {
				desc = styles.Truncate(desc, maxDesc)
			}
			header := fmt.Sprintf("%s %s  %s", icon, blueStyle.Render(desc), grayStyle.Render(ts))
			entryLines = append(entryLines, header)

			// Parameter detail lines
			for k, v := range entry.Params {
				paramLine := fmt.Sprintf("  %s %s", grayStyle.Render(k+":"), styles.Truncate(v, w-len(k)-4))
				entryLines = append(entryLines, paramLine)
			}
		}

		// Auto-scroll: show the last lines that fit in the available height
		available := h - 3 // header + separator + margin
		if available < 1 {
			available = 1
		}
		start := 0
		if len(entryLines) > available {
			start = len(entryLines) - available
		}
		for _, line := range entryLines[start:] {
			b.WriteString(line + "\n")
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

	// Calculate panel widths
	leftW := innerW
	var rightW int
	if m.showDetail && innerW > 60 {
		leftW = innerW * 40 / 100
		rightW = innerW - leftW - 1 // -1 for divider
		if leftW < 30 {
			leftW = 30
			rightW = innerW - leftW - 1
		}
	}

	contentWidth := leftW - 11
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

	// Tab bar (left panel width when split, otherwise full)
	tabBarW := leftW
	if !m.showDetail || innerW <= 60 {
		tabBarW = innerW
	}

	// Build left panel content (tab bar + tab content)
	var left strings.Builder
	left.WriteString(m.renderTabBar(tabBarW))

	if m.activeTab == 0 {
		left.WriteString(fmt.Sprintf("%s %s  %s\n", spinner, boldStyle.Render("Status:"), sColor.Render(m.status)))
		left.WriteString(fmt.Sprintf("  %s  %s\n", boldStyle.Render("Output:"), styles.Truncate(m.output, contentWidth)))

		left.WriteString(fmt.Sprintf("  %s   %s\n", boldStyle.Render("Tools:"), grayStyle.Render(fmt.Sprintf("(%d total)", m.toolCount))))
		for i, t := range m.toolHistory {
			prefix := grayStyle.Render("│")
			if i == len(m.toolHistory)-1 {
				prefix = blueStyle.Render("▶")
			}
			left.WriteString(fmt.Sprintf("  %s %s\n", prefix, blueStyle.Render(styles.Truncate(t, contentWidth))))
		}
		for i := len(m.toolHistory); i < maxToolHistory; i++ {
			left.WriteString("\n")
		}

		left.WriteString(fmt.Sprintf("  %s  %s\n", boldStyle.Render("Extras:"), cyanStyle.Render(styles.Truncate(extrasStr, contentWidth))))
		left.WriteString(fmt.Sprintf("  %s   %s\n", boldStyle.Render("Model:"), grayStyle.Render(m.model)))
		left.WriteString(fmt.Sprintf("  %s %s\n", boldStyle.Render("Elapsed:"), grayStyle.Render(elapsed.String())))

		if m.hasUsageData {
			tokenStr := fmt.Sprintf("%s in / %s out", FormatTokens(m.totalInputTokens), FormatTokens(m.totalOutputTokens))
			left.WriteString(fmt.Sprintf("  %s  %s\n", boldStyle.Render("Tokens:"), grayStyle.Render(tokenStr)))
		} else {
			left.WriteString(fmt.Sprintf("  %s  %s\n", boldStyle.Render("Tokens:"), grayStyle.Render("N/A")))
		}
	} else {
		if len(m.commits) == 0 {
			left.WriteString(grayStyle.Render("No commits yet.") + "\n")
		} else {
			for _, c := range m.commits {
				line := styles.Truncate(c, tabBarW-4)
				left.WriteString(fmt.Sprintf("  %s %s\n",
					grayStyle.Render("•"),
					grayStyle.Render(line)))
			}
		}
	}

	if m.showDetail && innerW > 60 {
		// Split layout: left + divider + right
		leftContent := left.String()
		rightContent := m.renderDetailPanel(rightW, innerH-6) // reserve lines for header/task/footer

		// Build the divider column
		leftLines := strings.Split(strings.TrimRight(leftContent, "\n"), "\n")
		rightLines := strings.Split(strings.TrimRight(rightContent, "\n"), "\n")

		// Ensure both have the same number of lines
		maxLines := len(leftLines)
		if len(rightLines) > maxLines {
			maxLines = len(rightLines)
		}
		for len(leftLines) < maxLines {
			leftLines = append(leftLines, "")
		}
		for len(rightLines) < maxLines {
			rightLines = append(rightLines, "")
		}

		divider := grayStyle.Render("│")
		for i := 0; i < maxLines; i++ {
			// Pad left line to leftW
			leftLine := leftLines[i]
			leftVisible := lipgloss.Width(leftLine)
			if leftVisible < leftW {
				leftLine += strings.Repeat(" ", leftW-leftVisible)
			}
			b.WriteString(leftLine + divider + rightLines[i] + "\n")
		}
	} else {
		b.WriteString(left.String())
	}

	// Footer with context-sensitive keybindings
	var footerParts []string
	footerParts = append(footerParts, "1/2 tabs")
	if m.showDetail {
		footerParts = append(footerParts, "alt+i hide detail")
	} else {
		footerParts = append(footerParts, "alt+i detail")
	}
	footerParts = append(footerParts, "ctrl+c stop")
	footer := styles.StatusBar.Render(strings.Join(footerParts, " · "))

	if m.width > 0 && m.height > 0 {
		return styles.FullScreenLeft(b.String(), footer, m.width, m.height)
	}
	return styles.Box.Render(b.String()) + "\n"
}
