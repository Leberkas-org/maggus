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
	"github.com/leberkas-org/maggus/internal/config"
	"github.com/leberkas-org/maggus/internal/filewatcher"
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

// hideShortcutsMsg is sent after a delay to hide the shortcut underlines.
type hideShortcutsMsg struct {
	timerID int // only hide if this matches the current timer ID
}

// autoWorkTickMsg is sent every second while the delayed auto-work countdown is active.
type autoWorkTickMsg struct {
	id int // countdown ID; stale ticks (from a previous countdown) are ignored
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
	if strings.HasPrefix(Version, "dev") {
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
	shortcut          rune   // keyboard shortcut (0 = none)
	shortcutLabel     string // display label for the shortcut (e.g. "w", "s")
	requiresClaude    bool
	hideIfInitialized bool
	separator         bool // render a blank line before this item
	isExit            bool // quit the menu instead of dispatching a command
}

var allMenuItems = []menuItem{
	// Core workflow
	{name: "work", desc: "Work through all tasks in the feature", shortcut: 'w', shortcutLabel: "w"},
	{name: "status", desc: "Show a compact summary of feature progress", shortcut: 's', shortcutLabel: "s"},
	{name: "repos", desc: "Manage configured repositories", shortcut: 'r', shortcutLabel: "r"},
	// AI-assisted creation
	{name: "prompt", desc: "Launch interactive Claude session with usage tracking", requiresClaude: true, separator: true, shortcut: 'o', shortcutLabel: "o"},
	{name: "vision", desc: "Create or improve VISION.md", requiresClaude: true, shortcut: 'v', shortcutLabel: "v"},
	{name: "architecture", desc: "Create or improve ARCHITECTURE.md", requiresClaude: true, shortcut: 'a', shortcutLabel: "a"},
	{name: "plan", desc: "Create an implementation plan", requiresClaude: true, shortcut: 'p', shortcutLabel: "p"},
	// Project management
	{name: "release", desc: "Generate RELEASE.md with changelog", separator: true, shortcut: 'z', shortcutLabel: "z"},
	{name: "clean", desc: "Remove completed features and finished runs", shortcut: 'n', shortcutLabel: "n"},
	{name: "update", desc: "Check for and install updates", shortcut: 'u', shortcutLabel: "u"},
	// Group 5: Confguration
	{name: "config", desc: "Edit project settings", separator: true, shortcut: 'c', shortcutLabel: "c"},
	{name: "worktree", desc: "Manage Maggus worktrees", shortcut: 't', shortcutLabel: "t"},
	{name: "init", desc: "Initialize a .maggus project", hideIfInitialized: true, shortcut: 'i', shortcutLabel: "i"},
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
		"worktree": {options: []subMenuOption{
			{label: "Action", values: []string{"list", "clean"}, current: 0},
		}},
	}
}

// buildArgs converts the sub-menu selections into CLI args for the command.
func buildArgs(cmdName string, opts []subMenuOption) []string {
	switch cmdName {
	case "worktree":
		return []string{opts[0].values[opts[0].current]}
	}
	return nil
}

// featureSummaryUpdateMsg is sent when the file watcher detects changes
// to feature or bug files, triggering a summary reload.
// HasNewFile mirrors filewatcher.UpdateMsg.HasNewFile: true when a Create
// event was seen in the debounce window.
type featureSummaryUpdateMsg struct {
	HasNewFile bool
}

// featureSummary holds the aggregated feature and bug statistics for the menu header.
type featureSummary struct {
	features int
	tasks    int
	done     int
	blocked  int
	workable int

	bugs        int
	bugTasks    int
	bugDone     int
	bugBlocked  int
	bugWorkable int
}

// loadFeatureSummary computes feature and bug statistics from the current working directory.
func loadFeatureSummary() featureSummary {
	dir, err := os.Getwd()
	if err != nil {
		return featureSummary{}
	}

	var s featureSummary

	features, err := parseFeatures(dir)
	if err == nil {
		s.features = len(features)
		for _, f := range features {
			s.tasks += len(f.tasks)
			s.done += f.doneCount()
			s.blocked += f.blockedCount()
			for _, t := range f.tasks {
				if t.IsWorkable() {
					s.workable++
				}
			}
		}
	}

	bugs, err := parseBugs(dir)
	if err == nil {
		s.bugs = len(bugs)
		for _, b := range bugs {
			s.bugTasks += len(b.tasks)
			s.bugDone += b.doneCount()
			s.bugBlocked += b.blockedCount()
			for _, t := range b.tasks {
				if t.IsWorkable() {
					s.bugWorkable++
				}
			}
		}
	}

	return s
}

