package git

import (
	"fmt"
	"slices"
	"strings"
)

func (o *ops) CurrentBranch(dir string) (string, error) {
	return o.cmd.Output(dir, "rev-parse", "--abbrev-ref", "HEAD")
}

func (o *ops) DefaultBranch(dir string) (string, error) {
	out, err := o.cmd.Output(dir, "symbolic-ref", "refs/remotes/origin/HEAD")
	if err == nil {
		return strings.TrimPrefix(out, "refs/remotes/origin/"), nil
	}
	// Fallback: check if main or master exists
	for _, name := range []string{"main", "master"} {
		if o.BranchExists(dir, name) {
			return name, nil
		}
	}
	return "", fmt.Errorf("cannot determine default branch")
}

func (o *ops) BranchExists(dir string, branch string) bool {
	err := o.cmd.Run(dir, "rev-parse", "--verify", "refs/heads/"+branch)
	return err == nil
}

func (o *ops) CreateBranch(dir, name, from string) error {
	if o.IsProtected(name) {
		return fmt.Errorf("cannot create branch: %q is protected", name)
	}
	return o.cmd.Run(dir, "branch", name, from)
}

func (o *ops) CheckoutBranch(dir, name string) error {
	return o.cmd.Run(dir, "checkout", name)
}

func (o *ops) DeleteBranch(dir, name string) error {
	if o.IsProtected(name) {
		return fmt.Errorf("cannot delete branch: %q is protected", name)
	}
	return o.cmd.Run(dir, "branch", "-d", name)
}

func (o *ops) IsProtected(branch string) bool {
	return slices.Contains(o.protectedBranches, branch)
}
