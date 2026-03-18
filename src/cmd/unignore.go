package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var unignoreCmd = &cobra.Command{
	Use:   "unignore",
	Short: "Re-include ignored plans or tasks in the work loop",
	Long:  `Unignore a plan or task so that it is picked up again by maggus work.`,
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

func init() {
	unignoreCmd.AddCommand(unignorePlanCmd)
	rootCmd.AddCommand(unignoreCmd)
}
