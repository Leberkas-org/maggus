package cmd

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWaitForChanges_FileChange(t *testing.T) {
	dir := t.TempDir()
	featDir := filepath.Join(dir, ".maggus", "features")
	if err := os.MkdirAll(featDir, 0o755); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	// Write a feature file after a short delay to trigger the watcher.
	go func() {
		time.Sleep(100 * time.Millisecond)
		_ = os.WriteFile(filepath.Join(featDir, "feature_099.md"), []byte("# Test"), 0o644)
	}()

	reason, path := waitForChanges(dir, ctx)

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

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel after a short delay.
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	reason, _ := waitForChanges(dir, ctx)
	elapsed := time.Since(start)

	if reason != wakeSignal {
		t.Errorf("expected wakeSignal, got %v", reason)
	}
	if elapsed > 2*time.Second {
		t.Errorf("took too long to respond to cancel: %v", elapsed)
	}
}

func TestWaitForChanges_NoFeaturesDir(t *testing.T) {
	dir := t.TempDir()
	// Don't create .maggus/features — watcher creation fails, so waitForChanges
	// blocks on context only; cancel to verify clean shutdown.

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	reason, _ := waitForChanges(dir, ctx)

	if reason != wakeSignal {
		t.Errorf("expected wakeSignal with missing features dir, got %v", reason)
	}
}
