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
		{"approval write", fsnotify.Event{Name: "/a/feature_approvals.yml", Op: fsnotify.Write}, true},
		{"approval create", fsnotify.Event{Name: "/a/feature_approvals.yml", Op: fsnotify.Create}, true},
		{"approval chmod", fsnotify.Event{Name: "/a/feature_approvals.yml", Op: fsnotify.Chmod}, false},
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
	var lastMsg atomic.Value
	send := func(msg any) {
		count.Add(1)
		lastMsg.Store(msg)
	}

	w, err := New(baseDir, send, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer w.Close()

	// Creating a new file should set HasNewFile = true.
	file := filepath.Join(featDir, "feature_001.md")
	if err := os.WriteFile(file, []byte("# Feature 1"), 0o644); err != nil {
		t.Fatal(err)
	}

	time.Sleep(200 * time.Millisecond)

	if count.Load() < 1 {
		t.Errorf("expected at least 1 UpdateMsg, got %d", count.Load())
	}
	if msg, ok := lastMsg.Load().(UpdateMsg); ok {
		if !msg.HasNewFile {
			t.Error("expected HasNewFile = true when creating a new file")
		}
	}
}

func TestWatcherDebounce(t *testing.T) {
	baseDir := t.TempDir()
	featDir := filepath.Join(baseDir, ".maggus", "features")
	if err := os.MkdirAll(featDir, 0o755); err != nil {
		t.Fatal(err)
	}

	var count atomic.Int32
	var lastMsg atomic.Value
	send := func(msg any) {
		count.Add(1)
		lastMsg.Store(msg)
	}

	w, err := New(baseDir, send, 200*time.Millisecond)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer w.Close()

	// First write creates the file (Create event), subsequent writes update it.
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
	// The first write creates a new file, so HasNewFile must be true.
	if msg, ok := lastMsg.Load().(UpdateMsg); ok {
		if !msg.HasNewFile {
			t.Error("expected HasNewFile = true when window includes a Create event")
		}
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
	var lastMsg atomic.Value
	send := func(msg any) {
		count.Add(1)
		lastMsg.Store(msg)
	}

	w, err := New(baseDir, send, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer w.Close()

	// Creating a new bug file should set HasNewFile = true.
	file := filepath.Join(bugDir, "bug_001.md")
	if err := os.WriteFile(file, []byte("# Bug 1"), 0o644); err != nil {
		t.Fatal(err)
	}

	time.Sleep(200 * time.Millisecond)

	if count.Load() < 1 {
		t.Errorf("expected at least 1 UpdateMsg for bug file, got %d", count.Load())
	}
	if msg, ok := lastMsg.Load().(UpdateMsg); ok {
		if !msg.HasNewFile {
			t.Error("expected HasNewFile = true when creating a new bug file")
		}
	}
}

func TestWatcherRecoversAfterInternalWatcherClosed(t *testing.T) {
	baseDir := t.TempDir()
	featDir := filepath.Join(baseDir, ".maggus", "features")
	if err := os.MkdirAll(featDir, 0o755); err != nil {
		t.Fatal(err)
	}

	var count atomic.Int32
	var lastMsg atomic.Value
	send := func(msg any) {
		count.Add(1)
		lastMsg.Store(msg)
	}

	w, err := New(baseDir, send, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer w.Close()

	// Simulate the OS handle going stale by closing the internal fsnotify.Watcher.
	w.fsw.Close()

	// Allow time for the watcher to detect the closure and reconnect.
	time.Sleep(500 * time.Millisecond)

	// Create a new file — the recovered watcher must detect this.
	file := filepath.Join(featDir, "feature_001.md")
	if err := os.WriteFile(file, []byte("# Feature 1"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Wait for debounce to fire.
	time.Sleep(200 * time.Millisecond)

	if count.Load() < 1 {
		t.Errorf("expected at least 1 UpdateMsg after recovery, got %d", count.Load())
	}
	if msg, ok := lastMsg.Load().(UpdateMsg); ok {
		if !msg.HasNewFile {
			t.Error("expected HasNewFile = true when creating a new file after recovery")
		}
	}
}

func TestWatcherWriteOnlyEventsHasNewFileFalse(t *testing.T) {
	baseDir := t.TempDir()
	featDir := filepath.Join(baseDir, ".maggus", "features")
	if err := os.MkdirAll(featDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Pre-create the file so subsequent writes are Write events, not Create events.
	file := filepath.Join(featDir, "feature_001.md")
	if err := os.WriteFile(file, []byte("initial"), 0o644); err != nil {
		t.Fatal(err)
	}

	var lastMsg atomic.Value
	send := func(msg any) { lastMsg.Store(msg) }

	w, err := New(baseDir, send, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer w.Close()

	// Wait for the watcher to settle after setup.
	time.Sleep(200 * time.Millisecond)
	lastMsg.Store(UpdateMsg{}) // reset

	// Write to the existing file — should produce only Write events.
	if err := os.WriteFile(file, []byte("updated content"), 0o644); err != nil {
		t.Fatal(err)
	}

	time.Sleep(200 * time.Millisecond)

	if msg, ok := lastMsg.Load().(UpdateMsg); ok {
		if msg.HasNewFile {
			t.Error("expected HasNewFile = false for pure Write event on existing file")
		}
	}
}

func TestWatcherCreateEventHasNewFileTrue(t *testing.T) {
	baseDir := t.TempDir()
	featDir := filepath.Join(baseDir, ".maggus", "features")
	if err := os.MkdirAll(featDir, 0o755); err != nil {
		t.Fatal(err)
	}

	var lastMsg atomic.Value
	send := func(msg any) { lastMsg.Store(msg) }

	w, err := New(baseDir, send, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer w.Close()

	// Create a brand-new file — should produce a Create event.
	file := filepath.Join(featDir, "feature_002.md")
	if err := os.WriteFile(file, []byte("# New Feature"), 0o644); err != nil {
		t.Fatal(err)
	}

	time.Sleep(200 * time.Millisecond)

	if msg, ok := lastMsg.Load().(UpdateMsg); ok {
		if !msg.HasNewFile {
			t.Error("expected HasNewFile = true when a new file is created")
		}
	} else {
		t.Error("expected an UpdateMsg to be sent")
	}
}

func TestWatcherUpdateMsgIncludesPath(t *testing.T) {
	baseDir := t.TempDir()
	featDir := filepath.Join(baseDir, ".maggus", "features")
	if err := os.MkdirAll(featDir, 0o755); err != nil {
		t.Fatal(err)
	}

	var lastMsg atomic.Value
	send := func(msg any) { lastMsg.Store(msg) }

	w, err := New(baseDir, send, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer w.Close()

	file := filepath.Join(featDir, "feature_042.md")
	if err := os.WriteFile(file, []byte("# Feature 42"), 0o644); err != nil {
		t.Fatal(err)
	}

	time.Sleep(200 * time.Millisecond)

	msg, ok := lastMsg.Load().(UpdateMsg)
	if !ok {
		t.Fatal("expected an UpdateMsg to be sent")
	}
	if msg.Path == "" {
		t.Error("expected UpdateMsg.Path to be non-empty")
	}
	base := filepath.Base(msg.Path)
	if base != "feature_042.md" {
		t.Errorf("expected Path to end with feature_042.md, got %q", msg.Path)
	}
}

func TestWatcherApprovalFileWakesDaemon(t *testing.T) {
	baseDir := t.TempDir()
	maggusDir := filepath.Join(baseDir, ".maggus")
	if err := os.MkdirAll(maggusDir, 0o755); err != nil {
		t.Fatal(err)
	}

	var count atomic.Int32
	var lastMsg atomic.Value
	send := func(msg any) {
		count.Add(1)
		lastMsg.Store(msg)
	}

	w, err := New(baseDir, send, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer w.Close()

	// Writing the approval file should trigger an UpdateMsg.
	file := filepath.Join(maggusDir, "feature_approvals.yml")
	if err := os.WriteFile(file, []byte("feature_001: true"), 0o644); err != nil {
		t.Fatal(err)
	}

	time.Sleep(200 * time.Millisecond)

	if count.Load() < 1 {
		t.Errorf("expected at least 1 UpdateMsg for approval file, got %d", count.Load())
	}
	if msg, ok := lastMsg.Load().(UpdateMsg); ok {
		if msg.Path == "" {
			t.Error("expected UpdateMsg.Path to be non-empty")
		}
		if filepath.Base(msg.Path) != "feature_approvals.yml" {
			t.Errorf("expected Path to end with feature_approvals.yml, got %q", msg.Path)
		}
	}
}

func TestWatcherApprovalDebounce(t *testing.T) {
	baseDir := t.TempDir()
	maggusDir := filepath.Join(baseDir, ".maggus")
	if err := os.MkdirAll(maggusDir, 0o755); err != nil {
		t.Fatal(err)
	}

	var count atomic.Int32
	send := func(msg any) { count.Add(1) }

	w, err := New(baseDir, send, 200*time.Millisecond)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer w.Close()

	// Rapid writes to the approval file should be debounced into one UpdateMsg.
	file := filepath.Join(maggusDir, "feature_approvals.yml")
	for range 5 {
		if err := os.WriteFile(file, []byte("update"), 0o644); err != nil {
			t.Fatal(err)
		}
		time.Sleep(20 * time.Millisecond)
	}

	time.Sleep(400 * time.Millisecond)

	c := count.Load()
	if c != 1 {
		t.Errorf("expected exactly 1 debounced UpdateMsg for approval file, got %d", c)
	}
}

func TestWatcherApprovalDoesNotBreakFeatureWatch(t *testing.T) {
	baseDir := t.TempDir()
	maggusDir := filepath.Join(baseDir, ".maggus")
	featDir := filepath.Join(maggusDir, "features")
	bugDir := filepath.Join(maggusDir, "bugs")
	if err := os.MkdirAll(featDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(bugDir, 0o755); err != nil {
		t.Fatal(err)
	}

	var count atomic.Int32
	send := func(msg any) { count.Add(1) }

	w, err := New(baseDir, send, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer w.Close()

	// Feature file should still trigger.
	feat := filepath.Join(featDir, "feature_001.md")
	if err := os.WriteFile(feat, []byte("# Feature"), 0o644); err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond)
	if count.Load() < 1 {
		t.Error("expected UpdateMsg for feature file change")
	}

	count.Store(0)

	// Bug file should still trigger.
	bug := filepath.Join(bugDir, "bug_001.md")
	if err := os.WriteFile(bug, []byte("# Bug"), 0o644); err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond)
	if count.Load() < 1 {
		t.Error("expected UpdateMsg for bug file change")
	}
}

func TestWatcherIgnoresIrrelevantInMaggusRoot(t *testing.T) {
	baseDir := t.TempDir()
	maggusDir := filepath.Join(baseDir, ".maggus")
	if err := os.MkdirAll(maggusDir, 0o755); err != nil {
		t.Fatal(err)
	}

	var count atomic.Int32
	send := func(msg any) { count.Add(1) }

	w, err := New(baseDir, send, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer w.Close()

	// A random file in .maggus/ should NOT trigger.
	file := filepath.Join(maggusDir, "config.yml")
	if err := os.WriteFile(file, []byte("model: sonnet"), 0o644); err != nil {
		t.Fatal(err)
	}

	time.Sleep(200 * time.Millisecond)

	if count.Load() != 0 {
		t.Errorf("expected 0 UpdateMsg for irrelevant file in .maggus/, got %d", count.Load())
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
