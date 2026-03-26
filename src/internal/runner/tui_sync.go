package runner

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/leberkas-org/maggus/internal/tui/styles"
)

// SyncCheckMsg tells the TUI to show the sync resolution screen between tasks.
// The work goroutine blocks on ResultCh until the user makes a choice.
type SyncCheckMsg struct {
	Behind       int
	Ahead        int
	RemoteBranch string
	ResultCh     chan<- SyncCheckResult
}

// SyncCheckResult is the user's resolution choice sent back to the work goroutine.
type SyncCheckResult struct {
	Action  SyncAction
	Message string // info message (e.g. "Pulled 3 commits")
	Err     error  // non-nil if the pull action failed fatally
}

// SyncAction represents the user's choice during a between-task sync check.
type SyncAction int

const (
	SyncProceed SyncAction = iota // continue (pull succeeded, skip, or up-to-date)
	SyncAbort                     // user chose to abort
)

// syncActionDoneMsg is an internal message sent after a sync pull action completes.
type syncActionDoneMsg struct {
	err     error
	message string
}

// syncMenuOption represents one item in the sync resolution menu.
type syncMenuOption struct {
	label string
	desc  string
}

// syncState holds all state for the between-task sync resolution screen.
type syncState struct {
	active       bool                   // true when showing sync resolution screen
	behind       int                    // commits behind remote
	ahead        int                    // commits ahead of remote
	remoteBranch string                 // remote branch name
	resultCh     chan<- SyncCheckResult // channel to send result back to work goroutine
	options      []syncMenuOption       // resolution menu options
	cursor       int                    // selected menu item
	confirmForce bool                   // showing force-pull confirmation
	running      bool                   // executing a pull action
	errorMsg     string                 // error from failed action (returns to menu)
	dir          string                 // directory for git operations
}

// syncPullFunc, syncPullRebaseFunc, syncForcePullFunc are package-level vars
// pointing to gitsync functions. They are set by InitSyncFuncs from the cmd package.
var (
	syncPullFunc       func(string) error
	syncPullRebaseFunc func(string) error
	syncForcePullFunc  func(string, bool) error
)

// InitSyncFuncs sets the git sync functions used by the TUI's between-task sync screen.
// This should be called once at startup from the cmd package to inject the gitsync functions.
func InitSyncFuncs(pull func(string) error, pullRebase func(string) error, forcePull func(string, bool) error) {
	syncPullFunc = pull
	syncPullRebaseFunc = pullRebase
	syncForcePullFunc = forcePull
}

// buildOptions populates the sync resolution menu.
func (s *syncState) buildOptions() {
	s.options = []syncMenuOption{
		{label: "Pull", desc: "git pull"},
		{label: "Pull with rebase", desc: "git pull --rebase"},
		{label: "Force pull", desc: "reset to remote (discards ALL local commits and changes)"},
		{label: "Skip", desc: "continue without pulling"},
		{label: "Abort", desc: "stop maggus work"},
	}
	s.cursor = 0
}

// handleSyncMsg handles sync-related messages in Update(), returning true if the message was handled.
func (s *syncState) handleSyncMsg(msg tea.Msg, infoMessages *[]string) (handled bool, cmd tea.Cmd) {
	switch msg := msg.(type) {
	case SyncCheckMsg:
		s.active = true
		s.behind = msg.Behind
		s.ahead = msg.Ahead
		s.remoteBranch = msg.RemoteBranch
		s.resultCh = msg.ResultCh
		s.confirmForce = false
		s.running = false
		s.errorMsg = ""
		s.buildOptions()
		return true, nil

	case syncActionDoneMsg:
		s.running = false
		if msg.err != nil {
			s.errorMsg = msg.err.Error()
			s.buildOptions() // rebuild menu so user can retry
			return true, nil
		}
		// Action succeeded — send result and exit sync mode
		if s.resultCh != nil {
			s.resultCh <- SyncCheckResult{Action: SyncProceed, Message: msg.message}
			s.resultCh = nil
		}
		s.active = false
		*infoMessages = append(*infoMessages, msg.message)
		return true, nil
	}
	return false, nil
}

