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
	runTaskID string // task ID to run when user presses Alt+R

	// Delete confirmation
	confirmDelete bool
	deleteErr     string
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
			w, h := styles.FullScreenInnerSize(msg.Width, msg.Height)
			m.detailViewport.Width = w
			m.detailViewport.Height = h - 1 // footer line
			m.detailReady = true
		}
		return m, nil

	case tea.KeyMsg:
		if m.confirmDelete {
			return m.updateConfirmDelete(msg)
		}
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
		m.runTaskID = m.tasks[m.cursor].ID
		return m, tea.Quit
	case "alt+backspace":
		m.confirmDelete = true
		m.deleteErr = ""
		return m, nil
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
		w, h := styles.FullScreenInnerSize(m.width, m.height)
		m.detailViewport = viewport.New(w, h-1)
		m.detailViewport.SetContent(content)
		m.detailReady = true
		return m, nil
	}
	return m, nil
}

func (m listModel) updateDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "alt+r":
		m.runTaskID = m.tasks[m.cursor].ID
		return m, tea.Quit
	case "alt+backspace":
		m.confirmDelete = true
		m.deleteErr = ""
		return m, nil
	case "q":
		return m, tea.Quit
	case "esc", "backspace":
		m.showDetail = false
		m.detailReady = false
		return m, nil
	case "ctrl+c":
		return m, tea.Quit
	case "pgdown":
		if m.cursor < len(m.tasks)-1 {
			m.cursor++
			content := m.renderDetailContent(m.tasks[m.cursor])
			m.detailViewport.SetContent(content)
			m.detailViewport.GotoTop()
		}
		return m, nil
	case "pgup":
		if m.cursor > 0 {
			m.cursor--
			content := m.renderDetailContent(m.tasks[m.cursor])
			m.detailViewport.SetContent(content)
			m.detailViewport.GotoTop()
		}
		return m, nil
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

func (m listModel) updateConfirmDelete(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y", "enter":
		t := m.tasks[m.cursor]
		if err := parser.DeleteTask(t.SourceFile, t.ID); err != nil {
			m.deleteErr = err.Error()
			m.confirmDelete = false
			return m, nil
		}
		// Remove from local list and reset cursor
		m.tasks = append(m.tasks[:m.cursor], m.tasks[m.cursor+1:]...)
		if m.cursor >= len(m.tasks) && m.cursor > 0 {
			m.cursor--
		}
		m.confirmDelete = false
		m.showDetail = false
		if len(m.tasks) == 0 {
			return m, tea.Quit
		}
		return m, nil
	case "n", "N", "esc", "ctrl+c":
		m.confirmDelete = false
		return m, nil
	}
	return m, nil
}

func (m listModel) View() string {
	if m.confirmDelete {
		return m.viewConfirmDelete()
	}
	if m.showDetail {
		return m.viewDetail()
	}
	return m.viewList()
}

func (m listModel) viewConfirmDelete() string {
	t := m.tasks[m.cursor]
	warnStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.Warning)
	mutedStyle := lipgloss.NewStyle().Foreground(styles.Muted)

	var sb strings.Builder
	sb.WriteString(warnStyle.Render(fmt.Sprintf("Delete %s: %s?", t.ID, t.Title)))
	sb.WriteString("\n\n")
	sb.WriteString(mutedStyle.Render(fmt.Sprintf("  Plan: %s", filepath.Base(t.SourceFile))))
	sb.WriteString("\n\n")
	sb.WriteString("  This will permanently remove the task from the plan file.\n\n")
	sb.WriteString(fmt.Sprintf("  %s / %s",
		lipgloss.NewStyle().Bold(true).Render("y/enter: confirm"),
		mutedStyle.Render("n/esc: cancel")))

	if m.width > 0 && m.height > 0 {
		return styles.FullScreen(sb.String(), "", m.width, m.height)
	}
	return styles.Box.Render(sb.String()) + "\n"
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

	footer := styles.StatusBar.Render("↑/↓: navigate · enter: details · alt+r: run · alt+bksp: delete · q/esc: exit")

	if m.width > 0 && m.height > 0 {
		return styles.FullScreen(sb.String(), footer, m.width, m.height)
	}
	return styles.Box.Render(sb.String()+"\n\n"+footer) + "\n"
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

	return sb.String()
}

func (m listModel) viewDetail() string {
	if !m.detailReady {
		return ""
	}

	footer := styles.StatusBar.Render("↑/↓: scroll · pgup/pgdn: prev/next task · alt+r: run · alt+bksp: delete · esc: back · q: exit")
	if m.detailViewport.TotalLineCount() <= m.detailViewport.Height {
		footer = styles.StatusBar.Render("pgup/pgdn: prev/next task · alt+r: run · alt+bksp: delete · esc: back · q: exit")
	}

	if m.width > 0 && m.height > 0 {
		return styles.FullScreenLeft(m.detailViewport.View(), footer, m.width, m.height)
	}
	return styles.Box.Render(m.detailViewport.View()+"\n"+footer) + "\n"
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
	if final, ok := result.(listModel); ok && final.runTaskID != "" {
		return dispatchWork(final.runTaskID)
	}
	return nil
}

func init() {
	rootCmd.AddCommand(listCmd)
	listCmd.Flags().IntP("count", "c", 5, "Number of tasks to show")
	listCmd.Flags().Bool("plain", false, "Strip colors and use ASCII characters for scripting/piping")
	listCmd.Flags().Bool("all", false, "Show all upcoming workable tasks with no count cap")
}
