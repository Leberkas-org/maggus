package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/leberkas-org/maggus/internal/parser"
	"github.com/leberkas-org/maggus/internal/tui/styles"
)

// taskListAction signals what happened after an Update call.
type taskListAction int

const (
	taskListNone      taskListAction = iota
	taskListQuit                     // user wants to quit the program
	taskListRun                      // user wants to run the current task (check RunTaskID)
	taskListDeleted                  // current task was deleted from disk; parent should reload
	taskListUnhandled                // key was not consumed; parent should handle it
)

// taskListComponent is a reusable sub-component for browsing a list of tasks
// with detail view, criteria mode, action picker, and delete confirmation.
// Both listModel and statusModel can embed this to eliminate duplicated logic.
type taskListComponent struct {
	Tasks        []parser.Task
	Cursor       int
	ScrollOffset int
	Width        int
	Height       int
	HeaderLines  int // header lines above the task list (differs between list and status)
	AgentName    string

	// Detail view state
	ShowDetail     bool
	detailViewport viewport.Model
	detailReady    bool
	Detail         detailState

	// Delete confirmation
	ConfirmDelete bool
	DeleteErr     string

	// Extra content appended to detail viewport (e.g. status note)
	DetailSuffix string

	// BorderColor overrides the default border color for full-screen views.
	// When zero-value, defaults to styles.Primary.
	BorderColor lipgloss.Color

	// Run action — set when user presses Alt+R
	RunTaskID string
}

// effectiveBorderColor returns the border color to use, defaulting to Primary.
func (c *taskListComponent) effectiveBorderColor() lipgloss.Color {
	if c.BorderColor != "" {
		return c.BorderColor
	}
	return styles.Primary
}

// visibleTaskLines returns how many task lines fit in the visible area.
func (c *taskListComponent) visibleTaskLines() int {
	footerLines := 1
	_, innerH := styles.FullScreenInnerSize(c.Width, c.Height)
	avail := innerH - c.HeaderLines - footerLines
	if avail < 1 {
		avail = 1
	}
	return avail
}

// ensureCursorVisible adjusts ScrollOffset so Cursor is within the visible window.
func (c *taskListComponent) ensureCursorVisible() {
	visible := c.visibleTaskLines()
	if c.Cursor < c.ScrollOffset {
		c.ScrollOffset = c.Cursor
	}
	if c.Cursor >= c.ScrollOffset+visible {
		c.ScrollOffset = c.Cursor - visible + 1
	}
	if c.ScrollOffset < 0 {
		c.ScrollOffset = 0
	}
}

// refreshDetailViewport re-renders the detail content for the current task.
func (c *taskListComponent) refreshDetailViewport() {
	content := renderDetailContent(c.Tasks[c.Cursor], &c.Detail)
	if c.DetailSuffix != "" {
		content += c.DetailSuffix
	}
	c.detailViewport.SetContent(content)
}

// openDetail opens the detail view for the current task.
func (c *taskListComponent) openDetail() {
	c.ShowDetail = true
	c.Detail = detailState{}
	content := renderDetailContent(c.Tasks[c.Cursor], &c.Detail)
	if c.DetailSuffix != "" {
		content += c.DetailSuffix
	}
	w, h := styles.FullScreenInnerSize(c.Width, c.Height)
	c.detailViewport = viewport.New(w, h-1)
	c.detailViewport.SetContent(content)
	c.detailReady = true
}

// closeDetail closes the detail view and resets state.
func (c *taskListComponent) closeDetail() {
	c.ShowDetail = false
	c.detailReady = false
	c.Detail.exitCriteriaMode()
}

// HandleResize updates the viewport dimensions when the window size changes.
func (c *taskListComponent) HandleResize(width, height int) {
	c.Width = width
	c.Height = height
	if c.ShowDetail {
		w, h := styles.FullScreenInnerSize(width, height)
		c.detailViewport.Width = w
		c.detailViewport.Height = h - 1
		c.detailReady = true
	}
}

// UpdateViewport forwards non-key messages to the detail viewport.
func (c *taskListComponent) UpdateViewport(msg tea.Msg) tea.Cmd {
	if c.ShowDetail && c.detailReady {
		var cmd tea.Cmd
		c.detailViewport, cmd = c.detailViewport.Update(msg)
		return cmd
	}
	return nil
}

// Update handles all component key messages, dispatching to the appropriate handler.
// Returns taskListUnhandled if the key was not consumed by the component.
func (c *taskListComponent) Update(msg tea.KeyMsg) (tea.Cmd, taskListAction) {
	if c.ConfirmDelete {
		return c.updateConfirmDelete(msg)
	}
	if c.ShowDetail {
		return c.updateDetail(msg)
	}
	return c.updateListNav(msg)
}

