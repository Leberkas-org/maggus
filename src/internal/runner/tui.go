package runner

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/leberkas-org/maggus/internal/tui/styles"
)

// Message types for the bubbletea model.

// ProgressMsg is sent when iteration progress changes.
type ProgressMsg struct {
	Current int
	Total   int
}

// ToolMsg is sent when a new tool use is detected.
type ToolMsg struct {
	Description string
}

// OutputMsg is sent when new assistant text output arrives.
type OutputMsg struct {
	Text string
}

// StatusMsg is sent when the status changes (e.g. "Thinking...", "Running tool", "Done").
type StatusMsg struct {
	Status string
}

// SkillMsg is sent when a skill is used.
type SkillMsg struct {
	Name string
}

// MCPMsg is sent when an MCP tool is used.
type MCPMsg struct {
	Name string
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

// UsageMsg is sent when a result event contains token usage data.
type UsageMsg struct {
	InputTokens  int
	OutputTokens int
}

// TaskUsage records token usage for a single task/iteration.
type TaskUsage struct {
	TaskID       string
	TaskTitle    string
	InputTokens  int
	OutputTokens int
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

// tuiModel is the bubbletea model that replaces the old display struct.
type tuiModel struct {
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
	showSummary bool
	summary     SummaryData
	pushStatus  string
	pushDone    bool

	// Task info
	taskID    string
	taskTitle string

	// Recent commits
	commits []string

	// Token usage tracking
	iterInputTokens   int         // current iteration input tokens
	iterOutputTokens  int         // current iteration output tokens
	totalInputTokens  int         // cumulative input tokens
	totalOutputTokens int         // cumulative output tokens
	hasUsageData      bool        // true if any usage data was received
	taskUsages        []TaskUsage // per-task usage history

	status      string
	toolHistory []string
	output      string
	extras      string
	model       string
	toolCount   int
	skills      []string
	mcps        []string
	startTime   time.Time
	frame       int
	width       int
	cancelFunc  func() // called on Ctrl+C to cancel the context
	quitting    bool
}

// NewTUIModel creates a new TUI model. The cancelFunc is called on Ctrl+C to cancel the work context.
func NewTUIModel(model string, version string, fingerprint string, cancelFunc func(), banner BannerInfo) tuiModel {
	if model == "" {
		model = "default"
	}
	return tuiModel{
		version:     version,
		fingerprint: fingerprint,
		banner:      banner,
		status:      "Waiting...",
		output:      "-",
		model:       model,
		startTime:   time.Now(),
		width:       120,
		cancelFunc:  cancelFunc,
	}
}

func (m tuiModel) Init() tea.Cmd {
	return tickCmd()
}

func tickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.done {
			// Any key exits when done
			m.quitting = true
			return m, tea.Quit
		}
		if m.showSummary && msg.Type == tea.KeyCtrlC {
			// Ctrl+C on summary exits immediately
			m.quitting = true
			return m, tea.Quit
		}
		if msg.Type == tea.KeyCtrlC {
			m.status = "Interrupted"
			m.quitting = true
			if m.cancelFunc != nil {
				m.cancelFunc()
			}
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		return m, tea.ClearScreen

	case tickMsg:
		m.frame = (m.frame + 1) % len(spinnerFrames)
		return m, tickCmd()

	case SummaryMsg:
		// Save last iteration's usage before transitioning to summary.
		if m.taskID != "" && (m.iterInputTokens > 0 || m.iterOutputTokens > 0) {
			m.taskUsages = append(m.taskUsages, TaskUsage{
				TaskID:       m.taskID,
				TaskTitle:    m.taskTitle,
				InputTokens:  m.iterInputTokens,
				OutputTokens: m.iterOutputTokens,
			})
		}
		m.showSummary = true
		m.summary = msg.Data
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

	case UsageMsg:
		m.iterInputTokens += msg.InputTokens
		m.iterOutputTokens += msg.OutputTokens
		m.totalInputTokens += msg.InputTokens
		m.totalOutputTokens += msg.OutputTokens
		if msg.InputTokens > 0 || msg.OutputTokens > 0 {
			m.hasUsageData = true
		}

	case IterationStartMsg:
		// Save previous iteration's usage before resetting.
		if m.taskID != "" && (m.iterInputTokens > 0 || m.iterOutputTokens > 0) {
			m.taskUsages = append(m.taskUsages, TaskUsage{
				TaskID:       m.taskID,
				TaskTitle:    m.taskTitle,
				InputTokens:  m.iterInputTokens,
				OutputTokens: m.iterOutputTokens,
			})
		}
		m.currentIter = msg.Current
		m.totalIters = msg.Total
		m.taskID = msg.TaskID
		m.taskTitle = msg.TaskTitle
		// Reset per-iteration state
		m.status = "Starting..."
		m.output = "-"
		m.toolHistory = nil
		m.toolCount = 0
		m.extras = ""
		m.skills = nil
		m.mcps = nil
		m.iterInputTokens = 0
		m.iterOutputTokens = 0
		m.startTime = time.Now()

	case StatusMsg:
		m.status = msg.Status

	case OutputMsg:
		text := strings.TrimSpace(msg.Text)
		if idx := strings.LastIndex(text, "\n"); idx >= 0 {
			text = strings.TrimSpace(text[idx+1:])
		}
		if text != "" {
			m.output = text
		}

	case ToolMsg:
		m.toolHistory = append(m.toolHistory, msg.Description)
		if len(m.toolHistory) > maxToolHistory {
			m.toolHistory = m.toolHistory[len(m.toolHistory)-maxToolHistory:]
		}
		m.toolCount++

	case SkillMsg:
		for _, s := range m.skills {
			if s == msg.Name {
				return m, nil
			}
		}
		m.skills = append(m.skills, msg.Name)
		m.rebuildExtras()

	case MCPMsg:
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

func (m *tuiModel) rebuildExtras() {
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

func (m tuiModel) View() string {
	if m.showSummary || m.done {
		return m.renderSummaryView()
	}
	if m.taskID == "" {
		return m.renderBannerView()
	}
	return m.renderView()
}

func (m tuiModel) renderBannerView() string {
	var b strings.Builder
	b.WriteString(m.renderHeader())
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  %s  %s\n", boldStyle.Render("Model:"), m.model))
	b.WriteString(fmt.Sprintf("  %s  %d\n", boldStyle.Render("Tasks:"), m.banner.Iterations))
	if m.banner.Branch != "" {
		b.WriteString(fmt.Sprintf("  %s %s\n", boldStyle.Render("Branch:"), m.banner.Branch))
	}
	b.WriteString(fmt.Sprintf("  %s %s\n", boldStyle.Render("Run ID:"), m.banner.RunID))
	if m.banner.Worktree != "" {
		b.WriteString(fmt.Sprintf("  %s  %s\n", boldStyle.Render("Worktree:"), m.banner.Worktree))
	}
	b.WriteString("\n")
	for _, msg := range m.infoMessages {
		b.WriteString(fmt.Sprintf("  %s\n", msg))
	}
	if len(m.infoMessages) == 0 {
		b.WriteString(fmt.Sprintf("  %s\n", grayStyle.Render("Starting...")))
	}
	return b.String()
}

func (m tuiModel) renderSummaryView() string {
	w := m.width
	if w < 50 {
		w = 50
	}

	// Build the inner content of the summary box.
	boxWidth := w - 4 // account for box border + padding
	if boxWidth < 40 {
		boxWidth = 40
	}

	var content strings.Builder

	// Title
	elapsed := time.Since(m.summary.StartTime).Truncate(time.Second)
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
				styles.Truncate(c, boxWidth-4)))
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
				fmt.Sprintf("%s: %s", t.ID, styles.Truncate(t.Title, boxWidth-len(t.ID)-6))))
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

	// Render inside a lipgloss box
	box := styles.Box.Width(boxWidth)
	var out strings.Builder
	out.WriteString(m.renderHeader())
	out.WriteString("\n")
	out.WriteString(box.Render(content.String()))
	out.WriteString("\n\n")
	out.WriteString(fmt.Sprintf("  %s\n", lipgloss.NewStyle().Foreground(styles.Muted).Render("Press any key to exit")))
	return out.String()
}

