package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/leberkas-org/maggus/internal/parser"
	"github.com/leberkas-org/maggus/internal/tui/styles"

	"github.com/spf13/cobra"
)

const progressBarWidth = 10

// Lipgloss styles for the status command.
var (
	statusGreenStyle  = lipgloss.NewStyle().Foreground(styles.Success)
	statusCyanStyle   = lipgloss.NewStyle().Foreground(styles.Primary)
	statusYellowStyle = lipgloss.NewStyle().Foreground(styles.Warning)
	statusRedStyle    = lipgloss.NewStyle().Foreground(styles.Error)
	statusBlueStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("4"))
	statusDimStyle    = lipgloss.NewStyle().Faint(true)
	statusDimGreen    = lipgloss.NewStyle().Faint(true).Foreground(styles.Success)
)

type planInfo struct {
	filename  string
	tasks     []parser.Task
	completed bool // filename contains _completed
}

func (p *planInfo) doneCount() int {
	n := 0
	for _, t := range p.tasks {
		if t.IsComplete() {
			n++
		}
	}
	return n
}

func (p *planInfo) blockedCount() int {
	n := 0
	for _, t := range p.tasks {
		if !t.IsComplete() && t.IsBlocked() {
			n++
		}
	}
	return n
}

func buildProgressBar(done, total int) string {
	return styles.ProgressBar(done, total, progressBarWidth)
}

func buildProgressBarPlain(done, total int) string {
	return styles.ProgressBarPlain(done, total, progressBarWidth)
}

// statusModel is the bubbletea model for the status TUI with scrolling.
type statusModel struct {
	viewport viewport.Model
	content  string // pre-rendered status content
	ready    bool
}

func (m statusModel) Init() tea.Cmd {
	return nil
}

func (m statusModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// Reserve 1 line for the footer
		m.viewport = viewport.New(msg.Width, msg.Height-1)
		m.viewport.SetContent(m.content)
		m.ready = true
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		case "home":
			if m.ready {
				m.viewport.GotoTop()
				return m, nil
			}
		case "end":
			if m.ready {
				m.viewport.GotoBottom()
				return m, nil
			}
		}
	}

	if m.ready {
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m statusModel) View() string {
	if !m.ready {
		return ""
	}
	footer := statusFooter(m.viewport)
	return m.viewport.View() + "\n" + footer
}

func statusFooter(vp viewport.Model) string {
	if vp.TotalLineCount() <= vp.Height {
		return styles.StatusBar.Render("q/esc: exit")
	}
	pct := vp.ScrollPercent() * 100
	return styles.StatusBar.Render(fmt.Sprintf("↑/↓: scroll · q/esc: exit · %.0f%%", pct))
}

