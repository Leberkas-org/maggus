package agent

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
)

type OpenCodeAgent struct{}

func (a *OpenCodeAgent) Name() string { return "opencode" }

func (a *OpenCodeAgent) Validate() error {
	_, err := exec.LookPath("opencode")
	if err != nil {
		return fmt.Errorf("opencode CLI not found in PATH")
	}
	return nil
}

func (a *OpenCodeAgent) Run(ctx context.Context, opts RunOptions) error {
	args := []string{"--non-interactive"}
	if opts.Model != "" {
		args = append(args, "--model", opts.Model)
	}
	args = append(args, opts.Prompt)

	cmd := exec.CommandContext(ctx, "opencode", args...)
	cmd.Dir = opts.WorkDir

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start opencode: %w", err)
	}

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		if opts.Output != nil {
			opts.Output.OnOutput(scanner.Text())
		}
	}

	if err := cmd.Wait(); err != nil {
		if opts.Output != nil {
			opts.Output.OnComplete(false)
		}
		return fmt.Errorf("opencode exited: %w", err)
	}

	if opts.Output != nil {
		opts.Output.OnComplete(true)
	}
	return nil
}
