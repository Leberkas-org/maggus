package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dirnei/maggus/internal/parser"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// blockedAction represents the user's choice for a blocked criterion.
type blockedAction int

const (
	actionUnblock blockedAction = iota
	actionResolve
	actionSkip
	actionAbort
)

func (a blockedAction) String() string {
	switch a {
	case actionUnblock:
		return "Unblock"
	case actionResolve:
		return "Resolve"
	case actionSkip:
		return "Skip"
	case actionAbort:
		return "Abort"
	}
	return ""
}

// actionPickerModel is a bubbletea model for selecting an action on a blocked criterion.
type actionPickerModel struct {
	criterion parser.Criterion
	actions   []blockedAction
	cursor    int
	chosen    blockedAction
	done      bool
}

func newActionPickerModel(criterion parser.Criterion) actionPickerModel {
	return actionPickerModel{
		criterion: criterion,
		actions:   []blockedAction{actionUnblock, actionResolve, actionSkip, actionAbort},
	}
}

func (m actionPickerModel) Init() tea.Cmd {
	return nil
}

func (m actionPickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.actions)-1 {
				m.cursor++
			}
		case "enter":
			m.chosen = m.actions[m.cursor]
			m.done = true
			return m, tea.Quit
		case "ctrl+c", "q":
			m.chosen = actionAbort
			m.done = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m actionPickerModel) View() string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("\n   %s⚠ %s%s\n\n", colorRed, m.criterion.Text, colorReset))
	b.WriteString("   Choose action:\n\n")
	for i, a := range m.actions {
		cursor := "  "
		if i == m.cursor {
			cursor = "> "
		}
		style := ""
		reset := ""
		switch a {
		case actionUnblock:
			style = colorGreen
			reset = colorReset
		case actionResolve:
			style = colorYellow
			reset = colorReset
		case actionAbort:
			style = colorRed
			reset = colorReset
		}
		b.WriteString(fmt.Sprintf("   %s%s%s%s\n", cursor, style, a.String(), reset))
	}
	b.WriteString(fmt.Sprintf("\n   %s↑/↓ navigate • enter select • q abort%s\n", colorDim, colorReset))
	return b.String()
}

// runActionPicker runs the interactive bubbletea picker for a blocked criterion.
// Returns the chosen action. The pickerFunc is injectable for testing.
var runActionPicker = func(criterion parser.Criterion) (blockedAction, error) {
	m := newActionPickerModel(criterion)
	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return actionAbort, err
	}
	result := finalModel.(actionPickerModel)
	return result.chosen, nil
}

