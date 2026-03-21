package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/leberkas-org/maggus/internal/config"
	"github.com/leberkas-org/maggus/internal/parser"
	"github.com/leberkas-org/maggus/internal/tui/styles"
	"github.com/spf13/cobra"
)

const listHeaderLines = 4 // title + agent + separator + blank

// listModel is the bubbletea model for the interactive task list.
type listModel struct {
	taskListComponent
}

func newListModel(tasks []parser.Task, agentName string) listModel {
	return listModel{
		taskListComponent: taskListComponent{
			Tasks:       tasks,
			AgentName:   agentName,
			HeaderLines: listHeaderLines,
		},
	}
}

func (m listModel) Init() tea.Cmd {
	return nil
}

func (m listModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.HandleResize(msg.Width, msg.Height)
		return m, nil
	case tea.KeyMsg:
		cmd, action := m.taskListComponent.Update(msg)
		switch action {
		case taskListQuit, taskListRun:
			return m, tea.Quit
		}
		return m, cmd
	}
	cmd := m.UpdateViewport(msg)
	return m, cmd
}

func (m listModel) View() string {
	if len(m.Tasks) == 0 {
		return m.viewEmpty()
	}
	if v := m.taskListComponent.View(); v != "" {
		return v
	}
	return m.viewList()
}

func (m listModel) viewEmpty() string {
	body := styles.Title.Render("Task List") + "\n\n" +
		lipgloss.NewStyle().Foreground(styles.Success).Render("All done!") + "\n\n" +
		lipgloss.NewStyle().Foreground(styles.Muted).Render("No pending tasks found. All tasks are complete or no plans exist.") + "\n"
	footer := styles.StatusBar.Render("q/esc: exit")
	if m.Width > 0 && m.Height > 0 {
		return styles.FullScreen(body, footer, m.Width, m.Height)
	}
	return styles.Box.Render(body) + "\n"
}

func (m listModel) viewList() string {
	accentStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.Primary)
	blockedStyle := lipgloss.NewStyle().Foreground(styles.Warning)
	ignoredStyle := lipgloss.NewStyle().Foreground(styles.Warning).Faint(true)
	mutedStyle := lipgloss.NewStyle().Faint(true)

	var sb strings.Builder
	sb.WriteString(styles.Title.Render(fmt.Sprintf("All incomplete tasks (%d)", len(m.Tasks))) + "\n")
	sb.WriteString(mutedStyle.Render(fmt.Sprintf(" Agent: %s", m.AgentName)) + "\n")
	sb.WriteString(" " + styles.Separator(42) + "\n")

	visible := m.visibleTaskLines()
	end := m.ScrollOffset + visible
	if end > len(m.Tasks) {
		end = len(m.Tasks)
	}

	for i := m.ScrollOffset; i < end; i++ {
		t := m.Tasks[i]
		planFile := mutedStyle.Render(filepath.Base(t.SourceFile))
		label := fmt.Sprintf("#%-2d %s: %s", i+1, t.ID, t.Title)

		prefix, icon := "  ", " "
		labelStyle := lipgloss.NewStyle()
		if i == m.Cursor {
			prefix = " " + accentStyle.Render("→")
			labelStyle = accentStyle
		}
		if t.Ignored {
			icon = ignoredStyle.Render("~")
			if i != m.Cursor {
				labelStyle = ignoredStyle
			}
		} else if t.IsBlocked() {
			icon = blockedStyle.Render("⊘")
			if i != m.Cursor {
				labelStyle = blockedStyle
			}
		}
		fmt.Fprintf(&sb, " %s %s %s  %s\n", prefix, icon, labelStyle.Render(label), planFile)
	}

	scrollHint := ""
	if len(m.Tasks) > visible {
		scrollHint = fmt.Sprintf(" [%d-%d of %d]", m.ScrollOffset+1, end, len(m.Tasks))
	}
	footer := styles.StatusBar.Render("↑/↓: navigate · enter: details · alt+r: run · alt+bksp: delete · q/esc: exit" + scrollHint)

	if m.Width > 0 && m.Height > 0 {
		return styles.FullScreen(sb.String(), footer, m.Width, m.Height)
	}
	return styles.Box.Render(sb.String()+"\n\n"+footer) + "\n"
}

