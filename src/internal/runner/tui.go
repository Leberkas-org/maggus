package runner

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Message types for the bubbletea model.

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

// tickMsg is sent by the spinner ticker.
type tickMsg time.Time

// tuiModel is the bubbletea model that replaces the old display struct.
type tuiModel struct {
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

func newTUIModel(model string, cancelFunc func()) tuiModel {
	if model == "" {
		model = "default"
	}
	return tuiModel{
		status:     "Starting...",
		output:     "-",
		model:      model,
		startTime:  time.Now(),
		width:      120,
		cancelFunc: cancelFunc,
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

	case StatusMsg:
		m.status = msg.Status
		if msg.Status == "Done" || msg.Status == "Failed" {
			m.quitting = true
			return m, tea.Quit
		}

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
	if m.quitting && (m.status == "Done" || m.status == "Failed" || m.status == "Interrupted") {
		// Final view before exit — show status one last time
		return m.renderView()
	}
	return m.renderView()
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

	return b.String()
}
