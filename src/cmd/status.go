package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/leberkas-org/maggus/internal/claude2x"
	"github.com/leberkas-org/maggus/internal/config"
	"github.com/leberkas-org/maggus/internal/parser"
	"github.com/leberkas-org/maggus/internal/tui/styles"

	"github.com/spf13/cobra"
)

const statusHeaderLines = 9 // title + blank + tab bar (~2) + separator + blank + progress + blank + tasks header + separator

// Lipgloss styles for the status command.
var (
	statusGreenStyle   = lipgloss.NewStyle().Foreground(styles.Success)
	statusCyanStyle    = lipgloss.NewStyle().Foreground(styles.Primary)
	statusRedStyle     = lipgloss.NewStyle().Foreground(styles.Error)
	statusDimStyle     = lipgloss.NewStyle().Faint(true)
	statusDimGreen     = lipgloss.NewStyle().Faint(true).Foreground(styles.Success)
	statusIgnoredStyle = lipgloss.NewStyle().Foreground(styles.Warning).Faint(true)
)

// statusModel is the bubbletea model for the interactive status TUI.
type statusModel struct {
	taskListComponent

	features     []featureInfo
	showAll      bool
	nextTaskID   string
	nextTaskFile string
	agentName    string

	// Feature tab selection
	selectedFeature int // index into visibleFeatures()

	dir string // working directory for file operations

	is2x bool // true when Claude is in 2x mode (border turns yellow)

	// Temporary status note (e.g. "feature is already ignored")
	statusNote string
}

func newStatusModel(features []featureInfo, showAll bool, nextTaskID, nextTaskFile, agentName, dir string) statusModel {
	m := statusModel{
		taskListComponent: taskListComponent{
			HeaderLines: statusHeaderLines,
		},
		features:     features,
		showAll:      showAll,
		nextTaskID:   nextTaskID,
		nextTaskFile: nextTaskFile,
		agentName:    agentName,
		dir:          dir,
	}
	visible := m.visibleFeatures()
	if len(visible) > 0 {
		m.Tasks = buildSelectableTasksForFeature(visible[0], showAll)
	}
	return m
}

// visibleFeatures returns the features that should be shown based on the showAll flag.
func (m statusModel) visibleFeatures() []featureInfo {
	var visible []featureInfo
	for _, f := range m.features {
		if f.completed && !m.showAll {
			continue
		}
		visible = append(visible, f)
	}
	return visible
}

// rebuildForSelectedFeature rebuilds the selectable tasks and resets the cursor
// for the currently selected feature.
func (m *statusModel) rebuildForSelectedFeature() {
	visible := m.visibleFeatures()
	if m.selectedFeature >= len(visible) {
		m.selectedFeature = 0
	}
	if len(visible) > 0 {
		m.Tasks = buildSelectableTasksForFeature(visible[m.selectedFeature], m.showAll)
	} else {
		m.Tasks = nil
	}
	m.Cursor = 0
	m.ScrollOffset = 0
}

// reloadFeatures reloads all features and bugs from disk and rebuilds the current view.
func (m *statusModel) reloadFeatures() {
	features, err := parseFeatures(m.dir)
	if err != nil {
		m.rebuildForSelectedFeature()
		return
	}
	bugs, err := parseBugs(m.dir)
	if err == nil {
		features = append(features, bugs...)
	}
	m.features = features
	m.nextTaskID, m.nextTaskFile = findNextTask(features)
	m.rebuildForSelectedFeature()
}

// syncDetailSuffix updates the component's DetailSuffix from statusNote.
func (m *statusModel) syncDetailSuffix() {
	if m.statusNote != "" {
		mutedStyle := lipgloss.NewStyle().Foreground(styles.Muted)
		m.DetailSuffix = "\n" + mutedStyle.Render("  "+m.statusNote)
	} else {
		m.DetailSuffix = ""
	}
}

func (m statusModel) Init() tea.Cmd {
	return func() tea.Msg {
		return claude2xResultMsg{status: claude2x.FetchStatus()}
	}
}

func (m statusModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.HandleResize(msg.Width, msg.Height)
		return m, nil

	case claude2xResultMsg:
		m.is2x = msg.status.Is2x
		m.BorderColor = styles.ThemeColor(m.is2x)
		if m.is2x {
			return m, next2xTick()
		}
		return m, nil
	case claude2xTickMsg:
		is2x, _, tickCmd := fetch2xAndUpdate()
		m.is2x = is2x
		m.BorderColor = styles.ThemeColor(m.is2x)
		return m, tickCmd

	case tea.KeyMsg:
		if m.ConfirmDelete {
			return m.updateStatusConfirmDelete(msg)
		}
		if m.ShowDetail {
			return m.updateStatusDetail(msg)
		}
		return m.updateList(msg)
	}

	cmd := m.UpdateViewport(msg)
	return m, cmd
}

