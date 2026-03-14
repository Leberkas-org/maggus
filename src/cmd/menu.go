package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/leberkas-org/maggus/internal/tui/styles"
)

// menuItem represents a single entry in the main menu.
type menuItem struct {
	name              string
	desc              string
	requiresClaude    bool
	hideIfInitialized bool
}

var allMenuItems = []menuItem{
	{name: "work", desc: "Work on the next N tasks from the implementation plan"},
	{name: "plan", desc: "Open an interactive AI session to create a plan", requiresClaude: true},
	{name: "vision", desc: "Open an interactive AI session to create VISION.md", requiresClaude: true},
	{name: "architecture", desc: "Open an interactive AI session to create ARCHITECTURE.md", requiresClaude: true},
	{name: "list", desc: "Preview the next N upcoming workable tasks"},
	{name: "status", desc: "Show a compact summary of plan progress"},
	{name: "blocked", desc: "Interactive wizard to manage blocked tasks"},
	{name: "clean", desc: "Remove completed plan files and finished run directories"},
	{name: "release", desc: "Generate RELEASE.md with changelog and AI summary"},
	{name: "worktree", desc: "Manage Maggus worktrees"},
	{name: "config", desc: "Edit project settings interactively"},
	{name: "init", desc: "Initialize a .maggus project in the current directory", hideIfInitialized: true},
}

// activeMenuItems returns the menu items filtered by available capabilities.
func activeMenuItems() []menuItem {
	initialized := isInitialized()
	var items []menuItem
	for _, item := range allMenuItems {
		if item.requiresClaude && !caps.HasClaude {
			continue
		}
		if item.hideIfInitialized && initialized {
			continue
		}
		items = append(items, item)
	}
	return items
}

// isInitialized returns true if the .maggus/ directory exists in the current working directory.
func isInitialized() bool {
	dir, err := os.Getwd()
	if err != nil {
		return false
	}
	info, err := os.Stat(filepath.Join(dir, ".maggus"))
	return err == nil && info.IsDir()
}

// subMenuOption represents a configurable option in a command's sub-menu.
type subMenuOption struct {
	label   string
	values  []string
	current int // index into values
}

// subMenuDef defines the sub-menu for a command.
type subMenuDef struct {
	options []subMenuOption
}

// buildSubMenus returns sub-menu definitions keyed by command name.
func buildSubMenus() map[string]subMenuDef {
	return map[string]subMenuDef{
		"work": {options: []subMenuOption{
			{label: "Tasks", values: []string{"1", "3", "5", "10", "all"}, current: 1},
			{label: "Worktree", values: []string{"off", "on"}, current: 0},
		}},
		"list": {options: []subMenuOption{
			{label: "Count", values: []string{"5", "10", "20", "all"}, current: 0},
		}},
		"status": {options: []subMenuOption{
			{label: "All", values: []string{"off", "on"}, current: 0},
		}},
		"worktree": {options: []subMenuOption{
			{label: "Action", values: []string{"list", "clean"}, current: 0},
		}},
	}
}

// buildArgs converts the sub-menu selections into CLI args for the command.
func buildArgs(cmdName string, opts []subMenuOption) []string {
	switch cmdName {
	case "work":
		var args []string
		// Tasks option
		if opts[0].values[opts[0].current] == "all" {
			args = append(args, "--count", "999")
		} else {
			args = append(args, "--count", opts[0].values[opts[0].current])
		}
		// Worktree option
		if opts[1].values[opts[1].current] == "on" {
			args = append(args, "--worktree")
		}
		return args
	case "list":
		var args []string
		if opts[0].values[opts[0].current] == "all" {
			args = append(args, "--all")
		} else {
			args = append(args, "--count", opts[0].values[opts[0].current])
		}
		return args
	case "status":
		var args []string
		if opts[0].values[opts[0].current] == "on" {
			args = append(args, "--all")
		}
		return args
	case "worktree":
		return []string{opts[0].values[opts[0].current]}
	}
	return nil
}

// planSummary holds the aggregated plan statistics for the menu header.
type planSummary struct {
	plans   int
	tasks   int
	done    int
	blocked int
}

// loadPlanSummary computes plan statistics from the current working directory.
func loadPlanSummary() planSummary {
	dir, err := os.Getwd()
	if err != nil {
		return planSummary{}
	}
	plans, err := parsePlans(dir)
	if err != nil || len(plans) == 0 {
		return planSummary{}
	}
	var s planSummary
	s.plans = len(plans)
	for _, p := range plans {
		s.tasks += len(p.tasks)
		s.done += p.doneCount()
		s.blocked += p.blockedCount()
	}
	return s
}

