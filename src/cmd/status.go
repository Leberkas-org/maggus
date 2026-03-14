package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/leberkas-org/maggus/internal/config"
	"github.com/leberkas-org/maggus/internal/parser"
	"github.com/leberkas-org/maggus/internal/tui/styles"

	"github.com/spf13/cobra"
)

const progressBarWidth = 10

// Lipgloss styles for the status command.
var (
	statusGreenStyle  = lipgloss.NewStyle().Foreground(styles.Success)
	statusCyanStyle   = lipgloss.NewStyle().Foreground(styles.Primary)
	statusYellowStyle = lipgloss.NewStyle().Foreground(styles.Warning)
	statusRedStyle    = lipgloss.NewStyle().Foreground(styles.Error)
	statusBlueStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("4"))
	statusDimStyle    = lipgloss.NewStyle().Faint(true)
	statusDimGreen    = lipgloss.NewStyle().Faint(true).Foreground(styles.Success)
)

type planInfo struct {
	filename  string
	tasks     []parser.Task
	completed bool // filename contains _completed
}

func (p *planInfo) doneCount() int {
	n := 0
	for _, t := range p.tasks {
		if t.IsComplete() {
			n++
		}
	}
	return n
}

func (p *planInfo) blockedCount() int {
	n := 0
	for _, t := range p.tasks {
		if !t.IsComplete() && t.IsBlocked() {
			n++
		}
	}
	return n
}

func buildProgressBar(done, total int) string {
	return styles.ProgressBar(done, total, progressBarWidth)
}

func buildProgressBarPlain(done, total int) string {
	return styles.ProgressBarPlain(done, total, progressBarWidth)
}

// statusModel is the bubbletea model for the interactive status TUI.
type statusModel struct {
	plans       []planInfo
	showAll     bool
	nextTaskID  string
	nextTaskFile string
	agentName   string

	// Flat list of selectable tasks (index into plans/tasks)
	selectableTasks []parser.Task
	cursor          int
	width           int
	height          int

	// Detail view state
	showDetail     bool
	detailViewport viewport.Model
	detailReady    bool

	// Run action
	runTaskID string

	// Delete confirmation
	confirmDelete bool
	deleteErr     string
	dir           string // working directory for file operations
}

func newStatusModel(plans []planInfo, showAll bool, nextTaskID, nextTaskFile, agentName, dir string) statusModel {
	// Build flat list of selectable tasks
	var selectable []parser.Task
	for _, p := range plans {
		if p.completed && !showAll {
			continue
		}
		selectable = append(selectable, p.tasks...)
	}

	return statusModel{
		plans:           plans,
		showAll:         showAll,
		nextTaskID:      nextTaskID,
		nextTaskFile:    nextTaskFile,
		agentName:       agentName,
		selectableTasks: selectable,
		dir:             dir,
	}
}

func (m statusModel) Init() tea.Cmd {
	return nil
}

func (m statusModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

func (m statusModel) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "alt+a":
		m.showAll = !m.showAll
		// Reload plans from disk to pick up external changes
		plans, err := parsePlans(m.dir)
		if err == nil {
			m.plans = plans
		}
		var selectable []parser.Task
		for _, p := range m.plans {
			if p.completed && !m.showAll {
				continue
			}
			selectable = append(selectable, p.tasks...)
		}
		m.selectableTasks = selectable
		m.nextTaskID, m.nextTaskFile = findNextTask(m.plans)
		m.cursor = 0
		return m, nil
	case "alt+r":
		if len(m.selectableTasks) == 0 {
			return m, nil
		}
		m.runTaskID = m.selectableTasks[m.cursor].ID
		return m, tea.Quit
	case "alt+backspace":
		if len(m.selectableTasks) > 0 {
			m.confirmDelete = true
			m.deleteErr = ""
		}
		return m, nil
	case "q", "esc", "ctrl+c":
		return m, tea.Quit
	case "up", "k":
		if len(m.selectableTasks) > 0 {
			if m.cursor > 0 {
				m.cursor--
			} else {
				m.cursor = len(m.selectableTasks) - 1
			}
		}
	case "down", "j":
		if len(m.selectableTasks) > 0 {
			if m.cursor < len(m.selectableTasks)-1 {
				m.cursor++
			} else {
				m.cursor = 0
			}
		}
	case "home":
		m.cursor = 0
	case "end":
		if len(m.selectableTasks) > 0 {
			m.cursor = len(m.selectableTasks) - 1
		}
	case "enter":
		if len(m.selectableTasks) > 0 {
			t := m.selectableTasks[m.cursor]
			m.showDetail = true
			content := m.renderDetailContent(t)
			w, h := styles.FullScreenInnerSize(m.width, m.height)
			m.detailViewport = viewport.New(w, h-1)
			m.detailViewport.SetContent(content)
			m.detailReady = true
			return m, nil
		}
	}
	return m, nil
}