func (m tuiModel) renderHeader() string {
	var b strings.Builder
	w := m.width
	if w < 40 {
		w = 40
	}

	// Line 1: version left, fingerprint right
	left := boldStyle.Render(fmt.Sprintf("Maggus v%s", m.version))
	right := ""
	if m.fingerprint != "" {
		right = grayStyle.Render(m.fingerprint)
	}
	// Pad between left and right to fill width
	// Use raw lengths for spacing calculation (lipgloss adds ANSI escapes)
	leftRaw := fmt.Sprintf("  Maggus v%s", m.version)
	rightRaw := m.fingerprint
	padding := w - len(leftRaw) - len(rightRaw) - 2
	if padding < 2 {
		padding = 2
	}
	b.WriteString(fmt.Sprintf("  %s%s%s\n", left, strings.Repeat(" ", padding), right))

	// Line 2: progress bar
	if m.totalIters > 0 {
		barWidth := 20
		bar := styles.ProgressBar(m.currentIter, m.totalIters, barWidth)
		progress := fmt.Sprintf("  [%s] %s", bar,
			greenStyle.Render(fmt.Sprintf("%d/%d Tasks", m.currentIter, m.totalIters)))
		b.WriteString(progress + "\n")
	}

	// Separator line
	sep := strings.Repeat("─", w)
	b.WriteString(grayStyle.Render(sep) + "\n")

	return b.String()
}

