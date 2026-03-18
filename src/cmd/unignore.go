package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/leberkas-org/maggus/internal/parser"
	"github.com/spf13/cobra"
)

var unignoreCmd = &cobra.Command{
	Use:          "unignore",
	Short:        "Re-include ignored plans or tasks in the work loop",
	Long:         `Unignore a plan or task so that it is picked up again by maggus work.`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		_ = cmd.Help()
		return fmt.Errorf("a subcommand is required")
	},
}

var unignorePlanCmd = &cobra.Command{
	Use:   "plan <plan-id>",
	Short: "Unignore a plan file",
	Long:  `Renames plan_<N>_ignored.md back to plan_<N>.md so that it is included in the work loop again.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}
		return runUnignorePlan(cmd, dir, args[0])
	},
}

func runUnignorePlan(cmd *cobra.Command, dir string, planID string) error {
	file, state, err := findPlanFile(dir, planID)
	if err != nil {
		return err
	}

	switch state {
	case planStateNotFound:
		cmd.PrintErrln(fmt.Sprintf("Error: plan %s not found", planID))
		return fmt.Errorf("plan %s not found", planID)
	case planStateCompleted:
		cmd.PrintErrln(fmt.Sprintf("Error: cannot unignore a completed plan (plan %s)", planID))
		return fmt.Errorf("cannot unignore a completed plan")
	case planStateActive:
		cmd.PrintErrln(fmt.Sprintf("Error: plan %s is not currently ignored", planID))
		return fmt.Errorf("plan %s is not currently ignored", planID)
	case planStateIgnored:
		newName := strings.TrimSuffix(file, "_ignored.md") + ".md"
		if err := os.Rename(file, newName); err != nil {
			return fmt.Errorf("rename %s: %w", file, err)
		}
		cmd.Println(fmt.Sprintf("Unignored plan %s (%s → %s)", planID, filepath.Base(file), filepath.Base(newName)))
		return nil
	}

	return nil
}

var unignoreTaskCmd = &cobra.Command{
	Use:   "task <TASK-NNN>",
	Short: "Unignore a single task",
	Long:  `Rewrites the task heading from "### IGNORED TASK-NNN:" to "### TASK-NNN:" so that it is picked up by the work loop again.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}
		return runUnignoreTask(cmd, dir, args[0])
	},
}

func runUnignoreTask(cmd *cobra.Command, dir string, taskID string) error {
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

			// Found the task — must be ignored to unignore
			if !t.Ignored {
				cmd.PrintErrln(fmt.Sprintf("Error: task %s is not currently ignored", taskID))
				return fmt.Errorf("task %s is not currently ignored", taskID)
			}

			// Rewrite the heading atomically (removeIgnored=true)
			if err := rewriteTaskHeading(f, taskID, true); err != nil {
				return err
			}

			cmd.Println(fmt.Sprintf("Unignored task %s in %s", taskID, filepath.Base(f)))
			return nil
		}
	}

	cmd.PrintErrln(fmt.Sprintf("Error: task %s not found", taskID))
	return fmt.Errorf("task %s not found", taskID)
}

func init() {
	unignoreCmd.AddCommand(unignorePlanCmd)
	unignoreCmd.AddCommand(unignoreTaskCmd)
	rootCmd.AddCommand(unignoreCmd)
}