func (m statusModel) updateDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "alt+r":
		m.runTaskID = m.selectableTasks[m.cursor].ID
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
		if m.cursor < len(m.selectableTasks)-1 {
			m.cursor++
			content := m.renderDetailContent(m.selectableTasks[m.cursor])
			m.detailViewport.SetContent(content)
			m.detailViewport.GotoTop()
		}
		return m, nil
	case "pgup":
		if m.cursor > 0 {
			m.cursor--
			content := m.renderDetailContent(m.selectableTasks[m.cursor])
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

func (m statusModel) updateConfirmDelete(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y", "enter":
		t := m.selectableTasks[m.cursor]
		if err := parser.DeleteTask(t.SourceFile, t.ID); err != nil {
			m.deleteErr = err.Error()
			m.confirmDelete = false
			return m, nil
		}
		// Reload plans from disk
		plans, err := parsePlans(m.dir)
		if err == nil {
			m.plans = plans
			var selectable []parser.Task
			for _, p := range plans {
				if p.completed && !m.showAll {
					continue
				}
				selectable = append(selectable, p.tasks...)
			}
			m.selectableTasks = selectable
			m.nextTaskID, m.nextTaskFile = findNextTask(plans)
		}
		if m.cursor >= len(m.selectableTasks) && m.cursor > 0 {
			m.cursor--
		}
		m.confirmDelete = false
		m.showDetail = false
		if len(m.selectableTasks) == 0 {
			return m, tea.Quit
		}
		return m, nil
	case "n", "N", "esc", "ctrl+c":
		m.confirmDelete = false
		return m, nil
	}
	return m, nil
}

func (m statusModel) View() string {
	if m.confirmDelete {
		return m.viewConfirmDelete()
	}
	if m.showDetail {
		return m.viewDetail()
	}
	return m.viewStatus()
}

