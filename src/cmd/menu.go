package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/leberkas-org/maggus/internal/claude2x"
	"github.com/leberkas-org/maggus/internal/globalconfig"
	"github.com/leberkas-org/maggus/internal/tui/styles"
)

// claude2xResultMsg carries the result of the async 2x status fetch.
type claude2xResultMsg struct {
	status claude2x.Status
}

// updateCheckResultMsg carries the result of the async startup update check.
type updateCheckResultMsg struct {
	banner string // styled one-line banner text to show in the menu (empty = nothing to show)
}

// loadSettings is injectable for testing.
var loadSettings = func() (globalconfig.Settings, error) {
	return globalconfig.LoadSettings()
}

// loadUpdateState is injectable for testing.
var loadUpdateState = func() (globalconfig.UpdateState, error) {
	return globalconfig.LoadUpdateState()
}

// saveUpdateState is injectable for testing.
var saveUpdateState = func(state globalconfig.UpdateState) error {
	return globalconfig.SaveUpdateState(state)
}

// timeNow is injectable for testing.
var timeNow = time.Now

// startupUpdateCheck runs the update check logic based on global config.
// Returns a banner string for notify mode, an applied-update message for auto mode,
// or empty string for off mode / no update / dev build / cooldown not passed.
func startupUpdateCheck() string {
	if Version == "dev" {
		return ""
	}

	settings, err := loadSettings()
	if err != nil || settings.AutoUpdate == globalconfig.AutoUpdateOff {
		return ""
	}

	state, err := loadUpdateState()
	if err != nil {
		return ""
	}

	now := timeNow()
	if !globalconfig.ShouldCheckUpdate(state, now) {
		return ""
	}

	info := checkLatestVersion(Version)

	// Update the last check timestamp regardless of result.
	_ = saveUpdateState(globalconfig.UpdateState{LastUpdateCheck: now})

	if !info.IsNewer {
		return ""
	}

	switch settings.AutoUpdate {
	case globalconfig.AutoUpdateNotify:
		return fmt.Sprintf("Update available: v%s → %s — run `maggus update` to install",
			strings.TrimPrefix(Version, "v"), info.TagName)
	case globalconfig.AutoUpdateAuto:
		if info.DownloadURL == "" {
			return ""
		}
		if err := applyUpdate(info.DownloadURL); err != nil {
			return ""
		}
		return fmt.Sprintf("Updated to %s — restart maggus to use the new version", info.TagName)
	}

	return ""
}

// menuItem represents a single entry in the main menu.
type menuItem struct {
	name              string
	desc              string
	requiresClaude    bool
	hideIfInitialized bool
	separator         bool // render a blank line before this item
	isExit            bool // quit the menu instead of dispatching a command
}