var blockedCmd = &cobra.Command{
	Use:   "blocked",
	Short: "Interactive wizard to manage blocked tasks",
	Long: `Walks through each blocked task in your plan files and lets you
unblock, resolve, or skip each blocked criterion interactively.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}
		return runBlocked(cmd, dir)
	},
}

// collectBlockedTasks parses all active plans and returns only blocked tasks,
// ordered by plan file name then document order within each plan.
func collectBlockedTasks(dir string) ([]parser.Task, error) {
	tasks, err := parser.ParsePlans(dir)
	if err != nil {
		return nil, err
	}

	var blocked []parser.Task
	for _, t := range tasks {
		if t.IsBlocked() {
			blocked = append(blocked, t)
		}
	}
	return blocked, nil
}

// getTerminalWidth returns the terminal width, defaulting to 80 if unavailable.
func getTerminalWidth() int {
	w, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || w <= 0 {
		return 80
	}
	return w
}

// wrapText wraps a string to fit within maxWidth, preserving an indent prefix on continuation lines.
func wrapText(text string, maxWidth int, indent string) string {
	if maxWidth <= len(indent)+10 {
		return indent + text
	}
	available := maxWidth - len(indent)
	if len(text) <= available {
		return indent + text
	}

	var lines []string
	remaining := text
	for len(remaining) > 0 {
		if len(remaining) <= available {
			lines = append(lines, indent+remaining)
			break
		}
		// Find last space within available width
		cut := available
		if idx := strings.LastIndex(remaining[:cut], " "); idx > 0 {
			cut = idx
		}
		lines = append(lines, indent+remaining[:cut])
		remaining = strings.TrimLeft(remaining[cut:], " ")
	}
	return strings.Join(lines, "\n")
}

// renderBlockedTaskDetail writes a formatted detail view of a blocked task.
func renderBlockedTaskDetail(w io.Writer, task parser.Task, termWidth int) {
	planFile := filepath.Base(task.SourceFile)

	fmt.Fprintf(w, "\n%s──────────────────────────────────────────%s\n", colorDim, colorReset)
	fmt.Fprintf(w, " Plan: %s\n", planFile)
	fmt.Fprintf(w, " %s%s: %s%s\n", colorCyan, task.ID, task.Title, colorReset)

	if task.Description != "" {
		fmt.Fprintln(w)
		for _, line := range strings.Split(task.Description, "\n") {
			wrapped := wrapText(line, termWidth, "   ")
			fmt.Fprintln(w, wrapped)
		}
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, " Acceptance Criteria:")

	for _, c := range task.Criteria {
		if c.Checked {
			// Completed: green with checkmark
			line := fmt.Sprintf("   %s✓ %s%s", colorGreen, c.Text, colorReset)
			fmt.Fprintln(w, line)
		} else if c.Blocked {
			// Blocked: red with warning marker
			line := fmt.Sprintf("   %s>>> ⚠ %s%s", colorRed, c.Text, colorReset)
			fmt.Fprintln(w, line)
		} else {
			// Normal unchecked: default color
			line := fmt.Sprintf("   ○ %s", c.Text)
			fmt.Fprintln(w, line)
		}
	}
}

// unblockCriterion reads the plan file, removes the "BLOCKED: " prefix from the
// matching criterion line, and writes the file back. Returns an error if the
// exact line cannot be found.
func unblockCriterion(filePath string, c parser.Criterion) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read plan file: %w", err)
	}

	oldLine := "- [ ] " + c.Text
	// Remove "BLOCKED: " or "⚠️ BLOCKED: " prefix from criterion text
	newText := c.Text
	if strings.HasPrefix(newText, "⚠️ BLOCKED: ") {
		newText = strings.TrimPrefix(newText, "⚠️ BLOCKED: ")
	} else if strings.HasPrefix(newText, "BLOCKED: ") {
		newText = strings.TrimPrefix(newText, "BLOCKED: ")
	}
	newLine := "- [ ] " + newText

	content := string(data)
	if !strings.Contains(content, oldLine) {
		return fmt.Errorf("criterion line not found in %s: %s", filepath.Base(filePath), c.Text)
	}

	content = strings.Replace(content, oldLine, newLine, 1)
	return os.WriteFile(filePath, []byte(content), 0o644)
}

// resolveCriterion reads the plan file, removes the entire criterion line,
// and writes the file back. Returns an error if the exact line cannot be found.
func resolveCriterion(filePath string, c parser.Criterion) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read plan file: %w", err)
	}

	targetLine := "- [ ] " + c.Text
	lines := strings.Split(string(data), "\n")
	found := false
	var result []string
	for _, line := range lines {
		if !found && strings.TrimSpace(line) == targetLine {
			found = true
			continue // skip this line
		}
		result = append(result, line)
	}

	if !found {
		return fmt.Errorf("criterion line not found in %s: %s", filepath.Base(filePath), c.Text)
	}

	return os.WriteFile(filePath, []byte(strings.Join(result, "\n")), 0o644)
}

func runBlocked(cmd *cobra.Command, dir string) error {
	out := cmd.OutOrStdout()

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		fmt.Fprintln(out, "No blocked tasks found.")
		return nil
	}

	blocked, err := collectBlockedTasks(dir)
	if err != nil {
		return err
	}

	if len(blocked) == 0 {
		fmt.Fprintln(out, "No blocked tasks found.")
		return nil
	}

	fmt.Fprintf(out, "Found %d blocked task(s).\n", len(blocked))

	termWidth := getTerminalWidth()
	aborted := false
	for _, task := range blocked {
		renderBlockedTaskDetail(out, task, termWidth)

		for _, c := range task.Criteria {
			if !c.Blocked {
				continue
			}

			action, err := runActionPicker(c)
			if err != nil {
				return fmt.Errorf("action picker: %w", err)
			}

			switch action {
			case actionUnblock:
				if err := unblockCriterion(task.SourceFile, c); err != nil {
					fmt.Fprintf(out, "   %s→ Error: %v%s\n", colorRed, err, colorReset)
				} else {
					fmt.Fprintf(out, "   %s→ Unblocked: %s%s\n", colorGreen, c.Text, colorReset)
				}
			case actionResolve:
				if err := resolveCriterion(task.SourceFile, c); err != nil {
					fmt.Fprintf(out, "   %s→ Error: %v%s\n", colorRed, err, colorReset)
				} else {
					fmt.Fprintf(out, "   %s→ Resolved: %s%s\n", colorYellow, c.Text, colorReset)
				}
			case actionSkip:
				fmt.Fprintf(out, "   → Skipped: %s\n", c.Text)
			case actionAbort:
				fmt.Fprintln(out, "\n   Aborted.")
				aborted = true
			}

			if aborted {
				break
			}
		}

		if aborted {
			break
		}
	}

	return nil
}

func init() {
	rootCmd.AddCommand(blockedCmd)
}
