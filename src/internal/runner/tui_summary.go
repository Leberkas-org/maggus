package runner

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/leberkas-org/maggus/internal/tui/styles"
)

// StopReason describes why the work loop ended.
type StopReason int

const (
	StopReasonComplete        StopReason = iota // all requested tasks finished
	StopReasonUserStop                          // user pressed 's' (stop after task)
	StopReasonInterrupted                       // user pressed Ctrl+C
	StopReasonError                             // a task or commit failed
	StopReasonNoTasks                           // no workable tasks found
	StopReasonPartialComplete                   // loop finished but some tasks failed
)

// SummaryData holds information displayed on the post-completion summary screen.
type SummaryData struct {
	RunID          string
	Branch         string
	Model          string
	StartTime      time.Time
	TasksCompleted int
	TasksTotal     int
	CommitStart    string // short hash of first commit
	CommitEnd      string // short hash of last commit
	RemainingTasks []RemainingTask
	Reason         StopReason // why the run ended
	ErrorDetail    string     // error message when Reason == StopReasonError
	Warnings       []string   // non-fatal warnings (e.g. skipped commits)
	FailedTasks    []FailedTask
	TasksFailed    int
}

// RemainingTask is a task that was not completed during the run.
type RemainingTask struct {
	ID         string
	Title      string
	SourceFile string // filename (not full path) of the feature/bug file
}

// FailedTask records a task that could not be completed during the run.
type FailedTask struct {
	ID     string
	Title  string
	Reason string
}

// SummaryMsg tells the TUI to transition to the summary view.
type SummaryMsg struct {
	Data SummaryData
}

// PushStatusMsg updates the push status on the summary screen.
type PushStatusMsg struct {
	Status string // e.g. "Pushed to origin/branch" or "Push failed: reason"
	Done   bool
}

// QuitMsg tells the TUI to transition to the "done" state (waiting for keypress to exit).
type QuitMsg struct{}

// summaryState holds all state for the post-run summary screen.
type summaryState struct {
	show       bool          // true when showing summary view
	data       SummaryData   // summary data from the work loop
	elapsed    time.Duration // frozen elapsed time at summary display
	pushStatus string        // current push status message
	pushDone   bool          // true when push is complete
}

// handleSummaryMsg handles summary-related messages in Update(), returning true if the message was handled.
func (s *summaryState) handleSummaryMsg(msg tea.Msg, m *TUIModel) (handled bool) {
	switch msg := msg.(type) {
	case SummaryMsg:
		m.tokens.saveAndReset(m.taskID, m.itemID, m.itemShort, m.itemTitle, m.startTime)
		s.show = true
		s.data = msg.Data
		s.elapsed = time.Since(msg.Data.StartTime).Truncate(time.Second)
		s.pushStatus = "Pushing to remote..."
		return true

	case PushStatusMsg:
		s.pushStatus = msg.Status
		s.pushDone = msg.Done
		return true

	case QuitMsg:
		m.done = true
		return true
	}
	return false
}

// handleSummaryKeys processes key events while the summary/done screen is active.
// Any of Q, Esc, Enter, or Ctrl+C exits.
func (s *summaryState) handleSummaryKeys(msg tea.KeyMsg) (quitting bool, cmd tea.Cmd) {
	switch msg.Type {
	case tea.KeyEscape, tea.KeyCtrlC, tea.KeyEnter:
		return true, tea.Quit
	default:
		if len(msg.Runes) == 1 {
			switch msg.Runes[0] {
			case 'q', 'Q':
				return true, tea.Quit
			}
		}
		return false, nil
	}
}

