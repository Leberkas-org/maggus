package cmd

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/leberkas-org/maggus/internal/claude2x"
	"github.com/leberkas-org/maggus/internal/gitutil"
	"github.com/leberkas-org/maggus/internal/globalconfig"
	"github.com/leberkas-org/maggus/internal/tui/filebrowser"
	"github.com/leberkas-org/maggus/internal/tui/styles"
)

// reposState represents the current state of the repos TUI.
type reposState int

const (
	reposStateList        reposState = iota // showing repo list
	reposStateBrowsing                      // file browser active
	reposStateConfirmInit                   // asking whether to initialize .maggus
)

// reposDaemonUpdateMsg is delivered when a specific repo's daemon state changes.
type reposDaemonUpdateMsg struct {
	idx   int
	state daemonPIDState
}

// listenForRepoDaemonUpdate returns a Cmd that blocks on ch until a daemon state
// update arrives, then delivers a reposDaemonUpdateMsg for the given repo index.
func listenForRepoDaemonUpdate(idx int, ch <-chan daemonPIDState) tea.Cmd {
	if ch == nil {
		return nil
	}
	return func() tea.Msg {
		state, ok := <-ch
		if !ok {
			return nil // channel closed, cache stopped
		}
		return reposDaemonUpdateMsg{idx: idx, state: state}
	}
}

// reposDaemonActionResultMsg carries the result of an async daemon start/stop.
type reposDaemonActionResultMsg struct {
	msg string
	err error
}

// reposStatusClearMsg is sent after 2 seconds to clear the status message.
type reposStatusClearMsg struct {
	id int // only clear if this matches the current timer ID
}

// reposModel is the bubbletea model for the repository management screen.
type reposModel struct {
	repos      []globalconfig.Repository
	lastOpened string
	cwd        string // current working directory
	cursor     int
	state      reposState
	width      int
	height     int
	quitting   bool
	switched   bool // true if user switched repos (triggers menu reload)
	statusMsg  string
	statusID   int // incremented each time statusMsg is set, for timed clearing

	// Per-repo daemon running state, indexed same as repos.
	daemonRunning  []bool
	daemonCaches   []*DaemonStateCache
	daemonSubChans []chan daemonPIDState

	is2x bool // true when Claude is in 2x mode (border turns yellow)

	// File browser sub-model (only active in browsing state)
	browser    filebrowser.Model
	pendingDir string // directory selected by browser, pending init confirmation

	// Injected for testing
	loadConfig func() (globalconfig.GlobalConfig, error)
	saveConfig func(globalconfig.GlobalConfig) error
	isGitRepo  func(string) bool
	chdir      func(string) error
}

func newReposModel() reposModel {
	cfg, _ := globalconfig.Load()
	cwd, _ := os.Getwd()
	cwd, _ = filepath.Abs(cwd)

	m := reposModel{
		repos:      cfg.Repositories,
		lastOpened: cfg.LastOpened,
		cwd:        cwd,
		loadConfig: globalconfig.Load,
		saveConfig: globalconfig.Save,
		isGitRepo:  isGitRepoCheck,
		chdir:      os.Chdir,
	}
	m.buildCaches()
	return m
}

// buildCaches stops any existing caches, then creates one DaemonStateCache per repo.
// It pre-populates daemonRunning from cache.Get() and subscribes to each cache.
// Repos whose path has no .maggus/ directory are skipped (running = false).
func (m *reposModel) buildCaches() {
	for _, c := range m.daemonCaches {
		if c != nil {
			c.Stop()
		}
	}
	n := len(m.repos)
	m.daemonCaches = make([]*DaemonStateCache, n)
	m.daemonSubChans = make([]chan daemonPIDState, n)
	m.daemonRunning = make([]bool, n)
	for i, repo := range m.repos {
		cache, err := NewDaemonStateCache(repo.Path)
		if err != nil {
			log.Printf("repos: DaemonStateCache for %s: %v", repo.Path, err)
			continue
		}
		m.daemonCaches[i] = cache
		m.daemonRunning[i] = cache.Get().Running
		m.daemonSubChans[i] = cache.Subscribe()
	}
}

// cacheListenCmds returns one listener Cmd per active cache subscription.
func (m *reposModel) cacheListenCmds() tea.Cmd {
	var cmds []tea.Cmd
	for i, ch := range m.daemonSubChans {
		if ch != nil {
			cmds = append(cmds, listenForRepoDaemonUpdate(i, ch))
		}
	}
	return tea.Batch(cmds...)
}

