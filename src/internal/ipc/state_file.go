package ipc

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
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

	go func() {
		defer close(ch)
		var lastMod time.Time

		ticker := time.NewTicker(200 * time.Millisecond)
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
	}()

	return ch
}
