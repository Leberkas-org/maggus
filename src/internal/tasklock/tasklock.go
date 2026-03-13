package tasklock

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	lockDir       = ".maggus/locks"
	staleDuration = 2 * time.Hour
)

// Lock represents an acquired task lock.
type Lock struct {
	path string
}

// Acquire creates a lock file for the given task ID. The lock file is created
// atomically using O_CREATE|O_EXCL so that concurrent callers will fail if the
// lock already exists. If an existing lock file is older than 2 hours it is
// considered stale and is overwritten.
//
// dir is the repository root directory. taskID is the task identifier
// (e.g. "TASK-001"). runID is written into the lock file for diagnostics.
func Acquire(dir, taskID, runID string) (Lock, error) {
	locksDir := filepath.Join(dir, lockDir)
	if err := os.MkdirAll(locksDir, 0755); err != nil {
		return Lock{}, fmt.Errorf("create locks directory: %w", err)
	}

	path := filepath.Join(locksDir, taskID+".lock")

	// Check for stale lock and remove it.
	if info, err := os.Stat(path); err == nil {
		if time.Since(info.ModTime()) > staleDuration {
			_ = os.Remove(path)
		}
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		if os.IsExist(err) {
			return Lock{}, fmt.Errorf("task %s is already locked", taskID)
		}
		return Lock{}, fmt.Errorf("acquire lock for %s: %w", taskID, err)
	}

	content := fmt.Sprintf("run_id: %s\ntimestamp: %s\n", runID, time.Now().UTC().Format(time.RFC3339))
	_, _ = f.WriteString(content)
	_ = f.Close()

	return Lock{path: path}, nil
}

// Release removes the lock file, freeing the task for other sessions.
func (l Lock) Release() error {
	if l.path == "" {
		return nil
	}
	if err := os.Remove(l.path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("release lock: %w", err)
	}
	return nil
}

// CleanAll removes all lock files in the locks directory.
func CleanAll(dir string) error {
	locksDir := filepath.Join(dir, lockDir)
	entries, err := os.ReadDir(locksDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read locks directory: %w", err)
	}
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".lock" {
			os.Remove(filepath.Join(locksDir, e.Name()))
		}
	}
	return nil
}

// AllStale returns true if all lock files in the locks directory are stale
// (older than 2 hours), or if there are no lock files.
func AllStale(dir string) bool {
	locksDir := filepath.Join(dir, lockDir)
	entries, err := os.ReadDir(locksDir)
	if err != nil {
		return true
	}
	for _, e := range entries {
		if filepath.Ext(e.Name()) != ".lock" {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if time.Since(info.ModTime()) <= staleDuration {
			return false
		}
	}
	return true
}

// IsLocked checks whether a lock file exists for the given task ID.
// Stale locks (older than 2 hours) are not considered locked.
func IsLocked(dir, taskID string) bool {
	path := filepath.Join(dir, lockDir, taskID+".lock")
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	// Stale locks are not considered locked.
	if time.Since(info.ModTime()) > staleDuration {
		return false
	}
	return true
}
