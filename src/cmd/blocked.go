package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/dirnei/maggus/internal/parser"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

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
	for _, task := range blocked {
		renderBlockedTaskDetail(out, task, termWidth)
	}

	return nil
}

func init() {
	rootCmd.AddCommand(blockedCmd)
}
