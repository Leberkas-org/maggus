package git

type Operations interface {
	// Branch
	CurrentBranch(dir string) (string, error)
	DefaultBranch(dir string) (string, error)
	BranchExists(dir string, branch string) bool
	CreateBranch(dir, name, from string) error
	CheckoutBranch(dir, name string) error
	DeleteBranch(dir, name string) error
	IsProtected(branch string) bool

	// Worktree
	CreateWorktree(repoRoot, path, branch string) error
	RemoveWorktree(repoRoot, path string) error
	ListWorktrees(repoRoot string) ([]WorktreeInfo, error)
	PruneWorktrees(repoRoot string) error

	// Merge
	MergeTaskBranch(repoRoot, featureBranch, taskBranch string) error
	RebaseOnto(dir, upstream, branch string) error
	AbortRebase(dir string) error

	// Commit
	StageAll(dir string) error
	Commit(dir, message string) (string, error)
	HasChanges(dir string) bool
	ReadCommitFile(dir string) (string, error)

	// Sync
	Fetch(dir string) error
	Pull(dir string) error
	RemoteExists(dir string) bool
	RepoURL(dir string) string

	// Recovery
	RecoverDirtyState(repoRoot string) error
}

type WorktreeInfo struct {
	Path   string
	Branch string
	Bare   bool
}