// handleSyncKeys processes key events while the sync screen is active.
func (s *syncState) handleSyncKeys(msg tea.KeyMsg, cancelFunc *func()) (tea.Cmd, bool) {
	if msg.Type == tea.KeyCtrlC {
		// Ctrl+C during sync aborts the run
		if s.resultCh != nil {
			s.resultCh <- SyncCheckResult{Action: SyncAbort}
			s.resultCh = nil
		}
		s.active = false
		if *cancelFunc != nil {
			(*cancelFunc)()
			*cancelFunc = nil
		}
		return nil, true // statusInterrupting=true
	}

	if s.running {
		return nil, false // ignore keys while action runs
	}

	if s.confirmForce {
		switch msg.String() {
		case "y", "Y":
			s.confirmForce = false
			s.running = true
			return s.runForcePull, false
		case "n", "N", "esc":
			s.confirmForce = false
			return nil, false
		}
		return nil, false
	}

	switch msg.Type {
	case tea.KeyUp:
		if s.cursor > 0 {
			s.cursor--
		}
		return nil, false
	case tea.KeyDown:
		if s.cursor < len(s.options)-1 {
			s.cursor++
		}
		return nil, false
	case tea.KeyEnter:
		return s.selectOption(cancelFunc)
	case tea.KeyEsc:
		if s.resultCh != nil {
			s.resultCh <- SyncCheckResult{Action: SyncAbort}
			s.resultCh = nil
		}
		s.active = false
		if *cancelFunc != nil {
			(*cancelFunc)()
			*cancelFunc = nil
		}
		return nil, true
	}

	if len(msg.Runes) == 1 {
		switch msg.Runes[0] {
		case 'q':
			if s.resultCh != nil {
				s.resultCh <- SyncCheckResult{Action: SyncAbort}
				s.resultCh = nil
			}
			s.active = false
			if *cancelFunc != nil {
				(*cancelFunc)()
				*cancelFunc = nil
			}
			return nil, true
		case 'k':
			if s.cursor > 0 {
				s.cursor--
			}
		case 'j':
			if s.cursor < len(s.options)-1 {
				s.cursor++
			}
		}
	}

	return nil, false
}

// selectOption handles the user pressing Enter on a sync menu option.
func (s *syncState) selectOption(cancelFunc *func()) (tea.Cmd, bool) {
	if s.cursor >= len(s.options) {
		return nil, false
	}
	opt := s.options[s.cursor]

	switch opt.label {
	case "Pull":
		s.running = true
		s.errorMsg = ""
		return s.runPull, false
	case "Pull with rebase":
		s.running = true
		s.errorMsg = ""
		return s.runPullRebase, false
	case "Force pull":
		s.confirmForce = true
		return nil, false
	case "Skip":
		if s.resultCh != nil {
			s.resultCh <- SyncCheckResult{Action: SyncProceed, Message: "Skipped git sync"}
			s.resultCh = nil
		}
		s.active = false
		return nil, false
	case "Abort":
		if s.resultCh != nil {
			s.resultCh <- SyncCheckResult{Action: SyncAbort}
			s.resultCh = nil
		}
		s.active = false
		if *cancelFunc != nil {
			(*cancelFunc)()
			*cancelFunc = nil
		}
		return nil, true
	}

	return nil, false
}

// runPull executes git pull as a tea.Cmd.
func (s *syncState) runPull() tea.Msg {
	err := syncPullFunc(s.dir)
	if err != nil {
		return syncActionDoneMsg{err: err}
	}
	return syncActionDoneMsg{message: fmt.Sprintf("Pulled %d commit(s) from %s", s.behind, s.remoteBranch)}
}