// renderStatusContent builds the styled status output for the TUI.
func renderStatusContent(plans []planInfo, showAll bool, nextTaskID, nextTaskFile string) string {
	var sb strings.Builder

	// Compute totals
	totalTasks := 0
	totalDone := 0
	totalBlocked := 0
	activePlans := 0
	for _, p := range plans {
		totalTasks += len(p.tasks)
		totalDone += p.doneCount()
		totalBlocked += p.blockedCount()
		if !p.completed {
			activePlans++
		}
	}
	totalPending := totalTasks - totalDone - totalBlocked

	// Header
	header := styles.Title.Render(fmt.Sprintf("Maggus Status — %d plans (%d active), %d tasks total",
		len(plans), activePlans, totalTasks))
	sb.WriteString(header)
	sb.WriteString("\n\n")

	// Summary
	summary := fmt.Sprintf(" Summary: %d/%d tasks complete · %d pending · %d blocked",
		totalDone, totalTasks, totalPending, totalBlocked)
	sb.WriteString(summary)

	// Task sections
	for _, p := range plans {
		if p.completed && !showAll {
			continue
		}
		sb.WriteString("\n\n")
		if p.completed {
			sb.WriteString(statusDimGreen.Render(fmt.Sprintf(" Tasks — %s (archived)", p.filename)))
		} else {
			fmt.Fprintf(&sb, " Tasks — %s", p.filename)
		}
		sb.WriteString("\n")
		sb.WriteString(" " + styles.Separator(42))

		for _, t := range p.tasks {
			var icon, prefix string
			var style lipgloss.Style

			if t.IsComplete() {
				icon = "✓"
				if p.completed {
					style = statusDimGreen
				} else {
					style = statusGreenStyle
				}
				prefix = "  "
			} else if t.IsBlocked() {
				icon = "⚠"
				style = statusRedStyle
				prefix = "  "
			} else if t.ID == nextTaskID && t.SourceFile == nextTaskFile {
				icon = "→"
				style = statusCyanStyle
				prefix = "→ "
			} else {
				icon = "○"
				style = lipgloss.NewStyle().Foreground(styles.Muted)
				prefix = "  "
			}

			if p.completed {
				style = statusDimStyle
			}

			line := fmt.Sprintf(" %s%s  %s: %s", prefix, icon, t.ID, t.Title)
			sb.WriteString("\n")
			sb.WriteString(style.Render(line))

			if t.IsBlocked() && !p.completed {
				for _, c := range t.Criteria {
					if !c.Blocked {
						continue
					}
					reason := strings.TrimPrefix(c.Text, "⚠️ BLOCKED: ")
					reason = strings.TrimPrefix(reason, "BLOCKED: ")
					blockedLine := fmt.Sprintf("         BLOCKED: %s", reason)
					sb.WriteString("\n")
					sb.WriteString(statusRedStyle.Render(blockedLine))
				}
			}
		}
	}

	// Plans table
	sb.WriteString("\n\n")
	sb.WriteString(" Plans")
	sb.WriteString("\n")
	sb.WriteString(" " + styles.Separator(42))

	maxCountWidth := 0
	for _, p := range plans {
		if p.completed && !showAll {
			continue
		}
		w := len(fmt.Sprintf("%d/%d", p.doneCount(), len(p.tasks)))
		if w > maxCountWidth {
			maxCountWidth = w
		}
	}
	countFmt := fmt.Sprintf("%%-%ds", maxCountWidth)

	for _, p := range plans {
		if p.completed && !showAll {
			continue
		}

		done := p.doneCount()
		total := len(p.tasks)
		bar := buildProgressBar(done, total)

		var prefix, suffix string
		var style lipgloss.Style

		if p.completed {
			prefix = " ✓ "
			style = statusDimGreen
			suffix = "done"
		} else if p.blockedCount() > 0 {
			prefix = "   "
			style = statusRedStyle
			suffix = "blocked"
		} else if total > 0 && done == total {
			prefix = "   "
			style = statusGreenStyle
			suffix = "done"
		} else if done == 0 {
			prefix = "   "
			style = statusBlueStyle
			suffix = "new"
		} else {
			prefix = "   "
			style = statusYellowStyle
			suffix = "in progress"
		}

		countStr := fmt.Sprintf(countFmt, fmt.Sprintf("%d/%d", done, total))
		line := fmt.Sprintf("%s%-32s [%s]  %s   %s", prefix, p.filename, bar, countStr, suffix)
		sb.WriteString("\n")
		sb.WriteString(style.Render(line))
	}

	return styles.Box.Render(sb.String())
}