// formatSummaryLine builds a lipgloss-styled summary string for the menu header.
// Format: "3 features, 5 open tasks · 2 bugs, 3 open tasks"
// Feature open count is green, bug open count is red, surrounding text is muted.
func formatSummaryLine(s featureSummary) string {
	mutedStyle := lipgloss.NewStyle().Foreground(styles.Muted)
	greenStyle := lipgloss.NewStyle().Foreground(styles.Success)
	redStyle := lipgloss.NewStyle().Foreground(styles.Error)

	if s.workable == 0 && s.bugWorkable == 0 {
		return mutedStyle.Render("No open tasks")
	}

	var parts []string

	if s.features > 0 {
		parts = append(parts, mutedStyle.Render(fmt.Sprintf("%d features, ", s.features))+
			greenStyle.Render(fmt.Sprintf("%d", s.workable))+
			mutedStyle.Render(" open tasks"))
	}

	if s.bugs > 0 {
		parts = append(parts, mutedStyle.Render(fmt.Sprintf("%d bugs, ", s.bugs))+
			redStyle.Render(fmt.Sprintf("%d", s.bugWorkable))+
			mutedStyle.Render(" open tasks"))
	}

	return strings.Join(parts, mutedStyle.Render(" · "))
}

// menuModel is the bubbletea model for the interactive main menu.
type menuModel struct {
	items           []menuItem
	cursor          int
	selected        string   // command name chosen by the user, empty if quit
	args            []string // args to pass to the selected command
	quitting        bool
	summary         featureSummary
	width           int
	height          int
	cwd             string // current working directory, shown in header
	is2x            bool   // true when Claude is in 2x mode (logo/border turn yellow)
	twoXExpiresIn   string // e.g. "17h 54m 44s" — only set when is2x is true
	updateBanner    string // one-line update notification shown below summary
	showShortcuts   bool   // true while alt is held — underlines shortcut chars
	shortcutTimerID int    // monotonic counter to identify the latest hide timer

	// Auto-work state
	autoWork            string // "disabled" | "enabled" | "delayed" — cached from config
	autoWorkActive      bool   // true while the delayed countdown is running
	autoWorkCountdown   int    // seconds remaining (5 → 0)
	autoWorkCountdownID int    // monotonic counter to discard stale tick messages

	// File watcher for live summary updates
	watcher   *filewatcher.Watcher
	watcherCh chan bool

	// Sub-menu state
	inSubMenu    bool
	subCursor    int // cursor within sub-menu (options + Run item)
	subMenuDefs  map[string]subMenuDef
	activeSubDef *subMenuDef // pointer to the active sub-menu definition (with live option state)
}

func newMenuModel(summary featureSummary) menuModel {
	cwd, _ := os.Getwd()

	autoWork := config.AutoWorkDisabled
	if cfg, err := config.Load(cwd); err == nil {
		autoWork = cfg.AutoWork
	}

	ch := make(chan bool, 1)
	w, _ := filewatcher.New(cwd, func(msg any) {
		hasNew := false
		if u, ok := msg.(filewatcher.UpdateMsg); ok {
			hasNew = u.HasNewFile
		}
		select {
		case ch <- hasNew:
		default: // don't block if channel already has a pending update
		}
	}, 300*time.Millisecond)

	return menuModel{
		items:       activeMenuItems(),
		summary:     summary,
		cwd:         cwd,
		autoWork:    autoWork,
		subMenuDefs: buildSubMenus(),
		watcher:     w,
		watcherCh:   ch,
	}
}

// autoWorkTick returns a Cmd that fires an autoWorkTickMsg after one second.
// The id parameter is used to discard ticks from superseded countdowns.
func autoWorkTick(id int) tea.Cmd {
	return tea.Tick(time.Second, func(_ time.Time) tea.Msg {
		return autoWorkTickMsg{id: id}
	})
}

