package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/leberkas-org/maggus/internal/gitbranch"
	"github.com/leberkas-org/maggus/internal/gitutil"
	"github.com/leberkas-org/maggus/internal/gitworktree"
	"github.com/leberkas-org/maggus/internal/parser"
	"github.com/leberkas-org/maggus/internal/runlog"
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

	finishedWorkers := findFinishedDispatchWorkers(dir)

	if len(completedFeatures) == 0 && len(completedBugs) == 0 && len(finishedWorkers) == 0 {
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
			maggusID := parser.ParseMaggusID(p)
			if err := os.Remove(p); err != nil {
				return fmt.Errorf("remove feature %s: %w", rel, err)
			}
			removeLogDir(dir, maggusID)
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
			maggusID := parser.ParseMaggusID(p)
			if err := os.Remove(p); err != nil {
				return fmt.Errorf("remove bug %s: %w", rel, err)
			}
			removeLogDir(dir, maggusID)
		}
	}

	if dryRun {
		fmt.Fprintln(out)
		fmt.Fprintf(out, "Would remove %d completed feature file(s) and %d completed bug file(s).\n", len(completedFeatures), len(completedBugs))
	} else {
		fmt.Fprintf(out, "Removed %d completed feature file(s) and %d completed bug file(s).\n", len(completedFeatures), len(completedBugs))
	}

	// Clean up finished dispatch worktrees (done/failed entries in the workers index).
	if len(finishedWorkers) > 0 {
		if err := cleanFinishedDispatchWorkers(out, dir, finishedWorkers, dryRun); err != nil {
			return err
		}
	}

	return nil
}

// findFinishedDispatchWorkers returns worker index entries that are done or failed.
// These correspond to dispatched task workers whose worktrees can be cleaned up.
func findFinishedDispatchWorkers(dir string) []runlog.WorkerIndexEntry {
	all := runlog.ReadWorkersIndex(dir)
	var finished []runlog.WorkerIndexEntry
	for _, w := range all {
		if w.Status == "done" || w.Status == "failed" {
			finished = append(finished, w)
		}
	}
	return finished
}

// cleanFinishedDispatchWorkers removes the worktree, per-worker snapshot, and
// workers index entry for each finished dispatch worker.
func cleanFinishedDispatchWorkers(out io.Writer, dir string, workers []runlog.WorkerIndexEntry, dryRun bool) error {
	removed := 0
	for _, w := range workers {
		worktreePath := filepath.Join(dir, ".maggus", "worktrees", w.TaskID)
		relWorktree, _ := filepath.Rel(dir, worktreePath)
		if relWorktree == "" {
			relWorktree = worktreePath
		}

		if dryRun {
			fmt.Fprintf(out, "  dispatch worker: %s (%s)\n", w.TaskID, w.Status)
			continue
		}

		// Remove the git worktree (try git first, then os.RemoveAll as fallback).
		if _, statErr := os.Stat(worktreePath); statErr == nil {
			if err := gitworktree.RemoveWorktree(dir, worktreePath); err != nil {
				// Git worktree remove failed (e.g. stale metadata or not a git worktree).
				// Fall back to plain directory removal.
				_ = os.RemoveAll(worktreePath)
			}
		}

		// Best-effort delete the task branch (may already be gone after a
		// successful merge-back, but still exists after a failed merge).
		taskBranch := gitbranch.BranchName(w.TaskID)
		delCmd := gitutil.Command("branch", "-D", taskBranch)
		delCmd.Dir = dir
		_, _ = delCmd.CombinedOutput()

		// Remove the per-worker snapshot file.
		runlog.RemoveWorkerSnapshot(dir, w.TaskID)

		removed++
	}

	if dryRun {
		fmt.Fprintf(out, "\nWould remove %d dispatch worker worktree(s).\n", len(workers))
		return nil
	}

	// Update the workers index: remove entries for cleaned-up workers.
	if removed > 0 {
		cleanedIDs := make(map[string]bool, len(workers))
		for _, w := range workers {
			cleanedIDs[w.TaskID] = true
		}
		remaining := runlog.ReadWorkersIndex(dir)
		var kept []runlog.WorkerIndexEntry
		for _, w := range remaining {
			if !cleanedIDs[w.TaskID] {
				kept = append(kept, w)
			}
		}
		if len(kept) == 0 {
			runlog.RemoveWorkersIndex(dir)
		} else {
			_ = runlog.WriteWorkersIndex(dir, kept)
		}
		fmt.Fprintf(out, "Removed %d dispatch worker worktree(s).\n", removed)
	}

	return nil
}

// removeLogDir removes the per-feature log directory .maggus/logs/<maggusID>/ when
// maggusID is non-empty. Errors are silently ignored (best-effort cleanup).
func removeLogDir(dir, maggusID string) {
	if maggusID == "" {
		return
	}
	logDir := filepath.Join(dir, ".maggus", "logs", maggusID)
	_ = os.RemoveAll(logDir)
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
