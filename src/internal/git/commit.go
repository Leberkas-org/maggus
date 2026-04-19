package git

import (
	"os"
	"path/filepath"
	"strings"
)

func (o *ops) StageAll(dir string) error {
	return o.cmd.Run(dir, "add", "-A")
}

func (o *ops) Commit(dir, message string) (string, error) {
	if err := o.cmd.Run(dir, "commit", "-m", message); err != nil {
		return "", err
	}
	hash, err := o.cmd.Output(dir, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	return hash, nil
}

func (o *ops) HasChanges(dir string) bool {
	out, err := o.cmd.Output(dir, "status", "--porcelain")
	if err != nil {
		return false
	}
	return strings.TrimSpace(out) != ""
}

func (o *ops) ReadCommitFile(dir string) (string, error) {
	path := filepath.Join(dir, "COMMIT.md")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	msg := strings.TrimSpace(string(data))
	if err := os.Remove(path); err != nil {
		return msg, err
	}
	return msg, nil
}