func renderListPlain(workable, ignored []parser.Task, all bool, agentName string) string {
	var sb strings.Builder
	if all {
		fmt.Fprintln(&sb, "All upcoming tasks:")
	} else {
		fmt.Fprintf(&sb, "Next %d task(s):\n", len(workable))
	}
	fmt.Fprintf(&sb, "Agent: %s\n", agentName)

	for i, t := range workable {
		prefix := "   "
		if i == 0 {
			prefix = " ->"
		}
		fmt.Fprintf(&sb, "%s #%-2d %s: %s  %s\n", prefix, i+1, t.ID, t.Title, filepath.Base(t.SourceFile))
	}
	for i, t := range ignored {
		fmt.Fprintf(&sb, " [~]#%-2d %s: %s  %s\n", len(workable)+i+1, t.ID, t.Title, filepath.Base(t.SourceFile))
	}
	return sb.String()
}

var listCmd = &cobra.Command{
	Use:   "list [N]",
	Short: "Preview the next N upcoming workable tasks",
	Long:  `Reads all plan files in .maggus/ and lists the next N workable (incomplete, not blocked) tasks.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		plain, _ := cmd.Flags().GetBool("plain")
		all, _ := cmd.Flags().GetBool("all")
		count, _ := cmd.Flags().GetInt("count")
		if !all && len(args) == 1 {
			n, err := strconv.Atoi(args[0])
			if err != nil || n < 1 {
				return fmt.Errorf("invalid count %q: must be a positive integer", args[0])
			}
			count = n
		}
		dir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}
		return runList(cmd, dir, plain, all, count)
	},
}

func runList(cmd *cobra.Command, dir string, plain, all bool, count int) error {
	cfg, err := config.Load(dir)
	if err != nil {
		return err
	}
	agentName := cfg.Agent

	files, err := parser.GlobPlanFiles(dir, false)
	if err != nil {
		return fmt.Errorf("glob plans: %w", err)
	}

	// Collect tasks from all active plan files
	var workable []parser.Task
	var ignored []parser.Task
	var incomplete []parser.Task
	for _, f := range files {
		tasks, err := parser.ParseFile(f)
		if err != nil {
			return fmt.Errorf("parse %s: %w", f, err)
		}
		fileIgnored := parser.IsIgnoredFile(f)
		for i := range tasks {
			if fileIgnored {
				tasks[i].Ignored = true
			}
		}
		for _, t := range tasks {
			if t.Ignored {
				ignored = append(ignored, t)
			} else if t.IsWorkable() {
				workable = append(workable, t)
			}
			if !t.IsComplete() {
				incomplete = append(incomplete, t)
			}
		}
	}

	if plain {
		// Plain mode: workable + ignored, respects --count and --all for workable
		if len(workable) == 0 && len(ignored) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No pending tasks found. All done!")
			return nil
		}
		if !all && count < len(workable) {
			workable = workable[:count]
		}
		fmt.Fprint(cmd.OutOrStdout(), renderListPlain(workable, ignored, all, agentName))
		return nil
	}

	// TUI mode: all incomplete tasks (workable + blocked), no count cap
	m := newListModel(incomplete, agentName)
	prog := tea.NewProgram(m, tea.WithAltScreen())
	result, err := prog.Run()
	if err != nil {
		return err
	}
	if final, ok := result.(listModel); ok && final.RunTaskID != "" {
		return dispatchWork(final.RunTaskID)
	}
	return nil
}

func init() {
	rootCmd.AddCommand(listCmd)
	listCmd.Flags().IntP("count", "c", 5, "Number of tasks to show")
	listCmd.Flags().Bool("plain", false, "Strip colors and use ASCII characters for scripting/piping")
	listCmd.Flags().Bool("all", false, "Show all upcoming workable tasks with no count cap")
}
