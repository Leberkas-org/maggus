package tui

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/leberkas-org/maggus/internal/config"
	"github.com/leberkas-org/maggus/internal/daemon"
	"github.com/leberkas-org/maggus/internal/ipc"
	"github.com/leberkas-org/maggus/internal/logging"
	"github.com/leberkas-org/maggus/internal/tui/component"
	"github.com/leberkas-org/maggus/internal/tui/pane"
	"github.com/leberkas-org/maggus/internal/tui/styles"
)

type stateMsg ipc.DaemonSnapshot
type tickMsg time.Time
type repoAddedMsg struct{ err error }
type repoRemovedMsg struct{ err error }
type planImportedMsg struct {
	title string
	tasks int
	err   error
}

var mainMenuItems = []component.MenuItem{
	{Label: "Repos", Key: "", Action: "repos"},
	{Label: "Import Plan", Key: "", Action: "import"},
	{Label: "Configuration", Key: "", Action: "config"},
	{Label: "Prompting", Key: "", Action: "prompting"},
	{Label: "Update", Key: "", Action: "update"},
	{Label: "Exit", Key: "", Action: "exit"},
}

type App struct {
	width, height int
	left          *pane.LeftPane
	right         *pane.RightPane
	footer        *pane.FooterPane
	focus         FocusTarget
	stateReader   ipc.StateReader
	lastSnap      ipc.DaemonSnapshot
	activeRepo    *config.RepoEntry
	log           *slog.Logger
	logCleanup    func()

	overlay       OverlayKind
	menu          *component.Menu
	repoList      *component.RepoList
	addRepo       *component.Dialog
	fileBrowser   *component.FileBrowser
	planBrowser   *component.FileBrowser
	quitModal     *QuitModal
	notifications []string
}

func NewApp(reader ipc.StateReader) *App {
	left := pane.NewLeftPane()
	left.Focus()

	logger, cleanup, _ := logging.SetupDaemon()
	pid := os.Getpid()
	logger = logger.With("component", "tui", "pid", pid)

	app := &App{
		left:        left,
		right:       pane.NewRightPane(),
		footer:      pane.NewFooterPane(),
		focus:       FocusLeft,
		stateReader: reader,
		log:         logger,
		logCleanup:  cleanup,
	}

	app.activeRepo = config.GetActiveRepo()
	logger.Info("TUI attached", "active_repo", app.activeRepoName())

	return app
}

func (a *App) activeRepoName() string {
	if a.activeRepo != nil {
		return filepath.Base(a.activeRepo.Path)
	}
	return "<none>"
}

func (a *App) Init() tea.Cmd {
	return tea.Batch(a.watchState(), a.tick())
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.resizePanes()
		return a, nil

	case stateMsg:
		a.lastSnap = ipc.DaemonSnapshot(msg)
		a.left.UpdateState(a.lastSnap)
		a.updateFooter()
		return a, a.watchState()

	case watchRetryMsg:
		return a, a.watchState()

	case tickMsg:
		return a, a.tick()

	case repoAddedMsg:
		if msg.err != nil {
			a.addNotification("Error: " + msg.err.Error())
		} else {
			a.addNotification("Repository added")
		}
		a.repoList = component.NewRepoList()
		a.overlay = OverlayRepoList
		a.updateFooter()
		return a, nil

	case planImportedMsg:
		if msg.err != nil {
			a.addNotification("Import error: " + msg.err.Error())
		} else {
			a.addNotification(fmt.Sprintf("Imported: %s (%d tasks)", msg.title, msg.tasks))
		}
		a.overlay = OverlayNone
		a.updateFooter()
		return a, nil

	case repoRemovedMsg:
		if msg.err != nil {
			a.addNotification("Error: " + msg.err.Error())
		} else {
			a.addNotification("Repository removed")
		}
		a.repoList.Reload()
		a.updateFooter()
		return a, nil

	case tea.KeyMsg:
		switch a.overlay {
		case OverlayMenu:
			return a.handleMenuKey(msg)
		case OverlayRepoList:
			return a.handleRepoListKey(msg)
		case OverlayAddRepo:
			return a.handleAddRepoKey(msg)
		case OverlayBrowseRepo:
			return a.handleBrowseRepoKey(msg)
		case OverlayImportPlan:
			return a.handleImportPlanKey(msg)
		case OverlayQuit:
			return a.handleQuitKey(msg)
		default:
			return a.handleKey(msg)
		}
	}

	return a, nil
}