// menuModel is the bubbletea model for the interactive main menu.
type menuModel struct {
	items    []menuItem
	cursor   int
	selected string   // command name chosen by the user, empty if quit
	args     []string // args to pass to the selected command
	quitting bool
	summary  planSummary
	width    int
	height   int

	// Sub-menu state
	inSubMenu    bool
	subCursor    int // cursor within sub-menu (options + Run item)
	subMenuDefs  map[string]subMenuDef
	activeSubDef *subMenuDef // pointer to the active sub-menu definition (with live option state)
}

func newMenuModel(summary planSummary) menuModel {
	return menuModel{
		items:       activeMenuItems(),
		summary:     summary,
		subMenuDefs: buildSubMenus(),
	}
}

func (m menuModel) Init() tea.Cmd {
	return nil
}

func (m menuModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		if m.inSubMenu {
			return m.updateSubMenu(msg)
		}
		return m.updateMainMenu(msg)
	}
	return m, nil
}

func (m menuModel) updateMainMenu(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc", "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		} else {
			m.cursor = len(m.items) - 1
		}
	case "down", "j":
		if m.cursor < len(m.items)-1 {
			m.cursor++
		} else {
			m.cursor = 0
		}
	case "home":
		m.cursor = 0
	case "end":
		m.cursor = len(m.items) - 1
	case "enter":
		name := m.items[m.cursor].name
		if def, ok := m.subMenuDefs[name]; ok {
			// Deep copy the sub-menu def so each entry resets
			copied := subMenuDef{options: make([]subMenuOption, len(def.options))}
			for i, opt := range def.options {
				copied.options[i] = subMenuOption{
					label:   opt.label,
					values:  opt.values,
					current: opt.current,
				}
			}
			m.activeSubDef = &copied
			m.inSubMenu = true
			m.subCursor = 0
			return m, nil
		}
		// No sub-menu — launch directly
		m.selected = name
		return m, tea.Quit
	}
	return m, nil
}

func (m menuModel) updateSubMenu(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	itemCount := len(m.activeSubDef.options) + 1 // options + Run

	switch msg.String() {
	case "esc":
		m.inSubMenu = false
		m.activeSubDef = nil
		m.subCursor = 0
		return m, nil
	case "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "up", "k":
		if m.subCursor > 0 {
			m.subCursor--
		} else {
			m.subCursor = itemCount - 1
		}
	case "down", "j":
		if m.subCursor < itemCount-1 {
			m.subCursor++
		} else {
			m.subCursor = 0
		}
	case "home":
		m.subCursor = 0
	case "end":
		m.subCursor = itemCount - 1
	case "left", "h":
		if m.subCursor < len(m.activeSubDef.options) {
			opt := &m.activeSubDef.options[m.subCursor]
			if opt.current > 0 {
				opt.current--
			} else {
				opt.current = len(opt.values) - 1
			}
		}
	case "right", "l":
		if m.subCursor < len(m.activeSubDef.options) {
			opt := &m.activeSubDef.options[m.subCursor]
			if opt.current < len(opt.values)-1 {
				opt.current++
			} else {
				opt.current = 0
			}
		}
	case "enter":
		if m.subCursor == len(m.activeSubDef.options) {
			// "Run" selected
			name := m.items[m.cursor].name
			m.selected = name
			m.args = buildArgs(name, m.activeSubDef.options)
			return m, tea.Quit
		}
		// On an option row: cycle value forward
		opt := &m.activeSubDef.options[m.subCursor]
		if opt.current < len(opt.values)-1 {
			opt.current++
		} else {
			opt.current = 0
		}
	}
	return m, nil
}

const logo = `
 ███╗   ███╗  █████╗   ██████╗  ██████╗  ██╗   ██╗ ███████╗
 ████╗ ████║ ██╔══██╗ ██╔════╝ ██╔════╝  ██║   ██║ ██╔════╝
 ██╔████╔██║ ███████║ ██║  ███╗██║  ███╗ ██║   ██║ ███████╗
 ██║╚██╔╝██║ ██╔══██║ ██║   ██║██║   ██║ ██║   ██║ ╚════██║
 ██║ ╚═╝ ██║ ██║  ██║ ╚██████╔╝╚██████╔╝ ╚██████╔╝ ███████║
 ╚═╝     ╚═╝ ╚═╝  ╚═╝  ╚═════╝  ╚═════╝   ╚═════╝  ╚══════╝`

