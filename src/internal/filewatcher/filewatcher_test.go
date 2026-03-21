package filewatcher

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
)

func TestIsRelevantEvent(t *testing.T) {
	tests := []struct {
		name     string
		event    fsnotify.Event
		relevant bool
	}{
		{"feature write", fsnotify.Event{Name: "/a/feature_001.md", Op: fsnotify.Write}, true},
		{"feature create", fsnotify.Event{Name: "/a/feature_002.md", Op: fsnotify.Create}, true},
		{"feature remove", fsnotify.Event{Name: "/a/feature_003.md", Op: fsnotify.Remove}, true},
		{"feature rename", fsnotify.Event{Name: "/a/feature_004.md", Op: fsnotify.Rename}, true},
		{"bug write", fsnotify.Event{Name: "/b/bug_001.md", Op: fsnotify.Write}, true},
		{"bug create", fsnotify.Event{Name: "/b/bug_002.md", Op: fsnotify.Create}, true},
		{"completed feature", fsnotify.Event{Name: "/a/feature_001_completed.md", Op: fsnotify.Write}, true},
		{"non-md file", fsnotify.Event{Name: "/a/feature_001.txt", Op: fsnotify.Write}, false},
		{"random file", fsnotify.Event{Name: "/a/notes.md", Op: fsnotify.Write}, false},
		{"chmod only", fsnotify.Event{Name: "/a/feature_001.md", Op: fsnotify.Chmod}, false},
		{"no op", fsnotify.Event{Name: "/a/feature_001.md", Op: 0}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isRelevantEvent(tt.event); got != tt.relevant {
				t.Errorf("isRelevantEvent(%v) = %v, want %v", tt.event, got, tt.relevant)
			}
		})
	}
}

func TestWatcherMissingDirectories(t *testing.T) {
	baseDir := t.TempDir()

	w, err := New(baseDir, func(any) {}, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer w.Close()
}

func TestWatcherSendsUpdateOnFileChange(t *testing.T) {
	baseDir := t.TempDir()
	featDir := filepath.Join(baseDir, ".maggus", "features")
	if err := os.MkdirAll(featDir, 0o755); err != nil {
		t.Fatal(err)
	}

	var count atomic.Int32
	send := func(any) { count.Add(1) }

	w, err := New(baseDir, send, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer w.Close()

	file := filepath.Join(featDir, "feature_001.md")
	if err := os.WriteFile(file, []byte("# Feature 1"), 0o644); err != nil {
		t.Fatal(err)
	}

	time.Sleep(200 * time.Millisecond)

	if count.Load() < 1 {
		t.Errorf("expected at least 1 UpdateMsg, got %d", count.Load())
	}
}

func TestWatcherDebounce(t *testing.T) {
	baseDir := t.TempDir()
	featDir := filepath.Join(baseDir, ".maggus", "features")
	if err := os.MkdirAll(featDir, 0o755); err != nil {
		t.Fatal(err)
	}

	var count atomic.Int32
	send := func(any) { count.Add(1) }

	w, err := New(baseDir, send, 200*time.Millisecond)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer w.Close()

	file := filepath.Join(featDir, "feature_001.md")
	for range 5 {
		if err := os.WriteFile(file, []byte("update"), 0o644); err != nil {
			t.Fatal(err)
		}
		time.Sleep(20 * time.Millisecond)
	}

	// Wait for debounce to fire.
	time.Sleep(400 * time.Millisecond)

	c := count.Load()
	if c != 1 {
		t.Errorf("expected exactly 1 debounced UpdateMsg, got %d", c)
	}
}

func TestWatcherIgnoresIrrelevantFiles(t *testing.T) {
	baseDir := t.TempDir()
	featDir := filepath.Join(baseDir, ".maggus", "features")
	if err := os.MkdirAll(featDir, 0o755); err != nil {
		t.Fatal(err)
	}

	var count atomic.Int32
	send := func(any) { count.Add(1) }

	w, err := New(baseDir, send, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer w.Close()

	file := filepath.Join(featDir, "notes.txt")
	if err := os.WriteFile(file, []byte("irrelevant"), 0o644); err != nil {
		t.Fatal(err)
	}

	time.Sleep(200 * time.Millisecond)

	if count.Load() != 0 {
		t.Errorf("expected 0 UpdateMsg for irrelevant file, got %d", count.Load())
	}
}

func TestWatcherBugDirectory(t *testing.T) {
	baseDir := t.TempDir()
	bugDir := filepath.Join(baseDir, ".maggus", "bugs")
	if err := os.MkdirAll(bugDir, 0o755); err != nil {
		t.Fatal(err)
	}

	var count atomic.Int32
	send := func(any) { count.Add(1) }

	w, err := New(baseDir, send, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer w.Close()

	file := filepath.Join(bugDir, "bug_001.md")
	if err := os.WriteFile(file, []byte("# Bug 1"), 0o644); err != nil {
		t.Fatal(err)
	}

	time.Sleep(200 * time.Millisecond)

	if count.Load() < 1 {
		t.Errorf("expected at least 1 UpdateMsg for bug file, got %d", count.Load())
	}
}

func TestWatcherCloseNoLeak(t *testing.T) {
	baseDir := t.TempDir()
	featDir := filepath.Join(baseDir, ".maggus", "features")
	if err := os.MkdirAll(featDir, 0o755); err != nil {
		t.Fatal(err)
	}

	w, err := New(baseDir, func(any) {}, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	done := make(chan struct{})
	go func() {
		w.Close()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Close() did not return within 2 seconds — possible goroutine leak")
	}
}
