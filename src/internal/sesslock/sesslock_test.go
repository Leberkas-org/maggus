package sesslock

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAcquireAndRelease(t *testing.T) {
	dir := t.TempDir()

	lock, err := Acquire(dir)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}

	if !IsActive(dir) {
		t.Fatal("expected IsActive to be true after Acquire")
	}

	if err := lock.Release(); err != nil {
		t.Fatalf("Release: %v", err)
	}

	if IsActive(dir) {
		t.Fatal("expected IsActive to be false after Release")
	}
}

func TestAcquireDoubleAcquireFails(t *testing.T) {
	dir := t.TempDir()

	lock, err := Acquire(dir)
	if err != nil {
		t.Fatalf("first Acquire: %v", err)
	}
	defer lock.Release()

	_, err = Acquire(dir)
	if err == nil {
		t.Fatal("expected error on second Acquire")
	}
}

func TestAcquireStaleLockReplaced(t *testing.T) {
	dir := t.TempDir()

	// Create a stale lock manually.
	path := filepath.Join(dir, lockFile)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("stale"), 0644); err != nil {
		t.Fatal(err)
	}
	// Set modification time to 31 minutes ago.
	staleTime := time.Now().Add(-31 * time.Minute)
	if err := os.Chtimes(path, staleTime, staleTime); err != nil {
		t.Fatal(err)
	}

	lock, err := Acquire(dir)
	if err != nil {
		t.Fatalf("Acquire over stale lock: %v", err)
	}
	defer lock.Release()

	if !IsActive(dir) {
		t.Fatal("expected IsActive to be true")
	}
}

func TestIsActiveNoLockFile(t *testing.T) {
	dir := t.TempDir()
	if IsActive(dir) {
		t.Fatal("expected IsActive to be false with no lock file")
	}
}

func TestReleaseIdempotent(t *testing.T) {
	dir := t.TempDir()

	lock, err := Acquire(dir)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}

	if err := lock.Release(); err != nil {
		t.Fatalf("first Release: %v", err)
	}
	if err := lock.Release(); err != nil {
		t.Fatalf("second Release: %v", err)
	}
}

func TestReleaseEmptyLock(t *testing.T) {
	lock := Lock{}
	if err := lock.Release(); err != nil {
		t.Fatalf("Release on empty lock: %v", err)
	}
}
