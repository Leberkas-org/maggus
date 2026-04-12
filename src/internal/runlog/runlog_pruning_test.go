package runlog_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/leberkas-org/maggus/internal/runlog"
)

func TestPruning_RemovesOldestFilesInFeatureDir(t *testing.T) {
	dir := t.TempDir()
	maggusID := "prune-guid-001"
	featureDir := filepath.Join(dir, ".maggus", "logs", maggusID)
	if err := os.MkdirAll(featureDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Pre-create 5 old log files in the feature directory.
	for i := 0; i < 5; i++ {
		name := fmt.Sprintf("%d.log", 1000+i)
		os.WriteFile(filepath.Join(featureDir, name), []byte("{}"), 0644)
	}

	// Open and SetCurrentMaggusID with maxFiles=3 triggers pruning.
	l, err := runlog.Open(dir, 3)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	l.SetCurrentMaggusID(maggusID)
	defer l.Close()

	entries, err := os.ReadDir(featureDir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	var logs []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".log" {
			logs = append(logs, e.Name())
		}
	}

	// 5 old + 1 new = 6, pruned to 3.
	if len(logs) != 3 {
		t.Errorf("expected 3 log files after pruning, got %d: %v", len(logs), logs)
	}
	// The oldest files (1000.log, 1001.log, 1002.log) should be removed.
	for _, name := range logs {
		if name == "1000.log" || name == "1001.log" || name == "1002.log" {
			t.Errorf("old file %q should have been pruned but still exists", name)
		}
	}
}

func TestPruning_NoPruneWhenUnderLimit(t *testing.T) {
	dir := t.TempDir()
	maggusID := "prune-guid-002"
	featureDir := filepath.Join(dir, ".maggus", "logs", maggusID)
	if err := os.MkdirAll(featureDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Pre-create 2 old log files.
	for i := 0; i < 2; i++ {
		name := fmt.Sprintf("%d.log", 2000+i)
		os.WriteFile(filepath.Join(featureDir, name), []byte("{}"), 0644)
	}

	// Open with maxFiles=10: 3 total (2 old + 1 new), well under limit.
	l, err := runlog.Open(dir, 10)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	l.SetCurrentMaggusID(maggusID)
	defer l.Close()

	entries, _ := os.ReadDir(featureDir)
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

func TestPruning_OnlyAffectsCurrentFeatureDir(t *testing.T) {
	dir := t.TempDir()

	// Create two feature directories with log files.
	for _, guid := range []string{"guid-aaa", "guid-bbb"} {
		fdir := filepath.Join(dir, ".maggus", "logs", guid)
		os.MkdirAll(fdir, 0755)
		for i := 0; i < 5; i++ {
			name := fmt.Sprintf("%d.log", 3000+i)
			os.WriteFile(filepath.Join(fdir, name), []byte("{}"), 0644)
		}
	}

	// Open with maxFiles=2 and set to guid-aaa.
	l, err := runlog.Open(dir, 2)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	l.SetCurrentMaggusID("guid-aaa")
	defer l.Close()

	// guid-aaa should be pruned to 2 files.
	entriesA, _ := os.ReadDir(filepath.Join(dir, ".maggus", "logs", "guid-aaa"))
	var logsA int
	for _, e := range entriesA {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".log" {
			logsA++
		}
	}
	if logsA != 2 {
		t.Errorf("guid-aaa: expected 2 log files after pruning, got %d", logsA)
	}

	// guid-bbb should be untouched (5 files).
	entriesB, _ := os.ReadDir(filepath.Join(dir, ".maggus", "logs", "guid-bbb"))
	var logsB int
	for _, e := range entriesB {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".log" {
			logsB++
		}
	}
	if logsB != 5 {
		t.Errorf("guid-bbb: expected 5 log files (untouched), got %d", logsB)
	}
}

func TestPruning_DisabledWhenMaxFilesZero(t *testing.T) {
	dir := t.TempDir()
	maggusID := "prune-guid-003"
	featureDir := filepath.Join(dir, ".maggus", "logs", maggusID)
	os.MkdirAll(featureDir, 0755)

	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("%d.log", 4000+i)
		os.WriteFile(filepath.Join(featureDir, name), []byte("{}"), 0644)
	}

	l, err := runlog.Open(dir, 0)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	l.SetCurrentMaggusID(maggusID)
	defer l.Close()

	entries, _ := os.ReadDir(featureDir)
	var logs int
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".log" {
			logs++
		}
	}
	// 10 old + 1 new = 11, none pruned.
	if logs != 11 {
		t.Errorf("expected 11 log files (no pruning with maxFiles=0), got %d", logs)
	}
}
