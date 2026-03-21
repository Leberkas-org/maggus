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
	Short:        "Re-include ignored features or tasks in the work loop",
	Long:         `Unignore a feature or task so that it is picked up again by maggus work.`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		_ = cmd.Help()
		return fmt.Errorf("a subcommand is required")
	},
}

var unignoreFeatureCmd = &cobra.Command{
	Use:   "feature <feature-id>",
	Short: "Unignore a feature file",
	Long:  `Renames feature_<N>_ignored.md back to feature_<N>.md so that it is included in the work loop again.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}
		return runUnignoreFeature(cmd, dir, args[0])
	},
}

func runUnignoreFeature(cmd *cobra.Command, dir string, featureID string) error {
	file, state, err := findFeatureFile(dir, featureID)
	if err != nil {
		return err
	}

	switch state {
	case featureStateNotFound:
		cmd.PrintErrln(fmt.Sprintf("Error: feature %s not found", featureID))
		return fmt.Errorf("feature %s not found", featureID)
	case featureStateCompleted:
		cmd.PrintErrln(fmt.Sprintf("Error: cannot unignore a completed feature (feature %s)", featureID))
		return fmt.Errorf("cannot unignore a completed feature")
	case featureStateActive:
		cmd.PrintErrln(fmt.Sprintf("Error: feature %s is not currently ignored", featureID))
		return fmt.Errorf("feature %s is not currently ignored", featureID)
	case featureStateIgnored:
		newName := strings.TrimSuffix(file, "_ignored.md") + ".md"
		if err := os.Rename(file, newName); err != nil {
			return fmt.Errorf("rename %s: %w", file, err)
		}
		cmd.Println(fmt.Sprintf("Unignored feature %s (%s → %s)", featureID, filepath.Base(file), filepath.Base(newName)))
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

	// Search all feature files (including ignored, excluding completed) for this task
	files, err := parser.GlobFeatureFiles(dir, false)
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
	unignoreCmd.AddCommand(unignoreFeatureCmd)
	unignoreCmd.AddCommand(unignoreTaskCmd)
	rootCmd.AddCommand(unignoreCmd)
}