// setStatusMsg sets the status message and schedules a clear after 2 seconds.
func (m *reposModel) setStatusMsg(msg string) tea.Cmd {
	m.statusID++
	m.statusMsg = msg
	id := m.statusID
	return tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
		return reposStatusClearMsg{id: id}
	})
}

func (m reposModel) Init() tea.Cmd {
	return tea.Batch(
		func() tea.Msg {
			return claude2xResultMsg{status: claude2x.FetchStatus()}
		},
		m.cacheListenCmds(),
	)
}

func (m reposModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.state == reposStateBrowsing {
			updated, cmd := m.browser.Update(msg)
			m.browser = updated.(filebrowser.Model)
			return m, cmd
		}
		return m, nil
	case claude2xResultMsg:
		m.is2x = msg.status.Is2x
		if m.is2x {
			return m, next2xTick()
		}
		return m, nil
	case claude2xTickMsg:
		is2x, _, tickCmd := fetch2xAndUpdate()
		m.is2x = is2x
		return m, tickCmd
	case reposDaemonUpdateMsg:
		if msg.idx < len(m.daemonRunning) {
			m.daemonRunning[msg.idx] = msg.state.Running
		}
		if msg.idx < len(m.daemonSubChans) {
			return m, listenForRepoDaemonUpdate(msg.idx, m.daemonSubChans[msg.idx])
		}
		return m, nil
	case reposDaemonActionResultMsg:
		text := msg.msg
		if msg.err != nil {
			text = fmt.Sprintf("%s: %v", msg.msg, msg.err)
		}
		cmd := m.setStatusMsg(text)
		return m, cmd
	case reposStatusClearMsg:
		if msg.id == m.statusID {
			m.statusMsg = ""
		}
		return m, nil
	case tea.KeyMsg:
		switch m.state {
		case reposStateList:
			return m.updateList(msg)
		case reposStateBrowsing:
			return m.updateBrowsing(msg)
		case reposStateConfirmInit:
			return m.updateConfirmInit(msg)
		}
	}
	return m, nil
}

func (m reposModel) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		return m, func() tea.Msg { return navigateBackMsg{} }
	case "up", "k":
		m.cursor = styles.CursorUp(m.cursor, len(m.repos))
	case "down", "j":
		m.cursor = styles.CursorDown(m.cursor, len(m.repos))
	case "home":
		m.cursor = 0
	case "end":
		if len(m.repos) > 0 {
			m.cursor = len(m.repos) - 1
		}
	case "enter":
		// Switch to selected repo
		if len(m.repos) > 0 {
			return m.switchToRepo(m.repos[m.cursor].Path)
		}
	case "n":
		// Add repo via file browser
		m.state = reposStateBrowsing
		m.browser = filebrowser.New(m.cwd)
		// Forward current size to browser
		updated, _ := m.browser.Update(tea.WindowSizeMsg{Width: m.width, Height: m.height})
		m.browser = updated.(filebrowser.Model)
		m.statusMsg = ""
		return m, nil
	case "d", "delete", "backspace":
		// Remove selected repo
		if len(m.repos) > 0 {
			return m.removeRepo()
		}
	case "s":
		// Start or stop daemon for selected repo
		if len(m.repos) > 0 {
			return m.toggleDaemon()
		}
	case "a":
		// Toggle auto-start for selected repo
		if len(m.repos) > 0 {
			return m.toggleAutoStart()
		}
	}
	return m, nil
}

// toggleDaemon starts or stops the daemon for the selected repo.
func (m reposModel) toggleDaemon() (tea.Model, tea.Cmd) {
	repo := m.repos[m.cursor]
	running := m.cursor < len(m.daemonRunning) && m.daemonRunning[m.cursor]

	if running {
		// Stop daemon asynchronously
		dir := repo.Path
		return m, func() tea.Msg {
			err := stopDaemonGracefully(dir)
			if err != nil {
				return reposDaemonActionResultMsg{msg: "failed to stop daemon", err: err}
			}
			return reposDaemonActionResultMsg{msg: "daemon stopped"}
		}
	}

	// Start daemon asynchronously (bypass auto-start preference — this is a manual action)
	dir := repo.Path
	return m, func() tea.Msg {
		err := startDaemon(dir)
		if err != nil {
			return reposDaemonActionResultMsg{msg: "failed to start daemon", err: err}
		}
		return reposDaemonActionResultMsg{msg: "daemon started"}
	}
}

