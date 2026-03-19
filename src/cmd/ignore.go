package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/leberkas-org/maggus/internal/parser"
	"github.com/spf13/cobra"
)

var ignoreCmd = &cobra.Command{
	Use:          "ignore",
	Short:        "Exclude plans or tasks from the work loop",
	Long:         `Ignore a plan or task so that it is skipped by maggus work.`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		_ = cmd.Help()
		return fmt.Errorf("a subcommand is required")
	},
}

var ignorePlanCmd = &cobra.Command{
	Use:   "plan <plan-id>",
	Short: "Ignore an entire plan file",
	Long:  `Renames plan_<N>.md to plan_<N>_ignored.md so that it is skipped by the work loop.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}
		return runIgnorePlan(cmd, dir, args[0])
	},
}

func runIgnorePlan(cmd *cobra.Command, dir string, planID string) error {
	file, state, err := findPlanFile(dir, planID)
	if err != nil {
		return err
	}

	switch state {
	case planStateNotFound:
		cmd.PrintErrln(fmt.Sprintf("Error: plan %s not found", planID))
		return fmt.Errorf("plan %s not found", planID)
	case planStateCompleted:
		cmd.PrintErrln(fmt.Sprintf("Error: plan %s is already completed", planID))
		return fmt.Errorf("plan %s is already completed", planID)
	case planStateIgnored:
		cmd.Println(fmt.Sprintf("Plan %s is already ignored", planID))
		return nil
	case planStateActive:
		newName := strings.TrimSuffix(file, ".md") + "_ignored.md"
		if err := os.Rename(file, newName); err != nil {
			return fmt.Errorf("rename %s: %w", file, err)
		}
		cmd.Println(fmt.Sprintf("Ignored plan %s (%s → %s)", planID, filepath.Base(file), filepath.Base(newName)))
		return nil
	}

	return nil
}

type planState int

const (
	planStateNotFound planState = iota
	planStateActive
	planStateIgnored
	planStateCompleted
)

// findPlanFile locates a plan file by its numeric ID across all states (active, ignored, completed).
// Returns the file path, its state, and any error from globbing.
func findPlanFile(dir string, planID string) (string, planState, error) {
	files, err := parser.GlobPlanFiles(dir, true)
	if err != nil {
		return "", planStateNotFound, err
	}

	target := fmt.Sprintf("plan_%s", planID)

	for _, f := range files {
		base := filepath.Base(f)
		if !strings.HasPrefix(base, target) {
			continue
		}

		// Verify exact number match (plan_3 should not match plan_30)
		rest := strings.TrimPrefix(base, target)
		if rest != ".md" && rest != "_ignored.md" && rest != "_completed.md" {
			continue
		}

		if strings.HasSuffix(base, "_completed.md") {
			return f, planStateCompleted, nil
		}
		if strings.HasSuffix(base, "_ignored.md") {
			return f, planStateIgnored, nil
		}
		return f, planStateActive, nil
	}

	return "", planStateNotFound, nil
}

var ignoreTaskCmd = &cobra.Command{
	Use:   "task <TASK-NNN>",
	Short: "Ignore a single task",
	Long:  `Rewrites the task heading from "### TASK-NNN:" to "### IGNORED TASK-NNN:" so that it is skipped by the work loop.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}
		return runIgnoreTask(cmd, dir, args[0])
	},
}

func runIgnoreTask(cmd *cobra.Command, dir string, taskID string) error {
	// Normalize: accept both "TASK-007" and "007"
	if !strings.HasPrefix(taskID, "TASK-") {
		taskID = "TASK-" + taskID
	}

	// Search all plan files (including ignored, excluding completed) for this task
	files, err := parser.GlobPlanFiles(dir, false)
	if err != nil {
		return err
	}

	for _, f := range files {
		tasks, err := parser.ParseFile(f)
		if err != nil {
			return err
		}

		for _, t := range tasks {
			if t.ID != taskID {
				continue
			}

			// Found the task
			if t.Ignored {
				cmd.Println(fmt.Sprintf("Task %s is already ignored", taskID))
				return nil
			}

			// Warn if plan is ignored
			if parser.IsIgnoredFile(f) {
				cmd.PrintErrln(fmt.Sprintf("Warning: plan is already ignored (%s)", filepath.Base(f)))
			}

			// Rewrite the heading atomically
			if err := rewriteTaskHeading(f, taskID, false); err != nil {
				return err
			}

			cmd.Println(fmt.Sprintf("Ignored task %s in %s", taskID, filepath.Base(f)))
			return nil
		}
	}

	cmd.PrintErrln(fmt.Sprintf("Error: task %s not found", taskID))
	return fmt.Errorf("task %s not found", taskID)
}

// rewriteTaskHeading rewrites a task heading to add or remove the IGNORED prefix.
// If addIgnored is false, it changes "### TASK-NNN:" to "### IGNORED TASK-NNN:".
// If addIgnored is true, it changes "### IGNORED TASK-NNN:" to "### TASK-NNN:".
// The file is written atomically (write to temp file, then rename).
func rewriteTaskHeading(filePath string, taskID string, removeIgnored bool) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read %s: %w", filePath, err)
	}

	lines := strings.Split(string(data), "\n")
	found := false

	for i, line := range lines {
		m := parser.TaskHeadingRe.FindStringSubmatch(line)
		if m == nil || m[2] != taskID {
			continue
		}

		found = true
		title := m[3]
		if removeIgnored {
			lines[i] = fmt.Sprintf("### %s: %s", taskID, title)
		} else {
			lines[i] = fmt.Sprintf("### IGNORED %s: %s", taskID, title)
		}
		break
	}

	if !found {
		return fmt.Errorf("task heading %s not found in %s", taskID, filePath)
	}

	// Atomic write: temp file + rename
	tmpFile := filePath + ".tmp"
	if err := os.WriteFile(tmpFile, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := os.Rename(tmpFile, filePath); err != nil {
		os.Remove(tmpFile) // best-effort cleanup
		return fmt.Errorf("rename temp file: %w", err)
	}

	return nil
}

func init() {
	ignoreCmd.AddCommand(ignorePlanCmd)
	ignoreCmd.AddCommand(ignoreTaskCmd)
	rootCmd.AddCommand(ignoreCmd)
}
