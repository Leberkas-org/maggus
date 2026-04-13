package cmd

import (
	"context"
	"errors"
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

	reason, path := waitForChanges(fw, ctx, dir)

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
	reason, _ := waitForChanges(fw, ctx, dir)
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
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	reason, _ := waitForChanges(nil, ctx, dir)

	if reason != wakeSignal {
		t.Errorf("expected wakeSignal with nil watcher, got %v", reason)
	}
}

func TestWaitForChanges_StopAfterTask(t *testing.T) {
	dir := t.TempDir()
	maggusDir := filepath.Join(dir, ".maggus")
	if err := os.MkdirAll(maggusDir, 0o755); err != nil {
		t.Fatal(err)
	}

	fw, err := filewatcher.New(dir, nil, 500*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	defer fw.Close()

	ctx := context.Background()

	// Write stop-after-task sentinel file after a short delay.
	go func() {
		time.Sleep(100 * time.Millisecond)
		_ = os.WriteFile(daemonStopAfterTaskFilePath(dir), []byte{}, 0o644)
	}()

	start := time.Now()
	reason, _ := waitForChanges(fw, ctx, dir)
	elapsed := time.Since(start)

	if reason != wakeStopAfterTask {
		t.Errorf("expected wakeStopAfterTask, got %v", reason)
	}
	if elapsed > 2*time.Second {
		t.Errorf("took too long to detect sentinel file: %v", elapsed)
	}
}

func TestErrStopAfterTask_IsSentinel(t *testing.T) {
	// errStopAfterTask must be a distinct sentinel that wraps cleanly through errors.Is.
	if !errors.Is(errStopAfterTask, errStopAfterTask) {
		t.Fatal("errStopAfterTask should match itself via errors.Is")
	}
	if errors.Is(errStopAfterTask, context.Canceled) {
		t.Fatal("errStopAfterTask must not match context.Canceled")
	}
}

// ── Dispatch sentinel helpers ─────────────────────────────────────────────────

func TestDispatchSentinelPath(t *testing.T) {
	dir := "/some/repo"
	got := dispatchSentinelPath(dir, "TASK-007")
	want := filepath.Join(dir, ".maggus", "dispatch-TASK-007")
	if got != want {
		t.Errorf("dispatchSentinelPath = %q, want %q", got, want)
	}
}

func TestTaskIDFromDispatchSentinel(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{filepath.Join("/repo", ".maggus", "dispatch-TASK-007"), "TASK-007"},
		{filepath.Join("/repo", ".maggus", "dispatch-BUG-003-001"), "BUG-003-001"},
		{filepath.Join("/repo", ".maggus", "daemon.stop"), ""},    // non-dispatch
		{filepath.Join("/repo", ".maggus", "dispatch-"), ""},      // empty task ID
	}
	for _, tt := range tests {
		got := taskIDFromDispatchSentinel(tt.input)
		if got != tt.want {
			t.Errorf("taskIDFromDispatchSentinel(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestWriteAndGlobDispatchSentinels(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".maggus"), 0o755); err != nil {
		t.Fatal(err)
	}

	// No sentinels initially.
	if got := globDispatchSentinels(dir); len(got) != 0 {
		t.Errorf("expected 0 sentinels, got %d", len(got))
	}

	// Write two sentinels.
	if err := writeDispatchSentinel(dir, "TASK-001"); err != nil {
		t.Fatalf("writeDispatchSentinel: %v", err)
	}
	if err := writeDispatchSentinel(dir, "TASK-002"); err != nil {
		t.Fatalf("writeDispatchSentinel: %v", err)
	}

	sentinels := globDispatchSentinels(dir)
	if len(sentinels) != 2 {
		t.Errorf("expected 2 sentinels, got %d", len(sentinels))
	}

	// Verify task IDs can be extracted from the paths.
	ids := make(map[string]bool)
	for _, s := range sentinels {
		ids[taskIDFromDispatchSentinel(s)] = true
	}
	if !ids["TASK-001"] || !ids["TASK-002"] {
		t.Errorf("unexpected task IDs: %v", ids)
	}
}

func TestWaitForChanges_DispatchSentinel(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".maggus"), 0o755); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	start := time.Now()
	// Write a dispatch sentinel after a short delay.
	go func() {
		time.Sleep(100 * time.Millisecond)
		_ = writeDispatchSentinel(dir, "TASK-007")
	}()

	reason, _ := waitForChanges(nil, ctx, dir) // nil filewatcher → polling only
	elapsed := time.Since(start)

	if reason != wakeDispatch {
		t.Errorf("expected wakeDispatch, got %v", reason)
	}
	if elapsed > 2*time.Second {
		t.Errorf("took too long to detect dispatch sentinel: %v", elapsed)
	}
}
