package tui

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/leberkas-org/maggus/internal/config"
	"github.com/leberkas-org/maggus/internal/ipc"
	"github.com/leberkas-org/maggus/internal/tui/pane"
)

type stateMsg ipc.DaemonSnapshot
type tickMsg time.Time

type App struct {
	width, height int
	left          *pane.LeftPane
	right         *pane.RightPane
	footer        *pane.FooterPane
	focus         FocusTarget
	stateReader   ipc.StateReader
	modal         *QuitModal
	lastSnap      ipc.DaemonSnapshot
}

func NewApp(reader ipc.StateReader) *App {
	left := pane.NewLeftPane()
	left.Focus()

	return &App{
		left:        left,
		right:       pane.NewRightPane(),
		footer:      pane.NewFooterPane(),
		focus:       FocusLeft,
		stateReader: reader,
	}
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

	case tickMsg:
		return a, a.tick()

	case tea.KeyMsg:
		if a.modal != nil {
			return a.handleModalKey(msg)
		}
		return a.handleKey(msg)
	}

	return a, nil
}

func (a *App) View() string {
	if a.width == 0 || a.height == 0 {
		return ""
	}

	if a.modal != nil {
		a.modal.width = a.width
		a.modal.height = a.height
		return a.modal.View()
	}

	leftW, rightW := splitWidth(a.width)
	innerH := a.height - 1 // footer

	a.left.Resize(leftW, innerH)
	a.right.Resize(rightW, innerH)

	leftView := a.left.View()
	rightView := a.right.View()

	content := lipgloss.JoinHorizontal(lipgloss.Top, leftView, rightView)
	a.footer.Resize(a.width, 1)
	footer := a.footer.View()

	return content + "\n" + footer
}

func (a *App) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		a.modal = &QuitModal{}
		return a, nil
	case "tab":
		a.toggleFocus()
		return a, nil
	case "a":
		if sel := a.left.Selected(); sel != nil {
			return a, a.approveItem(sel.ID)
		}
	case "x":
		if sel := a.left.Selected(); sel != nil {
			return a, a.skipItem(sel.ID)
		}
	}

	// Route to focused pane
	var cmd tea.Cmd
	if a.focus == FocusLeft {
		_, cmd = a.left.Update(msg)
	} else {
		_, cmd = a.right.Update(msg)
	}
	return a, cmd
}

func (a *App) handleModalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "s", "S":
		return a, tea.Quit
	case "d", "D":
		return a, tea.Quit
	case "esc", "escape":
		a.modal = nil
		return a, nil
	}
	return a, nil
}

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
	leftW, rightW := splitWidth(a.width)
	innerH := a.height - 1
	a.left.Resize(leftW, innerH)
	a.right.Resize(rightW, innerH)
	a.footer.Resize(a.width, 1)
}

func (a *App) updateFooter() {
	s := a.lastSnap
	status := fmt.Sprintf("Daemon running | %d tasks active", s.ActiveTasks)
	if s.BryanConnected {
		status += " | Bryan: connected"
	}
	a.footer.SetStatus(status)
}

func (a *App) watchState() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		ch := a.stateReader.Watch(ctx)
		select {
		case snap := <-ch:
			return stateMsg(snap)
		case <-ctx.Done():
			return nil
		}
	}
}

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

func getGlobalDir() (string, error) {
	return config.GlobalDir()
}
