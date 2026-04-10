package runlog

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/leberkas-org/maggus/internal/agent"
)

// SnapshotToolEntry represents a single tool invocation in the state snapshot.
type SnapshotToolEntry struct {
	Type        string `json:"type"`
	Icon        string `json:"icon"`
	Description string `json:"description"`
	Timestamp   string `json:"timestamp"`
}

// StateSnapshot is the live state written to state.json by the daemon work loop.
// The status view reads this file to render a rich TUI without IPC.
type StateSnapshot struct {
	RunID          string                       `json:"run_id"`
	TaskID         string                       `json:"task_id"`
	TaskTitle      string                       `json:"task_title"`
	ItemTitle      string                       `json:"item_title"`
	Status         string                       `json:"status"`
	ToolEntries    []SnapshotToolEntry          `json:"tool_entries"`
	TokenInput     int                          `json:"token_input"`
	TokenOutput    int                          `json:"token_output"`
	TokenCost      float64                      `json:"token_cost"`
	ModelBreakdown map[string]agent.ModelTokens `json:"model_breakdown"`
	Commits        []string                     `json:"commits"`
	RunStartedAt   string                       `json:"run_started_at,omitempty"`
	TaskStartedAt  string                       `json:"task_started_at,omitempty"`
	UpdatedAt      string                       `json:"updated_at"`
}

// snapshotPath returns the fixed path to state.json.
func snapshotPath(dir string) string {
	return filepath.Join(dir, ".maggus", "runs", "state.json")
}

// WriteSnapshot atomically writes the state snapshot to state.json.
// It writes to a temporary file first, then renames it into place.
func WriteSnapshot(dir string, snap StateSnapshot) error {
	target := snapshotPath(dir)

	// Ensure the directory exists.
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return fmt.Errorf("create snapshot dir: %w", err)
	}

	snap.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	data, err := json.Marshal(snap)
	if err != nil {
		return fmt.Errorf("marshal snapshot: %w", err)
	}

	// Write to temp file in the same directory, then rename for atomicity.
	tmp := target + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("write temp snapshot: %w", err)
	}
	if err := os.Rename(tmp, target); err != nil {
		// On Windows, rename can fail if target exists; remove and retry.
		os.Remove(target)
		if err2 := os.Rename(tmp, target); err2 != nil {
			os.Remove(tmp)
			return fmt.Errorf("rename snapshot: %w", err2)
		}
	}
	return nil
}

// RemoveSnapshot removes the state.json file for a clean daemon exit.
func RemoveSnapshot(dir string) {
	target := snapshotPath(dir)
	os.Remove(target)
	os.Remove(target + ".tmp") // clean up any leftover temp file
}

// ReadSnapshot reads and parses the state.json snapshot.
func ReadSnapshot(dir string) (*StateSnapshot, error) {
	target := snapshotPath(dir)
	data, err := os.ReadFile(target)
	if err != nil {
		return nil, err
	}
	var snap StateSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, fmt.Errorf("parse snapshot: %w", err)
	}
	return &snap, nil
}

// ── Per-worker snapshots for parallel mode ──

// WorkerIndexEntry describes one parallel worker's identity and status.
type WorkerIndexEntry struct {
	TaskID    string `json:"task_id"`
	TaskTitle string `json:"task_title"`
	Status    string `json:"status"` // "working", "done", "failed", "blocked"
	StartedAt string `json:"started_at,omitempty"`
}

// WorkerIndex is the on-disk list of parallel workers.
type WorkerIndex struct {
	Workers []WorkerIndexEntry `json:"workers"`
}

// workersIndexPath returns the fixed path to state-workers.json.
func workersIndexPath(dir string) string {
	return filepath.Join(dir, ".maggus", "runs", "state-workers.json")
}

// workerSnapshotPath returns the path for a per-worker snapshot.
func workerSnapshotPath(dir, taskID string) string {
	return filepath.Join(dir, ".maggus", "runs", "state-"+taskID+".json")
}

// WriteWorkersIndex atomically writes the workers index file.
func WriteWorkersIndex(dir string, workers []WorkerIndexEntry) error {
	target := workersIndexPath(dir)
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return err
	}
	data, err := json.Marshal(WorkerIndex{Workers: workers})
	if err != nil {
		return err
	}
	tmp := target + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	if err := os.Rename(tmp, target); err != nil {
		os.Remove(target)
		if err2 := os.Rename(tmp, target); err2 != nil {
			os.Remove(tmp)
			return err2
		}
	}
	return nil
}

// ReadWorkersIndex reads the workers index. Returns nil slice when the file
// does not exist (non-parallel mode or run finished).
func ReadWorkersIndex(dir string) []WorkerIndexEntry {
	data, err := os.ReadFile(workersIndexPath(dir))
	if err != nil {
		return nil
	}
	var idx WorkerIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil
	}
	return idx.Workers
}

// RemoveWorkersIndex removes the workers index file (called when a parallel run finishes).
func RemoveWorkersIndex(dir string) {
	target := workersIndexPath(dir)
	os.Remove(target)
	os.Remove(target + ".tmp")
}

// WriteWorkerSnapshot atomically writes a per-worker snapshot.
func WriteWorkerSnapshot(dir, taskID string, snap StateSnapshot) error {
	target := workerSnapshotPath(dir, taskID)
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return err
	}
	snap.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	data, err := json.Marshal(snap)
	if err != nil {
		return err
	}
	tmp := target + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	if err := os.Rename(tmp, target); err != nil {
		os.Remove(target)
		if err2 := os.Rename(tmp, target); err2 != nil {
			os.Remove(tmp)
			return err2
		}
	}
	return nil
}

// ReadWorkerSnapshot reads a single per-worker snapshot.
func ReadWorkerSnapshot(dir, taskID string) (*StateSnapshot, error) {
	data, err := os.ReadFile(workerSnapshotPath(dir, taskID))
	if err != nil {
		return nil, err
	}
	var snap StateSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, err
	}
	return &snap, nil
}

// RemoveWorkerSnapshot removes a per-worker snapshot file.
func RemoveWorkerSnapshot(dir, taskID string) {
	target := workerSnapshotPath(dir, taskID)
	os.Remove(target)
	os.Remove(target + ".tmp")
}

// RemoveAllWorkerSnapshots removes all per-worker snapshot files and the index.
func RemoveAllWorkerSnapshots(dir string) {
	runsDir := filepath.Join(dir, ".maggus", "runs")
	entries, err := os.ReadDir(runsDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, "state-") && strings.HasSuffix(name, ".json") && name != "state-workers.json" {
			os.Remove(filepath.Join(runsDir, name))
		}
		if strings.HasPrefix(name, "state-") && strings.HasSuffix(name, ".json.tmp") {
			os.Remove(filepath.Join(runsDir, name))
		}
	}
	RemoveWorkersIndex(dir)
}
