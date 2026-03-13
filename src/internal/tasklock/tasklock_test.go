package tasklock

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAcquireAndRelease(t *testing.T) {
	dir := t.TempDir()

	lock, err := Acquire(dir, "TASK-001", "run-123")
	if err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}

	// Lock file should exist.
	path := filepath.Join(dir, lockDir, "TASK-001.lock")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("lock file does not exist: %v", err)
	}

	// Lock file should contain run ID and timestamp.
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read lock file: %v", err)
	}
	if got := string(content); len(got) == 0 {
		t.Fatal("lock file is empty")
	}

	// Release should remove the file.
	if err := lock.Release(); err != nil {
		t.Fatalf("Release failed: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("lock file still exists after release")
	}
}

func TestDoubleAcquireFails(t *testing.T) {
	dir := t.TempDir()

	lock1, err := Acquire(dir, "TASK-002", "run-aaa")
	if err != nil {
		t.Fatalf("first Acquire failed: %v", err)
	}
	defer lock1.Release()

	_, err = Acquire(dir, "TASK-002", "run-bbb")
	if err == nil {
		t.Fatal("second Acquire should have failed but didn't")
	}
}

func TestStaleLockOverride(t *testing.T) {
	dir := t.TempDir()

	// Create a lock file manually and backdate it.
	locksDir := filepath.Join(dir, lockDir)
	if err := os.MkdirAll(locksDir, 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(locksDir, "TASK-003.lock")
	if err := os.WriteFile(path, []byte("stale"), 0644); err != nil {
		t.Fatal(err)
	}
	// Set mod time to 3 hours ago.
	staleTime := time.Now().Add(-3 * time.Hour)
	if err := os.Chtimes(path, staleTime, staleTime); err != nil {
		t.Fatal(err)
	}

	// Acquire should succeed because the lock is stale.
	lock, err := Acquire(dir, "TASK-003", "run-new")
	if err != nil {
		t.Fatalf("Acquire on stale lock failed: %v", err)
	}
	defer lock.Release()
}

func TestIsLocked(t *testing.T) {
	dir := t.TempDir()

	if IsLocked(dir, "TASK-004") {
		t.Fatal("should not be locked before acquire")
	}

	lock, err := Acquire(dir, "TASK-004", "run-xyz")
	if err != nil {
		t.Fatal(err)
	}

	if !IsLocked(dir, "TASK-004") {
		t.Fatal("should be locked after acquire")
	}

	lock.Release()
	if IsLocked(dir, "TASK-004") {
		t.Fatal("should not be locked after release")
	}
}

func TestIsLockedStaleLock(t *testing.T) {
	dir := t.TempDir()

	// Create a stale lock file.
	locksDir := filepath.Join(dir, lockDir)
	os.MkdirAll(locksDir, 0755)
	path := filepath.Join(locksDir, "TASK-005.lock")
	os.WriteFile(path, []byte("stale"), 0644)
	staleTime := time.Now().Add(-3 * time.Hour)
	os.Chtimes(path, staleTime, staleTime)

	if IsLocked(dir, "TASK-005") {
		t.Fatal("stale lock should not be considered locked")
	}
}

func TestReleaseEmptyPath(t *testing.T) {
	lock := Lock{}
	if err := lock.Release(); err != nil {
		t.Fatalf("releasing empty lock should not error: %v", err)
	}
}
