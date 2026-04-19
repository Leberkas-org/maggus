package agent

import "context"

type Agent interface {
	Run(ctx context.Context, opts RunOptions) error
	Name() string
	Validate() error
}

type RunOptions struct {
	Prompt  string
	Model   string
	WorkDir string
	Output  OutputSink
}
