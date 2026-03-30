package cmd

import (
	"os"
	"path/filepath"
	"sync"

	"github.com/charmbracelet/bubbletea"
	"github.com/fsnotify/fsnotify"
)

// daemonPIDState holds the cached daemon PID, running flag, and stop-after-task sentinel.
type daemonPIDState struct {
	PID                int
	Running            bool
	StoppingAfterTask  bool
}

// DaemonStateCache watches the .maggus/ directory for changes to daemon.pid,
// caches the current PID/running state, and notifies subscribers via channels.
type DaemonStateCache struct {
	mu          sync.RWMutex
	state       daemonPIDState
	watcher     *fsnotify.Watcher
	subscribers []chan daemonPIDState
	done        chan struct{}
	wg          sync.WaitGroup
	dir         string
}

// NewDaemonStateCache creates and starts a DaemonStateCache that watches dir/.maggus/.
// The constructor calls reload() synchronously so the first Get() is always populated.
func NewDaemonStateCache(dir string) (*DaemonStateCache, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	watchDir := filepath.Join(dir, ".maggus")
	if err := watcher.Add(watchDir); err != nil {
		watcher.Close()
		return nil, err
	}

	c := &DaemonStateCache{
		watcher: watcher,
		done:    make(chan struct{}),
		dir:     dir,
	}

	c.reload()

	c.wg.Add(1)
	go c.loop()

	return c, nil
}

// Get returns the cached daemon state without any I/O.
func (c *DaemonStateCache) Get() daemonPIDState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state
}

// Subscribe returns a buffered channel that receives state updates whenever
// the daemon state changes. The caller must eventually call Unsubscribe.
func (c *DaemonStateCache) Subscribe() chan daemonPIDState {
	ch := make(chan daemonPIDState, 1)
	c.mu.Lock()
	c.subscribers = append(c.subscribers, ch)
	c.mu.Unlock()
	return ch
}

// Unsubscribe removes the channel from the subscriber list.
func (c *DaemonStateCache) Unsubscribe(ch chan daemonPIDState) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i, sub := range c.subscribers {
		if sub == ch {
			c.subscribers = append(c.subscribers[:i], c.subscribers[i+1:]...)
			return
		}
	}
}

// Stop shuts down the background loop and closes all subscriber channels.
func (c *DaemonStateCache) Stop() {
	close(c.done)
	c.watcher.Close()
	c.wg.Wait()

	c.mu.Lock()
	defer c.mu.Unlock()
	for _, ch := range c.subscribers {
		close(ch)
	}
	c.subscribers = nil
}

// reload reads daemon.pid, isProcessRunning, and the stop-after-task sentinel,
// then notifies subscribers only if the state actually changed.
func (c *DaemonStateCache) reload() {
	pid, _ := readDaemonPID(c.dir)
	running := pid != 0 && isProcessRunning(pid)
	var stoppingAfterTask bool
	if running {
		_, err := os.Stat(daemonStopAfterTaskFilePath(c.dir))
		stoppingAfterTask = err == nil
	}
	newState := daemonPIDState{PID: pid, Running: running, StoppingAfterTask: stoppingAfterTask}

	c.mu.Lock()
	changed := c.state != newState
	if changed {
		c.state = newState
	}
	c.mu.Unlock()

	if changed {
		c.notify(newState)
	}
}

// notify fans out state to all subscribers using non-blocking sends.
func (c *DaemonStateCache) notify(state daemonPIDState) {
	c.mu.RLock()
	subs := make([]chan daemonPIDState, len(c.subscribers))
	copy(subs, c.subscribers)
	c.mu.RUnlock()

	for _, ch := range subs {
		select {
		case ch <- state:
		default:
			// drop stale update if channel is full
		}
	}
}

// loop processes fsnotify events and calls reload on relevant daemon.pid events.
func (c *DaemonStateCache) loop() {
	defer c.wg.Done()
	for {
		select {
		case <-c.done:
			return
		case event, ok := <-c.watcher.Events:
			if !ok {
				return
			}
			base := filepath.Base(event.Name)
			if base != "daemon.pid" && base != "daemon.stop-after-task" {
				continue
			}
			if event.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Remove|fsnotify.Rename) == 0 {
				continue
			}
			c.reload()
		case _, ok := <-c.watcher.Errors:
			if !ok {
				return
			}
		}
	}
}

// daemonCacheUpdateMsg is the Bubble Tea message delivered when daemon state changes.
type daemonCacheUpdateMsg struct {
	State daemonPIDState
}

// listenForDaemonCacheUpdate returns a Cmd that blocks until the cache channel
// delivers an update, then delivers a daemonCacheUpdateMsg.
func listenForDaemonCacheUpdate(ch <-chan daemonPIDState) tea.Cmd {
	if ch == nil {
		return nil
	}
	return func() tea.Msg {
		state, ok := <-ch
		if !ok {
			return nil // channel closed, cache stopped
		}
		return daemonCacheUpdateMsg{State: state}
	}
}
