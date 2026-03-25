package cmd

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/leberkas-org/maggus/internal/gitsync"
	"github.com/leberkas-org/maggus/internal/gitutil"
	"github.com/leberkas-org/maggus/internal/tui/styles"
)

// syncAction is the result of the git sync TUI.
type syncAction int

const (
	syncProceed syncAction = iota // up-to-date or user chose Skip
	syncAbort                     // user chose Abort
)

// syncResult holds the outcome of the git sync check.
type syncResult struct {
	action  syncAction
	message string // info message to display in the work TUI (e.g. "Pulled 3 commits")
}

// syncOption represents one item in the resolution menu.
type syncOption struct {
	label       string
	desc        string
	warning     bool // show a warning marker
	recommended bool
}

// syncState tracks the phase of the sync TUI.
type syncState int

const (
	syncStateLoading      syncState = iota // fetching remote status
	syncStateClean                         // up-to-date and clean, auto-proceeding
	syncStateMenu                          // showing resolution menu
	syncStateDirtyOnly                     // uncommitted changes but up-to-date
	syncStateConfirmForce                  // confirming force pull
	syncStateRunning                       // executing a pull action
	syncStateDone                          // action complete, proceeding
	syncStateError                         // action failed, returning to menu
)

var syncSpinner = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// syncModel is the bubbletea model for the git sync screen.
type syncModel struct {
	dir    string
	width  int
	height int
	frame  int

	state    syncState
	result   syncResult
	branch   string
	remote   gitsync.Status
	workTree gitsync.WorkTree
	fetchErr error // non-nil if fetch failed (offline mode)

	options  []syncOption
	cursor   int
	errorMsg string

	autoTimer int // countdown ticks for auto-proceed (clean state)
}

// Message types for async operations.
type syncFetchDoneMsg struct {
	remote   gitsync.Status
	workTree gitsync.WorkTree
	branch   string
	fetchErr error
}

type syncActionDoneMsg struct {
	err     error
	message string
}

type syncTickMsg time.Time

func newSyncModel(dir string) syncModel {
	return syncModel{
		dir:    dir,
		width:  120,
		height: 40,
		state:  syncStateLoading,
	}
}

func (m syncModel) Init() tea.Cmd {
	return tea.Batch(
		m.fetchStatus,
		syncTickCmd(),
	)
}

func syncTickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return syncTickMsg(t)
	})
}

func (m syncModel) fetchStatus() tea.Msg {
	// Get current branch
	branchCmd := gitutil.Command("rev-parse", "--abbrev-ref", "HEAD")
	branchCmd.Dir = m.dir
	branchOut, _ := branchCmd.Output()
	branch := strings.TrimSpace(string(branchOut))

	// Fetch remote (may fail if offline)
	fetchErr := gitsync.FetchRemote(m.dir)

	// Get remote status
	remote, err := gitsync.RemoteStatus(m.dir)
	if err != nil {
		remote = gitsync.Status{HasRemote: false}
	}

	// Get working tree status
	workTree, err := gitsync.WorkingTreeStatus(m.dir)
	if err != nil {
		workTree = gitsync.WorkTree{}
	}

	return syncFetchDoneMsg{
		remote:   remote,
		workTree: workTree,
		branch:   branch,
		fetchErr: fetchErr,
	}
}

