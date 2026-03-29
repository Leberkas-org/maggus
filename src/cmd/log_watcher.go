package cmd

import (
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/fsnotify/fsnotify"
)

// logFileUpdateMsg is sent when the active log file has new content or a new
// log file has been created in the runs directory.
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

// LogFileWatcher watches the runs directory for new log files and the active
// log file for writes, delivering logFileUpdateMsg via its channel.
type LogFileWatcher struct {
	watcher    *fsnotify.Watcher
	dir        string
	activePath string
	ch         chan logFileUpdateMsg
	done       chan struct{}
}

// NewLogFileWatcher creates and starts a LogFileWatcher that watches
// .maggus/runs/ for Create events and the current active log file for Write events.
// Returns (nil, err) if fsnotify cannot be initialized.
func NewLogFileWatcher(dir string) (*LogFileWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	runsDir := filepath.Join(dir, ".maggus", "runs")
	// Watch runs directory for Create events (new log files).
	// Ignore error if the directory doesn't exist yet.
	_ = watcher.Add(runsDir)

	lfw := &LogFileWatcher{
		watcher: watcher,
		dir:     dir,
		ch:      make(chan logFileUpdateMsg, 1),
		done:    make(chan struct{}),
	}

	// Start watching the current latest log file if one exists.
	_, logPath := findLatestRunLog(dir)
	if logPath != "" {
		lfw.activePath = filepath.Clean(logPath)
		_ = watcher.Add(lfw.activePath)
	}

	go lfw.run()
	return lfw, nil
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

	// Write on the active log file → signal new content.
	if event.Has(fsnotify.Write) && name == lfw.activePath {
		select {
		case lfw.ch <- logFileUpdateMsg{}:
		default: // drop if update already pending
		}
		return
	}

	// Create of a .log file in runs dir → check if active path changed.
	if event.Has(fsnotify.Create) &&
		strings.HasSuffix(name, ".log") &&
		!strings.HasSuffix(name, "daemon.log") {
		_, newPath := findLatestRunLog(lfw.dir)
		if newPath == "" {
			return
		}
		newPath = filepath.Clean(newPath)
		if newPath != lfw.activePath {
			if lfw.activePath != "" {
				_ = lfw.watcher.Remove(lfw.activePath)
			}
			lfw.activePath = newPath
			_ = lfw.watcher.Add(newPath)
			select {
			case lfw.ch <- logFileUpdateMsg{}:
			default:
			}
		}
	}
}
