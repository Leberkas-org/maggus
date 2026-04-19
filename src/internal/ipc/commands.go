package ipc

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type FileCommandWriter struct {
	dir string
}

func NewFileCommandWriter(globalDir string) *FileCommandWriter {
	return &FileCommandWriter{dir: globalDir}
}

func (c *FileCommandWriter) StopAll() error {
	return c.writeSentinel("cmd.stop", nil)
}

func (c *FileCommandWriter) StopRepo(repoURL string) error {
	return c.writeSentinel("cmd.stop."+sanitize(repoURL), nil)
}

func (c *FileCommandWriter) Approve(itemID string) error {
	return c.writeSentinel("cmd.approve."+itemID, nil)
}

func (c *FileCommandWriter) Skip(itemID string) error {
	return c.writeSentinel("cmd.skip."+itemID, nil)
}

func (c *FileCommandWriter) Reorder(priorities map[string]int) error {
	return c.writeSentinel("cmd.reorder", priorities)
}

func (c *FileCommandWriter) writeSentinel(name string, data any) error {
	path := filepath.Join(c.dir, name)
	if data == nil {
		return os.WriteFile(path, []byte{}, 0o644)
	}
	b, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal command data: %w", err)
	}
	return os.WriteFile(path, b, 0o644)
}

func sanitize(s string) string {
	out := make([]byte, 0, len(s))
	for i := range len(s) {
		c := s[i]
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' {
			out = append(out, c)
		} else {
			out = append(out, '_')
		}
	}
	return string(out)
}