func (m statusModel) updateStatusConfirmDelete(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y", "enter":
		t := m.Tasks[m.Cursor]
		if err := parser.DeleteTask(t.SourceFile, t.ID); err != nil {
			m.DeleteErr = err.Error()
			m.ConfirmDelete = false
			return m, nil
		}
		m.reloadFeatures()
		if m.Cursor >= len(m.Tasks) && m.Cursor > 0 {
			m.Cursor--
		}
		m.ConfirmDelete = false
		m.ShowDetail = false
		if len(m.Tasks) == 0 {
			return m, tea.Quit
		}
		return m, nil
	case "n", "N", "esc", "ctrl+c":
		m.ConfirmDelete = false
		return m, nil
	}
	return m, nil
}

func (m statusModel) updateStatusDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Intercept status-specific keys before delegating to component
	if msg.String() == "alt+i" {
		return m.handleIgnoreTask(true)
	}
	cmd, action := m.taskListComponent.Update(msg)
	switch action {
	case taskListQuit, taskListRun:
		return m, tea.Quit
	}
	return m, cmd
}

func (m statusModel) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Clear status note on any key except alt+i/alt+p
	if msg.String() != "alt+i" && msg.String() != "alt+p" {
		m.statusNote = ""
	}
	m.syncDetailSuffix()

	switch msg.String() {
	case "tab", "right":
		visible := m.visibleFeatures()
		if len(visible) > 1 {
			m.selectedFeature = (m.selectedFeature + 1) % len(visible)
			m.rebuildForSelectedFeature()
		}
		return m, nil
	case "shift+tab", "left":
		visible := m.visibleFeatures()
		if len(visible) > 1 {
			m.selectedFeature--
			if m.selectedFeature < 0 {
				m.selectedFeature = len(visible) - 1
			}
			m.rebuildForSelectedFeature()
		}
		return m, nil
	case "alt+a":
		m.showAll = !m.showAll
		features, err := parseFeatures(m.dir)
		if err == nil {
			bugs, bugErr := parseBugs(m.dir)
			if bugErr == nil {
				features = append(features, bugs...)
			}
			m.features = features
		}
		m.nextTaskID, m.nextTaskFile = findNextTask(m.features)
		m.rebuildForSelectedFeature()
		return m, nil
	case "alt+i":
		return m.handleIgnoreTask(false)
	case "alt+p":
		return m.handleIgnoreFeature()
	}

	// Delegate to component for shared navigation
	cmd, action := m.taskListComponent.Update(msg)
	switch action {
	case taskListQuit, taskListRun:
		return m, tea.Quit
	case taskListDeleted:
		m.reloadFeatures()
	}
	return m, cmd
}

func (m statusModel) handleIgnoreTask(inDetail bool) (tea.Model, tea.Cmd) {
	if len(m.Tasks) == 0 {
		return m, nil
	}
	t := m.Tasks[m.Cursor]
	if t.IsComplete() {
		m.statusNote = "cannot ignore a completed task"
		if inDetail {
			m.syncDetailSuffix()
			m.taskListComponent.refreshDetailViewport()
		}
		return m, nil
	}
	m.statusNote = ""
	visible := m.visibleFeatures()
	if m.selectedFeature < len(visible) && visible[m.selectedFeature].ignored {
		m.statusNote = "feature is already ignored"
	}
	if err := rewriteTaskHeading(t.SourceFile, t.ID, t.Ignored); err != nil {
		m.statusNote = "error: " + err.Error()
		if inDetail {
			m.syncDetailSuffix()
			m.taskListComponent.refreshDetailViewport()
		}
		return m, nil
	}
	cursorTaskID := t.ID
	m.reloadFeatures()
	for i, st := range m.Tasks {
		if st.ID == cursorTaskID {
			m.Cursor = i
			break
		}
	}
	m.ensureCursorVisible()
	if inDetail {
		if updated := reloadTask(m.Tasks[m.Cursor].SourceFile, m.Tasks[m.Cursor].ID); updated != nil {
			m.Tasks[m.Cursor] = *updated
		}
		m.Detail.exitCriteriaMode()
		m.syncDetailSuffix()
		m.taskListComponent.refreshDetailViewport()
	}
	return m, nil
}