// toggleAutoStart toggles the auto-start preference for the selected repo.
func (m reposModel) toggleAutoStart() (tea.Model, tea.Cmd) {
	cfg, err := m.loadConfig()
	if err != nil {
		cmd := m.setStatusMsg(fmt.Sprintf("Failed to load config: %v", err))
		return m, cmd
	}

	if m.cursor >= len(cfg.Repositories) {
		return m, nil
	}

	cfg.Repositories[m.cursor].AutoStartDisabled = !cfg.Repositories[m.cursor].AutoStartDisabled
	if err := m.saveConfig(cfg); err != nil {
		cmd := m.setStatusMsg(fmt.Sprintf("Failed to save: %v", err))
		return m, cmd
	}

	m.repos = cfg.Repositories
	var statusText string
	if cfg.Repositories[m.cursor].IsAutoStartEnabled() {
		statusText = "auto-start enabled"
	} else {
		statusText = "auto-start disabled"
	}
	cmd := m.setStatusMsg(statusText)
	return m, cmd
}

func (m reposModel) updateBrowsing(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	updated, cmd := m.browser.Update(msg)
	m.browser = updated.(filebrowser.Model)

	if m.browser.Cancelled() {
		m.state = reposStateList
		m.statusMsg = ""
		return m, nil
	}

	if sel := m.browser.Selected(); sel != "" {
		// Validate it's a git repo
		if !m.isGitRepo(sel) {
			m.state = reposStateList
			m.statusMsg = "Not a git repository"
			return m, nil
		}

		// Check if .maggus/ exists
		maggusDir := filepath.Join(sel, ".maggus")
		if info, err := os.Stat(maggusDir); err != nil || !info.IsDir() {
			// Ask whether to initialize
			m.pendingDir = sel
			m.state = reposStateConfirmInit
			return m, nil
		}

		// It's a git repo with .maggus — add it
		return m.addRepo(sel)
	}

	return m, cmd
}

func (m reposModel) updateConfirmInit(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y":
		// Initialize .maggus directory
		maggusDir := filepath.Join(m.pendingDir, ".maggus")
		if err := os.MkdirAll(maggusDir, 0o755); err != nil {
			m.state = reposStateList
			m.statusMsg = fmt.Sprintf("Failed to create .maggus: %v", err)
			return m, nil
		}
		return m.addRepo(m.pendingDir)
	case "n", "esc":
		// Add without initializing
		return m.addRepo(m.pendingDir)
	}
	return m, nil
}

func (m reposModel) switchToRepo(path string) (tea.Model, tea.Cmd) {
	if err := m.chdir(path); err != nil {
		m.statusMsg = fmt.Sprintf("Failed to switch: %v", err)
		return m, nil
	}

	// Update global config
	cfg, err := m.loadConfig()
	if err == nil {
		cfg.SetLastOpened(path)
		_ = m.saveConfig(cfg)
	}

	m.cwd = path
	m.switched = true
	return m, func() tea.Msg { return navigateBackMsg{} }
}

func (m reposModel) addRepo(path string) (tea.Model, tea.Cmd) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}

	cfg, err := m.loadConfig()
	if err != nil {
		m.state = reposStateList
		m.statusMsg = fmt.Sprintf("Failed to load config: %v", err)
		return m, nil
	}

	if !cfg.AddRepository(absPath) {
		m.state = reposStateList
		m.statusMsg = "Repository already configured"
		return m, nil
	}

	if err := m.saveConfig(cfg); err != nil {
		m.state = reposStateList
		m.statusMsg = fmt.Sprintf("Failed to save: %v", err)
		return m, nil
	}

	// Refresh the repo list
	m.repos = cfg.Repositories
	m.lastOpened = cfg.LastOpened
	m.cursor = len(m.repos) - 1 // move cursor to newly added
	m.state = reposStateList
	m.statusMsg = fmt.Sprintf("Added %s", filepath.Base(absPath))
	m.buildCaches()
	return m, m.cacheListenCmds()
}

func (m reposModel) removeRepo() (tea.Model, tea.Cmd) {
	path := m.repos[m.cursor].Path

	cfg, err := m.loadConfig()
	if err != nil {
		m.statusMsg = fmt.Sprintf("Failed to load config: %v", err)
		return m, nil
	}

	cfg.RemoveRepository(path)
	if err := m.saveConfig(cfg); err != nil {
		m.statusMsg = fmt.Sprintf("Failed to save: %v", err)
		return m, nil
	}

	m.repos = cfg.Repositories
	m.lastOpened = cfg.LastOpened
	if m.cursor >= len(m.repos) && m.cursor > 0 {
		m.cursor = len(m.repos) - 1
	}
	m.statusMsg = fmt.Sprintf("Removed %s", filepath.Base(path))
	m.buildCaches()
	return m, m.cacheListenCmds()
}

