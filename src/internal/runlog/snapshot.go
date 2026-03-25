package runlog

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
	TaskID         string                      `json:"task_id"`
	TaskTitle      string                      `json:"task_title"`
	FeatureFile    string                      `json:"feature_file"`
	Status         string                      `json:"status"`
	ToolEntries    []SnapshotToolEntry         `json:"tool_entries"`
	TokenInput     int                         `json:"token_input"`
	TokenOutput    int                         `json:"token_output"`
	TokenCost      float64                     `json:"token_cost"`
	ModelBreakdown map[string]agent.ModelTokens `json:"model_breakdown"`
	Commits        []string                    `json:"commits"`
	UpdatedAt      string                      `json:"updated_at"`
}

// snapshotPath returns the path to state.json for the given run.
func snapshotPath(dir, runID string) string {
	return filepath.Join(dir, ".maggus", "runs", runID, "state.json")
}

// WriteSnapshot atomically writes the state snapshot to state.json.
// It writes to a temporary file first, then renames it into place.
func WriteSnapshot(dir, runID string, snap StateSnapshot) error {
	target := snapshotPath(dir, runID)

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
func RemoveSnapshot(dir, runID string) {
	target := snapshotPath(dir, runID)
	os.Remove(target)
	os.Remove(target + ".tmp") // clean up any leftover temp file
}

// ReadSnapshot reads and parses the state.json snapshot for the given run.
func ReadSnapshot(dir, runID string) (*StateSnapshot, error) {
	target := snapshotPath(dir, runID)
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
