package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Remove completed plan files and finished run directories",
	Long:  `Removes all _completed.md plan files from .maggus/ and run directories that have finished (contain an ## End section in run.md).`,
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

	completedPlans, err := findCompletedPlans(dir)
	if err != nil {
		return err
	}

	completedRuns, err := findCompletedRuns(dir)
	if err != nil {
		return err
	}

	if len(completedPlans) == 0 && len(completedRuns) == 0 {
		fmt.Fprintln(out, "Nothing to clean.")
		return nil
	}

	if dryRun {
		fmt.Fprintln(out, "Dry run — the following would be removed:")
		fmt.Fprintln(out)
	}

	for _, p := range completedPlans {
		rel, _ := filepath.Rel(dir, p)
		if rel == "" {
			rel = p
		}
		if dryRun {
			fmt.Fprintf(out, "  plan: %s\n", filepath.ToSlash(rel))
		} else {
			if err := os.Remove(p); err != nil {
				return fmt.Errorf("remove plan %s: %w", rel, err)
			}
		}
	}

	for _, r := range completedRuns {
		rel, _ := filepath.Rel(dir, r)
		if rel == "" {
			rel = r
		}
		if dryRun {
			fmt.Fprintf(out, "  run:  %s\n", filepath.ToSlash(rel))
		} else {
			if err := os.RemoveAll(r); err != nil {
				return fmt.Errorf("remove run dir %s: %w", rel, err)
			}
		}
	}

	if dryRun {
		fmt.Fprintln(out)
		fmt.Fprintf(out, "Would remove %d completed plan(s), %d run directory(ies).\n", len(completedPlans), len(completedRuns))
	} else {
		fmt.Fprintf(out, "Removed %d completed plan(s), %d run directory(ies).\n", len(completedPlans), len(completedRuns))
	}

	return nil
}

// findCompletedPlans returns paths to all _completed.md plan files in .maggus/.
func findCompletedPlans(dir string) ([]string, error) {
	pattern := filepath.Join(dir, ".maggus", "plan_*_completed.md")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("glob completed plans: %w", err)
	}
	return files, nil
}

// findCompletedRuns returns paths to run directories in .maggus/runs/ whose run.md contains an "## End" section.
func findCompletedRuns(dir string) ([]string, error) {
	runsDir := filepath.Join(dir, ".maggus", "runs")
	entries, err := os.ReadDir(runsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read runs directory: %w", err)
	}

	var completed []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		runMdPath := filepath.Join(runsDir, e.Name(), "run.md")
		if hasEndSection(runMdPath) {
			completed = append(completed, filepath.Join(runsDir, e.Name()))
		}
	}
	return completed, nil
}

// hasEndSection returns true if the file at path contains a line starting with "## End".
func hasEndSection(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), "## End") {
			return true
		}
	}
	return false
}

func init() {
	rootCmd.AddCommand(cleanCmd)
	cleanCmd.Flags().Bool("dry-run", false, "Show what would be removed without actually deleting anything")
}
