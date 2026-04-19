package git

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func (o *ops) RecoverDirtyState(repoRoot string) error {
	worktreesDir := filepath.Join(repoRoot, ".maggus", "worktrees")

	if err := o.recoverUncommitted(repoRoot, worktreesDir); err != nil {
		log.Printf("recover uncommitted: %v", err)
	}

	if err := o.consolidateOrphanedBranches(repoRoot); err != nil {
		log.Printf("consolidate orphaned branches: %v", err)
	}

	if err := o.cleanOrphanedWorktrees(repoRoot, worktreesDir); err != nil {
		log.Printf("clean orphaned worktrees: %v", err)
	}

	_ = o.PruneWorktrees(repoRoot)
	if o.RemoteExists(repoRoot) {
		_ = o.cmd.Run(repoRoot, "remote", "prune", "origin")
	}

	return nil
}

func (o *ops) recoverUncommitted(_ string, worktreesDir string) error {
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
			log.Printf("stage in %s: %v", wtPath, err)
			continue
		}
		hash, err := o.Commit(wtPath, msg)
		if err != nil {
			log.Printf("commit in %s: %v", wtPath, err)
			continue
		}
		log.Printf("recovered commit %s in %s", hash, wtPath)
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

		// Task branches contain a "/" after the feature branch prefix
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
			log.Printf("deleting orphaned branch %s (no commits ahead)", branch)
			_ = o.cmd.Run(repoRoot, "branch", "-D", branch)
			continue
		}

		log.Printf("merging orphaned branch %s into %s", branch, featureBranch)
		if err := o.MergeTaskBranch(repoRoot, featureBranch, branch); err != nil {
			log.Printf("merge failed for %s, leaving for manual review: %v", branch, err)
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
				log.Printf("remove worktree %s: %v", wtPath, err)
			}
		} else {
			log.Printf("removing orphaned directory %s", wtPath)
			if err := os.RemoveAll(wtPath); err != nil {
				log.Printf("remove dir %s: %v", wtPath, err)
			}
		}
	}

	return nil
}
