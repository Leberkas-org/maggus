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
	statusGreenStyle   = lipgloss.NewStyle().Foreground(styles.Success)
	statusCyanStyle    = lipgloss.NewStyle().Foreground(styles.Primary)
	statusRedStyle     = lipgloss.NewStyle().Foreground(styles.Error)
	statusDimStyle     = lipgloss.NewStyle().Faint(true)
	statusDimGreen     = lipgloss.NewStyle().Faint(true).Foreground(styles.Success)
	statusIgnoredStyle = lipgloss.NewStyle().Foreground(styles.Warning).Faint(true)
)

type planInfo struct {
	filename  string
	tasks     []parser.Task
	completed bool // filename contains _completed
	ignored   bool // filename contains _ignored
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

	// Plan tab selection
	selectedPlan int // index into visiblePlans()

	// Flat list of selectable tasks (index into plans/tasks)
	selectableTasks []parser.Task
	cursor          int
	scrollOffset    int
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

	// Temporary status note (e.g. "plan is already ignored")
	statusNote string

	// Criteria mode state (shared detail component)
	detail detailState
}

func newStatusModel(plans []planInfo, showAll bool, nextTaskID, nextTaskFile, agentName, dir string) statusModel {
	m := statusModel{
		plans:        plans,
		showAll:      showAll,
		nextTaskID:   nextTaskID,
		nextTaskFile: nextTaskFile,
		agentName:    agentName,
		dir:          dir,
	}
	visible := m.visiblePlans()
	if len(visible) > 0 {
		m.selectableTasks = buildSelectableTasksForPlan(visible[0], showAll)
	}
	return m
}

// visiblePlans returns the plans that should be shown based on the showAll flag.
func (m statusModel) visiblePlans() []planInfo {
	var visible []planInfo
	for _, p := range m.plans {
		if p.completed && !m.showAll {
			continue
		}
		visible = append(visible, p)
	}
	return visible
}

// buildSelectableTasksForPlan returns the flat list of tasks for a single plan.
// When showAll is false, completed tasks are excluded.
func buildSelectableTasksForPlan(plan planInfo, showAll bool) []parser.Task {
	var selectable []parser.Task
	for _, t := range plan.tasks {
		if !showAll && t.IsComplete() {
			continue
		}
		selectable = append(selectable, t)
	}
	return selectable
}

// rebuildForSelectedPlan rebuilds the selectable tasks and resets the cursor
// for the currently selected plan.
func (m *statusModel) rebuildForSelectedPlan() {
	visible := m.visiblePlans()
	if m.selectedPlan >= len(visible) {
		m.selectedPlan = 0
	}
	if len(visible) > 0 {
		m.selectableTasks = buildSelectableTasksForPlan(visible[m.selectedPlan], m.showAll)
	} else {
		m.selectableTasks = nil
	}
	m.cursor = 0
	m.scrollOffset = 0
}

// visibleTaskLines returns how many task lines fit in the status task list area.
func (m statusModel) visibleTaskLines() int {
	// Header: title + blank + tab bar (estimate 2 lines) + separator + blank + progress + blank + tasks header + separator = ~9 lines
	// Footer: 1 line
	headerLines := 9
	footerLines := 1
	_, innerH := styles.FullScreenInnerSize(m.width, m.height)
	avail := innerH - headerLines - footerLines
	if avail < 1 {
		avail = 1
	}
	return avail
}