func (m syncModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case syncTickMsg:
		m.frame = (m.frame + 1) % len(syncSpinner)
		if m.state == syncStateClean {
			m.autoTimer++
			// Auto-proceed after ~1.5 seconds (15 ticks at 100ms)
			if m.autoTimer >= 15 {
				m.result = syncResult{action: syncProceed}
				return m, tea.Quit
			}
		}
		return m, syncTickCmd()

	case syncFetchDoneMsg:
		m.branch = msg.branch
		m.remote = msg.remote
		m.workTree = msg.workTree
		m.fetchErr = msg.fetchErr
		m.determineState()
		return m, nil

	case syncActionDoneMsg:
		if msg.err != nil {
			m.errorMsg = msg.err.Error()
			m.state = syncStateMenu
			// Rebuild options (state may have changed)
			m.buildOptions()
			return m, nil
		}
		m.result = syncResult{action: syncProceed, message: msg.message}
		m.state = syncStateDone
		return m, tea.Quit

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

func (m *syncModel) determineState() {
	hasDirty := m.workTree.HasUncommittedChanges || m.workTree.HasUntrackedFiles
	isBehind := m.remote.HasRemote && m.remote.Behind > 0

	if !isBehind && !hasDirty {
		// Clean and up-to-date
		m.state = syncStateClean
		return
	}

	if hasDirty && !isBehind {
		// Only local changes, not behind
		m.state = syncStateDirtyOnly
		return
	}

	// Behind remote (possibly also dirty)
	m.state = syncStateMenu
	m.buildOptions()
}

func (m *syncModel) buildOptions() {
	hasDirty := m.workTree.HasUncommittedChanges || m.workTree.HasUntrackedFiles

	m.options = nil
	m.cursor = 0

	if hasDirty {
		m.options = append(m.options,
			syncOption{label: "Pull", desc: "git pull (may fail with uncommitted changes)", warning: true},
			syncOption{label: "Pull with rebase", desc: "git pull --rebase (may fail with uncommitted changes)", warning: true},
			syncOption{label: "Force pull", desc: "reset to remote (discards ALL local commits and changes)", warning: true},
			syncOption{label: "Stash & pull", desc: "stash changes, pull, then restore (recommended)", recommended: true},
		)
		// Default cursor to Stash & pull (recommended)
		m.cursor = 3
	} else {
		m.options = append(m.options,
			syncOption{label: "Pull", desc: "git pull"},
			syncOption{label: "Pull with rebase", desc: "git pull --rebase"},
			syncOption{label: "Force pull", desc: "reset to remote (discards ALL local commits and changes)"},
			syncOption{label: "Stash & pull", desc: "stash changes, pull, then restore"},
		)
	}

	m.options = append(m.options,
		syncOption{label: "Skip", desc: "continue without pulling"},
		syncOption{label: "Abort", desc: "exit maggus work"},
	)
}

func (m syncModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.state {
	case syncStateClean:
		// Any key press proceeds immediately
		m.result = syncResult{action: syncProceed}
		return m, tea.Quit

	case syncStateDirtyOnly:
		switch msg.String() {
		case "enter":
			m.result = syncResult{action: syncProceed, message: "Proceeding with uncommitted changes"}
			return m, tea.Quit
		case "q", "esc":
			m.result = syncResult{action: syncAbort}
			return m, tea.Quit
		}

	case syncStateConfirmForce:
		switch msg.String() {
		case "y", "Y":
			m.state = syncStateRunning
			return m, m.runForcePull
		case "n", "N", "esc":
			m.state = syncStateMenu
			return m, nil
		}

	case syncStateMenu:
		switch msg.Type {
		case tea.KeyUp:
			m.cursor = styles.ClampCursor(m.cursor-1, len(m.options))
			return m, nil
		case tea.KeyDown:
			m.cursor = styles.ClampCursor(m.cursor+1, len(m.options))
			return m, nil
		case tea.KeyEnter:
			return m.selectOption()
		case tea.KeyEsc:
			m.result = syncResult{action: syncAbort}
			return m, tea.Quit
		}
		if len(msg.Runes) == 1 {
			switch msg.Runes[0] {
			case 'q':
				m.result = syncResult{action: syncAbort}
				return m, tea.Quit
			case 'k':
				m.cursor = styles.ClampCursor(m.cursor-1, len(m.options))
			case 'j':
				m.cursor = styles.ClampCursor(m.cursor+1, len(m.options))
			}
		}
	}

	// Global Ctrl+C
	if msg.Type == tea.KeyCtrlC {
		m.result = syncResult{action: syncAbort}
		return m, tea.Quit
	}

	return m, nil
}

func (m syncModel) selectOption() (tea.Model, tea.Cmd) {
	if m.cursor >= len(m.options) {
		return m, nil
	}
	opt := m.options[m.cursor]

	switch opt.label {
	case "Pull":
		m.state = syncStateRunning
		m.errorMsg = ""
		return m, m.runPull
	case "Pull with rebase":
		m.state = syncStateRunning
		m.errorMsg = ""
		return m, m.runPullRebase
	case "Force pull":
		m.state = syncStateConfirmForce
		return m, nil
	case "Stash & pull":
		m.state = syncStateRunning
		m.errorMsg = ""
		return m, m.runStashAndPull
	case "Skip":
		m.result = syncResult{action: syncProceed, message: "Skipped git sync"}
		return m, tea.Quit
	case "Abort":
		m.result = syncResult{action: syncAbort}
		return m, tea.Quit
	}

	return m, nil
}

func (m syncModel) runPull() tea.Msg {
	err := gitsync.Pull(m.dir)
	if err != nil {
		return syncActionDoneMsg{err: err}
	}
	return syncActionDoneMsg{message: fmt.Sprintf("Pulled %d commit(s) from %s", m.remote.Behind, m.remote.RemoteBranch)}
}

func (m syncModel) runPullRebase() tea.Msg {
	err := gitsync.PullRebase(m.dir)
	if err != nil {
		return syncActionDoneMsg{err: err}
	}
	return syncActionDoneMsg{message: fmt.Sprintf("Rebased onto %s (%d commit(s))", m.remote.RemoteBranch, m.remote.Behind)}
}

func (m syncModel) runForcePull() tea.Msg {
	err := gitsync.ForcePull(m.dir, true)
	if err != nil {
		return syncActionDoneMsg{err: err}
	}
	return syncActionDoneMsg{message: fmt.Sprintf("Force-pulled: reset to %s", m.remote.RemoteBranch)}
}

func (m syncModel) runStashAndPull() tea.Msg {
	err := gitsync.StashAndPull(m.dir)
	if err != nil {
		return syncActionDoneMsg{err: err}
	}
	return syncActionDoneMsg{message: fmt.Sprintf("Stashed, pulled %d commit(s), restored changes", m.remote.Behind)}
}

// View renders the sync TUI.
func (m syncModel) View() string {
	innerW, _ := styles.FullScreenInnerSize(m.width, m.height)
	if innerW < 40 {
		innerW = 40
	}

	var b strings.Builder

	// Header
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.Primary)
	b.WriteString(titleStyle.Render("Git Sync") + "\n")
	b.WriteString(styles.Separator(innerW) + "\n\n")

	labelStyle := lipgloss.NewStyle().Bold(true).Width(10).Align(lipgloss.Right)
	valStyle := lipgloss.NewStyle().Foreground(styles.Muted)
	warnStyle := lipgloss.NewStyle().Foreground(styles.Warning)
	errStyle := lipgloss.NewStyle().Foreground(styles.Error)
	successStyle := lipgloss.NewStyle().Foreground(styles.Success)
	mutedStyle := lipgloss.NewStyle().Foreground(styles.Muted)

	switch m.state {
	case syncStateLoading:
		spinner := lipgloss.NewStyle().Foreground(styles.Primary).Render(syncSpinner[m.frame])
		b.WriteString(fmt.Sprintf("%s Checking remote status...\n", spinner))

	case syncStateClean:
		b.WriteString(fmt.Sprintf("%s  %s\n", labelStyle.Render("Branch:"), valStyle.Render(m.branch)))
		if m.remote.HasRemote {
			b.WriteString(fmt.Sprintf("%s  %s\n", labelStyle.Render("Remote:"), valStyle.Render(m.remote.RemoteBranch)))
		}
		if m.fetchErr != nil {
			b.WriteString(fmt.Sprintf("\n%s\n", warnStyle.Render("⚠ Could not reach remote — working offline")))
		} else {
			b.WriteString(fmt.Sprintf("\n%s\n", successStyle.Render("✓ Up to date and clean")))
		}
		b.WriteString(mutedStyle.Render("\nProceeding automatically...") + "\n")

	case syncStateDirtyOnly:
		b.WriteString(fmt.Sprintf("%s  %s\n", labelStyle.Render("Branch:"), valStyle.Render(m.branch)))
		if m.remote.HasRemote {
			b.WriteString(fmt.Sprintf("%s  %s\n", labelStyle.Render("Remote:"), successStyle.Render("up to date")))
		}
		b.WriteString("\n")
		b.WriteString(warnStyle.Render("⚠ Uncommitted changes detected") + "\n\n")
		m.renderChanges(&b, innerW)
		b.WriteString("\n")
		b.WriteString(mutedStyle.Render("You can proceed, but uncommitted changes will not be included in commits.") + "\n")

	case syncStateMenu, syncStateError:
		b.WriteString(fmt.Sprintf("%s  %s\n", labelStyle.Render("Branch:"), valStyle.Render(m.branch)))
		if m.remote.HasRemote {
			statusParts := []string{}
			if m.remote.Behind > 0 {
				statusParts = append(statusParts, errStyle.Render(fmt.Sprintf("%d behind", m.remote.Behind)))
			}
			if m.remote.Ahead > 0 {
				statusParts = append(statusParts, warnStyle.Render(fmt.Sprintf("%d ahead", m.remote.Ahead)))
			}
			b.WriteString(fmt.Sprintf("%s  %s (%s)\n", labelStyle.Render("Remote:"), valStyle.Render(m.remote.RemoteBranch), strings.Join(statusParts, ", ")))
		}
		b.WriteString("\n")

		hasDirty := m.workTree.HasUncommittedChanges || m.workTree.HasUntrackedFiles
		if hasDirty {
			b.WriteString(warnStyle.Render("⚠ Uncommitted changes detected — pull may fail or overwrite work") + "\n\n")
			m.renderChanges(&b, innerW)
			b.WriteString("\n")
		}

		if m.errorMsg != "" {
			b.WriteString(errStyle.Render("✗ "+m.errorMsg) + "\n\n")
		}

		// Render menu options
		for i, opt := range m.options {
			cursor := "  "
			nameStyle := lipgloss.NewStyle().Foreground(styles.Muted)
			descStyle := mutedStyle
			if i == m.cursor {
				cursor = lipgloss.NewStyle().Bold(true).Foreground(styles.Primary).Render("→ ")
				nameStyle = lipgloss.NewStyle().Bold(true).Foreground(styles.Primary)
			}

			label := opt.label
			if opt.warning {
				label += " ⚠"
			}
			if opt.recommended {
				label += " ★"
				if i == m.cursor {
					nameStyle = lipgloss.NewStyle().Bold(true).Foreground(styles.Success)
				} else {
					nameStyle = lipgloss.NewStyle().Foreground(styles.Success)
				}
			}

			b.WriteString(fmt.Sprintf("%s%s  %s\n", cursor, nameStyle.Render(label), descStyle.Render(opt.desc)))
		}

	case syncStateConfirmForce:
		b.WriteString(fmt.Sprintf("%s  %s\n", labelStyle.Render("Branch:"), valStyle.Render(m.branch)))
		b.WriteString("\n")
		b.WriteString(errStyle.Render("⚠ Force Pull Confirmation") + "\n\n")
		b.WriteString(errStyle.Render("This will discard ALL local commits and changes. Are you sure?") + "\n\n")
		b.WriteString(fmt.Sprintf("  %s to confirm, %s to cancel\n",
			lipgloss.NewStyle().Bold(true).Foreground(styles.Error).Render("y"),
			mutedStyle.Render("n/esc")))

	case syncStateRunning:
		b.WriteString(fmt.Sprintf("%s  %s\n", labelStyle.Render("Branch:"), valStyle.Render(m.branch)))
		b.WriteString("\n")
		spinner := lipgloss.NewStyle().Foreground(styles.Primary).Render(syncSpinner[m.frame])
		b.WriteString(fmt.Sprintf("%s Running...\n", spinner))
	}

	// Footer
	var footer string
	switch m.state {
	case syncStateDirtyOnly:
		footer = styles.StatusBar.Render("enter: proceed · q/esc: abort")
	case syncStateMenu:
		footer = styles.StatusBar.Render("↑/↓: select · enter: confirm · q/esc: abort")
	case syncStateConfirmForce:
		footer = styles.StatusBar.Render("y: confirm · n/esc: cancel")
	default:
		footer = ""
	}

	if m.width > 0 && m.height > 0 {
		return styles.FullScreenLeft(b.String(), footer, m.width, m.height)
	}
	return styles.Box.Render(b.String()) + "\n"
}

