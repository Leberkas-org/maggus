package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWorkPIDPath(t *testing.T) {
	got := workPIDPath("/tmp/repo")
	want := filepath.Join("/tmp/repo", ".maggus", "work.pid")
	if got != want {
		t.Errorf("workPIDPath = %q, want %q", got, want)
	}
}

func TestWriteAndReadWorkPID(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".maggus"), 0755)

	// Write PID.
	if err := writeWorkPID(dir, 12345); err != nil {
		t.Fatalf("writeWorkPID: %v", err)
	}

	// Read it back.
	pid, err := readWorkPID(dir)
	if err != nil {
		t.Fatalf("readWorkPID: %v", err)
	}
	if pid != 12345 {
		t.Errorf("readWorkPID = %d, want 12345", pid)
	}
}

func TestReadWorkPID_NotExist(t *testing.T) {
	dir := t.TempDir()
	pid, err := readWorkPID(dir)
	if err != nil {
		t.Fatalf("readWorkPID: %v", err)
	}
	if pid != 0 {
		t.Errorf("readWorkPID = %d, want 0 for missing file", pid)
	}
}

func TestReadWorkPID_MalformedContent(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".maggus"), 0755)
	os.WriteFile(workPIDPath(dir), []byte("not-a-number\n"), 0644)

	pid, err := readWorkPID(dir)
	if err != nil {
		t.Fatalf("readWorkPID: %v", err)
	}
	if pid != 0 {
		t.Errorf("readWorkPID = %d, want 0 for malformed content", pid)
	}
}

func TestReadWorkPID_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".maggus"), 0755)
	os.WriteFile(workPIDPath(dir), []byte(""), 0644)

	pid, err := readWorkPID(dir)
	if err != nil {
		t.Fatalf("readWorkPID: %v", err)
	}
	if pid != 0 {
		t.Errorf("readWorkPID = %d, want 0 for empty file", pid)
	}
}

func TestRemoveWorkPID(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".maggus"), 0755)

	if err := writeWorkPID(dir, 99); err != nil {
		t.Fatalf("writeWorkPID: %v", err)
	}

	removeWorkPID(dir)

	// File should be gone.
	if _, err := os.Stat(workPIDPath(dir)); !os.IsNotExist(err) {
		t.Error("work.pid should be removed")
	}
}

func TestRemoveWorkPID_NoFile(t *testing.T) {
	// Should not panic or error when file doesn't exist.
	dir := t.TempDir()
	removeWorkPID(dir)
}

func TestWriteWorkPID_CreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	// .maggus dir does not exist yet.
	if err := writeWorkPID(dir, 42); err != nil {
		t.Fatalf("writeWorkPID should create directory: %v", err)
	}
	pid, err := readWorkPID(dir)
	if err != nil {
		t.Fatalf("readWorkPID: %v", err)
	}
	if pid != 42 {
		t.Errorf("readWorkPID = %d, want 42", pid)
	}
}
