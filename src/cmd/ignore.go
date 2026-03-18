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
	Use:   "ignore",
	Short: "Exclude plans or tasks from the work loop",
	Long:  `Ignore a plan or task so that it is skipped by maggus work.`,
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

func init() {
	ignoreCmd.AddCommand(ignorePlanCmd)
	rootCmd.AddCommand(ignoreCmd)
}