// renderChanges writes the uncommitted changes summary into the builder.
func (m syncModel) renderChanges(b *strings.Builder, maxWidth int) {
	mutedStyle := lipgloss.NewStyle().Foreground(styles.Muted)
	warnStyle := lipgloss.NewStyle().Foreground(styles.Warning)

	for _, f := range m.workTree.ModifiedFiles {
		line := styles.Truncate(f, maxWidth-4)
		b.WriteString(fmt.Sprintf("  %s %s\n", warnStyle.Render("•"), mutedStyle.Render(line)))
	}
	if m.workTree.TotalModified > len(m.workTree.ModifiedFiles) {
		b.WriteString(fmt.Sprintf("  %s\n",
			mutedStyle.Render(fmt.Sprintf("... and %d more", m.workTree.TotalModified-len(m.workTree.ModifiedFiles)))))
	}
}

// runGitSyncTUI runs the git sync TUI screen and returns the result.
// It creates a separate short-lived bubbletea program.
var runGitSyncTUI = func(dir string) (syncResult, error) {
	m := newSyncModel(dir)
	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return syncResult{action: syncAbort}, fmt.Errorf("sync TUI: %w", err)
	}
	if fm, ok := finalModel.(syncModel); ok {
		return fm.result, nil
	}
	return syncResult{action: syncAbort}, nil
}