func (a *App) View() string {
	if a.width == 0 || a.height == 0 {
		return ""
	}

	// Always render the base view
	base := a.renderBase()

	// Composite overlay on top if active
	switch a.overlay {
	case OverlayMenu:
		a.menu.Width = a.width
		a.menu.Height = a.height
		return a.compositeOverlay(base, a.menu.View())
	case OverlayRepoList:
		a.repoList.Width = a.width
		a.repoList.Height = a.height
		return a.compositeOverlay(base, a.repoList.View())
	case OverlayAddRepo:
		a.addRepo.Width = a.width
		a.addRepo.Height = a.height
		return a.compositeOverlay(base, a.addRepo.View())
	case OverlayBrowseRepo:
		a.fileBrowser.Width = a.width
		a.fileBrowser.Height = a.height
		return a.compositeOverlay(base, a.fileBrowser.View())
	case OverlayImportPlan:
		a.planBrowser.Width = a.width
		a.planBrowser.Height = a.height
		return a.compositeOverlay(base, a.planBrowser.View())
	case OverlayQuit:
		a.quitModal.width = a.width
		a.quitModal.height = a.height
		return a.compositeOverlay(base, a.quitModal.View())
	}

	return base
}

func (a *App) renderBase() string {
	innerW, innerH := styles.FullScreenInnerSize(a.width, a.height)

	leftW := min(innerW/3, 50)
	rightW := max(innerW-leftW, 0)
	contentH := innerH - 1

	a.left.Resize(leftW, contentH)
	a.right.Resize(rightW, contentH)

	// Feed data into tabs based on selection
	a.updateTabData()

	leftView := a.left.View()
	rightView := a.right.View()

	content := lipgloss.JoinHorizontal(lipgloss.Top, leftView, rightView)

	a.footer.Resize(innerW, 1)
	footer := a.footer.View()

	return styles.FullScreenLeftColor(content, footer, a.width, a.height, styles.Primary)
}

func (a *App) compositeOverlay(base, overlayBox string) string {
	baseLines := strings.Split(base, "\n")
	boxLines := strings.Split(overlayBox, "\n")

	boxH := len(boxLines)
	boxW := 0
	for _, line := range boxLines {
		if w := lipgloss.Width(line); w > boxW {
			boxW = w
		}
	}

	startY := (len(baseLines) - boxH) / 2
	startX := (a.width - boxW) / 2
	if startY < 0 {
		startY = 0
	}
	if startX < 0 {
		startX = 0
	}

	for i, boxLine := range boxLines {
		row := startY + i
		if row >= len(baseLines) {
			break
		}
		bl := baseLines[row]
		blW := lipgloss.Width(bl)

		// ANSI-safe slicing: left of base | box | right of base
		leftPart := ansi.Cut(bl, 0, startX)
		rightPart := ""
		if startX+boxW < blW {
			rightPart = ansi.Cut(bl, startX+boxW, blW)
		}

		baseLines[row] = leftPart + boxLine + rightPart
	}

	return strings.Join(baseLines, "\n")
}

// --- Key handlers ---

func (a *App) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "escape":
		a.openMenu()
		return a, nil
	case "q", "ctrl+c":
		a.openQuit()
		return a, nil
	case "tab":
		a.right.NextTab()
		return a, nil
	case "shift+tab":
		a.right.PrevTab()
		return a, nil
	case "a":
		if itemID := a.left.SelectedItemID(); itemID != "" {
			a.log.Info("approve requested", "item_id", itemID)
			return a, a.approveItem(itemID)
		}
	case "x":
		if itemID := a.left.SelectedItemID(); itemID != "" {
			a.log.Info("skip requested", "item_id", itemID)
			return a, a.skipItem(itemID)
		}
	case "pgup", "pgdown", "home", "end", "alt+up", "alt+down":
		if t := a.right.ActiveTab(); t != nil {
			updated, cmd := t.Update(msg)
			a.right.SetActiveTab(updated)
			return a, cmd
		}
	}

	// Route remaining keys to the left pane (tree navigation)
	_, cmd := a.left.Update(msg)
	return a, cmd
}

