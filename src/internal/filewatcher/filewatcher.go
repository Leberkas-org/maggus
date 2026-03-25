package filewatcher

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// UpdateMsg is sent to the bubbletea program when watched files change.
// HasNewFile is true when at least one fsnotify.Create event was seen during
// the debounce window; false for pure Write/Remove/Rename bursts.
// Path contains the name of the last file event that triggered the message.
type UpdateMsg struct {
	HasNewFile bool
	Path       string
}

// SendFunc is a function that sends a message. In production this is
// tea.Program.Send; in tests it can be any function.
type SendFunc func(msg any)

// Watcher watches .maggus/features/ and .maggus/bugs/ directories for
// changes to feature_*.md and bug_*.md files, debouncing rapid events
// and sending UpdateMsg via the provided send function.
type Watcher struct {
	fsw      *fsnotify.Watcher
	send     SendFunc
	debounce time.Duration
	done     chan struct{}
	wg       sync.WaitGroup
	dirs     []string
}

// New creates a Watcher that monitors the features and bugs directories
// under baseDir/.maggus/. Missing directories are silently skipped.
// The debounce duration controls how long to wait after the last event
// before sending an UpdateMsg via the send function.
func New(baseDir string, send SendFunc, debounce time.Duration) (*Watcher, error) {
	dirs := []string{
		filepath.Join(baseDir, ".maggus", "features"),
		filepath.Join(baseDir, ".maggus", "bugs"),
	}

	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	for _, d := range dirs {
		if info, err := os.Stat(d); err == nil && info.IsDir() {
			_ = fsw.Add(d)
		}
	}

	// If no directories exist, still return a valid watcher — it just won't
	// fire events until directories are created.
	w := &Watcher{
		fsw:      fsw,
		send:     send,
		debounce: debounce,
		done:     make(chan struct{}),
		dirs:     dirs,
	}

	w.wg.Add(1)
	go w.loop()

	return w, nil
}

// Close stops the watcher and waits for the goroutine to exit.
// Closing w.done is sufficient to unblock the loop goroutine; the
// underlying fsnotify.Watcher is closed afterward once the goroutine exits.
func (w *Watcher) Close() {
	close(w.done)
	w.wg.Wait()
	w.fsw.Close()
}

// reconnect recreates the underlying fsnotify.Watcher and re-adds the
// watched directories. It uses exponential backoff (100 ms → 30 s) between
// attempts. Returns false if w.done is closed (caller should exit).
func (w *Watcher) reconnect() bool {
	const (
		baseDelay = 100 * time.Millisecond
		maxDelay  = 30 * time.Second
	)
	delay := baseDelay

	for {
		select {
		case <-w.done:
			return false
		case <-time.After(delay):
		}

		fsw, err := fsnotify.NewWatcher()
		if err != nil {
			delay = min(delay*2, maxDelay)
			continue
		}

		for _, d := range w.dirs {
			if info, err := os.Stat(d); err == nil && info.IsDir() {
				_ = fsw.Add(d)
			}
		}

		// Check done once more before committing the new watcher, so we
		// don't leak it if Close() was called while we were creating it.
		select {
		case <-w.done:
			fsw.Close()
			return false
		default:
		}

		old := w.fsw
		w.fsw = fsw
		old.Close()
		return true
	}
}

func (w *Watcher) loop() {
	defer w.wg.Done()

	var timer *time.Timer
	var timerC <-chan time.Time
	var hasCreate bool  // true if any Create event seen in the current debounce window
	var lastPath string // path of the last relevant event in the current debounce window

	for {
		select {
		case <-w.done:
			if timer != nil {
				timer.Stop()
			}
			return

		case event, ok := <-w.fsw.Events:
			if !ok {
				// fsnotify channel closed unexpectedly — attempt to reconnect.
				if !w.reconnect() {
					return
				}
				continue
			}
			if !isRelevantEvent(event) {
				continue
			}
			if event.Op&fsnotify.Create != 0 {
				hasCreate = true
			}
			lastPath = event.Name
			// Reset the debounce timer on each relevant event.
			if timer == nil {
				timer = time.NewTimer(w.debounce)
				timerC = timer.C
			} else {
				timer.Reset(w.debounce)
			}

		case _, ok := <-w.fsw.Errors:
			if !ok {
				// fsnotify channel closed unexpectedly — attempt to reconnect.
				if !w.reconnect() {
					return
				}
				continue
			}
			// Ignore watcher errors — they are non-fatal.

		case <-timerC:
			w.send(UpdateMsg{HasNewFile: hasCreate, Path: lastPath})
			timer = nil
			timerC = nil
			hasCreate = false
			lastPath = ""
		}
	}
}

// isRelevantEvent returns true if the event is a file modification on a
// feature_*.md or bug_*.md file.
func isRelevantEvent(event fsnotify.Event) bool {
	if event.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Remove|fsnotify.Rename) == 0 {
		return false
	}
	name := filepath.Base(event.Name)
	if strings.HasPrefix(name, "feature_") && strings.HasSuffix(name, ".md") {
		return true
	}
	if strings.HasPrefix(name, "bug_") && strings.HasSuffix(name, ".md") {
		return true
	}
	return false
}