func (m statusModel) handleIgnoreFeature() (tea.Model, tea.Cmd) {
	m.statusNote = ""
	visible := m.visibleFeatures()
	if m.selectedFeature >= len(visible) {
		return m, nil
	}
	f := visible[m.selectedFeature]
	if f.completed {
		return m, nil
	}
	subdir := "features"
	if f.isBug {
		subdir = "bugs"
	}
	fullPath := filepath.Join(m.dir, ".maggus", subdir, f.filename)
	var newPath string
	if f.ignored {
		newPath = strings.TrimSuffix(fullPath, "_ignored.md") + ".md"
	} else {
		newPath = strings.TrimSuffix(fullPath, ".md") + "_ignored.md"
	}
	if err := os.Rename(fullPath, newPath); err != nil {
		m.statusNote = "error: " + err.Error()
		return m, nil
	}
	newBase := filepath.Base(newPath)
	features, err := parseFeatures(m.dir)
	if err == nil {
		bugs, bugErr := parseBugs(m.dir)
		if bugErr == nil {
			features = append(features, bugs...)
		}
		m.features = features
		m.nextTaskID, m.nextTaskFile = findNextTask(features)
	}
	newVisible := m.visibleFeatures()
	m.selectedFeature = 0
	for i, vf := range newVisible {
		if vf.filename == newBase {
			m.selectedFeature = i
			break
		}
	}
	m.rebuildForSelectedFeature()
	return m, nil
}

func (m statusModel) View() string {
	if len(m.features) == 0 {
		return m.viewEmpty()
	}
	if v := m.taskListComponent.View(); v != "" {
		return v
	}
	return m.viewStatus()
}

func (m statusModel) viewEmpty() string {
	mutedStyle := lipgloss.NewStyle().Foreground(styles.Muted)

	var sb strings.Builder
	sb.WriteString(styles.Title.Render("Status") + "\n\n")
	sb.WriteString(mutedStyle.Render("No features found.") + "\n\n")
	sb.WriteString(mutedStyle.Render("Create a feature with ") +
		lipgloss.NewStyle().Bold(true).Foreground(styles.Primary).Render("maggus plan") +
		mutedStyle.Render(" to get started.") + "\n")

	footer := styles.StatusBar.Render("q/esc: exit")

	borderColor := styles.ThemeColor(m.is2x)
	if m.Width > 0 && m.Height > 0 {
		return styles.FullScreenColor(sb.String(), footer, m.Width, m.Height, borderColor)
	}
	return styles.Box.BorderForeground(borderColor).Render(sb.String()) + "\n"
}