// updateListNav handles common list navigation keys (up/down/home/end/enter).
func (c *taskListComponent) updateListNav(msg tea.KeyMsg) (tea.Cmd, taskListAction) {
	switch msg.String() {
	case "alt+r":
		if len(c.Tasks) > 0 {
			c.RunTaskID = c.Tasks[c.Cursor].ID
			return nil, taskListRun
		}
		return nil, taskListNone
	case "alt+backspace":
		if len(c.Tasks) > 0 {
			c.ConfirmDelete = true
			c.DeleteErr = ""
		}
		return nil, taskListNone
	case "up", "k":
		c.Cursor = styles.CursorUp(c.Cursor, len(c.Tasks))
		c.ensureCursorVisible()
		return nil, taskListNone
	case "down", "j":
		c.Cursor = styles.CursorDown(c.Cursor, len(c.Tasks))
		c.ensureCursorVisible()
		return nil, taskListNone
	case "home":
		c.Cursor = 0
		c.ensureCursorVisible()
		return nil, taskListNone
	case "end":
		if len(c.Tasks) > 0 {
			c.Cursor = len(c.Tasks) - 1
		}
		c.ensureCursorVisible()
		return nil, taskListNone
	case "enter":
		if len(c.Tasks) > 0 {
			c.openDetail()
		}
		return nil, taskListNone
	case "q":
		return nil, taskListQuit
	}
	return nil, taskListUnhandled
}

// updateDetail handles keys in the detail view.
// Returns taskListUnhandled for keys the parent should handle.
func (c *taskListComponent) updateDetail(msg tea.KeyMsg) (tea.Cmd, taskListAction) {
	// Handle action picker mode
	if c.Detail.showActionPicker {
		return c.updateActionPicker(msg)
	}

	// Handle criteria mode
	if c.Detail.criteriaMode {
		return c.updateCriteriaMode(msg)
	}

	switch msg.String() {
	case "alt+r":
		if len(c.Tasks) > 0 {
			c.RunTaskID = c.Tasks[c.Cursor].ID
			return nil, taskListRun
		}
		return nil, taskListNone
	case "alt+backspace":
		c.ConfirmDelete = true
		c.DeleteErr = ""
		return nil, taskListNone
	case "q", "backspace":
		c.closeDetail()
		return nil, taskListNone
	case "tab", "b":
		c.Detail.noBlockedMsg = false
		if !c.Detail.initCriteriaMode(c.Tasks[c.Cursor]) {
			c.Detail.noBlockedMsg = true
			c.refreshDetailViewport()
			return nil, taskListNone
		}
		c.refreshDetailViewport()
		return nil, taskListNone
	case "pgdown":
		if c.Cursor < len(c.Tasks)-1 {
			c.Cursor++
			c.Detail.exitCriteriaMode()
			c.refreshDetailViewport()
		}
		return nil, taskListNone
	case "pgup":
		if c.Cursor > 0 {
			c.Cursor--
			c.Detail.exitCriteriaMode()
			c.refreshDetailViewport()
		}
		return nil, taskListNone
	case "home":
		if c.detailReady {
			c.detailViewport.GotoTop()
			return nil, taskListNone
		}
	case "end":
		if c.detailReady {
			c.detailViewport.GotoBottom()
			return nil, taskListNone
		}
	}

	// Forward to viewport for scroll handling
	if c.detailReady {
		var cmd tea.Cmd
		c.detailViewport, cmd = c.detailViewport.Update(msg)
		return cmd, taskListNone
	}
	return nil, taskListUnhandled
}

func (c *taskListComponent) updateCriteriaMode(msg tea.KeyMsg) (tea.Cmd, taskListAction) {
	switch msg.String() {
	case "up", "k":
		c.Detail.criteriaCursor = styles.ClampCursor(c.Detail.criteriaCursor-1, len(c.Detail.blockedIndices))
		c.refreshDetailViewport()
	case "down", "j":
		c.Detail.criteriaCursor = styles.ClampCursor(c.Detail.criteriaCursor+1, len(c.Detail.blockedIndices))
		c.refreshDetailViewport()
	case "enter":
		c.Detail.showActionPicker = true
		c.Detail.actionCursor = 0
		c.refreshDetailViewport()
	case "tab":
		c.Detail.exitCriteriaMode()
		c.refreshDetailViewport()
	case "q", "backspace":
		c.closeDetail()
		return nil, taskListNone
	}
	return nil, taskListNone
}

