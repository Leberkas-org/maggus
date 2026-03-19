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
	cursor    int
	width     int
	height    int

	// Scrollable list state
	scrollOffset int

	// Detail view state
	showDetail     bool
	detailViewport viewport.Model
	detailReady    bool

	// Run action
	runTaskID string // task ID to run when user presses Alt+R

	// Delete confirmation
	confirmDelete bool
	deleteErr     string

	// Criteria mode state (shared detail component)
	detail detailState
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

// visibleTaskLines returns how many task lines fit in the list view.
func (m listModel) visibleTaskLines() int {
	// Header: title + agent + separator + blank = 4 lines
	// Footer: 1 line
	headerLines := 4
	footerLines := 1
	_, innerH := styles.FullScreenInnerSize(m.width, m.height)
	avail := innerH - headerLines - footerLines
	if avail < 1 {
		avail = 1
	}
	return avail
}

// ensureCursorVisible adjusts scrollOffset so cursor is within the visible window.
func (m *listModel) ensureCursorVisible() {
	visible := m.visibleTaskLines()
	if m.cursor < m.scrollOffset {
		m.scrollOffset = m.cursor
	}
	if m.cursor >= m.scrollOffset+visible {
		m.scrollOffset = m.cursor - visible + 1
	}
	if m.scrollOffset < 0 {
		m.scrollOffset = 0
	}
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
		m.ensureCursorVisible()
	case "down", "j":
		if m.cursor < len(m.tasks)-1 {
			m.cursor++
		} else {
			m.cursor = 0
		}
		m.ensureCursorVisible()
	case "home":
		m.cursor = 0
		m.ensureCursorVisible()
	case "end":
		m.cursor = len(m.tasks) - 1
		m.ensureCursorVisible()
	case "enter":
		m.showDetail = true
		m.detail = detailState{}
		content := renderDetailContent(m.tasks[m.cursor], &m.detail)
		w, h := styles.FullScreenInnerSize(m.width, m.height)
		m.detailViewport = viewport.New(w, h-1)
		m.detailViewport.SetContent(content)
		m.detailReady = true
		return m, nil
	}
	return m, nil
}

func (m listModel) updateDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle action picker mode
	if m.detail.showActionPicker {
		return m.updateActionPicker(msg)
	}

	// Handle criteria mode
	if m.detail.criteriaMode {
		return m.updateCriteriaMode(msg)
	}

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
		m.detail.exitCriteriaMode()
		return m, nil
	case "ctrl+c":
		return m, tea.Quit
	case "tab", "b":
		m.detail.noBlockedMsg = false
		if !m.detail.initCriteriaMode(m.tasks[m.cursor]) {
			m.detail.noBlockedMsg = true
			m.refreshDetailViewport()
			return m, nil
		}
		m.refreshDetailViewport()
		return m, nil
	case "pgdown":
		if m.cursor < len(m.tasks)-1 {
			m.cursor++
			m.detail.exitCriteriaMode()
			m.refreshDetailViewport()
		}
		return m, nil
	case "pgup":
		if m.cursor > 0 {
			m.cursor--
			m.detail.exitCriteriaMode()
			m.refreshDetailViewport()
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

func (m listModel) updateCriteriaMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.detail.criteriaCursor > 0 {
			m.detail.criteriaCursor--
			m.refreshDetailViewport()
		}
	case "down", "j":
		if m.detail.criteriaCursor < len(m.detail.blockedIndices)-1 {
			m.detail.criteriaCursor++
			m.refreshDetailViewport()
		}
	case "enter":
		m.detail.showActionPicker = true
		m.detail.actionCursor = 0
		m.refreshDetailViewport()
	case "tab":
		m.detail.exitCriteriaMode()
		m.refreshDetailViewport()
	case "esc", "backspace":
		m.showDetail = false
		m.detailReady = false
		m.detail.exitCriteriaMode()
		return m, nil
	case "q":
		return m, tea.Quit
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m listModel) updateActionPicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.detail.actionCursor > 0 {
			m.detail.actionCursor--
			m.refreshDetailViewport()
		}
	case "down", "j":
		if m.detail.actionCursor < len(criteriaActions)-1 {
			m.detail.actionCursor++
			m.refreshDetailViewport()
		}
	case "enter":
		action := criteriaActions[m.detail.actionCursor]
		modified, _ := m.detail.performAction(m.tasks[m.cursor], action)
		m.detail.showActionPicker = false
		if modified {
			// Reload task from disk
			if updated := reloadTask(m.tasks[m.cursor].SourceFile, m.tasks[m.cursor].ID); updated != nil {
				m.tasks[m.cursor] = *updated
			}
			// Re-init criteria mode with updated task
			if !m.detail.initCriteriaMode(m.tasks[m.cursor]) {
				m.detail.exitCriteriaMode()
			}
		}
		m.refreshDetailViewport()
	case "esc":
		m.detail.showActionPicker = false
		m.refreshDetailViewport()
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m *listModel) refreshDetailViewport() {
	content := renderDetailContent(m.tasks[m.cursor], &m.detail)
	m.detailViewport.SetContent(content)
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
	if len(m.tasks) == 0 {
		return m.viewEmpty()
	}
	if m.confirmDelete {
		return m.viewConfirmDelete()
	}
	if m.showDetail {
		return m.viewDetail()
	}
	return m.viewList()
}

