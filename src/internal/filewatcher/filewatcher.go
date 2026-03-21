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
type UpdateMsg struct{}

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
}

// New creates a Watcher that monitors the features and bugs directories
// under baseDir/.maggus/. Missing directories are silently skipped.
// The debounce duration controls how long to wait after the last event
// before sending an UpdateMsg via the send function.
func New(baseDir string, send SendFunc, debounce time.Duration) (*Watcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	dirs := []string{
		filepath.Join(baseDir, ".maggus", "features"),
		filepath.Join(baseDir, ".maggus", "bugs"),
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
	}

	w.wg.Add(1)
	go w.loop()

	return w, nil
}

// Close stops the watcher and waits for the goroutine to exit.
func (w *Watcher) Close() {
	close(w.done)
	w.fsw.Close()
	w.wg.Wait()
}

func (w *Watcher) loop() {
	defer w.wg.Done()

	var timer *time.Timer
	var timerC <-chan time.Time

	for {
		select {
		case <-w.done:
			if timer != nil {
				timer.Stop()
			}
			return

		case event, ok := <-w.fsw.Events:
			if !ok {
				return
			}
			if !isRelevantEvent(event) {
				continue
			}
			// Reset the debounce timer on each relevant event.
			if timer == nil {
				timer = time.NewTimer(w.debounce)
				timerC = timer.C
			} else {
				timer.Reset(w.debounce)
			}

		case _, ok := <-w.fsw.Errors:
			if !ok {
				return
			}
			// Ignore watcher errors — they are non-fatal.

		case <-timerC:
			w.send(UpdateMsg{})
			timer = nil
			timerC = nil
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