func (a *App) handleMenuKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "escape":
		a.overlay = OverlayNone
		return a, nil
	case "up", "k":
		a.menu.MoveUp()
		return a, nil
	case "down", "j":
		a.menu.MoveDown()
		return a, nil
	case "enter":
		return a.executeMenuItem()
	}
	return a, nil
}

func (a *App) handleAddRepoKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "escape":
		a.overlay = OverlayMenu
		return a, nil
	case "enter":
		input := a.addRepo.ActiveInput()
		if input != nil && input.Value != "" {
			path := input.Value
			return a, func() tea.Msg {
				err := config.AddRepo(path)
				return repoAddedMsg{err: err}
			}
		}
		return a, nil
	case "backspace":
		if input := a.addRepo.ActiveInput(); input != nil {
			input.Backspace()
		}
		return a, nil
	case "delete":
		if input := a.addRepo.ActiveInput(); input != nil {
			input.Delete()
		}
		return a, nil
	case "left":
		if input := a.addRepo.ActiveInput(); input != nil {
			input.MoveLeft()
		}
		return a, nil
	case "right":
		if input := a.addRepo.ActiveInput(); input != nil {
			input.MoveRight()
		}
		return a, nil
	case "home":
		if input := a.addRepo.ActiveInput(); input != nil {
			input.Home()
		}
		return a, nil
	case "end":
		if input := a.addRepo.ActiveInput(); input != nil {
			input.End()
		}
		return a, nil
	default:
		// Type characters
		runes := msg.Runes
		if len(runes) > 0 {
			if input := a.addRepo.ActiveInput(); input != nil {
				for _, r := range runes {
					input.InsertRune(r)
				}
			}
		}
		return a, nil
	}
}

func (a *App) handleRepoListKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "escape":
		a.overlay = OverlayMenu
		return a, nil
	case "up", "k":
		a.repoList.MoveUp()
		return a, nil
	case "down", "j":
		a.repoList.MoveDown()
		return a, nil
	case "enter":
		sel := a.repoList.Selected()
		if sel != nil {
			a.activeRepo = sel
			_ = config.SetActiveRepo(sel.Path)
			a.log.Info("switched active repo", "repo", filepath.Base(sel.Path))
			a.addNotification(fmt.Sprintf("Active repo: %s", filepath.Base(sel.Path)))
			a.overlay = OverlayNone
			a.updateFooter()
		}
		return a, nil
	case "a":
		a.fileBrowser = component.NewFileBrowser()
		a.overlay = OverlayBrowseRepo
		return a, nil
	case "d", "delete":
		sel := a.repoList.Selected()
		if sel != nil {
			path := sel.Path
			return a, func() tea.Msg {
				err := config.RemoveRepo(path)
				return repoRemovedMsg{err: err}
			}
		}
		return a, nil
	}
	return a, nil
}

func (a *App) handleBrowseRepoKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "escape":
		a.overlay = OverlayRepoList
		return a, nil
	case "up", "k":
		a.fileBrowser.MoveUp()
		return a, nil
	case "down", "j":
		a.fileBrowser.MoveDown()
		return a, nil
	case "enter":
		sel := a.fileBrowser.Selected()
		if sel != nil && sel.IsGit {
			// Selected a git repo — add it
			path := sel.Path
			return a, func() tea.Msg {
				err := config.AddRepo(path)
				return repoAddedMsg{err: err}
			}
		}
		// Navigate into directory
		a.fileBrowser.Enter()
		return a, nil
	case "backspace":
		a.fileBrowser.GoUp()
		return a, nil
	case "s", "S":
		// Select current directory if it's a git repo
		if a.fileBrowser.CurrentDirIsGit() {
			path := a.fileBrowser.Dir
			return a, func() tea.Msg {
				err := config.AddRepo(path)
				return repoAddedMsg{err: err}
			}
		}
		return a, nil
	}
	return a, nil
}

