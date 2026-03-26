package runlog_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leberkas-org/maggus/internal/runlog"
)

func TestPruning_RemovesOldestFiles(t *testing.T) {
	dir := t.TempDir()
	runsDir := filepath.Join(dir, ".maggus", "runs")
	if err := os.MkdirAll(runsDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Pre-create 5 old log files with ascending timestamps.
	oldFiles := []string{
		"20260101-100000_uuid1.log",
		"20260101-110000_uuid2.log",
		"20260101-120000_uuid3.log",
		"20260101-130000_uuid4.log",
		"20260101-140000_uuid5.log",
	}
	for _, name := range oldFiles {
		os.WriteFile(filepath.Join(runsDir, name), []byte("{}"), 0644)
	}

	// Open with maxFiles=5: opening creates a 6th file, so oldest is pruned.
	l, err := runlog.Open("new-uuid", dir, 5)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer l.Close()

	entries, err := os.ReadDir(runsDir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	var logs []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".log" {
			logs = append(logs, e.Name())
		}
	}

	if len(logs) != 5 {
		t.Errorf("expected 5 log files after pruning, got %d: %v", len(logs), logs)
	}
	// The oldest file should have been removed.
	for _, name := range logs {
		if name == oldFiles[0] {
			t.Errorf("oldest file %q should have been pruned but still exists", oldFiles[0])
		}
	}
}

func TestPruning_DaemonLogNeverPruned(t *testing.T) {
	dir := t.TempDir()
	runsDir := filepath.Join(dir, ".maggus", "runs")
	if err := os.MkdirAll(runsDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Create daemon.log and enough old files to trigger pruning.
	os.WriteFile(filepath.Join(runsDir, "daemon.log"), []byte("daemon"), 0644)
	for i := 0; i < 5; i++ {
		name := strings.Replace("20260101-1X0000_uuid.log", "X", string(rune('0'+i)), 1)
		os.WriteFile(filepath.Join(runsDir, name), []byte("{}"), 0644)
	}

	// Open with maxFiles=3 — will prune task logs but never daemon.log.
	l, err := runlog.Open("new-uuid", dir, 3)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer l.Close()

	// daemon.log must still exist.
	if _, err := os.Stat(filepath.Join(runsDir, "daemon.log")); err != nil {
		t.Errorf("daemon.log was pruned: %v", err)
	}
}

func TestPruning_NoPruneWhenUnderLimit(t *testing.T) {
	dir := t.TempDir()
	runsDir := filepath.Join(dir, ".maggus", "runs")
	if err := os.MkdirAll(runsDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Pre-create 2 old log files.
	for _, name := range []string{"20260101-100000_a.log", "20260101-110000_b.log"} {
		os.WriteFile(filepath.Join(runsDir, name), []byte("{}"), 0644)
	}

	// Open with maxFiles=10: 3 total files, well under limit — nothing pruned.
	l, err := runlog.Open("new", dir, 10)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer l.Close()

	entries, _ := os.ReadDir(runsDir)
	var logs []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".log" {
			logs = append(logs, e.Name())
		}
	}
	if len(logs) != 3 {
		t.Errorf("expected 3 log files (no pruning), got %d", len(logs))
	}
}
