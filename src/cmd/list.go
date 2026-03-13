package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/leberkas-org/maggus/internal/config"
	"github.com/leberkas-org/maggus/internal/parser"
	"github.com/leberkas-org/maggus/internal/tui/styles"
	"github.com/spf13/cobra"
)

// listModel is the bubbletea model for the interactive task list.
type listModel struct {
	tasks     []parser.Task
	agentName string
	all       bool
	cursor    int
	width     int
	height    int

	// Detail view state
	showDetail     bool
	detailViewport viewport.Model
	detailReady    bool

	// Run action
	runTask bool // true if user pressed Alt+R to run the selected task
}

func (m listModel) Init() tea.Cmd {
	return nil
}

func (m listModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.showDetail {
			m.detailViewport.Width = msg.Width
			m.detailViewport.Height = msg.Height - 2 // header + footer
			m.detailReady = true
		}
		return m, nil

	case tea.KeyMsg:
		if m.showDetail {
			return m.updateDetail(msg)
		}
		return m.updateList(msg)
	}

	if m.showDetail && m.detailReady {
		var cmd tea.Cmd
		m.detailViewport, cmd = m.detailViewport.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m listModel) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "alt+r":
		m.runTask = true
		return m, tea.Quit
	case "q", "esc", "ctrl+c":
		return m, tea.Quit
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		} else {
			m.cursor = len(m.tasks) - 1
		}
	case "down", "j":
		if m.cursor < len(m.tasks)-1 {
			m.cursor++
		} else {
			m.cursor = 0
		}
	case "home":
		m.cursor = 0
	case "end":
		m.cursor = len(m.tasks) - 1
	case "enter":
		m.showDetail = true
		content := m.renderDetailContent(m.tasks[m.cursor])
		m.detailViewport = viewport.New(m.width, m.height-2)
		m.detailViewport.SetContent(content)
		m.detailReady = true
		return m, nil
	}
	return m, nil
}

func (m listModel) updateDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "alt+r":
		m.runTask = true
		return m, tea.Quit
	case "q":
		return m, tea.Quit
	case "esc", "backspace":
		m.showDetail = false
		m.detailReady = false
		return m, nil
	case "ctrl+c":
		return m, tea.Quit
	case "home":
		if m.detailReady {
			m.detailViewport.GotoTop()
			return m, nil
		}
	case "end":
		if m.detailReady {
			m.detailViewport.GotoBottom()
			return m, nil
		}
	}
	if m.detailReady {
		var cmd tea.Cmd
		m.detailViewport, cmd = m.detailViewport.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m listModel) View() string {
	if m.showDetail {
		return m.viewDetail()
	}
	return m.viewList()
}

func (m listModel) viewList() string {
	selectedStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.Primary)
	cursorStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.Primary)
	mutedStyle := lipgloss.NewStyle().Faint(true)
	normalStyle := lipgloss.NewStyle()

	var sb strings.Builder

	// Header
	var header string
	if m.all {
		header = styles.Title.Render(fmt.Sprintf("All upcoming tasks (%d)", len(m.tasks)))
	} else {
		header = styles.Title.Render(fmt.Sprintf("Next %d task(s)", len(m.tasks)))
	}
	sb.WriteString(header)
	sb.WriteString("\n")
	sb.WriteString(mutedStyle.Render(fmt.Sprintf(" Agent: %s", m.agentName)))
	sb.WriteString("\n")
	sb.WriteString(" " + styles.Separator(42))
	sb.WriteString("\n")

	for i, t := range m.tasks {
		planFile := mutedStyle.Render(filepath.Base(t.SourceFile))
		label := fmt.Sprintf("#%-2d %s: %s", i+1, t.ID, t.Title)

		if i == m.cursor {
			fmt.Fprintf(&sb, " %s %s  %s\n",
				cursorStyle.Render("→"),
				selectedStyle.Render(label),
				planFile)
		} else {
			fmt.Fprintf(&sb, "   %s  %s\n",
				normalStyle.Render(label),
				planFile)
		}
	}

	content := styles.Box.Render(sb.String())

	footer := styles.StatusBar.Render("↑/↓: navigate · enter: details · alt+r: run task · q/esc: exit")
	return content + "\n" + footer
}

