package git

import (
	"fmt"
	"log"
)

func (o *ops) MergeTaskBranch(repoRoot, featureBranch, taskBranch string) error {
	if err := o.CheckoutBranch(repoRoot, featureBranch); err != nil {
		return fmt.Errorf("checkout feature branch: %w", err)
	}

	if o.RemoteExists(repoRoot) {
		_ = o.Pull(repoRoot)
	}

	if err := o.RebaseOnto(repoRoot, featureBranch, taskBranch); err != nil {
		log.Printf("rebase failed, falling back to merge: %v", err)
		_ = o.AbortRebase(repoRoot)

		if err := o.CheckoutBranch(repoRoot, featureBranch); err != nil {
			return fmt.Errorf("checkout for merge fallback: %w", err)
		}
		if err := o.cmd.Run(repoRoot, "merge", taskBranch, "-m",
			fmt.Sprintf("Merge %s into %s (rebase failed)", taskBranch, featureBranch)); err != nil {
			return fmt.Errorf("merge fallback: %w", err)
		}
		return nil
	}

	if err := o.CheckoutBranch(repoRoot, featureBranch); err != nil {
		return fmt.Errorf("checkout for ff merge: %w", err)
	}

	if err := o.cmd.Run(repoRoot, "merge", "--ff-only", taskBranch); err != nil {
		return o.cmd.Run(repoRoot, "merge", taskBranch, "-m",
			fmt.Sprintf("Merge %s into %s", taskBranch, featureBranch))
	}

	return nil
}

func (o *ops) RebaseOnto(dir, upstream, branch string) error {
	return o.cmd.Run(dir, "rebase", upstream, branch)
}

func (o *ops) AbortRebase(dir string) error {
	return o.cmd.Run(dir, "rebase", "--abort")
}
