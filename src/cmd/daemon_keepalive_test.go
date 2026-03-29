package cmd

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/leberkas-org/maggus/internal/filewatcher"
)

func TestWaitForChanges_FileChange(t *testing.T) {
	dir := t.TempDir()
	featDir := filepath.Join(dir, ".maggus", "features")
	if err := os.MkdirAll(featDir, 0o755); err != nil {
		t.Fatal(err)
	}

	fw, err := filewatcher.New(dir, nil, 500*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	defer fw.Close()

	ctx := context.Background()

	// Write a feature file after a short delay to trigger the watcher.
	go func() {
		time.Sleep(100 * time.Millisecond)
		_ = os.WriteFile(filepath.Join(featDir, "feature_099.md"), []byte("# Test"), 0o644)
	}()

	reason, path := waitForChanges(fw, ctx)

	if reason != wakeFileChange {
		t.Errorf("expected wakeFileChange, got %v", reason)
	}
	if path == "" {
		t.Error("expected non-empty path on file change")
	}
}

func TestWaitForChanges_ContextCancel(t *testing.T) {
	dir := t.TempDir()
	featDir := filepath.Join(dir, ".maggus", "features")
	if err := os.MkdirAll(featDir, 0o755); err != nil {
		t.Fatal(err)
	}

	fw, err := filewatcher.New(dir, nil, 500*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	defer fw.Close()

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel after a short delay.
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	reason, _ := waitForChanges(fw, ctx)
	elapsed := time.Since(start)

	if reason != wakeSignal {
		t.Errorf("expected wakeSignal, got %v", reason)
	}
	if elapsed > 2*time.Second {
		t.Errorf("took too long to respond to cancel: %v", elapsed)
	}
}

func TestWaitForChanges_NilWatcher(t *testing.T) {
	// When watcher is nil, waitForChanges blocks on context only.
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	reason, _ := waitForChanges(nil, ctx)

	if reason != wakeSignal {
		t.Errorf("expected wakeSignal with nil watcher, got %v", reason)
	}
}
