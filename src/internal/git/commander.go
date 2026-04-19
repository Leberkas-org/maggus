package git

import (
	"fmt"
	"os/exec"
	"strings"
)

type Commander interface {
	Run(dir string, args ...string) error
	Output(dir string, args ...string) (string, error)
	CombinedOutput(dir string, args ...string) (string, error)
}

type ExecCommander struct{}

func NewCommander() Commander {
	return &ExecCommander{}
}

func (c *ExecCommander) Run(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, string(out))
	}
	return nil
}

func (c *ExecCommander) Output(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, string(ee.Stderr))
		}
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out)), nil
}

func (c *ExecCommander) CombinedOutput(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, string(out))
	}
	return strings.TrimSpace(string(out)), nil
}