// ensureCursorVisible adjusts scrollOffset so cursor is within the visible window.
func (m *statusModel) ensureCursorVisible() {
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
	// Clear status note on any key except alt+i/alt+p
	if msg.String() != "alt+i" && msg.String() != "alt+p" {
		m.statusNote = ""
	}
	switch msg.String() {
	case "tab", "right":
		// Next plan tab (wraps around)
		visible := m.visiblePlans()
		if len(visible) > 1 {
			m.selectedPlan = (m.selectedPlan + 1) % len(visible)
			m.rebuildForSelectedPlan()
		}
		return m, nil
	case "shift+tab", "left":
		// Previous plan tab (wraps around)
		visible := m.visiblePlans()
		if len(visible) > 1 {
			m.selectedPlan--
			if m.selectedPlan < 0 {
				m.selectedPlan = len(visible) - 1
			}
			m.rebuildForSelectedPlan()
		}
		return m, nil
	case "alt+a":
		m.showAll = !m.showAll
		// Reload plans from disk to pick up external changes
		plans, err := parsePlans(m.dir)
		if err == nil {
			m.plans = plans
		}
		m.nextTaskID, m.nextTaskFile = findNextTask(m.plans)
		m.rebuildForSelectedPlan()
		return m, nil
	case "alt+r":
		if len(m.selectableTasks) == 0 {
			return m, nil
		}
		m.runTaskID = m.selectableTasks[m.cursor].ID
		return m, tea.Quit
	case "alt+i":
		if len(m.selectableTasks) == 0 {
			return m, nil
		}
		t := m.selectableTasks[m.cursor]
		// Clear any previous note
		m.statusNote = ""
		// Show note if plan is ignored, but still toggle
		visible := m.visiblePlans()
		if m.selectedPlan < len(visible) && visible[m.selectedPlan].ignored {
			m.statusNote = "plan is already ignored"
		}
		// Toggle: if task is ignored, remove prefix; if not, add prefix
		if err := rewriteTaskHeading(t.SourceFile, t.ID, t.Ignored); err != nil {
			m.statusNote = "error: " + err.Error()
			return m, nil
		}
		// Remember current task ID for cursor restore
		cursorTaskID := t.ID
		// Reload plans from disk
		plans, err := parsePlans(m.dir)
		if err == nil {
			m.plans = plans
			m.nextTaskID, m.nextTaskFile = findNextTask(plans)
		}
		m.rebuildForSelectedPlan()
		// Restore cursor to same logical task
		for i, st := range m.selectableTasks {
			if st.ID == cursorTaskID {
				m.cursor = i
				break
			}
		}
		m.ensureCursorVisible()
		return m, nil
	case "alt+p":
		m.statusNote = ""
		visible := m.visiblePlans()
		if m.selectedPlan >= len(visible) {
			return m, nil
		}
		p := visible[m.selectedPlan]
		// Completed plans cannot be ignored — silent no-op
		if p.completed {
			return m, nil
		}
		// Toggle: rename file
		fullPath := filepath.Join(m.dir, ".maggus", p.filename)
		var newPath string
		if p.ignored {
			newPath = strings.TrimSuffix(fullPath, "_ignored.md") + ".md"
		} else {
			newPath = strings.TrimSuffix(fullPath, ".md") + "_ignored.md"
		}
		if err := os.Rename(fullPath, newPath); err != nil {
			m.statusNote = "error: " + err.Error()
			return m, nil
		}
		// Remember the new filename base for cursor restore
		newBase := filepath.Base(newPath)
		// Reload plans from disk
		plans, err := parsePlans(m.dir)
		if err == nil {
			m.plans = plans
			m.nextTaskID, m.nextTaskFile = findNextTask(plans)
		}
		// Restore tab selection to the same plan (by new filename)
		newVisible := m.visiblePlans()
		m.selectedPlan = 0
		for i, vp := range newVisible {
			if vp.filename == newBase {
				m.selectedPlan = i
				break
			}
		}
		m.rebuildForSelectedPlan()
		return m, nil
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
			m.ensureCursorVisible()
		}
	case "down", "j":
		if len(m.selectableTasks) > 0 {
			if m.cursor < len(m.selectableTasks)-1 {
				m.cursor++
			} else {
				m.cursor = 0
			}
			m.ensureCursorVisible()
		}
	case "home":
		m.cursor = 0
		m.ensureCursorVisible()
	case "end":
		if len(m.selectableTasks) > 0 {
			m.cursor = len(m.selectableTasks) - 1
			m.ensureCursorVisible()
		}
	case "enter":
		if len(m.selectableTasks) > 0 {
			t := m.selectableTasks[m.cursor]
			m.showDetail = true
			m.detail = detailState{}
			content := renderDetailContent(t, &m.detail)
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
	// Handle action picker mode
	if m.detail.showActionPicker {
		return m.updateActionPicker(msg)
	}

	// Handle criteria mode
	if m.detail.criteriaMode {
		return m.updateCriteriaMode(msg)
	}

	switch msg.String() {
	case "alt+i":
		t := m.selectableTasks[m.cursor]
		if t.IsComplete() {
			m.statusNote = "cannot ignore a completed task"
			m.refreshDetailViewport()
			return m, nil
		}
		m.statusNote = ""
		// Show note if plan is ignored
		visible := m.visiblePlans()
		if m.selectedPlan < len(visible) && visible[m.selectedPlan].ignored {
			m.statusNote = "plan is already ignored"
		}
		if err := rewriteTaskHeading(t.SourceFile, t.ID, t.Ignored); err != nil {
			m.statusNote = "error: " + err.Error()
			m.refreshDetailViewport()
			return m, nil
		}
		cursorTaskID := t.ID
		plans, err := parsePlans(m.dir)
		if err == nil {
			m.plans = plans
			m.nextTaskID, m.nextTaskFile = findNextTask(plans)
		}
		m.rebuildForSelectedPlan()
		for i, st := range m.selectableTasks {
			if st.ID == cursorTaskID {
				m.cursor = i
				break
			}
		}
		m.ensureCursorVisible()
		// Reload task and refresh detail viewport
		if updated := reloadTask(m.selectableTasks[m.cursor].SourceFile, m.selectableTasks[m.cursor].ID); updated != nil {
			m.selectableTasks[m.cursor] = *updated
		}
		m.detail.exitCriteriaMode()
		m.refreshDetailViewport()
		return m, nil
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
		m.detail.exitCriteriaMode()
		return m, nil
	case "ctrl+c":
		return m, tea.Quit
	case "tab", "b":
		m.detail.noBlockedMsg = false
		if !m.detail.initCriteriaMode(m.selectableTasks[m.cursor]) {
			m.detail.noBlockedMsg = true
			m.refreshDetailViewport()
			return m, nil
		}
		m.refreshDetailViewport()
		return m, nil
	case "pgdown":
		if m.cursor < len(m.selectableTasks)-1 {
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

func (m statusModel) updateCriteriaMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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

func (m statusModel) updateActionPicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
		modified, _ := m.detail.performAction(m.selectableTasks[m.cursor], action)
		m.detail.showActionPicker = false
		if modified {
			// Reload task from disk
			if updated := reloadTask(m.selectableTasks[m.cursor].SourceFile, m.selectableTasks[m.cursor].ID); updated != nil {
				m.selectableTasks[m.cursor] = *updated
			}
			// Re-init criteria mode with updated task
			if !m.detail.initCriteriaMode(m.selectableTasks[m.cursor]) {
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

func (m *statusModel) refreshDetailViewport() {
	content := renderDetailContent(m.selectableTasks[m.cursor], &m.detail)
	if m.statusNote != "" {
		mutedStyle := lipgloss.NewStyle().Foreground(styles.Muted)
		content += "\n" + mutedStyle.Render("  "+m.statusNote)
	}
	m.detailViewport.SetContent(content)
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
			m.nextTaskID, m.nextTaskFile = findNextTask(plans)
		}
		m.rebuildForSelectedPlan()
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
	if len(m.plans) == 0 {
		return m.viewEmpty()
	}
	if m.confirmDelete {
		return m.viewConfirmDelete()
	}
	if m.showDetail {
		return m.viewDetail()
	}
	return m.viewStatus()
}

func (m statusModel) viewEmpty() string {
	mutedStyle := lipgloss.NewStyle().Foreground(styles.Muted)

	var sb strings.Builder
	sb.WriteString(styles.Title.Render("Status") + "\n\n")
	sb.WriteString(mutedStyle.Render("No plans found.") + "\n\n")
	sb.WriteString(mutedStyle.Render("Create a plan with ") +
		lipgloss.NewStyle().Bold(true).Foreground(styles.Primary).Render("maggus plan") +
		mutedStyle.Render(" to get started.") + "\n")

	footer := styles.StatusBar.Render("q/esc: exit")

	if m.width > 0 && m.height > 0 {
		return styles.FullScreen(sb.String(), footer, m.width, m.height)
	}
	return styles.Box.Render(sb.String()) + "\n"
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

// renderTabBar renders the horizontal plan tab bar.
func (m statusModel) renderTabBar() string {
	visible := m.visiblePlans()
	if len(visible) == 0 {
		return ""
	}

	selectedStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.Primary)
	unselectedStyle := lipgloss.NewStyle().Foreground(styles.Muted)
	ignoredTabStyle := lipgloss.NewStyle().Foreground(styles.Warning).Faint(true)

	var tabs []string
	for i, p := range visible {
		done := p.doneCount()
		total := len(p.tasks)
		name := strings.TrimSuffix(p.filename, ".md")
		prefix := ""
		if p.ignored {
			prefix = "~"
		}
		label := fmt.Sprintf(" %s%s %d/%d ", prefix, name, done, total)
		if i == m.selectedPlan {
			if p.ignored {
				tabs = append(tabs, ignoredTabStyle.Bold(true).Render(label))
			} else {
				tabs = append(tabs, selectedStyle.Render(label))
			}
		} else {
			if p.ignored {
				tabs = append(tabs, ignoredTabStyle.Render(label))
			} else {
				tabs = append(tabs, unselectedStyle.Render(label))
			}
		}
	}

	// Join tabs with a separator, wrapping to next line if needed
	sep := statusDimStyle.Render("│")
	maxWidth := m.width - 8 // account for box border/padding
	if maxWidth <= 0 {
		maxWidth = 80
	}

	var lines []string
	var currentLine string
	currentVisualWidth := 0
	for _, tab := range tabs {
		tabWidth := lipgloss.Width(tab)
		sepWidth := 0
		if currentLine != "" {
			sepWidth = 1 // separator character
		}
		if currentVisualWidth+sepWidth+tabWidth > maxWidth && currentLine != "" {
			lines = append(lines, currentLine)
			currentLine = tab
			currentVisualWidth = tabWidth
		} else {
			if currentLine != "" {
				currentLine += sep
				currentVisualWidth += sepWidth
			}
			currentLine += tab
			currentVisualWidth += tabWidth
		}
	}
	if currentLine != "" {
		lines = append(lines, currentLine)
	}

	return " " + strings.Join(lines, "\n ")
}

func (m statusModel) viewStatus() string {
	var sb strings.Builder

	visible := m.visiblePlans()

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

	// Tab bar
	if len(visible) > 0 {
		sb.WriteString(m.renderTabBar())
		sb.WriteString("\n")
		sb.WriteString(" " + styles.Separator(42))
		sb.WriteString("\n")
	}

	// Progress bar and summary for selected plan
	if m.selectedPlan < len(visible) {
		p := visible[m.selectedPlan]
		done := p.doneCount()
		total := len(p.tasks)
		blocked := p.blockedCount()
		pending := total - done - blocked
		sb.WriteString("\n " + buildProgressBar(done, total))
		summary := fmt.Sprintf("  %d/%d tasks · %d pending · %d blocked",
			done, total, pending, blocked)
		sb.WriteString(statusDimStyle.Render(summary))
	} else {
		sb.WriteString("\n " + buildProgressBar(totalDone, totalTasks))
		summary := fmt.Sprintf("  %d/%d tasks · %d pending · %d blocked",
			totalDone, totalTasks, totalPending, totalBlocked)
		sb.WriteString(statusDimStyle.Render(summary))
	}

	// Task list for selected plan
	if m.selectedPlan < len(visible) {
		p := visible[m.selectedPlan]

		sb.WriteString("\n\n")
		if p.completed {
			sb.WriteString(statusDimGreen.Render(fmt.Sprintf(" Tasks — %s (archived)", p.filename)))
		} else {
			fmt.Fprintf(&sb, " Tasks — %s", p.filename)
		}
		sb.WriteString("\n")
		sb.WriteString(" " + styles.Separator(42))

		// Determine visible window for scrolling
		visibleLines := m.visibleTaskLines()
		end := m.scrollOffset + visibleLines
		if end > len(m.selectableTasks) {
			end = len(m.selectableTasks)
		}

		for taskIdx := m.scrollOffset; taskIdx < end; taskIdx++ {
			t := m.selectableTasks[taskIdx]

			var icon string
			var style lipgloss.Style

			if t.Ignored {
				icon = "~"
				style = statusIgnoredStyle
			} else if t.IsComplete() {
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
		}

		// Scroll indicator
		if len(m.selectableTasks) > visibleLines {
			scrollHint := fmt.Sprintf(" [%d-%d of %d]", m.scrollOffset+1, end, len(m.selectableTasks))
			sb.WriteString("\n")
			sb.WriteString(statusDimStyle.Render(scrollHint))
		}
	}

	// Status note (e.g. "plan is already ignored")
	if m.statusNote != "" {
		sb.WriteString("\n")
		sb.WriteString(statusDimStyle.Render("  " + m.statusNote))
	}

	toggleHint := "alt+a: show all"
	if m.showAll {
		toggleHint = "alt+a: hide completed"
	}
	footer := styles.StatusBar.Render("tab/shift+tab: switch plan · ↑/↓: navigate · enter: details · " + toggleHint + " · alt+i: ignore/unignore · alt+p: ignore/unignore plan · alt+r: run · alt+bksp: delete · q/esc: exit")

	if m.width > 0 && m.height > 0 {
		return styles.FullScreen(sb.String(), footer, m.width, m.height)
	}
	return styles.Box.Render(sb.String()+"\n\n"+footer) + "\n"
}


func (m statusModel) viewDetail() string {
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
		} else if p.ignored {
			fmt.Fprintf(w, " Tasks — [~] %s (ignored)\n", p.filename)
		} else {
			fmt.Fprintf(w, " Tasks — %s\n", p.filename)
		}
		fmt.Fprintln(w, " ──────────────────────────────────────────")

		for _, t := range p.tasks {
			var icon, prefix string

			if t.Ignored {
				icon = "[~]"
				prefix = "  "
			} else if t.IsComplete() {
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
		} else if p.ignored {
			prefix = " [~] "
			suffix = "ignored"
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
		ignored := parser.IsIgnoredFile(f)
		if ignored {
			for i := range tasks {
				tasks[i].Ignored = true
			}
		}
		plans = append(plans, planInfo{
			filename:  filepath.Base(f),
			tasks:     tasks,
			completed: strings.HasSuffix(f, "_completed.md"),
			ignored:   ignored,
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
			if plain {
				fmt.Fprintln(cmd.OutOrStdout(), "No plans found.")
				return nil
			}
			// TUI mode: show empty status view
			plans = []planInfo{}
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
