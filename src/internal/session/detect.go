package session

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ProjectHash computes the Claude project hash for a given directory path.
// Claude replaces all path separators and colons with dashes.
// For example, "C:\c\maggus" becomes "C--c-maggus" on Windows,
// and "/home/user/project" becomes "-home-user-project" on Unix.
func ProjectHash(dir string) string {
	// Clean and resolve to absolute path format
	dir = filepath.Clean(dir)

	// Replace colons (Windows drive letters) with dashes
	dir = strings.ReplaceAll(dir, ":", "-")

	// Replace both forward and back slashes with dashes
	dir = strings.ReplaceAll(dir, `\`, "-")
	dir = strings.ReplaceAll(dir, "/", "-")

	return dir
}

// SessionDir returns the path to the Claude projects session directory
// for the given working directory.
func SessionDir(workDir string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get user home dir: %w", err)
	}

	hash := ProjectHash(workDir)
	dir := filepath.Join(home, ".claude", "projects", hash)
	return dir, nil
}

// SnapshotDir reads the current set of .jsonl files in a directory
// and returns them as a set (map of filename → true).
func SnapshotDir(dir string) (map[string]bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]bool), nil
		}
		return nil, fmt.Errorf("read session dir: %w", err)
	}

	files := make(map[string]bool, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(e.Name(), ".jsonl") {
			files[e.Name()] = true
		}
	}
	return files, nil
}

// DetectNewSessions compares a before-snapshot with the current state of the
// directory and returns the full paths of any new .jsonl files that appeared.
func DetectNewSessions(dir string, before map[string]bool) ([]string, error) {
	after, err := SnapshotDir(dir)
	if err != nil {
		return nil, err
	}

	var newFiles []string
	for name := range after {
		if !before[name] {
			newFiles = append(newFiles, filepath.Join(dir, name))
		}
	}
	return newFiles, nil
}

// DetectSessionFile is a convenience function that performs the full detection
// workflow: resolves the session directory for the current working directory,
// diffs against a before-snapshot, and returns the path to the new session file.
// If multiple new files are found, it returns the first one.
// If no new files are found, it returns an empty string and no error.
func DetectSessionFile(workDir string, before map[string]bool) (string, error) {
	sessionDir, err := SessionDir(workDir)
	if err != nil {
		return "", err
	}

	newFiles, err := DetectNewSessions(sessionDir, before)
	if err != nil {
		return "", err
	}

	if len(newFiles) == 0 {
		return "", nil
	}

	return newFiles[0], nil
}

