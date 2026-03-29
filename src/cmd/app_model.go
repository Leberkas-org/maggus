package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/leberkas-org/maggus/internal/config"
	"github.com/leberkas-org/maggus/internal/filewatcher"
	"github.com/leberkas-org/maggus/internal/stores"
)

// screenID identifies which sub-screen is currently active in the app router.
type screenID int

const (
	screenMenu   screenID = iota
	screenStatus          // live status / feature management
	screenConfig          // project & global config editor
	screenRepos           // repository management
	screenPrompt          // prompt picker / interactive claude session
)

// navigateToMsg tells the app router to switch to the given screen.
// The target sub-model is initialised (or re-initialised) when this message is received.
type navigateToMsg struct {
	screen screenID
}

// navigateBackMsg tells the app router to return to the main menu.
// Sub-screens emit this instead of tea.Quit so the router can handle teardown.
type navigateBackMsg struct{}

// execProcessMsg asks the app router to suspend the TUI, run cmd in the foreground,
// and call onDone with the process error when it exits.
// The router handles this via tea.ExecProcess so the alt-screen is preserved.
// onDone may return a tea.Msg that is dispatched back to the Update loop after
// the process exits (e.g. navigateBackMsg{} to return to the menu).
type execProcessMsg struct {
	cmd    *exec.Cmd
	onDone func(error) tea.Msg
}

// appModel is the top-level BubbleTea model that acts as a screen router.
// A single tea.NewProgram wraps appModel; navigation between sub-screens is
// driven by navigateToMsg / navigateBackMsg messages rather than program restarts.
// Sub-models are lazy-initialised on first navigation and torn down (watchers closed,
// channels unsubscribed) when navigating away.
type appModel struct {
	active screenID

	// Lazy-initialised sub-models. A nil pointer means the screen has not been
	// visited yet (or has been torn down and needs re-initialisation).
	menu   *menuModel
	status *statusModel
	cfg    *configModel
	repos  *reposModel
	prompt *promptPickerModel
}

// newAppModel creates an appModel starting on screenMenu with the menu pre-initialised.
func newAppModel() appModel {
	summary := loadFeatureSummary()
	m := newMenuModel(summary)
	return appModel{
		active: screenMenu,
		menu:   &m,
	}
}

// activeTeaModel returns the currently active sub-model as a tea.Model.
// Returns nil when no sub-model is initialised for the active screen.
func (m appModel) activeTeaModel() tea.Model {
	switch m.active {
	case screenMenu:
		if m.menu != nil {
			return m.menu
		}
	case screenStatus:
		if m.status != nil {
			return m.status
		}
	case screenConfig:
		if m.cfg != nil {
			return m.cfg
		}
	case screenRepos:
		if m.repos != nil {
			return m.repos
		}
	case screenPrompt:
		if m.prompt != nil {
			return m.prompt
		}
	}
	return nil
}

// Init delegates to the active screen's Init.
func (m appModel) Init() tea.Cmd {
	if am := m.activeTeaModel(); am != nil {
		return am.Init()
	}
	return nil
}

// Update intercepts navigation and subprocess messages, then delegates all
// other messages to the active sub-model.
func (m appModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case navigateToMsg:
		return m.navigateTo(msg.screen)

	case navigateBackMsg:
		return m.navigateTo(screenMenu)

	case execProcessMsg:
		return m, tea.ExecProcess(msg.cmd, func(err error) tea.Msg {
			if msg.onDone != nil {
				return msg.onDone(err)
			}
			return nil
		})

	case tea.WindowSizeMsg:
		// Forward to active sub-model and also store for future sub-model inits.
		return m.forwardToActive(msg)
	}

	return m.forwardToActive(msg)
}

// View delegates rendering to the active screen's View.
func (m appModel) View() string {
	switch m.active {
	case screenMenu:
		if m.menu != nil {
			return m.menu.View()
		}
	case screenStatus:
		if m.status != nil {
			return m.status.View()
		}
	case screenConfig:
		if m.cfg != nil {
			return m.cfg.View()
		}
	case screenRepos:
		if m.repos != nil {
			return m.repos.View()
		}
	case screenPrompt:
		if m.prompt != nil {
			return m.prompt.View()
		}
	}
	return ""
}

// forwardToActive sends msg to the currently active sub-model, stores the
// updated model back, and returns the result.
func (m appModel) forwardToActive(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.active {
	case screenMenu:
		if m.menu != nil {
			updated, cmd := m.menu.Update(msg)
			if mm, ok := updated.(menuModel); ok {
				m.menu = &mm
			}
			return m, cmd
		}
	case screenStatus:
		if m.status != nil {
			updated, cmd := m.status.Update(msg)
			if sm, ok := updated.(statusModel); ok {
				m.status = &sm
			}
			return m, cmd
		}
	case screenConfig:
		if m.cfg != nil {
			updated, cmd := m.cfg.Update(msg)
			if cm, ok := updated.(configModel); ok {
				m.cfg = &cm
			}
			return m, cmd
		}
	case screenRepos:
		if m.repos != nil {
			updated, cmd := m.repos.Update(msg)
			if rm, ok := updated.(reposModel); ok {
				m.repos = &rm
			}
			return m, cmd
		}
	case screenPrompt:
		if m.prompt != nil {
			updated, cmd := m.prompt.Update(msg)
			if pm, ok := updated.(promptPickerModel); ok {
				m.prompt = &pm
			}
			return m, cmd
		}
	}
	return m, nil
}

