package git

import (
	"bufio"
	"strings"
)

func (o *ops) CreateWorktree(repoRoot, path, branch string) error {
	return o.cmd.Run(repoRoot, "worktree", "add", path, branch)
}

func (o *ops) RemoveWorktree(repoRoot, path string) error {
	return o.cmd.Run(repoRoot, "worktree", "remove", "--force", path)
}

func (o *ops) ListWorktrees(repoRoot string) ([]WorktreeInfo, error) {
	out, err := o.cmd.Output(repoRoot, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}

	var trees []WorktreeInfo
	var current WorktreeInfo

	scanner := bufio.NewScanner(strings.NewReader(out))
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "worktree "):
			current.Path = strings.TrimPrefix(line, "worktree ")
		case strings.HasPrefix(line, "branch "):
			current.Branch = strings.TrimPrefix(line, "branch refs/heads/")
		case line == "bare":
			current.Bare = true
		case line == "":
			if current.Path != "" {
				trees = append(trees, current)
			}
			current = WorktreeInfo{}
		}
	}
	if current.Path != "" {
		trees = append(trees, current)
	}

	return trees, nil
}

func (o *ops) PruneWorktrees(repoRoot string) error {
	return o.cmd.Run(repoRoot, "worktree", "prune")
}
