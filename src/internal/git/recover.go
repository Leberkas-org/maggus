package git

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func (o *ops) RecoverDirtyState(repoRoot string) error {
	worktreesDir := filepath.Join(repoRoot, ".maggus", "worktrees")

	if err := o.recoverUncommitted(worktreesDir); err != nil {
		o.log.Warn("recover uncommitted failed", "error", err)
	}

	if err := o.consolidateOrphanedBranches(repoRoot); err != nil {
		o.log.Warn("consolidate orphaned branches failed", "error", err)
	}

	if err := o.cleanOrphanedWorktrees(repoRoot, worktreesDir); err != nil {
		o.log.Warn("clean orphaned worktrees failed", "error", err)
	}

	_ = o.PruneWorktrees(repoRoot)
	if o.RemoteExists(repoRoot) {
		_ = o.cmd.Run(repoRoot, "remote", "prune", "origin")
	}

	return nil
}

func (o *ops) recoverUncommitted(worktreesDir string) error {
	entries, err := os.ReadDir(worktreesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		wtPath := filepath.Join(worktreesDir, entry.Name())
		if !o.HasChanges(wtPath) {
			continue
		}

		msg := "chore: recover uncommitted changes from interrupted run"
		if commitMsg, err := o.ReadCommitFile(wtPath); err == nil && commitMsg != "" {
			msg = commitMsg
		}

		if err := o.StageAll(wtPath); err != nil {
			o.log.Warn("stage failed during recovery", "worktree", wtPath, "error", err)
			continue
		}
		hash, err := o.Commit(wtPath, msg)
		if err != nil {
			o.log.Warn("commit failed during recovery", "worktree", wtPath, "error", err)
			continue
		}
		o.log.Info("recovered uncommitted changes", "commit", hash, "worktree", wtPath)
	}
	return nil
}

func (o *ops) consolidateOrphanedBranches(repoRoot string) error {
	out, err := o.cmd.Output(repoRoot, "branch", "--list")
	if err != nil {
		return err
	}

	for line := range strings.SplitSeq(out, "\n") {
		branch := strings.TrimSpace(strings.TrimPrefix(line, "*"))
		if branch == "" {
			continue
		}

		parts := strings.SplitN(branch, "/", 3)
		if len(parts) < 3 {
			continue
		}
		featureBranch := parts[0] + "/" + parts[1]

		if !o.BranchExists(repoRoot, featureBranch) {
			continue
		}

		ahead, err := o.cmd.Output(repoRoot, "rev-list", "--count", featureBranch+".."+branch)
		if err != nil {
			continue
		}
		if strings.TrimSpace(ahead) == "0" {
			o.log.Info("deleting orphaned branch", "branch", branch)
			_ = o.cmd.Run(repoRoot, "branch", "-D", branch)
			continue
		}

		o.log.Info("merging orphaned branch", "branch", branch, "into", featureBranch)
		if err := o.MergeTaskBranch(repoRoot, featureBranch, branch); err != nil {
			o.log.Warn("merge failed, leaving for manual review", "branch", branch, "error", err)
			continue
		}
		_ = o.cmd.Run(repoRoot, "branch", "-d", branch)
	}
	return nil
}

func (o *ops) cleanOrphanedWorktrees(repoRoot, worktreesDir string) error {
	trees, err := o.ListWorktrees(repoRoot)
	if err != nil {
		return fmt.Errorf("list worktrees: %w", err)
	}

	knownPaths := make(map[string]bool)
	for _, t := range trees {
		knownPaths[t.Path] = true
	}

	entries, err := os.ReadDir(worktreesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		wtPath := filepath.Join(worktreesDir, entry.Name())

		if knownPaths[wtPath] {
			if err := o.RemoveWorktree(repoRoot, wtPath); err != nil {
				o.log.Warn("remove worktree failed", "path", wtPath, "error", err)
			}
		} else {
			o.log.Info("removing orphaned directory", "path", wtPath)
			if err := os.RemoveAll(wtPath); err != nil {
				o.log.Warn("remove directory failed", "path", wtPath, "error", err)
			}
		}
	}

	return nil
}
