package cmd

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
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

// hideShortcutsMsg is sent after a delay to hide the shortcut underlines.
type hideShortcutsMsg struct {
	timerID int // only hide if this matches the current timer ID
}

// daemonAutoStartResultMsg carries the result of the silent daemon auto-start attempt.
type daemonAutoStartResultMsg struct {
	err error // nil = started or already running; non-nil = failed (show warning)
}

// daemonStopResultMsg carries the result of the async daemon stop attempt.
type daemonStopResultMsg struct {
	err error
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

func (m menuModel) Init() tea.Cmd {
	cmds := []tea.Cmd{
		func() tea.Msg {
			return claude2xResultMsg{status: claude2x.FetchStatus()}
		},
		func() tea.Msg {
			return updateCheckResultMsg{banner: startupUpdateCheck()}
		},
		func() tea.Msg {
			return daemonAutoStartResultMsg{err: autoStartDaemon(m.cwd)}
		},
		listenForWatcherUpdate(m.watcherCh),
		listenForDaemonCacheUpdate(m.daemonCacheCh),
	}
	return tea.Batch(cmds...)
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
	case daemonAutoStartResultMsg:
		if msg.err != nil {
			m.daemonAutoWarning = fmt.Sprintf("daemon auto-start failed: %s", msg.err)
		}
		return m, nil
	case daemonStopResultMsg:
		// Daemon stop finished (success or failure) — exit the program.
		m.quitting = true
		return m, tea.Quit
	case daemonCacheUpdateMsg:
		m.daemon.PID = msg.State.PID
		m.daemon.Running = msg.State.Running
		return m, listenForDaemonCacheUpdate(m.daemonCacheCh)
	case featureSummaryUpdateMsg:
		m.summary = loadFeatureSummary()
		return m, listenForWatcherUpdate(m.watcherCh)

	case hideShortcutsMsg:
		// Only hide if this timer is still the latest one
		if msg.timerID == m.shortcutTimerID {
			m.showShortcuts = false
		}
		return m, nil

	case tea.KeyMsg:
		// Handle stop-daemon confirmation prompt.
		if m.confirmStopDaemon {
			return m.updateConfirmStopDaemon(msg)
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
	case "q":
		if m.daemon.Running {
			m.confirmStopDaemon = true
			return m, nil
		}
		m.quitting = true
		return m, tea.Quit
	case "up":
		m.cursor = styles.CursorUp(m.cursor, len(m.items))
	case "down":
		m.cursor = styles.CursorDown(m.cursor, len(m.items))
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
		if m.daemon.Running {
			m.confirmStopDaemon = true
			return m, nil
		}
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

func (m menuModel) updateSubMenu(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	itemCount := len(m.activeSubDef.options) + 1 // options + Run

	switch msg.String() {
	case "q":
		m.inSubMenu = false
		m.activeSubDef = nil
		m.subCursor = 0
		return m, nil
	case "up", "k":
		m.subCursor = styles.CursorUp(m.subCursor, itemCount)
	case "down", "j":
		m.subCursor = styles.CursorDown(m.subCursor, itemCount)
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

// updateConfirmStopDaemon handles keys in the "Stop daemon?" confirmation prompt.
func (m menuModel) updateConfirmStopDaemon(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter, tea.KeyEscape:
		// Cancel the prompt and return to the main menu.
		m.confirmStopDaemon = false
		return m, nil
	case tea.KeyRunes:
		if len(msg.Runes) == 1 {
			switch msg.Runes[0] {
			case 'y', 'Y':
				// Stop the daemon asynchronously, then quit.
				cwd := m.cwd
				return m, func() tea.Msg {
					_ = stopDaemonGracefully(cwd)
					return daemonStopResultMsg{}
				}
			case 'n', 'N', 'd', 'D':
				// Exit without stopping the daemon (detached).
				m.quitting = true
				return m, tea.Quit
			}
		}
	}
	// Ignore other keys while the prompt is shown.
	return m, nil
}
