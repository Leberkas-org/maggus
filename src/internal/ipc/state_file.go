package ipc

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
)

type FileStateWriter struct {
	path string
}

func NewFileStateWriter(globalDir string) *FileStateWriter {
	return &FileStateWriter{path: filepath.Join(globalDir, "state.json")}
}

func (w *FileStateWriter) WriteState(state DaemonSnapshot) error {
	state.UpdatedAt = time.Now()
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}

	tmp := w.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write temp state: %w", err)
	}
	if err := os.Rename(tmp, w.path); err != nil {
		return fmt.Errorf("rename state: %w", err)
	}
	return nil
}

type FileStateReader struct {
	path string
}

func NewFileStateReader(globalDir string) *FileStateReader {
	return &FileStateReader{path: filepath.Join(globalDir, "state.json")}
}

func (r *FileStateReader) ReadState() (DaemonSnapshot, error) {
	data, err := os.ReadFile(r.path)
	if err != nil {
		if os.IsNotExist(err) {
			return DaemonSnapshot{}, nil
		}
		return DaemonSnapshot{}, fmt.Errorf("read state: %w", err)
	}

	var snap DaemonSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return DaemonSnapshot{}, fmt.Errorf("unmarshal state: %w", err)
	}
	return snap, nil
}

func (r *FileStateReader) Watch(ctx context.Context) <-chan DaemonSnapshot {
	ch := make(chan DaemonSnapshot, 1)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		slog.Warn("fsnotify init failed, falling back to polling", "error", err)
		go r.pollWatch(ctx, ch)
		return ch
	}

	dir := filepath.Dir(r.path)
	if err := watcher.Add(dir); err != nil {
		slog.Warn("fsnotify watch failed, falling back to polling", "error", err)
		watcher.Close()
		go r.pollWatch(ctx, ch)
		return ch
	}

	go func() {
		defer close(ch)
		defer watcher.Close()

		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Name != r.path {
					continue
				}
				if !event.Has(fsnotify.Write) && !event.Has(fsnotify.Create) {
					continue
				}
				snap, err := r.ReadState()
				if err != nil {
					continue
				}
				select {
				case ch <- snap:
				default:
				}
			case _, ok := <-watcher.Errors:
				if !ok {
					return
				}
			}
		}
	}()

	return ch
}

func (r *FileStateReader) pollWatch(ctx context.Context, ch chan<- DaemonSnapshot) {
	defer close(ch)
	var lastMod time.Time

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			info, err := os.Stat(r.path)
			if err != nil {
				continue
			}
			if info.ModTime().After(lastMod) {
				lastMod = info.ModTime()
				snap, err := r.ReadState()
				if err != nil {
					continue
				}
				select {
				case ch <- snap:
				default:
				}
			}
		}
	}
}
