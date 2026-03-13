package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dirnei/maggus/internal/tasklock"
	"github.com/dirnei/maggus/internal/worktree"
	"github.com/spf13/cobra"
)

const maggusWorkDir = ".maggus-work"

var worktreeCmd = &cobra.Command{
	Use:   "worktree",
	Short: "Manage Maggus worktrees",
	Long:  "Commands for listing and cleaning up git worktrees created by maggus work --worktree.",
}

var worktreeCleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Remove all worktrees in .maggus-work/ and their associated branches",
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}
		return runWorktreeClean(cmd, dir)
	},
}

var worktreeListCmd = &cobra.Command{
	Use:   "list",
	Short: "Show active worktrees with their run IDs and branches",
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}
		return runWorktreeList(cmd, dir)
	},
}

func init() {
	worktreeCmd.AddCommand(worktreeCleanCmd)
	worktreeCmd.AddCommand(worktreeListCmd)
	rootCmd.AddCommand(worktreeCmd)
}

func runWorktreeClean(cmd *cobra.Command, dir string) error {
	w := cmd.OutOrStdout()
	workDir := filepath.Join(dir, maggusWorkDir)

	entries, err := os.ReadDir(workDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintln(w, "No worktrees found.")
			return nil
		}
		return fmt.Errorf("read %s: %w", maggusWorkDir, err)
	}

	if len(entries) == 0 {
		fmt.Fprintln(w, "No worktrees found.")
		return nil
	}

	// Get worktree details to find associated branches.
	details, _ := worktree.ListDetailed(dir)
	branchByPath := make(map[string]string)
	for _, d := range details {
		branchByPath[filepath.ToSlash(d.Path)] = d.Branch
	}

	removed := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		wtPath := filepath.Join(workDir, e.Name())

		// Remove worktree via git.
		if err := worktree.Remove(dir, wtPath); err != nil {
			fmt.Fprintf(w, "Warning: could not remove worktree %s: %v\n", e.Name(), err)
			continue
		}

		// Delete the associated branch if known.
		normalizedPath := filepath.ToSlash(wtPath)
		if branch, ok := branchByPath[normalizedPath]; ok {
			// Strip refs/heads/ prefix.
			shortBranch := strings.TrimPrefix(branch, "refs/heads/")
			if err := worktree.DeleteBranch(dir, shortBranch); err != nil {
				fmt.Fprintf(w, "Warning: could not delete branch %s: %v\n", shortBranch, err)
			} else {
				fmt.Fprintf(w, "Removed worktree %s (branch %s)\n", e.Name(), shortBranch)
			}
		} else {
			fmt.Fprintf(w, "Removed worktree %s\n", e.Name())
		}
		removed++
	}

	// Prune stale worktree references.
	if err := worktree.Prune(dir); err != nil {
		fmt.Fprintf(w, "Warning: git worktree prune failed: %v\n", err)
	}

	// Clean up all lock files.
	if err := tasklock.CleanAll(dir); err != nil {
		fmt.Fprintf(w, "Warning: could not clean lock files: %v\n", err)
	} else {
		fmt.Fprintln(w, "Cleaned lock files.")
	}

	// Remove the .maggus-work directory itself if now empty.
	remaining, _ := os.ReadDir(workDir)
	if len(remaining) == 0 {
		os.Remove(workDir)
	}

	fmt.Fprintf(w, "Cleaned %d worktree(s).\n", removed)
	return nil
}

func runWorktreeList(cmd *cobra.Command, dir string) error {
	w := cmd.OutOrStdout()
	workDir := filepath.Join(dir, maggusWorkDir)

	entries, err := os.ReadDir(workDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintln(w, "No worktrees found.")
			return nil
		}
		return fmt.Errorf("read %s: %w", maggusWorkDir, err)
	}

	if len(entries) == 0 {
		fmt.Fprintln(w, "No worktrees found.")
		return nil
	}

	// Get worktree details to find associated branches.
	details, _ := worktree.ListDetailed(dir)
	branchByPath := make(map[string]string)
	for _, d := range details {
		branchByPath[filepath.ToSlash(d.Path)] = d.Branch
	}

	fmt.Fprintln(w, "Active worktrees:")
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		runID := e.Name()
		wtPath := filepath.Join(workDir, runID)
		normalizedPath := filepath.ToSlash(wtPath)

		branch := "(unknown)"
		if b, ok := branchByPath[normalizedPath]; ok {
			branch = strings.TrimPrefix(b, "refs/heads/")
		}

		fmt.Fprintf(w, "  %s  branch: %s\n", runID, branch)
	}
	return nil
}
