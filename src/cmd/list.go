package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/leberkas-org/maggus/internal/parser"
	"github.com/leberkas-org/maggus/internal/tui/styles"
	"github.com/spf13/cobra"
)

// listModel is the bubbletea model for the list TUI.
type listModel struct {
	content string // pre-rendered list content
}

func (m listModel) Init() tea.Cmd {
	return nil
}

func (m listModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case tea.KeyMsg:
		return m, tea.Quit
	}
	return m, nil
}

func (m listModel) View() string {
	footer := styles.StatusBar.Render("Press any key to exit")
	return m.content + "\n" + footer + "\n"
}

// renderListContent builds the styled list output for the TUI.
func renderListContent(workable []parser.Task, all bool) string {
	var sb strings.Builder

	// Header
	var header string
	if all {
		header = styles.Title.Render(fmt.Sprintf("All upcoming tasks (%d)", len(workable)))
	} else {
		header = styles.Title.Render(fmt.Sprintf("Next %d task(s)", len(workable)))
	}
	sb.WriteString(header)
	sb.WriteString("\n")
	sb.WriteString(" " + styles.Separator(42))

	primaryStyle := lipgloss.NewStyle().Foreground(styles.Primary)
	mutedStyle := lipgloss.NewStyle().Faint(true)

	for i, t := range workable {
		sb.WriteString("\n")
		planFile := mutedStyle.Render(filepath.Base(t.SourceFile))

		if i == 0 {
			line := fmt.Sprintf(" → #%-2d %s: %s", i+1, t.ID, t.Title)
			sb.WriteString(primaryStyle.Render(line))
		} else {
			line := fmt.Sprintf("   #%-2d %s: %s", i+1, t.ID, t.Title)
			sb.WriteString(line)
		}
		sb.WriteString("  " + planFile)
	}

	return styles.Box.Render(sb.String())
}

// renderListPlain builds the plain-text list output (no ANSI, no TUI).
func renderListPlain(workable []parser.Task, all bool) string {
	var sb strings.Builder

	if all {
		fmt.Fprintln(&sb, "All upcoming tasks:")
	} else {
		fmt.Fprintf(&sb, "Next %d task(s):\n", len(workable))
	}
	fmt.Fprintln(&sb)

	for i, t := range workable {
		planFile := filepath.Base(t.SourceFile)
		if i == 0 {
			fmt.Fprintf(&sb, " -> #%-2d %s: %s  %s\n", i+1, t.ID, t.Title, planFile)
		} else {
			fmt.Fprintf(&sb, "    #%-2d %s: %s  %s\n", i+1, t.ID, t.Title, planFile)
		}
	}

	return sb.String()
}

var listCmd = &cobra.Command{
	Use:   "list [N]",
	Short: "Preview the next N upcoming workable tasks",
	Long:  `Reads all plan files in .maggus/ and lists the next N workable (incomplete, not blocked) tasks.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		plain, err := cmd.Flags().GetBool("plain")
		if err != nil {
			return err
		}

		all, err := cmd.Flags().GetBool("all")
		if err != nil {
			return err
		}

		count, err := cmd.Flags().GetInt("count")
		if err != nil {
			return err
		}

		// Positional arg overrides --count (ignored when --all is set)
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
	files, err := parser.GlobPlanFiles(dir, false)
	if err != nil {
		return fmt.Errorf("glob plans: %w", err)
	}

	// Collect workable tasks in order
	var workable []parser.Task
	for _, f := range files {
		tasks, err := parser.ParseFile(f)
		if err != nil {
			return fmt.Errorf("parse %s: %w", f, err)
		}
		for _, t := range tasks {
			if t.IsWorkable() {
				workable = append(workable, t)
			}
		}
	}

	if len(workable) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No pending tasks found. All done!")
		return nil
	}

	// Cap to count unless --all is set
	if !all && count < len(workable) {
		workable = workable[:count]
	}

	if plain {
		fmt.Fprint(cmd.OutOrStdout(), renderListPlain(workable, all))
		return nil
	}

	// TUI mode: render content and display in alt-screen
	content := renderListContent(workable, all)
	m := listModel{content: content}
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err = p.Run()
	return err
}

func init() {
	rootCmd.AddCommand(listCmd)
	listCmd.Flags().IntP("count", "c", 5, "Number of tasks to show")
	listCmd.Flags().Bool("plain", false, "Strip colors and use ASCII characters for scripting/piping")
	listCmd.Flags().Bool("all", false, "Show all upcoming workable tasks with no count cap")
}
