package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStaleDaemonPID_CleanedUp(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".maggus"), 0755)

	// Write a PID that doesn't correspond to a running process.
	// Use a very high PID that's almost certainly not running.
	writeWorkPID(dir, 9999999)

	pid, _ := readWorkPID(dir)
	if pid == 0 {
		t.Fatal("expected non-zero PID from file")
	}

	// Stale: process is not running.
	if isProcessRunning(pid) {
		t.Skip("PID 9999999 is somehow running, skipping")
	}

	// After detecting stale, we should clean up.
	removeWorkPID(dir)
	pidAfter, _ := readWorkPID(dir)
	if pidAfter != 0 {
		t.Error("stale work.pid should have been removed")
	}
}

func TestLiveProcessDetected(t *testing.T) {
	// Our own process should be detected as running.
	if !isProcessRunning(os.Getpid()) {
		t.Error("isProcessRunning should return true for our own PID")
	}
}

func TestDeadProcessNotDetected(t *testing.T) {
	// A very high PID should not be running.
	if isProcessRunning(9999999) {
		t.Skip("PID 9999999 is somehow running, skipping")
	}
}

func TestDaemonPIDHelpers(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".maggus"), 0755)

	// Verify daemon PID path.
	got := daemonPIDPath(dir)
	want := filepath.Join(dir, ".maggus", "daemon.pid")
	if got != want {
		t.Errorf("daemonPIDPath = %q, want %q", got, want)
	}

	// Write and read back.
	if err := writeDaemonPID(dir, 54321); err != nil {
		t.Fatalf("writeDaemonPID: %v", err)
	}
	pid, err := readDaemonPID(dir)
	if err != nil {
		t.Fatalf("readDaemonPID: %v", err)
	}
	if pid != 54321 {
		t.Errorf("readDaemonPID = %d, want 54321", pid)
	}

	// Remove.
	removeDaemonPID(dir)
	pid, _ = readDaemonPID(dir)
	if pid != 0 {
		t.Error("daemon.pid should be gone after removal")
	}
}
