package git

type ops struct {
	cmd               Commander
	protectedBranches []string
}

func New(cmd Commander, protectedBranches []string) Operations {
	return &ops{
		cmd:               cmd,
		protectedBranches: protectedBranches,
	}
}