func (m tuiModel) renderView() string {
	elapsed := time.Since(m.startTime).Truncate(time.Second)
	w := m.width
	contentWidth := w - 13
	if contentWidth < 20 {
		contentWidth = 20
	}

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

	var b strings.Builder

	// Render header
	b.WriteString(m.renderHeader())

	// Render task info
	if m.taskID != "" {
		taskLine := fmt.Sprintf("  %s %s", cyanStyle.Render(m.taskID+":"), m.taskTitle)
		b.WriteString(taskLine + "\n\n")
	}

	b.WriteString(fmt.Sprintf("  %s %s  %s\n", spinner, boldStyle.Render("Status:"), sColor.Render(m.status)))
	b.WriteString(fmt.Sprintf("    %s  %s\n", boldStyle.Render("Output:"), styles.Truncate(m.output, contentWidth)))

	b.WriteString(fmt.Sprintf("    %s   %s\n", boldStyle.Render("Tools:"), grayStyle.Render(fmt.Sprintf("(%d total)", m.toolCount))))
	for i, t := range m.toolHistory {
		prefix := grayStyle.Render("│")
		if i == len(m.toolHistory)-1 {
			prefix = blueStyle.Render("▶")
		}
		b.WriteString(fmt.Sprintf("    %s %s\n", prefix, blueStyle.Render(styles.Truncate(t, contentWidth))))
	}
	// Pad empty lines for consistent layout
	for i := len(m.toolHistory); i < maxToolHistory; i++ {
		b.WriteString("\n")
	}

	b.WriteString(fmt.Sprintf("    %s  %s\n", boldStyle.Render("Extras:"), cyanStyle.Render(styles.Truncate(extrasStr, contentWidth))))
	b.WriteString(fmt.Sprintf("    %s   %s\n", boldStyle.Render("Model:"), grayStyle.Render(m.model)))
	b.WriteString(fmt.Sprintf("    %s %s\n", boldStyle.Render("Elapsed:"), grayStyle.Render(elapsed.String())))

	// Token usage
	if m.hasUsageData {
		tokenStr := fmt.Sprintf("%s in / %s out", FormatTokens(m.totalInputTokens), FormatTokens(m.totalOutputTokens))
		b.WriteString(fmt.Sprintf("    %s  %s\n", boldStyle.Render("Tokens:"), grayStyle.Render(tokenStr)))
	} else {
		b.WriteString(fmt.Sprintf("    %s  %s\n", boldStyle.Render("Tokens:"), grayStyle.Render("N/A")))
	}

	// Recent commits section
	if len(m.commits) > 0 {
		b.WriteString("\n")
		b.WriteString(grayStyle.Render(strings.Repeat("─", w)) + "\n")
		b.WriteString(grayStyle.Render("  Commits:") + "\n")
		for _, c := range m.commits {
			line := styles.Truncate(c, w-6)
			b.WriteString(fmt.Sprintf("    %s\n", grayStyle.Render(line)))
		}
	}

	return b.String()
}