func (m statusModel) viewConfirmDelete() string {
	t := m.selectableTasks[m.cursor]
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

func (m statusModel) viewStatus() string {
	var sb strings.Builder

	// Compute totals
	totalTasks := 0
	totalDone := 0
	totalBlocked := 0
	activePlans := 0
	for _, p := range m.plans {
		totalTasks += len(p.tasks)
		totalDone += p.doneCount()
		totalBlocked += p.blockedCount()
		if !p.completed {
			activePlans++
		}
	}
	totalPending := totalTasks - totalDone - totalBlocked

	// Header
	header := styles.Title.Render(fmt.Sprintf("Maggus Status — %d plans (%d active), %d tasks total",
		len(m.plans), activePlans, totalTasks))
	sb.WriteString(header)
	sb.WriteString("\n\n")

	// Summary
	summary := fmt.Sprintf(" Summary: %d/%d tasks complete · %d pending · %d blocked · Agent: %s",
		totalDone, totalTasks, totalPending, totalBlocked, m.agentName)
	sb.WriteString(summary)

	// Task sections with cursor
	taskIdx := 0
	for _, p := range m.plans {
		if p.completed && !m.showAll {
			continue
		}
		sb.WriteString("\n\n")
		if p.completed {
			sb.WriteString(statusDimGreen.Render(fmt.Sprintf(" Tasks — %s (archived)", p.filename)))
		} else {
			fmt.Fprintf(&sb, " Tasks — %s", p.filename)
		}
		sb.WriteString("\n")
		sb.WriteString(" " + styles.Separator(42))

		for _, t := range p.tasks {
			var icon string
			var style lipgloss.Style

			if t.IsComplete() {
				icon = "✓"
				if p.completed {
					style = statusDimGreen
				} else {
					style = statusGreenStyle
				}
			} else if t.IsBlocked() {
				icon = "⚠"
				style = statusRedStyle
			} else if t.ID == m.nextTaskID && t.SourceFile == m.nextTaskFile {
				icon = "→"
				style = statusCyanStyle
			} else {
				icon = "○"
				style = lipgloss.NewStyle().Foreground(styles.Muted)
			}

			if p.completed {
				style = statusDimStyle
			}

			// Cursor indicator
			var prefix string
			if taskIdx == m.cursor {
				prefix = " ▸ "
				// Override style for selected row
				if !p.completed {
					style = lipgloss.NewStyle().Bold(true).Foreground(styles.Primary)
				}
			} else {
				prefix = "   "
			}

			line := fmt.Sprintf("%s%s  %s: %s", prefix, icon, t.ID, t.Title)
			sb.WriteString("\n")
			sb.WriteString(style.Render(line))

			if t.IsBlocked() && !p.completed {
				for _, c := range t.Criteria {
					if !c.Blocked {
						continue
					}
					reason := strings.TrimPrefix(c.Text, "⚠️ BLOCKED: ")
					reason = strings.TrimPrefix(reason, "BLOCKED: ")
					blockedLine := fmt.Sprintf("         BLOCKED: %s", reason)
					sb.WriteString("\n")
					sb.WriteString(statusRedStyle.Render(blockedLine))
				}
			}

			taskIdx++
		}
	}

	// Plans table
	sb.WriteString("\n\n")
	sb.WriteString(" Plans")
	sb.WriteString("\n")
	sb.WriteString(" " + styles.Separator(42))

	maxCountWidth := 0
	for _, p := range m.plans {
		if p.completed && !m.showAll {
			continue
		}
		w := len(fmt.Sprintf("%d/%d", p.doneCount(), len(p.tasks)))
		if w > maxCountWidth {
			maxCountWidth = w
		}
	}
	countFmt := fmt.Sprintf("%%-%ds", maxCountWidth)

	for _, p := range m.plans {
		if p.completed && !m.showAll {
			continue
		}

		done := p.doneCount()
		total := len(p.tasks)
		bar := buildProgressBar(done, total)

		var prefix, suffix string
		var style lipgloss.Style

		if p.completed {
			prefix = " ✓ "
			style = statusDimGreen
			suffix = "done"
		} else if p.blockedCount() > 0 {
			prefix = "   "
			style = statusRedStyle
			suffix = "blocked"
		} else if total > 0 && done == total {
			prefix = "   "
			style = statusGreenStyle
			suffix = "done"
		} else if done == 0 {
			prefix = "   "
			style = statusBlueStyle
			suffix = "new"
		} else {
			prefix = "   "
			style = statusYellowStyle
			suffix = "in progress"
		}

		countStr := fmt.Sprintf(countFmt, fmt.Sprintf("%d/%d", done, total))
		// Render parts separately so the progress bar keeps its own colors
		labelPart := fmt.Sprintf("%s%-32s [", prefix, p.filename)
		afterPart := fmt.Sprintf("]  %s   %s", countStr, suffix)
		sb.WriteString("\n")
		sb.WriteString(style.Render(labelPart) + bar + style.Render(afterPart))
	}

	toggleHint := "alt+a: show all"
	if m.showAll {
		toggleHint = "alt+a: hide completed"
	}
	footer := styles.StatusBar.Render("↑/↓: navigate · enter: details · " + toggleHint + " · alt+r: run · alt+bksp: delete · q/esc: exit")

	if m.width > 0 && m.height > 0 {
		return styles.FullScreen(sb.String(), footer, m.width, m.height)
	}
	return styles.Box.Render(sb.String()+"\n\n"+footer) + "\n"
}

func (m statusModel) renderDetailContent(t parser.Task) string {
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

	// Status
	var statusText string
	var statusStyle lipgloss.Style
	if t.IsComplete() {
		statusText = "Complete"
		statusStyle = successStyle
	} else if t.IsBlocked() {
		statusText = "Blocked"
		statusStyle = warningStyle
	} else {
		statusText = "Pending"
		statusStyle = mutedStyle
	}
	sb.WriteString(fmt.Sprintf("%s  %s\n", labelStyle.Render("Status:"), statusStyle.Render(statusText)))

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

func (m statusModel) viewDetail() string {
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

// renderStatusPlain builds the plain-text status output (no ANSI, no TUI).
func renderStatusPlain(w *strings.Builder, plans []planInfo, showAll bool, nextTaskID, nextTaskFile, agentName string) {
	totalTasks := 0
	totalDone := 0
	totalBlocked := 0
	activePlans := 0
	for _, p := range plans {
		totalTasks += len(p.tasks)
		totalDone += p.doneCount()
		totalBlocked += p.blockedCount()
		if !p.completed {
			activePlans++
		}
	}
	totalPending := totalTasks - totalDone - totalBlocked

	fmt.Fprintf(w, "Maggus Status — %d plans (%d active), %d tasks total\n\n", len(plans), activePlans, totalTasks)
	fmt.Fprintf(w, " Summary: %d/%d tasks complete · %d pending · %d blocked\n", totalDone, totalTasks, totalPending, totalBlocked)
	fmt.Fprintf(w, " Agent: %s\n", agentName)

	// Find next task
	for _, p := range plans {
		if p.completed && !showAll {
			continue
		}
		fmt.Fprintln(w)
		if p.completed {
			fmt.Fprintf(w, " Tasks — %s (archived)\n", p.filename)
		} else {
			fmt.Fprintf(w, " Tasks — %s\n", p.filename)
		}
		fmt.Fprintln(w, " ──────────────────────────────────────────")

		for _, t := range p.tasks {
			var icon, prefix string

			if t.IsComplete() {
				icon = "[x]"
				prefix = "  "
			} else if t.IsBlocked() {
				icon = "[!]"
				prefix = "  "
			} else if t.ID == nextTaskID && t.SourceFile == nextTaskFile {
				icon = "o"
				prefix = "-> "
			} else {
				icon = "o"
				prefix = "  "
			}

			fmt.Fprintf(w, " %s%s  %s: %s\n", prefix, icon, t.ID, t.Title)

			if t.IsBlocked() && !p.completed {
				for _, c := range t.Criteria {
					if !c.Blocked {
						continue
					}
					reason := strings.TrimPrefix(c.Text, "⚠️ BLOCKED: ")
					reason = strings.TrimPrefix(reason, "BLOCKED: ")
					fmt.Fprintf(w, "         BLOCKED: %s\n", reason)
				}
			}
		}
	}

	// Plans table
	fmt.Fprintln(w)
	fmt.Fprintln(w, " Plans")
	fmt.Fprintln(w, " ──────────────────────────────────────────")

	maxCountWidth := 0
	for _, p := range plans {
		if p.completed && !showAll {
			continue
		}
		cw := len(fmt.Sprintf("%d/%d", p.doneCount(), len(p.tasks)))
		if cw > maxCountWidth {
			maxCountWidth = cw
		}
	}
	countFmt := fmt.Sprintf("%%-%ds", maxCountWidth)

	for _, p := range plans {
		if p.completed && !showAll {
			continue
		}

		done := p.doneCount()
		total := len(p.tasks)
		bar := buildProgressBarPlain(done, total)

		var prefix, suffix string

		if p.completed {
			prefix = " [x] "
			suffix = "done"
		} else if p.blockedCount() > 0 {
			prefix = "   "
			suffix = "blocked"
		} else if total > 0 && done == total {
			prefix = "   "
			suffix = "done"
		} else if done == 0 {
			prefix = "   "
			suffix = "new"
		} else {
			prefix = "   "
			suffix = "in progress"
		}

		countStr := fmt.Sprintf(countFmt, fmt.Sprintf("%d/%d", done, total))
		fmt.Fprintf(w, "%s%-32s [%s]  %s   %s\n", prefix, p.filename, bar, countStr, suffix)
	}
}

func parsePlans(dir string) ([]planInfo, error) {
	files, err := parser.GlobPlanFiles(dir, true)
	if err != nil {
		return nil, fmt.Errorf("glob plans: %w", err)
	}

	var plans []planInfo
	for _, f := range files {
		tasks, err := parser.ParseFile(f)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", f, err)
		}
		plans = append(plans, planInfo{
			filename:  filepath.Base(f),
			tasks:     tasks,
			completed: strings.HasSuffix(f, "_completed.md"),
		})
	}
	return plans, nil
}

func findNextTask(plans []planInfo) (string, string) {
	for _, p := range plans {
		if p.completed {
			continue
		}
		next := parser.FindNextIncomplete(p.tasks)
		if next != nil {
			return next.ID, next.SourceFile
		}
	}
	return "", ""
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show a compact summary of plan progress",
	Long:  `Reads all plan files in .maggus/ and displays a compact progress summary.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		plain, err := cmd.Flags().GetBool("plain")
		if err != nil {
			return err
		}
		all, err := cmd.Flags().GetBool("all")
		if err != nil {
			return err
		}

		dir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}

		cfg, err := config.Load(dir)
		if err != nil {
			return err
		}
		agentName := cfg.Agent

		plans, err := parsePlans(dir)
		if err != nil {
			return err
		}

		if len(plans) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No plans found.")
			return nil
		}

		nextTaskID, nextTaskFile := findNextTask(plans)

		if plain {
			var sb strings.Builder
			renderStatusPlain(&sb, plans, all, nextTaskID, nextTaskFile, agentName)
			fmt.Fprint(cmd.OutOrStdout(), sb.String())
			return nil
		}

		// TUI mode: interactive status with detail view
		m := newStatusModel(plans, all, nextTaskID, nextTaskFile, agentName, dir)
		prog := tea.NewProgram(m, tea.WithAltScreen())
		result, err := prog.Run()
		if err != nil {
			return err
		}
		if final, ok := result.(statusModel); ok && final.runTaskID != "" {
			return dispatchWork(final.runTaskID)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
	statusCmd.Flags().Bool("plain", false, "Strip colors and use ASCII characters for scripting/piping")
	statusCmd.Flags().Bool("all", false, "Show completed plans in task sections and Plans table")
}
