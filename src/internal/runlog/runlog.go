package runlog

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Logger writes structured run events to a flat log file in .maggus/runs/.
// All methods are safe to call on a nil Logger (no-op).
type Logger struct {
	w               *os.File
	dir             string
	currentMaggusID string
}

// ModelTokensEntry holds per-model token counts and cost for a task_usage event.
type ModelTokensEntry struct {
	InputTokens              int     `json:"input_tokens,omitempty"`
	OutputTokens             int     `json:"output_tokens,omitempty"`
	CacheCreationInputTokens int     `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int     `json:"cache_read_input_tokens,omitempty"`
	CostUSD                  float64 `json:"cost_usd,omitempty"`
}

// TaskUsageData is the parameter type for Logger.TaskUsage.
type TaskUsageData struct {
	InputTokens              int
	OutputTokens             int
	CacheCreationInputTokens int
	CacheReadInputTokens     int
	CostUSD                  float64
	ModelUsage               map[string]ModelTokensEntry
}

// Entry represents a single JSONL log entry written to the log file.
type Entry struct {
	Ts                       string            `json:"ts"`
	Level                    string            `json:"level"`
	Event                    string            `json:"event"`
	MaggusID                 string            `json:"maggus_id,omitempty"`
	FeatureID                string            `json:"feature_id,omitempty"`
	TaskID                   string            `json:"task_id,omitempty"`
	Title                    string            `json:"title,omitempty"`
	Commit                   string            `json:"commit,omitempty"`
	Tool                     string            `json:"tool,omitempty"`
	Input                    map[string]string `json:"input,omitempty"`
	Text                     string            `json:"text,omitempty"`
	Reason                   string            `json:"reason,omitempty"`
	InputTokens              int               `json:"input_tokens,omitempty"`
	OutputTokens             int               `json:"output_tokens,omitempty"`
	CacheCreationInputTokens int               `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int               `json:"cache_read_input_tokens,omitempty"`
	CostUSD                  float64           `json:"cost_usd,omitempty"`
	ModelUsage               map[string]ModelTokensEntry `json:"model_usage,omitempty"`
}

// Open creates a log file at .maggus/runs/<timestamp>_<maggusID>.log, or
// .maggus/runs/<timestamp>.log when maggusID is empty. The runs directory is
// created if it does not exist. After opening, older log files are pruned so
// that at most maxFiles log files are retained (daemon.log is never pruned).
func Open(maggusID, dir string, maxFiles int) (*Logger, error) {
	runsDir := filepath.Join(dir, ".maggus", "runs")
	if err := os.MkdirAll(runsDir, 0755); err != nil {
		return nil, fmt.Errorf("create runs dir: %w", err)
	}

	ts := time.Now().Format("20060102-150405")
	var name string
	if maggusID == "" {
		name = ts + ".log"
	} else {
		name = ts + "_" + maggusID + ".log"
	}

	logPath := filepath.Join(runsDir, name)
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("open log file: %w", err)
	}

	pruneLogFiles(runsDir, maxFiles)

	return &Logger{w: f, dir: dir}, nil
}

// pruneLogFiles removes the oldest .log files in runsDir when the count
// exceeds maxFiles. daemon.log is always excluded from pruning.
func pruneLogFiles(runsDir string, maxFiles int) {
	if maxFiles <= 0 {
		return
	}
	entries, err := os.ReadDir(runsDir)
	if err != nil {
		return
	}

	var logFiles []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if name == "daemon.log" {
			continue
		}
		if filepath.Ext(name) == ".log" {
			logFiles = append(logFiles, name)
		}
	}

	// ReadDir returns entries sorted by name; timestamp-prefixed names sort
	// chronologically, so logFiles[0] is the oldest.
	sort.Strings(logFiles)
	for len(logFiles) > maxFiles {
		_ = os.Remove(filepath.Join(runsDir, logFiles[0]))
		logFiles = logFiles[1:]
	}
}

// Close flushes and closes the log file.
func (l *Logger) Close() error {
	if l == nil || l.w == nil {
		return nil
	}
	return l.w.Close()
}

// SetCurrentMaggusID sets the maggus ID that will be injected into all subsequent log entries.
// Pass an empty string to clear the current ID (entries outside any active plan).
// Safe to call on a nil Logger (no-op).
func (l *Logger) SetCurrentMaggusID(maggusID string) {
	if l == nil {
		return
	}
	l.currentMaggusID = maggusID
}

// emit writes a single JSONL entry to the log file.
func (l *Logger) emit(entry Entry) {
	if l == nil || l.w == nil {
		return
	}
	if entry.MaggusID == "" {
		entry.MaggusID = l.currentMaggusID
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

// ToolUse logs a tool use event from the agent with structured input params.
func (l *Logger) ToolUse(taskID, toolType string, params map[string]string) {
	l.emit(Entry{Level: "info", Event: "tool_use", TaskID: taskID, Tool: toolType, Input: params})
}

// TaskUsage logs token and cost usage for a completed task.
func (l *Logger) TaskUsage(data TaskUsageData) {
	l.emit(Entry{
		Level:                    "info",
		Event:                    "task_usage",
		InputTokens:              data.InputTokens,
		OutputTokens:             data.OutputTokens,
		CacheCreationInputTokens: data.CacheCreationInputTokens,
		CacheReadInputTokens:     data.CacheReadInputTokens,
		CostUSD:                  data.CostUSD,
		ModelUsage:               data.ModelUsage,
	})
}

// Output logs agent output text for a task. The text is written as-is with no truncation.
func (l *Logger) Output(taskID, text string) {
	l.emit(Entry{Level: "output", Event: "output", TaskID: taskID, Text: text})
}

// Info logs a general informational message.
func (l *Logger) Info(msg string) {
	l.emit(Entry{Level: "info", Event: "info", Text: msg})
}