// listenForWatcherUpdate returns a Cmd that blocks until the watcher channel
// signals a file change, then delivers a featureSummaryUpdateMsg.
func listenForWatcherUpdate(ch <-chan bool) tea.Cmd {
	if ch == nil {
		return nil
	}
	return func() tea.Msg {
		hasNew, ok := <-ch
		if !ok {
			return nil // channel closed, watcher stopped
		}
		return featureSummaryUpdateMsg{HasNewFile: hasNew}
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
		listenForWatcherUpdate(m.watcherCh),
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
		if m.is2x {
			return m, next2xTick()
		}
		return m, nil
	case claude2xTickMsg:
		is2x, expiresIn, tickCmd := fetch2xAndUpdate()
		m.is2x = is2x
		m.twoXExpiresIn = expiresIn
		return m, tickCmd
	case updateCheckResultMsg:
		m.updateBanner = msg.banner
		return m, nil
	case featureSummaryUpdateMsg:
		m.summary = loadFeatureSummary()
		// Re-read config so changes made in the config TUI take effect immediately.
		if cwd, err := os.Getwd(); err == nil {
			if cfg, err := config.Load(cwd); err == nil {
				m.autoWork = cfg.AutoWork
			}
		}
		// Only trigger auto-work when a new file was created; pure Write/Remove/Rename
		// events refresh the summary display but must not dispatch work.
		workable := m.summary.workable + m.summary.bugWorkable
		if msg.HasNewFile && workable > 0 {
			switch m.autoWork {
			case config.AutoWorkEnabled:
				m.selected = "work"
				m.args = []string{"--count", "0"}
				return m, tea.Quit
			case config.AutoWorkDelayed:
				if !m.autoWorkActive {
					m.autoWorkActive = true
					m.autoWorkCountdown = 5
					m.autoWorkCountdownID++
					return m, tea.Batch(
						listenForWatcherUpdate(m.watcherCh),
						autoWorkTick(m.autoWorkCountdownID),
					)
				}
			}
		}
		return m, listenForWatcherUpdate(m.watcherCh)

	case autoWorkTickMsg:
		if !m.autoWorkActive || msg.id != m.autoWorkCountdownID {
			return m, nil
		}
		m.autoWorkCountdown--
		if m.autoWorkCountdown <= 0 {
			m.autoWorkActive = false
			m.selected = "work"
			m.args = []string{"--count", "0"}
			return m, tea.Quit
		}
		return m, autoWorkTick(m.autoWorkCountdownID)

	case hideShortcutsMsg:
		// Only hide if this timer is still the latest one
		if msg.timerID == m.shortcutTimerID {
			m.showShortcuts = false
		}
		return m, nil

	case tea.KeyMsg:
		// Any key press cancels an active auto-work countdown.
		if m.autoWorkActive {
			m.autoWorkActive = false
			m.autoWorkCountdown = 0
			m.autoWorkCountdownID++
			if msg.String() == "ctrl+c" {
				m.quitting = true
				return m, tea.Quit
			}
			return m, nil
		}

		if msg.Alt {
			// Show shortcuts and schedule auto-hide
			m.showShortcuts = true
			m.shortcutTimerID++
			timerID := m.shortcutTimerID
			hideCmd := tea.Tick(1500*time.Millisecond, func(_ time.Time) tea.Msg {
				return hideShortcutsMsg{timerID: timerID}
			})

			// Alt+key shortcuts (main menu only, not sub-menu)
			if !m.inSubMenu && len(msg.Runes) == 1 {
				r := msg.Runes[0]
				for i, item := range m.items {
					if item.shortcut != 0 && item.shortcut == r {
						m.cursor = i
						return m.activateItem(item)
					}
				}
			}
			return m, hideCmd
		}

		// Non-alt key: hide shortcuts immediately
		m.showShortcuts = false

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
	case "up":
		if m.cursor > 0 {
			m.cursor--
		} else {
			m.cursor = len(m.items) - 1
		}
	case "down":
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
		return m.activateItem(m.items[m.cursor])
	}
	return m, nil
}