func (a *App) handleImportPlanKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "escape":
		a.overlay = OverlayMenu
		return a, nil
	case "up", "k":
		a.planBrowser.MoveUp()
		return a, nil
	case "down", "j":
		a.planBrowser.MoveDown()
		return a, nil
	case "enter":
		sel := a.planBrowser.Selected()
		if sel != nil && !sel.IsDir {
			path := sel.Path
			return a, a.importPlan(path)
		}
		a.planBrowser.Enter()
		return a, nil
	case "backspace":
		a.planBrowser.GoUp()
		return a, nil
	}
	return a, nil
}

func (a *App) importPlan(path string) tea.Cmd {
	repo := a.activeRepo
	return func() tea.Msg {
		if repo == nil {
			return planImportedMsg{err: fmt.Errorf("no active repo")}
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return planImportedMsg{err: err}
		}
		plan, err := daemon.Parse(string(content))
		if err != nil {
			return planImportedMsg{err: err}
		}
		tasksDir := filepath.Join(repo.Path, ".maggus", "tasks")
		if err := os.MkdirAll(tasksDir, 0o755); err != nil {
			return planImportedMsg{err: err}
		}
		dest := filepath.Join(tasksDir, filepath.Base(path))
		if err := os.WriteFile(dest, content, 0o644); err != nil {
			return planImportedMsg{err: err}
		}
		return planImportedMsg{title: plan.Title, tasks: len(plan.Tasks)}
	}
}

func (a *App) handleQuitKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "s", "S":
		a.log.Info("TUI stopping daemon and quitting")
		return a, tea.Sequence(a.stopDaemon(), tea.Quit)
	case "d", "D":
		a.log.Info("TUI detaching (daemon stays)")
		return a, tea.Quit
	case "esc", "escape":
		a.overlay = OverlayNone
		return a, nil
	}
	return a, nil
}

func (a *App) stopDaemon() tea.Cmd {
	return func() tea.Msg {
		globalDir, err := getGlobalDir()
		if err != nil {
			return nil
		}
		writer := ipc.NewFileCommandWriter(globalDir)
		_ = writer.StopAll()
		return nil
	}
}

// --- Menu actions ---

func (a *App) openMenu() {
	a.menu = component.NewMenu("Maggus", mainMenuItems)
	a.overlay = OverlayMenu
}

func (a *App) openQuit() {
	a.quitModal = &QuitModal{}
	a.overlay = OverlayQuit
}

func (a *App) executeMenuItem() (tea.Model, tea.Cmd) {
	item := a.menu.Selected()
	switch item.Action {
	case "repos":
		a.repoList = component.NewRepoList()
		a.overlay = OverlayRepoList
	case "import":
		if a.activeRepo == nil {
			a.addNotification("No active repo — select one in Repos first")
			a.overlay = OverlayNone
			a.updateFooter()
			return a, nil
		}
		repoRoot := a.activeRepo.Path
		startDir := repoRoot
		plansDir := filepath.Join(repoRoot, "docs", "superpowers", "plans")
		if info, err := os.Stat(plansDir); err == nil && info.IsDir() {
			startDir = plansDir
		}
		a.planBrowser = component.NewFileBrowserScoped(repoRoot, startDir, "Import Plan", ".md")
		a.overlay = OverlayImportPlan
	case "config":
		a.log.Warn("not yet implemented", "feature", "configuration")
		a.overlay = OverlayNone
		a.updateFooter()
	case "prompting":
		a.log.Warn("not yet implemented", "feature", "prompting")
		a.overlay = OverlayNone
		a.updateFooter()
	case "update":
		a.log.Warn("not yet implemented", "feature", "update")
		a.overlay = OverlayNone
		a.updateFooter()
	case "exit":
		a.openQuit()
	}
	return a, nil
}

// --- Helpers ---

func (a *App) toggleFocus() {
	if a.focus == FocusLeft {
		a.focus = FocusRight
		a.left.Blur()
		a.right.Focus()
	} else {
		a.focus = FocusLeft
		a.right.Blur()
		a.left.Focus()
	}
}