func (m menuModel) View() string {
	logoStyle := lipgloss.NewStyle().Foreground(styles.Primary).Bold(true)
	versionStyle := lipgloss.NewStyle().Foreground(styles.Muted)
	header := logoStyle.Render(logo) + "\n" + versionStyle.Render(fmt.Sprintf("  v%s — Markdown Agent for Goal-Gated Unsupervised Sprints", Version))

	// Plan summary line
	mutedStyle := lipgloss.NewStyle().Foreground(styles.Muted)
	var summaryLine string
	if m.summary.tasks == 0 {
		summaryLine = mutedStyle.Render("No plans found")
	} else {
		greenStyle := lipgloss.NewStyle().Foreground(styles.Success)
		redStyle := lipgloss.NewStyle().Foreground(styles.Error)
		summaryLine = fmt.Sprintf("%s · %s · %s · %s",
			mutedStyle.Render(fmt.Sprintf("%d plans", m.summary.plans)),
			mutedStyle.Render(fmt.Sprintf("%d tasks", m.summary.tasks)),
			greenStyle.Render(fmt.Sprintf("%d done", m.summary.done)),
			redStyle.Render(fmt.Sprintf("%d blocked", m.summary.blocked)),
		)
	}

	var body, footer string
	if m.inSubMenu {
		body, footer = m.viewSubMenu()
	} else {
		body, footer = m.viewMainMenu()
	}

	content := header + "\n" + summaryLine + "\n\n" + body

	if m.width > 0 && m.height > 0 {
		return styles.FullScreen(content, footer, m.width, m.height)
	}
	return styles.Box.Render(content+"\n\n"+footer) + "\n"
}

func (m menuModel) viewMainMenu() (string, string) {
	selectedStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.Primary)
	cursorStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.Primary)
	descStyle := lipgloss.NewStyle().Foreground(styles.Muted)
	normalStyle := lipgloss.NewStyle()

	var sb strings.Builder
	for i, item := range m.items {
		if i == m.cursor {
			fmt.Fprintf(&sb, "  %s %s  %s\n",
				cursorStyle.Render("→"),
				selectedStyle.Render(item.name),
				descStyle.Render(item.desc),
			)
		} else {
			fmt.Fprintf(&sb, "    %s  %s\n",
				normalStyle.Render(item.name),
				descStyle.Render(item.desc),
			)
		}
	}

	footer := styles.StatusBar.Render("↑/↓: navigate · enter: select · q/esc: exit")
	return sb.String(), footer
}

func (m menuModel) viewSubMenu() (string, string) {
	selectedStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.Primary)
	cursorStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.Primary)
	mutedStyle := lipgloss.NewStyle().Foreground(styles.Muted)
	normalStyle := lipgloss.NewStyle()
	activeValueStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.Success)

	cmdName := m.items[m.cursor].name
	titleLine := selectedStyle.Render(cmdName) + "  " + mutedStyle.Render(m.items[m.cursor].desc)

	var sb strings.Builder
	sb.WriteString(titleLine + "\n")
	sb.WriteString(styles.Separator(40) + "\n")

	for i, opt := range m.activeSubDef.options {
		label := fmt.Sprintf("%-10s", opt.label)

		// Render value choices
		var valueParts []string
		for vi, v := range opt.values {
			if vi == opt.current {
				valueParts = append(valueParts, activeValueStyle.Render(v))
			} else {
				valueParts = append(valueParts, mutedStyle.Render(v))
			}
		}
		valueStr := strings.Join(valueParts, mutedStyle.Render(" / "))

		if i == m.subCursor {
			fmt.Fprintf(&sb, "  %s %s  %s\n",
				cursorStyle.Render("→"),
				normalStyle.Render(label),
				valueStr,
			)
		} else {
			fmt.Fprintf(&sb, "    %s  %s\n",
				normalStyle.Render(label),
				valueStr,
			)
		}
	}

	// Run item
	runIdx := len(m.activeSubDef.options)
	sb.WriteString("\n")
	if m.subCursor == runIdx {
		fmt.Fprintf(&sb, "  %s %s\n",
			cursorStyle.Render("→"),
			selectedStyle.Render("Run"),
		)
	} else {
		fmt.Fprintf(&sb, "    %s\n",
			normalStyle.Render("Run"),
		)
	}

	footer := styles.StatusBar.Render("↑/↓: navigate · ←/→: change value · enter: select/run · esc: back")
	return sb.String(), footer
}