func (c *taskListComponent) updateActionPicker(msg tea.KeyMsg) (tea.Cmd, taskListAction) {
	switch msg.String() {
	case "up", "k":
		c.Detail.actionCursor = styles.ClampCursor(c.Detail.actionCursor-1, len(criteriaActions))
		c.refreshDetailViewport()
	case "down", "j":
		c.Detail.actionCursor = styles.ClampCursor(c.Detail.actionCursor+1, len(criteriaActions))
		c.refreshDetailViewport()
	case "enter":
		action := criteriaActions[c.Detail.actionCursor]
		modified, _ := c.Detail.performAction(c.Tasks[c.Cursor], action)
		c.Detail.showActionPicker = false
		if modified {
			// Reload task from disk
			if updated := reloadTask(c.Tasks[c.Cursor].SourceFile, c.Tasks[c.Cursor].ID); updated != nil {
				c.Tasks[c.Cursor] = *updated
			}
			// Re-init criteria mode with updated task
			if !c.Detail.initCriteriaMode(c.Tasks[c.Cursor]) {
				c.Detail.exitCriteriaMode()
			}
		}
		c.refreshDetailViewport()
	case "esc":
		c.Detail.showActionPicker = false
		c.refreshDetailViewport()
	}
	return nil, taskListNone
}

// updateConfirmDelete handles keys in the delete confirmation dialog.
// Returns taskListDeleted if the task was successfully deleted from disk.
// The parent is responsible for any additional cleanup (e.g. reloading plans).
func (c *taskListComponent) updateConfirmDelete(msg tea.KeyMsg) (tea.Cmd, taskListAction) {
	switch msg.String() {
	case "y", "Y", "enter":
		t := c.Tasks[c.Cursor]
		if err := parser.DeleteTask(t.SourceFile, t.ID); err != nil {
			c.DeleteErr = err.Error()
			c.ConfirmDelete = false
			return nil, taskListNone
		}
		// Remove from local list and adjust cursor
		c.Tasks = append(c.Tasks[:c.Cursor], c.Tasks[c.Cursor+1:]...)
		if c.Cursor >= len(c.Tasks) && c.Cursor > 0 {
			c.Cursor--
		}
		c.ConfirmDelete = false
		c.ShowDetail = false
		if len(c.Tasks) == 0 {
			return nil, taskListQuit
		}
		return nil, taskListDeleted
	case "n", "N", "esc":
		c.ConfirmDelete = false
		return nil, taskListNone
	}
	return nil, taskListNone
}

// View renders the current view based on component state.
// Returns empty string when in list mode — the parent should render the list itself.
func (c *taskListComponent) View() string {
	if c.ConfirmDelete {
		return c.viewConfirmDelete()
	}
	if c.ShowDetail {
		return c.viewDetail()
	}
	return ""
}

// viewDetail renders the detail view.
func (c *taskListComponent) viewDetail() string {
	if !c.detailReady {
		return ""
	}

	scrollable := c.detailViewport.TotalLineCount() > c.detailViewport.Height
	footer := detailFooter(&c.Detail, scrollable)

	bc := c.effectiveBorderColor()
	if c.Width > 0 && c.Height > 0 {
		return styles.FullScreenLeftColor(c.detailViewport.View(), footer, c.Width, c.Height, bc)
	}
	return styles.Box.BorderForeground(bc).Render(c.detailViewport.View()+"\n"+footer) + "\n"
}

// viewConfirmDelete renders the delete confirmation dialog.
func (c *taskListComponent) viewConfirmDelete() string {
	t := c.Tasks[c.Cursor]
	warnStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.Warning)
	mutedStyle := lipgloss.NewStyle().Foreground(styles.Muted)

	var sb strings.Builder
	sb.WriteString(warnStyle.Render(fmt.Sprintf("Delete %s: %s?", t.ID, t.Title)))
	sb.WriteString("\n\n")
	sb.WriteString(mutedStyle.Render(fmt.Sprintf("  Plan: %s", filepath.Base(t.SourceFile))))
	sb.WriteString("\n\n")
	sb.WriteString("  This will permanently remove the task from the plan file.\n\n")
	sb.WriteString(fmt.Sprintf("  %s / %s",
		lipgloss.NewStyle().Bold(true).Render("y/enter: confirm"),
		mutedStyle.Render("n/esc: cancel")))

	bc := c.effectiveBorderColor()
	if c.Width > 0 && c.Height > 0 {
		return styles.FullScreenColor(sb.String(), "", c.Width, c.Height, bc)
	}
	return styles.Box.BorderForeground(bc).Render(sb.String()) + "\n"
}

// CurrentTask returns the task at the current cursor position, or nil if no tasks.
func (c *taskListComponent) CurrentTask() *parser.Task {
	if len(c.Tasks) == 0 || c.Cursor >= len(c.Tasks) {
		return nil
	}
	return &c.Tasks[c.Cursor]
}