func (m listModel) viewEmpty() string {
	mutedStyle := lipgloss.NewStyle().Foreground(styles.Muted)

	var sb strings.Builder
	sb.WriteString(styles.Title.Render("Task List") + "\n\n")
	sb.WriteString(lipgloss.NewStyle().Foreground(styles.Success).Render("All done!") + "\n\n")
	sb.WriteString(mutedStyle.Render("No pending tasks found. All tasks are complete or no plans exist.") + "\n")

	footer := styles.StatusBar.Render("q/esc: exit")

	if m.width > 0 && m.height > 0 {
		return styles.FullScreen(sb.String(), footer, m.width, m.height)
	}
	return styles.Box.Render(sb.String()) + "\n"
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
	blockedStyle := lipgloss.NewStyle().Foreground(styles.Warning)
	ignoredStyle := lipgloss.NewStyle().Foreground(styles.Warning).Faint(true)
	mutedStyle := lipgloss.NewStyle().Faint(true)
	normalStyle := lipgloss.NewStyle()

	var sb strings.Builder

	// Header — always shows total incomplete count
	header := styles.Title.Render(fmt.Sprintf("All incomplete tasks (%d)", len(m.tasks)))
	sb.WriteString(header)
	sb.WriteString("\n")
	sb.WriteString(mutedStyle.Render(fmt.Sprintf(" Agent: %s", m.agentName)))
	sb.WriteString("\n")
	sb.WriteString(" " + styles.Separator(42))
	sb.WriteString("\n")

	// Determine visible window
	visible := m.visibleTaskLines()
	end := m.scrollOffset + visible
	if end > len(m.tasks) {
		end = len(m.tasks)
	}

	for i := m.scrollOffset; i < end; i++ {
		t := m.tasks[i]
		planFile := mutedStyle.Render(filepath.Base(t.SourceFile))
		blocked := t.IsBlocked()
		ignored := t.Ignored

		var icon string
		if ignored {
			icon = ignoredStyle.Render("~")
		} else if blocked {
			icon = blockedStyle.Render("⊘")
		} else {
			icon = " "
		}

		label := fmt.Sprintf("#%-2d %s: %s", i+1, t.ID, t.Title)

		if i == m.cursor {
			fmt.Fprintf(&sb, " %s %s %s  %s\n",
				cursorStyle.Render("→"),
				icon,
				selectedStyle.Render(label),
				planFile)
		} else if ignored {
			fmt.Fprintf(&sb, "   %s %s  %s\n",
				icon,
				ignoredStyle.Render(label),
				planFile)
		} else if blocked {
			fmt.Fprintf(&sb, "   %s %s  %s\n",
				icon,
				blockedStyle.Render(label),
				planFile)
		} else {
			fmt.Fprintf(&sb, "   %s %s  %s\n",
				icon,
				normalStyle.Render(label),
				planFile)
		}
	}

	// Scroll indicator
	scrollHint := ""
	if len(m.tasks) > visible {
		scrollHint = fmt.Sprintf(" [%d-%d of %d]", m.scrollOffset+1, end, len(m.tasks))
	}

	footer := styles.StatusBar.Render("↑/↓: navigate · enter: details · alt+r: run · alt+bksp: delete · q/esc: exit" + scrollHint)

	if m.width > 0 && m.height > 0 {
		return styles.FullScreen(sb.String(), footer, m.width, m.height)
	}
	return styles.Box.Render(sb.String()+"\n\n"+footer) + "\n"
}