// renderSummaryView renders the post-run summary screen.
func (s *summaryState) renderSummaryView(m *TUIModel) string {
	innerW, _ := styles.FullScreenInnerSize(m.width, m.height)
	if innerW < 40 {
		innerW = 40
	}

	var content strings.Builder

	// Header inside box
	content.WriteString(m.renderHeaderInner(innerW))

	// Title and reason
	elapsed := s.elapsed
	var title string
	switch s.data.Reason {
	case StopReasonComplete:
		title = styles.Title.Render("✓ Work Complete")
	case StopReasonUserStop:
		title = styles.Title.Foreground(styles.Warning).Render("⊘ Stopped by User")
		content.WriteString(title + "\n")
		content.WriteString(grayStyle.Render("  You requested to stop after the completed task.") + "\n\n")
		goto afterTitle
	case StopReasonInterrupted:
		title = styles.Title.Foreground(styles.Error).Render("⊘ Work Interrupted")
		content.WriteString(title + "\n")
		content.WriteString(grayStyle.Render("  Cancelled via Ctrl+C — the in-progress task was aborted.") + "\n\n")
		goto afterTitle
	case StopReasonError:
		title = styles.Title.Foreground(styles.Error).Render("✗ Work Failed")
		content.WriteString(title + "\n")
		if s.data.ErrorDetail != "" {
			content.WriteString(redStyle.Render("  "+s.data.ErrorDetail) + "\n\n")
		}
		goto afterTitle
	case StopReasonNoTasks:
		title = styles.Title.Foreground(styles.Warning).Render("⊘ No Tasks Available")
		content.WriteString(title + "\n")
		content.WriteString(grayStyle.Render("  No workable tasks found — all tasks may be complete, blocked, or ignored.") + "\n")
		if s.data.ErrorDetail != "" {
			content.WriteString(grayStyle.Render("  "+s.data.ErrorDetail) + "\n")
		}
		content.WriteString("\n")
		goto afterTitle
	case StopReasonPartialComplete:
		title = styles.Title.Foreground(styles.Warning).Render("⚠ Work Complete (with failures)")
	default:
		title = styles.Title.Foreground(styles.Warning).Render("⊘ Work Interrupted")
	}
	content.WriteString(title + "\n\n")
afterTitle:

	// Key-value pairs
	labelStyle := styles.Label.Width(10).Align(lipgloss.Right)
	valStyle := lipgloss.NewStyle().Foreground(styles.Muted)

	content.WriteString(fmt.Sprintf("%s  %s\n", labelStyle.Render("Run ID:"), valStyle.Render(s.data.RunID)))
	content.WriteString(fmt.Sprintf("%s  %s\n", labelStyle.Render("Branch:"), valStyle.Render(s.data.Branch)))
	content.WriteString(fmt.Sprintf("%s  %s\n", labelStyle.Render("Model:"), valStyle.Render(s.data.Model)))
	content.WriteString(fmt.Sprintf("%s  %s\n", labelStyle.Render("Elapsed:"), valStyle.Render(formatHHMMSS(elapsed))))
	content.WriteString(fmt.Sprintf("%s  %s\n",
		labelStyle.Render("Tasks:"),
		lipgloss.NewStyle().Foreground(styles.Success).Render(
			fmt.Sprintf("%d/%d completed", s.data.TasksCompleted, s.data.TasksTotal))))

	// Token usage totals
	if m.tokens.hasData {
		totalIn := m.tokens.totalInput + m.tokens.totalCacheCreation + m.tokens.totalCacheRead
		var tokenStr string
		if m.tokens.totalCacheCreation > 0 || m.tokens.totalCacheRead > 0 {
			tokenStr = fmt.Sprintf("%s in / %s out (cache: %s write, %s read)",
				FormatTokens(totalIn), FormatTokens(m.tokens.totalOutput),
				FormatTokens(m.tokens.totalCacheCreation), FormatTokens(m.tokens.totalCacheRead))
		} else {
			tokenStr = fmt.Sprintf("%s in / %s out", FormatTokens(totalIn), FormatTokens(m.tokens.totalOutput))
		}
		content.WriteString(fmt.Sprintf("%s  %s\n", labelStyle.Render("Tokens:"), valStyle.Render(tokenStr)))

		costStr := "N/A"
		if m.tokens.totalCost > 0 {
			costStr = FormatCost(m.tokens.totalCost)
		}
		content.WriteString(fmt.Sprintf("%s  %s\n", labelStyle.Render("Cost:"), valStyle.Render(costStr)))
	} else {
		content.WriteString(fmt.Sprintf("%s  %s\n", labelStyle.Render("Tokens:"), valStyle.Render("N/A")))
	}

	// Per-task token breakdown
	if len(m.tokens.usages) > 0 {
		content.WriteString("\n")
		content.WriteString(styles.Subtitle.Render("Token Usage") + "\n")
		for _, tu := range m.tokens.usages {
			taskIn := tu.InputTokens + tu.CacheCreationInputTokens + tu.CacheReadInputTokens
			var taskTokenStr string
			if tu.CacheCreationInputTokens > 0 || tu.CacheReadInputTokens > 0 {
				taskTokenStr = fmt.Sprintf("%s in (cache: %s write, %s read) / %s out",
					FormatTokens(taskIn),
					FormatTokens(tu.CacheCreationInputTokens), FormatTokens(tu.CacheReadInputTokens),
					FormatTokens(tu.OutputTokens))
			} else {
				taskTokenStr = fmt.Sprintf("%s in / %s out", FormatTokens(taskIn), FormatTokens(tu.OutputTokens))
			}
			taskElapsed := tu.EndTime.Sub(tu.StartTime).Truncate(time.Second)
			content.WriteString(fmt.Sprintf("  %s %s  %s  %s\n",
				lipgloss.NewStyle().Foreground(styles.Muted).Render("•"),
				fmt.Sprintf("%-12s", tu.TaskShort),
				valStyle.Render(formatHHMMSS(taskElapsed)),
				taskTokenStr))
		}
	}

	// Per-model usage breakdown
	if len(m.tokens.totalModelUsage) > 0 {
		content.WriteString("\n")
		content.WriteString(styles.Subtitle.Render("Per-Model Usage") + "\n")
		// Sort model names for stable output
		modelNames := make([]string, 0, len(m.tokens.totalModelUsage))
		for name := range m.tokens.totalModelUsage {
			modelNames = append(modelNames, name)
		}
		sort.Strings(modelNames)
		for _, name := range modelNames {
			mt := m.tokens.totalModelUsage[name]
			modelIn := mt.InputTokens + mt.CacheCreationInputTokens + mt.CacheReadInputTokens
			var modelTokenStr string
			if mt.CacheCreationInputTokens > 0 || mt.CacheReadInputTokens > 0 {
				modelTokenStr = fmt.Sprintf("%s in (cache: %s write, %s read) / %s out",
					FormatTokens(modelIn),
					FormatTokens(mt.CacheCreationInputTokens), FormatTokens(mt.CacheReadInputTokens),
					FormatTokens(mt.OutputTokens))
			} else {
				modelTokenStr = fmt.Sprintf("%s in / %s out", FormatTokens(modelIn), FormatTokens(mt.OutputTokens))
			}
			costStr := ""
			if mt.CostUSD > 0 {
				costStr = fmt.Sprintf("  %s", FormatCost(mt.CostUSD))
			}
			content.WriteString(fmt.Sprintf("  %s %s\n",
				lipgloss.NewStyle().Foreground(styles.Muted).Render("•"),
				fmt.Sprintf("%s: %s%s", name, modelTokenStr, costStr)))
		}
	}

	// Commit range and list
	if len(m.commits) > 0 {
		content.WriteString("\n")
		commitHeader := fmt.Sprintf("Commits (%d)", len(m.commits))
		if s.data.CommitStart != "" && s.data.CommitEnd != "" {
			commitHeader += fmt.Sprintf("  %s..%s", s.data.CommitStart, s.data.CommitEnd)
		}
		content.WriteString(styles.Subtitle.Render(commitHeader) + "\n")
		for _, c := range m.commits {
			content.WriteString(fmt.Sprintf("  %s %s\n",
				lipgloss.NewStyle().Foreground(styles.Muted).Render("•"),
				styles.Truncate(c, innerW-4)))
		}
	}

	// Warnings
	if len(s.data.Warnings) > 0 {
		content.WriteString("\n")
		content.WriteString(lipgloss.NewStyle().Foreground(styles.Warning).Render("Warnings") + "\n")
		for _, w := range s.data.Warnings {
			content.WriteString(fmt.Sprintf("  %s %s\n",
				lipgloss.NewStyle().Foreground(styles.Warning).Render("⚠"),
				w))
		}
	}

	// Failed tasks
	if len(s.data.FailedTasks) > 0 {
		content.WriteString("\n")
		content.WriteString(lipgloss.NewStyle().Foreground(styles.Error).Render("Failed Tasks:") + "\n")
		for _, ft := range s.data.FailedTasks {
			content.WriteString(fmt.Sprintf("  %s %s: %s\n",
				lipgloss.NewStyle().Foreground(styles.Error).Render("✗"),
				ft.ID,
				styles.Truncate(ft.Title, innerW-len(ft.ID)-6)))
			content.WriteString(fmt.Sprintf("    %s\n",
				lipgloss.NewStyle().Foreground(styles.Muted).Render(styles.Truncate(ft.Reason, innerW-4))))
		}
	}

	// Remaining incomplete tasks
	if len(s.data.RemainingTasks) > 0 {
		content.WriteString("\n")
		content.WriteString(lipgloss.NewStyle().Foreground(styles.Warning).Render(
			fmt.Sprintf("Remaining (%d)", len(s.data.RemainingTasks))) + "\n")
		maxShow := 5
		for i, t := range s.data.RemainingTasks {
			if i >= maxShow {
				content.WriteString(fmt.Sprintf("  %s\n",
					lipgloss.NewStyle().Foreground(styles.Muted).Render(
						fmt.Sprintf("... and %d more", len(s.data.RemainingTasks)-maxShow))))
				break
			}
			content.WriteString(fmt.Sprintf("  %s %s\n",
				lipgloss.NewStyle().Foreground(styles.Muted).Render("•"),
				fmt.Sprintf("%s: %s", t.ID, styles.Truncate(t.Title, innerW-len(t.ID)-6))))
		}
	}

	// Push status
	content.WriteString("\n")
	if s.pushDone {
		content.WriteString(lipgloss.NewStyle().Foreground(styles.Success).Render(s.pushStatus) + "\n")
	} else if s.pushStatus != "" {
		spinner := cyanStyle.Render(spinnerFrames[m.frame])
		content.WriteString(fmt.Sprintf("%s %s\n", spinner, s.pushStatus))
	}

	// Footer: summary menu or waiting message
	var footer string
	if m.done {
		footer = s.renderSummaryMenu()
	} else {
		footer = lipgloss.NewStyle().Foreground(styles.Muted).Render("Waiting for push to complete...")
	}

	if m.width > 0 && m.height > 0 {
		return styles.FullScreenLeft(content.String(), footer, m.width, m.height)
	}
	return styles.Box.Render(content.String()) + "\n"
}

// renderSummaryMenu renders the exit hint at the bottom of the summary screen.
func (s *summaryState) renderSummaryMenu() string {
	hintStyle := lipgloss.NewStyle().Foreground(styles.Muted).Faint(true)
	return fmt.Sprintf("  %s\n", hintStyle.Render("Press q/esc/enter to exit"))
}
