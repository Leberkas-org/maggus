package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Remove completed feature and bug files",
	Long:  `Removes all _completed.md files from .maggus/features/ and .maggus/bugs/.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dryRun, err := cmd.Flags().GetBool("dry-run")
		if err != nil {
			return err
		}

		dir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}

		return runClean(cmd, dir, dryRun)
	},
}

func runClean(cmd *cobra.Command, dir string, dryRun bool) error {
	out := cmd.OutOrStdout()

	completedFeatures, err := findCompletedFeatures(dir)
	if err != nil {
		return err
	}

	completedBugs, err := findCompletedBugs(dir)
	if err != nil {
		return err
	}

	if len(completedFeatures) == 0 && len(completedBugs) == 0 {
		fmt.Fprintln(out, "Nothing to clean.")
		return nil
	}

	if dryRun {
		fmt.Fprintln(out, "Dry run — the following would be removed:")
		fmt.Fprintln(out)
	}

	for _, p := range completedFeatures {
		rel, _ := filepath.Rel(dir, p)
		if rel == "" {
			rel = p
		}
		if dryRun {
			fmt.Fprintf(out, "  feature: %s\n", filepath.ToSlash(rel))
		} else {
			if err := os.Remove(p); err != nil {
				return fmt.Errorf("remove feature %s: %w", rel, err)
			}
		}
	}

	for _, p := range completedBugs {
		rel, _ := filepath.Rel(dir, p)
		if rel == "" {
			rel = p
		}
		if dryRun {
			fmt.Fprintf(out, "  bug: %s\n", filepath.ToSlash(rel))
		} else {
			if err := os.Remove(p); err != nil {
				return fmt.Errorf("remove bug %s: %w", rel, err)
			}
		}
	}

	if dryRun {
		fmt.Fprintln(out)
		fmt.Fprintf(out, "Would remove %d completed feature file(s) and %d completed bug file(s).\n", len(completedFeatures), len(completedBugs))
	} else {
		fmt.Fprintf(out, "Removed %d completed feature file(s) and %d completed bug file(s).\n", len(completedFeatures), len(completedBugs))
	}

	return nil
}

// findCompletedFeatures returns paths to all _completed.md feature files in .maggus/features/.
func findCompletedFeatures(dir string) ([]string, error) {
	pattern := filepath.Join(dir, ".maggus", "features", "feature_*_completed.md")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("glob completed features: %w", err)
	}
	return files, nil
}

// findCompletedBugs returns paths to all _completed.md bug files in .maggus/bugs/.
func findCompletedBugs(dir string) ([]string, error) {
	pattern := filepath.Join(dir, ".maggus", "bugs", "bug_*_completed.md")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("glob completed bugs: %w", err)
	}
	return files, nil
}

func init() {
	rootCmd.AddCommand(cleanCmd)
	cleanCmd.Flags().Bool("dry-run", false, "Show what would be removed without actually deleting anything")
}