func (m listModel) viewDetail() string {
	if !m.detailReady {
		return ""
	}

	scrollable := m.detailViewport.TotalLineCount() > m.detailViewport.Height
	footer := detailFooter(&m.detail, scrollable)

	if m.width > 0 && m.height > 0 {
		return styles.FullScreenLeft(m.detailViewport.View(), footer, m.width, m.height)
	}
	return styles.Box.Render(m.detailViewport.View()+"\n"+footer) + "\n"
}

// renderListPlain builds the plain-text list output (no ANSI, no TUI).
func renderListPlain(workable []parser.Task, ignored []parser.Task, all bool, agentName string) string {
	var sb strings.Builder

	if all {
		fmt.Fprintln(&sb, "All upcoming tasks:")
	} else {
		fmt.Fprintf(&sb, "Next %d task(s):\n", len(workable))
	}
	fmt.Fprintf(&sb, "Agent: %s\n", agentName)
	fmt.Fprintln(&sb)

	idx := 1
	// Merge workable and ignored tasks in order, but workable first then ignored
	for i, t := range workable {
		planFile := filepath.Base(t.SourceFile)
		if i == 0 {
			fmt.Fprintf(&sb, " -> #%-2d %s: %s  %s\n", idx, t.ID, t.Title, planFile)
		} else {
			fmt.Fprintf(&sb, "    #%-2d %s: %s  %s\n", idx, t.ID, t.Title, planFile)
		}
		idx++
	}
	for _, t := range ignored {
		planFile := filepath.Base(t.SourceFile)
		fmt.Fprintf(&sb, " [~]#%-2d %s: %s  %s\n", idx, t.ID, t.Title, planFile)
		idx++
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

	// Collect tasks from all active plan files
	var workable []parser.Task
	var ignored []parser.Task
	var incomplete []parser.Task
	for _, f := range files {
		tasks, err := parser.ParseFile(f)
		if err != nil {
			return fmt.Errorf("parse %s: %w", f, err)
		}
		fileIgnored := parser.IsIgnoredFile(f)
		for i := range tasks {
			if fileIgnored {
				tasks[i].Ignored = true
			}
		}
		for _, t := range tasks {
			if t.Ignored {
				ignored = append(ignored, t)
			} else if t.IsWorkable() {
				workable = append(workable, t)
			}
			if !t.IsComplete() {
				incomplete = append(incomplete, t)
			}
		}
	}

	if plain {
		// Plain mode: workable + ignored, respects --count and --all for workable
		if len(workable) == 0 && len(ignored) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No pending tasks found. All done!")
			return nil
		}
		if !all && count < len(workable) {
			workable = workable[:count]
		}
		fmt.Fprint(cmd.OutOrStdout(), renderListPlain(workable, ignored, all, agentName))
		return nil
	}

	// TUI mode: all incomplete tasks (workable + blocked), no count cap
	m := listModel{tasks: incomplete, agentName: agentName}
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