// renderStatusPlain builds the plain-text status output (no ANSI, no TUI).
func renderStatusPlain(w *strings.Builder, plans []planInfo, showAll bool, nextTaskID, nextTaskFile string) {
	totalTasks := 0
	totalDone := 0
	totalBlocked := 0
	activePlans := 0
	for _, p := range plans {
		totalTasks += len(p.tasks)
		totalDone += p.doneCount()
		totalBlocked += p.blockedCount()
		if !p.completed {
			activePlans++
		}
	}
	totalPending := totalTasks - totalDone - totalBlocked

	fmt.Fprintf(w, "Maggus Status — %d plans (%d active), %d tasks total\n\n", len(plans), activePlans, totalTasks)
	fmt.Fprintf(w, " Summary: %d/%d tasks complete · %d pending · %d blocked\n", totalDone, totalTasks, totalPending, totalBlocked)

	// Find next task
	for _, p := range plans {
		if p.completed && !showAll {
			continue
		}
		fmt.Fprintln(w)
		if p.completed {
			fmt.Fprintf(w, " Tasks — %s (archived)\n", p.filename)
		} else {
			fmt.Fprintf(w, " Tasks — %s\n", p.filename)
		}
		fmt.Fprintln(w, " ──────────────────────────────────────────")

		for _, t := range p.tasks {
			var icon, prefix string

			if t.IsComplete() {
				icon = "[x]"
				prefix = "  "
			} else if t.IsBlocked() {
				icon = "[!]"
				prefix = "  "
			} else if t.ID == nextTaskID && t.SourceFile == nextTaskFile {
				icon = "o"
				prefix = "-> "
			} else {
				icon = "o"
				prefix = "  "
			}

			fmt.Fprintf(w, " %s%s  %s: %s\n", prefix, icon, t.ID, t.Title)

			if t.IsBlocked() && !p.completed {
				for _, c := range t.Criteria {
					if !c.Blocked {
						continue
					}
					reason := strings.TrimPrefix(c.Text, "⚠️ BLOCKED: ")
					reason = strings.TrimPrefix(reason, "BLOCKED: ")
					fmt.Fprintf(w, "         BLOCKED: %s\n", reason)
				}
			}
		}
	}

	// Plans table
	fmt.Fprintln(w)
	fmt.Fprintln(w, " Plans")
	fmt.Fprintln(w, " ──────────────────────────────────────────")

	maxCountWidth := 0
	for _, p := range plans {
		if p.completed && !showAll {
			continue
		}
		cw := len(fmt.Sprintf("%d/%d", p.doneCount(), len(p.tasks)))
		if cw > maxCountWidth {
			maxCountWidth = cw
		}
	}
	countFmt := fmt.Sprintf("%%-%ds", maxCountWidth)

	for _, p := range plans {
		if p.completed && !showAll {
			continue
		}

		done := p.doneCount()
		total := len(p.tasks)
		bar := buildProgressBarPlain(done, total)

		var prefix, suffix string

		if p.completed {
			prefix = " [x] "
			suffix = "done"
		} else if p.blockedCount() > 0 {
			prefix = "   "
			suffix = "blocked"
		} else if total > 0 && done == total {
			prefix = "   "
			suffix = "done"
		} else if done == 0 {
			prefix = "   "
			suffix = "new"
		} else {
			prefix = "   "
			suffix = "in progress"
		}

		countStr := fmt.Sprintf(countFmt, fmt.Sprintf("%d/%d", done, total))
		fmt.Fprintf(w, "%s%-32s [%s]  %s   %s\n", prefix, p.filename, bar, countStr, suffix)
	}
}

func parsePlans(dir string) ([]planInfo, error) {
	files, err := parser.GlobPlanFiles(dir, true)
	if err != nil {
		return nil, fmt.Errorf("glob plans: %w", err)
	}

	var plans []planInfo
	for _, f := range files {
		tasks, err := parser.ParseFile(f)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", f, err)
		}
		plans = append(plans, planInfo{
			filename:  filepath.Base(f),
			tasks:     tasks,
			completed: strings.HasSuffix(f, "_completed.md"),
		})
	}
	return plans, nil
}

func findNextTask(plans []planInfo) (string, string) {
	for _, p := range plans {
		if p.completed {
			continue
		}
		next := parser.FindNextIncomplete(p.tasks)
		if next != nil {
			return next.ID, next.SourceFile
		}
	}
	return "", ""
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show a compact summary of plan progress",
	Long:  `Reads all plan files in .maggus/ and displays a compact progress summary.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		plain, err := cmd.Flags().GetBool("plain")
		if err != nil {
			return err
		}
		all, err := cmd.Flags().GetBool("all")
		if err != nil {
			return err
		}

		dir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}

		plans, err := parsePlans(dir)
		if err != nil {
			return err
		}

		if len(plans) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No plans found.")
			return nil
		}

		nextTaskID, nextTaskFile := findNextTask(plans)

		if plain {
			var sb strings.Builder
			renderStatusPlain(&sb, plans, all, nextTaskID, nextTaskFile)
			fmt.Fprint(cmd.OutOrStdout(), sb.String())
			return nil
		}

		// TUI mode: render content and display in alt-screen
		content := renderStatusContent(plans, all, nextTaskID, nextTaskFile)
		m := statusModel{content: content}
		p := tea.NewProgram(m, tea.WithAltScreen())
		_, err = p.Run()
		return err
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
	statusCmd.Flags().Bool("plain", false, "Strip colors and use ASCII characters for scripting/piping")
	statusCmd.Flags().Bool("all", false, "Show completed plans in task sections and Plans table")
}
