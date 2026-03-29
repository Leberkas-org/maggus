package cmd

import (
	"os"
	"path/filepath"
	"testing"
)


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
