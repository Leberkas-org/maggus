package runner

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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

// InfoMsg displays an informational message in the TUI.
type InfoMsg struct {
	Text string
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

	// Task info
	taskID    string
	taskTitle string

	// Recent commits
	commits []string

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

	case tickMsg:
		m.frame = (m.frame + 1) % len(spinnerFrames)
		return m, tickCmd()

	case QuitMsg:
		m.done = true
		return m, nil

	case InfoMsg:
		m.infoMessages = append(m.infoMessages, msg.Text)

	case IterationStartMsg:
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

// Styles
var (
	boldStyle   = lipgloss.NewStyle().Bold(true)
	statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("3")) // yellow
	greenStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	redStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	cyanStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	blueStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("4"))
	grayStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
)

func (m tuiModel) View() string {
	if m.done {
		return m.renderDoneView()
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

func (m tuiModel) renderDoneView() string {
	var b strings.Builder
	b.WriteString(m.renderHeader())
	b.WriteString("\n")

	// Show commits
	if len(m.commits) > 0 {
		b.WriteString(grayStyle.Render("  Commits:") + "\n")
		for _, c := range m.commits {
			b.WriteString(fmt.Sprintf("    %s\n", grayStyle.Render(truncate(c, m.width-6))))
		}
		b.WriteString("\n")
	}

	// Show info messages (push status, etc.)
	for _, msg := range m.infoMessages {
		b.WriteString(fmt.Sprintf("  %s\n", msg))
	}
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  %s\n", grayStyle.Render("Press any key to exit")))
	return b.String()
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
		filled := 0
		if m.totalIters > 0 {
			filled = (m.currentIter * barWidth) / m.totalIters
		}
		if filled > barWidth {
			filled = barWidth
		}
		bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)
		progress := fmt.Sprintf("  [%s] %d/%d Tasks", bar, m.currentIter, m.totalIters)
		b.WriteString(greenStyle.Render(progress) + "\n")
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
	b.WriteString(fmt.Sprintf("    %s  %s\n", boldStyle.Render("Output:"), truncate(m.output, contentWidth)))

	b.WriteString(fmt.Sprintf("    %s   %s\n", boldStyle.Render("Tools:"), grayStyle.Render(fmt.Sprintf("(%d total)", m.toolCount))))
	for i, t := range m.toolHistory {
		prefix := grayStyle.Render("│")
		if i == len(m.toolHistory)-1 {
			prefix = blueStyle.Render("▶")
		}
		b.WriteString(fmt.Sprintf("    %s %s\n", prefix, blueStyle.Render(truncate(t, contentWidth))))
	}
	// Pad empty lines for consistent layout
	for i := len(m.toolHistory); i < maxToolHistory; i++ {
		b.WriteString("\n")
	}

	b.WriteString(fmt.Sprintf("    %s  %s\n", boldStyle.Render("Extras:"), cyanStyle.Render(truncate(extrasStr, contentWidth))))
	b.WriteString(fmt.Sprintf("    %s   %s\n", boldStyle.Render("Model:"), grayStyle.Render(m.model)))
	b.WriteString(fmt.Sprintf("    %s %s\n", boldStyle.Render("Elapsed:"), grayStyle.Render(elapsed.String())))

	// Recent commits section
	if len(m.commits) > 0 {
		b.WriteString("\n")
		b.WriteString(grayStyle.Render(strings.Repeat("─", w)) + "\n")
		b.WriteString(grayStyle.Render("  Commits:") + "\n")
		for _, c := range m.commits {
			line := truncate(c, w-6)
			b.WriteString(fmt.Sprintf("    %s\n", grayStyle.Render(line)))
		}
	}

	return b.String()
}