func (a *App) resizePanes() {
	innerW, innerH := styles.FullScreenInnerSize(a.width, a.height)
	leftW := min(innerW/3, 50)
	rightW := max(innerW-leftW, 0)
	contentH := innerH - 1
	a.left.Resize(leftW, contentH)
	a.right.Resize(rightW, contentH)
	a.footer.Resize(innerW, 1)
}

func (a *App) updateTabData() {
	sel := a.left.Selected()
	if sel == nil {
		return
	}

	itemID := a.left.SelectedItemID()
	if itemID == "" {
		return
	}

	for _, q := range a.lastSnap.Queue {
		if q.ID != itemID {
			continue
		}

		// Determine what to show in Summary based on selection
		isTask := sel.ID != itemID

		if isTask {
			// Selected a task — show the task file
			taskFile := filepath.Join(filepath.Dir(q.PlanFile), sel.ID+".md")
			if data, err := os.ReadFile(taskFile); err == nil {
				a.setTabData("Summary", string(data))
			}
		} else {
			// Selected the item — show the source plan file
			if q.PlanFile != "" {
				if data, err := os.ReadFile(q.PlanFile); err == nil {
					a.setTabData("Summary", string(data))
				}
			}
		}

		// Plan tab always shows the context
		if q.Description != "" {
			a.setTabData("Plan", q.Description)
		}

		break
	}
}

func (a *App) setTabData(name string, data string) {
	for _, t := range a.right.AllTabs() {
		if t.Name() == name {
			t.SetData(data)
			return
		}
	}
}

func (a *App) updateFooter() {
	s := a.lastSnap

	var status string
	if daemon.IsLocked() {
		status = fmt.Sprintf("Daemon running | %d tasks active", s.ActiveTasks)
	} else {
		status = "Daemon stopped"
	}
	if a.activeRepo != nil {
		status += " | " + filepath.Base(a.activeRepo.Path)
	}
	if s.BryanConnected {
		status += " | Bryan: connected"
	}
	a.footer.SetStatus(status)
	a.footer.SetKeyHints("esc: menu  q: exit  tab/shift+tab: tabs  a: approve  x: skip")
}

type watchRetryMsg struct{}

func (a *App) watchState() tea.Cmd {
	return func() tea.Msg {
		// Try reading current state first
		snap, err := a.stateReader.ReadState()
		if err == nil && !snap.UpdatedAt.IsZero() && len(snap.Queue) > 0 {
			return stateMsg(snap)
		}

		// Wait for a change via fsnotify
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		ch := a.stateReader.Watch(ctx)
		select {
		case s := <-ch:
			if len(s.Queue) > 0 || !s.UpdatedAt.IsZero() {
				return stateMsg(s)
			}
			return watchRetryMsg{}
		case <-ctx.Done():
			// Re-read on timeout — state may have been written before watch started
			snap, err := a.stateReader.ReadState()
			if err == nil && !snap.UpdatedAt.IsZero() {
				return stateMsg(snap)
			}
			return watchRetryMsg{}
		}
	}
}

// re-exported so watchState timeout still triggers re-watch
type DaemonSnapshot = ipc.DaemonSnapshot

func (a *App) tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (a *App) approveItem(itemID string) tea.Cmd {
	return func() tea.Msg {
		globalDir, err := getGlobalDir()
		if err != nil {
			return nil
		}
		writer := ipc.NewFileCommandWriter(globalDir)
		_ = writer.Approve(itemID)
		return nil
	}
}

func (a *App) skipItem(itemID string) tea.Cmd {
	return func() tea.Msg {
		globalDir, err := getGlobalDir()
		if err != nil {
			return nil
		}
		writer := ipc.NewFileCommandWriter(globalDir)
		_ = writer.Skip(itemID)
		return nil
	}
}

func (a *App) addNotification(msg string) {
	a.notifications = append(a.notifications, msg)
	if len(a.notifications) > 50 {
		a.notifications = a.notifications[len(a.notifications)-50:]
	}
}

func getGlobalDir() (string, error) {
	return config.GlobalDir()
}
