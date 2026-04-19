package bryan

import (
	"os"
	"path/filepath"
	"strings"
)

func ReadMemory(repoPath string) (string, error) {
	path := filepath.Join(repoPath, ".maggus", "MEMORY.md")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func WriteMemory(repoPath string, content string) error {
	dir := filepath.Join(repoPath, ".maggus")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "MEMORY.md"), []byte(content), 0o644)
}