// activateItem handles selecting a menu item (enter or shortcut).
func (m menuModel) activateItem(item menuItem) (tea.Model, tea.Cmd) {
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
	if item.name == "work" {
		m.args = []string{"--count", "999"}
	}
	return m, tea.Quit
}

func (m menuModel) updateSubMenu(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	itemCount := len(m.activeSubDef.options) + 1 // options + Run

	switch msg.String() {
	case "esc", "q":
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

	// Feature & bug summary line (pre-styled by formatSummaryLine)
	summaryLine := formatSummaryLine(m.summary)

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
		cwdStyle := lipgloss.NewStyle().Foreground(styles.Primary).Bold(true)
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

	// Show auto-work countdown when active
	if m.autoWorkActive {
		countdownStyle := lipgloss.NewStyle().Foreground(styles.Warning).Bold(true)
		countdownLine := countdownStyle.Render(fmt.Sprintf("Auto-work starting in %ds… press any key to cancel", m.autoWorkCountdown))
		header += "\n" + centerLine(countdownLine, contentW)
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

// highlightShortcut renders the name with the shortcut character underlined
// when active is true. Otherwise renders the full name with the base style.
func highlightShortcut(name string, shortcut rune, base lipgloss.Style, active bool) string {
	if !active || shortcut == 0 {
		return base.Render(name)
	}
	underline := base.Underline(true)
	for i, ch := range name {
		if ch == shortcut {
			before := name[:i]
			after := name[i+len(string(ch)):]
			return base.Render(before) + underline.Render(string(ch)) + base.Render(after)
		}
	}
	return base.Render(name)
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
				highlightShortcut(padded, item.shortcut, nameStyle, m.showShortcuts),
				descStyle.Render(item.desc),
			)
		} else {
			fmt.Fprintf(&sb, "%s  %s  %s\n",
				indent,
				highlightShortcut(padded, item.shortcut, normalStyle, m.showShortcuts),
				descStyle.Render(item.desc),
			)
		}
	}

	footer := styles.StatusBar.Render("↑/↓ navigate · enter select · hold alt for shortcuts · esc exit")
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

	// Measure the widest row to center the sub-menu table.
	// Row structure: cursor(4) + label(10) + gap(2) + values
	const cursorCol = 4
	const labelCol = 10
	const gap = 2
	maxValuesW := 0
	for _, opt := range m.activeSubDef.options {
		// Measure raw (unstyled) values width: "v1 / v2 / v3"
		valW := 0
		for vi, v := range opt.values {
			if vi > 0 {
				valW += 3 // " / "
			}
			valW += len(v)
		}
		if valW > maxValuesW {
			maxValuesW = valW
		}
	}
	tableW := cursorCol + labelCol + gap + maxValuesW

	// Also account for the title line width
	titleW := len(cmdName) + 2 + len(m.items[m.cursor].desc)
	if titleW > tableW {
		tableW = titleW
	}

	const contentW = 90
	leftPad := (contentW - tableW) / 2
	if leftPad < 0 {
		leftPad = 0
	}
	indent := strings.Repeat(" ", leftPad)

	var sb strings.Builder
	sb.WriteString(indent + titleLine + "\n")
	sb.WriteString(indent + styles.Separator(tableW) + "\n")

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
			fmt.Fprintf(&sb, "%s  %s %s  %s\n",
				indent,
				cursorStyle.Render("→"),
				normalStyle.Render(label),
				valueStr,
			)
		} else {
			fmt.Fprintf(&sb, "%s    %s  %s\n",
				indent,
				normalStyle.Render(label),
				valueStr,
			)
		}
	}

	// Run item
	runIdx := len(m.activeSubDef.options)
	sb.WriteString("\n")
	if m.subCursor == runIdx {
		fmt.Fprintf(&sb, "%s  %s %s\n",
			indent,
			cursorStyle.Render("→"),
			selectedStyle.Render("Run"),
		)
	} else {
		fmt.Fprintf(&sb, "%s    %s\n",
			indent,
			normalStyle.Render("Run"),
		)
	}

	footer := styles.StatusBar.Render("↑/↓: navigate · ←/→: change value · enter: select/run · esc: back")
	return sb.String(), footer
}
