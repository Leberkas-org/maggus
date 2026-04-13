package cmd

import (
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/fsnotify/fsnotify"
)

// logFileUpdateMsg is sent when the active log file has new content or a new
// log file has been created in the logs directory.
type logFileUpdateMsg struct{}

// listenForLogFileUpdate returns a Cmd that blocks until the log watcher channel
// delivers an update, then delivers a logFileUpdateMsg to the TUI.
func listenForLogFileUpdate(ch <-chan logFileUpdateMsg) tea.Cmd {
	if ch == nil {
		return nil
	}
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return nil // channel closed, watcher stopped
		}
		return msg
	}
}

// LogFileWatcher watches .maggus/logs/ for new log files and writes,
// and .maggus/runs/state.json for daemon state changes.
// It delivers logFileUpdateMsg via its channel.
type LogFileWatcher struct {
	watcher       *fsnotify.Watcher
	dir           string
	stateJsonPath string
	ch            chan logFileUpdateMsg
	done          chan struct{}
}

// NewLogFileWatcher creates and starts a LogFileWatcher that watches
// .maggus/logs/ for new .log files and writes, and state.json for daemon state.
// Returns (nil, err) if fsnotify cannot be initialized.
func NewLogFileWatcher(dir string) (*LogFileWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	// Watch .maggus/logs/ for Create events (new subdirectories and log files).
	logsDir := filepath.Join(dir, ".maggus", "logs")
	_ = watcher.Add(logsDir)

	// Also watch any existing feature subdirectories.
	addExistingLogSubdirs(watcher, logsDir)

	runsDir := filepath.Join(dir, ".maggus", "runs")
	stateJsonPath := filepath.Join(runsDir, "state.json")

	lfw := &LogFileWatcher{
		watcher:       watcher,
		dir:           dir,
		stateJsonPath: filepath.Clean(stateJsonPath),
		ch:            make(chan logFileUpdateMsg, 1),
		done:          make(chan struct{}),
	}

	// Ensure the runs directory exists and watch it so we catch state.json
	// creation even when the file doesn't exist at startup (e.g., the daemon
	// hasn't run yet). On Windows, watcher.Add on a non-existent file fails
	// silently; watching the parent directory is the reliable fallback.
	_ = os.MkdirAll(runsDir, 0755)
	_ = watcher.Add(runsDir)

	// Also try watching state.json directly for more reliable Write events
	// on platforms where directory-level Write notifications are not always
	// delivered for existing files.
	_ = watcher.Add(stateJsonPath)

	go lfw.run()
	return lfw, nil
}

// addExistingLogSubdirs adds all subdirectories of logsDir to the watcher so
// that writes to existing log files are detected on startup.
func addExistingLogSubdirs(w *fsnotify.Watcher, logsDir string) {
	entries, err := os.ReadDir(logsDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() {
			_ = w.Add(filepath.Join(logsDir, e.Name()))
		}
	}
}

// Chan returns the receive-only channel that delivers logFileUpdateMsg.
func (lfw *LogFileWatcher) Chan() <-chan logFileUpdateMsg {
	return lfw.ch
}

// Stop closes the fsnotify watcher and waits for the goroutine to exit cleanly.
func (lfw *LogFileWatcher) Stop() {
	lfw.watcher.Close()
	<-lfw.done
}

func (lfw *LogFileWatcher) run() {
	defer close(lfw.done)
	for {
		select {
		case event, ok := <-lfw.watcher.Events:
			if !ok {
				return
			}
			lfw.handleEvent(event)
		case _, ok := <-lfw.watcher.Errors:
			if !ok {
				return
			}
			// Ignore watcher errors; continue watching.
		}
	}
}

func (lfw *LogFileWatcher) handleEvent(event fsnotify.Event) {
	name := filepath.Clean(event.Name)

	// Write on state.json → signal update (daemon started a new run).
	if event.Has(fsnotify.Write) && name == lfw.stateJsonPath {
		lfw.signal()
		return
	}

	// Create of state.json → add it to the watcher (wasn't present at init).
	if event.Has(fsnotify.Create) && name == lfw.stateJsonPath {
		_ = lfw.watcher.Add(lfw.stateJsonPath)
		lfw.signal()
		return
	}

	// Create of a new directory under .maggus/logs/ → add it to the watcher.
	// Skip .tmp files: atomic writes (WriteSnapshot) create a .tmp file then
	// rename it to the real target. The .tmp Create event would fill the
	// buffered signal channel, causing the subsequent real file event to be
	// dropped. Filtering .tmp avoids this race.
	if event.Has(fsnotify.Create) && !strings.HasSuffix(name, ".tmp") {
		// Add the new path to the watcher so we receive events from inside it.
		_ = lfw.watcher.Add(name)
		lfw.signal()
		return
	}

	// Write on any .log file → signal new content.
	if event.Has(fsnotify.Write) && strings.HasSuffix(name, ".log") {
		lfw.signal()
		return
	}
}

func (lfw *LogFileWatcher) signal() {
	select {
	case lfw.ch <- logFileUpdateMsg{}:
	default: // drop if update already pending
	}
}
