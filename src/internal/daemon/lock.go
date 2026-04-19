package daemon

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/leberkas-org/maggus/internal/config"
)

type LockFile struct {
	file *os.File
}

func AcquireLock() (*LockFile, error) {
	globalDir, err := config.GlobalDir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(globalDir, 0o755); err != nil {
		return nil, err
	}

	path := filepath.Join(globalDir, "daemon.lock")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open lock: %w", err)
	}

	if err := lockFile(f); err != nil {
		f.Close()
		return nil, fmt.Errorf("daemon already running")
	}

	// Write PID for informational purposes
	f.Truncate(0)
	f.Seek(0, 0)
	fmt.Fprintf(f, "%d", os.Getpid())
	f.Sync()

	return &LockFile{file: f}, nil
}

func (l *LockFile) Release() {
	if l.file != nil {
		unlockFile(l.file)
		l.file.Close()
		os.Remove(l.file.Name())
	}
}

func IsLocked() bool {
	globalDir, err := config.GlobalDir()
	if err != nil {
		return false
	}
	path := filepath.Join(globalDir, "daemon.lock")
	f, err := os.OpenFile(path, os.O_WRONLY, 0o644)
	if err != nil {
		return false
	}
	defer f.Close()

	err = lockFileTry(f)
	if err != nil {
		return true // couldn't lock = someone else holds it
	}
	unlockFile(f)
	return false
}
