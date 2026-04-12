package runlog

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Logger writes structured run events to per-feature log files in
// .maggus/logs/<maggus_id>/<pid>.log. The active file is switched by
// SetCurrentMaggusID. All methods are safe to call on a nil Logger (no-op).
type Logger struct {
	w               *os.File
	logsDir         string
	pid             string
	maxFiles        int
	currentMaggusID string
	currentTaskID   string
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
	TaskID                   string
	InputTokens              int
	OutputTokens             int
	CacheCreationInputTokens int
	CacheReadInputTokens     int
	CostUSD                  float64
	ModelUsage               map[string]ModelTokensEntry
}

// Entry represents a single JSONL log entry written to the log file.
type Entry struct {
	Ts                       string                      `json:"ts"`
	Level                    string                      `json:"level"`
	Event                    string                      `json:"event"`
	MaggusID                 string                      `json:"maggus_id,omitempty"`
	FeatureID                string                      `json:"feature_id,omitempty"`
	TaskID                   string                      `json:"task_id,omitempty"`
	Title                    string                      `json:"title,omitempty"`
	Commit                   string                      `json:"commit,omitempty"`
	Tool                     string                      `json:"tool,omitempty"`
	Input                    map[string]string           `json:"input,omitempty"`
	Text                     string                      `json:"text,omitempty"`
	Reason                   string                      `json:"reason,omitempty"`
	InputTokens              int                         `json:"input_tokens,omitempty"`
	OutputTokens             int                         `json:"output_tokens,omitempty"`
	CacheCreationInputTokens int                         `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int                         `json:"cache_read_input_tokens,omitempty"`
	CostUSD                  float64                     `json:"cost_usd,omitempty"`
	ModelUsage               map[string]ModelTokensEntry `json:"model_usage,omitempty"`
}

// Open creates a Logger that writes to .maggus/logs/<maggus_id>/<pid>.log.
// No file is opened until SetCurrentMaggusID is called with a non-empty ID.
// The logs base directory is created if it does not exist.
func Open(dir string, maxFiles int) (*Logger, error) {
	logsDir := filepath.Join(dir, ".maggus", "logs")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return nil, fmt.Errorf("create logs dir: %w", err)
	}

	return &Logger{
		logsDir:  logsDir,
		pid:      fmt.Sprintf("%d", os.Getpid()),
		maxFiles: maxFiles,
	}, nil
}

// pruneLogFiles removes the oldest .log files in dir when the count
// exceeds maxFiles. The file named exclude is never pruned (used to
// protect the currently active log file).
func pruneLogFiles(dir string, maxFiles int, exclude string) {
	if maxFiles <= 0 {
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	var logFiles []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if name == exclude {
			continue
		}
		if filepath.Ext(name) == ".log" {
			logFiles = append(logFiles, name)
		}
	}

	// maxFiles includes the excluded file, so the budget for pruneable files
	// is maxFiles-1.
	limit := maxFiles - 1
	sort.Strings(logFiles)
	for len(logFiles) > limit {
		_ = os.Remove(filepath.Join(dir, logFiles[0]))
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

// SetCurrentMaggusID switches the active log file to .maggus/logs/<maggusID>/<pid>.log.
// If maggusID is empty, the current file is closed and subsequent entries are
// silently dropped until a non-empty ID is set.
// Safe to call on a nil Logger (no-op).
func (l *Logger) SetCurrentMaggusID(maggusID string) {
	if l == nil {
		return
	}

	// Close the current file handle if open.
	if l.w != nil {
		l.w.Close()
		l.w = nil
	}

	l.currentMaggusID = maggusID

	if maggusID == "" {
		return
	}

	// Create the per-feature directory and open the log file.
	featureDir := filepath.Join(l.logsDir, maggusID)
	if err := os.MkdirAll(featureDir, 0755); err != nil {
		return
	}

	logPath := filepath.Join(featureDir, l.pid+".log")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	l.w = f

	pruneLogFiles(featureDir, l.maxFiles, l.pid+".log")
}

// emit writes a single JSONL entry to the log file.
func (l *Logger) emit(entry Entry) {
	if l == nil || l.w == nil {
		return
	}
	if entry.MaggusID == "" {
		entry.MaggusID = l.currentMaggusID
	}
	if entry.TaskID == "" && l.currentTaskID != "" {
		entry.TaskID = l.currentTaskID
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

// TaskStart logs the start of a task and sets the current task ID so that
// subsequent Info entries are tagged with the active task.
func (l *Logger) TaskStart(taskID, title string) {
	if l != nil {
		l.currentTaskID = taskID
	}
	l.emit(Entry{Level: "info", Event: "task_start", TaskID: taskID, Title: title})
}

// TaskComplete logs successful task completion with the resulting commit hash
// and clears the current task ID.
func (l *Logger) TaskComplete(taskID, commitHash string) {
	l.emit(Entry{Level: "info", Event: "task_complete", TaskID: taskID, Commit: commitHash})
	if l != nil {
		l.currentTaskID = ""
	}
}

// TaskFailed logs a task failure with a reason and clears the current task ID.
func (l *Logger) TaskFailed(taskID, reason string) {
	l.emit(Entry{Level: "error", Event: "task_failed", TaskID: taskID, Reason: reason})
	if l != nil {
		l.currentTaskID = ""
	}
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
		TaskID:                   data.TaskID,
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
