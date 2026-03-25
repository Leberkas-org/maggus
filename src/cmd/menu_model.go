package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/leberkas-org/maggus/internal/filewatcher"
	"github.com/leberkas-org/maggus/internal/parser"
	"github.com/leberkas-org/maggus/internal/tui/styles"
	xterm "golang.org/x/term"
)

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
	isDaemonToggle    bool // dynamic start/stop daemon item
	isDaemonOnly      bool // only visible when daemon is running
}

var allMenuItems = []menuItem{
	// Core workflow
	{name: "status", desc: "Live log & feature management", shortcut: 's', shortcutLabel: "s"},
	{name: "repos", desc: "Manage configured repositories", shortcut: 'r', shortcutLabel: "r"},
	// AI-assisted creation
	{name: "prompt", desc: "Launch interactive Claude session with usage tracking", requiresClaude: true, separator: true, shortcut: 'o', shortcutLabel: "o"},
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
		if item.isDaemonOnly {
			continue // added/removed dynamically based on daemon state
		}
		// Don't start with a separator if this is the first visible item.
		if item.separator && len(items) == 0 {
			item.separator = false
		}
		items = append(items, item)
	}
	return items
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

	plans, err := parser.LoadPlans(dir, true)
	if err != nil {
		return featureSummary{}
	}

	pruneStaleApprovals(dir, plans)

	var s featureSummary
	for _, p := range plans {
		if p.IsBug {
			s.bugs++
			s.bugTasks += len(p.Tasks)
			s.bugDone += p.DoneCount()
			s.bugBlocked += p.BlockedCount()
			for _, t := range p.Tasks {
				if t.IsWorkable() {
					s.bugWorkable++
				}
			}
		} else {
			s.features++
			s.tasks += len(p.Tasks)
			s.done += p.DoneCount()
			s.blocked += p.BlockedCount()
			for _, t := range p.Tasks {
				if t.IsWorkable() {
					s.workable++
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

// formatDaemonStatusLine returns a styled one-line string showing daemon state.
// When running: "● daemon running (PID 12345)" in cyan.
// When not running: "○ daemon not running" in dim/muted.
func formatDaemonStatusLine(d daemonStatus) string {
	if d.Running {
		cyanStyle := lipgloss.NewStyle().Foreground(styles.Primary)
		return cyanStyle.Render(fmt.Sprintf("● daemon running (PID %d)", d.PID))
	}
	dimStyle := lipgloss.NewStyle().Faint(true)
	return dimStyle.Render("○ daemon not running")
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

	firstLaunch bool // true only on the very first menu open; prevents auto-dispatch on re-entry after work

	// File watcher for live summary updates
	watcher   *filewatcher.Watcher
	watcherCh chan bool

	// Daemon state
	daemon            daemonStatus
	daemonAutoWarning string // non-fatal warning if auto-start failed

	// Stop-daemon confirmation state
	confirmStopDaemon bool

	// Sub-menu state
	inSubMenu    bool
	subCursor    int // cursor within sub-menu (options + Run item)
	subMenuDefs  map[string]subMenuDef
	activeSubDef *subMenuDef // pointer to the active sub-menu definition (with live option state)
}

func newMenuModel(summary featureSummary, firstLaunch bool) menuModel {
	cwd, _ := os.Getwd()

	// Query actual terminal dimensions before the first render so View() always
	// uses FullScreenColor and never falls back to the Box fallback path.
	termW, termH, _ := xterm.GetSize(int(os.Stdout.Fd()))

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
		items:        activeMenuItems(),
		summary:      summary,
		cwd:          cwd,
		firstLaunch:  firstLaunch,
		subMenuDefs:  buildSubMenus(),
		watcher:      w,
		watcherCh:    ch,
		width:        termW,
		height:       termH,
	}
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