// runPullRebase executes git pull --rebase as a tea.Cmd.
func (s *syncState) runPullRebase() tea.Msg {
	err := syncPullRebaseFunc(s.dir)
	if err != nil {
		return syncActionDoneMsg{err: err}
	}
	return syncActionDoneMsg{message: fmt.Sprintf("Rebased onto %s (%d commit(s))", s.remoteBranch, s.behind)}
}

// runForcePull executes force pull (fetch + reset --hard) as a tea.Cmd.
func (s *syncState) runForcePull() tea.Msg {
	err := syncForcePullFunc(s.dir, true)
	if err != nil {
		return syncActionDoneMsg{err: err}
	}
	return syncActionDoneMsg{message: fmt.Sprintf("Force-pulled: reset to %s", s.remoteBranch)}
}

// renderSyncView renders the between-task sync resolution screen.
func (s *syncState) renderSyncView(m *TUIModel) string {
	innerW, _ := styles.FullScreenInnerSize(m.width, m.height)
	if innerW < 40 {
		innerW = 40
	}

	var b strings.Builder

	// Show header (preserves context)
	b.WriteString(m.renderHeaderInner(innerW))
	b.WriteString("\n")

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.Primary)
	b.WriteString(titleStyle.Render("Git Sync — Remote Changed") + "\n")
	b.WriteString(styles.Separator(innerW) + "\n\n")

	labelStyle := lipgloss.NewStyle().Bold(true).Width(10).Align(lipgloss.Right)
	valStyle := lipgloss.NewStyle().Foreground(styles.Muted)
	errStyle := lipgloss.NewStyle().Foreground(styles.Error)
	warnStyle := lipgloss.NewStyle().Foreground(styles.Warning)

	// Remote status
	statusParts := []string{}
	if s.behind > 0 {
		statusParts = append(statusParts, errStyle.Render(fmt.Sprintf("%d behind", s.behind)))
	}
	if s.ahead > 0 {
		statusParts = append(statusParts, warnStyle.Render(fmt.Sprintf("%d ahead", s.ahead)))
	}
	b.WriteString(fmt.Sprintf("%s  %s (%s)\n\n", labelStyle.Render("Remote:"), valStyle.Render(s.remoteBranch), strings.Join(statusParts, ", ")))

	if s.running {
		spinner := lipgloss.NewStyle().Foreground(styles.Primary).Render(styles.SpinnerFrames[m.frame])
		b.WriteString(fmt.Sprintf("%s Running...\n", spinner))
	} else if s.confirmForce {
		b.WriteString(errStyle.Render("⚠ Force Pull Confirmation") + "\n\n")
		b.WriteString(errStyle.Render("This will discard ALL local commits and changes. Are you sure?") + "\n\n")
		b.WriteString(fmt.Sprintf("  %s to confirm, %s to cancel\n",
			lipgloss.NewStyle().Bold(true).Foreground(styles.Error).Render("y"),
			valStyle.Render("n/esc")))
	} else {
		if s.errorMsg != "" {
			b.WriteString(errStyle.Render("✗ "+s.errorMsg) + "\n\n")
		}

		for i, opt := range s.options {
			cursor := "  "
			nameStyle := lipgloss.NewStyle().Foreground(styles.Muted)
			descStyle := valStyle
			if i == s.cursor {
				cursor = lipgloss.NewStyle().Bold(true).Foreground(styles.Primary).Render("→ ")
				nameStyle = lipgloss.NewStyle().Bold(true).Foreground(styles.Primary)
			}
			b.WriteString(fmt.Sprintf("%s%s  %s\n", cursor, nameStyle.Render(opt.label), descStyle.Render(opt.desc)))
		}
	}

	// Footer
	var footer string
	if s.confirmForce {
		footer = styles.StatusBar.Render("y: confirm · n/esc: cancel")
	} else if !s.running {
		footer = styles.StatusBar.Render("↑/↓: select · enter: confirm · q/esc: abort")
	}

	if m.width > 0 && m.height > 0 {
		return styles.FullScreenLeft(b.String(), footer, m.width, m.height)
	}
	return styles.Box.Render(b.String()) + "\n"
}