var allMenuItems = []menuItem{
	// Group 1: Core workflow
	{name: "work", desc: "Work on the next N tasks from the implementation plan"},
	{name: "status", desc: "Show a compact summary of plan progress"},
	{name: "list", desc: "Preview upcoming workable tasks"},
	// Group 2: AI-assisted creation
	{name: "vision", desc: "Create or improve VISION.md", requiresClaude: true, separator: true},
	{name: "architecture", desc: "Create or improve ARCHITECTURE.md", requiresClaude: true},
	{name: "plan", desc: "Create an implementation plan", requiresClaude: true},
	// Group 3: Project management
	{name: "config", desc: "Edit project settings", separator: true},
	{name: "worktree", desc: "Manage Maggus worktrees"},
	{name: "release", desc: "Generate RELEASE.md with changelog"},
	{name: "clean", desc: "Remove completed plans and finished runs"},
	{name: "update", desc: "Check for and install updates"},
	{name: "init", desc: "Initialize a .maggus project", hideIfInitialized: true},
	// Group 4: Repository management
	{name: "repos", desc: "Manage configured repositories", separator: true},
	// Exit
	{name: "exit", desc: "Exit Maggus", separator: true, isExit: true},
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
		// Don't start with a separator if this is the first visible item.
		if item.separator && len(items) == 0 {
			item.separator = false
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
	items         []menuItem
	cursor        int
	selected      string   // command name chosen by the user, empty if quit
	args          []string // args to pass to the selected command
	quitting      bool
	summary       planSummary
	width         int
	height        int
	cwd           string // current working directory, shown in header
	is2x          bool   // true when Claude is in 2x mode (logo/border turn yellow)
	twoXExpiresIn string // e.g. "17h 54m 44s" — only set when is2x is true
	updateBanner  string // one-line update notification shown below summary

	// Sub-menu state
	inSubMenu    bool
	subCursor    int // cursor within sub-menu (options + Run item)
	subMenuDefs  map[string]subMenuDef
	activeSubDef *subMenuDef // pointer to the active sub-menu definition (with live option state)
}

func newMenuModel(summary planSummary) menuModel {
	cwd, _ := os.Getwd()
	return menuModel{
		items:       activeMenuItems(),
		summary:     summary,
		cwd:         cwd,
		subMenuDefs: buildSubMenus(),
	}
}

func (m menuModel) Init() tea.Cmd {
	return tea.Batch(
		func() tea.Msg {
			return claude2xResultMsg{status: claude2x.FetchStatus()}
		},
		func() tea.Msg {
			return updateCheckResultMsg{banner: startupUpdateCheck()}
		},
	)
}

func (m menuModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case claude2xResultMsg:
		m.is2x = msg.status.Is2x
		m.twoXExpiresIn = msg.status.TwoXWindowExpiresIn
		return m, nil
	case updateCheckResultMsg:
		m.updateBanner = msg.banner
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
		item := m.items[m.cursor]
		if item.isExit {
			m.quitting = true
			return m, tea.Quit
		}
		if def, ok := m.subMenuDefs[item.name]; ok {
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
		m.selected = item.name
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
	themeColor := styles.ThemeColor(m.is2x)
	logoStyle := lipgloss.NewStyle().Foreground(themeColor).Bold(true)
	versionStyle := lipgloss.NewStyle().Foreground(styles.Muted)
	versionLine := versionStyle.Render(fmt.Sprintf("v%s — Markdown Agent for Goal-Gated Unsupervised Sprints", Version))

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

	// Center the logo, version, and summary lines within the content column.
	// FullScreen left-pads all content into a maxContentWidth (90) column,
	// so center relative to that width, not the full inner width.
	const contentW = 90
	styledLogo := logoStyle.Render(logo)
	header := centerBlock(styledLogo, contentW) + "\n" +
		centerLine(versionLine, contentW) + "\n" +
		centerLine(summaryLine, contentW)

	// Show current working directory below the summary
	if m.cwd != "" {
		cwdStyle := lipgloss.NewStyle().Foreground(styles.Muted)
		cwdDisplay := m.cwd
		// Only truncate if this is a git repo and not the home directory.
		if home, err := os.UserHomeDir(); err != nil || (m.cwd != home && isGitRepoCheck(m.cwd)) {
			cwdDisplay = truncateLeft(m.cwd, contentW-4)
		}
		header += "\n" + centerLine(cwdStyle.Render(cwdDisplay), contentW)
	}

	// Show 2x remaining time below the summary when active
	if m.is2x && m.twoXExpiresIn != "" {
		twoXStyle := lipgloss.NewStyle().Foreground(styles.Warning).Bold(true)
		twoXLine := twoXStyle.Render(fmt.Sprintf("2x expires in: %s", m.twoXExpiresIn))
		header += "\n" + centerLine(twoXLine, contentW)
	}

	// Show update banner when available
	if m.updateBanner != "" {
		updateStyle := lipgloss.NewStyle().Foreground(styles.Success).Bold(true)
		header += "\n" + centerLine(updateStyle.Render(m.updateBanner), contentW)
	}

	content := header + "\n\n" + body

	if m.width > 0 && m.height > 0 {
		return styles.FullScreenColor(content, footer, m.width, m.height, themeColor)
	}
	return styles.Box.BorderForeground(themeColor).Render(content+"\n\n"+footer) + "\n"
}

// centerLine centers a single line of text within the given width.
func centerLine(line string, width int) string {
	w := lipgloss.Width(line)
	pad := (width - w) / 2
	if pad < 0 {
		pad = 0
	}
	return strings.Repeat(" ", pad) + line
}

// centerBlock centers each line of a multi-line string independently.
func centerBlock(block string, width int) string {
	lines := strings.Split(strings.TrimRight(block, "\n"), "\n")
	for i, line := range lines {
		lines[i] = centerLine(line, width)
	}
	return strings.Join(lines, "\n")
}

// truncateLeft truncates a path from the left, adding "..." prefix.
func truncateLeft(path string, maxWidth int) string {
	if maxWidth <= 0 || len(path) <= maxWidth {
		return path
	}
	if maxWidth <= 3 {
		return path[len(path)-maxWidth:]
	}
	return "..." + path[len(path)-(maxWidth-3):]
}

func (m menuModel) viewMainMenu() (string, string) {
	selectedStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.Primary)
	cursorStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.Primary)
	descStyle := lipgloss.NewStyle().Foreground(styles.Muted)
	normalStyle := lipgloss.NewStyle()

	// Measure columns: left = "→ " + name, right = desc
	maxNameW := 0
	maxDescW := 0
	for _, item := range m.items {
		if len(item.name) > maxNameW {
			maxNameW = len(item.name)
		}
		if len(item.desc) > maxDescW {
			maxDescW = len(item.desc)
		}
	}

	// Total row width: cursor(4) + name(maxNameW) + gap(2) + desc(maxDescW)
	// cursor column is "  → " (4 chars) for selected, "    " (4 chars) for others
	const cursorCol = 4
	const gap = 2
	tableW := cursorCol + maxNameW + gap + maxDescW

	// Center the table within the content column (90 chars, matching FullScreen)
	const contentW = 90
	leftPad := (contentW - tableW) / 2
	if leftPad < 0 {
		leftPad = 0
	}
	indent := strings.Repeat(" ", leftPad)

	var sb strings.Builder
	for i, item := range m.items {
		if item.separator {
			sb.WriteString("\n")
		}
		// Right-align the name within the column.
		padded := fmt.Sprintf("%*s", maxNameW, item.name)
		if i == m.cursor {
			nameStyle := selectedStyle
			cursor := cursorStyle
			if item.isExit {
				nameStyle = lipgloss.NewStyle().Bold(true).Foreground(styles.Error)
				cursor = nameStyle
			}
			fmt.Fprintf(&sb, "%s%s %s  %s\n",
				indent,
				cursor.Render("→"),
				nameStyle.Render(padded),
				descStyle.Render(item.desc),
			)
		} else {
			fmt.Fprintf(&sb, "%s  %s  %s\n",
				indent,
				normalStyle.Render(padded),
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
