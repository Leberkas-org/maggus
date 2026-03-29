// Package sesslock manages an interactive session lock file that coordinates
// Discord presence updates between the prompt command and the daemon.
//
// When the prompt command starts an interactive skill session, it acquires
// a session lock. The daemon checks this lock and skips presence updates
// while it is held, allowing the prompt command's presence to remain visible.
package sesslock

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	lockFile      = ".maggus/interactive.lock"
	staleDuration = 30 * time.Minute
)

// Lock represents an acquired interactive session lock.
type Lock struct {
	path string
}

// Acquire creates the interactive session lock file. If a stale lock exists
// (older than 30 minutes), it is replaced. Returns an error if a fresh lock
// already exists (another interactive session is active).
func Acquire(dir string) (Lock, error) {
	path := filepath.Join(dir, lockFile)

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return Lock{}, fmt.Errorf("create lock directory: %w", err)
	}

	// Remove stale lock.
	if info, err := os.Stat(path); err == nil {
		if time.Since(info.ModTime()) > staleDuration {
			_ = os.Remove(path)
		}
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		if os.IsExist(err) {
			return Lock{}, fmt.Errorf("interactive session already active")
		}
		return Lock{}, fmt.Errorf("acquire session lock: %w", err)
	}

	content := fmt.Sprintf("pid: %d\ntimestamp: %s\n", os.Getpid(), time.Now().UTC().Format(time.RFC3339))
	_, _ = f.WriteString(content)
	_ = f.Close()

	return Lock{path: path}, nil
}

// Release removes the session lock file.
func (l Lock) Release() error {
	if l.path == "" {
		return nil
	}
	if err := os.Remove(l.path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("release session lock: %w", err)
	}
	return nil
}

// IsActive returns true if an interactive session lock exists and is not stale.
func IsActive(dir string) bool {
	path := filepath.Join(dir, lockFile)
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return time.Since(info.ModTime()) <= staleDuration
}