func (m reposModel) View() string {
	switch m.state {
	case reposStateBrowsing:
		return m.browser.View()
	case reposStateConfirmInit:
		return m.viewConfirmInit()
	default:
		return m.viewList()
	}
}

func (m reposModel) viewList() string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.Primary)
	selectedStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.Primary)
	mutedStyle := lipgloss.NewStyle().Foreground(styles.Muted)
	normalStyle := lipgloss.NewStyle()
	runningStyle := lipgloss.NewStyle().Foreground(styles.Success)
	stoppedStyle := lipgloss.NewStyle().Foreground(styles.Muted)
	autoStyle := lipgloss.NewStyle().Foreground(styles.Success)
	noAutoStyle := lipgloss.NewStyle().Foreground(styles.Muted)

	var b strings.Builder

	b.WriteString(titleStyle.Render("Repositories"))
	b.WriteString("\n")

	innerW, _ := styles.FullScreenInnerSize(m.width, m.height)
	if innerW < 20 {
		innerW = 60
	}
	b.WriteString(styles.Separator(innerW))
	b.WriteString("\n")

	if len(m.repos) == 0 {
		b.WriteString(mutedStyle.Render("No repositories configured. Press 'n' to add one."))
		b.WriteString("\n")
	} else {
		for i, repo := range m.repos {
			prefix := "  "
			if i == m.cursor {
				prefix = "→ "
			}

			label := filepath.Base(repo.Path)

			// Daemon status indicator
			running := i < len(m.daemonRunning) && m.daemonRunning[i]
			var indicator string
			if running {
				indicator = runningStyle.Render("●")
			} else {
				indicator = stoppedStyle.Render("○")
			}

			// Auto-start badge
			var autoBadge string
			if repo.IsAutoStartEnabled() {
				autoBadge = autoStyle.Render("[auto]")
			} else {
				autoBadge = noAutoStyle.Render("[no auto]")
			}

			// Build the line
			var nameStr string
			if i == m.cursor {
				nameStr = selectedStyle.Render(prefix + label)
			} else {
				nameStr = normalStyle.Render(prefix + label)
			}

			line := fmt.Sprintf("%s %s %s  %s", nameStr, indicator, autoBadge, mutedStyle.Render(repo.Path))
			b.WriteString(line)
			b.WriteString("\n")
		}
	}

	// Status message
	if m.statusMsg != "" {
		b.WriteString("\n")
		statusStyle := lipgloss.NewStyle().Foreground(styles.Warning)
		b.WriteString(statusStyle.Render(m.statusMsg))
		b.WriteString("\n")
	}

	content := b.String()
	footer := styles.StatusBar.Render("s start/stop daemon · a toggle auto-start · enter switch · n add · d remove · q: exit")

	borderColor := styles.ThemeColor(m.is2x)
	if m.width > 0 && m.height > 0 {
		return styles.FullScreenLeftColor(content, footer, m.width, m.height, borderColor)
	}
	return styles.Box.BorderForeground(borderColor).Render(content+"\n\n"+footer) + "\n"
}

func (m reposModel) viewConfirmInit() string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.Primary)
	mutedStyle := lipgloss.NewStyle().Foreground(styles.Muted)
	warningStyle := lipgloss.NewStyle().Foreground(styles.Warning)

	var b strings.Builder
	b.WriteString(titleStyle.Render("Initialize Repository"))
	b.WriteString("\n\n")
	fmt.Fprintf(&b, "The directory %s is a git repo but has no .maggus/ directory.\n\n", warningStyle.Render(m.pendingDir))
	b.WriteString("Initialize it? ")
	b.WriteString(mutedStyle.Render("(y/n)"))
	b.WriteString("\n")

	content := b.String()
	footer := styles.StatusBar.Render("y: initialize · n: add without initializing · esc: cancel")

	borderColor := styles.ThemeColor(m.is2x)
	if m.width > 0 && m.height > 0 {
		return styles.FullScreenLeftColor(content, footer, m.width, m.height, borderColor)
	}
	return styles.Box.BorderForeground(borderColor).Render(content+"\n\n"+footer) + "\n"
}

// isGitRepoCheck checks if a directory is inside a git work tree.
func isGitRepoCheck(dir string) bool {
	cmd := gitutil.Command("-C", dir, "rev-parse", "--is-inside-work-tree")
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run() == nil
}

func runRepos() error {
	rm := newReposModel()
	app := appModel{
		active: screenRepos,
		repos:  &rm,
	}
	p := tea.NewProgram(app, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
