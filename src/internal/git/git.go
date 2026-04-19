package git

import "log/slog"

type ops struct {
	cmd               Commander
	protectedBranches []string
	log               *slog.Logger
}

func New(cmd Commander, protectedBranches []string, logger ...*slog.Logger) Operations {
	l := slog.Default()
	if len(logger) > 0 && logger[0] != nil {
		l = logger[0]
	}
	return &ops{
		cmd:               cmd,
		protectedBranches: protectedBranches,
		log:               l,
	}
}
