package runlog

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Logger writes structured run events to .maggus/runs/<runID>/run.log.
// All methods are safe to call on a nil Logger (no-op).
type Logger struct {
	w             *os.File
	runID         string
	dir           string
	currentItemID string
}

// Entry represents a single JSONL log entry written to run.log.
type Entry struct {
	Ts          string `json:"ts"`
	Level       string `json:"level"`
	Event       string `json:"event"`
	ItemID      string `json:"item_id,omitempty"`
	FeatureID   string `json:"feature_id,omitempty"`
	TaskID      string `json:"task_id,omitempty"`
	Title       string `json:"title,omitempty"`
	Commit      string `json:"commit,omitempty"`
	Tool        string `json:"tool,omitempty"`
	Description string `json:"description,omitempty"`
	Text        string `json:"text,omitempty"`
	Reason      string `json:"reason,omitempty"`
}

// Open creates or opens the run log at .maggus/runs/<runID>/run.log.
// The run directory is created if it does not exist.
func Open(runID, dir string) (*Logger, error) {
	runDir := filepath.Join(dir, ".maggus", "runs", runID)
	if err := os.MkdirAll(runDir, 0755); err != nil {
		return nil, fmt.Errorf("create run dir: %w", err)
	}
	logPath := filepath.Join(runDir, "run.log")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("open run.log: %w", err)
	}
	return &Logger{w: f, runID: runID, dir: dir}, nil
}

// Close flushes and closes the log file.
func (l *Logger) Close() error {
	if l == nil || l.w == nil {
		return nil
	}
	return l.w.Close()
}

// SetCurrentItem sets the item ID that will be injected into all subsequent log entries.
// Pass an empty string to clear the current item (entries outside any active plan).
// Safe to call on a nil Logger (no-op).
func (l *Logger) SetCurrentItem(itemID string) {
	if l == nil {
		return
	}
	l.currentItemID = itemID
}

// emit writes a single JSONL entry to run.log.
func (l *Logger) emit(entry Entry) {
	if l == nil || l.w == nil {
		return
	}
	if entry.ItemID == "" {
		entry.ItemID = l.currentItemID
	}
	entry.Ts = time.Now().UTC().Format(time.RFC3339)
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	l.w.Write(data)
	l.w.Write([]byte("\n"))
}

// FeatureStart logs the start of a feature.
func (l *Logger) FeatureStart(featureID string) {
	l.emit(Entry{Level: "info", Event: "feature_start", FeatureID: featureID})
}

// FeatureComplete logs the completion of a feature.
func (l *Logger) FeatureComplete(featureID string) {
	l.emit(Entry{Level: "info", Event: "feature_complete", FeatureID: featureID})
}

// TaskStart logs the start of a task.
func (l *Logger) TaskStart(taskID, title string) {
	l.emit(Entry{Level: "info", Event: "task_start", TaskID: taskID, Title: title})
}

// TaskComplete logs successful task completion with the resulting commit hash.
func (l *Logger) TaskComplete(taskID, commitHash string) {
	l.emit(Entry{Level: "info", Event: "task_complete", TaskID: taskID, Commit: commitHash})
}

// TaskFailed logs a task failure with a reason.
func (l *Logger) TaskFailed(taskID, reason string) {
	l.emit(Entry{Level: "error", Event: "task_failed", TaskID: taskID, Reason: reason})
}

// ToolUse logs a tool use event from the agent.
func (l *Logger) ToolUse(taskID, toolType, description string) {
	l.emit(Entry{Level: "info", Event: "tool_use", TaskID: taskID, Tool: toolType, Description: description})
}

// Output logs agent output text for a task. The text is written as-is with no truncation.
func (l *Logger) Output(taskID, text string) {
	l.emit(Entry{Level: "output", Event: "output", TaskID: taskID, Text: text})
}

// Info logs a general informational message.
func (l *Logger) Info(msg string) {
	l.emit(Entry{Level: "info", Event: "info", Text: msg})
}