// navigateTo tears down the current screen, switches to target, initialises the
// target sub-model, and returns the model plus the target's Init() command.
func (m appModel) navigateTo(target screenID) (tea.Model, tea.Cmd) {
	m.teardownScreen(m.active)
	m.active = target
	cmd := m.initScreen(target)
	return m, cmd
}

// teardownScreen closes watchers and unsubscribes channels for screen s.
// It is a no-op when the sub-model for s is nil.
func (m *appModel) teardownScreen(s screenID) {
	switch s {
	case screenMenu:
		if m.menu != nil {
			if m.menu.watcher != nil {
				m.menu.watcher.Close()
				close(m.menu.watcherCh)
			}
			if daemonCache != nil && m.menu.daemonCacheCh != nil {
				daemonCache.Unsubscribe(m.menu.daemonCacheCh)
			}
			m.menu = nil
		}
	case screenStatus:
		if m.status != nil {
			if m.status.watcher != nil {
				m.status.watcher.Close()
			}
			if m.status.logWatcher != nil {
				m.status.logWatcher.Stop()
			}
			if daemonCache != nil && m.status.daemonCacheCh != nil {
				daemonCache.Unsubscribe(m.status.daemonCacheCh)
			}
			m.status = nil
		}
	case screenConfig:
		m.cfg = nil
	case screenRepos:
		if m.repos != nil {
			for _, c := range m.repos.daemonCaches {
				if c != nil {
					c.Stop()
				}
			}
			m.repos = nil
		}
	case screenPrompt:
		m.prompt = nil
	}
}

// initScreen creates (or re-creates) the sub-model for screen s, populates it with
// the resources it needs (watchers, daemon subscriptions), and returns Init().
func (m *appModel) initScreen(s screenID) tea.Cmd {
	switch s {
	case screenMenu:
		mm := newMenuModel(loadFeatureSummary())
		m.menu = &mm
		return m.menu.Init()

	case screenStatus:
		sm, err := buildStatusModel()
		if err != nil {
			// Fall back to an empty status model rather than crashing.
			fmt.Fprintf(os.Stderr, "warning: status init: %v\n", err)
			empty := statusModel{}
			m.status = &empty
			return nil
		}
		m.status = sm
		return m.status.Init()

	case screenConfig:
		dir, _ := os.Getwd()
		cfg, _ := config.Load(dir)
		cm := newConfigModel(cfg, dir)
		m.cfg = &cm
		return m.cfg.Init()

	case screenRepos:
		rm := newReposModel()
		m.repos = &rm
		return m.repos.Init()

	case screenPrompt:
		dir, _ := os.Getwd()
		cfg, _ := config.Load(dir)
		resolvedModel := config.ResolveModel(cfg.Model)
		agentName := cfg.Agent
		if agentName == "" {
			agentName = "claude"
		}
		pm := newPromptPickerModel(dir, resolvedModel, agentName)
		m.prompt = &pm
		return m.prompt.Init()
	}
	return nil
}

// buildStatusModel constructs a fully wired statusModel, mirroring the setup in
// runStatus(). It is used by appModel.initScreen when navigating to screenStatus.
func buildStatusModel() (*statusModel, error) {
	dir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("get working directory: %w", err)
	}

	cfg, err := config.Load(dir)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	agentName := cfg.Agent
	approvalRequired := cfg.IsApprovalRequired()

	featureStore := stores.NewFileFeatureStore(dir)
	bugStore := stores.NewFileBugStore(dir)

	features, approvals, err := loadPlansWithApprovals(dir, featureStore, bugStore, true)
	if err != nil {
		return nil, fmt.Errorf("load plans: %w", err)
	}
	pruneStaleApprovals(dir, features)

	nextTaskID, nextTaskFile := findNextTask(features)

	watcherCh := make(chan bool, 1)
	w, _ := filewatcher.New(dir, func(msg any) {
		hasNew := false
		if u, ok := msg.(filewatcher.UpdateMsg); ok {
			hasNew = u.HasNewFile
		}
		select {
		case watcherCh <- hasNew:
		default:
		}
	}, 300*time.Millisecond)

	var daemonCacheCh chan daemonPIDState
	if daemonCache != nil {
		daemonCacheCh = daemonCache.Subscribe()
	}

	logWatcher, _ := NewLogFileWatcher(dir)

	sm := newStatusModel(features, false, nextTaskID, nextTaskFile, agentName, dir, false, approvalRequired, approvals, featureStore, bugStore)
	sm.presence = sharedPresence
	sm.watcherCh = watcherCh
	sm.watcher = w
	sm.daemonCacheCh = daemonCacheCh
	if logWatcher != nil {
		sm.logWatcher = logWatcher
		sm.logWatcherCh = logWatcher.Chan()
	}
	if daemonCache != nil {
		cached := daemonCache.Get()
		sm.daemon.PID = cached.PID
		sm.daemon.Running = cached.Running
	}

	return &sm, nil
}