// renderTabBar renders the horizontal feature tab bar.
func (m statusModel) renderTabBar() string {
	visible := m.visibleFeatures()
	if len(visible) == 0 {
		return ""
	}

	selectedStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.Primary)
	selectedBugStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.Error)
	unselectedStyle := lipgloss.NewStyle().Foreground(styles.Muted)
	unselectedBugStyle := lipgloss.NewStyle().Foreground(styles.Muted).Faint(true)
	ignoredTabStyle := lipgloss.NewStyle().Foreground(styles.Warning).Faint(true)

	var tabs []string
	needsSep := false
	for i, p := range visible {
		// Insert separator between features and bugs
		if p.isBug && !needsSep {
			needsSep = true
			if len(tabs) > 0 {
				tabs = append(tabs, statusDimStyle.Render(" ┃ "))
			}
		}

		done := p.doneCount()
		total := len(p.tasks)
		name := strings.TrimSuffix(p.filename, ".md")
		prefix := ""
		if p.ignored {
			prefix = "~"
		}
		label := fmt.Sprintf(" %s%s %d/%d ", prefix, name, done, total)
		if i == m.selectedFeature {
			if p.ignored {
				tabs = append(tabs, ignoredTabStyle.Bold(true).Render(label))
			} else if p.isBug {
				tabs = append(tabs, selectedBugStyle.Render(label))
			} else {
				tabs = append(tabs, selectedStyle.Render(label))
			}
		} else {
			if p.ignored {
				tabs = append(tabs, ignoredTabStyle.Render(label))
			} else if p.isBug {
				tabs = append(tabs, unselectedBugStyle.Render(label))
			} else {
				tabs = append(tabs, unselectedStyle.Render(label))
			}
		}
	}

	// Join tabs with a separator, wrapping to next line if needed
	sep := statusDimStyle.Render("│")
	maxWidth := m.Width - 8
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
			sepWidth = 1
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

	visible := m.visibleFeatures()

	// Compute totals
	totalTasks := 0
	totalDone := 0
	totalBlocked := 0
	activeFeatures := 0
	totalBugs := 0
	activeBugs := 0
	for _, f := range m.features {
		totalTasks += len(f.tasks)
		totalDone += f.doneCount()
		totalBlocked += f.blockedCount()
		if f.isBug {
			totalBugs++
			if !f.completed {
				activeBugs++
			}
		} else {
			if !f.completed {
				activeFeatures++
			}
		}
	}
	totalPending := totalTasks - totalDone - totalBlocked
	featureCount := len(m.features) - totalBugs

	// Header
	headerParts := fmt.Sprintf("%d features (%d active)", featureCount, activeFeatures)
	if totalBugs > 0 {
		headerParts += fmt.Sprintf(", %d bugs (%d active)", totalBugs, activeBugs)
	}
	header := styles.Title.Render(fmt.Sprintf("Maggus Status — %s, %d tasks total",
		headerParts, totalTasks))
	sb.WriteString(header)
	sb.WriteString("\n\n")

	// Tab bar
	if len(visible) > 0 {
		sb.WriteString(m.renderTabBar())
		sb.WriteString("\n")
		sb.WriteString(" " + styles.Separator(42))
		sb.WriteString("\n")
	}

	// Progress bar and summary for selected feature
	if m.selectedFeature < len(visible) {
		p := visible[m.selectedFeature]
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

	// Task list for selected feature
	if m.selectedFeature < len(visible) {
		p := visible[m.selectedFeature]

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
		end := m.ScrollOffset + visibleLines
		if end > len(m.Tasks) {
			end = len(m.Tasks)
		}

		for taskIdx := m.ScrollOffset; taskIdx < end; taskIdx++ {
			t := m.Tasks[taskIdx]

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
			if taskIdx == m.Cursor {
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
		if len(m.Tasks) > visibleLines {
			scrollHint := fmt.Sprintf(" [%d-%d of %d]", m.ScrollOffset+1, end, len(m.Tasks))
			sb.WriteString("\n")
			sb.WriteString(statusDimStyle.Render(scrollHint))
		}
	}

	// Status note (e.g. "feature is already ignored")
	if m.statusNote != "" {
		sb.WriteString("\n")
		sb.WriteString(statusDimStyle.Render("  " + m.statusNote))
	}

	toggleHint := "alt+a: show all"
	if m.showAll {
		toggleHint = "alt+a: hide completed"
	}
	footer := styles.StatusBar.Render("tab/shift+tab: switch feature · ↑/↓: navigate · enter: details · " + toggleHint + " · alt+i: ignore/unignore · alt+p: ignore/unignore feature · alt+r: run · alt+bksp: delete · q/esc: exit")

	borderColor := styles.ThemeColor(m.is2x)
	if m.Width > 0 && m.Height > 0 {
		return styles.FullScreenColor(sb.String(), footer, m.Width, m.Height, borderColor)
	}
	return styles.Box.BorderForeground(borderColor).Render(sb.String()+"\n\n"+footer) + "\n"
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show a compact summary of feature progress",
	Long:  `Reads all feature files in .maggus/ and displays a compact progress summary.`,
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

		features, err := parseFeatures(dir)
		if err != nil {
			return err
		}
		bugs, bugErr := parseBugs(dir)
		if bugErr != nil {
			return bugErr
		}
		features = append(features, bugs...)

		if len(features) == 0 {
			if plain {
				fmt.Fprintln(cmd.OutOrStdout(), "No features found.")
				return nil
			}
			// TUI mode: show empty status view
			features = []featureInfo{}
		}

		nextTaskID, nextTaskFile := findNextTask(features)

		if plain {
			var sb strings.Builder
			renderStatusPlain(&sb, features, all, nextTaskID, nextTaskFile, agentName)
			fmt.Fprint(cmd.OutOrStdout(), sb.String())
			return nil
		}

		// TUI mode: interactive status with detail view
		m := newStatusModel(features, all, nextTaskID, nextTaskFile, agentName, dir)
		prog := tea.NewProgram(m, tea.WithAltScreen())
		result, err := prog.Run()
		if err != nil {
			return err
		}
		if final, ok := result.(statusModel); ok && final.RunTaskID != "" {
			return dispatchWork(final.RunTaskID)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
	statusCmd.Flags().Bool("plain", false, "Strip colors and use ASCII characters for scripting/piping")
	statusCmd.Flags().Bool("all", false, "Show completed features in task sections and Features table")
}
