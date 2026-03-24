package runlog

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// Logger writes structured run events to .maggus/runs/<runID>/run.log.
// All methods are safe to call on a nil Logger (no-op).
type Logger struct {
	w     *os.File
	runID string
	dir   string
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

// OpenDaemonLog opens daemon.log in the run directory for writing full agent output.
// It is intended for use in daemon mode; the caller is responsible for closing the writer.
func (l *Logger) OpenDaemonLog() (io.WriteCloser, error) {
	if l == nil {
		return nil, fmt.Errorf("logger is nil")
	}
	runDir := filepath.Join(l.dir, ".maggus", "runs", l.runID)
	logPath := filepath.Join(runDir, "daemon.log")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("open daemon.log: %w", err)
	}
	return f, nil
}

// log writes a single timestamped line to run.log.
func (l *Logger) log(level, msg string) {
	if l == nil || l.w == nil {
		return
	}
	ts := time.Now().UTC().Format(time.RFC3339)
	fmt.Fprintf(l.w, "%s [%s] %s\n", ts, level, msg)
}

// FeatureStart logs the start of a feature.
func (l *Logger) FeatureStart(featureID string) {
	l.log("INFO", fmt.Sprintf("Feature %s started", featureID))
}

// FeatureComplete logs the completion of a feature.
func (l *Logger) FeatureComplete(featureID string) {
	l.log("INFO", fmt.Sprintf("Feature %s complete", featureID))
}

// TaskStart logs the start of a task.
func (l *Logger) TaskStart(taskID, title string) {
	l.log("INFO", fmt.Sprintf("Task %s started: %s", taskID, title))
}

// TaskComplete logs successful task completion with the resulting commit hash.
func (l *Logger) TaskComplete(taskID, commitHash string) {
	l.log("INFO", fmt.Sprintf("Task %s complete (commit %s)", taskID, commitHash))
}

// TaskFailed logs a task failure with a reason.
func (l *Logger) TaskFailed(taskID, reason string) {
	l.log("ERROR", fmt.Sprintf("Task %s failed: %s", taskID, reason))
}

// ToolUse logs a tool use event from the agent.
func (l *Logger) ToolUse(taskID, toolType, description string) {
	l.log("INFO", fmt.Sprintf("Task %s tool: [%s] %s", taskID, toolType, description))
}

// Output logs agent output text for a task. The text is written as-is with no truncation.
func (l *Logger) Output(taskID, text string) {
	l.log("OUTPUT", fmt.Sprintf("[%s] %s", taskID, text))
}