func (m listModel) renderDetailContent(t parser.Task) string {
	var sb strings.Builder

	titleStyle := styles.Title
	labelStyle := styles.Label.Width(10).Align(lipgloss.Right)
	mutedStyle := lipgloss.NewStyle().Foreground(styles.Muted)
	successStyle := lipgloss.NewStyle().Foreground(styles.Success)
	warningStyle := lipgloss.NewStyle().Foreground(styles.Warning)

	sb.WriteString(titleStyle.Render(fmt.Sprintf("%s: %s", t.ID, t.Title)))
	sb.WriteString("\n\n")

	// Metadata
	sb.WriteString(fmt.Sprintf("%s  %s\n", labelStyle.Render("Plan:"), mutedStyle.Render(filepath.Base(t.SourceFile))))

	// Criteria counts
	done := 0
	blocked := 0
	for _, c := range t.Criteria {
		if c.Checked {
			done++
		}
		if c.Blocked {
			blocked++
		}
	}
	sb.WriteString(fmt.Sprintf("%s  %s\n", labelStyle.Render("Criteria:"),
		mutedStyle.Render(fmt.Sprintf("%d total, %d done, %d blocked", len(t.Criteria), done, blocked))))

	// Description
	if t.Description != "" {
		sb.WriteString("\n")
		sb.WriteString(styles.Subtitle.Render("Description"))
		sb.WriteString("\n")
		// Wrap description lines
		for _, line := range strings.Split(strings.TrimSpace(t.Description), "\n") {
			sb.WriteString("  " + line + "\n")
		}
	}

	// Acceptance criteria
	if len(t.Criteria) > 0 {
		sb.WriteString("\n")
		sb.WriteString(styles.Subtitle.Render("Acceptance Criteria"))
		sb.WriteString("\n")
		for _, c := range t.Criteria {
			var checkbox string
			if c.Checked {
				checkbox = successStyle.Render("✓")
			} else if c.Blocked {
				checkbox = warningStyle.Render("⊘")
			} else {
				checkbox = mutedStyle.Render("○")
			}
			sb.WriteString(fmt.Sprintf("  %s %s\n", checkbox, c.Text))
		}
	}

	return styles.Box.Render(sb.String())
}

func (m listModel) viewDetail() string {
	if !m.detailReady {
		return ""
	}

	footer := styles.StatusBar.Render("↑/↓: scroll · alt+r: run task · esc/backspace: back · q: exit")
	if m.detailViewport.TotalLineCount() <= m.detailViewport.Height {
		footer = styles.StatusBar.Render("alt+r: run task · esc/backspace: back · q: exit")
	}
	return m.detailViewport.View() + "\n" + footer
}

// renderListPlain builds the plain-text list output (no ANSI, no TUI).
func renderListPlain(workable []parser.Task, all bool, agentName string) string {
	var sb strings.Builder

	if all {
		fmt.Fprintln(&sb, "All upcoming tasks:")
	} else {
		fmt.Fprintf(&sb, "Next %d task(s):\n", len(workable))
	}
	fmt.Fprintf(&sb, "Agent: %s\n", agentName)
	fmt.Fprintln(&sb)

	for i, t := range workable {
		planFile := filepath.Base(t.SourceFile)
		if i == 0 {
			fmt.Fprintf(&sb, " -> #%-2d %s: %s  %s\n", i+1, t.ID, t.Title, planFile)
		} else {
			fmt.Fprintf(&sb, "    #%-2d %s: %s  %s\n", i+1, t.ID, t.Title, planFile)
		}
	}

	return sb.String()
}

var listCmd = &cobra.Command{
	Use:   "list [N]",
	Short: "Preview the next N upcoming workable tasks",
	Long:  `Reads all plan files in .maggus/ and lists the next N workable (incomplete, not blocked) tasks.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		plain, err := cmd.Flags().GetBool("plain")
		if err != nil {
			return err
		}

		all, err := cmd.Flags().GetBool("all")
		if err != nil {
			return err
		}

		count, err := cmd.Flags().GetInt("count")
		if err != nil {
			return err
		}

		// Positional arg overrides --count (ignored when --all is set)
		if !all && len(args) == 1 {
			n, err := strconv.Atoi(args[0])
			if err != nil || n < 1 {
				return fmt.Errorf("invalid count %q: must be a positive integer", args[0])
			}
			count = n
		}

		dir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}

		return runList(cmd, dir, plain, all, count)
	},
}

func runList(cmd *cobra.Command, dir string, plain, all bool, count int) error {
	cfg, err := config.Load(dir)
	if err != nil {
		return err
	}
	agentName := cfg.Agent

	files, err := parser.GlobPlanFiles(dir, false)
	if err != nil {
		return fmt.Errorf("glob plans: %w", err)
	}

	// Collect workable tasks in order
	var workable []parser.Task
	for _, f := range files {
		tasks, err := parser.ParseFile(f)
		if err != nil {
			return fmt.Errorf("parse %s: %w", f, err)
		}
		for _, t := range tasks {
			if t.IsWorkable() {
				workable = append(workable, t)
			}
		}
	}

	if len(workable) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No pending tasks found. All done!")
		return nil
	}

	// Cap to count unless --all is set
	if !all && count < len(workable) {
		workable = workable[:count]
	}

	if plain {
		fmt.Fprint(cmd.OutOrStdout(), renderListPlain(workable, all, agentName))
		return nil
	}

	// TUI mode: interactive list with detail view
	m := listModel{tasks: workable, agentName: agentName, all: all}
	prog := tea.NewProgram(m, tea.WithAltScreen())
	result, err := prog.Run()
	if err != nil {
		return err
	}
	if final, ok := result.(listModel); ok && final.runTask {
		return dispatchWork()
	}
	return nil
}

func init() {
	rootCmd.AddCommand(listCmd)
	listCmd.Flags().IntP("count", "c", 5, "Number of tasks to show")
	listCmd.Flags().Bool("plain", false, "Strip colors and use ASCII characters for scripting/piping")
	listCmd.Flags().Bool("all", false, "Show all upcoming workable tasks with no count cap")
}
